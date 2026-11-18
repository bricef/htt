package cli

import (
	"fmt"
	"maps"
	"os"

	"github.com/bricef/htt/internal/domain"
)

// warningRepository is the Repository CLI commands use (see repo). It
// loads through the Repository it wraps and, for each context a
// command loads, warns about the lines with dates htt can't read
// (Task.DateWarnings): "warning: <file>:<line>: <message>" on stderr.
// A warning changes nothing else. The line has loaded, the command
// goes on, stdout is untouched and the exit code is the command's.
//
// Only a command's own loads warn. A context the domain loads to
// append to (done.txt when a task is completed, the archive when one
// is deleted, the target of a move) comes through the wrapped
// Repository, and the archive never warns: it holds deleted tasks,
// whose dates nothing reads.
type warningRepository struct{ domain.Repository }

func (r warningRepository) Context(name string) (*domain.Context, error) {
	ctx, err := r.Repository.Context(name)
	if err == nil {
		warnContext(r.Repository, ctx, ctx.Tasks)
	}
	return ctx, err
}

func (r warningRepository) CurrentContext() (*domain.Context, error) {
	ctx, err := r.Repository.CurrentContext()
	if err == nil {
		warnContext(r.Repository, ctx, ctx.Tasks)
	}
	return ctx, err
}

func (r warningRepository) Contexts() ([]*domain.Context, error) {
	ctxs, err := r.Repository.Contexts()
	for _, ctx := range ctxs {
		warnContext(r.Repository, ctx, ctx.Tasks)
	}
	return ctxs, err
}

// warnTypedTask warns about the dates htt can't read in a task the user
// typed (todo add, add-to, replace, edit), once it's saved in ctx, so a
// bad date is pointed out as it's written rather than by the next
// command that reads the file. Saving set the task's FileLine. The
// warning counts as given for that line: a later load in the same run
// (the REPL reloads after every command) doesn't repeat it.
func warnTypedTask(ctx *domain.Context, task *domain.Task) {
	warnContext(baseRepo(), ctx, []*domain.Task{task})
}

// warnTypedEntry is warnTypedTask for a time-log entry the user typed
// (log add), once it's saved in tl.
func warnTypedEntry(tl *domain.Timelog, entry *domain.Task) {
	file := timelogRepo().LogPath(tl.Date)
	if file == "" {
		file = "timelog " + tl.Date.Format("2006-01-02") // a repository without files, in tests
	}
	warnDates(file, []*domain.Task{entry})
}

// warnContext warns about tasks, lines of ctx. The archive never warns.
func warnContext(r domain.Repository, ctx *domain.Context, tasks []*domain.Task) {
	if ctx.Name == domain.ArchiveContextName {
		return
	}
	file := r.ContextPath(ctx.Name)
	if file == "" {
		file = ctx.Name // a repository without files, in tests
	}
	warnDates(file, tasks)
}

// datesWarned holds the date warnings already given in this run, by
// file, task text and message, so a line warns once however often it
// is loaded: `htt report` reads done.txt twice, and the REPL reloads
// the context after every command, which may have moved the line.
var datesWarned = map[string]bool{}

// warnDates prints the date warnings of tasks, lines of file, that
// this run hasn't given yet.
func warnDates(file string, tasks []*domain.Task) {
	// Two identical lines in one file both warn; on later loads,
	// neither does.
	given := map[string]bool{}
	for _, task := range tasks {
		for _, w := range task.DateWarnings() {
			key := file + "\n" + task.Entry() + "\n" + w
			if datesWarned[key] {
				continue
			}
			given[key] = true
			fmt.Fprintf(os.Stderr, "warning: %s:%d: %s\n", file, task.FileLine, w)
		}
	}
	maps.Copy(datesWarned, given)
}
