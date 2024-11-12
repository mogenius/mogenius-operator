package kubernetes

import (
	"mogenius-k8s-manager/structs"
	"os"
	"testing"

	"helm.sh/helm/v3/pkg/action"
)

const (
	testNamespace = "default"
	testRepo      = "bitnami"
	testChartUrl  = "https://charts.bitnami.com/bitnami"
	testChart     = "bitnami/nginx"
	testRelease   = "nginx-test"
	testValues    = "#values_yaml"
	testDryRun    = false
	helmConfPath  = "./testData/registryConfigPath"
)

func cleanupRepo() {
	// PAT_NAMESPACE_HELM_REPO_REMOVE - remove repo is purposely placed at the end
	// no futher testing needed no error is sufficient
	repoRemoveData := HelmRepoRemoveRequest{
		Name: testRepo,
	}
	_, err := HelmRepoRemove(repoRemoveData)
	if err != nil {
		k8sLogger.Info("failed to remove helm repo", "error", err)
	}
}

func cleanupInstall() {
	// PAT_NAMESPACE_HELM_UNINSTALL - remove repo is purposely placed at the end
	// no futher testing needed no error is sufficient
	releaseUninstallData := HelmReleaseUninstallRequest{
		Namespace: testNamespace,
		Release:   testRelease,
		DryRun:    testDryRun,
	}
	_, err := HelmReleaseUninstall(releaseUninstallData)
	if err != nil {
		k8sLogger.Info("failed to uninstall helmrelease", "error", err)
	}
}

func deleteFolder(folderPath string) error {
	err := os.RemoveAll(folderPath)
	if err != nil {
		k8sLogger.Info("Error deleting folder %s: %v", folderPath, err)
		return err
	}
	k8sLogger.Info("Successfully deleted folder", "path", folderPath)
	return nil
}

func createRepoForTest(t *testing.T) error {
	// prerequisite configs
	config.Set("MO_HELM_DATA_PATH", helmConfPath)
	err := InitHelmConfig()
	if err != nil {
		t.Error(err)
	}

	repoAddData := HelmRepoAddRequest{
		Name: testRepo,
		Url:  testChartUrl,
	}
	_, err = HelmRepoAdd(repoAddData)
	if err != nil {
		t.Log(err)
	}
	return err
}

func installForTests(t *testing.T) error {
	// prerequisite configs
	// config.Set("MO_HELM_DATA_PATH", helmConfPath)
	err := InitHelmConfig()
	if err != nil {
		t.Error(err)
	}

	helmInstallData := HelmChartInstallRequest{
		Namespace: testNamespace,
		Chart:     testChart,
		Release:   testRelease,
		Values:    testValues,
		DryRun:    testDryRun,
	}
	_, err = HelmChartInstall(helmInstallData)
	if err != nil {
		t.Log(err)
		return err
	}
	return nil
}

func testSetup() error {
	config.Set("MO_HELM_DATA_PATH", helmConfPath)
	err := InitHelmConfig()
	if err != nil {
		return err
	}
	return nil
}
func TestHelmRepoAdd(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
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
	if testing.Short() {
		t.Skip()
	}
	err := testSetup()
	if err != nil {
		t.Error(err)
	}

	// PAT_NAMESPACE_HELM_REPO_UPDATE
	// no futher testing needed no error is sufficient
	_, err = HelmRepoUpdate()
	if err != nil {
		t.Error(err)
	}
}

func TestHelmRepoList(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
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
	listRepoData, err := HelmRepoList()
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
	if testing.Short() {
		t.Skip()
	}
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
	if testing.Short() {
		t.Skip()
	}
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
	releaseUpgradeData := HelmReleaseUpgradeRequest{
		Namespace: testNamespace,
		Chart:     testChart,
		Release:   testRelease,
		Values:    testValues,
		DryRun:    testDryRun,
	}
	_, err = HelmReleaseUpgrade(releaseUpgradeData)
	if err != nil {
		t.Error(err)
	}
}

func TestHelmListRequest(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
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
	releaseListData := HelmReleaseListRequest{
		Namespace: testNamespace,
	}
	releaseList, err := HelmReleaseList(releaseListData)
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
	if testing.Short() {
		t.Skip()
	}
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
	releaseStatusData := HelmReleaseStatusRequest{
		Namespace: testNamespace,
		Release:   testRelease,
	}
	_, err = HelmReleaseStatus(releaseStatusData)
	if err != nil {
		t.Error(err)
	}

	// PAT_NAMESPACE_HELM_SHOW
	// no futher testing needed no error is sufficient
	chartShowData := HelmChartShowRequest{
		Chart:      testChart,
		ShowFormat: action.ShowAll,
	}
	_, err = HelmChartShow(chartShowData)
	if err != nil {
		t.Error(err)
	}

	// PAT_NAMESPACE_HELM_GET
	// no futher testing needed no error is sufficient
	releaseGet := HelmReleaseGetRequest{
		Namespace: testNamespace,
		Release:   testRelease,
		GetFormat: structs.HelmGetAll,
	}
	_, err = HelmReleaseGet(releaseGet)
	if err != nil {
		t.Error(err)
	}

	// PAT_NAMESPACE_HELM_HISTORY
	// history should have at least 1 entry
	releaseHistoryData := HelmReleaseHistoryRequest{
		Namespace: testNamespace,
		Release:   testRelease,
	}
	historyList, err := HelmReleaseHistory(releaseHistoryData)
	if err != nil {
		t.Error(err)
	}
	if len(historyList) < 1 {
		t.Errorf("Release '%s' history not found but it should be", testRelease)
	}

	// PAT_NAMESPACE_HELM_ROLLBACK
	// no futher testing needed no error is sufficient
	releaseRollbackData := HelmReleaseRollbackRequest{
		Namespace: testNamespace,
		Release:   testRelease,
		Revision:  1,
	}
	_, err = HelmReleaseRollback(releaseRollbackData)
	if err != nil {
		t.Error(err)
	}
}
