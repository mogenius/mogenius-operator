package iacmanager

// import (
// 	"mogenius-k8s-manager/src/gitmanager"
// 	"mogenius-k8s-manager/src/logging"
// 	"mogenius-k8s-manager/src/utils"
// 	"sync"
// 	"time"

// 	"github.com/go-git/go-git/v5/plumbing/object"
// )

// type IacManagerStatus struct {
// 	SyncInfo                      IacManagerSyncInfo                 `json:"syncInfo"`
// 	LastSuccessfullyAppliedCommit *GitActionStatus                   `json:"lastSuccessfullyAppliedCommit"`
// 	IacConfiguration              IacConfiguration                   `json:"iacConfiguration"`
// 	ResourceStates                map[string]IacManagerResourceState `json:"resourceStates"`
// 	RepoPulse                     map[string]int                     `json:"repoPulse"`
// }

// type IacConfiguration struct {
// 	RepoUrl            string                    `json:"repoUrl"`
// 	RepoPat            string                    `json:"repoPat"`
// 	RepoBranch         string                    `json:"repoBranch"`
// 	SyncFrequencyInSec int                       `json:"syncFrequencyInSec"`
// 	AllowPush          bool                      `json:"allowPush"`
// 	AllowPull          bool                      `json:"allowPull"`
// 	SyncWorkloads      []utils.SyncResourceEntry `json:"syncWorkloads"`
// 	AvailableWorkloads []utils.SyncResourceEntry `json:"availableWorkloads"`
// 	ShowDiffInLog      bool                      `json:"showDiffInLog"`
// 	IgnoredNamespaces  []string                  `json:"ignoredNamespaces"`
// 	IgnoredNames       []string                  `json:"ignoredNames"`
// 	LogChanges         bool                      `json:"logChanges"`
// }

// type IacManagerSyncInfo struct {
// 	ExecutionTimeInMs           int64         `json:"executionTimeInMs"`
// 	NumberOfFiles               int           `json:"numberOfFiles"`
// 	Contributors                []Contributor `json:"contributors"`
// 	RecentlyAddedOrUpdatedFiles []string      `json:"recentlyAddedOrUpdatedFiles"`
// 	RecentlyDeletedFiles        []string      `json:"recentlyDeletedFiles"`
// 	RepoError                   string        `json:"repoError"`
// 	RemoteError                 string        `json:"remoteError"`
// 	PullError                   string        `json:"pullError"`
// 	PushError                   string        `json:"pushError"`
// 	SyncError                   string        `json:"syncError"`
// }

// type GitActionStatus struct {
// 	CommitMsg     string `json:"commitMsg"`
// 	CommitAuthor  string `json:"commitAuthor"`
// 	CommitHash    string `json:"commitHash"`
// 	CommitDate    string `json:"commitDate"`
// 	Diff          string `json:"diff,omitempty"`
// 	LastExecution string `json:"lastExecution"`
// }

// type Contributor struct {
// 	Name             string `json:"name"`
// 	Email            string `json:"email"`
// 	LastActivityTime string `json:"lastActivityTime"`
// }

// type IacManagerResourceState struct {
// 	Kind       string                      `json:"kind"`
// 	Namespace  string                      `json:"namespace"`
// 	Name       string                      `json:"name"`
// 	LastUpdate string                      `json:"lastUpdate"`
// 	Revisions  []gitmanager.CommitRevision `json:"revisions"`
// 	Author     string                      `json:"author"`
// 	Error      string                      `json:"error"`
// 	State      SyncStateEnum               `json:"state"`
// }

// type ChangedFile struct {
// 	Name       string                      `json:"name"`
// 	Kind       string                      `json:"kind"`
// 	Path       string                      `json:"path"`
// 	Author     string                      `json:"author"`
// 	Revisions  []gitmanager.CommitRevision `json:"revisions"`
// 	Message    string                      `json:"message"`
// 	Timestamp  string                      `json:"timestamp"`
// 	ChangeType SyncChangeType              `json:"changeType"`
// }

// type SyncChangeType string

// const (
// 	SyncChangeTypeUnknown SyncChangeType = "Unknown"
// 	SyncChangeTypeAdd     SyncChangeType = "Add"
// 	SyncChangeTypeDelete  SyncChangeType = "Delete"
// 	SyncChangeTypeModify  SyncChangeType = "Modify"
// )

