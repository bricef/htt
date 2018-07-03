package cmd

import (
	"fmt"

	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/spf13/cobra"
)

var context = &cobra.Command{
	Use:     "context [context]",
	Short:   "Change the context for tasks",
	Aliases: []string{"switch"},
	Long: `This will change the default context for todos. Note that 
the only supported characters are [A-Za-z_]. Other 
characters will be replaced by underscores. This 
means that different arguments may map to the 
same context.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			contexts := todo.GetContexts()
			current := todo.GetCurrentContext()
			fmt.Printf("➤ %s\n", current)
			for _, c := range contexts {
				if current != c {
					fmt.Printf("%s\n", c)
				}
			}
			// fmt.Print("\n")
		} else {
			todo.SetContext(args[0])
			println("Now using context: " + args[0])
		}
	},
}

func init() {
	RootCmd.AddCommand(context)
}
