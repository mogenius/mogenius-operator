package core

import (
	rbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
)

type WorkspaceManager interface {
	GetExisting()    // TODO: just a draft, this needs args
	CreateOrUpdate() // TODO: just a draft, this needs args
	Delete()         // TODO: just a draft, this needs args
}

type workspaceManager struct{}

func NewWorkspaceManager() WorkspaceManager {
	wm := &workspaceManager{}

	f := rbacv1.RbacV1Client{}
	f.ClusterRoleBindings()
	return wm
}

func (self *workspaceManager) GetExisting() {}

func (self *workspaceManager) CreateOrUpdate() {}

func (self *workspaceManager) Delete() {}
