package cli

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bricef/htt/internal/dates"
	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/utils"
	"github.com/spf13/cobra"
)

var TodoCommand = &cobra.Command{

	Use:     "todo",
	Short:   "Manage todo lists.",
	Args:    cobra.NoArgs,
	Aliases: []string{"t"},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		printContext(ctx, false)
		return nil
	},
}

var addDue, addUnder string

var add = &cobra.Command{
	Use:     "add <entry...>",
	Aliases: []string{"a"},
	Short:   "Add an item to the current tasklist",
	Long: `Add an item to the current tasklist.

The --due flag accepts an absolute date (YYYY-MM-DD) or a natural-
language phrase ("Friday", "tomorrow", "in two weeks", "next Monday").
The phrase is resolved to a date and stored as a due: annotation on
the task — the file always carries a structured date, regardless of
how the user typed it.

--under N adds the task as a subtask of task N: the task gets p:ID,
and task N gets id:ID if it has none (the smallest number no id: or
p: in the file uses). The subtask goes after task N's last subtask,
or among them by priority, so the numbers below it shift by one.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		return addTask(ctx, args, addDue, addUnder, "")
	},
}

// addTask adds the task typed as args to ctx, with its --due, and
// under the task --under names when it names one, and says so. to is
// what follows "Added: <task>" (" to <context>" for add-to).
func addTask(ctx *domain.Context, args []string, due, under, to string) error {
	task, err := domain.NewTask(strings.Join(args, " "))
	if err != nil {
		return fmt.Errorf("parse task: %w", err)
	}
	if err := applyDueFlag(task, due); err != nil {
		return err
	}
	if under == "" {
		if err := ctx.AddTask(task); err != nil {
			return fmt.Errorf("add task%s: %w", to, err)
		}
		fmt.Printf("Added: %v%s\n", task.ConsoleString(), to)
		warnTypedTask(ctx, task)
		return nil
	}
	parent, err := ctx.GetTaskByStrId(under)
	if err != nil {
		return fmt.Errorf("--under: %w", err)
	}
	if err := ctx.AddSubtask(under, task); err != nil {
		return fmt.Errorf("add subtask of task %s%s: %w", under, to, err)
	}
	fmt.Printf("Added: %v%s\n", task.ConsoleString(), to)
	fmt.Printf("Under: %v\n", parent.ConsoleString())
	warnTypedTask(ctx, task)
	return nil
}

var addToDue, addToUnder string

// taskNumber is a string flag that takes a task number, a whole number,
// and refuses anything else: an empty --under would otherwise read as
// no --under at all, and add a top-level task.
type taskNumber struct{ value *string }

func (f taskNumber) String() string {
	if f.value == nil { // pflag's zero value, for its help
		return ""
	}
	return *f.value
}

func (f taskNumber) Set(s string) error {
	if n, err := strconv.Atoi(s); err != nil || n < 0 {
		return errors.New("not a task number")
	}
	*f.value = s
	return nil
}

func (f taskNumber) Type() string { return "string" }

var addTo = &cobra.Command{
	Use:   "add-to <context> <entry...>",
	Short: "Add an item to a specific tasklist",
	Long: `Add an item to a specific tasklist, as add does to the current one.
--under N names a task of that list.`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := repo().Context(args[0])
		if err != nil {
			return fmt.Errorf("load context %s: %w", args[0], err)
		}
		return addTask(ctx, args[1:], addToDue, addToUnder, " to "+ctx.ConsoleString())
	},
}

// applyDueFlag resolves a --due flag value (absolute date or natural-
// language phrase) and stamps it as a due: annotation on task.
// Empty flag is a no-op so callers don't pre-filter. Anchored at
// time.Now() because the flag is a user-facing convenience —
// deterministic clocks are only needed in the dates.ParsePhrase tests.
func applyDueFlag(task *domain.Task, value string) error {
	if value == "" {
		return nil
	}
	due, err := dates.ParsePhrase(value, time.Now())
	if err != nil {
		return fmt.Errorf("parse --due: %w", err)
	}
	if _, err := task.Annotate("due", due.Format("2006-01-02")); err != nil {
		return fmt.Errorf("annotate due date: %w", err)
	}
	return nil
}

var edit = &cobra.Command{
	Use:     "edit <item index>",
	Short:   "Edit the item specified in your editor ($VISUAL or $EDITOR)",
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"e"},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		oldTask, err := ctx.GetTaskByStrId(args[0])
		if err != nil {
			return fmt.Errorf("get task: %w", err)
		}

		f, err := os.CreateTemp("", "hypothetical-tracker-todo")
		if err != nil {
			return fmt.Errorf("open temp file: %w", err)
		}
		name := f.Name()
		defer func() { _ = os.Remove(name) }()

		if _, err := f.WriteString(oldTask.Raw + "\n"); err != nil {
			return fmt.Errorf("write temp file: %w", err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close temp file: %w", err)
		}

		if err := utils.EditFilePath(name); err != nil {
			return err
		}

		content, err := os.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read temp file: %w", err)
		}
		raw := strings.TrimSpace(string(content))
		if raw == oldTask.Raw || raw == "" {
			utils.Info("New entry was identical or empty. No actions taken.")
			return nil
		}

		newTask, err := domain.NewTask(raw)
		if err != nil {
			return fmt.Errorf("parse replacement: %w", err)
		}
		if _, err := ctx.Replace(args[0], newTask); err != nil {
			return fmt.Errorf("replace: %w", err)
		}
		fmt.Printf("Before: %s\n", oldTask.ConsoleString())
		fmt.Printf("After:  %s\n", newTask.ConsoleString())
		warnTypedTask(ctx, newTask)
		return nil
	},
}

var editDone = &cobra.Command{
	Use:     "edit-done",
	Short:   "Open the done file in your editor ($VISUAL or $EDITOR)",
	Args:    cobra.NoArgs,
	Aliases: []string{"ed"},
	RunE: func(cmd *cobra.Command, args []string) error {
		path := repo().ContextPath(domain.DoneContextName)
		if path == "" {
			return fmt.Errorf("active repository does not expose a file path")
		}
		return utils.EditFilePath(path)
	},
}

var editArchive = &cobra.Command{
	Use:     "edit-archive",
	Short:   "Open the archive file in your editor (for manual garbage collection)",
	Args:    cobra.NoArgs,
	Aliases: []string{"ea"},
	RunE: func(cmd *cobra.Command, args []string) error {
		path := repo().ContextPath(domain.ArchiveContextName)
		if path == "" {
			return fmt.Errorf("active repository does not expose a file path")
		}
		return utils.EditFilePath(path)
	},
}

var deleteTodo = &cobra.Command{
	Use:     "delete <item index>",
	Short:   "Delete the item specified, with its subtasks (they move to the archive)",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		tasks, err := ctx.Delete(args[0])
		if err != nil {
			return fmt.Errorf("delete task: %w", err)
		}
		for _, task := range tasks {
			fmt.Printf("Deleted task: %v\n", task.ConsoleString())
		}
		return nil
	},
}

var move = &cobra.Command{
	Use:   "move <item index> <new context>",
	Short: "Move the task, with its subtasks, to a different context",
	Long: `Move the task, with its subtasks, to a different context.

The task becomes a top-level task there (it loses its p:, since a
parent is always in the same file as its subtasks), and any id: of the
tasks moved that the other context already uses is renumbered there,
their p: tags with it. A task whose only text is its p: can't lose it,
so it can't be moved.

A done task moved from the done file back to the context it was
completed in links up again: it keeps its p: when its parent is there,
and its id: when the p: tags there naming that id can name no other
task, so its subtasks are under it again.`,
	Aliases: []string{"mv"},
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		tasks, err := ctx.Move(args[0], args[1])
		if err != nil {
			return fmt.Errorf("move task: %w", err)
		}
		fromCtx := &domain.Context{Name: ctx.Name}
		toCtx := &domain.Context{Name: args[1]}
		for _, task := range tasks {
			fmt.Printf("Moved %v from %v to %v.\n", task.ConsoleString(), fromCtx.ConsoleString(), toCtx.ConsoleString())
		}
		return nil
	},
}

var nest = &cobra.Command{
	Use:   "nest <item index> <parent index>",
	Short: "Put a task, with its subtasks, under another task",
	Long: `Put a task, with its subtasks, under another task: the task gets
p:ID, naming the parent, and the parent gets id:ID if it has none (the
smallest number no id: or p: in the file uses).

The task goes after the parent's last subtask, or among them by
priority, so line numbers shift. Nesting a task under itself or under
one of its own subtasks is refused.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		task, parent, err := ctx.Nest(args[0], args[1])
		if err != nil {
			return fmt.Errorf("nest task %s under task %s: %w", args[0], args[1], err)
		}
		fmt.Printf("Nested: %v\n", task.ConsoleString())
		fmt.Printf("Under:  %v\n", parent.ConsoleString())
		return nil
	},
}

