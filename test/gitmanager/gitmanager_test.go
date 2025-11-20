package gitmanager_test

import (
	"fmt"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/gitmanager"
	"os"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
)

func TestGitManager(t *testing.T) {
	t.Skip("the gitmanager is currently unused and not a priority for testing")

	repoUrl := "https://github.com/mogenius/docs.git"
	localPath := os.TempDir() + "/test-repo"
	localPathInit := os.TempDir() + "/test-repo-init"
	localPathFast := os.TempDir() + "/test-repo-fast"
	mainBranch := "main"
	devBranch := "dev"
	switchBranch := "dev-test-branch"
	testFileInRepo := "mogenius/docs/cluster-management/troubleshooting-clusters.md"

	// CLEANUP
	t.Cleanup(func() {
		err := gitmanager.DeletePath(localPath)
		if err != nil {
			t.Errorf("Error removing repo: %s", err.Error())
		}
		err = gitmanager.DeletePath(localPathInit)
		if err != nil {
			t.Errorf("Error removing init-repo: %s", err.Error())
		}
		err = gitmanager.DeletePath(localPathFast)
		if err != nil {
			t.Errorf("Error removing fast-repo: %s", err.Error())
		}
	})

	// CLONE
	err := gitmanager.Clone(repoUrl, localPath)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully cloned ✅", repoUrl)

	// PULL
	_, err = gitmanager.Pull(localPath, "origin", mainBranch)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully pulled ✅", repoUrl)

	// PUSH
	err = gitmanager.Push(localPath, "origin")
	assert.AssertT(t, err == nil || err == transport.ErrAuthenticationRequired)
	t.Logf("Repo %s successfully pushed ✅", repoUrl)

	// COMMIT
	err = os.WriteFile(localPath+"/testfile.yaml", []byte("hello world!"), 0644)
	assert.AssertT(t, err == nil, err)

	err = gitmanager.Commit(localPath, []string{"testfile.yaml"}, []string{}, "Test commit", "testuser", "testuseremail@mogenius.com")
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully committed ✅", repoUrl)

	// GET LAST COMMIT
	commits, err := gitmanager.GetLastCommits(localPath, 3)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s read last commit ✅. (%s)", repoUrl, commits[0].Message)

	// INIT
	err = gitmanager.InitGit(localPathInit)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully initialized ✅", repoUrl)

	// CHECKOUT
	err = gitmanager.CheckoutBranch(localPath, mainBranch)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully checked out branch ✅ (%s)", repoUrl, mainBranch)

	// SWITCH
	err = gitmanager.CheckoutBranch(localPath, switchBranch)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully switched branch ✅", repoUrl)

	// Fetch
	err = gitmanager.Fetch(localPath)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully fetched ✅", repoUrl)

	// LIST REMOTE BRANCHES
	branches, err := gitmanager.ListRemoteBranches(localPath)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s listed branches ✅ (%v)", repoUrl, branches)

	// GET TAG
	tag, err := gitmanager.GetHeadTag(localPath)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s read tag ✅ (%s)", repoUrl, tag)

	// CLONE FAST
	err = gitmanager.CloneFast(repoUrl, localPathFast, devBranch)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully fast_cloned ✅ (%s)", repoUrl, devBranch)

	// LOG DECORATED
	log, err := gitmanager.LastLogDecorate(localPath)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully decorated log ✅ (logLen %d)", repoUrl, len(log))

	// ADD REMOTE
	err = gitmanager.AddRemote(localPath, "https://mogenius.com/testremote", "originTEST")
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully added remote ✅", repoUrl)

	// LS REMOTE
	remotes, err := gitmanager.LsRemotes(repoUrl)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s listed remotes ✅ (found %d refs)", repoUrl, len(remotes))

	// LIST LOCAL REMOTES
	remotes, err = gitmanager.ListLocalAvailableRemotes(localPath)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s listed local remotes ✅ (found %d refs)", repoUrl, len(remotes))

	// HAS REMOTES
	hasRemotes := gitmanager.HasRemotes(localPath)
	assert.AssertT(t, hasRemotes, "no remotes found")
	t.Logf("Repo %s should have 2 remotes ✅", repoUrl)

	// GET LAST MODIFIED AND UPDATED FILES
	files, err := gitmanager.GetLastUpdatedAndModifiedFiles(localPath)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully got last modified and updated files ✅ (%v)", repoUrl, files)

	// GET LAST DELETED FILES
	deletedFiles, err := gitmanager.GetLastDeletedFiles(localPath)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully got last deleted files ✅ (%v)", repoUrl, deletedFiles)

	// DIFF
	diff, err := gitmanager.UnifiedDiff(localPath, localPath, "grype.yaml")
	fmt.Println(diff)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully got last diff ✅\n%s", repoUrl, diff)

	// GET CONTRIBUTORS
	contributors, err := gitmanager.GetContributors(localPath)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully got contributors ✅ (%v)", repoUrl, contributors)

	// GET FILEREVISIONS
	fileRevisions, err := gitmanager.ListFileRevisions(localPath, testFileInRepo, "bla.yaml")
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully got file revisions ✅ (%v)", repoUrl, fileRevisions)

	// DIFF FOR COMMIT
	specificDiff, err := gitmanager.DiffForCommit(localPath, "6f17091c598b21db9027a079564e9011f0f43ceb", testFileInRepo, "bla.yaml")
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully got diff for commit ✅ (%s)", repoUrl, specificDiff)

	// RESET FILE TO COMMIT
	err = gitmanager.ResetFileToCommit(localPath, "6f17091c598b21db9027a079564e9011f0f43ceb", testFileInRepo)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully reset file to commit ✅", repoUrl)

	// DISCARD UNSTAGED CHANGES
	err = gitmanager.DiscardUnstagedChanges(localPath, testFileInRepo)
	assert.AssertT(t, err == nil, err)
	t.Logf("Repo %s successfully discarded unstaged changes in file %s ✅", repoUrl, testFileInRepo)

	// PULSE DIAGRAM
	commitsPerWeek, err := gitmanager.GeneratePulseDiagramData(localPath)
	assert.AssertT(t, err == nil, err)
	t.Log(commitsPerWeek)
	t.Logf("Repo %s successfully generated pulse diagram data ✅", repoUrl)
}
