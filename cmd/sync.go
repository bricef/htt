package cmd

import (
	"github.com/hypotheticalco/tracker-client/models"
	"github.com/hypotheticalco/tracker-client/vars"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(&cobra.Command{
		Use:   "sync",
		Short: "Sync the data to the backend manually",
		Long: `You shouldn't need to use this command under normal conditions.
Your data will be synced after modification automatically.

Use this when you manage your todos using another program 
and still want to benefit from syncing.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			repo := models.EnsureAndGetDataRepo(vars.Get(vars.ConfigKeyDataDir))
			models.EnsureOriginRemote(repo, vars.Get(vars.ConfigKeyRepoURL))

		},
	})
}