var unnest = &cobra.Command{
	Use:   "unnest <item index>",
	Short: "Make a subtask a top-level task, with its own subtasks",
	Long: `Make a subtask a top-level task, with its own subtasks: its p: tags
go, and it moves to just after the top-level task it was under. Its
old parent keeps its id:.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		task, err := ctx.Unnest(args[0])
		if err != nil {
			return fmt.Errorf("unnest task %s: %w", args[0], err)
		}
		fmt.Printf("Unnested: %v\n", task.ConsoleString())
		return nil
	},
}

var random = &cobra.Command{
	Use:   "random",
	Short: "Select an item at random from the tasklist",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		current, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		if len(current.Tasks) == 0 {
			return errors.New("current context is empty")
		}
		task := current.Tasks[rand.Intn(len(current.Tasks))]
		fmt.Println(task.ConsoleString())
		return nil
	},
}

var priPlus = &cobra.Command{
	Use:   "+ <item index>",
	Short: "increase the priority for the selected task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		old, neu, err := ctx.IncreasePriority(args[0])
		if err != nil {
			return fmt.Errorf("increase priority: %w", err)
		}
		fmt.Printf("Before: %s\n", old.ConsoleString())
		fmt.Printf("After:  %s\n", neu.ConsoleString())
		return nil
	},
}

var priMinus = &cobra.Command{
	Use:   "- <item index>",
	Short: "Decrease the priority for the selected task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		old, neu, err := ctx.DecreasePriority(args[0])
		if err != nil {
			return fmt.Errorf("decrease priority: %w", err)
		}
		fmt.Printf("Before: %s\n", old.ConsoleString())
		fmt.Printf("After:  %s\n", neu.ConsoleString())
		return nil
	},
}

var priority = &cobra.Command{
	Use:     "priority <item index> <priority>",
	Short:   "Sets the priority of a set item. Priority must be one of [A, B, C].",
	Args:    cobra.ExactArgs(2),
	Aliases: []string{"pri", "p"},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		old, neu, err := ctx.SetPriority(args[0], args[1])
		if err != nil {
			return fmt.Errorf("set priority: %w", err)
		}
		fmt.Printf("Before: %s\n", old.ConsoleString())
		fmt.Printf("After:  %s\n", neu.ConsoleString())
		return nil
	},
}

var replace = &cobra.Command{
	Use:     "replace <item number> <entry...>",
	Short:   "Replace an item with a new entry",
	Args:    cobra.MinimumNArgs(2),
	Aliases: []string{"r"},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		neu, err := domain.NewTask(strings.Join(args[1:], " "))
		if err != nil {
			return fmt.Errorf("parse replacement: %w", err)
		}
		old, err := ctx.Replace(args[0], neu)
		if err != nil {
			return fmt.Errorf("replace task: %w", err)
		}
		fmt.Printf("Before: %s\n", old.ConsoleString())
		fmt.Printf("After: %s\n", neu.ConsoleString())
		warnTypedTask(ctx, neu)
		return nil
	},
}

var context = &cobra.Command{
	Use:     "context [context]",
	Short:   "Show or change the context for tasks",
	Aliases: []string{"switch", "c"},
	Long: `With no argument, print the current context. With one, switch to
that context; a new name starts an empty one.

Context names keep only letters, digits and underscores; any other
character becomes an underscore, so different names can map to the
same context ("weekend hacks!" becomes weekend_hacks_). The name
actually used is printed.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			current, err := repo().CurrentContext()
			if err != nil {
				return fmt.Errorf("load current context: %w", err)
			}
			fmt.Printf("%s\n", current.ConsoleString())
			return nil
		}

		if err := repo().SetCurrent(args[0]); err != nil {
			return fmt.Errorf("switch context: %w", err)
		}
		name, err := repo().CurrentContextName()
		if err != nil {
			return fmt.Errorf("read sanitized context name: %w", err)
		}
		fmt.Println("Now using context: " + name)
		return nil
	},
}

