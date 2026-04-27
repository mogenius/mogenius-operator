package reconciler

import (
	"fmt"
	"hash/fnv"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/assert"

	rbacv1 "k8s.io/api/rbac/v1"
)

const (
	labelOrg               = "app.mogenius.com"
	labelManagedByMogenius = labelOrg + "/managed-by-mogenius"
	labelRoleName          = labelOrg + "/role-name"
	labelGrantGrantee      = labelOrg + "/grant-grantee"
	labelGrantTargetType   = labelOrg + "/grant-target-type"
	labelGrantTargetName   = labelOrg + "/grant-target-name"
	labelGrantRole         = labelOrg + "/grant-role"
	labelResourceId        = labelOrg + "/resource-id"
	labelResourceType      = labelOrg + "/resource-type"
	labelResourceNamespace = labelOrg + "/resource-namespace"
)

// managedRoleBindingNameSuffix returns a deterministic FNV64a hash suffix
// encoding all fields that uniquely identify a grant+resource combination.
func managedRoleBindingNameSuffix(grant v1alpha1.Grant, resource v1alpha1.WorkspaceResourceIdentifier) string {
	h := fnv.New64a()
	h.Write([]byte(grant.Spec.Grantee))
	h.Write([]byte(grant.Spec.TargetType))
	h.Write([]byte(grant.Spec.TargetName))
	h.Write([]byte(grant.Spec.Role))
	h.Write([]byte(resource.Id))
	h.Write([]byte(resource.Type))
	h.Write([]byte(resource.Namespace))
	return fmt.Sprintf("%x", h.Sum64())
}

// managedRoleBindingLabels returns the full label set for a mogenius-managed RoleBinding.
func managedRoleBindingLabels(grant v1alpha1.Grant, resource v1alpha1.WorkspaceResourceIdentifier) map[string]string {
	return map[string]string{
		labelManagedByMogenius: "",
		labelGrantGrantee:      grant.Spec.Grantee,
		labelGrantTargetType:   grant.Spec.TargetType,
		labelGrantTargetName:   grant.Spec.TargetName,
		labelGrantRole:         grant.Spec.Role,
		labelResourceId:        resource.Id,
		labelResourceType:      resource.Type,
		labelResourceNamespace: resource.Namespace,
	}
}

// ManagedRoleBindingLabelFields holds the decoded label values from a managed RoleBinding.
type ManagedRoleBindingLabelFields struct {
	GrantGrantee      string
	GrantTargetType   string
	GrantTargetName   string
	GrantRole         string
	ResourceId        string
	ResourceType      string
	ResourceNamespace string
}

// parseManagedRoleBindingLabels decodes the managed label set from a RoleBinding's labels.
// Returns an error if any required label is missing.
func parseManagedRoleBindingLabels(labels map[string]string) (ManagedRoleBindingLabelFields, error) {
	data := ManagedRoleBindingLabelFields{}

	grantee, ok := labels[labelGrantGrantee]
	if !ok {
		return ManagedRoleBindingLabelFields{}, fmt.Errorf("missing label: grant-grantee")
	}
	data.GrantGrantee = grantee

	targetType, ok := labels[labelGrantTargetType]
	if !ok {
		return ManagedRoleBindingLabelFields{}, fmt.Errorf("missing label: grant-target-type")
	}
	data.GrantTargetType = targetType

	targetName, ok := labels[labelGrantTargetName]
	if !ok {
		return ManagedRoleBindingLabelFields{}, fmt.Errorf("missing label: grant-target-name")
	}
	data.GrantTargetName = targetName

	role, ok := labels[labelGrantRole]
	if !ok {
		return ManagedRoleBindingLabelFields{}, fmt.Errorf("missing label: grant-role")
	}
	data.GrantRole = role

	resourceId, ok := labels[labelResourceId]
	if !ok {
		return ManagedRoleBindingLabelFields{}, fmt.Errorf("missing label: resource-id")
	}
	data.ResourceId = resourceId

	resourceType, ok := labels[labelResourceType]
	if !ok {
		return ManagedRoleBindingLabelFields{}, fmt.Errorf("missing label: resource-type")
	}
	data.ResourceType = resourceType

	resourceNamespace, ok := labels[labelResourceNamespace]
	if !ok {
		return ManagedRoleBindingLabelFields{}, fmt.Errorf("missing label: resource-namespace")
	}
	data.ResourceNamespace = resourceNamespace

	return data, nil
}

// subjectsEqual returns true if a and b represent the same RBAC subject.
func subjectsEqual(a, b *rbacv1.Subject) bool {
	if a == nil && b == nil {
		return true
	}
	if (a == nil) != (b == nil) {
		return false
	}
	assert.Assert(a != nil)
	assert.Assert(b != nil)
	return a.APIGroup == b.APIGroup && a.Kind == b.Kind && a.Name == b.Name && a.Namespace == b.Namespace
}
