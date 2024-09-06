package iacmanager

import (
	"fmt"
	"mogenius-k8s-manager/utils"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"
)

type IacManagerStatus struct {
	RepoError        string                             `json:"repoError"`
	RemoteError      string                             `json:"remoteError"`
	PullError        string                             `json:"pullError"`
	PushError        string                             `json:"pushError"`
	SyncError        string                             `json:"syncError"`
	LastPull         GitActionStatus                    `json:"lastPull"`
	LastPush         GitActionStatus                    `json:"lastPush"`
	IacConfiguration interface{}                        `json:"iacConfiguration"`
	ResourceStates   map[string]IacManagerResourceState `json:"resourceStates"`
}

type GitActionStatus struct {
	CommitMsg    string `json:"CommitMsg"`
	CommitAuthor string `json:"CommitAuthor"`
	CommitHash   string `json:"CommitHash"`
	CommitDate   string `json:"CommitDate"`
}

type IacManagerResourceState struct {
	Kind       string        `json:"kind"`
	Namespace  string        `json:"namespace"`
	Name       string        `json:"name"`
	LastUpdate string        `json:"lastUpdate"`
	Diff       string        `json:"diff"`
	Error      error         `json:"error"`
	State      SyncStateEnum `json:"state"`
}

type ChangedFile struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	AuthorName  string `json:"authorName"`
	AutgorEmail string `json:"authorEmail"`
	Message     string `json:"message"`
}

type SyncStateEnum string

const (
	SyncStateUnknown     SyncStateEnum = "Unknown"
	SyncStateInitialized SyncStateEnum = "Initialized" // Initial state. When the resource is read from the cluster.
	SyncStatePendingPush SyncStateEnum = "PendingPush" // When the resource is updated in the cluster but not yet synced.
	SyncStateSynced      SyncStateEnum = "Synced"      // When the resource is synced with the repository.
	SyncStateDeleted     SyncStateEnum = "Deleted"     // When the resource is deleted from the cluster.
	SyncStateReverted    SyncStateEnum = "Reverted"    // When the resource is reverted because Pull=true and yaml != repo.
	SyncStateSyncError   SyncStateEnum = "SyncError"
)

var dataModel IacManagerStatus
var dataModelMutex sync.Mutex

func InitDataModel() {
	dataModel = IacManagerStatus{
		RepoError:        "",
		RemoteError:      "",
		PullError:        "",
		PushError:        "",
		SyncError:        "",
		LastPull:         GitActionStatus{},
		LastPush:         GitActionStatus{},
		IacConfiguration: utils.CONFIG.Iac,
		ResourceStates:   make(map[string]IacManagerResourceState),
	}
}

func NotifyChange() {
	fmt.Println("Change detected")
}

// SETTERS
func SetRepoError(err error) {
	if err == nil {
		dataModel.RepoError = ""
	} else {
		dataModel.RepoError = err.Error()
	}
	NotifyChange()
}

func SetRemoteError(err error) {
	if err == nil {
		dataModel.RemoteError = ""
	} else {
		dataModel.RemoteError = err.Error()
	}
	NotifyChange()
}

func SetPullError(err error) {
	if err == nil {
		dataModel.PullError = ""
	} else {
		dataModel.PullError = err.Error()
	}
	NotifyChange()
}

func SetPushError(err error) {
	if err == nil {
		dataModel.PushError = ""
	} else {
		dataModel.PushError = err.Error()
	}
	NotifyChange()
}

func SetSyncError(err error) {
	if err == nil {
		dataModel.SyncError = ""
	} else {
		dataModel.SyncError = err.Error()
	}
	NotifyChange()
}

func SetLastPull(commit *object.Commit) {
	dataModelMutex.Lock()
	dataModel.LastPull.CommitAuthor = commit.Author.Name
	dataModel.LastPull.CommitDate = commit.Author.When.String()
	dataModel.LastPull.CommitHash = commit.Hash.String()
	dataModel.LastPull.CommitMsg = commit.Message
	dataModelMutex.Unlock()
	NotifyChange()
}

func SetLastPush(commit *object.Commit) {
	dataModelMutex.Lock()
	dataModel.LastPush.CommitAuthor = commit.Author.Name
	dataModel.LastPush.CommitDate = commit.Author.When.String()
	dataModel.LastPush.CommitHash = commit.Hash.String()
	dataModel.LastPush.CommitMsg = commit.Message
	dataModelMutex.Unlock()
	NotifyChange()
}

func SetResourceState(key string, state IacManagerResourceState) {
	dataModelMutex.Lock()
	dataModel.ResourceStates[key] = state
	dataModelMutex.Unlock()
	NotifyChange()
}

// GETTERS
func GetDataModel() IacManagerStatus {
	// allways get the current configuration state
	dataModel.IacConfiguration = utils.CONFIG.Iac
	return dataModel
}

func GetResourceState() map[string]IacManagerResourceState {
	dataModelMutex.Lock()
	result := dataModel.ResourceStates
	dataModelMutex.Unlock()
	return result
}

func UpdateResourceStatus(kind string, namespace string, name string, diff string, state SyncStateEnum, err error) {
	dataModelMutex.Lock()

	key := fmt.Sprintf("%s/%s/%s", kind, namespace, name)

	newStatus := IacManagerResourceState{
		Kind:       kind,
		Namespace:  namespace,
		Name:       name,
		Diff:       diff,
		LastUpdate: time.Now().String(),
		State:      state,
		Error:      err,
	}

	dataModel.ResourceStates[key] = newStatus

	dataModelMutex.Unlock()

	if err != nil {
		iaclogger.Errorf("Error with %s resource (%s): %s", state, key, err.Error())
	} else {
		iaclogger.Infof("✅ %s resource '%s'.", state, key)
	}
}

func UpdateResourceStatusByFile(file string, diff string, state SyncStateEnum, err error) {
	parts := strings.Split(file, "/")
	filename := strings.Replace(parts[len(parts)-1], ".yaml", "", -1)
	partsName := strings.Split(filename, "_")
	kind := parts[0]
	namespace := ""
	name := ""
	if len(partsName) > 1 {
		namespace = partsName[0]
		name = partsName[1]
	}

	UpdateResourceStatus(kind, namespace, name, diff, state, err)
}

func PrintIacStatus() string {
	dataModelMutex.Lock()
	result := utils.PrettyPrintInterface(dataModel)
	dataModelMutex.Unlock()
	return result
}