var todoStatus = &cobra.Command{
	Use:     "status",
	Short:   "Show the current status of todos",
	Aliases: []string{"?"},
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		current, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		names, err := domain.SwitchableContextNames(repo())
		if err != nil {
			return fmt.Errorf("list contexts: %w", err)
		}

		fmt.Printf("Available Contexts: ")
		for _, name := range names {
			c := &domain.Context{Name: name}
			fmt.Printf("%s ", c.ConsoleString())
		}
		fmt.Println()
		fmt.Printf("Current Context: %s\n", current.ConsoleString())
		printContext(current, false)
		return nil
	},
}

var search = &cobra.Command{
	Use:     "search <expression...>",
	Aliases: []string{"query", "q"},
	Short:   "Search in current context for tasks matching the given expression.",
	Long:    "Search in current context for tasks matching the given expression.\nThe expression is a Go regular expression, matched case-insensitively\nagainst the whole task line (dates, priority and annotations included).",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		current, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}

		restr := "(?i)" + strings.Join(args[0:], " ")
		re, err := regexp.Compile(restr)
		if err != nil {
			return fmt.Errorf("invalid regular expression: %w", err)
		}

		current.ShowOnly(func(t *domain.Task) bool {
			return re.MatchString(t.Raw)
		})
		return nil
	},
}

