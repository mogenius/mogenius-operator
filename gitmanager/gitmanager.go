package gitmanager

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func InitGit(path string) error {
	_, err := git.PlainInit(path, false)
	if err != nil && err != git.ErrRepositoryAlreadyExists {
		return err
	}
	return nil
}

func Clone(url, path string) error {
	_, err := git.PlainClone(path, false, &git.CloneOptions{
		URL: url,
	})
	if err != nil {
		return err
	}
	return nil
}

func CloneFast(url, path, branch string) error {
	_, err := git.PlainClone(path, false, &git.CloneOptions{
		URL:           url,
		Progress:      nil,
		SingleBranch:  true,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
	})
	if err != nil {
		return err
	}
	return nil
}

func DeletePath(path string) error {
	err := os.RemoveAll(path)
	if err != nil {
		return err
	}
	return nil
}

func Pull(path string, remote string, branchNane string) error {
	// We instantiate a new repository targeting the given path
	r, err := git.PlainOpen(path)
	if err != nil {
		return err
	}

	// Get the working directory for the repository
	w, err := r.Worktree()
	if err != nil {
		return err
	}

	// Pull the latest changes from the origin remote and merge into the current branch
	err = w.Pull(&git.PullOptions{
		RemoteName:    remote,
		ReferenceName: plumbing.NewBranchReferenceName(branchNane),
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}

	return nil
}

func Push(path string, remote string) error {
	r, err := git.PlainOpen(path)
	if err != nil {
		return err
	}
	err = r.Push(&git.PushOptions{
		RemoteName: remote,
	})
	if err != nil {
		return err
	}
	return nil
}

func GetLastCommit(path string) (*object.Commit, error) {
	// We instantiate a new repository targeting the given path
	r, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}

	// the latest commit
	ref, err := r.Head()
	if err != nil {
		return nil, err
	}
	commit, err := r.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	return commit, nil
}

func Commit(repoPath string, changedfilePaths []string, message, authorName, authorEmail string) error {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	for _, filePath := range changedfilePaths {
		_, err = w.Add(filePath)
		if err != nil {
			return fmt.Errorf("Error adding file %s: %s", filePath, err.Error())
		}
	}

	_, err = w.Status()
	if err != nil {
		return err
	}

	commit, err := w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  authorName,
			Email: authorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return err
	}

	_, err = r.CommitObject(commit)
	if err != nil {
		return err
	}

	return nil
}

func CheckoutBranch(path, branchName string) error {
	err := Fetch(path)
	if err != nil {
		return err
	}

	r, err := git.PlainOpen(path)
	if err != nil {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewRemoteReferenceName("origin", branchName),
		Force:  true,
	})
	if err != nil {
		return err
	}

	return nil
}

func SwitchBranch(path, branchName string) error {
	return CheckoutBranch(path, branchName)
}

func Fetch(path string) error {
	r, err := git.PlainOpen(path)
	if err != nil {
		return err
	}

	err = r.Fetch(&git.FetchOptions{})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}

	return nil
}

func ListRemoteBranches(path string) ([]string, error) {
	branches := []string{}
	r, err := git.PlainOpen(path)
	if err != nil {
		return branches, err
	}

	refs, err := r.References()
	if err != nil {
		return branches, err
	}
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsRemote() {
			branches = append(branches, strings.TrimPrefix(ref.Name().Short(), "origin/"))
		}
		return nil
	})
	return branches, err
}

// Comparable to "git tag --contains HEAD"
func GetHeadTag(path string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", err
	}

	headRef, err := repo.Head()
	if err != nil {
		return "", err
	}
	headCommitHash := headRef.Hash()

	tags, err := repo.Tags()
	if err != nil {
		return "", err
	}

	tagName := ""
	err = tags.ForEach(func(tagRef *plumbing.Reference) error {
		commitIter, err := repo.Log(&git.LogOptions{From: tagRef.Hash()})
		if err != nil {
			return err
		}

		defer commitIter.Close()

		commitIter.ForEach(func(commit *object.Commit) error {
			if commit.Hash == headCommitHash {
				tagName = tagRef.Name().Short()
				return nil
			}
			return nil
		})
		return nil
	})

	return tagName, err
}

func AddRemote(path string, remoteUrl string, remoteName string) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return err
	}

	remote, err := repo.Remote(remoteName)
	if err == nil {
		// remote already exists in the repo
		if remote.Config().URLs[0] == remoteUrl && remote.Config().Name == remoteName {
			// remote already exists with the same URL
			return nil
		} else {
			// remote already exists with a different URL so we delete it
			err = repo.DeleteRemote(remoteName)
			if err != nil {
				return fmt.Errorf("failed to delete remote: %v", err)
			}
		}
	}

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: remoteName,
		URLs: []string{remoteUrl},
	})
	if err != nil && err != git.ErrRemoteExists {
		return fmt.Errorf("failed to create remote: %v", err)
	}

	return nil
}

