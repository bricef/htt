package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/editor"
	tea "github.com/charmbracelet/bubbletea"
)

// errUnavailable marks an action that doesn't apply in the current
// view, such as completing a task in the done tab. Act logs these as
// warnings rather than errors.
var errUnavailable = errors.New("not available here")

// errNoTaskSelected is what task actions report when the list (or the
// search) shows no task to act on.
var errNoTaskSelected = fmt.Errorf("%w: no task selected", errUnavailable)

// indexOf finds a task in the context by its text, for following a task
// that a change has moved (a priority change re-sorts the context); -1
// when it isn't there.
func indexOf(ctx *domain.Context, task *domain.Task) int {
	return slices.IndexFunc(ctx.Tasks, func(t *domain.Task) bool { return t.Raw == task.Raw })
}

type Action struct {
	act         func(m model) (tea.Model, tea.Cmd, error)
	description string
}

// Act runs the action and records it in the TUI log: every action at
// debug, an action that doesn't apply in the current view at warn, and
// a failure at error. The action has already chosen what the TUI does
// next (quit or carry on); Act doesn't change that.
func (a *Action) Act(m model) (tea.Model, tea.Cmd) {
	m.logger.Debug("action", "action", a.description)
	next, cmd, err := a.act(m)
	switch {
	case err == nil:
	case errors.Is(err, errUnavailable):
		m.logger.Warn("action unavailable", "action", a.description, "reason", err)
	default:
		m.logger.Error("action failed", "action", a.description, "err", err)
	}
	return next, cmd
}

func mkAction(description string, f func(m model) (tea.Model, tea.Cmd, error)) *Action {
	return &Action{act: f, description: description}
}

// strID converts a task index into the form the Context methods expect
// (string IDs, since the CLI passes os.Args).
func strID(i int) string {
	return fmt.Sprintf("%d", i)
}

// reload loads the list on show from its file again, for an action
// that changes tasks, and finds in it the task the TUI showed at index
// as raw (shownAt). The action then works on the file as it is when the
// key is pressed, so it saves nothing older, on the task the TUI
// showed; the list shows the reloaded file. When that task isn't there
// any more, changed or removed elsewhere (the CLI, say), nothing
// changes: the status line says so (changed), and the error is
// errUnavailable's.
func (m *model) reload(index int, raw, changed string) (int, error) {
	ctx, err := m.repo.Context(m.context.Name)
	if err != nil {
		return -1, fmt.Errorf("reload %s: %w", m.context.Name, err)
	}
	m.context = ctx
	at := shownAt(ctx.Tasks, index, raw)
	m.showTasks(at)
	if at < 0 {
		m.status = changed
		return -1, fmt.Errorf("%w: task %d changed elsewhere", errUnavailable, index)
	}
	return at, nil
}

// shownAt finds the task the TUI showed at index as raw among tasks,
// the file as it is now: still at index, or, if the file moved it, the
// one task with that line. -1 when it isn't there, or several tasks
// have its line and none is at index.
func shownAt(tasks []*domain.Task, index int, raw string) int {
	if index >= 0 && index < len(tasks) && tasks[index].Raw == raw {
		return index
	}
	at := -1
	for i, t := range tasks {
		if t.Raw == raw {
			if at >= 0 {
				return -1
			}
			at = i
		}
	}
	return at
}

// changedElsewhere says on the status line that task index changed
// elsewhere, and what didn't happen because of it.
func changedElsewhere(index int, nothing string) string {
	return fmt.Sprintf("Task %d was changed or removed elsewhere; %s.", index, nothing)
}

// quitUnless is what an action that failed with err does next: quit,
// unless err is errUnavailable's, when the TUI stays open.
func quitUnless(err error) tea.Cmd {
	if errors.Is(err, errUnavailable) {
		return nil
	}
	return tea.Quit
}

// refresh reloads the current context from the repo and assigns it to m.
// Used after every mutating action so the rendered view reflects what's
// on disk.
func refresh(m *model) error {
	ctx, err := m.repo.CurrentContext()
	if err != nil {
		return fmt.Errorf("reload current context: %w", err)
	}
	m.context = ctx
	return nil
}

var Noop = mkAction("noop", func(m model) (tea.Model, tea.Cmd, error) { return m, nil, nil })

var NextContext = mkAction("move right", func(m model) (tea.Model, tea.Cmd, error) {
	if m.contextCursor < len(m.contexts)-1 {
		m.contextCursor++
	}
	newName := m.contexts[m.contextCursor].Name
	if err := m.repo.SetCurrent(newName); err != nil {
		return m, tea.Quit, fmt.Errorf("switch to %s: %w", newName, err)
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit, err
	}
	m.showTasks(-1)
	m.list.Select(0)
	return m, nil, nil
})

