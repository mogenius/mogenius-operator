package iacmanager

import (
	"errors"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/gitmanager"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/utils"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/yaml"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// 1. Create git repository locally
// 2. create a folder for every incoming resource
// 3. Clean the workload from unnecessary fields, and metadata
// 4. store a file for every incoming workload
// 5. commit changes
// 6. pull/push changes periodically

const (
	DELETE_DATA_RETRIES = 5
)

var SetupInProcess atomic.Bool
var initialRepoApplied = false

var gitSyncLock sync.Mutex

var iacLogger *slog.Logger
var config interfaces.ConfigModule

func Setup(logManagerModule interfaces.LogManagerModule, configModule interfaces.ConfigModule) {
	iacLogger = logManagerModule.CreateLogger("iac")
	config = configModule
}

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
	if strings.Contains(file, fmt.Sprintf("%s_", config.Get("MO_OWN_NAMESPACE"))) ||
		strings.Contains(file, fmt.Sprintf("namespaces/%s", config.Get("MO_OWN_NAMESPACE"))) {
		ownNamespace := config.Get("MO_OWN_NAMESPACE")
		return true, &ownNamespace
	}
	return false, nil
}

func Start() {
	// dependency injection to avoid circular dependencies
	kubernetes.IacManagerDeleteResourceYaml = DeleteResourceYaml
	kubernetes.IacManagerWriteResourceYaml = WriteResourceYaml
	kubernetes.IacManagerShouldWatchResources = ShouldWatchResources
	kubernetes.IacManagerSetupInProcess = &SetupInProcess
	kubernetes.IacManagerResetCurrentRepoData = ResetCurrentRepoData
	kubernetes.IacManagerSyncChanges = SyncChanges
	kubernetes.IacManagerApplyRepoStateToCluster = ApplyRepoStateToCluster
	kubernetes.IacManagerDeleteDataRetries = DELETE_DATA_RETRIES

	InitDataModel()

	if !IsIacEnabled() {
		err := ResetCurrentRepoData(3)
		if err != nil {
			iacLogger.Error("failed to reset repo data", "error", err)
		}
		InitDataModel()
		return
	}

	SetRepoError(GitInitRepo())

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

func GitInitRepo() error {
	var err error

	// Create a git repository
	if _, err := os.Stat(utils.CONFIG.Kubernetes.GitVaultDataPath); os.IsNotExist(err) {
		err := os.MkdirAll(utils.CONFIG.Kubernetes.GitVaultDataPath, 0755)
		if err != nil {
			iacLogger.Error("Error creating folder for git repository", "repoPath", utils.CONFIG.Kubernetes.GitVaultDataPath, "error", err)
			return err
		}
		err = gitmanager.InitGit(utils.CONFIG.Kubernetes.GitVaultDataPath)
		if err != nil {
			iacLogger.Error("Error creating git repository", "error", err)
			return err
		}
	}

	if utils.CONFIG.Iac.RepoUrl == "" {
		err = gitmanager.InitGit(utils.CONFIG.Kubernetes.GitVaultDataPath)
		if err != nil {
			iacLogger.Error("Error creating git repository", "error", err)
			return err
		}
	}

	if utils.CONFIG.Iac.RepoUrl != "" {
		gitRepoUrl := insertPATIntoURL(utils.CONFIG.Iac.RepoUrl, utils.CONFIG.Iac.RepoPat)
		err = gitmanager.CloneFast(gitRepoUrl, utils.CONFIG.Kubernetes.GitVaultDataPath, utils.CONFIG.Iac.RepoBranch)
		if err != nil {
			switch err {
			case transport.ErrEmptyRemoteRepository:
				err = initializeRemoteBranch(gitRepoUrl, utils.CONFIG.Kubernetes.GitVaultDataPath, utils.CONFIG.Iac.RepoBranch)
				if err != nil {
					iacLogger.Error("Error initializing new sync Repo", "syncRepo", utils.CONFIG.Iac.RepoUrl, "repoBranch", utils.CONFIG.Iac.RepoBranch, "error", err)
					return err
				}
			default:
				iacLogger.Error("Error cloning Repo", "syncRepo", utils.CONFIG.Iac.RepoUrl, "repoBranch", utils.CONFIG.Iac.RepoBranch, "error", err)
				return err
			}
		}
	}

	return err
}

func addRemote() error {
	gitRepoUrl := insertPATIntoURL(utils.CONFIG.Iac.RepoUrl, utils.CONFIG.Iac.RepoPat)
	if utils.CONFIG.Iac.RepoUrl == "" {
		return fmt.Errorf("Repository URL is empty. Please set the repository URL in the configuration file or as env var.")
	}
	err := gitmanager.AddRemote(utils.CONFIG.Kubernetes.GitVaultDataPath, gitRepoUrl, "origin")
	if err != nil {
		return err
	}
	err = gitmanager.CheckoutBranch(utils.CONFIG.Kubernetes.GitVaultDataPath, utils.CONFIG.Iac.RepoBranch)
	if err != nil {
		switch err {
		case transport.ErrEmptyRemoteRepository:
			err = initializeRemoteBranch(gitRepoUrl, utils.CONFIG.Kubernetes.GitVaultDataPath, utils.CONFIG.Iac.RepoBranch)
			if err != nil {
				iacLogger.Error("Error initializing new sync Branch", "syncRepo", utils.CONFIG.Iac.RepoUrl, "repoBranch", utils.CONFIG.Iac.RepoBranch, "error", err)
				return err
			}
			err = gitmanager.CheckoutBranch(utils.CONFIG.Kubernetes.GitVaultDataPath, utils.CONFIG.Iac.RepoBranch)
			if err != nil {
				iacLogger.Error("Error checking out newly created remote Branch", "syncRepo", utils.CONFIG.Iac.RepoUrl, "repoBranch", utils.CONFIG.Iac.RepoBranch, "error", err)
				return err
			}
		default:
			iacLogger.Error("Error checking out Branc", "syncRepo", utils.CONFIG.Iac.RepoUrl, "repoBranch", utils.CONFIG.Iac.RepoBranch, "error", err)
			return err
		}
	}

	return nil
}

func initializeRemoteBranch(remoteRepoUrl string, localRepoPath string, branchName string) error {
	var err error

	r, err := git.PlainInit(localRepoPath, false)
	if err != nil && err != git.ErrRepositoryAlreadyExists {
		iacLogger.Error("Error creating git repository", "error", err)
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		iacLogger.Error("Error opening git worktree", "error", err.Error())
		return err
	}

	err = gitmanager.AddRemote(localRepoPath, remoteRepoUrl, "origin")
	if err != nil {
		iacLogger.Error("Error adding remote", "error", err)
		return err
	}

	err = gitmanager.Commit(
		localRepoPath,
		[]string{},
		[]string{},
		"init",
		config.Get("MO_GIT_USER_NAME"),
		config.Get("MO_GIT_USER_EMAIL"),
	)
	if err != nil {
		iacLogger.Error("Error creating initial commit", "error", err)
		return err
	}

	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branchName),
		Create: true,
		Force:  true,
		Keep:   false,
	})
	if err != nil {
		iacLogger.Error("Error checking out new local branch", "error", err)
		return err
	}

	err = gitmanager.Push(localRepoPath, "origin")
	if err != nil {
		iacLogger.Error("Error pushing repo", "error", err)
		return err
	}

	return nil
}

