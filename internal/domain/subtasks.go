package domain

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// The operations on subtasks (docs/design/subtasks.md, "Behaviour").
// Each refuses, changing nothing, when it can't do what was asked.
var (
	// ErrNotATree: the done file and the archive don't hold subtasks.
	ErrNotATree = errors.New("the done file and the archive don't hold subtasks")
	// ErrLoop: the nesting would put a task below itself.
	ErrLoop = errors.New("that would put the task below itself")
	// ErrSharedID: the would-be parent's id is on another task too, so
	// a p: naming it couldn't say which.
	ErrSharedID = errors.New("its id is on more than one task, so a subtask couldn't name it")
	// ErrTopLevel: the task has no parent to leave.
	ErrTopLevel = errors.New("it's already a top-level task")
	// ErrNothingAbove: the task is the first at its level, with nothing
	// above it to nest under.
	ErrNothingAbove = errors.New("it's the first task at its level, with nothing above it to nest under")
	// ErrOpenSubtasks: completing the task needs an answer about its
	// open subtasks (see Subtasks).
	ErrOpenSubtasks = errors.New("it has open subtasks")
)

// Subtasks says what completing a task does with the open tasks below
// it, at any depth.
type Subtasks int

const (
	// SubtasksAsk refuses, with ErrOpenSubtasks, to complete a task
	// with open subtasks: the caller asks first. The zero value.
	SubtasksAsk Subtasks = iota
	// SubtasksLeave completes the task alone; its subtasks stay open,
	// still naming it ([x] in the prompt).
	SubtasksLeave
	// SubtasksComplete completes the task and every open task below it
	// ([A] in the prompt).
	SubtasksComplete
)

// Tree returns the context's tree (NewTree), telling done parents from
// broken links by the context's tasks in the done file: those whose
// context: annotation names it, as Complete writes them. The done file
// is read only when a link doesn't resolve in the context's own file.
// The done file and the archive are flat: every task at the top level,
// no problems.
func (c *Context) Tree() (*Tree, error) {
	if !c.IsTree() {
		tree := &Tree{Nodes: make([]*Node, len(c.Tasks))}
		for i, t := range c.Tasks {
			tree.Nodes[i] = &Node{Index: i, Task: t}
		}
		tree.Roots = slices.Clone(tree.Nodes)
		return tree, nil
	}
	tree := NewTree(c.Tasks, nil)
	if !tree.Unresolved || c.repo == nil {
		return tree, nil
	}
	done, err := c.repo.Context(DoneContextName)
	if err != nil {
		return nil, fmt.Errorf("read the done file for %s's done parents: %w", c.Name, err)
	}
	ids := map[string]bool{}
	for _, t := range done.Tasks {
		if id := t.ID(); id != "" && t.Annotations["context"] == c.Name {
			ids[id] = true
		}
	}
	return NewTree(c.Tasks, ids), nil
}

// node finds the task numbered strID in the context's tree.
func (c *Context) node(tree *Tree, strID string) (*Node, error) {
	task, err := c.GetTaskByStrId(strID)
	if err != nil {
		return nil, err
	}
	return tree.Nodes[slices.Index(c.Tasks, task)], nil
}

// below returns the tasks the task numbered strID takes with it when it
// moves or is deleted: itself and, in a context file, its subtree, in
// tree order.
func (c *Context) below(strID string) ([]*Task, error) {
	tree := NewTree(c.Tasks, nil)
	n, err := c.node(tree, strID)
	if err != nil {
		return nil, err
	}
	if !c.IsTree() {
		return []*Task{n.Task}, nil
	}
	var out []*Task
	for _, s := range n.Subtree() {
		out = append(out, s.Task)
	}
	return out, nil
}

// OpenSubtasks returns the open tasks below the task numbered strID, at
// any depth, in tree order: what completing it with SubtasksComplete
// completes too.
func (c *Context) OpenSubtasks(strID string) ([]*Task, error) {
	tasks, err := c.below(strID)
	if err != nil {
		return nil, err
	}
	var open []*Task
	for _, t := range tasks[1:] {
		if !t.Completed {
			open = append(open, t)
		}
	}
	return open, nil
}

