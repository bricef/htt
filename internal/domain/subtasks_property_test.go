package domain

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// fakeRepo is a Repository in memory, for the domain's own tests (the
// storage package's MemoryRepository imports the domain). It loads a
// context arranged, as the real ones do.
type fakeRepo struct {
	files map[string][]*Task
	loads map[string]int // Context calls, by name
}

func newFakeRepo() *fakeRepo { return &fakeRepo{files: map[string][]*Task{}, loads: map[string]int{}} }

func (r *fakeRepo) Context(name string) (*Context, error) {
	r.loads[name]++
	c := NewContext(r, name)
	c.Tasks = slices.Clone(r.files[name])
	c.Arrange()
	return c, nil
}
func (r *fakeRepo) Save(c *Context) error               { r.files[c.Name] = slices.Clone(c.Tasks); return nil }
func (r *fakeRepo) Contexts() ([]*Context, error)       { return nil, nil }
func (r *fakeRepo) ContextNames() ([]string, error)     { return nil, nil }
func (r *fakeRepo) CurrentContext() (*Context, error)   { return r.Context("work") }
func (r *fakeRepo) CurrentContextName() (string, error) { return "work", nil }
func (r *fakeRepo) SetCurrent(string) error             { return nil }
func (r *fakeRepo) ContextPath(string) string           { return "" }

// lineRepo is fakeRepo as a LineSource, as storage's repositories are.
type lineRepo struct{ *fakeRepo }

func (r lineRepo) ContextLines(name string) ([]string, error) {
	var out []string
	for _, t := range r.files[name] {
		out = append(out, t.Raw)
	}
	return out, nil
}

// Read from the lines that could hold one, or from every task of the
// done file, the done parents are the same.
func TestTree_DoneParentsFromLinesOrTasks(t *testing.T) {
	r := rand.New(rand.NewPCG(91, 5))
	for range 300 {
		repo := newFakeRepo()
		repo.files["work"] = randomTreeTasks(t, r, 12)
		var done []*Task
		for range r.IntN(12) {
			line := "x 2026-01-01 " + randomTreeLine(r)
			if ctx := []string{"work", "home", "worker", ""}[r.IntN(4)]; ctx != "" {
				line += " context:" + ctx
			}
			done = append(done, mustTask(t, line))
		}
		repo.files[DoneContextName] = done
		plain, _ := repo.Context("work")
		viaLines := NewContext(lineRepo{repo}, "work")
		viaLines.Tasks = plain.Tasks
		a, errA := plain.Tree()
		b, errB := viaLines.Tree()
		if errA != nil || errB != nil {
			t.Fatal(errA, errB)
		}
		for i := range a.Nodes {
			if a.Nodes[i].DoneParent != b.Nodes[i].DoneParent {
				t.Fatalf("task %q: done parent %v from tasks, %v from lines", a.Nodes[i].Task.Raw, a.Nodes[i].DoneParent, b.Nodes[i].DoneParent)
			}
		}
		if len(a.Problems) != len(b.Problems) {
			t.Fatalf("problems differ: %v and %v", a.Problems, b.Problems)
		}
	}
}

// The done file is read once for a context's tree, however often it's
// asked for, until the context changes.
func TestTree_ReadsTheDoneFileOnce(t *testing.T) {
	repo := newFakeRepo()
	repo.files["work"] = tasksOf(t, "a id:1", "b p:1", "c p:2", "d p:3")
	repo.files[DoneContextName] = tasksOf(t, "x parent id:2 context:work")
	c, _ := repo.Context("work")
	for range 3 {
		tree, err := c.Tree()
		if err != nil || !tree.Nodes[2].DoneParent {
			t.Fatalf("tree: %v, done parent %v", err, tree.Nodes[2].DoneParent)
		}
	}
	if repo.loads[DoneContextName] != 1 {
		t.Errorf("done read %d times, want once", repo.loads[DoneContextName])
	}
	// Completing a: c's done parent is still there, and so is a now.
	if _, err := c.Complete("0", SubtasksLeave); err != nil {
		t.Fatal(err)
	}
	tree, err := c.Tree()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range tree.Nodes {
		if want := n.Task.Raw != "d p:3"; n.DoneParent != want {
			t.Errorf("%q: done parent %v, want %v", n.Task.Raw, n.DoneParent, want)
		}
	}
}

