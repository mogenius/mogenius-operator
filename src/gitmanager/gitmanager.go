package gitmanager

import (
	"errors"
	"fmt"
	"mogenius-operator/src/shell"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type CommitRevision struct {
	Hash                string `json:"hash"`
	Author              string `json:"author"`
	Date                string `json:"date"`
	DiffToPreviosCommit string `json:"diff"`
}

const (
	Max_Commit_History = 10
	Max_Diff_Lines     = 5000
	Max_Diff_Count     = 10
)

var DiffFound = errors.New("diff found")
var MaxDiffsFound = errors.New("Max_Diff_Count hit")

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
	if err != nil && err != git.ErrRepositoryAlreadyExists {
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

func Pull(path string, remote string, branchNane string) (lastCommit *object.Commit, error error) {
	// We instantiate a new repository targeting the given path
	r, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}

	// Get the working directory for the repository
	w, err := r.Worktree()
	if err != nil {
		return nil, err
	}

	// Pull the latest changes from the origin remote and merge into the current branch
	err = w.Pull(&git.PullOptions{
		SingleBranch:  true,
		RemoteName:    remote,
		ReferenceName: plumbing.NewBranchReferenceName(branchNane),
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return nil, err
	}

	ref, err := r.Head()
	if err != nil {
		return nil, err
	}

	return r.CommitObject(ref.Hash())
}

func Push(path string, remote string) error {
	r, err := git.PlainOpen(path)
	if err != nil {
		return err
	}
	err = r.Push(&git.PushOptions{
		RemoteName: remote,
		Force:      true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}
	return nil
}

func GetLastCommits(path string, maxNoOfEntries int) ([]*object.Commit, error) {
	// We instantiate a new repository targeting the given path
	r, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}

	// Get the commit history starting from the latest commit
	commitIter, err := r.Log(&git.LogOptions{
		Order: git.LogOrderCommitterTime,
	})
	if err != nil {
		return nil, err
	}

	// Collect the last n commits
	var commits []*object.Commit
	count := 0
	err = commitIter.ForEach(func(c *object.Commit) error {
		if count >= maxNoOfEntries {
			return nil // stop iteration once we have n commits
		}
		commits = append(commits, c)
		count++
		return nil
	})
	if err != nil {
		return nil, err
	}

	return commits, nil
}

func Commit(repoPath string, addedAndUpdatedFils []string, deletedFiles []string, message, authorName, authorEmail string) error {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	// clean file path to make them relative
	for index, filePath := range addedAndUpdatedFils {
		addedAndUpdatedFils[index] = strings.TrimPrefix(filePath, repoPath+"/")
	}
	// clean file path to make them relative
	for index, filePath := range deletedFiles {
		deletedFiles[index] = strings.TrimPrefix(filePath, repoPath+"/")
	}

	// ADD
	for _, filePath := range addedAndUpdatedFils {
		_, err = w.Add(filePath)
		if err != nil {
			return fmt.Errorf("Error adding file %s: %s", filePath, err.Error())
		}
	}
	// REMOVE
	for _, filePath := range deletedFiles {
		_, err = w.Remove(filePath)
		if err != nil {
			return fmt.Errorf("Error removing file %s: %s", filePath, err.Error())
		}
	}

	_, err = w.Status()
	if err != nil {
		return err
	}

	commit, err := w.Commit(message, &git.CommitOptions{
		All:               true,
		AllowEmptyCommits: true,
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

	// Check if the branch exists locally
	branchRef := plumbing.NewBranchReferenceName(branchName)
	_, err = r.Reference(branchRef, true)
	switch err {
	case nil:
		// Branch exists locally, check it out
		err = w.Checkout(&git.CheckoutOptions{
			Branch: branchRef,
		})
	case plumbing.ErrReferenceNotFound:
		// Branch doesn't exist locally, create and track the remote branch
		err = w.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewRemoteReferenceName("origin", branchName),
			Create: true,
			Force:  true,
			Keep:   false,
		})
	}
	if err != nil {
		return err
	}

	return nil
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
		commitIter, err := repo.Log(&git.LogOptions{Order: git.LogOrderCommitterTime, From: tagRef.Hash()})
		if err != nil {
			return err
		}

		defer commitIter.Close()

		err = commitIter.ForEach(func(commit *object.Commit) error {
			if commit.Hash == headCommitHash {
				tagName = tagRef.Name().Short()
				return nil
			}
			return nil
		})
		if err != nil {
			return err
		}
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

func UnifiedDiff(filePath1 string, filePath2 string, resourceName string) (string, error) {
	cmd := exec.Command("diff", "-u", "-N", "-u", "--label", resourceName, "--label", resourceName, filePath1, filePath2)
	cmd.Dir = os.TempDir()
	out, err := cmd.CombinedOutput()

	if err != nil {
		// diff returns exit code 1 if files differ
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				return string(out), nil
			} else {
				return "", fmt.Errorf("Error running diff: %s\n%s\n", err.Error(), string(out))
			}
		} else {
			return "", err
		}
	}
	return "", nil
}

func DiffForCommit(path string, commitHash string, filePath string, resourceName string) (string, error) {
	diffOutput := ""

	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	commitIter, err := repo.Log(
		&git.LogOptions{
			From:     plumbing.NewHash(commitHash),
			Order:    git.LogOrderCommitterTime,
			FileName: &filePath,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve commit history: %w", err)
	}

	err = commitIter.ForEach(func(commit *object.Commit) error {
		// If the commit has a parent, compare it with the parent commit
		if commit.NumParents() == 0 {
			// No parent means it's the first commit, so stop
			return nil
		}

		parentCommit, err := commit.Parent(0)
		if err != nil {
			return fmt.Errorf("failed to get parent commit: %w", err)
		}

		// Get the tree objects for the current commit and its parent
		commitTree, err := commit.Tree()
		if err != nil {
			return fmt.Errorf("failed to get tree for commit: %w", err)
		}

		parentTree, err := parentCommit.Tree()
		if err != nil {
			return fmt.Errorf("failed to get tree for parent commit: %w", err)
		}

		// Get the file content at 'filePath' in current commit
		fileEntryCurrent, err := commitTree.File(filePath)
		var contentCurrent string
		if err == nil {
			contentCurrent, err = fileEntryCurrent.Contents()
			if err != nil {
				return fmt.Errorf("failed to get content of file in current commit: %w", err)
			}
		}

		// Get the file content at 'filePath' in parent commit
		fileEntryParent, err := parentTree.File(filePath)
		var contentParent string
		if err == nil {
			contentParent, err = fileEntryParent.Contents()
			if err != nil {
				return fmt.Errorf("failed to get content of file in parent commit: %w", err)
			}
		}

		// Write contents to temp files
		current := resourceName
		parent := "_" + resourceName
		err = os.WriteFile(os.TempDir()+"/"+current, []byte(contentCurrent), 0644)
		if err != nil {
			return fmt.Errorf("failed to write to temp file (%s): %w", current, err)
		}
		defer os.Remove(os.TempDir() + "/" + current)

		err = os.WriteFile(os.TempDir()+"/"+parent, []byte(contentParent), 0644)
		if err != nil {
			return fmt.Errorf("failed to write to temp file (%s): %w", parent, err)
		}
		defer os.Remove(os.TempDir() + "/" + parent)

		diffOutput, _ = UnifiedDiff(parent, current, resourceName)
		if diffOutput != "" {
			return DiffFound
		}

		return nil
	})
	if err != DiffFound {
		return "", fmt.Errorf("failed to iterate through commit history: %w", err)
	}

	// If no diff found for the file, return an error
	if len(diffOutput) == 0 {
		return "", fmt.Errorf("no changes found for the file: %s", filePath)
	}

	// shorten the diff output if it's too long
	if len(diffOutput) > Max_Diff_Lines {
		diffOutput = diffOutput[:Max_Diff_Lines] + "..."
	}
	return diffOutput, nil
}

func ListFileRevisions(repoPath string, filePath string, resourceName string) ([]CommitRevision, error) {
	revisions := []CommitRevision{}

	// Open the repository
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return revisions, err
	}

	// Get the commit history of the specific file
	iter, err := repo.Log(&git.LogOptions{
		FileName: &filePath,
		Order:    git.LogOrderCommitterTime,
	})
	if err != nil {
		return revisions, err
	}

	err = iter.ForEach(func(c *object.Commit) error {
		diff, _ := DiffForCommit(repoPath, c.Hash.String(), filePath, resourceName)
		if diff != "" {
			revisions = append(revisions, CommitRevision{
				Hash:                c.Hash.String(),
				Author:              c.Author.Name,
				Date:                c.Author.When.Format(time.RFC3339),
				DiffToPreviosCommit: diff,
			})
		}
		if len(revisions) >= Max_Diff_Count {
			return MaxDiffsFound
		}
		return nil
	})

	if err != nil && err != MaxDiffsFound {
		return nil, err
	}

	return revisions, nil
}

// Get a list of all contributors (unique authors)
func GetContributors(path string) ([]object.Signature, error) {
	contributorSet := make(map[string]object.Signature)

	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}

	iter, err := repo.Log(&git.LogOptions{Order: git.LogOrderCommitterTime})
	if err != nil {
		return nil, err
	}

	err = iter.ForEach(func(c *object.Commit) error {
		contributorSet[c.Author.Name] = c.Author
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Convert the map to a list
	contributors := make([]object.Signature, 0, len(contributorSet))
	for _, contributor := range contributorSet {
		contributors = append(contributors, contributor)
	}

	return contributors, nil
}

func GeneratePulseDiagramData(repoPath string) (map[string]int, error) {
	commitsPerDay := make(map[string]int)

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return commitsPerDay, err
	}

	ref, err := repo.Head()
	if err != nil {
		return commitsPerDay, err
	}

	commitIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return commitsPerDay, err
	}

	moreThan90Error := errors.New("more than 90 days found")

	err = commitIter.ForEach(func(c *object.Commit) error {
		week := c.Author.When.UTC().Format("2006-01-02") // YYYY-MM-DD
		commitsPerDay[week]++

		if len(commitsPerDay) > 90 {
			return moreThan90Error
		}
		return nil
	})
	if err != nil && err != moreThan90Error {
		return commitsPerDay, err
	}

	return commitsPerDay, nil
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

	decorations := findDecorations(repo, headRef.Hash())
	shortHash := commit.Hash.String()[:7]
	result := fmt.Sprintf("commit %s %s\n", shell.Colorize(shortHash, shell.Green), shell.Colorize(strings.Join(decorations, ", "), shell.Blue))
	result += fmt.Sprintf("Author: %s <%s>\n", shell.Colorize(commit.Author.Name, shell.Yellow), commit.Author.Email)
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

func ResetFileToCommit(repoPath, commitHash, filePath string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	hash := plumbing.NewHash(commitHash)
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return fmt.Errorf("failed to find commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return fmt.Errorf("failed to get tree: %w", err)
	}

	file, err := tree.File(filePath)
	if err != nil {
		return fmt.Errorf("failed to find file (%s) in tree: %w", filePath, err)
	}

	content, err := file.Contents()
	if err != nil {
		return fmt.Errorf("failed to read file content: %w", err)
	}

	completePath := repoPath + "/" + filePath
	err = os.WriteFile(completePath, []byte(content), os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to overwrite local file (%s): %w", completePath, err)
	}

	return nil
}

func ResetHard(path string) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("failed to open repository: %v", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %v", err)
	}

	err = worktree.Reset(&git.ResetOptions{
		Mode:   git.HardReset,
		Commit: plumbing.ZeroHash, // Reset to the current HEAD
	})
	if err != nil {
		return fmt.Errorf("failed to reset hard: %v", err)
	}
	return nil
}

