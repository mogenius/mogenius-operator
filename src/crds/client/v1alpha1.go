package client

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	mov1alpha1 "mogenius-k8s-manager/src/crds/v1alpha1"
)

type MogeniusV1alpha1 struct {
	restClient *rest.RESTClient
	config     *rest.Config
}

func NewMogeniusV1alpha1(config rest.Config) (*MogeniusV1alpha1, error) {
	myScheme := runtime.NewScheme()

	err := mov1alpha1.AddToScheme(myScheme)
	if err != nil {
		return nil, fmt.Errorf("failed to add to scheme: %w", err)
	}

	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.NewCodecFactory(myScheme).WithoutConversion()
	config.GroupVersion = &mov1alpha1.GroupVersion

	v1alpha1client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	self := new(MogeniusV1alpha1)
	self.restClient = v1alpha1client
	self.config = &config

	return self, nil
}

// ╭───────────────╮
// │ Client: Grant │
// ╰───────────────╯

func (self *MogeniusV1alpha1) ListGrants(namespace string) ([]mov1alpha1.Grant, error) {
	req := self.restClient.Get().Namespace(namespace).Resource("grants").VersionedParams(&metav1.ListOptions{}, metav1.ParameterCodec)

	var grantList mov1alpha1.GrantList
	err := req.Do(context.Background()).Into(&grantList)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return grantList.Items, nil
}

func (self *MogeniusV1alpha1) GetGrant(namespace string, name string) (*mov1alpha1.Grant, error) {
	result := &mov1alpha1.Grant{}
	err := self.restClient.Get().Namespace(namespace).Resource("grants").Name(name).Do(context.Background()).Into(result)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return result, nil
}

func (self *MogeniusV1alpha1) CreateGrant(namespace string, name string, spec mov1alpha1.GrantSpec) (*mov1alpha1.Grant, error) {
	res := &mov1alpha1.Grant{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Grant",
			APIVersion: "mogenius.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:         name,
			Namespace:    namespace,
			GenerateName: "grant-",
			Labels:       map[string]string{},
		},
		Spec: spec,
	}
	result := &mov1alpha1.Grant{}
	err := self.restClient.Post().Namespace(namespace).Resource("grants").Body(res).Do(context.Background()).Into(result)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return result, nil
}

func (self *MogeniusV1alpha1) UpdateGrant(namespace string, name string, spec mov1alpha1.GrantSpec) (*mov1alpha1.Grant, error) {
	res := &mov1alpha1.Grant{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Grant",
			APIVersion: "mogenius.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{},
		},
		Spec: spec,
	}
	result := &mov1alpha1.Grant{}
	err := self.restClient.Put().Namespace(namespace).Resource("grants").Name(name).Body(res).Do(context.Background()).Into(result)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return result, nil
}

// ReplaceGrant()
// DeleteGrant()

// ╭────────────────╮
// │ Client: Groups │
// ╰────────────────╯

func (self *MogeniusV1alpha1) ListGroups(namespace string) ([]mov1alpha1.Group, error) {
	req := self.restClient.Get().
		Namespace(namespace).
		Resource("groups").
		VersionedParams(&metav1.ListOptions{}, metav1.ParameterCodec)

	var groupList mov1alpha1.GroupList
	err := req.Do(context.Background()).Into(&groupList)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return groupList.Items, nil
}

func (self *MogeniusV1alpha1) GetGroup(namespace string, name string) (*mov1alpha1.Group, error) {
	result := &mov1alpha1.Group{}
	err := self.restClient.Get().Namespace(namespace).Resource("groups").Name(name).Do(context.Background()).Into(result)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return result, nil
}

func (self *MogeniusV1alpha1) CreateGroup(namespace string, name string, spec mov1alpha1.GroupSpec) (*mov1alpha1.Group, error) {
	res := &mov1alpha1.Group{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Group",
			APIVersion: "mogenius.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:         name,
			Namespace:    namespace,
			GenerateName: "group-",
			Labels:       map[string]string{},
		},
		Spec: spec,
	}
	result := &mov1alpha1.Group{}
	err := self.restClient.Post().Namespace(namespace).Resource("groups").Body(res).Do(context.Background()).Into(result)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return result, nil
}

// ╭─────────────────────╮
// │ Client: Permissions │
// ╰─────────────────────╯