// type SyncStateEnum string

// const (
// 	SyncStateUnknown     SyncStateEnum = "Unknown"
// 	SyncStatePendingSync SyncStateEnum = "PendingSync" // When the resource is updated in the cluster but not yet synced.
// 	SyncStateSynced      SyncStateEnum = "Synced"      // When the resource is synced with the repository.
// 	SyncStateDeleted     SyncStateEnum = "Deleted"     // When the resource is deleted from the cluster.
// 	SyncStateReverted    SyncStateEnum = "Reverted"    // When the resource is reverted because Pull=true and yaml != repo.
// 	SyncStateSyncError   SyncStateEnum = "SyncError"
// )

// type IacChangeTypeEnum string

// const (
// 	IacChangeTypeUnknown         IacChangeTypeEnum = "Unknown"
// 	IacChangeTypeRepoError       IacChangeTypeEnum = "RepoError"
// 	IacChangeTypeRemoteError     IacChangeTypeEnum = "RemoteError"
// 	IacChangeTypePullError       IacChangeTypeEnum = "PullError"
// 	IacChangeTypePushError       IacChangeTypeEnum = "PushError"
// 	IacChangeTypeSyncError       IacChangeTypeEnum = "SyncError"
// 	IacChangeTypeResourceUpdated IacChangeTypeEnum = "ResourceUpdated"
// 	IacChangeTypeLastPullUpdated IacChangeTypeEnum = "LastPullUpdated"
// 	IacChangeTypeLastPushUpdated IacChangeTypeEnum = "LastPushUpdated"
// )

// var changedFiles []ChangedFile

// var dataModel IacManagerStatus
// var dataModelMutex sync.Mutex

// func InitDataModel() {
// 	dataModel = IacManagerStatus{
// 		SyncInfo:         IacManagerSyncInfo{},
// 		IacConfiguration: iacConfigurationFromYamlConfig(),
// 		ResourceStates:   make(map[string]IacManagerResourceState),
// 		RepoPulse:        make(map[string]int),
// 	}
// 	dataModel.SyncInfo.Contributors = []Contributor{}
// 	dataModel.SyncInfo.RecentlyAddedOrUpdatedFiles = []string{}
// 	dataModel.SyncInfo.RecentlyDeletedFiles = []string{}
// 	dataModel.LastSuccessfullyAppliedCommit = nil

// 	addedFiles, err := gitmanager.GetLastUpdatedAndModifiedFiles(config.Get("MO_GIT_VAULT_DATA_PATH"))
// 	if err == nil {
// 		dataModel.SyncInfo.RecentlyAddedOrUpdatedFiles = addedFiles
// 	}
// 	deletedFiles, err := gitmanager.GetLastDeletedFiles(config.Get("MO_GIT_VAULT_DATA_PATH"))
// 	if err == nil {
// 		dataModel.SyncInfo.RecentlyDeletedFiles = deletedFiles
// 	}
// 	pulse, err := gitmanager.GeneratePulseDiagramData(config.Get("MO_GIT_VAULT_DATA_PATH"))
// 	if err == nil {
// 		dataModel.RepoPulse = pulse
// 	}
// }

// func NotifyChange(change IacChangeTypeEnum) {
// 	// fmt.Printf("Change detected %s\n", change)
// }

// // SETTERS
// func SetRepoError(err error) {
// 	updatedString := ""
// 	if err == nil {
// 		updatedString = ""
// 	} else {
// 		updatedString = err.Error()
// 	}
// 	if dataModel.SyncInfo.RepoError != updatedString {
// 		dataModel.SyncInfo.RepoError = updatedString
// 		NotifyChange(IacChangeTypeRepoError)
// 	}
// }

// func SetRemoteError(err error) {
// 	updatedString := ""
// 	if err == nil {
// 		updatedString = ""
// 	} else {
// 		updatedString = err.Error()
// 	}
// 	if dataModel.SyncInfo.RemoteError != updatedString {
// 		dataModel.SyncInfo.RemoteError = updatedString
// 		NotifyChange(IacChangeTypeRemoteError)
// 	}
// }

