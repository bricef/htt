package cmd

import (
	"os"
	"os/exec"

	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/hypotheticalco/tracker-client/utils"
	"github.com/spf13/cobra"
)

var editDone = &cobra.Command{
	Use:     "editDone",
	Short:   "Open the done file using $EDITOR",
	Args:    cobra.NoArgs,
	Aliases: []string{"ed"},
	Run: func(cmd *cobra.Command, args []string) {
		editor, ok := os.LookupEnv("EDITOR")
		if !ok || editor == "" {
			utils.Fatal("$EDITOR variable is empty or not set. Could not edit task.")
		}

		proc := exec.Command(editor, todo.DoneFilePath())
		proc.Stdin = os.Stdin
		proc.Stdout = os.Stdout
		proc.Stderr = os.Stderr

		err := proc.Start()
		utils.DieOnError("Failed to start the editor: ", err)

		err = proc.Wait()
		utils.DieOnError("Error running editor: ", err)
	},
}

func init() {
	RootCmd.AddCommand(editDone)
}
