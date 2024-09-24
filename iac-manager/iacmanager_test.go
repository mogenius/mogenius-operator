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
	utils.CONFIG.Iac.RepoUrl = "https://github.com/mogenius/docs.git"
	utils.CONFIG.Iac.RepoPat = ""
	utils.CONFIG.Iac.RepoBranch = "main"
	utils.CONFIG.Iac.AllowPull = true
	utils.CONFIG.Iac.SyncFrequencyInSec = 10
	utils.CONFIG.Kubernetes.GitVaultDataPath = os.TempDir()

	err := gitInitRepo()
	if err != nil {
		t.Errorf("Error initializing git repo: %v", err)
	} else {
		t.Log("Repo initialized ✅")
	}

	data := PrintIacStatus()
	if len(data) < 100 {
		t.Errorf("Error getting IAC status")
	} else {
		fmt.Print(data)
		t.Log("IAC status retrieved ✅")
	}

	// NEW DIFF
	utils.InitMogeniusContainerRegistryIngress()
	exampleConfigmapYaml := utils.InitUpgradeConfigMapYaml()
	tempPath := os.TempDir() + "/example-configmap.yaml"
	os.WriteFile(tempPath, []byte(exampleConfigmapYaml), 0644)
	diff, err := createDiffFromFile(exampleConfigmapYaml, tempPath, "bla")
	if err != nil {
		t.Errorf("Error creating diff: %v", err)
	} else {
		fmt.Print(diff)
		t.Log("Diff created ✅")
	}
}
