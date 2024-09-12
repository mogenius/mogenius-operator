package iacmanager

import (
	"fmt"
	"mogenius-k8s-manager/gitmanager"
	"mogenius-k8s-manager/utils"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"
)

type IacManagerStatus struct {
	SyncInfo                      IacManagerSyncInfo                 `json:"syncInfo"`
	CommitHistory                 []GitActionStatus                  `json:"commitHistory"`
	LastSuccessfullyAppliedCommit GitActionStatus                    `json:"lastSuccessfullyAppliedCommit"`
	IacConfiguration              interface{}                        `json:"iacConfiguration"`
	ResourceStates                map[string]IacManagerResourceState `json:"resourceStates"`
}

type IacManagerSyncInfo struct {
	ExecutionTimeInMs           int64         `json:"executionTimeInMs"`
	NumberOfFiles               int           `json:"numberOfFiles"`
	Contributors                []Contributor `json:"contributors"`
	RecentlyAddedOrUpdatedFiles []string      `json:"recentlyAddedOrUpdatedFiles"`
	RecentlyDeletedFiles        []string      `json:"recentlyDeletedFiles"`
	RepoError                   string        `json:"repoError"`
	RemoteError                 string        `json:"remoteError"`
	PullError                   string        `json:"pullError"`
	PushError                   string        `json:"pushError"`
	SyncError                   string        `json:"syncError"`
}

type GitActionStatus struct {
	CommitMsg     string `json:"commitMsg"`
	CommitAuthor  string `json:"commitAuthor"`
	CommitHash    string `json:"commitHash"`
	CommitDate    string `json:"commitDate"`
	Diff          string `json:"diff,omitempty"`
	LastExecution string `json:"lastExecution"`
}

type Contributor struct {
	Name             string `json:"name"`
	Email            string `json:"email"`
	LastActivityTime string `json:"lastActivityTime"`
}

type IacManagerResourceState struct {
	Kind       string        `json:"kind"`
	Namespace  string        `json:"namespace"`
	Name       string        `json:"name"`
	LastUpdate string        `json:"lastUpdate"`
	Diff       string        `json:"diff"`
	Author     string        `json:"author"`
	Error      string        `json:"error"`
	State      SyncStateEnum `json:"state"`
}

type ChangedFile struct {
	Name       string         `json:"name"`
	Kind       string         `json:"kind"`
	Path       string         `json:"path"`
	Author     string         `json:"author"`
	Diff       string         `json:"Diff"`
	Message    string         `json:"message"`
	Timestamp  string         `json:"timestamp"`
	ChangeType SyncChangeType `json:"changeType"`
}

type SyncChangeType string

const (
	SyncChangeTypeUnknown SyncChangeType = "Unknown"
	SyncChangeTypeAdd     SyncChangeType = "Add"
	SyncChangeTypeDelete  SyncChangeType = "Delete"
	SyncChangeTypeModify  SyncChangeType = "Modify"
)

type SyncStateEnum string

const (
	SyncStateUnknown     SyncStateEnum = "Unknown"
	SyncStateInitialized SyncStateEnum = "Initialized" // Initial state. When the resource is read from the cluster.
	SyncStatePendingSync SyncStateEnum = "PendingSync" // When the resource is updated in the cluster but not yet synced.
	SyncStateSynced      SyncStateEnum = "Synced"      // When the resource is synced with the repository.
	SyncStateDeleted     SyncStateEnum = "Deleted"     // When the resource is deleted from the cluster.
	SyncStateReverted    SyncStateEnum = "Reverted"    // When the resource is reverted because Pull=true and yaml != repo.
	SyncStateSyncError   SyncStateEnum = "SyncError"
)

type IacChangeTypeEnum string

const (
	IacChangeTypeUnknown         IacChangeTypeEnum = "Unknown"
	IacChangeTypeRepoError       IacChangeTypeEnum = "RepoError"
	IacChangeTypeRemoteError     IacChangeTypeEnum = "RemoteError"
	IacChangeTypePullError       IacChangeTypeEnum = "PullError"
	IacChangeTypePushError       IacChangeTypeEnum = "PushError"
	IacChangeTypeSyncError       IacChangeTypeEnum = "SyncError"
	IacChangeTypeResourceUpdated IacChangeTypeEnum = "ResourceUpdated"
	IacChangeTypeLastPullUpdated IacChangeTypeEnum = "LastPullUpdated"
	IacChangeTypeLastPushUpdated IacChangeTypeEnum = "LastPushUpdated"
)