func LsRemotes(url string) ([]string, error) {
	result := []string{}

	remote := git.NewRemote(nil, &config.RemoteConfig{
		URLs: []string{url},
	})

	// Fetch references from the remote
	refs, err := remote.List(&git.ListOptions{})
	if err != nil {
		return result, fmt.Errorf("failed to list references from remote: %v", err)
	}

	// Print all references
	for _, ref := range refs {
		result = append(result, fmt.Sprintf("%s\t%s\n", ref.Hash(), ref.Name()))
	}

	return result, nil
}

func ListLocalAvailableRemotes(path string) ([]string, error) {
	result := []string{}

	repo, err := git.PlainOpen(path)
	if err != nil {
		return result, err
	}

	remotes, err := repo.Remotes()
	if err != nil {
		return result, err
	}

	for _, remote := range remotes {
		config := remote.Config()
		result = append(result, config.URLs...)
	}

	return result, nil
}

func HasRemotes(path string) bool {
	remotes, err := ListLocalAvailableRemotes(path)
	if err != nil {
		return false
	}
	return len(remotes) > 0
}

func LastLogDecorate(path string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", err
	}

	headRef, err := repo.Head()
	if err != nil {
		return "", err
	}

	commit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return "", err
	}

	green := color.New(color.FgGreen).SprintFunc()
	blue := color.New(color.FgBlue).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	decorations := findDecorations(repo, headRef.Hash())
	shortHash := commit.Hash.String()[:7]
	result := fmt.Sprintf("commit %s %s\n", green(shortHash), blue(strings.Join(decorations, ", ")))
	result += fmt.Sprintf("Author: %s <%s>\n", yellow(commit.Author.Name), commit.Author.Email)
	result += fmt.Sprintf("Date:   %s\n\n", commit.Author.When.Format("Mon Jan 2 15:04:05 2006 -0700"))
	result += fmt.Sprintf("    %s\n", commit.Message)

	return result, nil
}

// git diff HEAD@{1} HEAD --name-only --diff-filter=AM
func GetLastUpdatedAndModifiedFiles(path string) ([]string, error) {
	result := []string{}

	repo, err := git.PlainOpen(path)
	if err != nil {
		return result, err
	}

	headRef, err := repo.Head()
	if err != nil {
		return result, err
	}

	headCommit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return result, err
	}

	prevHeadCommit, err := headCommit.Parent(0)
	if err != nil && err != object.ErrParentNotFound {
		return result, err
	}

	if prevHeadCommit == nil {
		return result, nil
	}

	// Compute the diff between the current HEAD and the previous HEAD state
	patch, err := prevHeadCommit.Patch(headCommit)
	if err != nil {
		return result, err
	}

	result = getAddedOrModifiedFiles(patch)

	return result, nil
}

// git diff HEAD@{1} HEAD --name-only --diff-filter=D
func GetLastDeletedFiles(path string) ([]string, error) {
	result := []string{}

	repo, err := git.PlainOpen(path)
	if err != nil {
		return result, err
	}

	headRef, err := repo.Head()
	if err != nil {
		return result, err
	}

	headCommit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return result, err
	}

	prevHeadCommit, err := headCommit.Parent(0)
	if err != nil && err != object.ErrParentNotFound {
		return result, err
	}

	if prevHeadCommit == nil {
		return result, nil
	}

	// Compute the diff between the current HEAD and the previous HEAD state
	patch, err := prevHeadCommit.Patch(headCommit)
	if err != nil {
		return result, err
	}

	result = getDeletedFiles(patch)

	return result, nil
}

func getAddedOrModifiedFiles(patch *object.Patch) []string {
	var updatedFiles []string

	for _, stat := range patch.Stats() {
		// Filter for added (A) or modified (M) files
		if stat.Addition > 0 && stat.Deletion == 0 {
			updatedFiles = append(updatedFiles, stat.Name)
		} else if stat.Addition > 0 {
			updatedFiles = append(updatedFiles, stat.Name)
		}
	}

	return updatedFiles
}

func getDeletedFiles(patch *object.Patch) []string {
	var deletedFiles []string

	for _, filePatch := range patch.FilePatches() {
		from, to := filePatch.Files()
		if to == nil && from != nil {
			deletedFiles = append(deletedFiles, from.Path())
		}
	}

	return deletedFiles
}

func findDecorations(repo *git.Repository, hash plumbing.Hash) []string {
	var decorations []string

	refs, err := repo.References()
	if err != nil {
		fmt.Printf("Error getting references %s\n", err)
	}

	refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() == plumbing.HashReference && ref.Hash() == hash {
			if ref.Name().IsBranch() || ref.Name().IsTag() {
				decorations = append(decorations, ref.Name().String())
			}
		}
		return nil
	})

	return decorations
}
