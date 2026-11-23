package domain

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// tasksOf parses each line as a task.
func tasksOf(t *testing.T, lines ...string) []*Task {
	t.Helper()
	out := make([]*Task, len(lines))
	for i, l := range lines {
		out[i] = mustTask(t, l)
	}
	return out
}

// outline draws a tree as an indented list of its lines, two spaces a
// level, done parents marked, in tree order.
func outline(tree *Tree) string {
	var b strings.Builder
	for _, n := range tree.Order() {
		mark := ""
		if n.DoneParent {
			mark = " [done parent]"
		}
		fmt.Fprintf(&b, "%s%s%s\n", strings.Repeat("  ", n.Depth), n.Task.Raw, mark)
	}
	return b.String()
}

// problems lists a tree's link problems as "index: message".
func problems(tree *Tree) []string {
	var out []string
	for _, p := range tree.Problems {
		out = append(out, fmt.Sprintf("%d: %s", p.Index, p.Message))
	}
	return out
}

func TestNewTree(t *testing.T) {
	cases := []struct {
		name     string
		lines    []string
		done     []string // ids of the context's done tasks
		want     string
		problems []string
	}{
		{
			name: "the design's example",
			lines: []string{
				"(A) Plan the launch +work id:1",
				"Write the announcement p:1 +work",
				"Book the venue p:1 id:2",
				"Call three venues p:2",
			},
			want: "(A) Plan the launch +work id:1\n" +
				"  Write the announcement p:1 +work\n" +
				"  Book the venue p:1 id:2\n" +
				"    Call three venues p:2\n",
		},
		{
			name:  "no subtasks: the list as it is",
			lines: []string{"b", "(A) a", "c"},
			want:  "b\n(A) a\nc\n",
		},
		{
			name: "subtasks before their parent, as topydo can leave them",
			lines: []string{
				"Call three venues p:2",
				"Write the announcement p:1",
				"Something else",
				"Book the venue p:1 id:2",
				"Plan the launch id:1",
			},
			want: "Something else\n" +
				"Plan the launch id:1\n" +
				"  Write the announcement p:1\n" +
				"  Book the venue p:1 id:2\n" +
				"    Call three venues p:2\n",
		},
		{
			name: "subtasks in priority order, ties as they were; the top level unsorted",
			lines: []string{
				"(C) top c",
				"parent id:1",
				"none one p:1",
				"(B) b one p:1",
				"(A) a p:1",
				"none two p:1",
				"(B) b two p:1",
				"(Z) z p:1",
				"(A) top a",
			},
			want: "(C) top c\n" +
				"parent id:1\n" +
				"  (A) a p:1\n" +
				"  (B) b one p:1\n" +
				"  (B) b two p:1\n" +
				"  (Z) z p:1\n" +
				"  none one p:1\n" +
				"  none two p:1\n" +
				"(A) top a\n",
		},
		{
			name: "a done task in the file is in the tree like any other",
			lines: []string{
				"parent id:1",
				"x (A) 2026-01-02 done child p:1",
				"(B) open child p:1",
			},
			want: "parent id:1\n" +
				"  x (A) 2026-01-02 done child p:1\n" +
				"  (B) open child p:1\n",
		},
		{
			name:     "a p: naming no task",
			lines:    []string{"parent id:1", "lost p:7", "kept p:1"},
			want:     "parent id:1\n  kept p:1\nlost p:7\n",
			problems: []string{"1: p:7 names no task, so the task is shown at the top level"},
		},
		{
			name:  "a p: naming a task in the done file: a done parent, no problem",
			lines: []string{"orphan p:3", "other"},
			done:  []string{"3"},
			want:  "orphan p:3 [done parent]\nother\n",
		},
		{
			name:  "a task in the file wins over a done one with the same id",
			lines: []string{"child p:3", "parent id:3"},
			done:  []string{"3"},
			want:  "parent id:3\n  child p:3\n",
		},
		{
			name: "a loop: every task in it at the top level, its subtasks kept",
			lines: []string{
				"a id:1 p:2",
				"b id:2 p:1",
				"under a p:1",
			},
			want: "a id:1 p:2\n  under a p:1\nb id:2 p:1\n",
			problems: []string{
				"0: p:2 makes a loop (the task would be below itself), so it is shown at the top level",
				"1: p:1 makes a loop (the task would be below itself), so it is shown at the top level",
			},
		},
		{
			name:  "a task naming itself is a loop",
			lines: []string{"me id:4 p:4"},
			want:  "me id:4 p:4\n",
			problems: []string{
				"0: p:4 makes a loop (the task would be below itself), so it is shown at the top level",
			},
		},
		{
			name: "a longer loop, with a tail leading into it",
			lines: []string{
				"tail p:3",
				"a id:1 p:3",
				"b id:2 p:1",
				"c id:3 p:2",
			},
			want: "a id:1 p:3\nb id:2 p:1\nc id:3 p:2\n  tail p:3\n",
			problems: []string{
				"1: p:3 makes a loop (the task would be below itself), so it is shown at the top level",
				"2: p:1 makes a loop (the task would be below itself), so it is shown at the top level",
				"3: p:2 makes a loop (the task would be below itself), so it is shown at the top level",
			},
		},
		{
			name:  "two tasks with the same id: the subtasks naming it at the top level",
			lines: []string{"one id:1", "two id:1 p:5", "child p:1", "five id:5"},
			want:  "one id:1\nchild p:1\nfive id:5\n  two id:1 p:5\n",
			problems: []string{
				"0: id:1 is on more than one task, so its subtasks are shown at the top level",
				"1: id:1 is on more than one task, so its subtasks are shown at the top level",
			},
		},
		{
			name:  "several p: on a task: under the first",
			lines: []string{"one id:1", "two id:2", "child p:2 p:1"},
			want:  "one id:1\ntwo id:2\n  child p:2 p:1\n",
			problems: []string{
				"2: has more than one p: (p:2 p:1); htt reads only the first",
			},
		},
		{
			name:  "several p:, the first naming nothing: at the top level",
			lines: []string{"one id:1", "child p:9 p:1"},
			want:  "one id:1\nchild p:9 p:1\n",
			problems: []string{
				"1: p:9 names no task, so the task is shown at the top level",
				"1: has more than one p: (p:9 p:1); htt reads only the first",
			},
		},
		{
			name:  "ids are text, matched exactly, as topydo matches them",
			lines: []string{"word id:abc", "under p:abc", "zero p:01", "one id:1"},
			want:  "word id:abc\n  under p:abc\nzero p:01\none id:1\n",
			problems: []string{
				"2: p:01 names no task, so the task is shown at the top level",
			},
		},
		{
			name:     "a task's id is its first id:",
			lines:    []string{"two ids id:1 id:2", "a p:1", "b p:2"},
			want:     "two ids id:1 id:2\n  a p:1\nb p:2\n",
			problems: []string{"2: p:2 names no task, so the task is shown at the top level"},
		},
		{
			name:  "tags anywhere in the line",
			lines: []string{"id:1 parent first", "(A) 2026-01-01 p:1 child +proj"},
			want:  "id:1 parent first\n  (A) 2026-01-01 p:1 child +proj\n",
		},
		{
			name:  "any annotation value is an id",
			lines: []string{"meet p:10:30", "see id:https://x.io"},
			want:  "meet p:10:30\nsee id:https://x.io\n",
			problems: []string{
				"0: p:10:30 names no task, so the task is shown at the top level",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			done := map[string]bool{}
			for _, id := range tc.done {
				done[id] = true
			}
			tree := NewTree(tasksOf(t, tc.lines...), done)
			if got := outline(tree); got != tc.want {
				t.Errorf("tree:\n%s\nwant:\n%s", got, tc.want)
			}
			if got := problems(tree); !slices.Equal(got, tc.problems) {
				t.Errorf("problems:\n  %q\nwant:\n  %q", got, tc.problems)
			}
		})
	}
}

