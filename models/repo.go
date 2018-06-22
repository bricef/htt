package models

import (
	"log"

	"github.com/hypotheticalco/tracker-client/vars"

	"github.com/spf13/viper"

	git "gopkg.in/src-d/go-git.v4"
	gitconfig "gopkg.in/src-d/go-git.v4/config"
)

// EnsureAndGetDataRepo will ensure that the git repository
// for the user data exists and is initialised or fail fatally
func EnsureAndGetDataRepo(path string) *git.Repository {

	repo, err := git.PlainOpen(path)

	if err == git.ErrRepositoryNotExists {
		log.Println("Could not open repository. Attempting to create one instead.")
		repo, err = git.PlainInit(path, false)
	}

	if err != nil {
		log.Fatal("Fatal: Ensuring a data repository failed:", err)
	}

	return repo
}

// createOriginRemote creates a remote named Origin with the remoteURL
func createOriginRemote(repo *git.Repository, remoteURL string) (*git.Remote, error) {
	// config := gitconfig.RemoteConfig{
	// 	Name:  viper.GetString(vars.ConfigKeyRemoteName),
	// 	URLs:  []string{remoteURL},
	// 	Fetch: []gitconfig.RefSpec{"+refs/heads/*:refs/remotes/foobar/*"},
	// }
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
		log.Fatal("Fatal: Error occured when getting remote configuration:", err)
	}

	// Check the remote is properly configured
	config := remote.Config()
	if config.URLs[0] != remoteURL { // Bad remote url
		err = repo.DeleteRemote(remoteName)
		if err != nil {
			log.Fatal("Fatal: Could not delete incorrectly configured remote.")
		}

		remote, err = createOriginRemote(repo, remoteURL)
		if err != nil {
			log.Fatal("Fatal: Failed to create new correctly configured remote. ")
		}
	}
	return remote, err
}

// SyncData will sync the current data to the remote repository.
// func SyncData() error {
// 	var path = viper.GetString("datadir")
// 	repo, err := ensureAndGetDataRepo(path)
// 	log.Println(repo.Config())
// 	return err
// }
