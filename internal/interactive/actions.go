package interactive

import (
	"time"

	"github.com/bricef/htt/internal/todo"
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
	new_context := m.contexts[m.contextCursor].Name
	todo.SetCurrentContext(new_context)
	m.context = todo.GetCurrentContext()
	m.cursor = 0
	return m, nil
})
var PreviousContext = mkAction("move left", func(m model) (tea.Model, tea.Cmd) {
	if m.contextCursor > 0 {
		m.contextCursor--
	}
	new_context := m.contexts[m.contextCursor].Name
	todo.SetCurrentContext(new_context)
	m.context = todo.GetCurrentContext()
	m.cursor = 0
	return m, nil
})
var Do = mkAction("do", func(m model) (tea.Model, tea.Cmd) {
	// this makes no sense if the current context is done
	if m.context.Name == "done" {
		return m, tea.Quit
	}

	t, err := m.context.GetTaskById(m.cursor)
	if err != nil {
		return m, tea.Quit
	}
	t = t.Do(m.context, time.Now())
	done := todo.NewContext("done")
	err = todo.Move(t, m.context, done)
	if err != nil {
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
	utils.EditFilePath(m.context.Filepath())
	m.context = m.context.Read()
	return m, tea.ClearScreen
})

var NewTask = mkAction("new task", func(m model) (tea.Model, tea.Cmd) {
	m.newTask = true
	return m, tea.ClearScreen
})

var CommandMode = mkAction("command mode", func(m model) (tea.Model, tea.Cmd) {
	return m, tea.ClearScreen
})