var PreviousContext = mkAction("move left", func(m model) (tea.Model, tea.Cmd, error) {
	if m.contextCursor > 0 {
		m.contextCursor--
	}
	newName := m.contexts[m.contextCursor].Name
	if err := m.repo.SetCurrent(newName); err != nil {
		return m, tea.Quit, fmt.Errorf("switch to %s: %w", newName, err)
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit, err
	}
	m.showTasks(-1)
	m.list.Select(0)
	return m, nil, nil
})

var Do = mkAction("do", func(m model) (tea.Model, tea.Cmd, error) {
	// Doing something in the "done" view doesn't make sense — bail out.
	if m.context.Name == domain.DoneContextName {
		return m, tea.Quit, fmt.Errorf("%w: tasks in the done tab are already complete", errUnavailable)
	}
	it, ok := m.selectedTask()
	if !ok {
		return m, nil, errNoTaskSelected
	}
	at, err := m.reload(it.index, it.task.Raw, changedElsewhere(it.index, "nothing was completed"))
	if err != nil {
		return m, quitUnless(err), err
	}
	open, err := m.context.OpenSubtasks(strID(at))
	if err != nil {
		return m, tea.Quit, fmt.Errorf("complete task %d: %w", at, err)
	}
	if len(open) > 0 {
		m.ask(at, open) // the next key answers (see answer)
		return m, nil, nil
	}
	if _, err := m.context.Complete(strID(at), domain.SubtasksAsk); err != nil {
		return m, tea.Quit, fmt.Errorf("complete task %d: %w", at, err)
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit, err
	}
	m.showTasks(-1)
	return m, nil, nil
})

var Quit = mkAction("quit", func(m model) (tea.Model, tea.Cmd, error) {
	return m, tea.Quit, nil
})

var Help = mkAction("toggle help", func(m model) (tea.Model, tea.Cmd, error) {
	m.showHelp = !m.showHelp
	return m, nil, nil
})

// execProcess is tea.ExecProcess. Tests replace it to see the command
// EditFile hands to Bubble Tea, without running an editor.
var execProcess = tea.ExecProcess

var EditFile = mkAction("edit file", func(m model) (tea.Model, tea.Cmd, error) {
	// Opens the context file in the user's editor, which
	// internal/editor picks and builds the command for as git does
	// ($VISUAL, then $EDITOR). tea.ExecProcess hands the terminal
	// over: Bubble Tea leaves the alt screen and stops reading keys
	// until the editor exits, then sends editorFinishedMsg. Nothing
	// here closes the TUI: when the editor can't run, the status line
	// says why and the user carries on. The repo owns path
	// resolution; an empty path means the repo isn't file-backed
	// (e.g. memory repo in tests).
	path := m.repo.ContextPath(m.context.Name)
	if path == "" {
		m.status = fmt.Sprintf("%s has no file to edit.", m.context.Name)
		return m, nil, fmt.Errorf("%w: context %s has no file to edit", errUnavailable, m.context.Name)
	}
	ed, err := editor.Choose(os.Getenv)
	if err != nil {
		m.status = fmt.Sprintf("Can't edit: %v.", err)
		return m, nil, fmt.Errorf("%w: %w", errUnavailable, err)
	}
	cmd, err := ed.Command(path)
	if err != nil {
		m.status = fmt.Sprintf("Couldn't run the editor: %v", err)
		return m, nil, err
	}
	return m, execProcess(cmd, func(err error) tea.Msg {
		return editorFinishedMsg{editor: ed, err: err}
	}), nil
})

// editorFinishedMsg reports that the editor EditFile started has
// exited, or failed to start. err is nil after a clean exit.
type editorFinishedMsg struct {
	editor editor.Editor
	err    error
}

// editorFinished reloads the edited context and reports how the editor
// session went. The file is reloaded even after an editor error: the
// editor may have written it anyway, and the view must match disk. A
// failed reload still quits, as other actions do; staying open on a
// stale context would let the next keypress save over the edits.
func editorFinished(m model, msg editorFinishedMsg) (tea.Model, tea.Cmd) {
	var exitErr *exec.ExitError
	switch {
	case msg.err == nil:
	case errors.As(msg.err, &exitErr):
		m.status = fmt.Sprintf("$%s exited with status %d; showing the file as it is on disk.", msg.editor.Var, exitErr.ExitCode())
		m.logger.Error("editor exited with an error", "editor", msg.editor.Value, "err", msg.err)
	default:
		m.status = fmt.Sprintf("Couldn't run $%s: %v", msg.editor.Var, msg.err)
		m.logger.Error("editor failed to run", "editor", msg.editor.Value, "err", msg.err)
	}
	if err := refresh(&m); err != nil {
		m.logger.Error("reload after editing failed", "err", err)
		return m, tea.Quit
	}
	m.showTasks(-1)
	return m, tea.ClearScreen
}

