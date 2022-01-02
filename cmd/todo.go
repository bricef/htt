package cmd

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bricef/htt/todo"
	"github.com/bricef/htt/utils"
	"github.com/bricef/htt/vars"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var TodoCommand = &cobra.Command{

	Use:     "todo",
	Short:   "Manage todo lists.",
	Args:    cobra.NoArgs,
	Aliases: []string{"t"},
	Run: func(cmd *cobra.Command, args []string) {
		todo.Show(todo.GetCurrentContext(), []string{})
	},
}

var add = &cobra.Command{
	Use:     "add [task to add]",
	Aliases: []string{"a"},
	Short:   "Add an item to the current tasklist",
	Args:    cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		todo.AddTodo(strings.Join(args, " "))
	},
}

var addTo = &cobra.Command{
	Use:   "add-to <list> [task to add]",
	Short: "Add an item to a specific tasklist",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		context := args[0]
		entry := strings.Join(args[1:], " ")
		todo.AddToContext(context, entry)
	},
}

var show = &cobra.Command{
	Use:     "show [search terms]",
	Aliases: []string{"s", "ls"},
	Short:   "Show the current tasklist.",
	Long: `Show the current tasklist. You can filter the task list using 
search terms. Search terms will be matched fuzzily.`,
	Run: func(cmd *cobra.Command, args []string) {
		// utils.ClearScreen()

		if len(args) > 0 {
			utils.Warning("Filtering is not yet implemented. Showing without filter applied.")
		}
		todo.Show(todo.GetCurrentContext(), args)
	},
}

var edit = &cobra.Command{
	Use:     "edit [item number]",
	Short:   "Edit the item specified using $EDITOR",
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"e"},
	Run: func(cmd *cobra.Command, args []string) {
		editor, ok := os.LookupEnv("EDITOR")
		if !ok || editor == "" {
			utils.Fatal("$EDITOR variable is empty or not set. Could not edit task.")
		}

		id, err := strconv.Atoi(args[0])
		utils.DieOnError("Supplied argument '"+args[0]+"' was not an integer: ", err)
		t := todo.GetTodoID(id)

		f, err := ioutil.TempFile("", "hypothetical-tracker-todo")
		utils.DieOnError("Failed to open temporary file: ", err)
		name := f.Name() // save the name so we cn reopen the file
		defer os.Remove(name)

		_, err = f.WriteString(t.ToString() + "\n")
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
		if raw == t.ToString() || raw == "" {
			utils.Info("New entry was identical or empty. No actions taken.")
		} else {
			todo.Replace(t.Line, todo.NewTask(raw))
			fmt.Printf("Before: %s\n", t.ToString())
			fmt.Printf("After:  %s\n", raw)
		}
	},
}

var do = &cobra.Command{
	Use:     "do [item number]",
	Short:   "Complete a task",
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"x"},
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.Atoi(args[0])
		utils.DieOnError("Supplied argument '"+args[0]+"' was not an integer: ", err)
		todo.CompleteTask(id)
	},
}

var editDone = &cobra.Command{
	Use:     "edit-done",
	Short:   "Open the done file using $EDITOR",
	Args:    cobra.NoArgs,
	Aliases: []string{"ed"},
	Run: func(cmd *cobra.Command, args []string) {
		utils.EditFilePath(todo.DoneFilePath())
	},
}

var delete = &cobra.Command{
	Use:     "delete [item index]",
	Short:   "Delete the item specified",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.Atoi(args[0])
		utils.DieOnError("Supplied argument '"+args[0]+"' was not an integer: ", err)
		t := todo.GetTodoID(id)
		todo.Delete(t)
		fmt.Printf("Deleted task: " + t.ToString() + "\n")
	},
}

var random = &cobra.Command{
	Use:   "random",
	Short: "Select an item at random from the tasklist",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		ts := todo.GetTodos()
		rand.Seed(time.Now().Unix())
		if vars.GetBool(vars.ConfigKeyDisableColor) {
			fmt.Println(ts[rand.Intn(len(ts))].ToString())
		} else {
			fmt.Println(ts[rand.Intn(len(ts))].ColorString())
		}

	},
}