// idFor returns the id a subtask of parent names, giving parent the
// next free id (NextID, counting extra too) if it has none. A parent
// whose id another task holds too is refused: a p: naming it would be a
// broken link.
func (c *Context) idFor(parent *Task, extra ...*Task) (string, error) {
	if id := parent.ID(); id != "" {
		holders := 0
		for _, t := range c.Tasks {
			if t.ID() == id {
				holders++
			}
		}
		if holders > 1 {
			return "", fmt.Errorf("%w (id:%s)", ErrSharedID, id)
		}
		return id, nil
	}
	id := NextID(append(slices.Clone(c.Tasks), extra...))
	if _, err := parent.Annotate(idKey, id); err != nil {
		return "", fmt.Errorf("give the parent an id: %w", err)
	}
	return id, nil
}

// leadsTo reports whether following first p: links up from from, each
// to the one task in the context holding the id, reaches to (or from is
// to): giving to a p: naming from would close a loop.
func (c *Context) leadsTo(from, to *Task) bool {
	seen := map[*Task]bool{}
	for t := from; t != nil && !seen[t]; {
		if t == to {
			return true
		}
		seen[t] = true
		ps := t.ParentIDs()
		if len(ps) == 0 {
			return false
		}
		var holder *Task
		for _, h := range c.Tasks {
			if h.ID() == ps[0] {
				if holder != nil {
					return false // shared: no link
				}
				holder = h
			}
		}
		t = holder
	}
	return false
}

// moveAfter moves block, tasks of the context in order, to just after
// anchor, which isn't one of them.
func (c *Context) moveAfter(block []*Task, anchor *Task) {
	c.Tasks = slices.DeleteFunc(c.Tasks, func(t *Task) bool { return slices.Contains(block, t) })
	at := slices.Index(c.Tasks, anchor) + 1
	c.Tasks = slices.Insert(c.Tasks, at, block...)
}

// lastOutside returns the last task of n's subtree that isn't in block.
func lastOutside(n *Node, block []*Task) *Task {
	sub := n.Subtree()
	for i := len(sub) - 1; i >= 0; i-- {
		if !slices.Contains(block, sub[i].Task) {
			return sub[i].Task
		}
	}
	return n.Task
}

func tasksOfNodes(nodes []*Node) []*Task {
	out := make([]*Task, len(nodes))
	for i, n := range nodes {
		out[i] = n.Task
	}
	return out
}

// AddSubtask adds task under the task numbered parentID, which gets an
// id: if it has none, and persists. The task is stamped with today's
// date as AddTask stamps it, and lands after its parent's last
// descendant, or at its priority's place among the parent's subtasks.
func (c *Context) AddSubtask(parentID string, task *Task) error {
	if !c.IsTree() {
		return ErrNotATree
	}
	parent, err := c.GetTaskByStrId(parentID)
	if err != nil {
		return err
	}
	id, err := c.idFor(parent, task)
	if err != nil {
		return err
	}
	task.setParent(id)
	task.StampCreated(time.Now())
	c.add(task)
	return c.save()
}

// Nest puts the task numbered strID, with its subtree, under the task
// numbered parentID (which gets an id: if it has none), last among the
// parent's subtasks of its priority, and persists. It returns the task
// and its parent, as saved. A nesting that would make a loop is
// refused (ErrLoop), as is a parent whose id another task holds
// (ErrSharedID). Nesting a task under its parent changes nothing.
func (c *Context) Nest(strID, parentID string) (*Task, *Task, error) {
	if !c.IsTree() {
		return nil, nil, ErrNotATree
	}
	tree := NewTree(c.Tasks, nil)
	n, err := c.node(tree, strID)
	if err != nil {
		return nil, nil, err
	}
	m, err := c.node(tree, parentID)
	if err != nil {
		return nil, nil, err
	}
	if c.leadsTo(m.Task, n.Task) {
		return nil, nil, ErrLoop
	}
	if n.Parent == m {
		return n.Task, m.Task, nil
	}
	id, err := c.idFor(m.Task)
	if err != nil {
		return nil, nil, err
	}
	block := tasksOfNodes(n.Subtree())
	c.moveAfter(block, lastOutside(m, block))
	n.Task.setParent(id)
	return n.Task, m.Task, c.save()
}

// Unnest makes the task numbered strID a top-level task, with its
// subtree, just after the subtree of the top-level task it was under,
// and persists. It removes every p: on the task, so a task shown at
// the top level for a broken link or a done parent loses the link. A
// task with no p: is refused (ErrTopLevel).
func (c *Context) Unnest(strID string) (*Task, error) {
	if !c.IsTree() {
		return nil, ErrNotATree
	}
	tree := NewTree(c.Tasks, nil)
	n, err := c.node(tree, strID)
	if err != nil {
		return nil, err
	}
	if len(n.Task.ParentIDs()) == 0 {
		return nil, ErrTopLevel
	}
	top := n
	for top.Parent != nil {
		top = top.Parent
	}
	if top != n {
		block := tasksOfNodes(n.Subtree())
		c.moveAfter(block, lastOutside(top, block))
	}
	n.Task.clearParent()
	return n.Task, c.save()
}

