package helm_test

import (
	"fmt"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/helm"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"os"
	"path/filepath"
	"testing"

	"helm.sh/helm/v3/pkg/action"
)

const (
	testNamespace string = "default"
	testRepo      string = "bitnami"
	testChartUrl  string = "https://charts.bitnami.com/bitnami"
	testChart     string = "bitnami/nginx"
	testRelease   string = "nginx-test"
	testValues    string = "#values_yaml"
	testDryRun    bool   = false
	helmConfPath  string = "./testData/registryConfigPath"
)

func cleanupRepo(t *testing.T) {
	// PAT_NAMESPACE_HELM_REPO_REMOVE - remove repo is purposely placed at the end
	// no futher testing needed no error is sufficient
	repoRemoveData := helm.HelmRepoRemoveRequest{
		Name: testRepo,
	}
	_, err := helm.HelmRepoRemove(repoRemoveData)
	if err != nil {
		t.Logf("failed to remove helm repo: %s", err.Error())
	}
}

func cleanupInstall(t *testing.T) {
	// PAT_NAMESPACE_HELM_UNINSTALL - remove repo is purposely placed at the end
	// no futher testing needed no error is sufficient
	releaseUninstallData := helm.HelmReleaseUninstallRequest{
		Namespace: testNamespace,
		Release:   testRelease,
		DryRun:    testDryRun,
	}
	_, err := helm.HelmReleaseUninstall(releaseUninstallData)
	if err != nil {
		t.Logf("failed to uninstall helmrelease: %s", err.Error())
	}
}

func deleteFolder(folderPath string) error {
	err := os.RemoveAll(folderPath)
	if err != nil {
		return err
	}
	return nil
}

func createRepoForTest(t *testing.T) error {
	// prerequisite configs
	err := helm.InitHelmConfig()
	if err != nil {
		t.Error(err)
	}

	repoAddData := helm.HelmRepoAddRequest{
		Name: testRepo,
		Url:  testChartUrl,
	}
	_, err = helm.HelmRepoAdd(repoAddData)
	if err != nil {
		t.Log(err)
	}
	return err
}

func installForTests(t *testing.T) error {
	// prerequisite configs
	err := helm.InitHelmConfig()
	if err != nil {
		t.Error(err)
	}

	helmInstallData := helm.HelmChartInstallRequest{
		Namespace: testNamespace,
		Chart:     testChart,
		Release:   testRelease,
		Values:    testValues,
		DryRun:    testDryRun,
	}
	_, err = helm.HelmChartInstall(helmInstallData)
	if err != nil {
		t.Log(err)
		return err
	}
	return nil
}

func testSetup() error {
	err := helm.InitHelmConfig()
	if err != nil {
		return err
	}
	return nil
}

func TestHelmRepoAdd(t *testing.T) {
	logManager := logging.NewMockSlogManager(t)
	config := cfg.NewConfig()
	helm.Setup(logManager, config)
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_HELM_DATA_PATH",
		DefaultValue: utils.Pointer(helmConfPath),
	})

	// clean config folder before test
	err := deleteFolder(helmConfPath)
	assert.AssertT(t, err == nil, err)

	cleanupRepo(t) // cleanup if it existed before

	// prerequisite configs
	err = testSetup()
	assert.AssertT(t, err == nil, err)

	// test with
	// helm --repository-config /tmp/registryConfigPath/helm/repositories.yaml repo list
	// no futher testing needed no error is sufficient
	// PAT_NAMESPACE_HELM_REPO_ADD
	err = createRepoForTest(t)
	t.Cleanup(func() {
		cleanupRepo(t)
	})
	assert.AssertT(t, err == nil, err)
}

func TestHelmRepoUpdate(t *testing.T) {
	logManager := logging.NewMockSlogManager(t)
	config := config.NewConfig()
	helm.Setup(logManager, config)
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_HELM_DATA_PATH",
		DefaultValue: utils.Pointer(helmConfPath),
	})

	err := testSetup()
	assert.AssertT(t, err == nil, err)

	// PAT_NAMESPACE_HELM_REPO_UPDATE
	// no futher testing needed no error is sufficient
	_, err = helm.HelmRepoUpdate()
	assert.AssertT(t, err == nil, err)
}

func TestHelmRepoList(t *testing.T) {
	logManager := logging.NewMockSlogManager(t)
	config := config.NewConfig()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_HELM_DATA_PATH",
		DefaultValue: utils.Pointer(helmConfPath),
	})
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_PATH",
		DefaultValue: utils.Pointer(filepath.Join(t.TempDir(), "mogenius.db")),
	})
	clientProvider := k8sclient.NewK8sClientProvider(logManager.CreateLogger("client-provider"))

	watcherModule := kubernetes.NewWatcher(clientProvider)
	err := kubernetes.Setup(logManager, config, watcherModule, clientProvider)
	assert.AssertT(t, err == nil, err)

	err = testSetup()
	assert.AssertT(t, err == nil, err)

	err = createRepoForTest(t)
	t.Cleanup(func() {
		cleanupRepo(t)
	})
	assert.AssertT(t, err == nil, err)

	// PAT_NAMESPACE_HELM_REPO_LIST
	// check if repo is added
	listRepoData, err := helm.HelmRepoList()
	assert.AssertT(t, err == nil, err)
	listSuccess := false
	for _, v := range listRepoData {
		t.Logf("Release found: %s", v.Name)
		if v.Name == testRepo {
			listSuccess = true
			break
		}
	}
	assert.AssertT(t, listSuccess, fmt.Sprintf("Repo '%s' not found but it should be", testRepo))
}

