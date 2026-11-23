package domain

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Subtasks (docs/design/subtasks.md): a task that has subtasks carries
// id:N, and each of its subtasks p:N, as topydo writes them. A task's
// id is its first id: (topydo's tag_value), and its parent is named by
// its first p:. Ids are text, matched exactly, as topydo matches them.

// idKey and parentKey are the annotations that link a subtask to its
// parent.
const (
	idKey     = "id"
	parentKey = "p"
)

// Node is a task's place in its context's tree.
type Node struct {
	Index      int     // the task's index in the tasks the tree was built from
	Task       *Task   //
	Parent     *Node   // nil for a top-level task
	Children   []*Node // in tree order: by priority, then in the order they came
	Depth      int     // 0 at the top level
	DoneParent bool    // at the top level because its parent is in the context's done file
}

// LinkProblem is a broken link: a reason the task at Index is shown at
// the top level (or, for several p:, under its first parent only), as
// a message naming the tag.
type LinkProblem struct {
	Index   int
	Message string
}

// Tree is a context's tasks as a tree. Every task is in it once:
// Nodes holds them by index, Roots the top-level ones in the order
// they came.
type Tree struct {
	Roots    []*Node
	Nodes    []*Node
	Problems []LinkProblem // in task order

	// Unresolved is true when some task's p: names no task among the
	// ones the tree was built from, nor a done id it was given: only
	// the context's done file could tell a done parent from a broken
	// link.
	Unresolved bool
}

// ID returns the task's id, its first id: annotation, or "" if it has
// none.
func (t *Task) ID() string {
	if v := t.annotationValues(idKey); len(v) > 0 {
		return v[0]
	}
	return ""
}

// ParentIDs returns the values of the task's p: annotations, in line
// order. The first names its parent.
func (t *Task) ParentIDs() []string {
	return t.annotationValues(parentKey)
}

// annotationValues returns the value of every key: annotation on the
// line, in line order.
func (t *Task) annotationValues(key string) []string {
	var out []string
	for _, w := range t.body {
		if w.key == key {
			out = append(out, strings.TrimPrefix(w.text, key+":"))
		}
	}
	return out
}

// NewTree builds the tree of tasks, a context's tasks in any order.
// doneIDs are the ids of the context's tasks in the done file: a p:
// naming one of them names a done parent, which puts the task at the
// top level without a problem. A p: that names nothing else, a loop,
// a parent id on several tasks and several p: on a task are problems
// (see LinkProblem); every task they affect is at the top level but
// for several p:, which leaves the task under its first parent. A
// task's subtasks are in priority order, (A) to (Z) then none, ties in
// the order they came; top-level tasks stay in the order they came.
func NewTree(tasks []*Task, doneIDs map[string]bool) *Tree {
	tree := &Tree{Nodes: make([]*Node, len(tasks))}
	for i, t := range tasks {
		tree.Nodes[i] = &Node{Index: i, Task: t}
	}

	owners := map[string][]int{} // id → the tasks holding it
	for i, t := range tasks {
		if id := t.ID(); id != "" {
			owners[id] = append(owners[id], i)
		}
	}

	// Each task's problems, in this order: its p:, then several p:.
	found := make([][]string, len(tasks))
	note := func(i int, format string, args ...any) {
		found[i] = append(found[i], fmt.Sprintf(format, args...))
	}

	parent := make([]int, len(tasks)) // -1 at the top level
	for i, t := range tasks {
		parent[i] = -1
		ps := t.ParentIDs()
		if len(ps) == 0 {
			continue
		}
		switch held := owners[ps[0]]; {
		case len(held) == 1:
			parent[i] = held[0] // itself, a loop, is caught below
		case len(held) > 1:
			// The holders' problem; this task is at the top level.
		case doneIDs[ps[0]]:
			tree.Nodes[i].DoneParent = true
		default:
			tree.Unresolved = true
			note(i, "p:%s names no task, so the task is shown at the top level", ps[0])
		}
	}

	// Loops: a walk up from each task either reaches the top level, a
	// task already walked, or a task on its own path, which closes a
	// loop. Every task of a loop goes to the top level.
	const unseen, onPath, walked = 0, 1, 2
	state := make([]int, len(tasks))
	inLoop := make([]bool, len(tasks))
	for i := range tasks {
		var path []int
		j := i
		for j >= 0 && state[j] == unseen {
			state[j] = onPath
			path = append(path, j)
			j = parent[j]
		}
		if j >= 0 && state[j] == onPath {
			for _, k := range path[slices.Index(path, j):] {
				inLoop[k] = true
			}
		}
		for _, k := range path {
			state[k] = walked
		}
	}
	for i := range tasks {
		if inLoop[i] {
			parent[i] = -1
			note(i, "p:%s makes a loop (the task would be below itself), so it is shown at the top level", tasks[i].ParentIDs()[0])
		}
	}

	for i, t := range tasks {
		if id := t.ID(); id != "" && len(owners[id]) > 1 {
			note(i, "id:%s is on more than one task, so its subtasks are shown at the top level", id)
		}
		if ps := t.ParentIDs(); len(ps) > 1 {
			tags := make([]string, len(ps))
			for k, p := range ps {
				tags[k] = parentKey + ":" + p
			}
			note(i, "has more than one p: (%s); htt reads only the first", strings.Join(tags, " "))
		}
	}
	for i, msgs := range found {
		for _, m := range msgs {
			tree.Problems = append(tree.Problems, LinkProblem{Index: i, Message: m})
		}
	}

	for i, n := range tree.Nodes {
		if parent[i] < 0 {
			tree.Roots = append(tree.Roots, n)
			continue
		}
		p := tree.Nodes[parent[i]]
		n.Parent = p
		p.Children = append(p.Children, n)
	}
	for _, n := range tree.Nodes {
		slices.SortStableFunc(n.Children, func(a, b *Node) int {
			return comparePriority(a.Task.Priority, b.Task.Priority)
		})
	}
	for _, n := range tree.Order() {
		if n.Parent != nil {
			n.Depth = n.Parent.Depth + 1
		}
	}
	return tree
}