// NestUnderPrevious puts the task numbered strID, with its subtree,
// under the task above it at its level, its previous sibling (for a
// top-level task, the top-level task above it), and persists: the
// TUI's Tab. It returns the task, as saved. The first task at its level
// is refused (ErrNothingAbove).
func (c *Context) NestUnderPrevious(strID string) (*Task, error) {
	if !c.IsTree() {
		return nil, ErrNotATree
	}
	tree := NewTree(c.Tasks, nil)
	n, err := c.node(tree, strID)
	if err != nil {
		return nil, err
	}
	siblings := tree.Roots
	if n.Parent != nil {
		siblings = n.Parent.Children
	}
	k := slices.Index(siblings, n)
	if k == 0 {
		return nil, ErrNothingAbove
	}
	above := siblings[k-1]
	if c.leadsTo(above.Task, n.Task) {
		return nil, ErrLoop
	}
	id, err := c.idFor(above.Task)
	if err != nil {
		return nil, err
	}
	block := tasksOfNodes(n.Subtree())
	c.moveAfter(block, lastOutside(above, block))
	n.Task.setParent(id)
	return n.Task, c.save()
}

// UnnestOneLevel makes the task numbered strID, with its subtree, a
// sibling of its parent, placed just after the parent's subtree: under
// its grandparent, or, under a top-level task, at the top level. It
// persists and returns the task, as saved: the TUI's Shift-Tab. A
// top-level task is refused (ErrTopLevel).
func (c *Context) UnnestOneLevel(strID string) (*Task, error) {
	if !c.IsTree() {
		return nil, ErrNotATree
	}
	tree := NewTree(c.Tasks, nil)
	n, err := c.node(tree, strID)
	if err != nil {
		return nil, err
	}
	if n.Parent == nil {
		return nil, ErrTopLevel
	}
	block := tasksOfNodes(n.Subtree())
	c.moveAfter(block, lastOutside(n.Parent, block))
	if g := n.Parent.Parent; g != nil {
		n.Task.setParent(g.Task.ID())
	} else {
		n.Task.clearParent()
	}
	return n.Task, c.save()
}

// fitInto readies tasks, a subtree in tree order, to join target, a
// context file: a parent is always in its subtasks' file, so the
// subtree's root loses its p:, and each of the subtree's ids that
// target already uses, as an id: or a p:, becomes target's smallest
// free id, the subtree's p: naming it with it.
func fitInto(tasks []*Task, target []*Task) {
	if len(tasks[0].ParentIDs()) > 0 {
		tasks[0].clearParent()
	}
	taken := map[string]bool{}
	for _, t := range target {
		for _, v := range t.annotationValues(idKey) {
			taken[v] = true
		}
		for _, v := range t.ParentIDs() {
			taken[v] = true
		}
	}
	used := append(slices.Clone(target), tasks...)
	renamed := map[string]bool{}
	for _, t := range tasks {
		old := t.ID()
		if old == "" || !taken[old] || renamed[old] {
			continue
		}
		renamed[old] = true
		id := NextID(used)
		for _, s := range tasks {
			s.renameAnnotation(idKey, old, id)
			s.renameAnnotation(parentKey, old, id)
		}
	}
}

// setParent makes the task name id as its parent: its first p: takes
// the value where it stands, any other p: goes, and a task without one
// gets p:id at the end.
func (t *Task) setParent(id string) {
	seen := false
	t.body = slices.DeleteFunc(t.body, func(w bodyWord) bool {
		if w.key != parentKey {
			return false
		}
		drop := seen
		seen = true
		return drop
	})
	t.Annotations[parentKey] = id
	t.rebuild()
}

// clearParent removes every p: from the task.
func (t *Task) clearParent() {
	t.RemoveAnnotation(parentKey)
}

// renameAnnotation rewrites every key:old on the line as key:new where
// it stands, and leaves a line without one alone.
func (t *Task) renameAnnotation(key, old, new string) {
	changed := false
	for i, w := range t.body {
		if w.key == key && w.text == key+":"+old {
			t.body[i].text = key + ":" + new
			changed = true
		}
	}
	if !changed {
		return
	}
	if v := t.annotationValues(key); len(v) > 0 {
		t.Annotations[key] = v[len(v)-1] // the map holds the last
	}
	t.rebuild()
}
