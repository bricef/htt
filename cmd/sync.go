package cmd

import (
	"github.com/hypotheticalco/tracker-client/repo"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(&cobra.Command{
		Use:   "sync",
		Short: "Sync the data to the backend manually",
		Long: `You shouldn't need to use this command under normal conditions.
Your data will be synced after modification automatically.

Use this when you manage your todos using another program 
and still want to benefit from syncing.

Hypothetical Tracker uses Git under the hood to keep track
of the todos and time tracking list. You can commit a
dditional files in the git repository and they will also 
be synced to the backend.

In addition, you can retarget Hypothetical Tracker to use 
yourown repository. See hypothetical.co/tracker/docs for
more info.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			repo.Sync()
		},
	})
}
