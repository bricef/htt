package todo

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"time"

	pu "github.com/hypotheticalco/tracker-client/parseutils"
	"github.com/hypotheticalco/tracker-client/utils"
	parsec "github.com/prataprc/goparsec"
)

// Task represents a task in a todo list.
// for tasks that is stable even though they might be filtered.
type Task struct {
	raw         string // This is the entire line, including all parsed items, as it would appear in the file
	Line        int
	Completed   bool
	Priority    string
	CompletedAt time.Time
	CreatedAt   time.Time
	entry       string              // This is the task without priority, annotations, completion mark or created/completed dates
	Tags        map[string][]string // { "@": ["abc", "def"], "#": ["foo"]}
	Annotations map[string]string
}

var (
	priorities = []string{"A", "B", "C", "D", "E", "F"}
	parser     = NewTodoParser()
)

// Creates a new task from a raw string
func NewTask(raw string) *Task {
	if raw == "" {
		utils.Fatal("Can't create empty task")
	}
	t := &Task{}

	t.raw = raw

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
		t.CompletedAt = date
	}

	// CreatedAt   time.Time
	createdAt := parser.QueryOne("CREATEDAT")
	if createdAt != nil {
		date, err := time.Parse("2006-01-02", createdAt.GetValue())
		utils.DieOnError("Failed to parse date after parsing. Something is seriously wrong. ", err)
		t.CreatedAt = date
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

	if t.Priority != "" {
		b.WriteString(fmt.Sprintf("(%s) ", t.Priority))
	}

	if !t.CompletedAt.Equal(time.Time{}) {
		b.WriteString(t.CompletedAt.Format("2006-01-02 "))
	}

	if !t.CreatedAt.Equal(time.Time{}) {
		b.WriteString(t.CreatedAt.Format("2006-01-02 "))
	}

	if t.entry != "" {
		b.WriteString(fmt.Sprintf("%s ", t.entry))
	}

	// Annotations map[string]string
	for k, v := range t.Annotations {
		b.WriteString(fmt.Sprintf("%s:%s ", k, v))
	}

	t.raw = strings.TrimSpace(b.String())
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

func (t *Task) ToString() string {
	return t.raw
}

func (t *Task) Do(context string, when time.Time) *Task {
	t.CompletedAt = when
	t.Annotate("context", context)
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
