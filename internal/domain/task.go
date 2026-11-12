package domain

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	pu "github.com/bricef/htt/internal/parseutils"
	"github.com/bricef/htt/internal/vars"
	"github.com/fatih/color"
	parsec "github.com/prataprc/goparsec"
)

// Domain errors. Callers can match on these with errors.Is.
var (
	ErrEmptyTask              = errors.New("cannot create task from empty string")
	ErrInvalidAnnotationKey   = errors.New("invalid annotation key")
	ErrInvalidAnnotationValue = errors.New("invalid annotation value")
	ErrMalformedTask          = errors.New("malformed task input")
)

// Task represents a task in a todo list.
// for tasks that is stable even though they might be filtered.
type Task struct {
	Raw         string // This is the entire line, including all parsed items, as it would appear in the file
	Line        int
	Completed   bool
	Priority    string
	CompletedOn time.Time
	CreatedOn   time.Time
	entry       string              // This is the task without priority, annotations, completion mark or created/completed dates
	Tags        map[string][]string // { "@": ["abc", "def"], "#": ["foo"]}
	Annotations map[string]string   // by key; for a key the line repeats, its last value
	body        []bodyWord          // the words after the preamble, in line order; rebuild writes them back
	parser      *pu.Parser
}

// bodyWord is one word of a line after its preamble: a word of the
// entry, or an annotation (key is set) as written.
type bodyWord struct {
	text string
	key  string
}

var (
	priorities = []string{"A", "B", "C", ""}
)

// NewTask parses raw into a Task. Returns ErrEmptyTask if raw is empty,
// or a wrapped ErrMalformedTask if any field fails to parse cleanly.
func NewTask(raw string) (*Task, error) {
	if raw == "" {
		return nil, ErrEmptyTask
	}
	t := &Task{Raw: raw}

	parser := NewTodoParser()
	t.parser = parser
	// The grammar reads any line, so an incomplete parse is a grammar
	// bug. Refuse it: rewriting the task would drop the unread text.
	if _, complete := parser.Parse(raw); !complete {
		return nil, fmt.Errorf("%w: could not read all of %q", ErrMalformedTask, raw)
	}

	if parser.QueryOne("COMPLETED") != nil {
		t.Completed = true
	}

	if pri := parser.QueryOne("PRIORITY"); pri != nil {
		t.Priority = pri.GetValue()
	}

	if completedAt := parser.QueryOne("COMPLETEDAT"); completedAt != nil {
		date, err := time.ParseInLocation("2006-01-02", completedAt.GetValue(), time.Local)
		if err != nil {
			return nil, fmt.Errorf("%w: parsing completed date %q: %v", ErrMalformedTask, completedAt.GetValue(), err)
		}
		t.CompletedOn = date
	}

	if createdAt := parser.QueryOne("CREATEDAT"); createdAt != nil {
		date, err := time.ParseInLocation("2006-01-02", createdAt.GetValue(), time.Local)
		if err != nil {
			return nil, fmt.Errorf("%w: parsing created date %q: %v", ErrMalformedTask, createdAt.GetValue(), err)
		}
		t.CreatedOn = date
	}

	var nodes []parsec.Queryable
	if node := parser.QueryOne("WORDS"); node != nil { // nil: a preamble-only line
		nodes = node.GetChildren()
	}
	t.Annotations = make(map[string]string)
	var entry []string
	for _, n := range nodes {
		if n.GetName() != "KVPAIR" {
			t.body = append(t.body, bodyWord{text: n.GetValue()})
			entry = append(entry, n.GetValue())
			continue
		}
		k, v, err := KVPAIR2String(n)
		if err != nil {
			return nil, fmt.Errorf("%w: extracting annotation: %v", ErrMalformedTask, err)
		}
		t.body = append(t.body, bodyWord{text: k + ":" + v, key: k})
		t.Annotations[k] = v
	}
	t.entry = strings.Join(entry, " ")

	t.Tags = map[string][]string{
		"@": pu.MapNodes(parser.Query("ATTAG"), pu.NodeToValue),
		"+": pu.MapNodes(parser.Query("PLUSTAG"), pu.NodeToValue),
		"#": pu.MapNodes(parser.Query("HASHTAG"), pu.NodeToValue),
	}

	return t, nil
}

