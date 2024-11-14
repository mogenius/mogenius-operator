package kubernetes_test

import (
	"fmt"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"os"
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

func cleanupRepo() {
	// PAT_NAMESPACE_HELM_REPO_REMOVE - remove repo is purposely placed at the end
	// no futher testing needed no error is sufficient
	repoRemoveData := kubernetes.HelmRepoRemoveRequest{
		Name: testRepo,
	}
	_, err := kubernetes.HelmRepoRemove(repoRemoveData)
	if err != nil {
		fmt.Printf("failed to remove helm repo: %s", err.Error())
	}
}

func cleanupInstall() {
	// PAT_NAMESPACE_HELM_UNINSTALL - remove repo is purposely placed at the end
	// no futher testing needed no error is sufficient
	releaseUninstallData := kubernetes.HelmReleaseUninstallRequest{
		Namespace: testNamespace,
		Release:   testRelease,
		DryRun:    testDryRun,
	}
	_, err := kubernetes.HelmReleaseUninstall(releaseUninstallData)
	if err != nil {
		fmt.Printf("failed to uninstall helmrelease: %s", err.Error())
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
	err := kubernetes.InitHelmConfig()
	if err != nil {
		t.Error(err)
	}

	repoAddData := kubernetes.HelmRepoAddRequest{
		Name: testRepo,
		Url:  testChartUrl,
	}
	_, err = kubernetes.HelmRepoAdd(repoAddData)
	if err != nil {
		t.Log(err)
	}
	return err
}

func installForTests(t *testing.T) error {
	// prerequisite configs
	err := kubernetes.InitHelmConfig()
	if err != nil {
		t.Error(err)
	}

	helmInstallData := kubernetes.HelmChartInstallRequest{
		Namespace: testNamespace,
		Chart:     testChart,
		Release:   testRelease,
		Values:    testValues,
		DryRun:    testDryRun,
	}
	_, err = kubernetes.HelmChartInstall(helmInstallData)
	if err != nil {
		t.Log(err)
		return err
	}
	return nil
}

func testSetup() error {
	err := kubernetes.InitHelmConfig()
	if err != nil {
		return err
	}
	return nil
}

func TestHelmRepoAdd(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)
	config.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_HELM_DATA_PATH",
		DefaultValue: utils.Pointer(helmConfPath),
	})

	// clean config folder before test
	err := deleteFolder(helmConfPath)
	if err != nil {
		t.Error(err)
	}

	cleanupRepo() // cleanup if it existed before

	// prerequisite configs
	err = testSetup()
	if err != nil {
		t.Error(err)
	}

	// test with
	// helm --repository-config /tmp/registryConfigPath/helm/repositories.yaml repo list
	// no futher testing needed no error is sufficient
	// PAT_NAMESPACE_HELM_REPO_ADD
	err = createRepoForTest(t)
	if err != nil {
		t.Error(err)
	}
	t.Cleanup(cleanupRepo)
}

func TestHelmRepoUpdate(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)
	config.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_HELM_DATA_PATH",
		DefaultValue: utils.Pointer(helmConfPath),
	})

	err := testSetup()
	if err != nil {
		t.Error(err)
	}

	// PAT_NAMESPACE_HELM_REPO_UPDATE
	// no futher testing needed no error is sufficient
	_, err = kubernetes.HelmRepoUpdate()
	if err != nil {
		t.Error(err)
	}
}

func TestHelmRepoList(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)
	config.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_HELM_DATA_PATH",
		DefaultValue: utils.Pointer(helmConfPath),
	})

	err := testSetup()
	if err != nil {
		t.Error(err)
	}

	err = createRepoForTest(t)
	if err != nil {
		t.Error(err)
	}
	t.Cleanup(cleanupRepo)

	// PAT_NAMESPACE_HELM_REPO_LIST
	// check if repo is added
	listRepoData, err := kubernetes.HelmRepoList()
	if err != nil {
		t.Error(err)
	}
	listSuccess := false
	for _, v := range listRepoData {
		t.Logf("Release found: %s", v.Name)
		if v.Name == testRepo {
			listSuccess = true
			break
		}
	}
	if !listSuccess {
		t.Errorf("Repo '%s' not found but it should be", testRepo)
	}
}

func TestHelmInstallRequest(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)
	config.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_HELM_DATA_PATH",
		DefaultValue: utils.Pointer(helmConfPath),
	})

	err := testSetup()
	if err != nil {
		t.Error(err)
	}

	err = createRepoForTest(t)
	if err != nil {
		t.Error(err)
	}
	t.Cleanup(cleanupRepo)

	cleanupInstall() // cleanup if it existed before

	// PAT_NAMESPACE_HELM_INSTALL
	// no futher testing needed no error is sufficient
	err = installForTests(t)
	if err != nil {
		t.Error(err)
	}
	t.Cleanup(cleanupInstall)
}