var changedFiles []ChangedFile

var dataModel IacManagerStatus
var dataModelMutex sync.Mutex

func InitDataModel() {
	dataModel = IacManagerStatus{
		SyncInfo:         IacManagerSyncInfo{},
		CommitHistory:    []GitActionStatus{},
		IacConfiguration: utils.CONFIG.Iac,
		ResourceStates:   make(map[string]IacManagerResourceState),
	}
	path := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	addedFiles, err := gitmanager.GetLastUpdatedAndModifiedFiles(path)
	if err == nil {
		dataModel.SyncInfo.RecentlyAddedOrUpdatedFiles = addedFiles
	}
	deletedFiles, err := gitmanager.GetLastDeletedFiles(path)
	if err == nil {
		dataModel.SyncInfo.RecentlyDeletedFiles = deletedFiles
	}
}

func NotifyChange(change IacChangeTypeEnum) {
	fmt.Printf("Change detected %s\n", change)
}

// SETTERS
func SetRepoError(err error) {
	updatedString := ""
	if err == nil {
		updatedString = ""
	} else {
		updatedString = err.Error()
	}
	if dataModel.SyncInfo.RepoError != updatedString {
		dataModel.SyncInfo.RepoError = updatedString
		NotifyChange(IacChangeTypeRepoError)
	}
}

func SetRemoteError(err error) {
	updatedString := ""
	if err == nil {
		updatedString = ""
	} else {
		updatedString = err.Error()
	}
	if dataModel.SyncInfo.RemoteError != updatedString {
		dataModel.SyncInfo.RemoteError = updatedString
		NotifyChange(IacChangeTypeRemoteError)
	}
}

func SetPullError(err error) {
	updatedString := ""
	if err == nil {
		updatedString = ""
	} else {
		updatedString = err.Error()
	}
	if dataModel.SyncInfo.PullError != updatedString {
		dataModel.SyncInfo.PullError = updatedString
		NotifyChange(IacChangeTypePullError)
	}
}

func SetPushError(err error) {
	updatedString := ""
	if err == nil {
		updatedString = ""
	} else {
		updatedString = err.Error()
	}
	if dataModel.SyncInfo.PushError != updatedString {
		dataModel.SyncInfo.PushError = updatedString
		NotifyChange(IacChangeTypePushError)
	}
}

func SetSyncError(err error) {
	updatedString := ""
	if err == nil {
		updatedString = ""
	} else {
		updatedString = err.Error()
	}
	if dataModel.SyncInfo.SyncError != updatedString {
		dataModel.SyncInfo.SyncError = updatedString
		NotifyChange(IacChangeTypeSyncError)
	}
}

func SetCommitHistory(commits []*object.Commit) {
	dataModelMutex.Lock()
	objects := []GitActionStatus{}
	for _, commit := range commits {
		objects = append(objects, GitActionStatus{
			CommitAuthor:  commit.Author.Name,
			CommitDate:    commit.Author.When.Format(time.RFC3339),
			CommitHash:    commit.Hash.String(),
			CommitMsg:     commit.Message,
			LastExecution: time.Now().Format(time.RFC3339),
		})
	}
	dataModel.CommitHistory = objects
	dataModelMutex.Unlock()
	NotifyChange(IacChangeTypeLastPullUpdated)
}

func SetLastSuccessfullyAppliedCommit(commit *object.Commit) {
	if commit == nil {
		return
	}
	dataModelMutex.Lock()
	dataModel.LastSuccessfullyAppliedCommit = GitActionStatus{
		CommitAuthor:  commit.Author.Name,
		CommitDate:    commit.Author.When.Format(time.RFC3339),
		CommitHash:    commit.Hash.String(),
		CommitMsg:     commit.Message,
		LastExecution: time.Now().Format(time.RFC3339),
	}
	dataModelMutex.Unlock()
	NotifyChange(IacChangeTypeLastPullUpdated)
}

