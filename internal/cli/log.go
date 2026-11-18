package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bricef/htt/internal/dates"
	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/utils"
	"github.com/spf13/cobra"
)

var LogCommand = &cobra.Command{
	Use:     "log",
	Short:   "Manage the activity log.",
	Args:    cobra.NoArgs,
	Aliases: []string{"l"},
}

// atFlag is the --at of a command that writes an entry: the time to log
// it at, if given. An --at given empty is given, so it is refused like
// any other time htt can't read, rather than logging now.
type atFlag struct {
	value string
	given bool
}

func (f *atFlag) String() string { return f.value }
func (f *atFlag) Type() string   { return "time" }
func (f *atFlag) Set(s string) error {
	f.value, f.given = s, true
	return nil
}

// logAt is the --at of the commands that write an entry (log add,
// start and end, and workon). They share it, since only one runs at a
// time; resetReplFlags clears it between the REPL's commands.
var logAt atFlag

// atFlagUsage is --at's line in each command's flags, short enough for
// a terminal: the help (atHelp) says the rest.
const atFlagUsage = `log it at HH:MM (today), "yesterday HH:MM", "YYYY-MM-DD HH:MM" or YYYY-MM-DDTHH:MM`

// atHelp ends the help of each command that writes an entry.
const atHelp = `--at logs the entry at an earlier time instead of now: HH:MM (or
H:MM) today, "yesterday HH:MM", or "YYYY-MM-DD HH:MM", on the 24-hour
clock in your time zone. It also takes an ISO 8601 date and time,
YYYY-MM-DDTHH:MM, with seconds (:SS) if you like: in your time zone,
or with a zone (Z, +05:30) at that instant, so a ts: copied from htt
log show works. The entry goes into that day's log, in time order,
and the confirmation says when. A time later than now is refused,
and nothing is logged.`

// logEntry is the small helper every "write a timelog entry" command
// shares (log add, start and end, and workon), flag is their --at:
// without it, load today's Timelog and append the task; with it, load
// the day of the time it names (dates.ParseAt) and put the task in its
// place there (Timelog.AppendAt), refusing a time later than now. Then
// confirm the entry, without the ts: annotation it was given, and, with
// --at, when it was logged: "Logging entry: @end (at 17:30
// yesterday)". It returns the Timelog, which now holds the task.
func logEntry(task *domain.Task, flag atFlag) (*domain.Timelog, error) {
	if !flag.given {
		tl, err := timelogRepo().Today()
		if err != nil {
			return nil, fmt.Errorf("load today's timelog: %w", err)
		}
		if err := tl.Append(task); err != nil {
			return nil, fmt.Errorf("append log entry: %w", err)
		}
		fmt.Printf("Logging entry: %v\n",
			task.RemoveAnnotation(domain.TimelogTimestampLabel).ColorString())
		return tl, nil
	}

	at, now := flag.value, time.Now()
	when, err := dates.ParseAt(at, now)
	if err != nil {
		return nil, fmt.Errorf("--at: %w", err)
	}
	tl, err := timelogRepo().Day(when)
	if err != nil {
		return nil, fmt.Errorf("load the timelog for %s: %w", when.Format("2006-01-02"), err)
	}
	if err := tl.AppendAt(task, when, now); err != nil {
		if errors.Is(err, domain.ErrFutureEntry) {
			return nil, fmt.Errorf("--at %q is later than now (%s): nothing logged", at, now.Format("15:04"))
		}
		return nil, fmt.Errorf("log entry at %s: %w", when.Format("2006-01-02 15:04"), err)
	}
	fmt.Printf("Logging entry: %v (at %s)\n",
		task.RemoveAnnotation(domain.TimelogTimestampLabel).ColorString(), describeAt(when, now))
	return tl, nil
}