// nonTagWords is a line's words but its id: and p: tags, which the
// subtask operations may change.
func nonTagWords(t *Task) []string {
	var out []string
	for _, w := range t.body {
		if w.key != idKey && w.key != parentKey {
			out = append(out, w.text)
		}
	}
	return out
}

// parents maps each task to its parent in the tree, nil at the top level.
func parents(tree *Tree) map[*Task]*Task {
	out := map[*Task]*Task{}
	for _, n := range tree.Nodes {
		out[n.Task] = nil
		if n.Parent != nil {
			out[n.Task] = n.Parent.Task
		}
	}
	return out
}

func subtreeTasks(n *Node) []*Task {
	var out []*Task
	for _, s := range n.Subtree() {
		out = append(out, s.Task)
	}
	return out
}

func idHolders(tasks []*Task, id string) int {
	n := 0
	for _, t := range tasks {
		if t.ID() == id {
			n++
		}
	}
	return n
}

// The operations that arrange a tree (add under, nest, unnest, and the
// TUI's one-level moves) over generated files: the task lands where the
// design says, takes its subtree with it, and nothing else moves out
// from under its parent; no task is lost or added, and only the task
// moved and its new parent have their lines changed, only in their id:
// and p: tags. A refused operation changes nothing.
func TestArranging_Properties(t *testing.T) {
	r := rand.New(rand.NewPCG(61, 7))
	ops := []string{"add", "nest", "unnest", "previous", "level"}
	for range 1500 {
		repo := newFakeRepo()
		repo.files["work"] = randomTreeTasks(t, r, 20)
		ctx, _ := repo.Context("work")
		if len(ctx.Tasks) == 0 {
			continue
		}
		before := slices.Clone(ctx.Tasks)
		stored := slices.Clone(repo.files["work"])
		raws := map[*Task]string{}
		words := map[*Task][]string{}
		for _, task := range before {
			raws[task], words[task] = task.Raw, nonTagWords(task)
		}
		tree := NewTree(before, nil)
		oldParent := parents(tree)
		ok := map[*Task]bool{} // tasks with no link problem
		for _, task := range before {
			ok[task] = true
		}
		for _, p := range tree.Problems {
			ok[before[p.Index]] = false
		}

		op := ops[r.IntN(len(ops))]
		i, j := r.IntN(len(before)), r.IntN(len(before))
		n, m := tree.Nodes[i], tree.Nodes[j]
		var moved, newParent *Task
		var wantErr error
		var err error
		label := op + " " + strconv.Itoa(i) + " " + strconv.Itoa(j)
		shared := func(task *Task) bool { return task.ID() != "" && idHolders(before, task.ID()) > 1 }
		switch op {
		case "add":
			moved, newParent = mustTask(t, "2026-01-01 new"), m.Task
			if shared(m.Task) {
				wantErr = ErrSharedID
			}
			err = ctx.AddSubtask(strconv.Itoa(j), moved)
		case "nest":
			moved, newParent = n.Task, m.Task
			switch {
			case reaches(before, m.Task, n.Task): // under itself, or below itself, by its tags
				wantErr = ErrLoop
			case shared(m.Task):
				wantErr = ErrSharedID
			}
			_, _, err = ctx.Nest(strconv.Itoa(i), strconv.Itoa(j))
		case "unnest":
			moved = n.Task
			if len(n.Task.ParentIDs()) == 0 {
				wantErr = ErrTopLevel
			}
			_, err = ctx.Unnest(strconv.Itoa(i))
		case "previous":
			moved = n.Task
			siblings := tree.Roots
			if n.Parent != nil {
				siblings = n.Parent.Children
			}
			k := slices.Index(siblings, n)
			switch {
			case k == 0:
				wantErr = ErrNothingAbove
			case reaches(before, siblings[k-1].Task, n.Task): // the two in a loop
				wantErr = ErrLoop
			case shared(siblings[k-1].Task):
				wantErr = ErrSharedID
			default:
				newParent = siblings[k-1].Task
			}
			_, err = ctx.NestUnderPrevious(strconv.Itoa(i))
		case "level":
			moved = n.Task
			if n.Parent == nil {
				wantErr = ErrTopLevel
			} else if n.Parent.Parent != nil {
				newParent = n.Parent.Parent.Task
			}
			_, err = ctx.UnnestOneLevel(strconv.Itoa(i))
		}

		after := repo.files["work"]
		fail := func(format string, args ...any) {
			t.Helper()
			var b strings.Builder
			for _, task := range before {
				b.WriteString("  " + raws[task] + "\n")
			}
			t.Fatalf("%s: %s\nfile:\n%sbefore:\n%safter:\n%s", label, strings.TrimSpace(sprintfT(format, args...)),
				b.String(), outline(tree), outline(NewTree(after, nil)))
		}
		if wantErr != nil || err != nil {
			if !errors.Is(err, wantErr) {
				fail("error %v, want %v", err, wantErr)
			}
			if !slices.Equal(after, stored) {
				fail("a refused operation changed the file")
			}
			for _, task := range before {
				if task.Raw != raws[task] {
					fail("a refused operation changed %q", raws[task])
				}
			}
			continue
		}

		// Every task once, plus the one added.
		want := slices.Clone(before)
		if op == "add" {
			want = append(want, moved)
		}
		if len(after) != len(want) {
			fail("%d tasks, want %d", len(after), len(want))
		}
		for _, task := range want {
			if !slices.Contains(after, task) {
				fail("%q is gone", task.Raw)
			}
		}

		// Only the moved task's and its new parent's tags changed.
		for _, task := range before {
			if task == moved || task == newParent {
				if !slices.Equal(nonTagWords(task), words[task]) {
					fail("%q became %q", raws[task], task.Raw)
				}
				continue
			}
			if task.Raw != raws[task] {
				fail("%q, which wasn't moved, became %q", raws[task], task.Raw)
			}
		}

		// The moved task is where the design puts it, with its subtree.
		newTree := NewTree(after, nil)
		newParents := parents(newTree)
		if newParents[moved] != newParent {
			fail("%q is under %v, want %v", moved.Raw, newParents[moved], newParent)
		}
		if op != "add" {
			for _, s := range n.Subtree()[1:] {
				if newParents[s.Task] != oldParent[s.Task] {
					fail("%q left its parent in the moved subtree", s.Task.Raw)
				}
			}
		}
		for _, task := range before {
			if task != moved && ok[task] && newParents[task] != oldParent[task] {
				fail("%q moved from under %v to under %v", task.Raw, oldParent[task], newParents[task])
			}
		}
		if newParent != nil && op != "level" && oldParent[moved] != newParent {
			// Under its new parent: last of its priority, as a task
			// added there is. (Nesting a task under its own parent
			// changes nothing.)
			siblings := newTree.Nodes[slices.Index(after, newParent)].Children
			k := slices.IndexFunc(siblings, func(s *Node) bool { return s.Task == moved })
			if k+1 < len(siblings) && siblings[k+1].Task.Priority == moved.Priority {
				fail("%q isn't last of its priority under %q", moved.Raw, newParent.Raw)
			}
		}
		if newParent == nil && op != "add" {
			// At the top level: right after the subtree it was in.
			anchor := n
			for op == "unnest" && anchor.Parent != nil || op == "level" && anchor == n {
				anchor = anchor.Parent
			}
			if anchor != nil && anchor != n {
				sub := newTree.Nodes[slices.Index(after, anchor.Task)].Subtree()
				last := sub[len(sub)-1].Task
				if pos := slices.Index(after, moved); pos == 0 || after[pos-1] != last {
					fail("%q isn't right after %q's subtree", moved.Raw, anchor.Task.Raw)
				}
			}
		}
	}
}

