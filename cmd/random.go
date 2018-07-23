package cmd

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/hypotheticalco/tracker-client/vars"
	"github.com/spf13/cobra"
)

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

func init() {
	RootCmd.AddCommand(random)
}