func (self *MogeniusV1alpha1) ListPermissions(namespace string) ([]mov1alpha1.Permission, error) {
	req := self.restClient.Get().
		Namespace(namespace).
		Resource("permissions").
		VersionedParams(&metav1.ListOptions{}, metav1.ParameterCodec)

	var permissionList mov1alpha1.PermissionList
	err := req.Do(context.Background()).Into(&permissionList)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return permissionList.Items, nil
}

func (self *MogeniusV1alpha1) GetPermission(namespace string, name string) (*mov1alpha1.Permission, error) {
	result := &mov1alpha1.Permission{}
	err := self.restClient.Get().Namespace(namespace).Resource("permissions").Name(name).Do(context.Background()).Into(result)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return result, nil
}

func (self *MogeniusV1alpha1) CreatePermission(namespace string, name string, spec mov1alpha1.PermissionSpec) (*mov1alpha1.Permission, error) {
	res := &mov1alpha1.Permission{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Permission",
			APIVersion: "mogenius.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:         name,
			Namespace:    namespace,
			GenerateName: "permission-",
			Labels:       map[string]string{},
		},
		Spec: spec,
	}
	result := &mov1alpha1.Permission{}
	err := self.restClient.Post().Namespace(namespace).Resource("permissions").Body(res).Do(context.Background()).Into(result)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return result, nil
}

// ╭───────────────╮
// │ Client: Users │
// ╰───────────────╯

func (self *MogeniusV1alpha1) ListUsers(namespace string) ([]mov1alpha1.User, error) {
	req := self.restClient.Get().
		Namespace(namespace).
		Resource("users").
		VersionedParams(&metav1.ListOptions{}, metav1.ParameterCodec)

	var userList mov1alpha1.UserList
	err := req.Do(context.Background()).Into(&userList)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return userList.Items, nil
}

func (self *MogeniusV1alpha1) GetUser(namespace string, name string) (*mov1alpha1.User, error) {
	result := &mov1alpha1.User{}
	err := self.restClient.Get().Namespace(namespace).Resource("users").Name(name).Do(context.Background()).Into(result)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return result, nil
}

func (self *MogeniusV1alpha1) CreateUser(namespace string, name string, spec mov1alpha1.UserSpec) (*mov1alpha1.User, error) {
	res := &mov1alpha1.User{
		TypeMeta: metav1.TypeMeta{
			Kind:       "User",
			APIVersion: "mogenius.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:         name,
			Namespace:    namespace,
			GenerateName: "user-",
			Labels:       map[string]string{},
		},
		Spec: spec,
	}
	result := &mov1alpha1.User{}
	err := self.restClient.Post().Namespace(namespace).Resource("users").Body(res).Do(context.Background()).Into(result)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return result, nil
}

// ╭────────────────────╮
// │ Client: Workspaces │
// ╰────────────────────╯

func (self *MogeniusV1alpha1) ListWorkspaces(namespace string) ([]mov1alpha1.Workspace, error) {
	req := self.restClient.Get().
		Namespace(namespace).
		Resource("workspaces").
		VersionedParams(&metav1.ListOptions{}, metav1.ParameterCodec)

	var workspaceList mov1alpha1.WorkspaceList
	err := req.Do(context.Background()).Into(&workspaceList)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return workspaceList.Items, nil
}

func (self *MogeniusV1alpha1) GetWorkspace(namespace string, name string) (*mov1alpha1.Workspace, error) {
	result := &mov1alpha1.Workspace{}
	err := self.restClient.Get().Namespace(namespace).Resource("workspaces").Name(name).Do(context.Background()).Into(result)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return result, nil
}

func (self *MogeniusV1alpha1) CreateWorkspaces(namespace string, name string, spec mov1alpha1.WorkspaceSpec) (*mov1alpha1.Workspace, error) {
	res := &mov1alpha1.Workspace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Workspace",
			APIVersion: "mogenius.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:         name,
			Namespace:    namespace,
			GenerateName: "workspace-",
			Labels:       map[string]string{},
		},
		Spec: spec,
	}
	result := &mov1alpha1.Workspace{}
	err := self.restClient.Post().Namespace(namespace).Resource("workspaces").Body(res).Do(context.Background()).Into(result)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return result, nil
}
