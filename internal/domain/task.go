package domain

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
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
	Annotations map[string]string
	parser      *pu.Parser
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
	parser.Parse(raw)

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

	words := pu.Select(parser.QueryOne("WORDS").GetChildren(), func(n parsec.Queryable) bool {
		return n.GetName() != "KVPAIR"
	})
	t.entry = strings.Join(pu.MapNodes(words, pu.NodeToValue), " ")

	t.Tags = map[string][]string{
		"@": pu.MapNodes(parser.Query("ATTAG"), pu.NodeToValue),
		"+": pu.MapNodes(parser.Query("PLUSTAG"), pu.NodeToValue),
		"#": pu.MapNodes(parser.Query("HASHTAG"), pu.NodeToValue),
	}

	t.Annotations = make(map[string]string)
	for _, n := range parser.Query("KVPAIR") {
		k, v, err := KVPAIR2String(n)
		if err != nil {
			return nil, fmt.Errorf("%w: extracting annotation: %v", ErrMalformedTask, err)
		}
		t.Annotations[k] = v
	}

	return t, nil
}

func (t *Task) rebuild() *Task {
	b := bytes.Buffer{}

	if t.Completed {
		b.WriteString("x ")
	}
	if !t.CompletedOn.Equal(time.Time{}) {
		b.WriteString(t.CompletedOn.Format("2006-01-02 "))
	}
	if !t.CreatedOn.Equal(time.Time{}) {
		b.WriteString(t.CreatedOn.Format("2006-01-02 "))
	}
	if t.Priority != "" {
		fmt.Fprintf(&b, "(%s) ", t.Priority)
	}
	if t.entry != "" {
		fmt.Fprintf(&b, "%s ", t.entry)
	}
	for k, v := range t.Annotations {
		fmt.Fprintf(&b, "%s:%s ", k, v)
	}

	t.Raw = strings.TrimSpace(b.String())
	return t
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
// the todo.txt key/value syntax check.
func (t *Task) Annotate(key, value string) (*Task, error) {
	if !annotateKeyRE.MatchString(key) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidAnnotationKey, key)
	}
	if !annotateValueRE.MatchString(value) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidAnnotationValue, value)
	}
	t.Annotations[key] = value
	t.rebuild()
	return t, nil
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