func TestHelmUpgradeRequest(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)
	config.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_HELM_DATA_PATH",
		DefaultValue: utils.Pointer(helmConfPath),
	})

	err := testSetup()
	if err != nil {
		t.Error(err)
	}
	err = createRepoForTest(t)
	if err != nil {
		t.Error(err)
	}
	t.Cleanup(cleanupRepo)

	err = installForTests(t)
	if err != nil {
		t.Error(err)
	}
	t.Cleanup(cleanupInstall)

	// PAT_NAMESPACE_HELM_UPGRADE
	// no futher testing needed no error is sufficient
	releaseUpgradeData := kubernetes.HelmReleaseUpgradeRequest{
		Namespace: testNamespace,
		Chart:     testChart,
		Release:   testRelease,
		Values:    testValues,
		DryRun:    testDryRun,
	}
	_, err = kubernetes.HelmReleaseUpgrade(releaseUpgradeData)
	if err != nil {
		t.Error(err)
	}
}

func TestHelmListRequest(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)
	config.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_HELM_DATA_PATH",
		DefaultValue: utils.Pointer(helmConfPath),
	})

	err := testSetup()
	if err != nil {
		t.Error(err)
	}
	err = createRepoForTest(t)
	if err != nil {
		t.Error(err)
	}
	t.Cleanup(cleanupRepo)

	err = installForTests(t)
	if err != nil {
		t.Error(err)
	}
	t.Cleanup(cleanupInstall)

	// PAT_NAMESPACE_HELM_LIST
	// check if release is added
	releaseListData := kubernetes.HelmReleaseListRequest{
		Namespace: testNamespace,
	}
	releaseList, err := kubernetes.HelmReleaseList(releaseListData)
	if err != nil {
		t.Error(err)
	}
	listReleasesSuccess := false
	for _, v := range releaseList {
		t.Logf("Release found: %s", v.Name)
		if v.Name == testRelease {
			listReleasesSuccess = true
			break
		}
	}
	if !listReleasesSuccess {
		t.Errorf("Release '%s' not found but it should be", testRelease)
	}
}

func TestHelmReleases(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)
	config.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_HELM_DATA_PATH",
		DefaultValue: utils.Pointer(helmConfPath),
	})

	err := testSetup()
	if err != nil {
		t.Error(err)
	}

	err = createRepoForTest(t)
	if err != nil {
		t.Error(err)
	}
	t.Cleanup(cleanupRepo)

	err = installForTests(t)
	if err != nil {
		t.Error(err)
	}
	t.Cleanup(cleanupInstall)

	// PAT_NAMESPACE_HELM_STATUS
	// no futher testing needed no error is sufficient
	releaseStatusData := kubernetes.HelmReleaseStatusRequest{
		Namespace: testNamespace,
		Release:   testRelease,
	}
	_, err = kubernetes.HelmReleaseStatus(releaseStatusData)
	if err != nil {
		t.Error(err)
	}

	// PAT_NAMESPACE_HELM_SHOW
	// no futher testing needed no error is sufficient
	chartShowData := kubernetes.HelmChartShowRequest{
		Chart:      testChart,
		ShowFormat: action.ShowAll,
	}
	_, err = kubernetes.HelmChartShow(chartShowData)
	if err != nil {
		t.Error(err)
	}

	// PAT_NAMESPACE_HELM_GET
	// no futher testing needed no error is sufficient
	releaseGet := kubernetes.HelmReleaseGetRequest{
		Namespace: testNamespace,
		Release:   testRelease,
		GetFormat: structs.HelmGetAll,
	}
	_, err = kubernetes.HelmReleaseGet(releaseGet)
	if err != nil {
		t.Error(err)
	}

	// PAT_NAMESPACE_HELM_HISTORY
	// history should have at least 1 entry
	releaseHistoryData := kubernetes.HelmReleaseHistoryRequest{
		Namespace: testNamespace,
		Release:   testRelease,
	}
	historyList, err := kubernetes.HelmReleaseHistory(releaseHistoryData)
	if err != nil {
		t.Error(err)
	}
	if len(historyList) < 1 {
		t.Errorf("Release '%s' history not found but it should be", testRelease)
	}

	// PAT_NAMESPACE_HELM_ROLLBACK
	// no futher testing needed no error is sufficient
	releaseRollbackData := kubernetes.HelmReleaseRollbackRequest{
		Namespace: testNamespace,
		Release:   testRelease,
		Revision:  1,
	}
	_, err = kubernetes.HelmReleaseRollback(releaseRollbackData)
	if err != nil {
		t.Error(err)
	}
}
