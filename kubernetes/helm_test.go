package kubernetes

import (
	"mogenius-k8s-manager/structs"
	"testing"

	"helm.sh/helm/v3/pkg/action"
)

func TestHelm(t *testing.T) {
	testNamespace := "default"
	testRepo := "bitnami"
	testChartUrl := "https://charts.bitnami.com/bitnami"
	testChart := "bitnami/nginx"
	testRelease := "nginx-test"
	testValues := "#values_yaml"
	testDryRun := false

	defer func() {
		// PAT_NAMESPACE_HELM_UNINSTALL - remove repo is purposely placed at the end
		// no futher testing needed no error is sufficient
		releaseUninstallData := HelmReleaseUninstallRequest{
			Namespace: testNamespace,
			Release:   testRelease,
			DryRun:    testDryRun,
		}
		_, err := HelmReleaseUninstall(releaseUninstallData)
		if err != nil {
			t.Error(err)
		}

		// PAT_NAMESPACE_HELM_REPO_REMOVE - remove repo is purposely placed at the end
		// no futher testing needed no error is sufficient
		repoRemoveData := HelmRepoRemoveRequest{
			Name: testRepo,
		}
		_, err = HelmRepoRemove(repoRemoveData)
		if err != nil {
			t.Error(err)
		}
	}()

	// PAT_NAMESPACE_HELM_REPO_ADD
	// no futher testing needed no error is sufficient
	repoAddData := HelmRepoAddRequest{
		Name: testRepo,
		Url:  testChartUrl,
	}
	_, err := HelmRepoAdd(repoAddData)
	if err != nil {
		t.Error(err)
	}

	// PAT_NAMESPACE_HELM_REPO_UPDATE
	// no futher testing needed no error is sufficient
	_, err = HelmRepoUpdate()
	if err != nil {
		t.Error(err)
	}

	// PAT_NAMESPACE_HELM_REPO_LIST
	// check if repo is added
	listRepoData, err := HelmRepoList()
	if err != nil {
		t.Error(err)
	}
	listSuccess := false
	for _, v := range listRepoData {
		if v.Name == testRepo {
			listSuccess = true
			break
		}
	}
	if !listSuccess {
		t.Errorf("Repo '%s' not found but it should be", testRepo)
	}

	// PAT_NAMESPACE_HELM_INSTALL
	// no futher testing needed no error is sufficient
	helmInstallData := HelmChartInstallRequest{
		Namespace: testNamespace,
		Chart:     testChart,
		Release:   "",
		Values:    testValues,
		DryRun:    testDryRun,
	}
	_, err = HelmChartInstall(helmInstallData)
	if err != nil {
		t.Error(err)
	}

	// PAT_NAMESPACE_HELM_UPGRADE
	// no futher testing needed no error is sufficient
	releaseUpgradeData := HelmReleaseUpgradeRequest{
		Namespace: testNamespace,
		Chart:     testChart,
		Release:   "",
		Values:    testValues,
		DryRun:    testDryRun,
	}
	_, err = HelmReleaseUpgrade(releaseUpgradeData)
	if err != nil {
		t.Error(err)
	}

	// PAT_NAMESPACE_HELM_LIST
	// check if release is added
	releaseListData := HelmReleaseListRequest{
		Namespace: "",
	}
	releaseList, err := HelmReleaseList(releaseListData)
	if err != nil {
		t.Error(err)
	}
	listReleasesSuccess := false
	for _, v := range releaseList {
		if v.Name == testRelease {
			listReleasesSuccess = true
			break
		}
	}
	if !listReleasesSuccess {
		t.Errorf("Release '%s' not found but it should be", testRelease)
	}

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
