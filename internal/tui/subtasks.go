package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/bricef/htt/internal/domain"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Subtasks in the TUI (docs/design/subtasks.md): the tree, unfolded by
// default; Space folds and unfolds; s adds a subtask; Tab nests a task
// under the task above it, Shift-Tab unnests it a level; x on a task
// with open subtasks asks first, as `htt todo do` does.

// question is the question x asks about a task's open subtasks, until
// the next key answers it.
type question struct {
	index int      // the task's index
	raw   string   // its line when asked, to be sure it's the task answered about
	open  []string // the lines of its open subtasks when asked: the ones the answer is about
}

// treeChange reports why a subtask action left the tree as it was, on
// the status line, and returns the error Act logs.
func (m *model) treeChange(index int, err error) error {
	reasons := map[error]string{
		domain.ErrNotATree:     "The done tab doesn't hold subtasks.",
		domain.ErrNothingAbove: "Task %d is the first at its level: there's nothing above it to nest it under.",
		domain.ErrTopLevel:     "Task %d is already a top-level task.",
		domain.ErrLoop:         "Task %d can't go there: it would be below itself.",
		domain.ErrSharedID:     "Task %d can't go there: the task above has an id another task holds too, so nothing can name it.",
		domain.ErrOnlyLink:     "Task %d has no text besides its link: at the top level it would be empty.",
	}
	for sentinel, reason := range reasons {
		if errors.Is(err, sentinel) {
			if strings.Contains(reason, "%d") {
				reason = fmt.Sprintf(reason, index)
			}
			m.status = reason
			return fmt.Errorf("%w: %w", errUnavailable, err)
		}
	}
	return nil
}

// arranged shows the tree after a subtask action moved task, which the
// list then selects, unfolding what's above it.
func (m *model) arranged(task *domain.Task) error {
	if err := refresh(m); err != nil {
		return err
	}
	// Saving arranged the context and numbered task in it, in the
	// order the reload has too.
	m.unfoldTo(task.Line)
	m.showTasks(task.Line)
	return nil
}

// NestTask is Tab: the selected task, with its subtree, under the task
// above it at its level.
var NestTask = mkAction("nest under the task above", func(m model) (tea.Model, tea.Cmd, error) {
	it, ok := m.selectedTask()
	if !ok {
		return m, nil, errNoTaskSelected
	}
	shownAbove := taskAbove(m.context, it.index)
	errAbove := fmt.Errorf("%w: the task above task %d changed elsewhere", errUnavailable, it.index)
	at := -1
	var moved *domain.Task
	err := m.onShown(it.index, it.task.Raw, changedElsewhere(it.index, "nothing was moved"), func(c *domain.Context, i int) error {
		at = i
		if taskAbove(c, at) != shownAbove {
			return errAbove
		}
		var err error
		moved, err = c.NestUnderPrevious(strID(at))
		return err
	})
	if errors.Is(err, errAbove) {
		m.status = fmt.Sprintf("The task above task %d was changed or moved elsewhere; nothing was moved.", it.index)
		return m, nil, err
	}
	if err != nil {
		if errors.Is(err, errGone) {
			return m, quitUnless(err), err
		}
		if unavailable := m.treeChange(at, err); unavailable != nil {
			return m, nil, unavailable
		}
		return m, quitUnless(err), fmt.Errorf("nest task %d: %w", at, err)
	}
	if err := m.arranged(moved); err != nil {
		return m, tea.Quit, err
	}
	return m, nil, nil
})

// UnnestTask is Shift-Tab: the selected task, with its subtree, a level
// up, after its old parent's subtree.
var UnnestTask = mkAction("unnest a level", func(m model) (tea.Model, tea.Cmd, error) {
	it, ok := m.selectedTask()
	if !ok {
		return m, nil, errNoTaskSelected
	}
	at := -1
	var moved *domain.Task
	err := m.onShown(it.index, it.task.Raw, changedElsewhere(it.index, "nothing was moved"), func(c *domain.Context, i int) error {
		at = i
		var err error
		moved, err = c.UnnestOneLevel(strID(at))
		return err
	})
	if err != nil {
		if errors.Is(err, errGone) {
			return m, quitUnless(err), err
		}
		if unavailable := m.treeChange(at, err); unavailable != nil {
			return m, nil, unavailable
		}
		return m, quitUnless(err), fmt.Errorf("unnest task %d: %w", at, err)
	}
	if err := m.arranged(moved); err != nil {
		return m, tea.Quit, err
	}
	return m, nil, nil
})

// taskAbove is the line of the task Tab would nest the task of c with
// the given index under (domain.Context.TaskAbove); "" when there's
// none.
func taskAbove(c *domain.Context, index int) string {
	above, err := c.TaskAbove(strID(index))
	if err != nil || above == nil {
		return ""
	}
	return above.Raw
}

