package todo

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"time"

	pu "github.com/bricef/htt/internal/parseutils"
	"github.com/bricef/htt/internal/utils"
	"github.com/bricef/htt/internal/vars"
	"github.com/fatih/color"
	parsec "github.com/prataprc/goparsec"
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
	priorities = []string{"A", "B", "C", "D", "E", "F"}
)

// Creates a new task from a raw string
func NewTask(raw string) *Task {
	if raw == "" {
		utils.Fatal("Can't create empty task")
	}
	t := &Task{}

	t.Raw = raw

	parser := NewTodoParser()
	t.parser = parser

	parser.Parse(raw)

	// Completed   bool
	completed := parser.QueryOne("COMPLETED")
	if completed != nil {
		t.Completed = true
	}

	// Priority    string
	priority := parser.QueryOne("PRIORITY")
	if priority != nil {
		t.Priority = priority.GetValue()
	}

	// CompletedAt time.Time
	completedAt := parser.QueryOne("COMPLETEDAT")
	if completedAt != nil {
		date, err := time.Parse("2006-01-02", completedAt.GetValue())
		utils.DieOnError("Failed to parse date after parsing. Something is seriously wrong. ", err)
		t.CompletedOn = date
	}

	// CreatedAt   time.Time
	createdAt := parser.QueryOne("CREATEDAT")
	if createdAt != nil {
		date, err := time.Parse("2006-01-02", createdAt.GetValue())
		utils.DieOnError("Failed to parse date after parsing. Something is seriously wrong. ", err)
		t.CreatedOn = date
	}

	// entry       string // This is the task without priority, annotations, completion mark or created/completed dates
	words := pu.Select(parser.QueryOne("WORDS").GetChildren(), func(n parsec.Queryable) bool {
		if n.GetName() != "KVPAIR" { //exclude kvpairs
			return true
		}
		return false
	})
	t.entry = strings.Join(pu.MapNodes(words, pu.NodeToValue), " ")

	// map[string][]string // { "@": ["abc", "def"], "#": ["foo"]}
	t.Tags = map[string][]string{
		"@": pu.MapNodes(parser.Query("ATTAG"), pu.NodeToValue),
		"+": pu.MapNodes(parser.Query("PLUSTAG"), pu.NodeToValue),
		"#": pu.MapNodes(parser.Query("HASHTAG"), pu.NodeToValue),
	}

	// Annotations map[string]string
	t.Annotations = make(map[string]string)
	annotations := parser.Query("KVPAIR")
	for _, n := range annotations {
		k, v, err := KVPAIR2String(n)
		utils.DieOnError("We failed to convert a KVPAIR to its key and value. Something very wrong happend. ", err)
		t.Annotations[k] = v
	}

	return t

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
		b.WriteString(fmt.Sprintf("(%s) ", t.Priority))
	}

	if t.entry != "" {
		b.WriteString(fmt.Sprintf("%s ", t.entry))
	}

	// Annotations map[string]string
	for k, v := range t.Annotations {
		b.WriteString(fmt.Sprintf("%s:%s ", k, v))
	}

	t.Raw = strings.TrimSpace(b.String())
	return t
}

func (t *Task) SetPriority(p string) *Task {
	// validate the priority
	_, err := utils.StringSliceIndex(priorities, p)
	if err != nil { // bad priority
		p = priorities[len(priorities)-1] //assume lowest
	}

	// set it
	t.Priority = p

	// rebuild the raw string
	return t.rebuild()

}

func (t *Task) IncreasePriority() *Task {
	i, err := utils.StringSliceIndex(priorities, t.Priority)
	if err != nil { // couldn't find it
		return t.SetPriority(priorities[0])
	}
	if i == 0 { // already max
		return t
	}
	return t.SetPriority(priorities[i-1])
}

func (t *Task) DecreasePriority() *Task {
	i, err := utils.StringSliceIndex(priorities, t.Priority)
	if err != nil { // couldn't find it
		return t.SetPriority(priorities[len(priorities)-1])
	}
	if i == len(priorities)-1 { // already min
		return t
	}
	return t.SetPriority(priorities[i+1])
}

func (t *Task) ConsoleString() string {
	if vars.GetBool(vars.ConfigKeyDisableColor) {
		return t.Raw
	} else {
		return t.ColorString()
	}
}

func (t *Task) Do(context *Context, when time.Time) *Task {
	t.Completed = true
	t.CompletedOn = when
	t.Annotate("context", context.Name)
	t.rebuild()
	return t
}

func (t *Task) Annotate(key string, value string) *Task {
	keyMatches, err := regexp.MatchString(kvpairKeyRegexp, key)
	utils.DieOnError("Invalid annotation key. ", err)

	valueMatches, err := regexp.MatchString(kvpairValueRegexp, key)
	utils.DieOnError("Invalid annotation value. ", err)

	if !keyMatches || !valueMatches {
		utils.Fatal("Invalid syntax for annotation. Key must not start with #@+ and key and values cannot include '`' or ':' ")
	}

	t.Annotations[key] = value
	t.rebuild()
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
	var lineColor lineColorFn
	switch t.Priority {
	case "A":
		lineColor = color.New(color.FgRed).SprintfFunc()
	case "B":
		lineColor = color.New(color.FgYellow).SprintfFunc()
	case "C":
		lineColor = color.New(color.FgGreen).SprintfFunc()
	default:
		lineColor = color.New(color.FgWhite).SprintfFunc()
	}

	return lineColor
}

func (t *Task) ColoredEntry() string {
	return t.ColorFn()(t.entry)
}

func (t *Task) ColorString() string { // doesn't feel like it should belong here, but nvm
	// raw         string
	// Line        int
	// Completed   bool
	// Priority    string
	// CompletedAt time.Time
	// CreatedAt   time.Time
	// entry       string
	// Tags        map[string][]string
	// Annotations map[string]string

	b := bytes.NewBuffer([]byte{})
	b.WriteString(t.ColorFn()(t.Raw))

	return b.String()
}
