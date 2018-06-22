package cmd

import (
	"log"

	"github.com/hypotheticalco/tracker-client/models"
	"github.com/hypotheticalco/tracker-client/vars"
	"github.com/spf13/cobra"
	git "gopkg.in/src-d/go-git.v4"
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
			// get repo
			repo := models.EnsureAndGetDataRepo(vars.Get(vars.ConfigKeyDataDir))

			// Ensure we have a master
			models.EnsureOriginRemote(repo, vars.Get(vars.ConfigKeyRepoURL))

			// Create commit
			hash, err := models.CreateCommitFromChanges(repo, "Commiting state via ht sync")
			if err != nil {
				if err == models.WorkingTreeUnchanged {
					log.Println("No relevant changes to be commited")
				} else {
					log.Fatal("Could not commit changes due to error: ", err)
				}
			} else {
				log.Println("Created commit of changes with hash ", hash.String())
			}

			// push commit
			details, err := models.PushChanges(repo, hash)
			if err == git.NoErrAlreadyUpToDate {
				log.Println("Remote " + details.RemoteName + " is already up to date")
			} else if err == nil {
				log.Println("Pushed commit " + details.Hash + " to " + details.RemoteName)
			}

		},
	})
}
