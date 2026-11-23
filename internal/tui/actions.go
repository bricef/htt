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

// errGone is an action's on a task the TUI showed that isn't in the
// file any more (onShown).
var errGone = fmt.Errorf("%w: the task shown changed elsewhere", errUnavailable)

// onShown runs act, for an action that changes tasks, on the task the
// TUI showed at index as raw: as one transaction
// (domain.Context.Transact) on the list's file as it is when the key is
// pressed, act getting the context and the task's index in it, found by
// its text (shownAt). What act saves is saved together, over nothing
// older; if another program changes the file before it's saved, act
// runs again on the new file, finding the task again. When the task
// isn't there any more, changed or removed elsewhere (the CLI, say),
// nothing changes: the status line says so (changed), and the error is
// errGone. A save that fails (another htt holding the files too long, a
// file that keeps changing, a full disk or a read-only directory) is
// said on the status line too (failed), and the TUI stays open, as it
// does after any action that fails: nothing was saved, and every action
// works on the file as it is, so trying again can't save over anything.
// Either way the list shows the file as the transaction left it, on the
// task found.
func (m *model) onShown(index int, raw, changed string, act func(c *domain.Context, at int) error) error {
	found := -1
	err := m.context.Transact(func(c *domain.Context) error {
		if found = shownAt(c.Tasks, index, raw); found < 0 {
			return errGone
		}
		return act(c, found)
	})
	m.showTasks(found)
	switch {
	case err == nil:
	case errors.Is(err, errGone):
		m.status = changed
	case !errors.Is(err, errUnavailable): // an action that can't run says why itself
		m.status = failed(err)
	}
	return err
}

// failed is what the status line says of an action that failed with
// err, having saved nothing: another htt busy with the files, as the
// domain says it, or the error, and that nothing was saved.
func failed(err error) string {
	switch {
	case errors.Is(err, domain.ErrBusy):
		return sentence(domain.ErrBusy)
	case errors.Is(err, domain.ErrKeepsChanging):
		return sentence(err) // "<file> keeps changing; nothing was saved."
	}
	return sentence(err) + " Nothing was saved."
}

// sentence is err's message as a sentence, for the status line.
func sentence(err error) string {
	s := err.Error()
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:] + "."
}

// shownAt finds the task the TUI showed at index as raw among tasks,
// the file as it is now, as the CLI's commands that wait for the user
// find theirs (domain.FindTask): still at index, or, if the file moved
// it, the one task with that line. -1 when it isn't there, or several
// tasks have its line and none is at index.
func shownAt(tasks []*domain.Task, index int, raw string) int {
	return domain.FindTask(tasks, index, raw)
}

// changedElsewhere says on the status line that task index changed
// elsewhere, and what didn't happen because of it.
func changedElsewhere(index int, nothing string) string {
	return fmt.Sprintf("Task %d was changed or removed elsewhere; %s.", index, nothing)
}

// refresh reloads the context the TUI shows from the repo, by its name,
// as the watch does: another htt switching the current context (the
// pointer) doesn't switch the TUI's. Used after every mutating action
// so the rendered view reflects what's on disk. The file is looked at
// first (watch.go), so the next tick doesn't take the action's own
// write for a change to reload, and a write landing between the look
// and the read is seen at the next one.
func refresh(m *model) error {
	seen := m.look()
	ctx, err := m.repo.Context(m.context.Name)
	if err != nil {
		return fmt.Errorf("reload %s: %w", m.context.Name, err)
	}
	m.context, m.seen = ctx, seen
	return nil
}

// switchTo shows the context of the tab at index: loaded by its name,
// and made the current context (the pointer, which the CLI follows).
func switchTo(m *model, index int) error {
	name := m.contexts[index].Name
	seen := m.lookAt(name)
	ctx, err := m.repo.Context(name)
	if err != nil {
		return fmt.Errorf("load %s: %w", name, err)
	}
	if err := m.repo.SetCurrent(name); err != nil {
		return fmt.Errorf("switch to %s: %w", name, err)
	}
	m.contextCursor, m.context, m.seen = index, ctx, seen
	m.showTasks(-1)
	m.list.Select(0)
	return nil
}

var Noop = mkAction("noop", func(m model) (tea.Model, tea.Cmd, error) { return m, nil, nil })

// NextContext and PreviousContext switch tabs. A switch that fails
// (another htt busy with the files, the pointer's directory read-only)
// says so on the status line and stays on the tab shown.
var NextContext = mkAction("move right", func(m model) (tea.Model, tea.Cmd, error) {
	if err := switchTo(&m, min(m.contextCursor+1, len(m.contexts)-1)); err != nil {
		m.status = failed(err)
		return m, nil, err
	}
	return m, nil, nil
})

