package cli

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strings"

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
		ctx, err := uc().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		ctx.Show()
		return nil
	},
}

var add = &cobra.Command{
	Use:     "add <entry...>",
	Aliases: []string{"a"},
	Short:   "Add an item to the current tasklist",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		task, _, err := uc().AddTask(strings.Join(args, " "))
		if err != nil {
			return fmt.Errorf("add task: %w", err)
		}
		fmt.Printf("Added: %v\n", task.ConsoleString())
		return nil
	},
}

var addTo = &cobra.Command{
	Use:   "add-to <context> <entry...>",
	Short: "Add an item to a specific tasklist",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		task, ctx, err := uc().AddTaskTo(args[0], strings.Join(args[1:], " "))
		if err != nil {
			return fmt.Errorf("add task to %s: %w", args[0], err)
		}
		fmt.Printf("Added: %v to %v", task.ConsoleString(), ctx.ConsoleString())
		return nil
	},
}

var show = &cobra.Command{
	Use:     "show",
	Aliases: []string{"s", "ls", "l"},
	Short:   "Show the current tasklist.",
	Long:    `Show the current tasklist.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := uc().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		ctx.Show()
		return nil
	},
}

var edit = &cobra.Command{
	Use:     "edit <item index>",
	Short:   "Edit the item specified using $EDITOR",
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"e"},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := uc().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		oldTask, err := ctx.GetTaskByStrId(args[0])
		if err != nil {
			return fmt.Errorf("get task: %w", err)
		}

		editor, ok := os.LookupEnv("EDITOR")
		if !ok || editor == "" {
			return errors.New("$EDITOR variable is empty or not set")
		}

		f, err := os.CreateTemp("", "hypothetical-tracker-todo")
		if err != nil {
			return fmt.Errorf("open temp file: %w", err)
		}
		name := f.Name()
		defer os.Remove(name)

		if _, err := f.WriteString(oldTask.Raw + "\n"); err != nil {
			return fmt.Errorf("write temp file: %w", err)
		}
		f.Close()

		proc := exec.Command(editor, name)
		proc.Stdin = os.Stdin
		proc.Stdout = os.Stdout
		proc.Stderr = os.Stderr
		if err := proc.Start(); err != nil {
			return fmt.Errorf("start editor: %w", err)
		}
		if err := proc.Wait(); err != nil {
			return fmt.Errorf("run editor: %w", err)
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

		// ReplaceTask persists; the legacy code path did the in-memory
		// Replace but never Sync'd, so edits silently failed to stick.
		_, newTask, err := uc().ReplaceTask(args[0], raw)
		if err != nil {
			return fmt.Errorf("replace: %w", err)
		}
		fmt.Printf("Before: %s\n", oldTask.ConsoleString())
		fmt.Printf("After:  %s\n", newTask.ConsoleString())
		return nil
	},
}

var do = &cobra.Command{
	Use:     "do <item index>",
	Short:   "Complete a task",
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"x"},
	RunE: func(cmd *cobra.Command, args []string) error {
		t, err := uc().CompleteTask(args[0])
		if err != nil {
			return fmt.Errorf("complete task: %w", err)
		}
		fmt.Printf("Completed: %s\n", t.ConsoleString())
		return nil
	},
}

var editDone = &cobra.Command{
	Use:     "edit-done",
	Short:   "Open the done file using $EDITOR",
	Args:    cobra.NoArgs,
	Aliases: []string{"ed"},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Legacy $EDITOR shell-out; needs the on-disk path.
		utils.EditFilePath(domain.NewContext("done").Filepath())
		return nil
	},
}

var deleteTodo = &cobra.Command{
	Use:     "delete <item index>",
	Short:   "Delete the item specified",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		task, err := uc().DeleteTask(args[0])
		if err != nil {
			return fmt.Errorf("delete task: %w", err)
		}
		fmt.Printf("Deleted task: %v\n", task.ConsoleString())
		return nil
	},
}

var move = &cobra.Command{
	Use:     "move <item index> <new context>",
	Short:   "Move the task to a different context",
	Aliases: []string{"mv"},
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		task, from, to, err := uc().MoveTask(args[0], args[1])
		if err != nil {
			return fmt.Errorf("move task: %w", err)
		}
		fromCtx := &domain.Context{Name: from}
		toCtx := &domain.Context{Name: to}
		fmt.Printf("Moved %v from %v to %v.\n", task.ConsoleString(), fromCtx.ConsoleString(), toCtx.ConsoleString())
		return nil
	},
}

var random = &cobra.Command{
	Use:   "random",
	Short: "Select an item at random from the tasklist",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		current, err := uc().CurrentContext()
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
		old, neu, err := uc().IncreasePriority(args[0])
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
		old, neu, err := uc().DecreasePriority(args[0])
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
		old, neu, err := uc().SetPriority(args[0], args[1])
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
		old, neu, err := uc().ReplaceTask(args[0], strings.Join(args[1:], " "))
		if err != nil {
			return fmt.Errorf("replace task: %w", err)
		}
		fmt.Printf("Before: %s\n", old.ConsoleString())
		fmt.Printf("After: %s\n", neu.ConsoleString())
		return nil
	},
}

var context = &cobra.Command{
	Use:     "context [context]",
	Short:   "Show or change the context for tasks",
	Aliases: []string{"switch", "c"},
	Long: `This will change the default context for todos. Note that
the only supported characters are [A-Za-z_]. Other
characters will be replaced by underscores. This
means that different arguments may map to the
same context.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			current, err := uc().CurrentContext()
			if err != nil {
				return fmt.Errorf("load current context: %w", err)
			}
			fmt.Printf("%s\n", current.ConsoleString())
			return nil
		}

		name, err := uc().SwitchContext(args[0])
		if err != nil {
			return fmt.Errorf("switch context: %w", err)
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
		current, err := uc().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		names, err := uc().ListContextNames()
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
		current.Show()
		return nil
	},
}

var search = &cobra.Command{
	Use:     "search <expression...>",
	Aliases: []string{"query", "q"},
	Short:   "Search in current context for tasks matching the given expression.",
	Long:    "Search in current context for tasks matching the given expression.\nThe Expression will be interepreted as a Golang regular expression.",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		current, err := uc().CurrentContext()
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
	TodoCommand.AddCommand(add)
	TodoCommand.AddCommand(addTo)
	TodoCommand.AddCommand(show)
	TodoCommand.AddCommand(edit)
	TodoCommand.AddCommand(do)
	TodoCommand.AddCommand(editDone)
	TodoCommand.AddCommand(deleteTodo)
	TodoCommand.AddCommand(random)
	TodoCommand.AddCommand(priPlus)
	TodoCommand.AddCommand(priMinus)
	TodoCommand.AddCommand(priority)
	TodoCommand.AddCommand(replace)
	TodoCommand.AddCommand(context)
	TodoCommand.AddCommand(todoStatus)
	TodoCommand.AddCommand(move)
	TodoCommand.AddCommand(search)

	RootCmd.AddCommand(TodoCommand)
}
