package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/bricef/htt/timelogs"
	"github.com/bricef/htt/todo"
	"github.com/bricef/htt/utils"
	"github.com/fatih/color"
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
		currentTask := timelogs.CurrentActive()
		i, err := utils.StringSliceIndex(contexts, current)
		if err == nil {
			contexts = append(contexts[:i], contexts[i+1:]...)
		}
		fmt.Printf("Current task context: %v\n", color.GreenString(current))
		fmt.Printf("Tasks outstanding: %v\n", len(todos))

		if len(contexts) != 0 {
			fmt.Printf("Available contexts: %v\n", strings.Join(contexts, ", "))
		}

		if currentTask != nil {
			startedAt := currentTask.Annotations["start"]
			startTime, err := time.Parse(time.RFC3339, startedAt)
			if err != nil {
				utils.Fatal("Failed to parse log entry.")
			}
			duration := utils.HumanizeDuration(time.Since(startTime))
			fmt.Printf("Currently working on '%v' (%v) \n", currentTask.RemoveAnnotation("start").ColorString(), duration)
		} else {
			fmt.Printf("Not currently working on any task.\n")
		}

	},
}

func init() {
	RootCmd.AddCommand(status)
}
