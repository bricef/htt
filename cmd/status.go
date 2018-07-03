package cmd

import (
	"fmt"
	"strings"

	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/hypotheticalco/tracker-client/utils"
	"github.com/spf13/cobra"
)

var status = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the tasklist and time log",
	Run: func(cmd *cobra.Command, args []string) {
		// sync status
		// which context current, what other contexts
		// how many todos
		contexts := todo.GetContexts()
		current := todo.GetCurrentContext()
		todos := todo.GetTodos()
		i, err := utils.StringSliceIndex(contexts, current)
		if err == nil {
			contexts = append(contexts[:i], contexts[i+1:]...)
		}
		fmt.Printf("Current task context: %v\n", current)
		fmt.Printf("Tasks outstandining: %v\n", len(todos))
		fmt.Printf("Available contexts: %v\n", strings.Join(contexts, ", "))
	},
}

func init() {
	RootCmd.AddCommand(status)
}