func SetResourceState(key string, state IacManagerResourceState) {
	dataModelMutex.Lock()
	if state.State == SyncStateDeleted {
		delete(dataModel.ResourceStates, key)
	} else {
		dataModel.ResourceStates[key] = state
	}
	dataModelMutex.Unlock()
	NotifyChange(IacChangeTypeResourceUpdated)
}

func SetSyncInfo(timeInMs int64) {
	dataModel.SyncInfo.ExecutionTimeInMs = timeInMs

	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	addedOrUpdatedfiles, err := gitmanager.GetLastUpdatedAndModifiedFiles(folder)
	if err == nil && len(addedOrUpdatedfiles) > 0 {
		dataModel.SyncInfo.RecentlyAddedOrUpdatedFiles = addedOrUpdatedfiles
	}
	deletedFiles, err := gitmanager.GetLastDeletedFiles(folder)
	if err == nil && len(deletedFiles) > 0 {
		dataModel.SyncInfo.RecentlyDeletedFiles = deletedFiles
	}
}

func SetContributors(signatures []object.Signature) {
	contributors := []Contributor{}
	for _, sig := range signatures {
		contributors = append(contributors, Contributor{
			Name:             sig.Name,
			Email:            sig.Email,
			LastActivityTime: sig.When.Format(time.RFC3339),
		})
	}
	dataModel.SyncInfo.Contributors = contributors
}

// GETTERS
func GetDataModel() IacManagerStatus {
	// allways get the current configuration state
	dataModel.IacConfiguration = utils.CONFIG.Iac
	dataModel.SyncInfo.NumberOfFiles = len(dataModel.ResourceStates)
	return dataModel
}

func GetLastAppliedCommit() GitActionStatus {
	return dataModel.LastSuccessfullyAppliedCommit
}

func GetDataModelJson() string {
	return utils.PrettyPrintInterface(GetDataModel())
}

func GetResourceState() map[string]IacManagerResourceState {
	dataModelMutex.Lock()
	result := dataModel.ResourceStates
	dataModelMutex.Unlock()
	return result
}

func UpdateResourceStatus(kind string, namespace string, name string, state SyncStateEnum, errMsg error) {
	key := fmt.Sprintf("%s/%s/%s", kind, namespace, name)

	newStatus := IacManagerResourceState{
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
		State:     state,
	}

	if errMsg != nil {
		newStatus.Error = errMsg.Error()
	}

	gitPath := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	filePath := GitFilePathForRaw(kind, namespace, name)
	diff, updateTime, author, err := gitmanager.LastDiff(gitPath, filePath)
	if err == nil {
		newStatus.Diff = diff
		newStatus.LastUpdate = updateTime.Format(time.RFC3339)
		newStatus.Author = author
	}

	SetResourceState(key, newStatus)

	if err != nil {
		iaclogger.Errorf("Error with %s resource (%s): %s", state, key, err.Error())
	} else {
		iaclogger.Infof("✅ %s resource '%s'.", state, key)
	}
}

func UpdateResourceStatusByFile(file string, state SyncStateEnum, err error) {
	kind, namespace, name := parseFileToK8sParts(file)
	UpdateResourceStatus(kind, namespace, name, state, err)
}

func PrintIacStatus() string {
	dataModelMutex.Lock()
	result := utils.PrettyPrintInterface(dataModel)
	dataModelMutex.Unlock()
	return result
}

// CHANGED FILES
func AddChangedFile(file ChangedFile) {
	file.Author = utils.CONFIG.Git.GitUserName + "<" + utils.CONFIG.Git.GitUserEmail + ">"
	file.Timestamp = time.Now().Format(time.RFC3339)
	// skip if already exists
	for _, v := range changedFiles {
		if v.Name == file.Name && v.Kind == file.Kind {
			return
		}
	}
	gitPath := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	diff, lastUpdate, author, err := gitmanager.LastDiff(gitPath, file.Path)
	if err == nil {
		file.Diff = diff
		file.Timestamp = lastUpdate.Format(time.RFC3339)
		file.Author = author
	}
	changedFiles = append(changedFiles, file)
}

func GetChangedFiles() []ChangedFile {
	return changedFiles
}

func ClearChangedFiles() {
	changedFiles = []ChangedFile{}
}

func ChangedFilesEmpty() bool {
	return len(changedFiles) <= 0
}

func ChangedFilesLen() int {
	return len(changedFiles)
}
