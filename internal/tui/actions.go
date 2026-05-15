package tui

import (
	"fmt"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/utils"
	tea "github.com/charmbracelet/bubbletea"
)

type Action struct {
	act         func(m model) (tea.Model, tea.Cmd)
	description string
}

func (a *Action) Act(m model) (tea.Model, tea.Cmd) {
	return a.act(m)
}

func mkAction(description string, f func(m model) (tea.Model, tea.Cmd)) *Action {
	return &Action{act: f, description: description}
}

// strID converts the integer cursor position into the form the Context
// methods expect (string IDs, since the CLI passes os.Args).
func strID(i int) string {
	return fmt.Sprintf("%d", i)
}

// refresh reloads the current context from the repo and assigns it to m.
// Used after every mutating action so the rendered view reflects what's
// on disk.
func refresh(m *model) error {
	ctx, err := m.repo.CurrentContext()
	if err != nil {
		return err
	}
	m.context = ctx
	return nil
}

// clampCursor keeps m.cursor inside the bounds of m.context.Tasks after
// an action shortens the list (bug_013). Without this, deleting or
// completing the last task leaves the cursor one past the end, and the
// next mutating keypress (d/x/+/-) routes to GetTaskById with an
// out-of-range index, errors, and tea.Quits the TUI unexpectedly.
func clampCursor(m *model) {
	n := len(m.context.Tasks)
	if n == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
}

var Noop = mkAction("noop", func(m model) (tea.Model, tea.Cmd) { return m, nil })

var Up = mkAction("move up", func(m model) (tea.Model, tea.Cmd) {
	if m.cursor > 0 {
		m.cursor--
	}
	return m, nil
})

var Down = mkAction("move down", func(m model) (tea.Model, tea.Cmd) {
	if m.cursor < len(m.context.Tasks)-1 {
		m.cursor++
	}
	return m, nil
})

var NextContext = mkAction("move right", func(m model) (tea.Model, tea.Cmd) {
	if m.contextCursor < len(m.contexts)-1 {
		m.contextCursor++
	}
	newName := m.contexts[m.contextCursor].Name
	if err := m.repo.SetCurrent(newName); err != nil {
		return m, tea.Quit
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit
	}
	m.cursor = 0
	return m, nil
})

var PreviousContext = mkAction("move left", func(m model) (tea.Model, tea.Cmd) {
	if m.contextCursor > 0 {
		m.contextCursor--
	}
	newName := m.contexts[m.contextCursor].Name
	if err := m.repo.SetCurrent(newName); err != nil {
		return m, tea.Quit
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit
	}
	m.cursor = 0
	return m, nil
})

var Do = mkAction("do", func(m model) (tea.Model, tea.Cmd) {
	// Doing something in the "done" view doesn't make sense — bail out.
	if m.context.Name == domain.DoneContextName {
		return m, tea.Quit
	}
	if _, err := m.context.Complete(strID(m.cursor)); err != nil {
		return m, tea.Quit
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit
	}
	clampCursor(&m)
	return m, nil
})

var Quit = mkAction("quit", func(m model) (tea.Model, tea.Cmd) {
	return m, tea.Quit
})

var Help = mkAction("toggle help", func(m model) (tea.Model, tea.Cmd) {
	m.showHelp = !m.showHelp
	return m, nil
})

var EditFile = mkAction("edit file", func(m model) (tea.Model, tea.Cmd) {
	// Shells out to $EDITOR on the context file. The repo owns path
	// resolution; an empty path means the repo isn't file-backed
	// (e.g. memory repo in tests) and editing isn't supported. After
	// the editor exits we refresh the displayed context from the repo
	// (the editor may have added or removed tasks).
	path := m.repo.ContextPath(m.context.Name)
	if path == "" {
		return m, tea.Quit
	}
	utils.EditFilePath(path)
	if err := refresh(&m); err != nil {
		return m, tea.Quit
	}
	return m, tea.ClearScreen
})

var NewTask = mkAction("new task", func(m model) (tea.Model, tea.Cmd) {
	m.newTask = true
	m.textInput.Focus()
	m.textInput.Cursor.Blink = true
	return m, tea.ClearScreen
})

var CommandMode = mkAction("command mode", func(m model) (tea.Model, tea.Cmd) {
	return m, tea.ClearScreen
})

var CancelNewTask = mkAction("cancel new task", func(m model) (tea.Model, tea.Cmd) {
	m.newTask = false
	m.textInput.SetValue("")
	m.textInput.Blur()
	return m, tea.ClearScreen
})

var AddTask = mkAction("add task", func(m model) (tea.Model, tea.Cmd) {
	m.newTask = false
	value := m.textInput.Value()
	m.textInput.SetValue("")
	m.textInput.Blur()
	if value == "" {
		return m, tea.ClearScreen
	}
	// bug_008: Adding an uncompleted task to the done context produces
	// a phantom entry (no `x ` prefix) that renders as completed but
	// parses as not completed — file/view divergence with no recovery
	// path from inside the TUI. The Do action has the same guard; this
	// just mirrors it for AddTask.
	if m.context.Name == domain.DoneContextName {
		return m, tea.ClearScreen
	}
	// Add to the visible context regardless of what the repo considers
	// "current". The TUI's NextContext action syncs the two (calls
	// SetCurrent on every move), but we name the target context
	// explicitly here for clarity.
	target, err := m.repo.Context(m.context.Name)
	if err != nil {
		return m, tea.ClearScreen
	}
	task, err := domain.NewTask(value)
	if err != nil {
		return m, tea.ClearScreen
	}
	if err := target.AddTask(task); err != nil {
		return m, tea.ClearScreen
	}
	if err := refresh(&m); err != nil {
		return m, tea.ClearScreen
	}
	return m, tea.ClearScreen
})

var Delete = mkAction("delete", func(m model) (tea.Model, tea.Cmd) {
	if _, err := m.context.Delete(strID(m.cursor)); err != nil {
		return m, tea.Quit
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit
	}
	clampCursor(&m)
	return m, tea.ClearScreen
})

var IncreasePriority = mkAction("increase priority", func(m model) (tea.Model, tea.Cmd) {
	if _, _, err := m.context.IncreasePriority(strID(m.cursor)); err != nil {
		return m, tea.Quit
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit
	}
	m.context.Sort()
	return m, tea.ClearScreen
})

var DecreasePriority = mkAction("decrease priority", func(m model) (tea.Model, tea.Cmd) {
	if _, _, err := m.context.DecreasePriority(strID(m.cursor)); err != nil {
		return m, tea.Quit
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit
	}
	m.context.Sort()
	return m, tea.ClearScreen
})
