package iacmanager

import (
	"fmt"
	"mogenius-k8s-manager/utils"
	"os"
	"testing"
)

func TestIacManager(t *testing.T) {
	utils.InitConfigSimple(utils.STAGE_DEV)

	InitDataModel()

	// SETUP TEST REPO
	utils.CONFIG.Iac.RepoUrl = "https://github.com/beneiltis/iac.git"
	utils.CONFIG.Iac.RepoPat = "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" // testtoken only for test repo (not crtical) expires 2025-09-05
	utils.CONFIG.Iac.RepoBranch = "main"
	utils.CONFIG.Iac.AllowPull = true
	utils.CONFIG.Iac.SyncFrequencyInSec = 10
	utils.CONFIG.Misc.DefaultMountPath = os.TempDir()

	err := gitInitRepo()
	if err != nil {
		t.Errorf("Error initializing git repo: %v", err)
	} else {
		t.Log("Repo initialized ✅")
	}

	err = addRemote()
	if err != nil {
		t.Errorf("Error adding remote: %v", err)
	} else {
		t.Log("Remote added ✅")
	}

	err = SyncChanges()
	if err != nil {
		t.Errorf("Error syncing changes: %v", err)
	} else {
		t.Log("Changes synced ✅")
	}

	data := PrintIacStatus()
	if len(data) < 100 {
		t.Errorf("Error getting IAC status")
	} else {
		fmt.Print(data)
		t.Log("IAC status retrieved ✅")
	}
}