func TestNewTree_NodesKnowTheirPlace(t *testing.T) {
	tree := NewTree(tasksOf(t,
		"Call p:2",
		"Plan id:1",
		"Book p:1 id:2",
	), nil)
	if len(tree.Nodes) != 3 {
		t.Fatalf("Nodes: %d, want 3", len(tree.Nodes))
	}
	for i, n := range tree.Nodes {
		if n.Index != i {
			t.Errorf("Nodes[%d].Index = %d", i, n.Index)
		}
	}
	call, plan, book := tree.Nodes[0], tree.Nodes[1], tree.Nodes[2]
	if call.Parent != book || book.Parent != plan || plan.Parent != nil {
		t.Errorf("parents: call→%v book→%v plan→%v", call.Parent, book.Parent, plan.Parent)
	}
	if call.Depth != 2 || book.Depth != 1 || plan.Depth != 0 {
		t.Errorf("depths: %d %d %d", call.Depth, book.Depth, plan.Depth)
	}
	if got := len(plan.Subtree()); got != 3 {
		t.Errorf("plan's subtree: %d nodes, want 3", got)
	}
	if got := len(call.Subtree()); got != 1 {
		t.Errorf("call's subtree: %d nodes, want 1", got)
	}
	if !slices.Equal(tree.Roots, []*Node{plan}) {
		t.Errorf("roots: %v", tree.Roots)
	}
}

func TestNewTree_FlagsLinksTheFileCantResolve(t *testing.T) {
	if NewTree(tasksOf(t, "a id:1", "b p:1"), nil).Unresolved {
		t.Error("every p: names a task in the file: nothing to look up")
	}
	if !NewTree(tasksOf(t, "a id:1", "b p:2"), nil).Unresolved {
		t.Error("p:2 names no task in the file: the done file may hold it")
	}
	if NewTree(tasksOf(t, "a id:1", "b id:1", "c p:1"), nil).Unresolved {
		t.Error("a duplicated id is in the file: the done file can't settle it")
	}
}

func TestTask_IDAndParentIDs(t *testing.T) {
	task := mustTask(t, "x (A) a id:3 p:1 b id:4 p:2 p:1")
	if got := task.ID(); got != "3" {
		t.Errorf("ID() = %q, want 3", got)
	}
	if got := task.ParentIDs(); !slices.Equal(got, []string{"1", "2", "1"}) {
		t.Errorf("ParentIDs() = %q", got)
	}
	plain := mustTask(t, "nothing here p: id: 10:30")
	if plain.ID() != "" || len(plain.ParentIDs()) != 0 {
		t.Errorf("no tags: ID %q, ParentIDs %q", plain.ID(), plain.ParentIDs())
	}
}

func TestNextID(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"an empty file", nil, "1"},
		{"no ids", []string{"a", "b"}, "1"},
		{"after id: and p:", []string{"a id:1", "b p:2"}, "3"},
		{"the smallest gap", []string{"a id:1", "b id:3", "c p:4"}, "2"},
		{"a p: alone holds its number", []string{"orphan p:1"}, "2"},
		{"every occurrence counts", []string{"a id:5 id:1", "b p:3 p:2"}, "4"},
		{"text isn't a number", []string{"a id:abc", "b id:01"}, "1"},
		{"done tasks in the file count", []string{"x a id:1"}, "2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NextID(tasksOf(t, tc.lines...)); got != tc.want {
				t.Errorf("NextID = %q, want %q", got, tc.want)
			}
		})
	}
}