// rebuild serialises the task back into its todo.txt Raw form: the
// preamble from the task's fields, then the rest of the line word for
// word, in the order it was written (see reconcileAnnotations). Words
// are separated by single spaces.
//
// Field order matches what the parser grammar in parser.go accepts:
//   [x] [(P)] [<completedOn>] [<createdOn>] body
//
// Notably priority comes BEFORE the dates, not after — that's the
// todo.txt convention and what NewTask round-trips. A previous
// version of this function put the priority after the dates, which
// produced strings the parser couldn't parse back: a stamped
// "<today> (A) task" lost its priority on the next reload because
// the parser saw "(A)" inside WORDS, not at the priority slot.
func (t *Task) rebuild() *Task {
	b := bytes.Buffer{}

	if t.Completed {
		b.WriteString("x ")
	}
	if t.Priority != "" {
		fmt.Fprintf(&b, "(%s) ", t.Priority)
	}
	if !t.CompletedOn.Equal(time.Time{}) {
		b.WriteString(t.CompletedOn.Format("2006-01-02 "))
	}
	if !t.CreatedOn.Equal(time.Time{}) {
		b.WriteString(t.CreatedOn.Format("2006-01-02 "))
	}
	t.body = t.reconcileAnnotations()
	for _, w := range t.body {
		b.WriteString(w.text + " ")
	}

	// Each part above ends in one space. TrimSpace would also eat
	// trailing Unicode spaces (U+00A0) that belong to the last word.
	t.Raw = strings.TrimSuffix(b.String(), " ")
	return t
}

