package client

import (
	"context"
	"fmt"
	mov1alpha1 "mogenius-operator/src/crds/v1alpha1"

	json "github.com/json-iterator/go"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type MogeniusV1alpha1 struct {
	restClient *rest.RESTClient
	config     *rest.Config
}

func NewMogeniusV1alpha1(config rest.Config) (*MogeniusV1alpha1, error) {
	config.APIPath = "/apis"
	config.ContentConfig = rest.ContentConfig{
		GroupVersion:         &mov1alpha1.GroupVersion,
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
	}

	v1alpha1client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	self := &MogeniusV1alpha1{
		restClient: v1alpha1client,
		config:     &config,
	}

	return self, nil
}

// ╭────────────────╮
// │ Client: Grants │
// ╰────────────────╯

func (self *MogeniusV1alpha1) ListGrants(namespace string) ([]mov1alpha1.Grant, error) {
	req := self.restClient.Get().
		Namespace(namespace).
		Resource("grants").
		VersionedParams(&metav1.ListOptions{}, metav1.ParameterCodec)

	var permissionList mov1alpha1.GrantList
	err := req.Do(context.Background()).Into(&permissionList)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return permissionList.Items, nil
}

func (self *MogeniusV1alpha1) GetGrant(namespace string, name string) (*mov1alpha1.Grant, error) {
	result := &mov1alpha1.Grant{}
	err := self.restClient.Get().Namespace(namespace).Resource("grants").Name(name).Do(context.Background()).Into(result)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}
	result.TypeMeta = metav1.TypeMeta{
		Kind:       "Grant",
		APIVersion: "mogenius.com/v1alpha1",
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
			Name:      name,
			Namespace: namespace,
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

func (self *MogeniusV1alpha1) ReplaceGrant(namespace string, name string, spec mov1alpha1.GrantSpec) (*mov1alpha1.Grant, error) {
	res, err := self.GetGrant(namespace, name)
	if err != nil {
		return nil, err
	}
	res.Spec = spec

	result := &mov1alpha1.Grant{}
	err = self.restClient.Put().Namespace(namespace).Resource("grants").Name(name).Body(res).Do(context.Background()).Into(result)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return result, nil
}

func (self *MogeniusV1alpha1) UpdateGrant(namespace string, name string, spec mov1alpha1.GrantSpec) (*mov1alpha1.Grant, error) {
	patchBytes, err := json.Marshal(&mov1alpha1.Grant{
		Spec: spec,
	})
	if err != nil {
		return nil, err
	}

	result := &mov1alpha1.Grant{}
	err = self.restClient.Patch(types.MergePatchType).Namespace(namespace).Resource("grants").Name(name).Body(patchBytes).Do(context.Background()).Into(result)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return result, nil
}

func (self *MogeniusV1alpha1) DeleteGrant(namespace string, name string) error {
	err := self.restClient.Delete().Namespace(namespace).Resource("grants").Name(name).Do(context.Background()).Error()
	if err != nil {
		return fmt.Errorf("RESTClient: %w", err)
	}

	return nil
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
	result.TypeMeta = metav1.TypeMeta{
		Kind:       "User",
		APIVersion: "mogenius.com/v1alpha1",
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
			Name:      name,
			Namespace: namespace,
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
func (self *MogeniusV1alpha1) ReplaceUser(namespace string, name string, spec mov1alpha1.UserSpec) (*mov1alpha1.User, error) {
	res, err := self.GetUser(namespace, name)
	if err != nil {
		return nil, err
	}
	res.Spec = spec

	result := &mov1alpha1.User{}
	err = self.restClient.Put().Namespace(namespace).Resource("users").Name(name).Body(res).Do(context.Background()).Into(result)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return result, nil
}

func (self *MogeniusV1alpha1) UpdateUser(namespace string, name string, spec mov1alpha1.UserSpec) (*mov1alpha1.User, error) {
	patchBytes, err := json.Marshal(&mov1alpha1.User{
		Spec: spec,
	})
	if err != nil {
		return nil, err
	}

	result := &mov1alpha1.User{}
	err = self.restClient.Patch(types.MergePatchType).Namespace(namespace).Resource("users").Name(name).Body(patchBytes).Do(context.Background()).Into(result)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return result, nil
}

func (self *MogeniusV1alpha1) DeleteUser(namespace string, name string) error {
	err := self.restClient.Delete().Namespace(namespace).Resource("users").Name(name).Do(context.Background()).Error()
	if err != nil {
		return fmt.Errorf("RESTClient: %w", err)
	}

	return nil
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
	result.TypeMeta = metav1.TypeMeta{
		Kind:       "Workspace",
		APIVersion: "mogenius.com/v1alpha1",
	}

	return result, nil
}

func (self *MogeniusV1alpha1) CreateWorkspace(namespace string, name string, spec mov1alpha1.WorkspaceSpec) (*mov1alpha1.Workspace, error) {
	res := &mov1alpha1.Workspace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Workspace",
			APIVersion: "mogenius.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
func (self *MogeniusV1alpha1) ReplaceWorkspace(namespace string, name string, spec mov1alpha1.WorkspaceSpec) (*mov1alpha1.Workspace, error) {
	res, err := self.GetWorkspace(namespace, name)
	if err != nil {
		return nil, err
	}
	res.Spec = spec

	result := &mov1alpha1.Workspace{}
	err = self.restClient.Put().Namespace(namespace).Resource("workspaces").Name(name).Body(res).Do(context.Background()).Into(result)
	if err != nil {
		return nil, fmt.Errorf("RESTClient: %w", err)
	}

	return result, nil
}

func (self *MogeniusV1alpha1) UpdateWorkspace(namespace string, name string, spec mov1alpha1.WorkspaceSpec) (*mov1alpha1.Workspace, error) {
	patchBytes, err := json.Marshal(&mov1alpha1.Workspace{
		Spec: spec,
	})
	if err != nil {
		return nil, err
	}

	if len(spec.Resources) > 0 {
		result := &mov1alpha1.Workspace{}
		err = self.restClient.Patch(types.MergePatchType).Namespace(namespace).Resource("workspaces").Name(name).Body(patchBytes).Do(context.Background()).Into(result)
		if err != nil {
			return nil, fmt.Errorf("RESTClient: %w", err)
		}
		return result, nil
	} else {
		wrksp, err := self.ReplaceWorkspace(namespace, name, spec)
		return wrksp, err
	}
}

func (self *MogeniusV1alpha1) DeleteWorkspace(namespace string, name string) error {
	err := self.restClient.Delete().Namespace(namespace).Resource("workspaces").Name(name).Do(context.Background()).Error()
	if err != nil {
		return fmt.Errorf("RESTClient: %w", err)
	}

	return nil
}