// Order returns every node in tree order: each top-level task followed
// by its subtree, each subtask followed by its own.
func (t *Tree) Order() []*Node {
	out := make([]*Node, 0, len(t.Nodes))
	for _, r := range t.Roots {
		out = r.appendSubtree(out)
	}
	return out
}

// Subtree returns n and every node below it, in tree order.
func (n *Node) Subtree() []*Node {
	return n.appendSubtree(nil)
}

func (n *Node) appendSubtree(out []*Node) []*Node {
	out = append(out, n)
	for _, c := range n.Children {
		out = c.appendSubtree(out)
	}
	return out
}

// comparePriority orders priorities (A) first, then (B) and on, then
// no priority.
func comparePriority(a, b string) int {
	return priorityRank(a) - priorityRank(b)
}

// priorityRank is a priority's place: 0 for A to 25 for Z, 26 for none.
func priorityRank(p string) int {
	if p == "" {
		return 26
	}
	return int(p[0] - 'A')
}

// NextID returns the id a task gets when it first gets a subtask: the
// smallest whole number that no id: or p: among tasks uses, every
// occurrence counted (topydo's find_next_id).
func NextID(tasks []*Task) string {
	used := map[string]bool{}
	for _, t := range tasks {
		for _, v := range t.annotationValues(idKey) {
			used[v] = true
		}
		for _, v := range t.ParentIDs() {
			used[v] = true
		}
	}
	n := 1
	for used[strconv.Itoa(n)] {
		n++
	}
	return strconv.Itoa(n)
}

// IsTree reports whether the context's file holds a tree: every context
// but the done file and the archive. Those collect tasks from every
// context, so their ids aren't one file's: they stay flat, in the order
// they were written.
func (c *Context) IsTree() bool {
	return c.Name != DoneContextName && c.Name != ArchiveContextName
}

// Arrange puts the context's tasks in tree order (NewTree's Order) and
// numbers them in it (Task.Line), so the tree reads top to bottom in
// number order. It only moves tasks: no line is read back, rebuilt or
// dropped, and a list already in order stays as it is. Repositories
// arrange a context as they load it; Context.save arranges it before
// every write. The done file and the archive are only numbered.
func (c *Context) Arrange() {
	if c.IsTree() {
		order := NewTree(c.Tasks, nil).Order()
		c.Tasks = make([]*Task, len(order))
		for i, n := range order {
			c.Tasks[i] = n.Task
		}
	}
	for i, t := range c.Tasks {
		t.Line = i
	}
}
