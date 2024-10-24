package repo

import (
	"errors"
	"os"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"
	"golang.org/x/crypto/ssh"

	"github.com/bricef/htt/internal/utils"
	"github.com/bricef/htt/internal/vars"
	"github.com/go-git/go-git/v5/plumbing"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

var (
	ErrWorkingTreeUnchanged = errors.New("no changes to relevant files")
)

type PushDetails struct {
	RemoteName string
	Hash       string
}

// EnsureAndGetDataRepo will ensure that the git repository
// for the user data exists and is initialised or fail fatally
func EnsureAndGetDataRepo(path string) *git.Repository {

	repo, err := git.PlainOpen(path)

	if err == git.ErrRepositoryNotExists {
		utils.Info("Could not open repository. Attempting to create one instead.")
		repo, err = git.PlainInit(path, false)
		if err != nil {
			utils.DieOnError("Fatal: Failed ot init a plain git directory: ", err)
		}
	} else if err != nil {
		utils.DieOnError("Fatal: Ensuring a data repository failed: ", err)
	}

	return repo
}

// createOriginRemote creates a remote named Origin with the remoteURL
func createOriginRemote(repo *git.Repository, remoteURL string) (*git.Remote, error) {
	remote, err := repo.CreateRemote(&gitconfig.RemoteConfig{
		Name:  vars.Get(vars.ConfigKeyRemoteName),
		URLs:  []string{remoteURL},
		Fetch: []gitconfig.RefSpec{"+refs/heads/*:refs/remotes/foobar/*"},
	})
	return remote, err
}

// EnsureOriginRemote will ensure that the data repository has a properly configured remote
func EnsureOriginRemote(repo *git.Repository, remoteURL string) (*git.Remote, error) {
	var remoteName = vars.Get(vars.ConfigKeyRemoteName)

	// Get a remote
	remote, err := repo.Remote(remoteName)
	if err == git.ErrRemoteNotFound {
		remote, err = createOriginRemote(repo, remoteURL)
	} else if err != nil {
		utils.DieOnError("Fatal: Error occured when getting remote configuration: ", err)
	}

	// Check the remote is properly configured
	config := remote.Config()
	if config.URLs[0] != remoteURL { // Bad remote url
		err = repo.DeleteRemote(remoteName)
		utils.DieOnError("Could not delete incorrectly configured remote: ", err)

		remote, err = createOriginRemote(repo, remoteURL)
		utils.DieOnError("Failed to create new correctly configured remote: ", err)
	}
	return remote, err
}

func statusRequiresCommit(status git.Status) bool {
	for _, fileStatus := range status {
		switch fileStatus.Worktree {
		case git.Modified, git.Added, git.Deleted, git.Renamed, git.Copied:
			return true
		}
		switch fileStatus.Staging {
		case git.Modified, git.Added, git.Deleted, git.Renamed, git.Copied:
			return true
		}
	}
	return false
}

// CreateCommitFromChanges will create a commit from the changes to the working copy
func CreateCommitFromChanges(repo *git.Repository, message string) (plumbing.Hash, error) {
	wt, err := repo.Worktree()
	utils.DieOnError("Failed to gain access to the work tree: ", err)

	patterns := vars.GetStringSlice(vars.ConfigKeyFilePatterns)
	for _, pattern := range patterns {
		wt.AddGlob(pattern)
	}
	status, err := wt.Status()
	utils.DieOnError("Could not acertain status of working directory: ", err)

	if statusRequiresCommit(status) {
		hash, err := wt.Commit(message, &git.CommitOptions{
			// All: true,
			Author: &object.Signature{
				Name:  vars.Get(vars.ConfigKeyAuthorName),
				Email: vars.Get(vars.ConfigKeyAuthorEmail),
				When:  time.Now(),
			},
			Committer: &object.Signature{
				Name:  vars.DefaultCommitterName,
				Email: vars.DefaultCommitterEmail,
				When:  time.Now(),
			},
		})
		utils.DieOnError("Fatal. Failed to commit changes to data directory: ", err)
		return hash, nil
	}
	// default to noop
	return plumbing.Hash{}, ErrWorkingTreeUnchanged

}

func getSSHAuth() *gitssh.PublicKeys {

	sshKey, err := os.ReadFile(vars.Get(vars.ConfigKeySSHKeyFile))
	utils.DieOnError("Failed to read SSH Key: ", err)

	signer, err := ssh.ParsePrivateKey([]byte(sshKey))
	utils.DieOnError("Failed to parse SSH key: ", err)

	return &gitssh.PublicKeys{
		User:   "git",
		Signer: signer,
	}
}

// PushChanges will Push changes in has to default remote repo.
func PushChanges(repo *git.Repository, hash plumbing.Hash) (PushDetails, error) {
	remote, err := repo.Remote(vars.Get(vars.ConfigKeyRemoteName))
	utils.DieOnError("Failed to access remote when pushing: ", err)

	err = remote.Push(&git.PushOptions{Auth: getSSHAuth()}) // use default options
	if err == git.NoErrAlreadyUpToDate {
		return PushDetails{
			RemoteName: remote.Config().Name,
			Hash:       hash.String(),
		}, err
	} else if err != nil {
		utils.DieOnError("Failed to push to remote "+remote.Config().Name+": ", err)
	}

	return PushDetails{
		RemoteName: remote.Config().Name,
		Hash:       hash.String(),
	}, nil
}

// func ensureDirectory(dir string) {

// }

// func repoExists(dir string) bool {

// }

// func remoteExists(remoteURL string) bool {

// }

// func cloneRepo(remoteURL string, directory string) {

// }

// func createRepo(dataDir string) {

// }

// func getRepo() *git.Repository {
// 	return &git.Repository{}
// }

// Sync will attempt to sync the local repo to the remote repo.
// Sync will fail is the remote repo is further ahead.
// TODO: handle remote repo ahead gracefully
func Sync() {

	dataDir := vars.Get(vars.ConfigKeyDataDir)
	remoteRepoURL := vars.Get(vars.ConfigKeyRepoURL)

	// ensureDirectory(dataDir)

	// if !repoExists(dataDir) {
	// 	if remoteExists(remoteRepoURL) {
	// 		cloneRepo(remoteRepoURL, dataDir)
	// 	} else {
	// 		createRepo(dataDir)
	// 	}
	// }

	// r := getRepo()
	r := EnsureAndGetDataRepo(dataDir)

	// Ensure we have a master
	EnsureOriginRemote(r, remoteRepoURL)

	// Create commit
	hash, err := CreateCommitFromChanges(r, "Commiting state via tracker client sync")
	if err != nil {
		if err == ErrWorkingTreeUnchanged {
			utils.Info("No relevant changes to be commited")
		} else {
			utils.Fatal("Could not commit changes due to error: ", err)
		}
	} else {
		utils.Success("Created commit of changes with hash ", hash.String())
	}

	// push commit
	details, err := PushChanges(r, hash)
	if err == git.NoErrAlreadyUpToDate {
		utils.Info("Remote " + details.RemoteName + " is already up to date")
	} else if err == nil {
		utils.Success("Pushed commit " + details.Hash + " to " + details.RemoteName)
	}

}