// func SetPullError(err error) {
// 	updatedString := ""
// 	if err == nil {
// 		updatedString = ""
// 	} else {
// 		updatedString = err.Error()
// 	}
// 	if dataModel.SyncInfo.PullError != updatedString {
// 		dataModel.SyncInfo.PullError = updatedString
// 		NotifyChange(IacChangeTypePullError)
// 	}
// }

// func SetPushError(err error) {
// 	updatedString := ""
// 	if err == nil {
// 		updatedString = ""
// 	} else {
// 		updatedString = err.Error()
// 	}
// 	if dataModel.SyncInfo.PushError != updatedString {
// 		dataModel.SyncInfo.PushError = updatedString
// 		NotifyChange(IacChangeTypePushError)
// 	}
// }

// func SetSyncError(err error) {
// 	updatedString := ""
// 	if err == nil {
// 		updatedString = ""
// 	} else {
// 		updatedString = err.Error()
// 	}
// 	if dataModel.SyncInfo.SyncError != updatedString {
// 		dataModel.SyncInfo.SyncError = updatedString
// 		NotifyChange(IacChangeTypeSyncError)
// 	}
// }

// func SetLastSuccessfullyAppliedCommit(commit *object.Commit) {
// 	if commit == nil {
// 		return
// 	}
// 	dataModelMutex.Lock()
// 	dataModel.LastSuccessfullyAppliedCommit = &GitActionStatus{
// 		CommitAuthor:  commit.Author.Name,
// 		CommitDate:    commit.Author.When.Format(time.RFC3339),
// 		CommitHash:    commit.Hash.String(),
// 		CommitMsg:     commit.Message,
// 		LastExecution: time.Now().Format(time.RFC3339),
// 	}
// 	dataModelMutex.Unlock()
// 	NotifyChange(IacChangeTypeLastPullUpdated)
// }

// func SetResourceState(key string, state IacManagerResourceState) {
// 	dataModelMutex.Lock()
// 	if state.State == SyncStateDeleted {
// 		delete(dataModel.ResourceStates, key)
// 	} else {
// 		dataModel.ResourceStates[key] = state
// 	}
// 	dataModelMutex.Unlock()
// 	NotifyChange(IacChangeTypeResourceUpdated)
// }

// func SetSyncInfo(timeInMs int64) {
// 	dataModel.SyncInfo.ExecutionTimeInMs = timeInMs

// 	addedOrUpdatedfiles, err := gitmanager.GetLastUpdatedAndModifiedFiles(config.Get("MO_GIT_VAULT_DATA_PATH"))
// 	if err == nil && len(addedOrUpdatedfiles) > 0 {
// 		dataModel.SyncInfo.RecentlyAddedOrUpdatedFiles = addedOrUpdatedfiles
// 	}
// 	deletedFiles, err := gitmanager.GetLastDeletedFiles(config.Get("MO_GIT_VAULT_DATA_PATH"))
// 	if err == nil && len(deletedFiles) > 0 {
// 		dataModel.SyncInfo.RecentlyDeletedFiles = deletedFiles
// 	}
// }

// func SetContributors(signatures []object.Signature) {
// 	contributors := []Contributor{}
// 	for _, sig := range signatures {
// 		contributors = append(contributors, Contributor{
// 			Name:             sig.Name,
// 			Email:            sig.Email,
// 			LastActivityTime: sig.When.Format(time.RFC3339),
// 		})
// 	}
// 	dataModel.SyncInfo.Contributors = contributors
// }

// func SetPulseDiagramData(pulse map[string]int) {
// 	dataModel.RepoPulse = pulse
// }

// // GETTERS
// func GetDataModel() IacManagerStatus {
// 	// allways get the current configuration state
// 	dataModel.IacConfiguration = iacConfigurationFromYamlConfig()
// 	dataModel.SyncInfo.NumberOfFiles = len(dataModel.ResourceStates)
// 	return dataModel
// }

// func GetLastAppliedCommit() *GitActionStatus {
// 	return dataModel.LastSuccessfullyAppliedCommit
// }

// func GetDataModelJson() string {
// 	return utils.PrettyPrintInterface(GetDataModel())
// }

// func GetResourceState() map[string]IacManagerResourceState {
// 	dataModelMutex.Lock()
// 	result := dataModel.ResourceStates
// 	dataModelMutex.Unlock()
// 	return result
// }