// Moving a subtree to another context: the subtree goes, every line
// with it, its links kept and its ids clear of the target's; nothing
// else changes in either file.
func TestMove_Properties(t *testing.T) {
	r := rand.New(rand.NewPCG(71, 9))
	for range 800 {
		repo := newFakeRepo()
		repo.files["work"] = randomTreeTasks(t, r, 20)
		repo.files["home"] = randomTreeTasks(t, r, 15)
		ctx, _ := repo.Context("work")
		if len(ctx.Tasks) == 0 {
			continue
		}
		source := slices.Clone(ctx.Tasks)
		target := slices.Clone(repo.files["home"])
		raws := map[*Task]string{}
		words := map[*Task][]string{}
		for _, task := range append(slices.Clone(source), target...) {
			raws[task], words[task] = task.Raw, nonTagWords(task)
		}
		taken := map[string]bool{}
		for _, task := range target {
			for _, v := range append(task.annotationValues(idKey), task.ParentIDs()...) {
				taken[v] = true
			}
		}
		tree := NewTree(source, nil)
		oldParent := parents(tree)
		i := r.IntN(len(source))
		sub := subtreeTasks(tree.Nodes[i])

		moved, err := ctx.Move(strconv.Itoa(i), "home")
		fail := func(format string, args ...any) {
			t.Helper()
			t.Fatalf("moving %d: %s\nsource:\n%starget before:\n%starget after:\n%s", i, sprintfT(format, args...),
				outline(tree), outline(NewTree(target, nil)), outline(NewTree(repo.files["home"], nil)))
		}
		if err != nil {
			fail("%v", err)
		}
		if !slices.Equal(moved, sub) {
			fail("moved %d tasks, the subtree has %d", len(moved), len(sub))
		}
		for _, task := range source {
			inSub := slices.Contains(sub, task)
			if inSub == slices.Contains(repo.files["work"], task) || inSub != slices.Contains(repo.files["home"], task) {
				fail("%q is in the wrong file", raws[task])
			}
			if !inSub && task.Raw != raws[task] {
				fail("%q, which stayed, changed", raws[task])
			}
			if inSub && !slices.Equal(nonTagWords(task), words[task]) {
				fail("%q became %q", raws[task], task.Raw)
			}
		}
		for _, task := range target {
			if task.Raw != raws[task] || !slices.Contains(repo.files["home"], task) {
				fail("%q, in the target, changed or went", raws[task])
			}
		}
		if len(repo.files["home"]) != len(target)+len(sub) {
			fail("the target has %d tasks, want %d", len(repo.files["home"]), len(target)+len(sub))
		}

		// The moved task is at the top level, without a p:; its ids clear
		// of the target's; every link below it kept.
		if len(sub[0].ParentIDs()) != 0 {
			fail("the moved task kept a p:: %q", sub[0].Raw)
		}
		for _, task := range sub {
			if id := task.ID(); id != "" && taken[id] {
				fail("%q's id clashes with the target's", task.Raw)
			}
		}
		newParents := parents(NewTree(repo.files["home"], nil))
		if newParents[sub[0]] != nil {
			fail("the moved task isn't at the top level")
		}
		for _, task := range sub[1:] {
			if newParents[task] != oldParent[task] {
				fail("%q lost its parent", task.Raw)
			}
		}
	}
}

