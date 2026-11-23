package domain

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// randomIDValue is an id: or p: value: mostly small numbers, so links,
// loops and duplicates are common, sometimes text topydo would match
// too, and a leading zero, which names a different id.
func randomIDValue(r *rand.Rand) string {
	switch r.IntN(12) {
	case 0:
		return "abc"
	case 1:
		return "01"
	default:
		return strconv.Itoa(1 + r.IntN(10))
	}
}

// randomTreeLine is a line as people and tools leave them: a preamble
// htt rewrites (done mark, priority, dates) or none, odd words, and
// id: and p: tags anywhere in it, none, one or several of each.
func randomTreeLine(r *rand.Rand) string {
	var pre []string
	if r.IntN(5) == 0 {
		pre = append(pre, "x")
	}
	if r.IntN(2) == 0 {
		pre = append(pre, []string{"(A)", "(B)", "(C)", "(D)"}[r.IntN(4)])
	}
	if r.IntN(3) == 0 {
		pre = append(pre, "2026-01-0"+strconv.Itoa(1+r.IntN(9)))
	}
	body := []string{"task" + strconv.Itoa(r.IntN(100))}
	for range r.IntN(4) {
		body = append(body, wordShapes[r.IntN(len(wordShapes))])
	}
	insert := func(word string) {
		at := r.IntN(len(body) + 1)
		body = slices.Insert(body, at, word)
	}
	for range []int{0, 0, 0, 0, 1, 1, 1, 1, 1, 2}[r.IntN(10)] {
		insert("id:" + randomIDValue(r))
	}
	switch r.IntN(10) {
	case 0, 1, 2, 3:
	case 9:
		insert("p:" + randomIDValue(r))
		insert("p:" + randomIDValue(r))
	default:
		insert("p:" + randomIDValue(r))
	}
	return strings.TrimSpace(strings.Join(append(pre, body...), " "))
}

// randomTreeTasks is a generated context file of up to max tasks.
func randomTreeTasks(t *testing.T, r *rand.Rand, max int) []*Task {
	t.Helper()
	tasks := make([]*Task, r.IntN(max+1))
	for i := range tasks {
		tasks[i] = mustTask(t, randomTreeLine(r))
	}
	return tasks
}

// randomDoneIDs is a set of ids a done file might hold.
func randomDoneIDs(r *rand.Rand) map[string]bool {
	ids := map[string]bool{}
	for range r.IntN(3) {
		ids[randomIDValue(r)] = true
	}
	return ids
}

// holders returns the indexes of the tasks whose id is id.
func holders(tasks []*Task, id string) []int {
	var out []int
	for i, t := range tasks {
		if t.ID() == id {
			out = append(out, i)
		}
	}
	return out
}

// loopsBack reports whether following first p: links, each to the one
// task holding the id, leads from task i back to i.
func loopsBack(tasks []*Task, i int) bool {
	seen := map[int]bool{}
	for j := i; ; {
		ps := tasks[j].ParentIDs()
		if len(ps) == 0 {
			return false
		}
		h := holders(tasks, ps[0])
		if len(h) != 1 {
			return false
		}
		j = h[0]
		if j == i {
			return true
		}
		if seen[j] {
			return false // a loop further up, not through i
		}
		seen[j] = true
	}
}

func hasProblem(tree *Tree, i int, prefix string) bool {
	return slices.ContainsFunc(tree.Problems, func(p LinkProblem) bool {
		return p.Index == i && strings.HasPrefix(p.Message, prefix)
	})
}

