package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/bricef/htt/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
)

// errUnavailable marks an action that doesn't apply in the current
// view, such as completing a task in the done tab. Act logs these as
// warnings rather than errors.
var errUnavailable = errors.New("not available here")

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
		return fmt.Errorf("reload current context: %w", err)
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

var Noop = mkAction("noop", func(m model) (tea.Model, tea.Cmd, error) { return m, nil, nil })

var Up = mkAction("move up", func(m model) (tea.Model, tea.Cmd, error) {
	if m.cursor > 0 {
		m.cursor--
	}
	return m, nil, nil
})

var Down = mkAction("move down", func(m model) (tea.Model, tea.Cmd, error) {
	if m.cursor < len(m.context.Tasks)-1 {
		m.cursor++
	}
	return m, nil, nil
})

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
	m.cursor = 0
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
	m.cursor = 0
	return m, nil, nil
})

var Do = mkAction("do", func(m model) (tea.Model, tea.Cmd, error) {
	// Doing something in the "done" view doesn't make sense — bail out.
	if m.context.Name == domain.DoneContextName {
		return m, tea.Quit, fmt.Errorf("%w: tasks in the done tab are already complete", errUnavailable)
	}
	if _, err := m.context.Complete(strID(m.cursor)); err != nil {
		return m, tea.Quit, fmt.Errorf("complete task %d: %w", m.cursor, err)
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit, err
	}
	clampCursor(&m)
	return m, nil, nil
})

var Quit = mkAction("quit", func(m model) (tea.Model, tea.Cmd, error) {
	return m, tea.Quit, nil
})

var Help = mkAction("toggle help", func(m model) (tea.Model, tea.Cmd, error) {
	m.showHelp = !m.showHelp
	return m, nil, nil
})

var EditFile = mkAction("edit file", func(m model) (tea.Model, tea.Cmd, error) {
	// Opens the context file in $EDITOR. tea.ExecProcess hands the
	// terminal over: Bubble Tea leaves the alt screen and stops
	// reading keys until the editor exits, then sends
	// editorFinishedMsg. Nothing here closes the TUI: when the editor
	// can't run, the status line says why and the user carries on.
	// The repo owns path resolution; an empty path means the repo
	// isn't file-backed (e.g. memory repo in tests).
	path := m.repo.ContextPath(m.context.Name)
	if path == "" {
		m.status = fmt.Sprintf("%s has no file to edit.", m.context.Name)
		return m, nil, fmt.Errorf("%w: context %s has no file to edit", errUnavailable, m.context.Name)
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		m.status = "Set $EDITOR to edit files from here."
		return m, nil, fmt.Errorf("%w: $EDITOR is not set", errUnavailable)
	}
	return m, tea.ExecProcess(exec.Command(editor, path), func(err error) tea.Msg {
		return editorFinishedMsg{editor: editor, err: err}
	}), nil
})

// editorFinishedMsg reports that the editor EditFile started has
// exited, or failed to start. err is nil after a clean exit.
type editorFinishedMsg struct {
	editor string
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
		m.status = fmt.Sprintf("$EDITOR exited with status %d; showing the file as it is on disk.", exitErr.ExitCode())
		m.logger.Error("editor exited with an error", "editor", msg.editor, "err", msg.err)
	default:
		m.status = fmt.Sprintf("Couldn't run $EDITOR: %v", msg.err)
		m.logger.Error("editor failed to run", "editor", msg.editor, "err", msg.err)
	}
	if err := refresh(&m); err != nil {
		m.logger.Error("reload after editing failed", "err", err)
		return m, tea.Quit
	}
	clampCursor(&m)
	return m, tea.ClearScreen
}

var NewTask = mkAction("new task", func(m model) (tea.Model, tea.Cmd, error) {
	m.newTask = true
	m.textInput.Focus()
	m.textInput.Cursor.Blink = true
	return m, tea.ClearScreen, nil
})

var CommandMode = mkAction("command mode", func(m model) (tea.Model, tea.Cmd, error) {
	return m, tea.ClearScreen, nil
})

var CancelNewTask = mkAction("cancel new task", func(m model) (tea.Model, tea.Cmd, error) {
	m.newTask = false
	m.textInput.SetValue("")
	m.textInput.Blur()
	return m, tea.ClearScreen, nil
})

var AddTask = mkAction("add task", func(m model) (tea.Model, tea.Cmd, error) {
	m.newTask = false
	value := m.textInput.Value()
	m.textInput.SetValue("")
	m.textInput.Blur()
	if value == "" {
		return m, tea.ClearScreen, nil
	}
	// bug_008: Adding an uncompleted task to the done context produces
	// a phantom entry (no `x ` prefix) that renders as completed but
	// parses as not completed — file/view divergence with no recovery
	// path from inside the TUI. The Do action has the same guard; this
	// just mirrors it for AddTask.
	if m.context.Name == domain.DoneContextName {
		return m, tea.ClearScreen, fmt.Errorf("%w: new tasks can't go in the done tab", errUnavailable)
	}
	// Add to the visible context regardless of what the repo considers
	// "current". The TUI's NextContext action syncs the two (calls
	// SetCurrent on every move), but we name the target context
	// explicitly here for clarity.
	target, err := m.repo.Context(m.context.Name)
	if err != nil {
		return m, tea.ClearScreen, fmt.Errorf("load %s: %w", m.context.Name, err)
	}
	task, err := domain.NewTask(value)
	if err != nil {
		return m, tea.ClearScreen, fmt.Errorf("parse task %q: %w", value, err)
	}
	if err := target.AddTask(task); err != nil {
		return m, tea.ClearScreen, fmt.Errorf("add task: %w", err)
	}
	if err := refresh(&m); err != nil {
		return m, tea.ClearScreen, err
	}
	return m, tea.ClearScreen, nil
})

var Delete = mkAction("delete", func(m model) (tea.Model, tea.Cmd, error) {
	if _, err := m.context.Delete(strID(m.cursor)); err != nil {
		return m, tea.Quit, fmt.Errorf("delete task %d: %w", m.cursor, err)
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit, err
	}
	clampCursor(&m)
	return m, tea.ClearScreen, nil
})

var IncreasePriority = mkAction("increase priority", func(m model) (tea.Model, tea.Cmd, error) {
	if _, _, err := m.context.IncreasePriority(strID(m.cursor)); err != nil {
		return m, tea.Quit, fmt.Errorf("raise priority of task %d: %w", m.cursor, err)
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit, err
	}
	m.context.Sort()
	return m, tea.ClearScreen, nil
})

var DecreasePriority = mkAction("decrease priority", func(m model) (tea.Model, tea.Cmd, error) {
	if _, _, err := m.context.DecreasePriority(strID(m.cursor)); err != nil {
		return m, tea.Quit, fmt.Errorf("lower priority of task %d: %w", m.cursor, err)
	}
	if err := refresh(&m); err != nil {
		return m, tea.Quit, err
	}
	m.context.Sort()
	return m, tea.ClearScreen, nil
})