func DiscardUnstagedChanges(repoPath, filePath string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repo: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return fmt.Errorf("failed to get commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return fmt.Errorf("failed to get tree: %w", err)
	}

	file, err := tree.File(filePath)
	if err != nil {
		return fmt.Errorf("failed to find file in tree: %w", err)
	}

	content, err := file.Contents()
	if err != nil {
		return fmt.Errorf("failed to read file content from HEAD: %w", err)
	}

	completePath := repoPath + "/" + filePath
	err = os.WriteFile(completePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to discard changes: %w", err)
	}
	return nil
}

func ListUnstagedFiles(repoPath string) ([]string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree status: %w", err)
	}

	var unstagedFiles []string
	// Loop over the status to collect unstaged files
	for filePath, fileStatus := range status {
		if fileStatus.Worktree != git.Unmodified {
			unstagedFiles = append(unstagedFiles, filePath)
		}
	}

	return unstagedFiles, nil
}

func getAddedOrModifiedFiles(patch *object.Patch) []string {
	updatedFiles := []string{}

	for _, stat := range patch.Stats() {
		// Filter for added (A) or modified (M) files
		if stat.Addition > 0 || stat.Deletion > 0 {
			updatedFiles = append(updatedFiles, stat.Name)
		}
	}

	return updatedFiles
}

func getDeletedFiles(patch *object.Patch) []string {
	deletedFiles := []string{}

	for _, filePatch := range patch.FilePatches() {
		from, to := filePatch.Files()
		if to == nil && from != nil {
			deletedFiles = append(deletedFiles, from.Path())
		}
	}

	return deletedFiles
}

func findDecorations(repo *git.Repository, hash plumbing.Hash) []string {
	decorations := []string{}

	refs, err := repo.References()
	if err != nil {
		fmt.Printf("Error getting references %s\n", err)
	}

	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() == plumbing.HashReference && ref.Hash() == hash {
			if ref.Name().IsBranch() || ref.Name().IsTag() {
				decorations = append(decorations, ref.Name().String())
			}
		}
		return nil
	})
	if err != nil {
		fmt.Printf("Failed iterating over references: %s\n", err.Error())
	}

	return decorations
}
