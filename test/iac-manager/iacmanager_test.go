package iacmanager_test

import (
	"mogenius-k8s-manager/src/config"
	iacmanager "mogenius-k8s-manager/src/iac-manager"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/watcher"
	"os"
	"testing"
)

func TestIacManager(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)
	iacmanager.Setup(logManager, config, watcher.NewWatcher())
	config.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_GIT_VAULT_DATA_PATH",
		DefaultValue: utils.Pointer(os.TempDir()),
	})

	utils.InitConfigSimple(utils.STAGE_DEV)

	iacmanager.InitDataModel()

	// SETUP TEST REPO
	utils.CONFIG.Iac.RepoUrl = "https://github.com/mogenius/docs.git"
	utils.CONFIG.Iac.RepoPat = ""
	utils.CONFIG.Iac.RepoBranch = "main"
	utils.CONFIG.Iac.AllowPull = true
	utils.CONFIG.Iac.SyncFrequencyInSec = 10

	err := iacmanager.GitInitRepo()
	if err != nil {
		t.Errorf("Error initializing git repo: %v", err)
	} else {
		t.Log("Repo initialized ✅")
	}

	data := iacmanager.PrintIacStatus()
	if len(data) < 100 {
		t.Errorf("Error getting IAC status")
	} else {
		t.Log(data)
		t.Log("IAC status retrieved ✅")
	}

	// NEW DIFF
	utils.InitMogeniusContainerRegistryIngress()
	exampleConfigmapYaml := utils.InitUpgradeConfigMapYaml()
	tempPath := os.TempDir() + "/example-configmap.yaml"
	err = os.WriteFile(tempPath, []byte(exampleConfigmapYaml), 0644)
	if err != nil {
		t.Errorf("Error creating example configmap: %s", err.Error())
	}
	diff, err := iacmanager.CreateDiffFromFile(exampleConfigmapYaml, tempPath, "bla")
	if err != nil {
		t.Errorf("Error creating diff: %v", err)
	} else {
		t.Log(diff)
		t.Log("Diff created ✅")
	}
}
