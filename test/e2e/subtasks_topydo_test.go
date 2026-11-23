package e2e

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// topydo (github.com/topydo/topydo, lib/TodoList.py) links tasks the
// same way: a parent carries id:N, a subtask p:N, N the smallest number
// no task in the file uses as an id: or a p: (find_next_id). These
// tests read the file with topydo's rule, written here independently
// of htt's parser, and compare.

var (
	topydoID     = regexp.MustCompile(`(?:^|\s)id:(\S+)`)
	topydoParent = regexp.MustCompile(`(?:^|\s)p:(\S+)`)
)

// topydoTree reads lines as topydo does: a task's id is its first id:
// (tag_value), and a task with p:N is a subtask of the task whose id is
// N. It returns each task's parents, as text, by task text.
func topydoTree(t *testing.T, content string) map[string][]string {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	byID := map[string]string{}
	for _, l := range lines {
		if m := topydoID.FindStringSubmatch(l); m != nil {
			byID[m[1]] = l
		}
	}
	out := map[string][]string{}
	for _, l := range lines {
		out[l] = nil
		for _, m := range topydoParent.FindAllStringSubmatch(l, -1) {
			if parent, ok := byID[m[1]]; ok {
				out[l] = append(out[l], parent)
			}
		}
	}
	return out
}

// topydoNextID is topydo's find_next_id over the file.
func topydoNextID(content string) string {
	used := map[string]bool{}
	for _, re := range []*regexp.Regexp{topydoID, topydoParent} {
		for _, m := range re.FindAllStringSubmatch(content, -1) {
			used[m[1]] = true
		}
	}
	n := 1
	for used[strconv.Itoa(n)] {
		n++
	}
	return strconv.Itoa(n)
}

// htt's tree, as `htt todo show` indents it: each task's parent, by
// task text.
func httTree(t *testing.T, e *env) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	var stack []string // the last task shown at each depth
	for _, row := range strings.Split(e.mustRun("todo", "show").stdout, "\n") {
		// "  3     Call three venues p:2": the number, then two spaces a level.
		m := regexp.MustCompile(`^\s*\d+ ( *)(.*)$`).FindStringSubmatch(row)
		if m == nil {
			continue
		}
		depth, text := len(m[1])/2, strings.TrimSuffix(m[2], "  (parent done)")
		stack = append(stack[:depth], text)
		out[text] = nil
		if depth > 0 {
			out[text] = []string{stack[depth-1]}
		}
	}
	return out
}

func sameTree(a, b map[string][]string) bool {
	return maps.EqualFunc(a, b, func(x, y []string) bool { return slices.Equal(x, y) })
}

// A file as topydo writes it, several levels deep, its tags at the end
// and subtasks wherever they were added (before their parent too),
// reads as the tree topydo reads.
func TestSubtasks_ATopydoFileReadsAsTopydoReadsIt(t *testing.T) {
	e := newEnv(t)
	topydo := "(A) 2026-11-20 Plan the launch +work id:1\n" +
		"2026-11-20 Ring Ann p:3\n" +
		"2026-11-20 Pack the van\n" +
		"2026-11-20 Write the announcement +work p:1\n" +
		"2026-11-21 Book the venue p:1 id:2\n" +
		"2026-11-21 Call three venues p:2 id:3\n" +
		"(B) 2026-11-22 Order the cake p:1\n" +
		"x 2026-11-22 2026-11-21 Email the caterer p:2\n"
	e.writeData("todo.txt", topydo)

	want := topydoTree(t, topydo)
	if got := httTree(t, e); !sameTree(got, want) {
		t.Errorf("htt reads the tree as\n%v\ntopydo as\n%v", got, want)
	}
	assertEqual(t, "reading changed the file", e.readData("todo.txt"), topydo)

	// htt's writes keep it a file topydo reads as htt does, its ids
	// given by topydo's rule.
	next := topydoNextID(e.readData("todo.txt"))
	e.mustRun("todo", "add", "--under", "2", "Draft the post") // Write the announcement, after (B) Order the cake, gets the next id
	if !strings.Contains(e.readData("todo.txt"), "Write the announcement +work p:1 id:"+next) {
		t.Errorf("the new id isn't topydo's next, %s:\n%s", next, e.readData("todo.txt"))
	}
	for _, args := range [][]string{
		{"todo", "nest", "8", "0"},     // Pack the van under Plan
		{"todo", "unnest", "5"},        // Call three venues, with Ring Ann
		{"todo", "priority", "4", "A"}, // a subtask among its siblings
		{"todo", "add", "--under", "0", "Book a photographer"},
		{"todo", "move", "4", "work"}, // Write the announcement, with Draft the post
	} {
		e.mustRun(args...)
		content := e.readData("todo.txt")
		if got, want := httTree(t, e), topydoTree(t, content); !sameTree(got, want) {
			t.Errorf("after htt %s: htt reads the tree as\n%v\ntopydo as\n%v\nfile:\n%s", strings.Join(args, " "), got, want, content)
		}
	}
	e.mustRun("todo", "context", "work")
	if got, want := httTree(t, e), topydoTree(t, e.readData("work.txt")); !sameTree(got, want) {
		t.Errorf("work: htt reads the tree as\n%v\ntopydo as\n%v", got, want)
	}
}
