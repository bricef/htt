package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/hypotheticalco/tracker-client/utils"
	"github.com/spf13/cobra"
)

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

		_, err = f.WriteString(t.Entry + "\n")
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

		newEntry := strings.TrimSpace(string(content))
		if newEntry == t.Entry || newEntry == "" {
			utils.Info("New entry was identical or empty. No actions taken.")
		} else {
			todo.Replace(t.Line, newEntry)
			fmt.Printf("Before: %s\n", t.Entry)
			fmt.Printf("After:  %s\n", newEntry)
		}
	},
}

func init() {
	RootCmd.AddCommand(edit)
}
