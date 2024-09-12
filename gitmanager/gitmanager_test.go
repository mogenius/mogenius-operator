package gitmanager

import (
	"fmt"
	"os"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
)

func TestGitManager(t *testing.T) {
	repoUrl := "https://github.com/mogenius/docs.git"
	localPath := "test-repo"
	localPathInit := "test-repo-init"
	localPathFast := "test-repo-fast"
	mainBranch := "main"
	switchBranch := "dev"

	// CLEANUP
	defer func() {
		err := DeletePath(localPath)
		if err != nil {
			t.Errorf("Error removing repo: %s", err.Error())
		}
		err = DeletePath(localPathInit)
		if err != nil {
			t.Errorf("Error removing init-repo: %s", err.Error())
		}
		err = DeletePath(localPathFast)
		if err != nil {
			t.Errorf("Error removing fast-repo: %s", err.Error())
		}
	}()

	// CLONE
	err := Clone(repoUrl, localPath)
	if err != nil {
		t.Errorf("Error cloning repo: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully cloned ✅", repoUrl)
	}

	// PULL
	_, err = Pull(localPath, "origin", mainBranch)
	if err != nil {
		t.Errorf("Error pulling repo: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully pulled ✅", repoUrl)
	}

	// PUSH
	err = Push(localPath, "origin")
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
	err = Commit(localPath, []string{"testfile.yaml"}, []string{}, "Test commit", "testuser", "testuseremail@mogenius.com")
	if err != nil {
		t.Errorf("Error committing repo: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully committed ✅", repoUrl)
	}

	// GET LAST COMMIT
	commits, err := GetLastCommits(localPath, 3)
	if err != nil {
		t.Errorf("Error getting last commit: %s", err.Error())
	} else {
		t.Logf("Repo %s read last commit ✅. (%s)", repoUrl, commits[0].Message)
	}

	// INIT
	err = InitGit(localPathInit)
	if err != nil {
		t.Errorf("Error initializing repo: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully initialized ✅", repoUrl)
	}

	// CHECKOUT
	err = CheckoutBranch(localPath, mainBranch)
	if err != nil {
		t.Errorf("Error checking out branch: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully checked out branch ✅ (%s)", repoUrl, mainBranch)
	}

	// SWITCH
	err = SwitchBranch(localPath, switchBranch)
	if err != nil {
		t.Errorf("Error switching branch (%s): %s", switchBranch, err.Error())
	} else {
		t.Logf("Repo %s successfully switched branch ✅", repoUrl)
	}

	// Fetch
	err = Fetch(localPath)
	if err != nil {
		t.Errorf("Error fetching repo: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully fetched ✅", repoUrl)
	}

	// LIST REMOTE BRANCHES
	branches, err := ListRemoteBranches(localPath)
	if err != nil {
		t.Errorf("Error listing branches: %s", err.Error())
	} else {
		t.Logf("Repo %s listed branches ✅ (%v)", repoUrl, branches)
	}

	// GET TAG
	tag, err := GetHeadTag(localPath)
	if err != nil {
		t.Errorf("Error getting tag: %s", err.Error())
	} else {
		t.Logf("Repo %s read tag ✅ (%s)", repoUrl, tag)
	}

	// CLONE FAST
	err = CloneFast(repoUrl, localPathFast, switchBranch)
	if err != nil {
		t.Errorf("Error fast cloning repo: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully fast_cloned ✅ (%s)", repoUrl, switchBranch)
	}

	// LOG DECORATED
	log, err := LastLogDecorate(localPath)
	if err != nil {
		t.Errorf("Error decorating log: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully decorated log ✅ (logLen %d)", repoUrl, len(log))
	}

	// ADD REMOTE
	err = AddRemote(localPath, "https://mogenius.com/testremote", "originTEST")
	if err != nil {
		t.Errorf("Error adding remote: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully added remote ✅", repoUrl)
	}

	// LS REMOTE
	remotes, err := LsRemotes(repoUrl)
	if err != nil {
		t.Errorf("Error listing remotes: %s", err.Error())
	} else {
		t.Logf("Repo %s listed remotes ✅ (found %d refs)", repoUrl, len(remotes))
	}

	// LIST LOCAL REMOTES
	remotes, err = ListLocalAvailableRemotes(localPath)
	if err != nil {
		t.Errorf("Error listing local remotes: %s", err.Error())
	} else {
		t.Logf("Repo %s listed local remotes ✅ (found %d refs)", repoUrl, len(remotes))
	}

	// HAS REMOTES
	hasRemotes := HasRemotes(localPath)
	if !hasRemotes {
		t.Errorf("Error checking has remotes")
	} else {
		t.Logf("Repo %s should have 2 remotes ✅", repoUrl)
	}

	// GET LAST MODIFIED AND UPDATED FILES
	files, err := GetLastUpdatedAndModifiedFiles(localPath)
	if err != nil {
		t.Errorf("Error getting last modified and updated files: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully got last modified and updated files ✅ (%v)", repoUrl, files)
	}

	// GET LAST DELETED FILES
	deletedFiles, err := GetLastDeletedFiles(localPath)
	if err != nil {
		t.Errorf("Error getting last deleted files: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully got last deleted files ✅ (%v)", repoUrl, deletedFiles)
	}

	// DIFF
	diff, _, _, err := LastDiff(localPath, "grype.yaml")
	fmt.Println(diff)
	if err != nil {
		t.Errorf("Error getting last diff: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully got last diff ✅\n%s", repoUrl, diff)
	}

	// GET CONTRIBUTORS
	contributors, err := GetContributors(localPath)
	if err != nil {
		t.Errorf("Error getting contributors: %s", err.Error())
	} else {
		t.Logf("Repo %s successfully got contributors ✅ (%v)", repoUrl, contributors)
	}

}