// Deleting and completing take what the design says, and leave the rest.
func TestDeleteAndComplete_Properties(t *testing.T) {
	r := rand.New(rand.NewPCG(83, 2))
	for range 600 {
		repo := newFakeRepo()
		repo.files["work"] = randomTreeTasks(t, r, 20)
		ctx, _ := repo.Context("work")
		if len(ctx.Tasks) == 0 {
			continue
		}
		source := slices.Clone(ctx.Tasks)
		tree := NewTree(source, nil)
		i := r.IntN(len(source))
		sub := subtreeTasks(tree.Nodes[i])
		var open []*Task
		for _, task := range sub[1:] {
			if !task.Completed {
				open = append(open, task)
			}
		}

		var gone []*Task
		var err error
		var want []*Task
		switch r.IntN(3) {
		case 0:
			gone, err = ctx.Delete(strconv.Itoa(i))
			want = sub
		case 1:
			gone, err = ctx.Complete(strconv.Itoa(i), SubtasksLeave)
			want = sub[:1]
		default:
			gone, err = ctx.Complete(strconv.Itoa(i), SubtasksComplete)
			want = append(sub[:1:1], open...)
		}
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(gone, want) {
			t.Fatalf("took %d tasks, want %d", len(gone), len(want))
		}
		left := repo.files["work"]
		for _, task := range source {
			if slices.Contains(want, task) == slices.Contains(left, task) {
				t.Fatalf("%q in the wrong place", task.Raw)
			}
		}
		if len(left)+len(want) != len(source) {
			t.Fatalf("%d left of %d, %d taken", len(left), len(source), len(want))
		}
	}
}

func sprintfT(format string, args ...any) string {
	return strings.TrimSpace(fmt.Sprintf(format, args...))
}

// reaches reports whether following first p: links up from from, each
// to the one task holding the id, gets to to: giving to a p: naming
// from would close a loop.
func reaches(tasks []*Task, from, to *Task) bool {
	seen := map[*Task]bool{}
	for t := from; !seen[t]; {
		if t == to {
			return true
		}
		seen[t] = true
		ps := t.ParentIDs()
		if len(ps) == 0 {
			return false
		}
		var holder []*Task
		for _, h := range tasks {
			if h.ID() == ps[0] {
				holder = append(holder, h)
			}
		}
		if len(holder) != 1 {
			return false
		}
		t = holder[0]
	}
	return false
}