// describeAt says when an entry logged at at runs from, for the
// confirmation: "17:30 today", "17:30 yesterday" or "17:30 on
// 2026-11-10", by at's date on the local calendar, whichever form --at
// named it in.
func describeAt(at, now time.Time) string {
	clock := at.Format("15:04")
	if at.Second() != 0 { // only an ISO --at gives seconds
		clock = at.Format("15:04:05")
	}
	day := func(t time.Time) time.Time { // the date alone, for calendar sums
		y, m, d := t.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	date, today := day(at), day(now)
	switch {
	case date.Equal(today):
		return clock + " today"
	case date.Equal(today.AddDate(0, 0, -1)):
		return clock + " yesterday"
	}
	return clock + " on " + at.Format("2006-01-02")
}

var Add = &cobra.Command{
	Use:   "add [entry]",
	Short: "Log an entry to the time log.",
	Long: `Log what you're doing from now on: the entry runs until the next
one.

` + atHelp + `

Examples:
  htt log add reviewing the auth PR +htt
  htt log add --at 14:00 call with Sam
  htt log add --at "yesterday 12:30" @lunch`,
	Args:    cobra.MinimumNArgs(1),
	Aliases: []string{"+"},
	RunE: func(cmd *cobra.Command, args []string) error {
		task, err := domain.NewTask(strings.Join(args, " "))
		if err != nil {
			return fmt.Errorf("parse log entry: %w", err)
		}
		tl, err := logEntry(task, logAt)
		if err != nil {
			return err
		}
		warnTypedEntry(tl, task)
		return nil
	},
}

var Show = &cobra.Command{
	Use:     "show",
	Short:   "Show today's time log.",
	Args:    cobra.NoArgs,
	Aliases: []string{"s"},
	RunE: func(cmd *cobra.Command, args []string) error {
		tl, err := timelogRepo().Today()
		if err != nil {
			return fmt.Errorf("load today's timelog: %w", err)
		}
		if tl.IsEmpty() {
			fmt.Println("Today's timelog does not yet exist. Add an entry.")
			return nil
		}
		for _, entry := range tl.Entries {
			fmt.Println(entry.Raw)
		}
		return nil
	},
}

var Edit = &cobra.Command{
	Use:     "edit",
	Short:   "Open the current time log file using $EDITOR.",
	Args:    cobra.NoArgs,
	Aliases: []string{"e"},
	RunE: func(cmd *cobra.Command, args []string) error {
		path := timelogRepo().CurrentLogPath()
		if path == "" {
			return fmt.Errorf("active timelog repository does not expose a file path")
		}
		return utils.EditFilePath(path)
	},
}

// todayHelp ends the help of log status and log active, which differ
// only in their first lines.
const todayHelp = `The category is the first in timelog.categories with one of the
entry's tags, or uncategorised when none has one; uncategorised time
counts as work or not as timelog.default_category says (working
unless set). After htt log end it says "Not working (day ended at
HH:MM)".

A timelog config htt can't use doesn't stop it: it warns on stderr
and shows the entry without its category.`

// unendedHelp ends the help of log status and htt status, which flag
// the last day logged before today when it has no @end (see
// printUnendedDay; "two weeks" is statusLookBack).
const unendedHelp = `It also looks at the last day you logged before today, up to two
weeks back. When that day has no @end, so its last entry counts until
midnight, a line after the status says so, with the command that ends
it, for the time you stopped:
  htt log end --at "YYYY-MM-DD HH:MM"`

var Status = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the time log.",
	Long: `Show the entry you're on today, its category and how long it has
run: "Currently working on: <entry> (<category>, <time>)", or "Not
working: <entry> (<category>, <time>)" when the category doesn't
count as work, such as a break.

` + todayHelp + `

` + unendedHelp,
	Args:    cobra.NoArgs,
	Aliases: []string{"?"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return printStatus()
	},
}

var Active = &cobra.Command{
	Use:   "active",
	Short: "Show active task, if any.",
	Long: `Show the entry you're on today, its category and how long it has
run: "Working on: <entry> (<category>, <time>)", or "Not working:
<entry> (<category>, <time>)" when the category doesn't count as
work, such as a break.

` + todayHelp,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return printToday("No active task.", "Working on:")
	},
}

var Start = &cobra.Command{
	Use:   "start",
	Short: "Start the timelog for the day.",
	Long: `Log a @start entry. It is an ordinary entry: the time from it to the
next entry is uncategorised unless a category has the @start tag.

` + atHelp + `

Example, for a day you started at 9:30 and are only now logging:
  htt log start --at 9:30`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		task, err := domain.NewTask("@start")
		if err != nil {
			return fmt.Errorf("build start entry: %w", err)
		}
		_, err = logEntry(task, logAt)
		return err
	},
}

var End = &cobra.Command{
	Use:   "end",
	Short: "End the day in the time log.",
	Long: `Log an @end entry, which ends the day: the entry before it stops
there. An entry logged after it starts again, and the time between
them counts as nothing.

A day without an @end counts its last entry until midnight, and
htt report flags it. To end it after the fact, log the @end at the
time you stopped, after the day's last entry:
  htt log end --at "yesterday 17:30"
  htt log end --at "2026-11-17 17:30"

For a break, log an entry rather than ending the day, such as
htt log add @lunch, and give it a category that doesn't count as work
(see timelog.categories in GETTING-STARTED.md).

` + atHelp,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		task, err := domain.NewTask("@end")
		if err != nil {
			return fmt.Errorf("build end entry: %w", err)
		}
		_, err = logEntry(task, logAt)
		return err
	},
}