var PreviousContext = mkAction("move left", func(m model) (tea.Model, tea.Cmd, error) {
	if err := switchTo(&m, max(m.contextCursor-1, 0)); err != nil {
		m.status = failed(err)
		return m, nil, err
	}
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
	var at int
	var open []*domain.Task
	err := m.onShown(it.index, it.task.Raw, changedElsewhere(it.index, "nothing was completed"), func(c *domain.Context, i int) error {
		var err error
		at = i
		if open, err = c.OpenSubtasks(strID(at)); err != nil {
			return fmt.Errorf("complete task %d: %w", at, err)
		}
		if len(open) > 0 {
			return nil // asked first: nothing changes yet
		}
		if _, err := c.Complete(strID(at), domain.SubtasksAsk); err != nil {
			return fmt.Errorf("complete task %d: %w", at, err)
		}
		return nil
	})
	if err != nil {
		return m, nil, err
	}
	if len(open) > 0 {
		m.ask(at, open) // the next key answers (see answer)
		return m, nil, nil
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
	task, err := domain.NewTask(d.raw)
	if err != nil {
		return m, nil, fmt.Errorf("%w: %v", errUnavailable, err)
	}
	at := -1
	err = m.onShown(m.editIndex, m.editRaw, "That task changed while you were editing it; nothing was saved.", func(c *domain.Context, i int) error {
		if d.raw == m.editRaw {
			return nil // unchanged: nothing to save
		}
		at = i
		_, err := c.Replace(strID(at), task)
		return err
	})
	if errors.Is(err, errGone) {
		m.closeInput()
		return m, nil, err
	}
	if err != nil {
		m.status = fmt.Sprintf("Couldn't save the task: %v", err)
		return m, nil, fmt.Errorf("replace task %d: %w", at, err)
	}
	if at < 0 {
		m.closeInput()
		return m, tea.ClearScreen, nil
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
	if m.input == inputSubtask && !m.editGone && len(d.problems) == 0 {
		// The check the preview makes. A task a reload found gone is
		// looked for again below, in the file as it is now.
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
	// explicitly here for clarity. Adding is a transaction on the file
	// as it is now, so nothing written elsewhere since the list was
	// loaded is lost.
	target := m.context
	if m.input == inputSubtask {
		// Under the task the input was opened for, wherever it is now.
		at := -1
		err := m.onShown(m.editIndex, m.editRaw, changedElsewhere(m.editIndex, "the subtask wasn't added"), func(c *domain.Context, i int) error {
			at = i
			// A task of this run's own: a run before may have given
			// the last one a p:.
			fresh, err := domain.NewTask(d.raw)
			if err != nil {
				return err
			}
			task = fresh
			return c.AddSubtask(strID(at), task)
		})
		if errors.Is(err, errGone) {
			m.closeInput()
			return m, tea.ClearScreen, err
		}
		if err != nil {
			m.status = fmt.Sprintf("Couldn't save the subtask: %v", err)
			return m, nil, fmt.Errorf("add subtask of task %d: %w", at, err)
		}
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
	var gone []*domain.Task
	err := m.onShown(it.index, it.task.Raw, changedElsewhere(it.index, "nothing was deleted"), func(c *domain.Context, at int) error {
		var err error
		if gone, err = c.Delete(strID(at)); err != nil {
			return fmt.Errorf("delete task %d: %w", at, err)
		}
		return nil
	})
	if err != nil {
		return m, nil, err
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
	var before, changed *domain.Task
	err := m.onShown(it.index, it.task.Raw, changedElsewhere(it.index, "its priority wasn't changed"), func(c *domain.Context, at int) error {
		var err error
		if before, changed, err = c.IncreasePriority(strID(at)); err != nil {
			return fmt.Errorf("raise priority of task %d: %w", at, err)
		}
		return nil
	})
	if err != nil {
		return m, nil, err
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
	var before, changed *domain.Task
	err := m.onShown(it.index, it.task.Raw, changedElsewhere(it.index, "its priority wasn't changed"), func(c *domain.Context, at int) error {
		var err error
		if before, changed, err = c.DecreasePriority(strID(at)); err != nil {
			return fmt.Errorf("lower priority of task %d: %w", at, err)
		}
		return nil
	})
	if err != nil {
		return m, nil, err
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit, err
	}
	m.showTasks(indexOf(m.context, changed))
	m.status = datedNote(before, changed)
	return m, tea.ClearScreen, nil
})
