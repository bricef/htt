package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
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
	// ErrOnlyLink: the task has no text besides its p:, so taking the
	// link away would leave it an empty line.
	ErrOnlyLink = errors.New("it has no text besides its link, so without it the task would be empty")
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

// SubtasksQuestion is what htt asks before completing task n, which has
// open tasks below it, at any depth: four lines, the CLI's and the
// TUI's alike. x answers SubtasksLeave, A SubtasksComplete, anything
// else cancels.
func SubtasksQuestion(n string, open int) []string {
	first := fmt.Sprintf("Task %s has %d open subtasks.", n, open)
	if open == 1 {
		first = fmt.Sprintf("Task %s has 1 open subtask.", n)
	}
	return []string{
		first,
		"[x] Mark it done, and leave the subtasks as they are.",
		"[A] Mark the subtasks done too.",
		"Anything else cancels.",
	}
}

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
	if c.doneIDs == nil {
		done, err := c.doneTasks()
		if err != nil {
			return nil, fmt.Errorf("read the done file for %s's done parents: %w", c.Name, err)
		}
		c.doneIDs = map[string]bool{}
		for _, t := range done {
			if id := t.ID(); id != "" && t.Annotations[contextKey] == c.Name {
				c.doneIDs[id] = true
			}
		}
	}
	return NewTree(c.Tasks, c.doneIDs), nil
}

// doneTasks returns the tasks of the done file that could be done
// parents of c's tasks: from a LineSource, only the lines with an id:
// and c's context: in them, parsed; from any other repository, all.
func (c *Context) doneTasks() ([]*Task, error) {
	source, ok := c.repo.(LineSource)
	if !ok {
		done, err := c.repo.Context(DoneContextName)
		if err != nil {
			return nil, err
		}
		return done.Tasks, nil
	}
	lines, err := source.ContextLines(DoneContextName)
	if err != nil {
		return nil, err
	}
	var out []*Task
	for _, l := range lines {
		if !strings.Contains(l, idKey+":") || !strings.Contains(l, "context:"+c.Name) {
			continue // can't hold an id completed from c
		}
		t, err := NewTask(l)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
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

// subtaskID returns the id a subtask of parent names: parent's own, or,
// when it has none, the next free id (NextID, counting extra too), which
// fresh says parent would need. A parent whose id another task holds
// too is refused: a p: naming it would be a broken link.
func (c *Context) subtaskID(parent *Task, extra ...*Task) (id string, fresh bool, err error) {
	if id := parent.ID(); id != "" {
		holders := 0
		for _, t := range c.Tasks {
			if t.ID() == id {
				holders++
			}
		}
		if holders > 1 {
			return "", false, fmt.Errorf("%w (id:%s)", ErrSharedID, id)
		}
		return id, false, nil
	}
	return NextID(append(slices.Clone(c.Tasks), extra...)), true, nil
}

// idFor is subtaskID, giving parent the id when it needs one.
func (c *Context) idFor(parent *Task, extra ...*Task) (string, error) {
	id, fresh, err := c.subtaskID(parent, extra...)
	if err != nil || !fresh {
		return id, err
	}
	if _, err := parent.Annotate(idKey, id); err != nil {
		return "", fmt.Errorf("give the parent an id: %w", err)
	}
	return id, nil
}

// SubtaskLine returns the line AddSubtask would save task as, under the
// task numbered parentID, changing nothing: for a preview that can't
// differ from the file.
func (c *Context) SubtaskLine(parentID string, task *Task) (string, error) {
	if !c.IsTree() {
		return "", ErrNotATree
	}
	parent, err := c.GetTaskByStrId(parentID)
	if err != nil {
		return "", err
	}
	id, _, err := c.subtaskID(parent, task)
	if err != nil {
		return "", err
	}
	draft, err := NewTask(task.Raw)
	if err != nil {
		return "", err
	}
	draft.setParent(id)
	return draft.StampCreated(time.Now()).Raw, nil
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
// task with no p: is refused (ErrTopLevel), as is one with no text
// besides them (ErrOnlyLink).
//
// The task needn't move in the list: the list is in tree order, so it
// is already between the top-level task it was under and the next one,
// where arranging, which keeps the top level's order, leaves it: right
// after the subtree it left.
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
	if err := n.Task.takeParent(); err != nil {
		return nil, err
	}
	return n.Task, c.save()
}

// NestUnderPrevious puts the task numbered strID, with its subtree,
// under the task above it at its level, its previous sibling (for a
// top-level task, the top-level task above it), and persists: the
// TUI's Tab. It returns the task, as saved. The first task at its level
// is refused (ErrNothingAbove). In the list, in tree order, the task
// already follows the subtree of the task above it, so it's last among
// that task's subtasks of its priority without moving.
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
	n.Task.setParent(id)
	return n.Task, c.save()
}

// UnnestOneLevel makes the task numbered strID, with its subtree, a
// sibling of its parent, placed just after the parent's subtree: under
// its grandparent, or, under a top-level task, at the top level. It
// persists and returns the task, as saved: the TUI's Shift-Tab. A
// top-level task is refused (ErrTopLevel), as is one with no text
// besides its p: going to the top level (ErrOnlyLink). As for Unnest,
// the task needn't move in the list: it is already between its parent
// and the parent's next sibling, which arranging, keeping the order of
// siblings of a priority, leaves it between.
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
	if g := n.Parent.Parent; g != nil {
		n.Task.setParent(g.Task.ID())
	} else if err := n.Task.takeParent(); err != nil {
		return nil, err
	}
	return n.Task, c.save()
}

