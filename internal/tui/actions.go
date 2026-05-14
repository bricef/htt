package tui

import (
	"fmt"

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

// strID converts the integer cursor position into the form the use cases
// expect (they use strconv.Atoi internally because the CLI layer passes
// strings from os.Args).
func strID(i int) string {
	return fmt.Sprintf("%d", i)
}

// refresh reloads the current context from the repo and assigns it to m.
// Used after every mutating action so the rendered view reflects what's
// on disk.
func refresh(m *model) error {
	ctx, err := m.uc.CurrentContext()
	if err != nil {
		return err
	}
	m.context = ctx
	return nil
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
	if _, err := m.uc.SwitchContext(newName); err != nil {
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
	if _, err := m.uc.SwitchContext(newName); err != nil {
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
	if m.context.Name == "done" {
		return m, tea.Quit
	}
	if _, err := m.uc.CompleteTask(strID(m.cursor)); err != nil {
		return m, tea.Quit
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit
	}
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
	// Shells out to $EDITOR on the context file. Filepath() builds the
	// path string from viper-backed config; it doesn't perform I/O, so
	// it survived the Step 10 cull. After the editor exits we refresh
	// the displayed context via the use case (the editor may have added
	// or removed tasks).
	utils.EditFilePath(m.context.Filepath())
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
	// AddTaskTo writes to the visible context regardless of what the repo
	// considers "current". The TUI's NextContext action syncs the two
	// (calls SwitchContext on every move), but we name the target context
	// explicitly here for clarity.
	if _, _, err := m.uc.AddTaskTo(m.context.Name, value); err != nil {
		return m, tea.ClearScreen
	}
	if err := refresh(&m); err != nil {
		return m, tea.ClearScreen
	}
	return m, tea.ClearScreen
})

var Delete = mkAction("delete", func(m model) (tea.Model, tea.Cmd) {
	if _, err := m.uc.DeleteTask(strID(m.cursor)); err != nil {
		return m, tea.Quit
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit
	}
	return m, tea.ClearScreen
})

var IncreasePriority = mkAction("increase priority", func(m model) (tea.Model, tea.Cmd) {
	if _, _, err := m.uc.IncreasePriority(strID(m.cursor)); err != nil {
		return m, tea.Quit
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit
	}
	m.context.Sort()
	return m, tea.ClearScreen
})

var DecreasePriority = mkAction("decrease priority", func(m model) (tea.Model, tea.Cmd) {
	if _, _, err := m.uc.DecreasePriority(strID(m.cursor)); err != nil {
		return m, tea.Quit
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit
	}
	m.context.Sort()
	return m, tea.ClearScreen
})