var NewTask = mkAction("new task", func(m model) (tea.Model, tea.Cmd, error) {
	if m.context.Name == domain.DoneContextName {
		m.status = "New tasks go in a list, not the done tab."
		return m, nil, fmt.Errorf("%w: new tasks can't go in the done tab", errUnavailable)
	}
	m.openInput(inputAdd, "", -1)
	return m, tea.ClearScreen, nil
})

// EditTask opens the selected task in the task input; Enter saves it
// in place (ReplaceTask).
var EditTask = mkAction("edit task", func(m model) (tea.Model, tea.Cmd, error) {
	if m.context.Name == domain.DoneContextName {
		m.status = "Tasks in the done tab can't be edited here; f opens the file."
		return m, nil, fmt.Errorf("%w: tasks in the done tab can't be edited", errUnavailable)
	}
	it, ok := m.selectedTask()
	if !ok {
		return m, nil, errNoTaskSelected
	}
	m.openInput(inputEdit, it.task.Raw, it.index)
	return m, tea.ClearScreen, nil
})

// ReplaceTask saves the edited task over the one EditTask opened, as
// the preview shows it. Anything that stops the save keeps the input
// open with the text as typed.
var ReplaceTask = mkAction("replace task", func(m model) (tea.Model, tea.Cmd, error) {
	d := readDraft(m.textInput.Value(), m.now(), false)
	if d.raw == "" {
		m.status = "A task can't be empty: esc cancels the edit, d deletes the task."
		return m, nil, fmt.Errorf("%w: empty task", errUnavailable)
	}
	if len(d.problems) > 0 {
		return m, nil, fmt.Errorf("%w: %s", errUnavailable, d.problems[0])
	}
	at, err := m.reload(m.editIndex, m.editRaw, "That task changed while you were editing it; nothing was saved.")
	if errors.Is(err, errUnavailable) {
		m.closeInput()
		return m, nil, err
	}
	if err != nil {
		m.status = fmt.Sprintf("Couldn't save the task: %v", err)
		return m, nil, err
	}
	if d.raw == m.editRaw {
		m.closeInput()
		return m, tea.ClearScreen, nil
	}
	task, err := domain.NewTask(d.raw)
	if err != nil {
		return m, nil, fmt.Errorf("%w: %v", errUnavailable, err)
	}
	if _, err := m.context.Replace(strID(at), task); err != nil {
		m.status = fmt.Sprintf("Couldn't save the task: %v", err)
		return m, nil, fmt.Errorf("replace task %d: %w", at, err)
	}
	m.closeInput()
	if err := refresh(&m); err != nil {
		return m, tea.Quit, err
	}
	m.showTasks(indexOf(m.context, task))
	return m, tea.ClearScreen, nil
})

var CommandMode = mkAction("command mode", func(m model) (tea.Model, tea.Cmd, error) {
	return m, tea.ClearScreen, nil
})

// CancelNewTask closes the task input, new or edit, saving nothing.
var CancelNewTask = mkAction("cancel task input", func(m model) (tea.Model, tea.Cmd, error) {
	m.closeInput()
	return m, tea.ClearScreen, nil
})

