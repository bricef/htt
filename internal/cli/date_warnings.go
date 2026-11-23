package cli

import (
	"fmt"
	"maps"
	"os"
	"time"

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

// warningTimelogRepository is the TimelogRepository CLI commands use
// (see timelogRepo): warningRepository for time logs. Each day's log a
// command loads warns about the entries with dates htt can't read, in
// the same form and with the same once-per-run record. An entry's ts:
// timestamp is htt's own, not a date Task.DateWarnings reads, so it
// never warns. Timelog.Append and AppendAt save through the wrapped
// repository.
type warningTimelogRepository struct{ domain.TimelogRepository }

func (r warningTimelogRepository) Today() (*domain.Timelog, error) {
	tl, err := r.TimelogRepository.Today()
	if err == nil {
		warnTimelog(r.TimelogRepository, tl, tl.Entries)
	}
	return tl, err
}

func (r warningTimelogRepository) Day(date time.Time) (*domain.Timelog, error) {
	tl, err := r.TimelogRepository.Day(date)
	if err == nil {
		warnTimelog(r.TimelogRepository, tl, tl.Entries)
	}
	return tl, err
}

// warnTypedTask warns about the dates htt can't read in a task the user
// typed (todo add, add-to, replace, edit), once it's saved in ctx, so a
// bad date is pointed out as it's written rather than by the next
// command that reads the file, and about the links the change broke
// (a p: typed naming no task, an id: edited away from a parent). Saving
// set the task's FileLine. The warning counts as given for that line:
// a later load in the same run (the REPL reloads after every command)
// doesn't repeat it.
func warnTypedTask(ctx *domain.Context, task *domain.Task) {
	r := baseRepo()
	file := contextFile(r, ctx)
	notes := dateNotes([]*domain.Task{task})
	links := linkProblems(file, ctx)
	for _, t := range ctx.Tasks {
		for _, msg := range links[t] {
			notes = append(notes, note{t, msg})
		}
	}
	warnOnce(file, notes)
}

// warnTypedEntry is warnTypedTask for a time-log entry the user typed
// (log add), once it's saved in tl.
func warnTypedEntry(tl *domain.Timelog, entry *domain.Task) {
	warnTimelog(timelogRepo(), tl, []*domain.Task{entry})
}

// warnTimelog warns about entries, lines of tl.
func warnTimelog(r domain.TimelogRepository, tl *domain.Timelog, entries []*domain.Task) {
	file := r.LogPath(tl.Date)
	if file == "" {
		file = "timelog " + tl.Date.Format("2006-01-02") // a repository without files, in tests
	}
	warnDates(file, entries)
}

// warnContext warns about tasks, lines of ctx: the dates htt can't
// read, and the broken links of subtasks (domain.LinkProblem), each
// task's in turn. The archive never warns.
func warnContext(r domain.Repository, ctx *domain.Context, tasks []*domain.Task) {
	if ctx.Name == domain.ArchiveContextName {
		return
	}
	file := contextFile(r, ctx)
	links := linkProblems(file, ctx)
	var notes []note
	for _, task := range tasks {
		notes = append(notes, dateNotes([]*domain.Task{task})...)
		for _, msg := range links[task] {
			notes = append(notes, note{task, msg})
		}
	}
	warnOnce(file, notes)
}

// contextFile names ctx's file in warnings.
func contextFile(r domain.Repository, ctx *domain.Context) string {
	if file := r.ContextPath(ctx.Name); file != "" {
		return file
	}
	return ctx.Name // a repository without files, in tests
}

// linkProblems returns the broken links of ctx's tasks, by task. The
// tree reads the done file when a link needs it (Context.Tree); if it
// can't, that's a warning of its own, and the links aren't checked.
func linkProblems(file string, ctx *domain.Context) map[*domain.Task][]string {
	tree, err := ctx.Tree()
	if err != nil {
		key := file + "\n\n" + err.Error()
		if !datesWarned[key] {
			datesWarned[key] = true
			fmt.Fprintf(os.Stderr, "warning: %s: subtasks not checked: %v\n", file, err)
		}
		return nil
	}
	out := map[*domain.Task][]string{}
	for _, p := range tree.Problems {
		task := ctx.Tasks[p.Index]
		out[task] = append(out[task], p.Message)
	}
	return out
}

// note is one warning about one task.
type note struct {
	task    *domain.Task
	message string
}

// dateNotes are the date warnings of tasks (Task.DateWarnings).
func dateNotes(tasks []*domain.Task) []note {
	var out []note
	for _, task := range tasks {
		for _, w := range task.DateWarnings() {
			out = append(out, note{task, w})
		}
	}
	return out
}

// datesWarned holds the warnings already given in this run, by file,
// task text and message, so a line warns once however often it is
// loaded: `htt report` reads done.txt twice, and the REPL reloads the
// context after every command, which may have moved the line.
var datesWarned = map[string]bool{}

// warnDates prints the date warnings of tasks, lines of file, that
// this run hasn't given yet.
func warnDates(file string, tasks []*domain.Task) {
	warnOnce(file, dateNotes(tasks))
}

// warnOnce prints the notes about lines of file that this run hasn't
// given yet: "warning: <file>:<line>: <message>" on stderr.
func warnOnce(file string, notes []note) {
	// Two identical lines in one file both warn; on later loads,
	// neither does.
	given := map[string]bool{}
	for _, n := range notes {
		key := file + "\n" + n.task.Entry() + "\n" + n.message
		if datesWarned[key] {
			continue
		}
		given[key] = true
		fmt.Fprintf(os.Stderr, "warning: %s:%d: %s\n", file, n.task.FileLine, n.message)
	}
	maps.Copy(datesWarned, given)
}
