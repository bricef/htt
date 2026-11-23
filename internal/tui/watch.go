package tui

import (
	"errors"
	"os"
	"slices"
	"time"

	"github.com/bricef/htt/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
)

// Watching (docs/design/safe-writes.md, "Seeing changes"): while the
// TUI is open, another htt or an editor can change the file it shows.
// Each second it looks at the file, a stat, and when it has changed
// since the last look, reloads the list and redraws it, the selection
// on the same task, by its text, where it's still there. This is about
// the view: what an action changes is found and saved in a transaction
// of its own (onShown), whatever the view shows.

// watchEvery is how often the TUI looks at the file it shows.
const watchEvery = time.Second

// watchMsg says it's time to look at the file again.
type watchMsg struct{}

// watchTick asks for the next look.
func watchTick() tea.Cmd {
	return tea.Tick(watchEvery, func(time.Time) tea.Msg { return watchMsg{} })
}

// fileLook is what a look at the file shown saw: its path and what
// the file system said of it, nil info when it doesn't exist.
type fileLook struct {
	path string
	info os.FileInfo
}

// look looks at the file the list shows. Its path is "" for a
// repository without files (the memory repository), which isn't
// watched.
func (m *model) look() fileLook {
	path := m.repo.ContextPath(m.context.Name)
	if path == "" {
		return fileLook{}
	}
	info, err := os.Stat(path)
	if err != nil {
		return fileLook{path: path}
	}
	return fileLook{path: path, info: info}
}

// same reports whether two looks saw the same file unchanged: the same
// file (htt replaces a file with a new one, which os.SameFile tells
// apart), of the same size, modified at the same time.
func (l fileLook) same(o fileLook) bool {
	if l.path != o.path || (l.info == nil) != (o.info == nil) {
		return false
	}
	if l.info == nil {
		return true
	}
	return os.SameFile(l.info, o.info) && l.info.Size() == o.info.Size() && l.info.ModTime().Equal(o.info.ModTime())
}

// watched is a look at the file the list shows, on a tick: when it has
// changed since the last look, the list is reloaded from it. Then the
// next look is asked for.
func (m model) watched() (tea.Model, tea.Cmd) {
	now := m.look()
	if now.path == "" {
		return m, nil
	}
	if !now.same(m.seen) {
		m.seen = now
		if err := m.reloadShown(); err != nil {
			m.logger.Error("reload the list shown", "file", now.path, "err", err)
		}
	}
	return m, watchTick()
}

// reloadShown loads the list shown from its file and, if its lines have
// changed, shows it, the selection on the task it was on, found by its
// text (shownAt), or where it was when that task is gone.
func (m *model) reloadShown() error {
	ctx, err := m.repo.Context(m.context.Name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if slices.EqualFunc(ctx.Tasks, m.context.Tasks, func(a, b *domain.Task) bool { return a.Raw == b.Raw }) {
		return nil
	}
	at := -1
	if it, ok := m.selectedTask(); ok {
		at = shownAt(ctx.Tasks, it.index, it.task.Raw)
	}
	m.context = ctx
	m.showTasks(at)
	return nil
}