// func UpdateResourceStatus(kind string, namespace string, name string, state SyncStateEnum, errMsg error) {
// 	key := GitFilePathForRaw(kind, namespace, name)

// 	newStatus := IacManagerResourceState{
// 		Kind:      kind,
// 		Namespace: namespace,
// 		Name:      name,
// 		State:     state,
// 		Revisions: []gitmanager.CommitRevision{},
// 	}

// 	if errMsg != nil {
// 		newStatus.Error = errMsg.Error()
// 	}

// 	revisions, _ := gitmanager.ListFileRevisions(config.Get("MO_GIT_VAULT_DATA_PATH"), key, name+".yaml")
// 	for index, rev := range revisions {
// 		// latest revision
// 		if index == 0 {
// 			newStatus.LastUpdate = rev.Date
// 			newStatus.Author = rev.Author
// 		}
// 		newStatus.Revisions = append(newStatus.Revisions, rev)
// 	}

// 	SetResourceState(key, newStatus)

// 	if errMsg != nil {
// 		iacLogger.Error("failure in UpdateResourceStatus", "syncState", state, "key", key, "error", errMsg.Error())
// 	} else {
// 		iacLogger.Debug("✅ UpdateResourceStatus finished", "syncState", state, "key", key)
// 	}
// }

// func UpdateResourceStatusByFile(file string, state SyncStateEnum, err error) {
// 	kind, namespace, name := parseFileToK8sParts(file)
// 	UpdateResourceStatus(kind, namespace, name, state, err)
// }

// func PrintIacStatus() string {
// 	dataModelMutex.Lock()
// 	result := utils.PrettyPrintInterface(dataModel)
// 	dataModelMutex.Unlock()
// 	return result
// }

// // CHANGED FILES
// func AddChangedFile(file ChangedFile) {
// 	file.Author = config.Get("MO_GIT_USER_NAME") + "<" + config.Get("MO_GIT_USER_EMAIL") + ">"
// 	file.Timestamp = time.Now().Format(time.RFC3339)
// 	// skip if already exists
// 	for _, v := range changedFiles {
// 		if v.Name == file.Name && v.Kind == file.Kind {
// 			return
// 		}
// 	}
// 	revisions, _ := gitmanager.ListFileRevisions(config.Get("MO_GIT_VAULT_DATA_PATH"), file.Path, file.Name+".yaml")
// 	for index, rev := range revisions {
// 		// latest revision
// 		if index == 0 {
// 			file.Timestamp = rev.Date
// 			file.Author = rev.Author
// 		}
// 		file.Revisions = append(file.Revisions, rev)
// 	}
// 	changedFiles = append(changedFiles, file)
// }

// func GetChangedFiles() []ChangedFile {
// 	return changedFiles
// }

// func ClearChangedFiles() {
// 	changedFiles = []ChangedFile{}
// }

// func IsChangedFilesEmpty() bool {
// 	return len(changedFiles) <= 0
// }

// func ChangedFilesLen() int {
// 	return len(changedFiles)
// }

// // HELPERS

// func iacConfigurationFromYamlConfig() IacConfiguration {
// 	repoPat := ""
// 	if utils.CONFIG.Iac.RepoPat != "" {
// 		repoPat = logging.REDACTED
// 	}
// 	return IacConfiguration{
// 		RepoUrl:            utils.CONFIG.Iac.RepoUrl,
// 		RepoPat:            repoPat,
// 		RepoBranch:         utils.CONFIG.Iac.RepoBranch,
// 		SyncFrequencyInSec: utils.CONFIG.Iac.SyncFrequencyInSec,
// 		AllowPush:          utils.CONFIG.Iac.AllowPush,
// 		AllowPull:          utils.CONFIG.Iac.AllowPull,
// 		AvailableWorkloads: utils.CONFIG.Iac.AvailableWorkloads,
// 		ShowDiffInLog:      utils.CONFIG.Iac.ShowDiffInLog,
// 		IgnoredNames:       utils.CONFIG.Iac.IgnoredNames,
// 		IgnoredNamespaces:  utils.CONFIG.Iac.IgnoredNamespaces,
// 		LogChanges:         utils.CONFIG.Iac.LogChanges,
// 	}
// }
