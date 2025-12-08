package core

import (
	"log/slog"
	"mogenius-operator/src/assert"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/helm"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/store"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"slices"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// api layer to be accessed through websockets, http and other exposed apis
//
//	[Layer 1: Exposed APIs]
//	+-----------------+     +----------------+
//	|  WebsocketAPI   |     |     HttpAPI     |
//	|-----------------|     |-----------------|
//	| - Parse Inputs  |     | - Parse Inputs  |
//	| - Serialize Data|     | - Serialize Data|
//	+-----------------+     +-----------------+
//	        |                     |
//	        \_____________________/
//	                  |
//	                  V
//	[Layer 2: API Interface]
//	+---------------------------------+
//	|         API Interface           |
//	|---------------------------------|
//	| - Unified High-Level API Calls  |
//	+---------------------------------+
//	                  |
//	                  V
//	[Layer 3: Services]
//	+--------------------+   +--------------------+   +--------------------+
//	|     Service 1      |   |     Service 2      |   |     Service N      |
//	|--------------------|   |--------------------|   |--------------------|
//	| - Manages Subsystem|   | - Manages Subsystem|   | - Manages Subsystem|
//	+--------------------+   +--------------------+   +--------------------+
//	                  |
//	                  V
//	[Layer 4: Packages/Modules]
//	+-------------------+   +-------------------+   +-------------------+
//	|   Package/Mod1    |   |   Package/Mod2    |   |   Package/ModN    |
//	|-------------------|   |-------------------|   |-------------------|
//	| - Low-Level Ops   |   | - Low-Level Ops   |   | - Low-Level Ops   |
//	+-------------------+   +-------------------+   +-------------------+
type Api interface {
	GetAllWorkspaces() ([]GetWorkspaceResult, error)
	GetWorkspace(name string) (*GetWorkspaceResult, error)
	CreateWorkspace(name string, spec v1alpha1.WorkspaceSpec) (string, error)
	UpdateWorkspace(name string, spec v1alpha1.WorkspaceSpec) (string, error)
	DeleteWorkspace(name string) (string, error)

	GetAllUsers(email *string) ([]v1alpha1.User, error)
	GetUser(name string) (*v1alpha1.User, error)
	CreateUser(name string, spec v1alpha1.UserSpec) (string, error)
	UpdateUser(name string, spec v1alpha1.UserSpec) (string, error)
	DeleteUser(name string) (string, error)

	GetAllGrants(targetType, targetName *string) ([]v1alpha1.Grant, error)
	GetGrant(name string) (*v1alpha1.Grant, error)
	CreateGrant(name string, spec v1alpha1.GrantSpec) (string, error)
	UpdateGrant(name string, spec v1alpha1.GrantSpec) (string, error)
	DeleteGrant(name string) (string, error)

	GetWorkspaceResources(workspaceName string, whitelist []*utils.ResourceDescriptor, blacklist []*utils.ResourceDescriptor, namespaceWhitelist []string) ([]unstructured.Unstructured, error)
	GetWorkspaceControllers(workspaceName string) ([]unstructured.Unstructured, error)
	GetWorkspacePods(workspaceName string) ([]unstructured.Unstructured, error)
	GetWorkspacePodsNames(workspaceName string) ([]string, error)
	GetWorkspaceNamespaces(workspaceName string) ([]string, error)

	Link(workspaceManager WorkspaceManager)
}

type api struct {
	workspaceManager WorkspaceManager
	logger           *slog.Logger
	valkeyClient     valkeyclient.ValkeyClient
	config           cfg.ConfigModule
}

func NewApi(logger *slog.Logger, valkeyClient valkeyclient.ValkeyClient, config cfg.ConfigModule) Api {
	self := &api{}

	self.logger = logger
	self.valkeyClient = valkeyClient
	self.config = config

	return self
}

func (self *api) Link(workspaceManager WorkspaceManager) {
	assert.Assert(workspaceManager != nil)

	self.workspaceManager = workspaceManager
}

type GetWorkspaceResult struct {
	Name              string                                 `json:"name" validate:"required"`
	CreationTimestamp v1.Time                                `json:"creationTimestamp,omitempty"`
	Resources         []v1alpha1.WorkspaceResourceIdentifier `json:"resources" validate:"required"`
}

func NewGetWorkspaceResult(name string, creationTimestamp v1.Time, resources []v1alpha1.WorkspaceResourceIdentifier) GetWorkspaceResult {
	return GetWorkspaceResult{
		Name:              name,
		CreationTimestamp: creationTimestamp,
		Resources:         resources,
	}
}

func (self *api) GetAllWorkspaces() ([]GetWorkspaceResult, error) {
	result := []GetWorkspaceResult{}

	namespace := self.config.Get("MO_OWN_NAMESPACE")
	resources, err := store.GetAllWorkspaces(namespace)
	if err != nil {
		return result, err
	}

	for _, resource := range resources {
		result = append(result, NewGetWorkspaceResult(
			resource.GetName(),
			resource.ObjectMeta.CreationTimestamp,
			resource.Spec.Resources,
		))
	}

	return result, nil
}

func (self *api) GetWorkspace(name string) (*GetWorkspaceResult, error) {
	namespace := self.config.Get("MO_OWN_NAMESPACE")
	resource, err := store.GetWorkspace(namespace, name)
	if err != nil {
		return nil, err
	}

	result := NewGetWorkspaceResult(
		resource.GetName(),
		resource.ObjectMeta.CreationTimestamp,
		resource.Spec.Resources,
	)

	return &result, nil
}

func (self *api) CreateWorkspace(name string, spec v1alpha1.WorkspaceSpec) (string, error) {
	_, err := self.workspaceManager.CreateWorkspace(name, spec)
	if err != nil {
		return "", err
	}

	return "Resource created successfully", nil
}

func (self *api) UpdateWorkspace(name string, spec v1alpha1.WorkspaceSpec) (string, error) {
	_, err := self.workspaceManager.UpdateWorkspace(name, spec)
	if err != nil {
		return "", err
	}

	return "Resource updated successfully", nil
}

func (self *api) DeleteWorkspace(name string) (string, error) {
	err := self.workspaceManager.DeleteWorkspace(name)
	if err != nil {
		return "", err
	}

	return "Resource deleted successfully", nil
}

func (self *api) GetAllUsers(email *string) ([]v1alpha1.User, error) {
	resources, err := self.workspaceManager.GetAllUsers(email)
	if err != nil {
		return []v1alpha1.User{}, err
	}

	return resources, nil
}

func (self *api) GetUser(name string) (*v1alpha1.User, error) {
	resource, err := self.workspaceManager.GetUser(name)
	if err != nil {
		return nil, err
	}

	return resource, nil
}

func (self *api) CreateUser(name string, spec v1alpha1.UserSpec) (string, error) {
	_, err := self.workspaceManager.CreateUser(name, spec)
	if err != nil {
		return "", err
	}

	return "Resource created successfully", nil
}

func (self *api) UpdateUser(name string, spec v1alpha1.UserSpec) (string, error) {
	_, err := self.workspaceManager.UpdateUser(name, spec)
	if err != nil {
		return "", err
	}

	return "Resource updated successfully", nil
}

func (self *api) DeleteUser(name string) (string, error) {
	err := self.workspaceManager.DeleteUser(name)
	if err != nil {
		return "", err
	}

	return "Resource deleted successfully", nil
}

func (self *api) GetAllGrants(targetType, targetName *string) ([]v1alpha1.Grant, error) {
	resources, err := self.workspaceManager.GetAllGrants(targetType, targetName)
	if err != nil {
		return []v1alpha1.Grant{}, err
	}

	return resources, nil
}

func (self *api) GetGrant(name string) (*v1alpha1.Grant, error) {
	resource, err := self.workspaceManager.GetGrant(name)
	if err != nil {
		return nil, err
	}

	return resource, nil
}

func (self *api) CreateGrant(name string, spec v1alpha1.GrantSpec) (string, error) {
	_, err := self.workspaceManager.CreateGrant(name, spec)
	if err != nil {
		return "", err
	}

	return "Resource created successfully", nil
}

func (self *api) UpdateGrant(name string, spec v1alpha1.GrantSpec) (string, error) {
	_, err := self.workspaceManager.UpdateGrant(name, spec)
	if err != nil {
		return "", err
	}

	return "Resource updated successfully", nil
}

func (self *api) DeleteGrant(name string) (string, error) {
	err := self.workspaceManager.DeleteGrant(name)
	if err != nil {
		return "", err
	}

	return "Resource deleted successfully", nil
}

func (self *api) GetWorkspaceResources(workspaceName string, whitelist []*utils.ResourceDescriptor, blacklist []*utils.ResourceDescriptor, namespaceWhitelist []string) ([]unstructured.Unstructured, error) {
	result := []unstructured.Unstructured{}

	// Get workspace
	namespace := self.config.Get("MO_OWN_NAMESPACE")
	workspace, err := store.GetWorkspace(namespace, workspaceName)
	if err != nil {
		return result, err
	}

	for _, v := range workspace.Spec.Resources {
		if v.Type == "namespace" {
			if len(namespaceWhitelist) > 0 {
				if !slices.Contains(namespaceWhitelist, v.Id) {
					continue
				}
			}
			nsResources, err := kubernetes.GetUnstructuredNamespaceResourceList(v.Id, whitelist, blacklist)
			if err != nil {
				return result, err
			}
			result = appendIfNotExists(result, nsResources...)
		}
		if v.Type == "helm" {
			if len(namespaceWhitelist) > 0 {
				if !slices.Contains(namespaceWhitelist, v.Namespace) {
					continue
				}
			}
			helmReq := helm.HelmReleaseGetWorkloadsRequest{
				Namespace: v.Namespace,
				Release:   v.Id,
				Whitelist: whitelist,
			}
			helmResources, err := helm.HelmReleaseGetWorkloads(self.valkeyClient, helmReq)
			if err != nil {
				return result, err
			}
			result = appendIfNotExists(result, helmResources...)
		}
	}

	return result, nil
}

func (self *api) GetWorkspaceControllers(workspaceName string) ([]unstructured.Unstructured, error) {
	whiteList := []*utils.ResourceDescriptor{
		&utils.DaemonSetResource,
		&utils.StatefulSetResource,
		&utils.DeploymentResource,
	}
	return self.GetWorkspaceResources(workspaceName, whiteList, nil, nil)
}

func (self *api) GetWorkspacePods(workspaceName string) ([]unstructured.Unstructured, error) {
	whiteList := []*utils.ResourceDescriptor{
		&utils.PodResource,
	}
	return self.GetWorkspaceResources(workspaceName, whiteList, nil, nil)
}

func (self *api) GetWorkspacePodsNames(workspaceName string) ([]string, error) {
	whiteList := []*utils.ResourceDescriptor{
		&utils.PodResource,
	}
	pods, err := self.GetWorkspaceResources(workspaceName, whiteList, nil, nil)
	if err != nil {
		return nil, err
	}

	podNames := []string{}
	for _, pod := range pods {
		podNames = append(podNames, pod.GetName())
	}

	return podNames, nil
}

func (self *api) GetWorkspaceNamespaces(workspaceName string) ([]string, error) {
	namespace := self.config.Get("MO_OWN_NAMESPACE")
	workspace, err := store.GetWorkspace(namespace, workspaceName)
	if err != nil {
		return nil, err
	}

	namespaceNames := []string{}
	for _, v := range workspace.Spec.Resources {
		if v.Type == "namespace" {
			namespaceNames = append(namespaceNames, v.Id)
		}
	}

	return namespaceNames, nil
}

func appendIfNotExists(list []unstructured.Unstructured, item ...unstructured.Unstructured) []unstructured.Unstructured {
	for _, i := range item {
		if !slices.ContainsFunc(list, func(u unstructured.Unstructured) bool {
			return u.GetName() == i.GetName() && u.GetNamespace() == i.GetNamespace()
		}) {
			list = append(list, i)
		}
	}
	return list
}
