package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var cmdTracking = &cobra.Command{
	Use:   "track",
	Short: "Print anything to the screen",
	Long: `print is for printing anything back to the screen.
For many years people have printed back to the screen.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Print: " + strings.Join(args, " "))
	},
}

func init() {
	// cmdTodo.AddCommand(cmdTodoAdd)
	// cmdTodo.AddCommand(cmdTodoComplete)
	// cmdTodo.AddCommand(cmdTodoRemove)
	// cmdTodo.AddCommand(cmdTodoRemove)
}
