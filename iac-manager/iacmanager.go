package iacmanager

import (
	"fmt"
	"mogenius-k8s-manager/gitmanager"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"os"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/yaml"

	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/kylelemons/godebug/pretty"
	log "github.com/sirupsen/logrus"
)

// 1. Create git repository locally
// 2. create a folder for every incoming resource
// 3. Clean the workload from unnecessary fields, and metadata
// 4. store a file for every incoming workload
// 5. commit changes
// 6. pull/push changes periodically

const (
	GIT_VAULT_FOLDER    = "git-vault"
	DELETE_DATA_RETRIES = 5
)

var commitMutex sync.Mutex

var syncInProcess = false
var SetupInProcess = false
var initialRepoApplied = false

var iaclogger = log.WithField("component", structs.ComponentIacManager)

func isIgnoredNamespace(namespace string) bool {
	return utils.ContainsString(utils.CONFIG.Iac.IgnoredNamespaces, namespace)
}

func isIgnoredNamespaceInFile(file string) (result bool, namespace *string) {
	for _, namespace := range utils.CONFIG.Iac.IgnoredNamespaces {
		if strings.Contains(file, fmt.Sprintf("%s_", namespace)) {
			return true, &namespace
		}
		if strings.Contains(file, fmt.Sprintf("namespaces/%s", namespace)) {
			return true, &namespace
		}
	}
	if strings.Contains(file, fmt.Sprintf("%s_", utils.CONFIG.Kubernetes.OwnNamespace)) ||
		strings.Contains(file, fmt.Sprintf("namespaces/%s", utils.CONFIG.Kubernetes.OwnNamespace)) {
		return true, &utils.CONFIG.Kubernetes.OwnNamespace
	}
	return false, nil
}

func Init() {
	// dependency injection to avoid circular dependencies
	kubernetes.IacManagerDeleteResourceYaml = DeleteResourceYaml
	kubernetes.IacManagerWriteResourceYaml = WriteResourceYaml
	kubernetes.IacManagerShouldWatchResources = ShouldWatchResources
	kubernetes.IacManagerSetupInProcess = SetupInProcess
	kubernetes.IacManagerResetCurrentRepoData = ResetCurrentRepoData
	kubernetes.IacManagerSyncChanges = SyncChanges
	kubernetes.IacManagerApplyRepoStateToCluster = ApplyRepoStateToCluster
	kubernetes.IacManagerDeleteDataRetries = DELETE_DATA_RETRIES

	InitDataModel()

	SetRepoError(gitInitRepo())

	// Set up the remote repository
	if !gitHasRemotes() {
		SetRemoteError(addRemote())
	}

	// START SYNCING CHANGES
	syncChangesTimer()
}

func ShouldWatchResources() bool {
	return utils.CONFIG.Iac.AllowPull || utils.CONFIG.Iac.AllowPush
}

func gitInitRepo() error {
	var err error

	// Create a git repository
	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	if _, err = os.Stat(folder); os.IsNotExist(err) {
		err := os.MkdirAll(folder, 0755)
		if err != nil {
			iaclogger.Errorf("Error creating folder for git repository (in %s): %s", folder, err.Error())
			return err
		}
	}

	if utils.CONFIG.Iac.RepoUrl == "" {
		err = gitmanager.InitGit(folder)
		if err != nil {
			iaclogger.Errorf("Error creating git repository: %s", err.Error())
			return err
		}
	}

	if utils.CONFIG.Iac.RepoUrl != "" {
		err = gitmanager.CloneFast(insertPATIntoURL(utils.CONFIG.Iac.RepoUrl, utils.CONFIG.Iac.RepoPat), folder, utils.CONFIG.Iac.RepoBranch)
		if err != nil {
			iaclogger.Errorf("Error setting up branch: %s", err.Error())
			return err
		}
	}

	return err
}