func ResetCurrentRepoData(tries int) error {
	ClearChangedFiles()

	err := os.RemoveAll(utils.CONFIG.Kubernetes.GitVaultDataPath)
	if err != nil {
		iacLogger.Error("Error deleting current repository data", "error", err)
		if tries > 0 {
			time.Sleep(1 * time.Second)
			return ResetCurrentRepoData(tries - 1)
		}
	}

	err = GitInitRepo()
	SetRepoError(err)
	if err != nil {
		return err
	}

	err = addRemote()
	SetRemoteError(err)

	InitDataModel()

	return err
}

func CheckRepoAccess() error {
	if utils.CONFIG.Iac.RepoUrl == "" {
		err := fmt.Errorf("Repository URL is empty. Please set the repository URL in the configuration file or as env var.")
		iacLogger.Error(err.Error())
		return err
	}
	if utils.CONFIG.Iac.RepoPat == "" {
		err := fmt.Errorf("Repository PAT is empty. Please set the repository PAT in the configuration file or as env var.")
		iacLogger.Error(err.Error())
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
	var err error

	if SetupInProcess.Load() {
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

	diff, err := createDiff(kind, namespace, resourceName, dataInf)
	if err != nil {
		UpdateResourceStatus(kind, namespace, resourceName, SyncStatePendingSync, fmt.Errorf("Error creating diff: %s", err.Error()))
		return
	}
	if diff == "" {
		UpdateResourceStatus(kind, namespace, resourceName, SyncStateSynced, nil)
		return
	}

	if utils.CONFIG.Iac.ShowDiffInLog {
		if diff != "" {
			iacLogger.Info("iac manager diff", "diff", diff)
		}
	}

	filename := fileNameForRaw(kind, namespace, resourceName)
	gitFilePath := GitFilePathForRaw(kind, namespace, resourceName)

	// all changes will be reversed if PULL only is allowed
	if utils.CONFIG.Iac.AllowPull && !utils.CONFIG.Iac.AllowPush && diff != "" {
		if _, err := os.Stat(filename); err == nil {
			err = kubernetesRevertFromPath(filename)
			if err == nil {
				iacLogger.Warn("🧹 Detected change. Reverting...", "kind", kind, "gitFilePath", gitFilePath)
			}
		}
		return
	}

	if !utils.CONFIG.Iac.AllowPush {
		return
	}
	if kind == "" {
		UpdateResourceStatus(kind, namespace, resourceName, SyncStateSyncError, fmt.Errorf("Kind is empty for resource %s:%s/%s", kind, namespace, resourceName))
		return
	}
	yamlData, err := yaml.Marshal(dataInf)
	if err != nil {
		err = fmt.Errorf("Error marshaling to YAML: %s\n", err.Error())
		iacLogger.Error(err.Error())
		return
	}
	err = createFolderForResource(kind, namespace)
	if err != nil {
		UpdateResourceStatus(kind, namespace, resourceName, SyncStateSyncError, fmt.Errorf("Error creating folder for resource: %s", err.Error()))
		return
	}
	data, err := utils.CleanYaml(string(yamlData), utils.IacSecurityNeedsEncryption)
	if err != nil {
		UpdateResourceStatus(kind, namespace, resourceName, SyncStateSyncError, fmt.Errorf("Error cleaning YAML: %s", err.Error()))
		return
	}

	err = os.WriteFile(filename, []byte(data), 0755)
	if err != nil {
		UpdateResourceStatus(kind, namespace, resourceName, SyncStateSyncError, fmt.Errorf("Error writing file: %s", err.Error()))
		return
	}
	if utils.CONFIG.Iac.LogChanges {
		iacLogger.Debug("🧹 Detected change. Updated resource.", "kind", kind, "namespace", namespace, "resourceName", resourceName)
	}
	UpdateResourceStatus(kind, namespace, resourceName, SyncStatePendingSync, err)

	if diff != "" {
		AddChangedFile(ChangedFile{
			Author:     config.Get("MO_GIT_USER_NAME") + " <" + config.Get("MO_GIT_USER_EMAIL") + ">",
			Kind:       kind,
			Name:       resourceName,
			Path:       gitFilePath,
			Message:    fmt.Sprintf("Updated [%s] %s/%s", kind, namespace, resourceName),
			ChangeType: SyncChangeTypeModify,
		})
	} else {
		UpdateResourceStatus(kind, namespace, resourceName, SyncStateSynced, err)
	}
}

func DeleteResourceYaml(kind string, namespace string, resourceName string, objectToDelete interface{}) {
	if SetupInProcess.Load() {
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

	diff, err := createDiff(kind, namespace, resourceName, make(map[string]interface{}))
	if err != nil {
		UpdateResourceStatus(kind, namespace, resourceName, SyncStateDeleted, fmt.Errorf("Error creating diff: %s", err.Error()))
		return
	}
	if utils.CONFIG.Iac.ShowDiffInLog {
		if diff != "" {
			iacLogger.Info("iac manager diff", "diff", diff)
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
				iacLogger.Warn("🧹 Detected deletion. Reverting.", "kind", kind, "namespace", namespace, "resourceName", resourceName)
			}
		}
		return
	}

	if !utils.CONFIG.Iac.AllowPush {
		return
	}

	filename := fileNameForRaw(kind, namespace, resourceName)
	err = os.Remove(filename)
	if err != nil && !os.IsNotExist(err) {
		iacLogger.Error("Error deleting resource file", "kind", kind, "namespace", namespace, "resourceName", resourceName, "error", err)
		return
	}
	if utils.CONFIG.Iac.LogChanges {
		iacLogger.Info("Detected deletion. Removed resource. ♻️", "kind", kind, "namespace", namespace, "resourceName", resourceName)
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

func createDiff(kind string, namespace string, resourceName string, dataInf interface{}) (string, error) {
	filePath1 := fileNameForRaw(kind, namespace, resourceName)

	yamlRawData2, err := yaml.Marshal(dataInf)
	if err != nil {
		return "", fmt.Errorf("Error marshaling to YAML: %s\n", err.Error())
	}
	yamlData2Str, err := utils.CleanYaml(string(yamlRawData2), utils.IacSecurityNeedsNothing)
	if err != nil {
		return "", fmt.Errorf("Error cleaning yaml: %s\n", err.Error())
	}

	return CreateDiffFromFile(yamlData2Str, filePath1, resourceName)
}

func CreateDiffFromFile(yaml string, filePath string, resourceName string) (string, error) {
	yamlData, err := os.ReadFile(filePath)
	if err != nil {
		// this happens if the file has not been created yet
		yamlData = []byte("")
	}
	yamlDataCleaned, err := utils.CleanYaml(string(yamlData), utils.IacSecurityNeedsDecryption)
	if err != nil {
		iacLogger.Error("Error cleaning YAML", "error", err)
		return "", err
	}
	file1, err := os.CreateTemp("", "temp1*")
	if err != nil {
		iacLogger.Error("Error creating temp file", "error", err)
		return "", err
	}
	defer os.Remove(file1.Name())
	file2, err := os.CreateTemp("", "temp2*")
	if err != nil {
		iacLogger.Error("Error creating temp file", "error", err)
		return "", err
	}
	defer os.Remove(file2.Name())
	_, err = file1.WriteString(yamlDataCleaned)
	if err != nil {
		iacLogger.Error("Error writing to temp file1", "error", err)
		return "", err
	}
	_, err = file2.WriteString(yaml)
	if err != nil {
		iacLogger.Error("Error writing to temp file2", "error", err)
		return "", err
	}

	cmd := exec.Command("diff", "-u", "-N", "-u", "--label", resourceName, "--label", resourceName, file1.Name(), file2.Name())
	out, err := cmd.CombinedOutput()

	if err != nil {
		// diff returns exit code 1 if files differ
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				return string(out), nil
			} else {
				return "", fmt.Errorf("Error running diff: %s\n%s\n", err.Error(), string(out))
			}
		} else {
			return "", err
		}
	}
	return "", nil
}

func insertPATIntoURL(gitRepoURL, pat string) string {
	if pat == "" {
		return gitRepoURL
	}
	if !strings.HasPrefix(gitRepoURL, "https://") {
		return gitRepoURL // Non-HTTPS URLs are not handled here
	}
	logging.AddSecret(utils.CONFIG.Iac.RepoPat)
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
						iacLogger.Error("Error syncing changes", "error", err)
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

	folders, err := os.ReadDir(utils.CONFIG.Kubernetes.GitVaultDataPath)
	if err != nil {
		iacLogger.Error("Error reading directory", "error", err)
		return nil
	}
	for _, folder := range folders {
		if folder.IsDir() && !strings.HasPrefix(folder.Name(), ".") {
			nextFolder := fmt.Sprintf("%s/%s", utils.CONFIG.Kubernetes.GitVaultDataPath, folder.Name())
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
		err := kubernetesReplaceResource(file)
		if err != nil {
			iacLogger.Error("Error replacing resource", "error", err)
		}
	}

	return nil
}

func SyncChanges() error {
	startTime := time.Now()

	var err error
	if gitHasRemotes() {
		if !SetupInProcess.Load() {
			gitSyncLock.Lock()
			defer gitSyncLock.Unlock()
			lastCommit, updatedFiles, deletedFiles, err := pullChanges(GetLastAppliedCommit())
			// update Pulse
			if len(updatedFiles) > 0 || len(deletedFiles) > 0 {
				pulse, err := gitmanager.GeneratePulseDiagramData(utils.CONFIG.Kubernetes.GitVaultDataPath)
				if err == nil {
					SetPulseDiagramData(pulse)
				}
			}
			// skip this error
			if errors.Is(err, transport.ErrEmptyRemoteRepository) {
				err = nil
			}

			SetPullError(err)
			if err != nil {
				iacLogger.Error("Error pulling changes", "error", err)
				if err == git.ErrNonFastForwardUpdate {
					iacLogger.Warn("Non-fast-forward update detected. Deleting local repository. Changes will not be lost because they will be synced again in the next run.")
					err2 := ResetCurrentRepoData(3)
					if err2 != nil {
						iacLogger.Error("Error resetting repo data", "error", err)
					}
					return err
				}
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
				if err != nil {
					iacLogger.Warn("Error replacing resource", "error", err)
				}
			}
			deletedFiles = applyPriotityToChangesForDeletes(deletedFiles)
			for _, v := range deletedFiles {
				err = kubernetesDeleteResource(v)
				if err != nil {
					iacLogger.Warn("Error deleting resource", "error", err)
				}
			}
			SetLastSuccessfullyAppliedCommit(lastCommit)
			err = pushChanges()
			SetPushError(err)
			if err != nil {
				iacLogger.Error("Error pushing changes", "error", err)
				return err
			}
			SetSyncError(nil)
			SetSyncInfo(time.Since(startTime).Milliseconds())
		} else {
			err = fmt.Errorf("Setup in process. Skipping sync.")
		}
	} else {
		err = fmt.Errorf("No remotes found. Skipping sync.")
	}
	if err != nil {
		iacLogger.Warn(err.Error())
	}
	SetSyncError(err)
	return err
}

func fileNameForRaw(kind string, namespace string, resourceName string) string {
	name := fmt.Sprintf("%s/%s/%s/%s.yaml", utils.CONFIG.Kubernetes.GitVaultDataPath, kind, namespace, resourceName)
	if namespace == "" {
		name = fmt.Sprintf("%s/%s/%s.yaml", utils.CONFIG.Kubernetes.GitVaultDataPath, kind, resourceName)
	}
	return name
}

func GitFilePathForRaw(kind string, namespace string, resourceName string) string {
	file := fileNameForRaw(kind, namespace, resourceName)
	return strings.Replace(file, fmt.Sprintf("%s/", utils.CONFIG.Kubernetes.GitVaultDataPath), "", 1)
}

func createFolderForResource(resource string, namespace string) error {
	resourceFolder := fmt.Sprintf("%s/%s", utils.CONFIG.Kubernetes.GitVaultDataPath, resource)
	if namespace != "" {
		resourceFolder = fmt.Sprintf("%s/%s/%s", utils.CONFIG.Kubernetes.GitVaultDataPath, resource, namespace)
	}
	if _, err := os.Stat(resourceFolder); os.IsNotExist(err) {
		err := os.MkdirAll(resourceFolder, 0755)
		if err != nil {
			iacLogger.Error("Error creating folder for resource", "error", err)
			return err
		}
	}
	return nil
}

func ResetFile(filePath, commitHash string) error {
	kind, _, name := parseFileToK8sParts(filePath)

	err := gitmanager.ResetFileToCommit(utils.CONFIG.Kubernetes.GitVaultDataPath, commitHash, filePath)
	if err != nil {
		iacLogger.Error("Error resetting file", "error", err)
		return err
	}

	err = gitmanager.Commit(
		utils.CONFIG.Kubernetes.GitVaultDataPath,
		[]string{filePath},
		[]string{},
		fmt.Sprintf("Reset [%s] %s to %s.", kind, name, commitHash),
		config.Get("MO_GIT_USER_NAME"),
		config.Get("MO_GIT_USER_EMAIL"),
	)
	if err != nil {
		iacLogger.Error("Error committing reset", "error", err)
		return err
	}

	return nil
}

func gitHasRemotes() bool {
	return gitmanager.HasRemotes(utils.CONFIG.Kubernetes.GitVaultDataPath)
}

func pullChanges(lastAppliedCommit *GitActionStatus) (lastCommit *object.Commit, updatedFiles []string, deletedFiles []string, error error) {
	if !utils.CONFIG.Iac.AllowPull {
		return
	}

	// Pull changes from the remote repository
	lastCommit, err := gitmanager.Pull(utils.CONFIG.Kubernetes.GitVaultDataPath, "origin", utils.CONFIG.Iac.RepoBranch)
	if err != nil {
		return lastCommit, updatedFiles, deletedFiles, err
	}
	if lastCommit == nil {
		return lastCommit, updatedFiles, deletedFiles, fmt.Errorf("Last commit cannot be empty.")
	}

	// nothing changed
	if lastAppliedCommit != nil {
		if lastCommit.Hash.String() == lastAppliedCommit.CommitHash {
			return lastCommit, []string{}, []string{}, nil
		}
	}

	//Get the list of updated or newly added files since the last pull
	updatedFiles, err = gitmanager.GetLastUpdatedAndModifiedFiles(utils.CONFIG.Kubernetes.GitVaultDataPath)
	if err != nil {
		iacLogger.Error("Error getting updated files", "error", err)
		return
	}

	// Get the list of deleted files since the last pull
	deletedFiles, err = gitmanager.GetLastDeletedFiles(utils.CONFIG.Kubernetes.GitVaultDataPath)
	if err != nil {
		iacLogger.Error("Error getting deleted files", "error", err)
		return
	}

	iacLogger.Info("🔄 Pulled changes from the remote repository.", "modifiedFilesCount", len(updatedFiles), "deletedFilesCount", len(deletedFiles))
	iacLogger.Debug("Added/Updated Files", "count", len(updatedFiles), "files", updatedFiles)
	iacLogger.Debug("Deleted Files", "count", len(deletedFiles), "files", deletedFiles)

	// Get the list of contributors
	contributor, contributorErr := gitmanager.GetContributors(utils.CONFIG.Kubernetes.GitVaultDataPath)
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
	if IsChangedFilesEmpty() {
		return nil
	}

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
			iacLogger.Error("SyncChangeType unimplemented", "changeType", file.ChangeType)
		}
		commitMsg += file.Message + "\n"
	}
	err := gitmanager.Commit(
		utils.CONFIG.Kubernetes.GitVaultDataPath,
		updatedOrAddedFiles,
		deletedFiles,
		commitMsg,
		config.Get("MO_GIT_USER_NAME"),
		config.Get("MO_GIT_USER_EMAIL"),
	)
	if err != nil {
		return fmt.Errorf("Error running git commit: %s", err.Error())
	}

	// Push changes to the remote repository
	err = gitmanager.Push(utils.CONFIG.Kubernetes.GitVaultDataPath, "origin")
	if err != nil {
		return fmt.Errorf("Error running git push: %s", err.Error())
	}
	iacLogger.Info("🔄 Pushed changes to remote repository.", "count", ChangedFilesLen())
	iacLogger.Debug("changed files", "files", GetChangedFiles())
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
	file = strings.Replace(file, fmt.Sprintf("%s/", utils.CONFIG.Kubernetes.GitVaultDataPath), "", 1)
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
	filePath := file
	if !strings.HasPrefix(file, "/") {
		filePath = fmt.Sprintf("%s/%s", utils.CONFIG.Kubernetes.GitVaultDataPath, file)
	}

	if shouldSkipResource(filePath) {
		return nil
	}

	kind, namespace, name := parseFileToK8sParts(file)
	existingResource, err := kubernetes.GetK8sObjectFor(file, namespace != "")
	if err != nil && !apierrors.IsNotFound(err) {
		UpdateResourceStatusByFile(file, SyncStateSynced, fmt.Errorf("Error getting existing kubernetes resource %s: %s", file, err.Error()))
		return nil
	}

	// make sure secrets are encrypted
	if kind == "secrets" {
		err := EncryptUnencryptedSecrets(filePath)
		if err != nil {
			UpdateResourceStatusByFile(file, SyncStateSyncError, fmt.Errorf("Error encrypting unencrypted secrets: %s", err.Error()))
			return err
		}
	}

	diff, err := createDiff(kind, namespace, name, existingResource)
	if err != nil {
		UpdateResourceStatusByFile(file, SyncStateSyncError, fmt.Errorf("Error creating diff for %s: %s", file, err.Error()))
		return nil
	}
	if diff == "" {
		UpdateResourceStatusByFile(file, SyncStateSynced, nil)
		return nil
	}

	if kind == "secrets" {
		// load yaml file, decrypt, apply
		yamlData, err := os.ReadFile(filePath)
		if err != nil {
			iacLogger.Error("Error reading file", "filePath", filePath, "error", err)
			return err
		}
		decryptedYaml, err := utils.CleanYaml(string(yamlData), utils.IacSecurityNeedsDecryption)
		if err != nil {
			iacLogger.Error("Error decrypting YAML", "error", err)
			return err
		}
		applyCmd := fmt.Sprintf(`kubectl apply -f -<<EOF
%s
EOF`, decryptedYaml)
		err = utils.ExecuteShellCommandRealySilent(applyCmd, applyCmd)
		if err != nil {
			iacLogger.Error("Error applying secret", "error", err)
		}
	} else {
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
	}

	UpdateResourceStatusByFile(file, SyncStateSynced, nil)
	return nil
}

func EncryptUnencryptedSecrets(filePath string) error {
	if !utils.CONFIG.Iac.AllowPush {
		return nil
	}

	// encrypt if needed
	fileChanged, err := utils.EncryptSecretIfNecessary(filePath)
	if err != nil {
		return err
	}

	if fileChanged {
		// push it back to the repo
		kind, namespace, resourceName := parseFileToK8sParts(filePath)
		AddChangedFile(ChangedFile{
			Name:       resourceName,
			Kind:       kind,
			Path:       filePath,
			Message:    fmt.Sprintf("Encrypted [%s] %s/%s", kind, namespace, resourceName),
			ChangeType: SyncChangeTypeModify,
		})
	}

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
	hasIgnoredNs, nsName := isIgnoredNamespaceInFile(path)
	if hasIgnoredNs {
		iacLogger.Debug("😑 Skipping because contains ignored namespace", "namespace", *nsName, "path", path)
		return true
	}

	for _, pattern := range utils.CONFIG.Iac.IgnoredNames {
		if pattern != "" {
			match, _ := regexp.MatchString(pattern, path)
			if match {
				return true
			}
		}
	}

	return false
}