func init() {
	add.Flags().StringVar(&addDue, "due", "",
		"Due date: absolute (YYYY-MM-DD) or phrase (Friday, tomorrow, in two weeks)")
	addTo.Flags().StringVar(&addToDue, "due", "",
		"Due date: absolute (YYYY-MM-DD) or phrase (Friday, tomorrow, in two weeks)")
	add.Flags().Var(taskNumber{&addUnder}, "under", "Add it as a subtask of task `N`")
	addTo.Flags().Var(taskNumber{&addToUnder}, "under", "Add it as a subtask of task `N` of that context")

	TodoCommand.AddCommand(add)
	TodoCommand.AddCommand(addTo)
	TodoCommand.AddCommand(show)
	TodoCommand.AddCommand(edit)
	TodoCommand.AddCommand(do)
	TodoCommand.AddCommand(editDone)
	TodoCommand.AddCommand(editArchive)
	TodoCommand.AddCommand(deleteTodo)
	TodoCommand.AddCommand(random)
	TodoCommand.AddCommand(priPlus)
	TodoCommand.AddCommand(priMinus)
	TodoCommand.AddCommand(priority)
	TodoCommand.AddCommand(replace)
	TodoCommand.AddCommand(context)
	TodoCommand.AddCommand(todoStatus)
	TodoCommand.AddCommand(move)
	TodoCommand.AddCommand(nest)
	TodoCommand.AddCommand(unnest)
	TodoCommand.AddCommand(search)

	RootCmd.AddCommand(TodoCommand)
}
