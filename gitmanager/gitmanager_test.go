package gitmanager_test

import (
	"fmt"
	"mogenius-k8s-manager/gitmanager"
	"os"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/mogenius/punq/structs"
)

func TestGitManager(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	repoUrl := "https://github.com/mogenius/docs.git"
	localPath := os.TempDir() + "/test-repo"
	localPathInit := os.TempDir() + "/test-repo-init"
	localPathFast := os.TempDir() + "/test-repo-fast"
	mainBranch := "main"
	devBranch := "dev"
	switchBranch := "dev-test-branch"
	testFileInRepo := "mogenius/docs/cluster-management/troubleshooting-clusters.md"

	// CLEANUP
	defer func() {
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
	}()

	// CLONE
	err := gitmanager.Clone(repoUrl, localPath)
	if err != nil {
		t.Errorf("Error cloning repo: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully cloned ✅", repoUrl)
	}

	// PULL
	_, err = gitmanager.Pull(localPath, "origin", mainBranch)
	if err != nil {
		t.Errorf("Error pulling repo: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully pulled ✅", repoUrl)
	}

	// PUSH
	err = gitmanager.Push(localPath, "origin")
	if err != nil && err != transport.ErrAuthenticationRequired {
		t.Errorf("Error pushing repo: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully pushed ✅", repoUrl)
	}

	// COMMIT
	err = os.WriteFile(localPath+"/testfile.yaml", []byte("hello world!"), 0644)
	if err != nil {
		t.Errorf("Error writing commit-test-file: %s", err.Error())
	}
	err = gitmanager.Commit(localPath, []string{"testfile.yaml"}, []string{}, "Test commit", "testuser", "testuseremail@mogenius.com")
	if err != nil {
		t.Errorf("Error committing repo: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully committed ✅", repoUrl)
	}

	// GET LAST COMMIT
	commits, err := gitmanager.GetLastCommits(localPath, 3)
	if err != nil {
		t.Errorf("Error getting last commit: %s", err.Error())
	} else {
		t.Logf("Repo %s read last commit ✅. (%s)", repoUrl, commits[0].Message)
	}

	// INIT
	err = gitmanager.InitGit(localPathInit)
	if err != nil {
		t.Errorf("Error initializing repo: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully initialized ✅", repoUrl)
	}

	// CHECKOUT
	err = gitmanager.CheckoutBranch(localPath, mainBranch)
	if err != nil {
		t.Errorf("Error checking out branch: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully checked out branch ✅ (%s)", repoUrl, mainBranch)
	}

	// SWITCH
	err = gitmanager.CheckoutBranch(localPath, switchBranch)
	if err != nil {
		t.Errorf("Error switching branch (%s): %s", switchBranch, err.Error())
	} else {
		t.Logf("Repo %s successfully switched branch ✅", repoUrl)
	}

	// Fetch
	err = gitmanager.Fetch(localPath)
	if err != nil {
		t.Errorf("Error fetching repo: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully fetched ✅", repoUrl)
	}

	// LIST REMOTE BRANCHES
	branches, err := gitmanager.ListRemoteBranches(localPath)
	if err != nil {
		t.Errorf("Error listing branches: %s", err.Error())
	} else {
		t.Logf("Repo %s listed branches ✅ (%v)", repoUrl, branches)
	}

	// GET TAG
	tag, err := gitmanager.GetHeadTag(localPath)
	if err != nil {
		t.Errorf("Error getting tag: %s", err.Error())
	} else {
		t.Logf("Repo %s read tag ✅ (%s)", repoUrl, tag)
	}

	// CLONE FAST
	err = gitmanager.CloneFast(repoUrl, localPathFast, devBranch)
	if err != nil {
		t.Errorf("Error fast cloning repo: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully fast_cloned ✅ (%s)", repoUrl, devBranch)
	}

	// LOG DECORATED
	log, err := gitmanager.LastLogDecorate(localPath)
	if err != nil {
		t.Errorf("Error decorating log: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully decorated log ✅ (logLen %d)", repoUrl, len(log))
	}

	// ADD REMOTE
	err = gitmanager.AddRemote(localPath, "https://mogenius.com/testremote", "originTEST")
	if err != nil {
		t.Errorf("Error adding remote: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully added remote ✅", repoUrl)
	}

	// LS REMOTE
	remotes, err := gitmanager.LsRemotes(repoUrl)
	if err != nil {
		t.Errorf("Error listing remotes: %s", err.Error())
	} else {
		t.Logf("Repo %s listed remotes ✅ (found %d refs)", repoUrl, len(remotes))
	}

	// LIST LOCAL REMOTES
	remotes, err = gitmanager.ListLocalAvailableRemotes(localPath)
	if err != nil {
		t.Errorf("Error listing local remotes: %s", err.Error())
	} else {
		t.Logf("Repo %s listed local remotes ✅ (found %d refs)", repoUrl, len(remotes))
	}

	// HAS REMOTES
	hasRemotes := gitmanager.HasRemotes(localPath)
	if !hasRemotes {
		t.Errorf("Error checking has remotes")
	} else {
		t.Logf("Repo %s should have 2 remotes ✅", repoUrl)
	}

	// GET LAST MODIFIED AND UPDATED FILES
	files, err := gitmanager.GetLastUpdatedAndModifiedFiles(localPath)
	if err != nil {
		t.Errorf("Error getting last modified and updated files: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully got last modified and updated files ✅ (%v)", repoUrl, files)
	}

	// GET LAST DELETED FILES
	deletedFiles, err := gitmanager.GetLastDeletedFiles(localPath)
	if err != nil {
		t.Errorf("Error getting last deleted files: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully got last deleted files ✅ (%v)", repoUrl, deletedFiles)
	}

	// DIFF
	diff, err := gitmanager.UnifiedDiff(localPath, localPath, "grype.yaml")
	fmt.Println(diff)
	if err != nil {
		t.Errorf("Error getting last diff: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully got last diff ✅\n%s", repoUrl, diff)
	}

	// GET CONTRIBUTORS
	contributors, err := gitmanager.GetContributors(localPath)
	if err != nil {
		t.Errorf("Error getting contributors: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully got contributors ✅ (%v)", repoUrl, contributors)
	}

	// GET FILEREVISIONS
	fileRevisions, err := gitmanager.ListFileRevisions(localPath, testFileInRepo, "bla.yaml")
	if err != nil {
		t.Errorf("Error getting file revisions: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully got file revisions ✅ (%v)", repoUrl, fileRevisions)
	}

	// DIFF FOR COMMIT
	specificDiff, err := gitmanager.DiffForCommit(localPath, "6f17091c598b21db9027a079564e9011f0f43ceb", testFileInRepo, "bla.yaml")
	if err != nil {
		t.Errorf("Error getting diff for commit: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully got diff for commit ✅ (%s)", repoUrl, specificDiff)
	}

	// RESET FILE TO COMMIT
	err = gitmanager.ResetFileToCommit(localPath, "6f17091c598b21db9027a079564e9011f0f43ceb", testFileInRepo)
	if err != nil {
		t.Errorf("Error resetting file to commit: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully reset file to commit ✅", repoUrl)
	}

	// DISCARD UNSTAGED CHANGES
	err = gitmanager.DiscardUnstagedChanges(localPath, testFileInRepo)
	if err != nil {
		t.Errorf("Error resetting file to commit: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully discarded unstaged changes in file %s ✅", repoUrl, testFileInRepo)
	}

	// PULSE DIAGRAM
	commitsPerWeek, err := gitmanager.GeneratePulseDiagramData(localPath)
	if err != nil {
		t.Errorf("Error generating pulse diagram data: %s", err.Error())
	} else {
		structs.PrettyPrint(commitsPerWeek)
		t.Logf("Repo %s successfully generated pulse diagram data ✅", repoUrl)
	}
}