func TestNewTree_Properties(t *testing.T) {
	r := rand.New(rand.NewPCG(23, 11))
	for range 600 {
		tasks := randomTreeTasks(t, r, 30)
		done := randomDoneIDs(r)
		tree := NewTree(tasks, done)
		lines := make([]string, len(tasks))
		for i, task := range tasks {
			lines[i] = task.Raw
		}
		fail := func(format string, args ...any) {
			t.Helper()
			t.Fatalf("%s\nfile:\n  %s\ndone ids: %v\ntree:\n%s", strings.TrimSpace(fmt.Sprintf(format, args...)), strings.Join(lines, "\n  "), done, outline(tree))
		}

		// Every task once, in an order where each subtree is contiguous
		// and follows its root.
		order := tree.Order()
		if len(order) != len(tasks) {
			fail("%d nodes in order, %d tasks", len(order), len(tasks))
		}
		seen := map[int]bool{}
		for pos, n := range order {
			if seen[n.Index] || n.Task != tasks[n.Index] || tree.Nodes[n.Index] != n {
				fail("node %d at %d is repeated or isn't its task", n.Index, pos)
			}
			seen[n.Index] = true
			sub := n.Subtree()
			if pos+len(sub) > len(order) || !slices.Equal(order[pos:pos+len(sub)], sub) {
				fail("the subtree of task %d isn't contiguous after it", n.Index)
			}
			if n.Parent != nil && n.Depth != n.Parent.Depth+1 || n.Parent == nil && n.Depth != 0 {
				fail("task %d has depth %d", n.Index, n.Depth)
			}
		}

		// Top-level tasks in the order they came; subtasks by priority,
		// ties in the order they came.
		for k := 1; k < len(tree.Roots); k++ {
			if tree.Roots[k-1].Index > tree.Roots[k].Index {
				fail("top-level tasks %d and %d swapped", tree.Roots[k-1].Index, tree.Roots[k].Index)
			}
		}
		for _, n := range tree.Nodes {
			for k := 1; k < len(n.Children); k++ {
				a, b := n.Children[k-1], n.Children[k]
				c := comparePriority(a.Task.Priority, b.Task.Priority)
				if c > 0 || c == 0 && a.Index > b.Index {
					fail("subtasks %d and %d of task %d out of order", a.Index, b.Index, n.Index)
				}
			}
		}

		// Each link is what the tags say, and each task at the top level
		// with a p: is there for a reason the design gives.
		for i, n := range tree.Nodes {
			ps := tasks[i].ParentIDs()
			if len(ps) > 1 && !hasProblem(tree, i, "has more than one p:") {
				fail("task %d has several p: and no problem", i)
			}
			if n.Parent != nil {
				if len(ps) == 0 || n.Parent.Task.ID() != ps[0] || len(holders(tasks, ps[0])) != 1 || loopsBack(tasks, i) {
					fail("task %d is under task %d, which its first p: doesn't name alone", i, n.Parent.Index)
				}
				continue
			}
			if len(ps) == 0 {
				continue
			}
			h := holders(tasks, ps[0])
			switch {
			case len(h) == 0 && done[ps[0]]:
				if !n.DoneParent || hasProblem(tree, i, "p:") {
					fail("task %d names a done task, but isn't marked or has a problem", i)
				}
			case len(h) == 0:
				if !hasProblem(tree, i, "p:"+ps[0]+" names no task") || !tree.Unresolved {
					fail("task %d names no task, without a problem", i)
				}
			case len(h) > 1:
				for _, k := range h {
					if !hasProblem(tree, k, "id:"+ps[0]+" is on more than one task") {
						fail("task %d holds a duplicated id, without a problem", k)
					}
				}
			case !loopsBack(tasks, i):
				fail("task %d is at the top level, but its p: names one task and makes no loop", i)
			case !hasProblem(tree, i, "p:"+ps[0]+" makes a loop"):
				fail("task %d is in a loop, without a problem", i)
			}
			if n.DoneParent != (len(h) == 0 && done[ps[0]]) {
				fail("task %d's done parent mark is wrong", i)
			}
		}
	}
}

func TestNextID_Properties(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 17))
	for range 400 {
		tasks := randomTreeTasks(t, r, 15)
		id := NextID(tasks)
		used := map[string]bool{}
		for _, task := range tasks {
			for _, v := range append(task.annotationValues(idKey), task.ParentIDs()...) {
				used[v] = true
			}
		}
		n, err := strconv.Atoi(id)
		if err != nil || n < 1 || used[id] {
			t.Fatalf("NextID = %q, which isn't a free whole number (used: %v)", id, used)
		}
		for k := 1; k < n; k++ {
			if !used[strconv.Itoa(k)] {
				t.Fatalf("NextID = %q, but %d is free", id, k)
			}
		}
	}
}

func TestArrange_Properties(t *testing.T) {
	r := rand.New(rand.NewPCG(41, 3))
	for range 600 {
		tasks := randomTreeTasks(t, r, 30)
		raws := make(map[*Task]string, len(tasks))
		for _, task := range tasks {
			raws[task] = task.Raw
		}
		c := &Context{Name: "work", Tasks: slices.Clone(tasks)}
		c.Arrange()

		// A permutation: every task once, its line untouched.
		if len(c.Tasks) != len(tasks) {
			t.Fatalf("%d tasks became %d", len(tasks), len(c.Tasks))
		}
		seen := map[*Task]bool{}
		for i, task := range c.Tasks {
			if _, ok := raws[task]; !ok || seen[task] {
				t.Fatalf("task %d of the arranged list is new or repeated: %q", i, task.Raw)
			}
			seen[task] = true
			if task.Raw != raws[task] {
				t.Fatalf("arranging changed %q to %q", raws[task], task.Raw)
			}
			if task.Line != i {
				t.Fatalf("task %d numbered %d", i, task.Line)
			}
		}

		// The tree's order, and already in it: arranging again changes
		// nothing, and the arranged list reads as the same tree.
		want := NewTree(tasks, nil).Order()
		for i, n := range want {
			if c.Tasks[i] != n.Task {
				t.Fatalf("task %d is %q, the tree has %q there", i, c.Tasks[i].Raw, n.Task.Raw)
			}
		}
		again := &Context{Name: "work", Tasks: slices.Clone(c.Tasks)}
		again.Arrange()
		if !slices.Equal(again.Tasks, c.Tasks) {
			t.Fatal("arranging an arranged list moved a task")
		}
		if got, want := outline(NewTree(c.Tasks, nil)), outline(NewTree(tasks, nil)); got != want {
			t.Fatalf("the arranged list reads as another tree:\n%s\nwant:\n%s", got, want)
		}

		// The done file and the archive aren't trees: as they were.
		for _, name := range []string{DoneContextName, ArchiveContextName} {
			flat := &Context{Name: name, Tasks: slices.Clone(tasks)}
			flat.Arrange()
			if !slices.Equal(flat.Tasks, tasks) {
				t.Fatalf("%s was reordered", name)
			}
		}
	}
}
