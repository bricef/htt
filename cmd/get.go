package cmd

import (
	"fmt"

	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/hypotheticalco/tracker-client/vars"
	"github.com/spf13/cobra"
)

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

func init() {
	RootCmd.AddCommand(get)
}