func addRemote() error {
	if utils.CONFIG.Iac.RepoUrl == "" {
		return fmt.Errorf("Repository URL is empty. Please set the repository URL in the configuration file or as env var.")
	}
	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	err := gitmanager.AddRemote(folder, insertPATIntoURL(utils.CONFIG.Iac.RepoUrl, utils.CONFIG.Iac.RepoPat), "origin")
	if err != nil {
		return err
	}
	err = gitmanager.CheckoutBranch(folder, utils.CONFIG.Iac.RepoBranch)
	if err != nil {
		iaclogger.Errorf("Error setting up branch: %s", err.Error())
	}

	return nil
}

func ResetCurrentRepoData(tries int) error {
	ClearChangedFiles()

	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	err := os.RemoveAll(folder)
	if err != nil {
		iaclogger.Errorf("Error deleting current repository data: %s", err.Error())
		if tries > 0 {
			time.Sleep(1 * time.Second)
			return ResetCurrentRepoData(tries - 1)
		}
	}

	err = gitInitRepo()
	SetRepoError(err)
	if err != nil {
		return err
	}

	err = addRemote()
	SetRemoteError(err)

	return err
}

func CheckRepoAccess() error {
	if utils.CONFIG.Iac.RepoUrl == "" {
		err := fmt.Errorf("Repository URL is empty. Please set the repository URL in the configuration file or as env var.")
		iaclogger.Error(err)
		return err
	}
	if utils.CONFIG.Iac.RepoPat == "" {
		err := fmt.Errorf("Repository PAT is empty. Please set the repository PAT in the configuration file or as env var.")
		iaclogger.Error(err)
		return err
	}
	// Insert the PAT into the repository URL
	repoURLWithPAT := insertPATIntoURL(utils.CONFIG.Iac.RepoUrl, utils.CONFIG.Iac.RepoPat)

	_, err := gitmanager.LsRemotes(repoURLWithPAT)
	return err
}

func IsIacEnabled() bool {
	if utils.CONFIG.Iac.AllowPull {
		return true
	}
	if utils.CONFIG.Iac.AllowPush {
		return true
	}
	return false
}

func WriteResourceYaml(kind string, namespace string, resourceName string, dataInf interface{}) {
	if SetupInProcess {
		return
	}
	if !IsIacEnabled() {
		return
	}

	// Exceptions
	if isIgnoredNamespace(namespace) {
		return
	}
	if shouldSkipResource(fileNameForRaw(kind, namespace, resourceName)) {
		return
	}

	diff := createDiff(kind, namespace, resourceName, dataInf)
	if utils.CONFIG.Iac.ShowDiffInLog {
		if diff != "" {
			iaclogger.Warnf("Diff: \n%s", diff)
		}
	}

	// all changes will be reversed if PULL only is allowed
	if utils.CONFIG.Iac.AllowPull && !utils.CONFIG.Iac.AllowPush && diff != "" {
		filename := fileNameForRaw(kind, namespace, resourceName)
		if _, err := os.Stat(filename); err == nil {
			err = kubernetesRevertFromPath(filename)
			if err == nil {
				iaclogger.Warnf("🧹 Detected %s change. Reverting %s/%s.", kind, namespace, resourceName)
			}
		}
		return
	}

	if !utils.CONFIG.Iac.AllowPush {
		return
	}
	if kind == "" {
		iaclogger.Errorf("Kind is empty for resource %s:%s/%s", kind, namespace, resourceName)
		return
	}
	yamlData, err := yaml.Marshal(dataInf)
	if err != nil {
		iaclogger.Errorf("Error marshaling to YAML: %s\n", err.Error())
		return
	}
	createFolderForResource(kind, namespace)
	data := utils.CleanYaml(string(yamlData))
	filename := fileNameForRaw(kind, namespace, resourceName)
	err = os.WriteFile(filename, []byte(data), 0755)
	if err != nil {
		iaclogger.Errorf("Error writing resource %s:%s/%s file: %s", kind, namespace, resourceName, err.Error())
		return
	}
	if utils.CONFIG.Iac.LogChanges {
		iaclogger.Infof("🧹 Detected %s change. Updated %s/%s.", kind, namespace, resourceName)
	}
	UpdateResourceStatus(kind, namespace, resourceName, SyncStatePendingSync, nil)

	if diff != "" {
		AddChangedFile(ChangedFile{
			Author:     utils.CONFIG.Git.GitUserName + " <" + utils.CONFIG.Git.GitUserEmail + ">",
			Kind:       kind,
			Name:       resourceName,
			Path:       filename,
			Message:    fmt.Sprintf("Updated [%s] %s/%s", kind, namespace, resourceName),
			ChangeType: SyncChangeTypeModify,
		})
	} else {
		UpdateResourceStatus(kind, namespace, resourceName, SyncStateSynced, nil)
	}
}