// fitInto readies tasks, a subtree in tree order moving from c, to join
// to, a context file, or refuses, changing nothing.
//
// Each of the subtree's ids that to already uses, as an id: or a p:,
// becomes to's smallest free id, the subtree's p: naming it with it. A
// done task going back from the done file to the context it was
// completed in (its context: names to's file) keeps its id when to's
// p: naming it can name nothing else: no task in to holds it, and no
// other task in the done file completed from to does. Its subtasks
// there are under it again.
//
// A parent is always in its subtasks' file, so the subtree's root loses
// its p:, unless it's a done task going back and its parent is there,
// the one task holding the id, and not below it: it keeps it. A root
// with no text besides its p: is refused (ErrOnlyLink): without it, it
// would be an empty line.
func (c *Context) fitInto(tasks []*Task, to *Context) error {
	root := tasks[0]
	back := c.Name == DoneContextName &&
		ContextFileName(root.Annotations[contextKey]) == ContextFileName(to.Name)
	own := ""
	if id := root.ID(); back && id != "" && len(holding(to.Tasks, id)) == 0 &&
		len(completedFrom(holding(c.Tasks, id), to.Name)) == 1 {
		own = id
	}
	if ps := root.ParentIDs(); len(ps) > 0 {
		parent := holding(to.Tasks, ps[0])
		keep := back && len(parent) == 1 &&
			(own == "" || !(&Context{Tasks: append(slices.Clone(to.Tasks), root)}).leadsTo(parent[0], root))
		if !keep {
			if err := root.takeParent(); err != nil {
				return err
			}
		}
	}
	taken := map[string]bool{}
	for _, t := range to.Tasks {
		for _, v := range append(t.annotationValues(idKey), t.ParentIDs()...) {
			if v != own { // no task's id there: only p: and second id: tags have it
				taken[v] = true
			}
		}
	}
	used := append(slices.Clone(to.Tasks), tasks...)
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
		}
		for _, s := range tasks[1:] { // the root's p:, if it kept one, names a task of to's
			s.renameAnnotation(parentKey, old, id)
		}
	}
	return nil
}

// holding returns the tasks whose id is id.
func holding(tasks []*Task, id string) []*Task {
	var out []*Task
	for _, t := range tasks {
		if t.ID() == id {
			out = append(out, t)
		}
	}
	return out
}

// completedFrom returns the tasks, of the done file, completed from the
// context named name: their context: names its file.
func completedFrom(tasks []*Task, name string) []*Task {
	var out []*Task
	for _, t := range tasks {
		if ContextFileName(t.Annotations[contextKey]) == ContextFileName(name) {
			out = append(out, t)
		}
	}
	return out
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

// takeParent removes every p: from the task, or, when they're all the
// text it has, refuses (ErrOnlyLink), changing nothing: without them,
// it would be an empty line, or only a preamble.
func (t *Task) takeParent() error {
	if !slices.ContainsFunc(t.body, func(w bodyWord) bool { return w.key != parentKey }) {
		return fmt.Errorf("%w: %s", ErrOnlyLink, t.Raw)
	}
	t.RemoveAnnotation(parentKey)
	return nil
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
