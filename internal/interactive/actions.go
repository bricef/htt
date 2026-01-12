package interactive

import (
	"time"

	"github.com/bricef/htt/internal/todo"
	"github.com/bricef/htt/internal/utils"
	tea "github.com/charmbracelet/bubbletea"
)

// High level refactor, actions should be bare functions which return (tea.Model, tea.Cmd)
// and close over the model
// type Action func() (tea.Model, tea.Cmd)
// optionally, we could have actions have a .Do() method instead

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

//	var Up = mkAction("move up", func(m model) (tea.Model, tea.Cmd) {
//		if m.cursor > 0 {
//			m.cursor--
//		}
//		return m, nil
//	})
//
//	var Down = mkAction("move down", func(m model) (tea.Model, tea.Cmd) {
//		if m.cursor < len(m.context.Tasks)-1 {
//			m.cursor++
//		}
//		return m, nil
//	})
var NextContext = mkAction("move right", func(m model) (tea.Model, tea.Cmd) {
	if m.contextCursor < len(m.contexts)-1 {
		m.contextCursor++
	}
	new_context := m.contexts[m.contextCursor].Name
	todo.SetCurrentContext(new_context)
	m.context = todo.GetCurrentContext()
	m.cursor = 0
	m.list = NewTaskList(m.context, m)
	// Update focused to point to the new list if it was pointing to the old one
	if _, ok := m.focused.(*TaskList); ok {
		m.focused = &m.list
	}
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
	m.list = NewTaskList(m.context, m)
	// Update focused to point to the new list if it was pointing to the old one
	if _, ok := m.focused.(*TaskList); ok {
		m.focused = &m.list
	}
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
	m.Focus(m.prompt)
	return m, tea.ClearScreen
})

var CommandMode = mkAction("command mode", func(m model) (tea.Model, tea.Cmd) {
	return m, tea.ClearScreen
})

var CancelNewTask = mkAction("cancel new task", func(m model) (tea.Model, tea.Cmd) {
	m.prompt.ti.SetValue("")
	m.prompt.ti.Blur()
	return m, tea.ClearScreen
})

type AddedTaskMsg struct {
	task *todo.Task
}

var AddTask = func(t string) tea.Cmd {
	return func() tea.Msg {
		task := todo.NewTask(t)
		return AddedTaskMsg{task}
	}
}

// var AddTask = mkAction("add task", func(m model) (tea.Model, tea.Cmd) {
// 	// t := m.prompt.GetValue()
// 	// if t == "" {
// 	// 	m.prompt.Reset()
// 	// 	m.Focus(m.list)
// 	// 	return m, tea.ClearScreen
// 	// }
// 	// m.context.Add(todo.NewTask(t))
// 	// m.context.Sync()
// 	// m.prompt.Reset()
// 	// m.Focus(m.list)
// 	if m.prompt.ti.Value() == "" {

// 		m.prompt.ti.SetValue("")
// 		m.prompt.ti.Blur()
// 		return m, tea.ClearScreen
// 	}

// 	task := todo.NewTask(m.prompt.ti.Value())
// 	m.context.Add(task)
// 	m.context.Sync()
// 	m.prompt.ti.SetValue("")
// 	m.prompt.ti.Blur()
// 	return m, tea.ClearScreen
// })

var Delete = mkAction("delete", func(m model) (tea.Model, tea.Cmd) {
	t, err := m.context.GetTaskById(m.cursor)
	if err != nil {
		return m, tea.Quit
	}
	err = m.context.Remove(t)
	if err != nil {
		return m, tea.Quit
	}
	m.context.Sync()
	return m, tea.ClearScreen
})

var IncreasePriority = mkAction("increase priority", func(m model) (tea.Model, tea.Cmd) {
	t, err := m.context.GetTaskById(m.cursor)
	if err != nil {
		return m, tea.Quit
	}
	t = t.IncreasePriority()
	m.context.Replace(t, t)
	m.context.Sort()
	m.context.Sync()
	return m, tea.ClearScreen
})

var DecreasePriority = mkAction("decrease priority", func(m model) (tea.Model, tea.Cmd) {
	t, err := m.context.GetTaskById(m.cursor)
	if err != nil {
		return m, tea.Quit
	}
	t = t.DecreasePriority()
	m.context.Replace(t, t)
	m.context.Sort()
	m.context.Sync()
	return m, tea.ClearScreen
})