// ToggleFold is Space: folds the selected task's subtree out of the
// list, or unfolds it. Folds last the session.
var ToggleFold = mkAction("fold or unfold", func(m model) (tea.Model, tea.Cmd, error) {
	it, ok := m.selectedTask()
	if !ok {
		return m, nil, errNoTaskSelected
	}
	tree := domain.NewTree(m.context.Tasks, nil)
	if len(tree.Nodes[it.index].Children) == 0 {
		m.status = fmt.Sprintf("Task %d has no subtasks to fold.", it.index)
		return m, nil, fmt.Errorf("%w: task %d has no subtasks", errUnavailable, it.index)
	}
	k := m.foldKey(it.task)
	if m.folded[k] {
		delete(m.folded, k)
	} else {
		m.folded[k] = true
	}
	m.showTasks(it.index)
	return m, nil, nil
})

// NewSubtask is s: the task input, for a subtask of the selected task.
var NewSubtask = mkAction("new subtask", func(m model) (tea.Model, tea.Cmd, error) {
	if !m.context.IsTree() {
		m.status = "The done tab doesn't hold subtasks."
		return m, nil, fmt.Errorf("%w: %w", errUnavailable, domain.ErrNotATree)
	}
	it, ok := m.selectedTask()
	if !ok {
		return m, nil, errNoTaskSelected
	}
	m.openInput(inputSubtask, "", it.index)
	return m, tea.ClearScreen, nil
})

// ask puts the question about the open subtasks of the task with the
// given index on the status line, for the next key to answer (answer).
func (m *model) ask(index int, open []*domain.Task) {
	m.asking = &question{index: index, raw: m.context.Tasks[index].Raw, open: lineOf(open)}
	m.status = strings.Join(domain.SubtasksQuestion(strID(index), len(open)), "\n")
}

func lineOf(tasks []*domain.Task) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.Raw
	}
	return out
}

// answer takes the key pressed after the question: x completes the task
// alone, A with every open task below it, any other key cancels (Ctrl-C
// quits, as everywhere).
func answer(m model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	q := m.asking
	m.asking = nil
	choice := domain.SubtasksAsk
	switch msg.String() {
	case "x":
		choice = domain.SubtasksLeave
	case "A":
		choice = domain.SubtasksComplete
	case "ctrl+c":
		return m, tea.Quit
	}
	if choice == domain.SubtasksAsk {
		return cancelCompleting.Act(m)
	}
	return completeAnswered(*q, choice).Act(m)
}

// cancelCompleting answers the question with any key but x and A.
var cancelCompleting = mkAction("cancel completing", func(m model) (tea.Model, tea.Cmd, error) {
	m.status = "Cancelled: nothing changed."
	return m, nil, nil
})

// completeAnswered completes the task q was about, as answered, in the
// file as it is now, provided the task and its open subtasks are still
// the ones asked about.
func completeAnswered(q question, choice domain.Subtasks) *Action {
	return mkAction("complete, subtasks answered", func(m model) (tea.Model, tea.Cmd, error) {
		errOpen := fmt.Errorf("%w: task %d's open subtasks changed elsewhere", errUnavailable, q.index)
		var done []*domain.Task
		err := m.onShown(q.index, q.raw, changedElsewhere(q.index, "nothing was completed"), func(c *domain.Context, at int) error {
			open, err := c.OpenSubtasks(strID(at))
			if err != nil {
				return fmt.Errorf("complete task %d: %w", at, err)
			}
			if !slices.Equal(lineOf(open), q.open) {
				return errOpen
			}
			if done, err = c.Complete(strID(at), choice); err != nil {
				return fmt.Errorf("complete task %d: %w", at, err)
			}
			return nil
		})
		if errors.Is(err, errOpen) {
			m.status = fmt.Sprintf("Task %d's open subtasks changed elsewhere; nothing was completed.", q.index)
		}
		if err != nil {
			return m, quitUnless(err), err
		}
		if err := refresh(&m); err != nil {
			return m, tea.Quit, err
		}
		m.showTasks(-1)
		if len(done) > 1 {
			m.status = fmt.Sprintf("Completed task %d and %s.", q.index, plural(len(done)-1, "open subtask"))
		} else {
			m.status = fmt.Sprintf("Completed task %d; its subtasks stay open.", q.index)
		}
		return m, nil, nil
	})
}

// plural says "1 open subtask" or "2 open subtasks".
func plural(n int, thing string) string {
	if n == 1 {
		return "1 " + thing
	}
	return fmt.Sprintf("%d %ss", n, thing)
}

// questionKeys are the keys that answer the question, for the help bar.
type questionKeys struct{}

func (questionKeys) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "done, subtasks left open")),
		key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "done with its subtasks")),
		key.NewBinding(key.WithKeys("other"), key.WithHelp("other keys", "cancel")),
	}
}

func (k questionKeys) FullHelp() [][]key.Binding { return [][]key.Binding{k.ShortHelp()} }