func DeleteResourceYaml(kind string, namespace string, resourceName string, objectToDelete interface{}) {
	if SetupInProcess {
		return
	}
	if !IsIacEnabled() {
		return
	}

	// Exceptions
	if isIgnoredNamespace(namespace) {
		return
	}
	if shouldSkipResource(fileNameForRaw(kind, namespace, resourceName)) {
		return
	}

	diff := createDiff(kind, namespace, resourceName, make(map[string]interface{}))
	if utils.CONFIG.Iac.ShowDiffInLog {
		if diff != "" {
			iaclogger.Warnf("Diff: \n%s", diff)
		}
	}

	// all changes will be reversed if PULL only is allowed
	if utils.CONFIG.Iac.AllowPull && !utils.CONFIG.Iac.AllowPush {
		filename := fileNameForRaw(kind, namespace, resourceName)
		_, err := os.Stat(filename)
		if os.IsNotExist(err) {
			return
		} else {
			err = kubernetesRevertFromPath(filename)
			if err == nil {
				iaclogger.Warnf("🧹 Detected %s deletion. Reverting %s/%s.", kind, namespace, resourceName)
			}
		}
		return
	}

	if !utils.CONFIG.Iac.AllowPush {
		return
	}

	filename := fileNameForRaw(kind, namespace, resourceName)
	err := os.Remove(filename)
	if err != nil && !os.IsNotExist(err) {
		iaclogger.Errorf("Error deleting resource %s:%s/%s file: %s", kind, namespace, resourceName, err.Error())
		return
	}
	if utils.CONFIG.Iac.LogChanges {
		iaclogger.Infof("Detected %s deletion. Removed %s/%s. ♻️", kind, namespace, resourceName)
	}
	UpdateResourceStatus(kind, namespace, resourceName, SyncStateDeleted, nil)

	if diff != "" {
		AddChangedFile(ChangedFile{
			Name:       resourceName,
			Kind:       kind,
			Path:       filename,
			Message:    fmt.Sprintf("Deleted [%s] %s/%s", kind, namespace, resourceName),
			ChangeType: SyncChangeTypeDelete,
		})
	}
}

func createDiff(kind string, namespace string, resourceName string, dataInf interface{}) string {
	filename := fileNameForRaw(kind, namespace, resourceName)
	return createDiffFromFile(filename, dataInf)
}

func createDiffFromFile(filename string, dataInf interface{}) string {
	yamlData1, _ := os.ReadFile(filename)
	if yamlData1 == nil {
		yamlData1 = []byte{}
	}
	yamlData1 = []byte(utils.CleanYaml(string(yamlData1)))

	yamlRawData2, err := yaml.Marshal(dataInf)
	yamlData2 := utils.CleanYaml(string(yamlRawData2))
	if err != nil {
		iaclogger.Errorf("Error marshaling to YAML: %s\n", err.Error())
		return ""
	}

	var obj1, obj2 interface{}

	err = yaml.Unmarshal(yamlData1, &obj1)
	if err != nil {
		iaclogger.Errorf("Error unmarshalling yaml1 for diff: %s", err.Error())
		return ""
	}
	if obj1 == nil {
		obj1 = make(map[string]interface{})
	}

	err = yaml.Unmarshal([]byte(yamlData2), &obj2)
	if err != nil {
		iaclogger.Errorf("Error unmarshalling yaml2 for diff: %s", err.Error())
		return ""
	}

	diffRaw := pretty.Compare(obj1, obj2)
	return diffRaw
}