var priPlus = &cobra.Command{
	Use:   "+ [item number]",
	Short: "increase the priority for the selected task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		id, err := strconv.Atoi(args[0])
		utils.DieOnError("Supplied argument '"+args[0]+"' was not an integer: ", err)
		t := todo.GetTodoID(id)

		todo.Replace(id, t.IncreasePriority())

	},
}

var priMinus = &cobra.Command{
	Use:   "- [item number]",
	Short: "Decrease the priority for the selected task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.Atoi(args[0])
		utils.DieOnError("Supplied argument '"+args[0]+"' was not an integer: ", err)
		t := todo.GetTodoID(id)
		todo.Replace(id, t.DecreasePriority())
	},
}

var priority = &cobra.Command{
	Use:     "priority [item number] [priority]",
	Short:   "Sets the priority of a set item. Priority must be one of [A, B, C].",
	Args:    cobra.ExactArgs(2),
	Aliases: []string{"pri", "p"},
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.Atoi(args[0])
		utils.DieOnError("Supplied argument '"+args[0]+"' was not an integer: ", err)
		t := todo.GetTodoID(id)

		matches, err := regexp.MatchString("^[ABCDEF]$", args[1])
		if err != nil || !matches {
			utils.Fatal(fmt.Sprintf("Invalid priority '%v'.", args[1]))
		}
		todo.Replace(id, t.SetPriority(args[1]))
	},
}

var replace = &cobra.Command{
	Use:     "replace [item number] [new entry]",
	Short:   "Replace an item with a new entry",
	Args:    cobra.MinimumNArgs(2),
	Aliases: []string{"r"},
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.Atoi(args[0])
		utils.DieOnError("Supplied argument '"+args[0]+"' was not an integer: ", err)
		raw := strings.Join(args[1:], " ")
		todo.Replace(id, todo.NewTask(raw))
	},
}

var get = &cobra.Command{
	Use:     "get [priorities...]",
	Aliases: []string{"tp"},
	Short:   "Show tasks across lists.",
	Long:    `Will show the high priority items across all contexts. The priority range defaults to "A", but can be specified through arguments`,
	Run: func(cmd *cobra.Command, args []string) {

		priorities := args
		if len(priorities) == 0 {
			priorities = append(priorities, "A")
		}

		contexts := todo.GetContexts()

		// this makes me sad.
		for _, context := range contexts {
			todos := todo.GetTodosForContext(context)

			ts := []*todo.Task{}
			for _, task := range todos {
				for _, priority := range priorities {
					if task.Priority == priority {
						ts = append(ts, task)
					}
				}
			}
			if len(ts) > 0 {
				fmt.Printf("%s\n", context)
				for _, todo := range ts {
					if vars.GetBool(vars.ConfigKeyDisableColor) {
						fmt.Printf("%3d %s\n", todo.Line, todo.ToString())
					} else {
						fmt.Printf("%3d %s\n", todo.Line, todo.ColorString())
					}
				}
				print("\n")
			}
		}

	},
}

var context = &cobra.Command{
	Use:     "context [context]",
	Short:   "Change the context for tasks",
	Aliases: []string{"switch", "c"},
	Long: `This will change the default context for todos. Note that 
the only supported characters are [A-Za-z_]. Other 
characters will be replaced by underscores. This 
means that different arguments may map to the 
same context.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			// contexts := todo.GetContexts()
			current := todo.GetCurrentContext()
			if vars.GetBool(vars.ConfigKeyDisableColor) {
				fmt.Printf("%s\n", current)
			} else {
				color.Green(current)
			}

			// for _, c := range contexts {
			// 	if current != c {
			// 		fmt.Printf("%s\n", c)
			// 	}
			// }
			// fmt.Print("\n")
		} else {
			todo.SetContext(args[0])
			fmt.Println("Now using context: " + args[0])
		}
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
	TodoCommand.AddCommand(get)
	TodoCommand.AddCommand(context)
	TodoCommand.AddCommand(todoStatus)

	RootCmd.AddCommand(TodoCommand)
}
