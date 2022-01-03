package cmd

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/bricef/htt/internal/todo"
	"github.com/bricef/htt/internal/utils"
	"github.com/spf13/cobra"
)

var TodoCommand = &cobra.Command{

	Use:     "todo",
	Short:   "Manage todo lists.",
	Args:    cobra.NoArgs,
	Aliases: []string{"t"},
	Run: func(cmd *cobra.Command, args []string) {
		todo.GetCurrentContext().Show()
	},
}

var add = &cobra.Command{
	Use:     "add <entry...>",
	Aliases: []string{"a"},
	Short:   "Add an item to the current tasklist",
	Args:    cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		task := todo.NewTask(strings.Join(args, " "))
		todo.GetCurrentContext().Add(task).Sync()
		fmt.Printf("Added: %v\n", task.String())
	},
}

var addTo = &cobra.Command{
	Use:   "add-to <context> <entry...>",
	Short: "Add an item to a specific tasklist",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		context := todo.NewContext(args[0]).Read()
		task := todo.NewTask(strings.Join(args[1:], " "))
		context.Add(task)
		context.Sync()
		fmt.Printf("Added: %v to %v", task.String(), context.String())
	},
}

var show = &cobra.Command{
	Use:     "show",
	Aliases: []string{"s", "ls"},
	Short:   "Show the current tasklist.",
	Long:    `Show the current tasklist.`,
	Run: func(cmd *cobra.Command, args []string) {
		todo.GetCurrentContext().Show()
	},
}

var edit = &cobra.Command{
	Use:     "edit <item index>",
	Short:   "Edit the item specified using $EDITOR",
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"e"},
	Run: func(cmd *cobra.Command, args []string) {
		context := todo.GetCurrentContext()

		// Ensure editor
		editor, ok := os.LookupEnv("EDITOR")
		if !ok || editor == "" {
			utils.Fatal("$EDITOR variable is empty or not set. Could not edit task.")
		}

		oldTask, err := context.GetTaskByStrId(args[0])
		utils.DieOnError("Could not get task from context", err)

		f, err := ioutil.TempFile("", "hypothetical-tracker-todo")
		utils.DieOnError("Failed to open temporary file: ", err)
		name := f.Name() // save the name so we can reopen the file
		defer os.Remove(name)

		_, err = f.WriteString(oldTask.Raw + "\n")
		utils.DieOnError("Failed to write entry into temp file: ", err)
		f.Close() // close the file to let editor have at it

		proc := exec.Command(editor, f.Name())
		proc.Stdin = os.Stdin
		proc.Stdout = os.Stdout
		proc.Stderr = os.Stderr

		err = proc.Start()
		utils.DieOnError("Failed to start the editor: ", err)

		err = proc.Wait()
		utils.DieOnError("Error running editor: ", err)

		content, err := ioutil.ReadFile(name)
		utils.DieOnError("Failed to read the temp file after editing: ", err)

		raw := strings.TrimSpace(string(content))
		if raw == oldTask.String() || raw == "" {
			utils.Info("New entry was identical or empty. No actions taken.")
		} else {
			newTask := todo.NewTask(raw)
			context.Replace(oldTask, newTask)
			fmt.Printf("Before: %s\n", oldTask.String())
			fmt.Printf("After:  %s\n", newTask.String())
		}
	},
}

var do = &cobra.Command{
	Use:     "do <item index>",
	Short:   "Complete a task",
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"x"},
	Run: func(cmd *cobra.Command, args []string) {
		t, err := todo.CompleteTask(args[0])
		utils.DieOnError("Failed to complete the task.", err)

		fmt.Printf("Completed: %s\n", t.String())
	},
}

var editDone = &cobra.Command{
	Use:     "edit-done",
	Short:   "Open the done file using $EDITOR",
	Args:    cobra.NoArgs,
	Aliases: []string{"ed"},
	Run: func(cmd *cobra.Command, args []string) {
		utils.EditFilePath(todo.NewContext("done").Filepath())
	},
}

var delete = &cobra.Command{
	Use:     "delete <item index>",
	Short:   "Delete the item specified",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		context := todo.GetCurrentContext()

		task, err := context.GetTaskByStrId(args[0])
		utils.DieOnError("Could not get specfied task.", err)

		err = context.Remove(task)
		utils.DieOnError("Could not remove specified task.", err)

		err = context.Sync()
		utils.DieOnError("Could not save current context.", err)

		fmt.Printf("Deleted task: %v\n", task.String())
	},
}