func insertPATIntoURL(gitRepoURL, pat string) string {
	if pat == "" {
		return gitRepoURL
	}
	if !strings.HasPrefix(gitRepoURL, "https://") {
		return gitRepoURL // Non-HTTPS URLs are not handled here
	}
	return strings.Replace(gitRepoURL, "https://", "https://"+pat+"@", 1)
}

func syncChangesTimer() {
	ticker := time.NewTicker(time.Duration(utils.CONFIG.Iac.SyncFrequencyInSec) * time.Second)
	quit := make(chan struct{}) // Create a channel to signal the ticker to stop

	go func() {
		for {
			select {
			case <-ticker.C:
				if IsIacEnabled() {
					err := SyncChanges()
					if err != nil {
						iaclogger.Errorf("Error syncing changes: %s", err.Error())
					}
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func ApplyRepoStateToCluster() error {
	initialRepoApplied = true

	if !utils.CONFIG.Iac.AllowPull {
		return nil
	}

	allFiles := []string{}

	rootFolder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	folders, err := os.ReadDir(rootFolder)
	if err != nil {
		iaclogger.Errorf("Error reading directory: %s", err.Error())
		return nil
	}
	for _, folder := range folders {
		if folder.IsDir() && !strings.HasPrefix(folder.Name(), ".") {
			nextFolder := fmt.Sprintf("%s/%s", rootFolder, folder.Name())
			files, err := os.ReadDir(nextFolder)
			if err == nil {
				for _, f := range files {
					if f.IsDir() && !strings.HasPrefix(folder.Name(), ".") {
						nextFolder := fmt.Sprintf("%s/%s", nextFolder, f.Name())
						namespacedFiles, err := os.ReadDir(nextFolder)
						if err == nil {
							for _, namespacedFile := range namespacedFiles {
								if !strings.HasPrefix(namespacedFile.Name(), ".") {
									allFiles = append(allFiles, fmt.Sprintf("%s/%s", nextFolder, namespacedFile.Name()))
								}
							}
						}
					} else if !strings.HasPrefix(f.Name(), ".") {
						allFiles = append(allFiles, fmt.Sprintf("%s/%s", folder.Name(), f.Name()))
					}
				}
			}
		}
	}

	allFiles = applyPriotityToChangesForUpdates(allFiles)

	for _, file := range allFiles {
		kubernetesReplaceResource(file)
	}

	return nil
}

func SyncChanges() error {
	defer func() {
		syncInProcess = false
	}()

	startTime := time.Now()

	var err error
	if gitHasRemotes() {
		if !SetupInProcess {
			if !syncInProcess {
				syncInProcess = true
				lastCommit, updatedFiles, deletedFiles, err := pullChanges(GetLastAppliedCommit())
				SetPullError(err)
				if err != nil {
					iaclogger.Errorf("Error pulling changes: %s", err.Error())
					return err
				} else if !initialRepoApplied {
					err = ApplyRepoStateToCluster()
					SetSyncError(err)
					if err != nil {
						return err
					}
				}
				updatedFiles = applyPriotityToChangesForUpdates(updatedFiles)
				for _, v := range updatedFiles {
					err = kubernetesReplaceResource(v)
					SetSyncError(err)
					if err != nil {
						return err
					}
				}
				deletedFiles = applyPriotityToChangesForDeletes(deletedFiles)
				for _, v := range deletedFiles {
					err = kubernetesDeleteResource(v)
					SetSyncError(err)
					if err != nil {
						return err
					}
				}
				SetLastSuccessfullyAppliedCommit(lastCommit)
				err = pushChanges()
				SetPushError(err)
				if err != nil {
					iaclogger.Errorf("Error pushing changes: %s", err.Error())
					return err
				}
				SetSyncError(nil)
				SetSyncInfo(time.Since(startTime).Milliseconds())
			} else {
				err = fmt.Errorf("Sync in process. Skipping sync.")
			}
		} else {
			err = fmt.Errorf("Setup in process. Skipping sync.")
		}
	} else {
		err = fmt.Errorf("No remotes found. Skipping sync.")
	}
	if err != nil {
		iaclogger.Warnf(err.Error())
	}
	SetSyncError(err)
	return err
}

func fileNameForRaw(kind string, namespace string, resourceName string) string {
	name := fmt.Sprintf("%s/%s/%s/%s/%s.yaml", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER, kind, namespace, resourceName)
	if namespace == "" {
		name = fmt.Sprintf("%s/%s/%s/%s.yaml", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER, kind, resourceName)
	}
	return name
}

func GitFilePathForRaw(kind string, namespace string, resourceName string) string {
	file := fileNameForRaw(kind, namespace, resourceName)
	return strings.Replace(file, fmt.Sprintf("%s/%s/", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER), "", 1)
}

func createFolderForResource(resource string, namespace string) error {
	basePath := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	resourceFolder := fmt.Sprintf("%s/%s", basePath, resource)
	if namespace != "" {
		resourceFolder = fmt.Sprintf("%s/%s/%s", basePath, resource, namespace)
	}
	if _, err := os.Stat(resourceFolder); os.IsNotExist(err) {
		err := os.MkdirAll(resourceFolder, 0755)
		if err != nil {
			iaclogger.Errorf("Error creating folder for resource: %s", err.Error())
			return err
		}
	}
	return nil
}

func ResetFile(filePath, commitHash string) error {
	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	kind, _, name := parseFileToK8sParts(filePath)

	err := gitmanager.ResetFileToCommit(folder, filePath, commitHash)
	if err != nil {
		iaclogger.Errorf("Error resetting file: %s", err.Error())
		return err
	}

	err = gitmanager.Commit(folder, []string{filePath}, []string{}, fmt.Sprintf("Reset [%s] %s to %s.", kind, name, commitHash), utils.CONFIG.Git.GitUserName, utils.CONFIG.Git.GitUserEmail)
	if err != nil {
		iaclogger.Errorf("Error committing reset: %s", err.Error())
		return err
	}

	return nil
}

func gitHasRemotes() bool {
	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	return gitmanager.HasRemotes(folder)
}

func pullChanges(lastAppliedCommit GitActionStatus) (lastCommit *object.Commit, updatedFiles []string, deletedFiles []string, error error) {
	if !utils.CONFIG.Iac.AllowPull {
		return
	}
	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)

	defer func() {
		commits, err := gitmanager.GetLastCommits(folder, gitmanager.Max_Commit_History)
		if err != nil {
			iaclogger.Errorf("Error getting last commit: %s", err.Error())
			return
		}
		SetCommitHistory(commits)
	}()

	// Pull changes from the remote repository
	lastCommit, err := gitmanager.Pull(folder, "origin", utils.CONFIG.Iac.RepoBranch)
	if err != nil {
		return lastCommit, updatedFiles, deletedFiles, err
	}

	// nothing changed
	if lastCommit.Hash.String() == lastAppliedCommit.CommitHash {
		return lastCommit, []string{}, []string{}, nil
	}

	//Get the list of updated or newly added files since the last pull
	updatedFiles, err = gitmanager.GetLastUpdatedAndModifiedFiles(folder)
	if err != nil {
		iaclogger.Errorf("Error getting updated files: %s", err.Error())
		return
	}

	// Get the list of deleted files since the last pull
	deletedFiles, err = gitmanager.GetLastDeletedFiles(folder)
	if err != nil {
		iaclogger.Errorf("Error getting deleted files: %s", err.Error())
		return
	}

	iaclogger.Infof("🔄 Pulled changes from the remote repository (Modified: %d / Deleted: %d).", len(updatedFiles), len(deletedFiles))
	if utils.CONFIG.Misc.Debug {
		iaclogger.Infof("Added/Updated Files (%d):", len(updatedFiles))
		for _, file := range updatedFiles {
			iaclogger.Info(file)
		}

		iaclogger.Infof("Deleted Files (%d):", len(deletedFiles))
		for _, file := range deletedFiles {
			iaclogger.Info(file)
		}
	}

	// Get the list of contributors
	contributor, contributorErr := gitmanager.GetContributors(folder)
	if contributorErr == nil {
		SetContributors(contributor)
	}

	// clean files
	updatedFiles = cleanFileListFromNonYamls(updatedFiles)
	deletedFiles = cleanFileListFromNonYamls(deletedFiles)

	return lastCommit, updatedFiles, deletedFiles, err
}

func pushChanges() error {
	if !utils.CONFIG.Iac.AllowPush {
		return nil
	}
	if ChangedFilesEmpty() {
		return nil
	}

	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)

	// Commit changes
	deletedFiles := []string{}
	updatedOrAddedFiles := []string{}
	commitMsg := ""
	for _, file := range GetChangedFiles() {
		if file.ChangeType == SyncChangeTypeDelete {
			deletedFiles = append(deletedFiles, file.Path)
		} else if file.ChangeType == SyncChangeTypeModify {
			updatedOrAddedFiles = append(updatedOrAddedFiles, file.Path)
		} else {
			iaclogger.Errorf("SyncChangeType unimplemented: %s", file.ChangeType)
		}
		commitMsg += file.Message + "\n"
	}
	err := gitmanager.Commit(folder, updatedOrAddedFiles, deletedFiles, commitMsg, utils.CONFIG.Git.GitUserName, utils.CONFIG.Git.GitUserEmail)
	if err != nil {
		return fmt.Errorf("Error running git commit: %s", err.Error())
	}

	// Push changes to the remote repository
	err = gitmanager.Push(folder, "origin")
	if err != nil {
		return fmt.Errorf("Error running git push: %s", err.Error())
	}
	iaclogger.Infof("🔄 Pushed %d changes to remote repository.", ChangedFilesLen())
	if utils.CONFIG.Misc.Debug {
		for _, file := range GetChangedFiles() {
			iaclogger.Info(file)
		}
	}
	ClearChangedFiles()
	return nil
}

// namespaces should be applied first
func applyPriotityToChangesForUpdates(resources []string) []string {
	result := []string{}
	// sort by kind to apply the changes in the right order
	for _, v := range resources {
		if strings.Contains(v, "namespaces/") {
			// instert at first position
			result = append([]string{v}, result...)
		} else {
			// append at the end
			result = append(result, v)
		}

	}
	return result
}

// namespaces should be applied last
func applyPriotityToChangesForDeletes(resources []string) []string {
	result := []string{}
	// sort by kind to apply the changes in the right order
	for _, v := range resources {
		if strings.Contains(v, "namespaces/") {
			// append at the end
			result = append(result, v)
		} else {
			// instert at first position
			result = append([]string{v}, result...)
		}

	}
	return result
}

func cleanFileListFromNonYamls(files []string) []string {
	result := []string{}
	for _, file := range files {
		if strings.HasSuffix(file, ".yaml") {
			result = append(result, file)
		}
	}
	return result
}

func kubernetesDeleteResource(file string) error {
	if shouldSkipResource(file) {
		return nil
	}

	kind, namespace, name := parseFileToK8sParts(file)

	if kind == "" {
		UpdateResourceStatus(kind, namespace, name, SyncStateDeleted, fmt.Errorf("Kind cannot be empty. File: %s", file))
		return nil
	}
	if name == "" {
		UpdateResourceStatus(kind, namespace, name, SyncStateDeleted, fmt.Errorf("Name cannot be empty. File: %s", file))
		return nil
	}

	namespaceflag := ""
	if namespace != "" {
		namespaceflag = fmt.Sprintf("--namespace=%s ", namespace)
	}
	delCmd := fmt.Sprintf("kubectl delete %s %s%s", kind, namespaceflag, name)
	err := utils.ExecuteShellCommandRealySilent(delCmd, delCmd)

	if err != nil {
		if strings.Contains(err.Error(), "Error from server (NotFound):") {
			UpdateResourceStatus(kind, namespace, name, SyncStateDeleted, nil)
			return nil
		}
	}

	UpdateResourceStatus(kind, namespace, name, SyncStateDeleted, err)

	return err
}

func parseFileToK8sParts(file string) (kind string, namespace string, name string) {
	file = strings.Replace(file, fmt.Sprintf("%s/%s/", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER), "", 1)
	parts := strings.Split(file, "/")
	filename := strings.Replace(parts[len(parts)-1], ".yaml", "", -1)
	kind = parts[0]
	namespace = ""
	name = filename

	if len(parts) == 3 {
		namespace = parts[1]
	}

	return kind, namespace, name
}

func kubernetesReplaceResource(file string) error {
	filePath := fmt.Sprintf("%s/%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER, file)
	if shouldSkipResource(filePath) {
		return nil
	}

	kind, namespace, name := parseFileToK8sParts(file)
	existingResource, err := kubernetes.ObjectFor(kind, namespace, name)
	if err != nil && !apierrors.IsNotFound(err) {
		UpdateResourceStatusByFile(file, SyncStateSynced, fmt.Errorf("Error getting existing kubernetes resource %s: %s", file, err.Error()))
		return nil
	}

	diff := createDiff(kind, namespace, name, existingResource)
	if diff == "" {
		UpdateResourceStatusByFile(file, SyncStateSynced, nil)
		return nil
	}

	syncType := SyncStateSynced

	// if the resource is not found, create it
	if apierrors.IsNotFound(err) {
		createCmd := fmt.Sprintf("kubectl create -f %s", filePath)
		err = utils.ExecuteShellCommandRealySilent(createCmd, createCmd)
	} else {
		replaceCmd := fmt.Sprintf("kubectl replace -f %s", filePath)
		err = utils.ExecuteShellCommandRealySilent(replaceCmd, replaceCmd)
	}

	if err != nil {
		if strings.Contains(err.Error(), "Error from server (NotFound):") {
			createCmd := fmt.Sprintf("kubectl create -f %s", filePath)
			err = utils.ExecuteShellCommandRealySilent(createCmd, createCmd)
			if err != nil {
				UpdateResourceStatusByFile(file, SyncStateSyncError, err)
				return err
			}
		} else {
			UpdateResourceStatusByFile(file, SyncStateSyncError, err)
			return err
		}
	}
	UpdateResourceStatusByFile(file, syncType, nil)
	return nil
}

func kubernetesRevertFromPath(filePath string) error {
	if shouldSkipResource(filePath) {
		return nil
	}

	applyCmd := fmt.Sprintf("kubectl replace -f %s", filePath)
	err := utils.ExecuteShellCommandRealySilent(applyCmd, applyCmd)
	if err != nil {
		// not found means the resource was already deleted. this is fine for us
		if strings.Contains(err.Error(), "Error from server (NotFound):") {
			UpdateResourceStatusByFile(filePath, SyncStateSynced, nil)
			return err
		}
		UpdateResourceStatusByFile(filePath, SyncStateReverted, fmt.Errorf("Error applying revert file %s: %s", filePath, err.Error()))
		return err
	}

	UpdateResourceStatusByFile(filePath, SyncStateReverted, nil)
	return nil
}

func shouldSkipResource(path string) bool {
	if strings.Contains(path, "kube-root-ca.crt") {
		iaclogger.Debugf("😑 Skipping (because kube-root-ca.crt): %s", path)
		return true
	}

	if strings.Contains(path, "/pods/") {
		iaclogger.Debugf("😑 Skipping (because pods won't by synced): %s", path)
		return true
	}
	if utils.CONFIG.Kubernetes.RunInCluster {
		if strings.Contains(path, "/mogenius-k8s-manager") {
			iaclogger.Debugf("😑 Skipping (because contains keyword mogenius-k8s-manager): %s", path)
			return true
		}
	}
	hasIgnoredNs, nsName := isIgnoredNamespaceInFile(path)
	if hasIgnoredNs {
		iaclogger.Debugf("😑 Skipping (because contains ignored namespace '%s'): %s", *nsName, path)
		return true
	}
	return false
}