// reconcileAnnotations brings the body's annotations in line with
// t.Annotations without moving anything. A key's last occurrence, the
// one Annotations shows, takes the key's current value where it
// stands; earlier occurrences of a key still present stay as written;
// every occurrence of a removed key goes; and keys the line doesn't
// have yet are added at the end, in key order.
func (t *Task) reconcileAnnotations() []bodyWord {
	last := map[string]int{}
	for i, w := range t.body {
		if w.key != "" {
			last[w.key] = i
		}
	}
	out := make([]bodyWord, 0, len(t.body)+len(t.Annotations))
	for i, w := range t.body {
		if w.key == "" {
			out = append(out, w)
			continue
		}
		value, kept := t.Annotations[w.key]
		switch {
		case !kept:
		case last[w.key] == i:
			out = append(out, bodyWord{text: w.key + ":" + value, key: w.key})
		default:
			out = append(out, w)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(t.Annotations)) {
		if _, inLine := last[k]; !inLine {
			out = append(out, bodyWord{text: k + ":" + t.Annotations[k], key: k})
		}
	}
	return out
}

// setPriority assigns p as the task's priority. Unknown letters are
// silently coerced to the lowest priority (empty string) to preserve
// legacy behavior — this is asserted in task_test.go and exploited by
// increasePriority / decreasePriority's "unknown" branches.
//
// Unexported: external callers should mutate priority through
// Context.SetPriority / IncreasePriority / DecreasePriority, which
// persist via the injected repo. Going around them produces a mutated
// Task that never reaches disk.
func (t *Task) setPriority(p string) *Task {
	if _, err := indexOf(priorities, p); err != nil {
		p = priorities[len(priorities)-1]
	}
	t.Priority = p
	return t.rebuild()
}

func (t *Task) increasePriority() *Task {
	i, err := indexOf(priorities, t.Priority)
	if err != nil {
		return t.setPriority(priorities[0])
	}
	if i == 0 {
		return t
	}
	return t.setPriority(priorities[i-1])
}

func (t *Task) decreasePriority() *Task {
	i, err := indexOf(priorities, t.Priority)
	if err != nil {
		return t.setPriority(priorities[len(priorities)-1])
	}
	if i == len(priorities)-1 {
		return t
	}
	return t.setPriority(priorities[i+1])
}

// indexOf replaces utils.StringSliceIndex so the domain doesn't pull in
// utils for one helper. Same contract.
func indexOf(slice []string, item string) (int, error) {
	for i, s := range slice {
		if s == item {
			return i, nil
		}
	}
	return 0, fmt.Errorf("not in slice: %q", item)
}

func (t *Task) RawString() string {
	return t.Raw
}

func (t *Task) ConsoleString() string {
	if vars.GetBool(vars.ConfigKeyDisableColor) {
		return t.Raw
	}
	return t.ColorString()
}

// markCompleted marks the task complete at the given time and annotates
// it with the originating context. Returns an error if the annotation
// fails (which shouldn't happen for the static "context" key, but
// propagated so the signature stays honest).
//
// Unexported: external callers should complete a task through
// Context.Complete, which persists via the injected repo.
func (t *Task) markCompleted(context *Context, when time.Time) (*Task, error) {
	t.Completed = true
	t.CompletedOn = when
	if _, err := t.Annotate("context", context.Name); err != nil {
		return nil, fmt.Errorf("annotate completed task: %w", err)
	}
	t.rebuild()
	return t, nil
}

// markDeleted stamps two annotations on a task being archived:
//   - archived-from:<source.Name> — the context the task was deleted from
//   - deleted-on:<YYYY-MM-DD>      — the date of deletion
//
// A distinct "archived-from" key (rather than reusing the completion-
// time "context" annotation) avoids overwriting completion history
// when a done.txt entry is deleted — the task keeps both context:work
// (where it was completed) and archived-from:done (where it was
// deleted). Symmetric with markCompleted; unexported for the same
// reason.
func (t *Task) markDeleted(source *Context, when time.Time) (*Task, error) {
	if _, err := t.Annotate("archived-from", source.Name); err != nil {
		return nil, fmt.Errorf("annotate archived-from: %w", err)
	}
	if _, err := t.Annotate("deleted-on", when.Format("2006-01-02")); err != nil {
		return nil, fmt.Errorf("annotate deleted-on: %w", err)
	}
	return t, nil
}

// annotateKeyRE and annotateValueRE are the anchored validation forms of
// the parser's tokenization regexes. The parser uses the unanchored
// versions to tokenize from a cursor position; for whole-string input
// validation in Annotate we need ^...$.
var (
	annotateKeyRE   = regexp.MustCompile(`^(?:` + kvpairKeyRegexp + `)$`)
	annotateValueRE = regexp.MustCompile(`^(?:` + kvpairValueRegexp + `)$`)
)

// Annotate adds a key:value annotation to the task. Returns
// ErrInvalidAnnotationKey or ErrInvalidAnnotationValue if either fails
// the todo.txt key/value syntax check, and ErrInvalidAnnotationKey for
// a pair the parser reads as a word (a time or a link; see
// isAnnotation), which would not read back as an annotation.
func (t *Task) Annotate(key, value string) (*Task, error) {
	if !annotateKeyRE.MatchString(key) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidAnnotationKey, key)
	}
	if !annotateValueRE.MatchString(value) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidAnnotationValue, value)
	}
	if !isAnnotation(key, value) {
		return nil, fmt.Errorf("%w: %s:%s reads as text (a time or a link), not an annotation", ErrInvalidAnnotationKey, key, value)
	}
	t.Annotations[key] = value
	t.rebuild()
	return t, nil
}

// StampCreated gives a task with no created-on date the given day,
// rewriting its line, and leaves a dated task alone. Context.AddTask
// stamps every new task this way; the TUI calls it to preview the line
// a new task will be saved as.
func (t *Task) StampCreated(day time.Time) *Task {
	if t.CreatedOn.IsZero() {
		t.CreatedOn = day
		t.rebuild()
	}
	return t
}

func (t *Task) RemoveAnnotation(key string) *Task {
	delete(t.Annotations, key)
	t.rebuild()
	return t
}

func (t *Task) Entry() string {
	return t.entry
}

type lineColorFn func(a string, args ...interface{}) string

func (t *Task) ColorFn() lineColorFn {
	switch t.Priority {
	case "A":
		return color.New(color.FgRed).SprintfFunc()
	case "B":
		return color.New(color.FgYellow).SprintfFunc()
	case "C":
		return color.New(color.FgGreen).SprintfFunc()
	default:
		return color.New(color.FgWhite).SprintfFunc()
	}
}

func (t *Task) ColoredEntry() string {
	return t.ColorFn()(t.entry)
}

func (t *Task) ColorString() string {
	b := bytes.NewBuffer([]byte{})
	b.WriteString(t.ColorFn()(t.Raw))
	return b.String()
}