func TestHelmInstallRequest(t *testing.T) {
	logManager := logging.NewMockSlogManager(t)
	config := config.NewConfig()
	helm.Setup(logManager, config)
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_HELM_DATA_PATH",
		DefaultValue: utils.Pointer(helmConfPath),
	})

	err := testSetup()
	assert.AssertT(t, err == nil, err)

	err = createRepoForTest(t)
	t.Cleanup(func() {
		cleanupRepo(t)
	})
	assert.AssertT(t, err == nil, err)

	cleanupInstall(t) // cleanup if it existed before

	// PAT_NAMESPACE_HELM_INSTALL
	// no futher testing needed no error is sufficient
	err = installForTests(t)
	t.Cleanup(func() {
		cleanupInstall(t)
	})
	assert.AssertT(t, err == nil, err)
}

func TestHelmUpgradeRequest(t *testing.T) {
	logManager := logging.NewMockSlogManager(t)
	config := config.NewConfig()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_HELM_DATA_PATH",
		DefaultValue: utils.Pointer(helmConfPath),
	})
	helm.Setup(logManager, config)

	err := testSetup()
	assert.AssertT(t, err == nil, err)
	err = createRepoForTest(t)
	t.Cleanup(func() {
		cleanupRepo(t)
	})
	assert.AssertT(t, err == nil, err)

	err = installForTests(t)
	t.Cleanup(func() {
		cleanupInstall(t)
	})
	assert.AssertT(t, err == nil, err)

	// PAT_NAMESPACE_HELM_UPGRADE
	// no futher testing needed no error is sufficient
	releaseUpgradeData := helm.HelmReleaseUpgradeRequest{
		Namespace: testNamespace,
		Chart:     testChart,
		Release:   testRelease,
		Values:    testValues,
		DryRun:    testDryRun,
	}
	_, err = helm.HelmReleaseUpgrade(releaseUpgradeData)
	assert.AssertT(t, err == nil, err)
}

func TestHelmListRequest(t *testing.T) {
	logManager := logging.NewMockSlogManager(t)
	config := config.NewConfig()
	helm.Setup(logManager, config)
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_HELM_DATA_PATH",
		DefaultValue: utils.Pointer(helmConfPath),
	})

	err := testSetup()
	assert.AssertT(t, err == nil, err)
	err = createRepoForTest(t)
	t.Cleanup(func() {
		cleanupRepo(t)
	})
	assert.AssertT(t, err == nil, err)

	err = installForTests(t)
	t.Cleanup(func() {
		cleanupInstall(t)
	})
	assert.AssertT(t, err == nil, err)

	// PAT_NAMESPACE_HELM_LIST
	// check if release is added
	releaseListData := helm.HelmReleaseListRequest{
		Namespace: testNamespace,
	}
	releaseList, err := helm.HelmReleaseList(releaseListData)
	assert.AssertT(t, err == nil, err)
	listReleasesSuccess := false
	for _, v := range releaseList {
		t.Logf("Release found: %s", v.Name)
		if v.Name == testRelease {
			listReleasesSuccess = true
			break
		}
	}
	assert.AssertT(t, listReleasesSuccess, fmt.Sprintf("Release '%s' not found but it should be", testRelease))
}

func TestHelmReleases(t *testing.T) {
	logManager := logging.NewMockSlogManager(t)
	config := config.NewConfig()
	helm.Setup(logManager, config)
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_HELM_DATA_PATH",
		DefaultValue: utils.Pointer(helmConfPath),
	})

	err := testSetup()
	assert.AssertT(t, err == nil, err)

	err = createRepoForTest(t)
	t.Cleanup(func() {
		cleanupRepo(t)
	})
	assert.AssertT(t, err == nil, err)

	err = installForTests(t)
	t.Cleanup(func() {
		cleanupInstall(t)
	})
	assert.AssertT(t, err == nil, err)

	// PAT_NAMESPACE_HELM_STATUS
	// no futher testing needed no error is sufficient
	releaseStatusData := helm.HelmReleaseStatusRequest{
		Namespace: testNamespace,
		Release:   testRelease,
	}
	_, err = helm.HelmReleaseStatus(releaseStatusData)
	assert.AssertT(t, err == nil, err)

	// PAT_NAMESPACE_HELM_SHOW
	// no futher testing needed no error is sufficient
	chartShowData := helm.HelmChartShowRequest{
		Chart:      testChart,
		ShowFormat: action.ShowAll,
	}
	_, err = helm.HelmChartShow(chartShowData)
	assert.AssertT(t, err == nil, err)

	// PAT_NAMESPACE_HELM_GET
	// no futher testing needed no error is sufficient
	releaseGet := helm.HelmReleaseGetRequest{
		Namespace: testNamespace,
		Release:   testRelease,
		GetFormat: structs.HelmGetAll,
	}
	_, err = helm.HelmReleaseGet(releaseGet)
	assert.AssertT(t, err == nil, err)

	// PAT_NAMESPACE_HELM_HISTORY
	// history should have at least 1 entry
	releaseHistoryData := helm.HelmReleaseHistoryRequest{
		Namespace: testNamespace,
		Release:   testRelease,
	}
	historyList, err := helm.HelmReleaseHistory(releaseHistoryData)
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, len(historyList) > 0, fmt.Sprintf("Release '%s' history not found but it should be", testRelease))

	// PAT_NAMESPACE_HELM_ROLLBACK
	// no futher testing needed no error is sufficient
	releaseRollbackData := helm.HelmReleaseRollbackRequest{
		Namespace: testNamespace,
		Release:   testRelease,
		Revision:  1,
	}
	_, err = helm.HelmReleaseRollback(releaseRollbackData)
	assert.AssertT(t, err == nil, err)
}