// printStatus is shared by `htt log status` and `htt status` (the outer
// rollup command): "Currently working on: <entry> (<category>,
// <duration>)", "Not working: <entry> (<category>, <duration>)" on an
// entry whose category isn't work, "Not working (day ended at HH:MM)"
// after an @end, or "Not currently working on any task." on a day with
// nothing logged; then the line about the last day logged before today
// if it has no @end (printUnendedDay). `log active` prints today's line
// alone.
func printStatus() error {
	if err := printToday("Not currently working on any task.", "Currently working on:"); err != nil {
		return err
	}
	printUnendedDay(time.Now())
	return nil
}

// statusLookBack is how many days before today status looks back for
// the last day logged (printUnendedDay). unendedHelp says two weeks.
const statusLookBack = 14

// printUnendedDay prints status's line about the most recent day before
// now's with a time log, up to statusLookBack days back, when that day
// has no @end (domain.DayUnended), with the command that ends it, as
// the report flags it:
//
//	2026-11-17 has no @end (counted to 24:00): end it with htt log end --at "2026-11-17 HH:MM"
//
// Only that day: an older day without an @end is the report's to flag.
// It needs no categories, so a timelog config htt can't use doesn't
// change it, and it loads the day without date warnings
// (baseTimelogRepo), since it shows none of the day's entries. A log it
// can't read doesn't stop the status: it warns on stderr, and prints no
// line.
func printUnendedDay(now time.Time) {
	tl, err := domain.LastLogBefore(baseTimelogRepo(), now, statusLookBack)
	if err == nil && tl == nil {
		return // nothing logged in the days it looks at
	}
	var sheet *domain.Timesheet
	if err == nil {
		if sheet, err = tl.Timesheet(nil, now); err != nil {
			err = fmt.Errorf("read the timelog for %s: %w", tl.Date.Format("2006-01-02"), err)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v; not checking that day for an @end\n", err)
		return
	}
	if sheet.Status == domain.DayUnended {
		date := tl.Date.Format("2006-01-02")
		fmt.Printf("%s has no @end (counted to 24:00): end it with htt log end --at %q\n", date, date+" HH:MM")
	}
}

// notWorking is printToday's prefix for an entry whose category doesn't
// count as work, such as a break, from log status and log active alike.
const notWorking = "Not working:"

// printToday prints where today's time log stands: the entry running
// now, its category (uncategorisedLabel if none matched) and how long
// it has run, after the working prefix, or after notWorking when its
// category doesn't count as work; that the day ended, and when, after
// an @end; or the empty line on a day with nothing logged. `log status`
// and `log active` word the working and empty lines differently.
//
// A timelog config htt can't use doesn't stop it, as it stops the
// report: it warns on stderr and prints the entry without its category
// (every entry working), as status did before it read the categories.
// It warns only when it shows an entry, the only line a category
// changes.
func printToday(empty, working string) error {
	tl, err := timelogRepo().Today()
	if err != nil {
		return fmt.Errorf("load today's timelog: %w", err)
	}
	categories, configErr := timelogCategoriser() // nil on an error: no categories
	now := time.Now()
	sheet, err := tl.Timesheet(categories, now)
	if err != nil {
		return fmt.Errorf("read today's timelog: %w", err)
	}
	switch sheet.Status {
	case domain.DayEnded:
		fmt.Printf("Not working (day ended at %s)\n", sheet.EndedAt.In(now.Location()).Format("15:04"))
	case domain.DayInProgress:
		current := sheet.Current()
		entry := current.Entry.RemoveAnnotation(domain.TimelogTimestampLabel).ColorString()
		running := utils.HumanizeDuration(current.Duration())
		if configErr != nil {
			fmt.Fprintf(os.Stderr, "warning: %v; showing entries without their categories\n", configErr)
			fmt.Printf("%s %s (%s)\n", working, entry, running)
			return nil
		}
		prefix, category := working, current.Category.Name
		if !current.Category.Work {
			prefix = notWorking
		}
		if current.Uncategorised {
			category = uncategorisedLabel
		}
		fmt.Printf("%s %s (%s, %s)\n", prefix, entry, category, running)
	default: // nothing logged today
		fmt.Println(empty)
	}
	return nil
}

func init() {
	LogCommand.AddCommand(Add)
	LogCommand.AddCommand(Show)
	LogCommand.AddCommand(Edit)
	LogCommand.AddCommand(Status)
	LogCommand.AddCommand(Active)
	LogCommand.AddCommand(Start)
	LogCommand.AddCommand(End)

	for _, cmd := range []*cobra.Command{Add, Start, End} {
		cmd.Flags().Var(&logAt, "at", atFlagUsage)
	}

	RootCmd.AddCommand(LogCommand)
}
