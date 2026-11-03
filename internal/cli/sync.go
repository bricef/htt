package cli

import (
	gitsync "github.com/bricef/htt/internal/repo"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(&cobra.Command{
		Use:   "sync",
		Short: "Commit the data directory and push it to the backing git repo.",
		Long: `Commit the data directory and push it to the backing git repo.

Sync only runs when you run this command; nothing syncs
automatically. It turns the data directory into a git repository
(branch master) if it isn't one, commits the files matching
file_match_patterns, and pushes over SSH to backing_repo_url using
the private key in ssh_key_file. It never pulls.

Before the first run, set in your config file:
  backing_repo_url   an SSH remote, e.g. git@github.com:you/tasks.git
  ssh_key_file       a private key without a passphrase (the default,
                     ~/.ssh/id_rsa.pub, is a public key and won't work)

Set backing_repo_url before running sync: without it, sync pushes
into whatever git repository the current directory belongs to.

To use a second machine, clone the backing repo into its data
directory before its first sync. If another machine has pushed since
your last sync, the push is rejected: in the data directory run
git pull --no-rebase origin master, resolve any conflicts, then sync
again. HTTPS remotes and passphrase-protected keys aren't supported.

Known problems and limits: https://github.com/bricef/htt/issues/12`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			gitsync.Sync()
			return nil
		},
	})
}
