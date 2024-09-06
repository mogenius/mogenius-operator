package iacmanager

import (
	"fmt"
	"mogenius-k8s-manager/gitmanager"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"os"
	"strings"
	"sync"
	"time"

	"sigs.k8s.io/yaml"

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

var changedFiles []ChangedFile

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
	// Create a git repository
	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	if _, err := os.Stat(folder); os.IsNotExist(err) {
		err := os.MkdirAll(folder, 0755)
		if err != nil {
			iaclogger.Errorf("Error creating folder for git repository (in %s): %s", folder, err.Error())
			return err
		}
	}

	err := gitmanager.InitGit(folder)
	if err != nil {
		iaclogger.Errorf("Error creating git repository: %s", err.Error())
		return err
	}

	err = gitmanager.CheckoutBranch(folder, utils.CONFIG.Iac.RepoBranch)
	if err != nil {
		iaclogger.Errorf("Error setting up branch: %s", err.Error())
		return err
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
	changedFiles = []ChangedFile{}

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
	return utils.CONFIG.Iac.AllowPull || utils.CONFIG.Iac.AllowPush
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
	createFolderForResource(kind)
	data := cleanYaml(string(yamlData))
	filename := fileNameForRaw(kind, namespace, resourceName)
	err = os.WriteFile(filename, []byte(data), 0755)
	if err != nil {
		iaclogger.Errorf("Error writing resource %s:%s/%s file: %s", kind, namespace, resourceName, err.Error())
		return
	}
	if utils.CONFIG.Iac.LogChanges {
		iaclogger.Infof("🧹 Detected %s change. Updated %s/%s.", kind, namespace, resourceName)
	}

	changedFiles = append(changedFiles, ChangedFile{
		AuthorName:  utils.CONFIG.Git.GitUserName,
		AutgorEmail: utils.CONFIG.Git.GitUserEmail,
		Name:        resourceName,
		Path:        filename,
		Message:     fmt.Sprintf("Updated [%s] %s/%s", kind, namespace, resourceName),
	})
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
		if _, err := os.Stat(filename); os.IsExist(err) {
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
	if err != nil {
		iaclogger.Errorf("Error deleting resource %s:%s/%s file: %s", kind, namespace, resourceName, err.Error())
		return
	}
	if utils.CONFIG.Iac.LogChanges {
		iaclogger.Infof("Detected %s deletion. Removed %s/%s. ♻️", kind, namespace, resourceName)
	}
	changedFiles = append(changedFiles, ChangedFile{
		AuthorName:  utils.CONFIG.Git.GitUserName,
		AutgorEmail: utils.CONFIG.Git.GitUserEmail,
		Name:        resourceName,
		Path:        filename,
		Message:     fmt.Sprintf("Deleted [%s] %s/%s", kind, namespace, resourceName),
	})
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
	yamlData1 = []byte(cleanYaml(string(yamlData1)))

	yamlRawData2, err := yaml.Marshal(dataInf)
	yamlData2 := cleanYaml(string(yamlRawData2))
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

func cleanYaml(data string) string {
	var dataMap map[string]interface{}
	err := yaml.Unmarshal([]byte(data), &dataMap)
	if err != nil {
		iaclogger.Errorf("Error unmarshalling yaml: %s", err.Error())
	}

	removeFieldAtPath(dataMap, "uid", []string{"metadata"}, []string{})
	removeFieldAtPath(dataMap, "selfLink", []string{"metadata"}, []string{})
	removeFieldAtPath(dataMap, "generation", []string{"metadata"}, []string{})
	removeFieldAtPath(dataMap, "managedFields", []string{"metadata"}, []string{})
	removeFieldAtPath(dataMap, "deployment.kubernetes.io/revision", []string{"annotations"}, []string{})
	removeFieldAtPath(dataMap, "kubectl.kubernetes.io/last-applied-configuration", []string{"annotations"}, []string{})

	removeFieldAtPath(dataMap, "creationTimestamp", []string{}, []string{})
	removeFieldAtPath(dataMap, "resourceVersion", []string{}, []string{})
	removeFieldAtPath(dataMap, "status", []string{}, []string{})

	cleanedYaml, err := yaml.Marshal(dataMap)
	if err != nil {
		iaclogger.Errorf("Error marshalling yaml: %s", err.Error())
	}
	return string(cleanedYaml)
}

func syncChangesTimer() {
	ticker := time.NewTicker(time.Duration(utils.CONFIG.Iac.SyncFrequencyInSec) * time.Second)
	quit := make(chan struct{}) // Create a channel to signal the ticker to stop

	go func() {
		for {
			select {
			case <-ticker.C:
				if IsIacEnabled() {
					SyncChanges()
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func ApplyRepoStateToCluster() {
	initialRepoApplied = true

	if !utils.CONFIG.Iac.AllowPull {
		return
	}

	allFiles := []string{}

	rootFolder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	folders, err := os.ReadDir(rootFolder)
	if err != nil {
		iaclogger.Errorf("Error reading directory: %s", err.Error())
		return
	}
	for _, folder := range folders {
		if folder.IsDir() && !strings.HasPrefix(folder.Name(), ".") {
			nextFolder := fmt.Sprintf("%s/%s", rootFolder, folder.Name())
			files, err := os.ReadDir(nextFolder)
			if err == nil {
				for _, f := range files {
					allFiles = append(allFiles, fmt.Sprintf("%s/%s", folder.Name(), f.Name()))
				}
			}
		}
	}

	allFiles = applyPriotityToChangesForUpdates(allFiles)

	for _, file := range allFiles {
		kubernetesReplaceResource(file, true)
	}
}

func SyncChanges() error {
	var err error
	if gitHasRemotes() {
		if !SetupInProcess {
			if !syncInProcess {
				syncInProcess = true
				updatedFiles, deletedFiles, err := pullChanges()
				SetPullError(err)
				if err != nil {
					iaclogger.Errorf("Error pulling changes: %s", err.Error())
				} else if !initialRepoApplied {
					ApplyRepoStateToCluster()
				}
				updatedFiles = applyPriotityToChangesForUpdates(updatedFiles)
				for _, v := range updatedFiles {
					kubernetesReplaceResource(v, false)
				}
				deletedFiles = applyPriotityToChangesForDeletes(deletedFiles)
				for _, v := range deletedFiles {
					kubernetesDeleteResource(v)
				}
				err = pushChanges()
				SetPushError(err)
				if err != nil {
					iaclogger.Errorf("Error pushing changes: %s", err.Error())
				}
				syncInProcess = false
				SetSyncError(nil)
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
	name := fmt.Sprintf("%s/%s/%s/%s_%s.yaml", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER, kind, namespace, resourceName)
	if namespace == "" {
		name = fmt.Sprintf("%s/%s/%s/%s.yaml", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER, kind, resourceName)
	}
	return name
}

func createFolderForResource(resource string) error {
	basePath := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	resourceFolder := fmt.Sprintf("%s/%s", basePath, resource)

	if _, err := os.Stat(resourceFolder); os.IsNotExist(err) {
		err := os.Mkdir(resourceFolder, 0755)
		if err != nil {
			iaclogger.Errorf("Error creating folder for resource: %s", err.Error())
			return err
		}
	}
	return nil
}

func gitHasRemotes() bool {
	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	return gitmanager.HasRemotes(folder)
}

func commitChanges(authorName string, authorEmail string, message string, files []string) {
	commitMutex.Lock()
	defer commitMutex.Unlock()

	if !utils.CONFIG.Iac.AllowPush {
		return
	}

	if authorName == "" {
		authorName = utils.CONFIG.Git.GitUserName
	}
	if authorEmail == "" {
		authorEmail = utils.CONFIG.Git.GitUserEmail
	}

	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)

	gitmanager.Commit(folder, files, message, authorName, authorEmail)
}

func pullChanges() (updatedFiles []string, deletedFiles []string, error error) {
	if !utils.CONFIG.Iac.AllowPull {
		return
	}
	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)

	defer func() {
		commit, err := gitmanager.GetLastCommit(folder)
		if err != nil {
			iaclogger.Errorf("Error getting last commit: %s", err.Error())
			return
		}
		SetLastPull(commit)
	}()

	// Pull changes from the remote repository
	err := gitmanager.Pull(folder, "origin", utils.CONFIG.Iac.RepoBranch)
	if err != nil {
		return updatedFiles, deletedFiles, err
	}
	// if err != nil {
	// 	if !strings.Contains(stderr.String(), "Your local changes to the following files would be overwritten by merge") {
	// 		iaclogger.Errorf("Error running git pull origin %s (%s): %s %s %s", utils.CONFIG.Iac.RepoBranch, utils.CONFIG.Iac.RepoUrl, err.Error(), out.String(), stderr.String())
	// 	}
	// 	if strings.Contains(stderr.String(), "fatal: refusing to merge unrelated histories") {
	// 		iaclogger.Warnf("Unrelated histories. Deleting current repository data and reinitializing.")
	// 		ResetCurrentRepoData(DELETE_DATA_RETRIES)
	// 	}
	// 	return updatedFiles, deletedFiles, err
	// }

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

	// clean files
	updatedFiles = cleanFileListFromNonYamls(updatedFiles)
	deletedFiles = cleanFileListFromNonYamls(deletedFiles)

	return updatedFiles, deletedFiles, nil
}

func pushChanges() error {
	if !utils.CONFIG.Iac.AllowPush {
		return nil
	}
	if len(changedFiles) <= 0 {
		return nil
	}

	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)

	defer func() {
		commit, err := gitmanager.GetLastCommit(folder)
		if err != nil {
			iaclogger.Errorf("Error getting last commit: %s", err.Error())
			return
		}
		SetLastPush(commit)
	}()

	// Push changes to the remote repository
	err := gitmanager.Push(folder, "origin")
	if err != nil {
		return fmt.Errorf("Error running git push: %s", err.Error())
	}
	iaclogger.Infof("🔄 Pushed %d changes to remote repository.", len(changedFiles))
	if utils.CONFIG.Misc.Debug {
		for _, file := range changedFiles {
			iaclogger.Info(file)
		}
	}
	changedFiles = []ChangedFile{}
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

func kubernetesDeleteResource(file string) {
	if shouldSkipResource(file) {
		return
	}

	diff := createDiffFromFile(file, "")

	parts := strings.Split(file, "/")
	filename := strings.Replace(parts[len(parts)-1], ".yaml", "", -1)
	partsName := strings.Split(filename, "_")
	kind := parts[0]
	namespace := ""
	name := ""
	if len(partsName) > 1 {
		namespace = fmt.Sprintf("--namespace=%s ", partsName[0])
		name = partsName[1]
	}

	if kind == "" {
		UpdateResourceStatus(kind, namespace, name, diff, SyncStateDeleted, fmt.Errorf("Kind cannot be empty. File: %s", file))
		return
	}
	if name == "" {
		// check if resource is cluster wide (so we dont habe a namespace)
		if len(partsName) > 0 {
			name = partsName[0]
		} else {
			UpdateResourceStatus(kind, namespace, name, diff, SyncStateDeleted, fmt.Errorf("Name cannot be empty. File: %s", file))
			return
		}
	}

	delCmd := fmt.Sprintf("kubectl delete %s %s%s", kind, namespace, name)
	err := utils.ExecuteShellCommandRealySilent(delCmd, delCmd)

	UpdateResourceStatus(kind, namespace, name, diff, SyncStateDeleted, err)
}

func kubernetesReplaceResource(file string, isInit bool) {
	filePath := fmt.Sprintf("%s/%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER, file)
	if shouldSkipResource(filePath) {
		return
	}

	diff := createDiffFromFile(filePath, "")

	syncType := SyncStateSynced
	if isInit {
		syncType = SyncStateInitialized
	}

	replaceCmd := fmt.Sprintf("kubectl replace -f %s", filePath)
	err := utils.ExecuteShellCommandRealySilent(replaceCmd, replaceCmd)
	if err != nil {
		if strings.Contains(err.Error(), "Error from server (NotFound):") {
			createCmd := fmt.Sprintf("kubectl create -f %s", filePath)
			err = utils.ExecuteShellCommandRealySilent(createCmd, createCmd)
			if err != nil {
				UpdateResourceStatusByFile(file, diff, syncType, fmt.Errorf("Error creating kubernetes resource file %s: %s", file, err.Error()))
				return
			}
		} else {
			UpdateResourceStatusByFile(file, diff, syncType, fmt.Errorf("Error replacing kubernetes resource file %s: %s", file, err.Error()))
			return
		}
	}
	UpdateResourceStatusByFile(file, diff, syncType, nil)
}

func kubernetesRevertFromPath(filePath string) error {
	if shouldSkipResource(filePath) {
		return nil
	}

	diff := createDiffFromFile(filePath, "")

	applyCmd := fmt.Sprintf("kubectl replace -f %s", filePath)
	err := utils.ExecuteShellCommandRealySilent(applyCmd, applyCmd)
	if err != nil {
		UpdateResourceStatusByFile(filePath, diff, SyncStateReverted, fmt.Errorf("Error applying revert file %s: %s", filePath, err.Error()))
		return err
	}

	UpdateResourceStatusByFile(filePath, diff, SyncStateReverted, err)
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
	if utils.CONFIG.Misc.Stage != "local" {
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

func removeFieldAtPath(data map[string]interface{}, field string, targetPath []string, currentPath []string) {
	// Check if the current path matches the target path for removal.
	if len(currentPath) >= len(targetPath) && strings.Join(currentPath[len(currentPath)-len(targetPath):], "/") == strings.Join(targetPath, "/") {
		delete(data, field)
	}
	// Continue searching within the map.
	for key, value := range data {
		switch v := value.(type) {
		case map[string]interface{}:
			removeFieldAtPath(v, field, targetPath, append(currentPath, key))
			// After processing the nested map, check if it's empty and remove it if so.
			if len(v) == 0 {
				delete(data, key)
			}
		case []interface{}:
			for i, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					// Construct a new path for each item in the list.
					newPath := append(currentPath, fmt.Sprintf("%s[%d]", key, i))
					removeFieldAtPath(itemMap, field, targetPath, newPath)
				}
			}
			// Clean up the slice if it becomes empty after deletion.
			if len(v) == 0 {
				delete(data, key)
			}
		default:
			// Check and delete empty values here.
			if isEmptyValue(value) {
				delete(data, key)
			}
		}
	}
}

// Helper function to determine if a value is "empty" for our purposes.
func isEmptyValue(value interface{}) bool {
	switch v := value.(type) {
	case string:
		return v == ""
	case []interface{}:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	case nil:
		return true
	default:
		return false
	}
}