var move = &cobra.Command{
	Use:     "move <item index> <new context>",
	Short:   "Move the task to a different context",
	Aliases: []string{"mv"},
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		fromContext := todo.GetCurrentContext()
		toContext := todo.NewContext(args[1])

		task, err := fromContext.GetTaskByStrId(args[0])
		utils.DieOnError("Could not get specified task.", err)

		err = todo.Move(task, fromContext, toContext)
		utils.DieOnError("Could not move task.", err)

		err = fromContext.Sync()
		utils.DieOnError("Could not save context.", err)

		err = toContext.Sync()
		utils.DieOnError("Could not save context.", err)

		fmt.Printf("Moved %v from %v to %v.\n", task.String(), fromContext.String(), toContext.String())
	},
}

var random = &cobra.Command{
	Use:   "random",
	Short: "Select an item at random from the tasklist",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		rand.Seed(time.Now().Unix())

		current := todo.GetCurrentContext()
		task := current.Tasks[rand.Intn(len(current.Tasks))]

		fmt.Println(task.String())
	},
}

var priPlus = &cobra.Command{
	Use:   "+ <item index>",
	Short: "increase the priority for the selected task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := todo.GetCurrentContext()
		oldTask, err := ctx.GetTaskByStrId(args[0])
		utils.DieOnError("Could not find task specified.", err)

		fmt.Printf("Before: %s\n", oldTask.String())

		newTask := oldTask.IncreasePriority()
		ctx.Replace(oldTask, newTask)
		ctx.Sync()

		fmt.Printf("After:  %s\n", newTask.String())

	},
}

var priMinus = &cobra.Command{
	Use:   "- <item index>",
	Short: "Decrease the priority for the selected task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := todo.GetCurrentContext()
		oldTask, err := ctx.GetTaskByStrId(args[0])
		utils.DieOnError("Could not find task specified.", err)

		fmt.Printf("Before: %s\n", oldTask.String())

		newTask := oldTask.DecreasePriority()
		ctx.Replace(oldTask, newTask)
		ctx.Sync()

		fmt.Printf("After:  %s\n", newTask.String())
	},
}

var priority = &cobra.Command{
	Use:     "priority <item index> <priority>",
	Short:   "Sets the priority of a set item. Priority must be one of [A, B, C].",
	Args:    cobra.ExactArgs(2),
	Aliases: []string{"pri", "p"},
	Run: func(cmd *cobra.Command, args []string) {
		// Check valid input
		matches, err := regexp.MatchString("^[ABCDEF]$", args[1])
		if err != nil || !matches {
			utils.Fatal(fmt.Sprintf("Invalid priority '%v'.", args[1]))
		}

		ctx := todo.GetCurrentContext()
		oldTask, err := ctx.GetTaskByStrId(args[0])
		utils.DieOnError("Could not find task specified.", err)

		fmt.Printf("Before: %s\n", oldTask.String())

		newTask := oldTask.SetPriority(args[1])
		ctx.Replace(oldTask, newTask)
		ctx.Sync()

		fmt.Printf("After:  %s\n", newTask.String())

	},
}

var replace = &cobra.Command{
	Use:     "replace <item number> <entry...>",
	Short:   "Replace an item with a new entry",
	Args:    cobra.MinimumNArgs(2),
	Aliases: []string{"r"},
	Run: func(cmd *cobra.Command, args []string) {
		ctx := todo.GetCurrentContext()
		oldTask, err := ctx.GetTaskByStrId(args[0])
		utils.DieOnError("Could not find task specified.", err)

		fmt.Printf("Before: %s\n", oldTask.String())

		newTask := todo.NewTask(strings.Join(args[1:], " "))
		ctx.Replace(oldTask, newTask)
		ctx.Sync()

		fmt.Printf("After: %s\n", newTask.String())
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
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			current := todo.GetCurrentContext()
			fmt.Printf("%s\n", current.String())
			return
		}

		todo.SetCurrentContext(args[0])
		fmt.Println("Now using context: " + args[0])
	},
}

var todoStatus = &cobra.Command{
	Use:     "status",
	Short:   "Show the current status of todos",
	Aliases: []string{"?"},
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		todo.ShowStatus()
	},
}

var search = &cobra.Command{
	Use:     "search <expression...>",
	Aliases: []string{"query", "q"},
	Short:   "Search in current context for tasks matching the given expression.",
	Long:    "Search in current context for tasks matching the given expression.\nThe Expression will be interepreted as a Golang regular expression.",
	Args:    cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// interpret expression as golang regxp
		current := todo.GetCurrentContext()

		// Case insensitive
		restr := "(?i)" + strings.Join(args[0:], " ")
		re, err := regexp.Compile(restr)
		utils.DieOnError("Invalid regular expression", err)

		current.ShowOnly(func(t *todo.Task) bool {
			return re.MatchString(t.Raw)
		})
	},
}

func init() {
	TodoCommand.AddCommand(add)
	TodoCommand.AddCommand(addTo)
	TodoCommand.AddCommand(show)
	TodoCommand.AddCommand(edit)
	TodoCommand.AddCommand(do)
	TodoCommand.AddCommand(editDone)
	TodoCommand.AddCommand(delete)
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