// AddTask saves the new task in the input to the current context, as
// the preview shows it. Anything that stops the save keeps the input
// open with the text as typed.
var AddTask = mkAction("add task", func(m model) (tea.Model, tea.Cmd, error) {
	// bug_008: Adding an uncompleted task to the done context produces
	// a phantom entry (no `x ` prefix) that renders as completed but
	// parses as not completed — file/view divergence with no recovery
	// path from inside the TUI. The Do action has the same guard; this
	// just mirrors it for AddTask.
	if m.context.Name == domain.DoneContextName {
		m.closeInput()
		return m, tea.ClearScreen, fmt.Errorf("%w: new tasks can't go in the done tab", errUnavailable)
	}
	d := readDraft(m.textInput.Value(), m.now(), true)
	if d.raw == "" {
		m.closeInput()
		return m, tea.ClearScreen, nil
	}
	if m.input == inputSubtask && len(d.problems) == 0 {
		_, d.problems = m.subtaskDraft(d.raw)
	}
	if len(d.problems) > 0 {
		return m, nil, fmt.Errorf("%w: %s", errUnavailable, d.problems[0])
	}
	// d.raw already carries the created date, so AddTask keeps it as is.
	task, err := domain.NewTask(d.raw)
	if err != nil {
		return m, nil, fmt.Errorf("%w: %v", errUnavailable, err)
	}
	// Add to the visible context regardless of what the repo considers
	// "current". The TUI's NextContext action syncs the two (calls
	// SetCurrent on every move), but we name the target context
	// explicitly here for clarity. It's loaded as the file is now, so
	// nothing written elsewhere since the list was loaded is lost.
	var target *domain.Context
	if m.input == inputSubtask {
		// Under the task the input was opened for, wherever it is now.
		at, err := m.reload(m.editIndex, m.editRaw, changedElsewhere(m.editIndex, "the subtask wasn't added"))
		if errors.Is(err, errUnavailable) {
			m.closeInput()
			return m, tea.ClearScreen, err
		}
		if err != nil {
			m.status = fmt.Sprintf("Couldn't save the subtask: %v", err)
			return m, nil, err
		}
		target = m.context
		if err := target.AddSubtask(strID(at), task); err != nil {
			m.status = fmt.Sprintf("Couldn't save the subtask: %v", err)
			return m, nil, fmt.Errorf("add subtask of task %d: %w", at, err)
		}
	} else if target, err = m.repo.Context(m.context.Name); err != nil {
		m.status = fmt.Sprintf("Couldn't save the task: %v", err)
		return m, nil, fmt.Errorf("load %s: %w", m.context.Name, err)
	} else if err := target.AddTask(task); err != nil {
		m.status = fmt.Sprintf("Couldn't save the task: %v", err)
		return m, nil, fmt.Errorf("add task: %w", err)
	}
	m.closeInput()
	if err := refresh(&m); err != nil {
		return m, tea.ClearScreen, err
	}
	// Saving arranged the context: the task is where it was put, in the
	// order the reload has too.
	index := slices.Index(target.Tasks, task)
	m.unfoldTo(index)
	m.showTasks(index)
	return m, tea.ClearScreen, nil
})

var Delete = mkAction("delete", func(m model) (tea.Model, tea.Cmd, error) {
	it, ok := m.selectedTask()
	if !ok {
		return m, nil, errNoTaskSelected
	}
	at, err := m.reload(it.index, it.task.Raw, changedElsewhere(it.index, "nothing was deleted"))
	if err != nil {
		return m, quitUnless(err), err
	}
	gone, err := m.context.Delete(strID(at))
	if err != nil {
		return m, tea.Quit, fmt.Errorf("delete task %d: %w", at, err)
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit, err
	}
	m.showTasks(-1)
	if len(gone) > 1 {
		m.status = fmt.Sprintf("Deleted task %d and %s.", it.index, plural(len(gone)-1, "subtask"))
	}
	return m, tea.ClearScreen, nil
})

var IncreasePriority = mkAction("increase priority", func(m model) (tea.Model, tea.Cmd, error) {
	it, ok := m.selectedTask()
	if !ok {
		return m, nil, errNoTaskSelected
	}
	at, err := m.reload(it.index, it.task.Raw, changedElsewhere(it.index, "its priority wasn't changed"))
	if err != nil {
		return m, quitUnless(err), err
	}
	before, changed, err := m.context.IncreasePriority(strID(at))
	if err != nil {
		return m, tea.Quit, fmt.Errorf("raise priority of task %d: %w", at, err)
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit, err
	}
	m.showTasks(indexOf(m.context, changed))
	m.status = datedNote(before, changed)
	return m, tea.ClearScreen, nil
})

// datedNote explains a date a rewrite gave a task: with less in front
// of its text, the text's first word would have been read as the done
// mark, a priority or a date, so the domain put today's date in front
// (see domain's rebuild). "" when no date was added.
func datedNote(before, after *domain.Task) string {
	dated := (before.CreatedOn.IsZero() && !after.CreatedOn.IsZero()) ||
		(before.CompletedOn.IsZero() && !after.CompletedOn.IsZero())
	if !dated {
		return ""
	}
	if first, _, _ := strings.Cut(after.Entry(), " "); first != "" {
		return fmt.Sprintf("Added today's date, so %q stays part of the task.", first)
	}
	return "Added today's date, so the task still reads as it did."
}

var DecreasePriority = mkAction("decrease priority", func(m model) (tea.Model, tea.Cmd, error) {
	it, ok := m.selectedTask()
	if !ok {
		return m, nil, errNoTaskSelected
	}
	at, err := m.reload(it.index, it.task.Raw, changedElsewhere(it.index, "its priority wasn't changed"))
	if err != nil {
		return m, quitUnless(err), err
	}
	before, changed, err := m.context.DecreasePriority(strID(at))
	if err != nil {
		return m, tea.Quit, fmt.Errorf("lower priority of task %d: %w", at, err)
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit, err
	}
	m.showTasks(indexOf(m.context, changed))
	m.status = datedNote(before, changed)
	return m, tea.ClearScreen, nil
})
