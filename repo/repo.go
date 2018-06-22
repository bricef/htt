package repo

import (
	"errors"
	"io/ioutil"
	"time"

	"golang.org/x/crypto/ssh"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	"log"

	"github.com/hypotheticalco/tracker-client/utils"
	"github.com/hypotheticalco/tracker-client/vars"
	"github.com/spf13/viper"
	"gopkg.in/src-d/go-git.v4/plumbing"

	git "gopkg.in/src-d/go-git.v4"
	gitconfig "gopkg.in/src-d/go-git.v4/config"
	gitssh "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
)

var (
	WorkingTreeUnchanged = errors.New("No changes to relevant files")
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
		log.Println("Could not open repository. Attempting to create one instead.")
		repo, err = git.PlainInit(path, false)
	} else if err != nil {
		utils.DieOnError("Fatal: Ensuring a data repository failed: ", err)
	}

	return repo
}

// createOriginRemote creates a remote named Origin with the remoteURL
func createOriginRemote(repo *git.Repository, remoteURL string) (*git.Remote, error) {
	remote, err := repo.CreateRemote(&gitconfig.RemoteConfig{
		Name:  viper.GetString(vars.ConfigKeyRemoteName),
		URLs:  []string{remoteURL},
		Fetch: []gitconfig.RefSpec{"+refs/heads/*:refs/remotes/foobar/*"},
	})
	return remote, err
}

// EnsureOriginRemote will ensure that the data repository has a properly configured remote
func EnsureOriginRemote(repo *git.Repository, remoteURL string) (*git.Remote, error) {
	var remoteName = viper.GetString(vars.ConfigKeyRemoteName)

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
		default:
			break
		}
		switch fileStatus.Staging {
		case git.Modified, git.Added, git.Deleted, git.Renamed, git.Copied:
			return true
		default:
			break
		}
	}
	return false
}

// CreateCommitFromChanges will create a commit from the changes to the working copy
func CreateCommitFromChanges(repo *git.Repository, message string) (plumbing.Hash, error) {
	wt, err := repo.Worktree()
	utils.DieOnError("Failed to gain access to the work tree: ", err)

	patterns := viper.GetStringSlice(vars.ConfigKeyFilePatterns)
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
	return plumbing.Hash{}, WorkingTreeUnchanged

}

func getSSHAuth() *gitssh.PublicKeys {

	sshKey, err := ioutil.ReadFile(vars.Get(vars.ConfigKeySSHKeyFile))
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
	remote, err := repo.Remote(viper.GetString(vars.ConfigKeyRemoteName))
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

func Sync() {

	// get repo
	r := EnsureAndGetDataRepo(vars.Get(vars.ConfigKeyDataDir))

	// Ensure we have a master
	EnsureOriginRemote(r, vars.Get(vars.ConfigKeyRepoURL))

	// Create commit
	hash, err := CreateCommitFromChanges(r, "Commiting state via ht sync")
	if err != nil {
		if err == WorkingTreeUnchanged {
			log.Println("✓ No relevant changes to be commited")
		} else {
			log.Fatal("Could not commit changes due to error: ", err)
		}
	} else {
		log.Println("✓ Created commit of changes with hash ", hash.String())
	}

	// push commit
	details, err := PushChanges(r, hash)
	if err == git.NoErrAlreadyUpToDate {
		log.Println("✓ Remote " + details.RemoteName + " is already up to date")
	} else if err == nil {
		log.Println("✓ Pushed commit " + details.Hash + " to " + details.RemoteName)
	}

}

// SyncData will sync the current data to the remote repository.
// func SyncData() error {
// 	var path = viper.GetString("datadir")
// 	repo, err := ensureAndGetDataRepo(path)
// 	log.Println(repo.Config())
// 	return err
// }
