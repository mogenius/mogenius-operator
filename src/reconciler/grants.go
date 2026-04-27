package reconciler

import (
	"context"
	"errors"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/store"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

func (d *reconcilerModule) reconcileGrants(ctx context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	var grant v1alpha1.Grant
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &grant); err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("failed to parse Grant: %w", err)}}
	}

	if op == deleteOperation {
		return d.deleteGrantBindings(ctx, grant)
	}
	return d.reconcileGrantInternal(ctx, grant)
}

// reconcileGrantInternal computes required RoleBindings for a single Grant and
// reconciles the cluster state to match: creates missing bindings, deletes
// superfluous ones.
func (d *reconcilerModule) reconcileGrantInternal(ctx context.Context, grant v1alpha1.Grant) []ReconcileResult {
	namespace := d.config.Get("MO_OWN_NAMESPACE")
	k8sClient := d.clientProvider.K8sClientSet()

	// 1. Look up the Grantee user.
	user, err := store.GetUser(namespace, grant.Spec.Grantee)
	if err != nil || user == nil {
		return []ReconcileResult{{Err: fmt.Errorf("user not found: %s", grant.Spec.Grantee), IsWarning: true}}
	}

	// 2. No subject → user has no cluster identity; nothing to bind.
	if user.Spec.Subject == nil {
		return nil
	}

	// 3. Resolve the ClusterRole referenced by the grant's role name.
	clusterRole, err := d.findClusterRoleByRolename(ctx, grant.Spec.Role)
	if err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("clusterrole not found for role %q: %w", grant.Spec.Role, err), IsWarning: true}}
	}

	// 4. Look up the target workspace.
	workspace, err := store.GetWorkspace(namespace, grant.Spec.TargetName)
	if err != nil || workspace == nil {
		return []ReconcileResult{{Err: fmt.Errorf("workspace not found: %s", grant.Spec.TargetName), IsWarning: true}}
	}

	// 5. Build the set of required RoleBindings — one per workspace resource whose
	//    namespace exists in the cluster.
	var required []rbacv1.RoleBinding
	for _, resource := range workspace.Spec.Resources {
		targetNS := ""
		switch resource.Type {
		case "namespace":
			targetNS = resource.Id
		case "helm":
			targetNS = resource.Namespace
		default:
			continue
		}
		if targetNS == "" {
			continue
		}
		// Only create a binding if the target namespace actually exists.
		if _, err := k8sClient.CoreV1().Namespaces().Get(ctx, targetNS, metav1.GetOptions{}); err != nil {
			continue
		}
		required = append(required, rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mogenius-rb-" + managedRoleBindingNameSuffix(grant, resource),
				Namespace: targetNS,
				Labels:    managedRoleBindingLabels(grant, resource),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     clusterRole.GetName(),
			},
			Subjects: []rbacv1.Subject{*user.Spec.Subject},
		})
	}

	// 6. List all existing mogenius-managed RoleBindings for this specific grant
	//    across all namespaces (workspace namespaces vary).
	existing, err := d.listManagedRoleBindingsForGrant(grant)
	if err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("failed to list managed rolebindings: %w", err)}}
	}

	// 7 & 8. Sync: create missing, delete superfluous.
	return d.syncRoleBindings(ctx, required, existing)
}

// deleteGrantBindings removes all mogenius-managed RoleBindings that belong to
// the given (now-deleted) Grant.
func (d *reconcilerModule) deleteGrantBindings(ctx context.Context, grant v1alpha1.Grant) []ReconcileResult {
	k8sClient := d.clientProvider.K8sClientSet()

	existing, err := d.listManagedRoleBindingsForGrant(grant)
	if err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("failed to list managed rolebindings for deleted grant: %w", err)}}
	}

	var results []ReconcileResult
	for _, rb := range existing {
		if err := k8sClient.RbacV1().RoleBindings(rb.Namespace).Delete(ctx, rb.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			results = append(results, ReconcileResult{Err: fmt.Errorf("failed to delete rolebinding %s/%s: %w", rb.Namespace, rb.Name, err)})
		}
	}
	return results
}

// listManagedRoleBindingsForGrant returns all mogenius-managed RoleBindings for the
// given grant by querying the Valkey store and filtering in-memory by label values.
func (d *reconcilerModule) listManagedRoleBindingsForGrant(grant v1alpha1.Grant) ([]rbacv1.RoleBinding, error) {
	items, err := store.SearchResourceByKeyParts(d.valkeyClient, "rbac.authorization.k8s.io/v1", "RoleBinding")
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("failed to search rolebindings in store: %w", err)
	}

	var result []rbacv1.RoleBinding
	for _, item := range items {
		lbls := item.GetLabels()
		if _, ok := lbls[labelManagedByMogenius]; !ok {
			continue
		}
		if lbls[labelGrantGrantee] != grant.Spec.Grantee {
			continue
		}
		if lbls[labelGrantTargetName] != grant.Spec.TargetName {
			continue
		}
		if lbls[labelGrantRole] != grant.Spec.Role {
			continue
		}
		var rb rbacv1.RoleBinding
		if convErr := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &rb); convErr != nil {
			d.logger.Warn("failed to convert rolebinding from store", "name", item.GetName(), "error", convErr)
			continue
		}
		result = append(result, rb)
	}
	return result, nil
}

// syncRoleBindings reconciles the set of managed RoleBindings: creates/updates
// missing or changed bindings and deletes superfluous ones.
func (d *reconcilerModule) syncRoleBindings(ctx context.Context, required []rbacv1.RoleBinding, existing []rbacv1.RoleBinding) []ReconcileResult {
	k8sClient := d.clientProvider.K8sClientSet()
	var results []ReconcileResult

	// Build a map of existing bindings by "namespace/name".
	existingByKey := make(map[string]rbacv1.RoleBinding, len(existing))
	for i := range existing {
		rb := &existing[i]
		existingByKey[rb.Namespace+"/"+rb.Name] = *rb
	}

	// Build a set of required keys for deletion check.
	requiredKeys := make(map[string]struct{}, len(required))
	for i := range required {
		rb := &required[i]
		requiredKeys[rb.Namespace+"/"+rb.Name] = struct{}{}
	}

	// Create or update required bindings that are missing or have changed subjects.
	for i := range required {
		rb := required[i]
		key := rb.Namespace + "/" + rb.Name
		if len(rb.Subjects) != 1 {
			continue
		}
		if erb, found := existingByKey[key]; found {
			if len(erb.Subjects) == 1 && subjectsEqual(&erb.Subjects[0], &rb.Subjects[0]) {
				continue // already correct
			}
			// Subject changed — update.
			rb.ResourceVersion = erb.ResourceVersion
			if _, err := k8sClient.RbacV1().RoleBindings(rb.Namespace).Update(ctx, &rb, metav1.UpdateOptions{}); err != nil {
				results = append(results, ReconcileResult{Err: fmt.Errorf("failed to update rolebinding %s: %w", key, err)})
			}
		} else {
			// Create new binding; fall back to update on race condition.
			if _, err := k8sClient.RbacV1().RoleBindings(rb.Namespace).Create(ctx, &rb, metav1.CreateOptions{}); err != nil {
				if apierrors.IsAlreadyExists(err) {
					current, getErr := k8sClient.RbacV1().RoleBindings(rb.Namespace).Get(ctx, rb.Name, metav1.GetOptions{})
					if getErr == nil {
						rb.ResourceVersion = current.ResourceVersion
						_, err = k8sClient.RbacV1().RoleBindings(rb.Namespace).Update(ctx, &rb, metav1.UpdateOptions{})
					}
				}
				if err != nil {
					results = append(results, ReconcileResult{Err: fmt.Errorf("failed to create rolebinding %s: %w", key, err)})
				}
			}
		}
	}

	// Delete superfluous bindings (exist but not required).
	for key, erb := range existingByKey {
		if _, found := requiredKeys[key]; !found {
			if err := k8sClient.RbacV1().RoleBindings(erb.Namespace).Delete(ctx, erb.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				results = append(results, ReconcileResult{Err: fmt.Errorf("failed to delete superfluous rolebinding %s: %w", key, err)})
			}
		}
	}

	return results
}

// findClusterRoleByRolename finds the ClusterRole matching rolename by the
// app.mogenius.com/role-name label first, then falls back to an exact name match.
func (d *reconcilerModule) findClusterRoleByRolename(ctx context.Context, rolename string) (*rbacv1.ClusterRole, error) {
	k8sClient := d.clientProvider.K8sClientSet()

	crList, err := k8sClient.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{labelRoleName: rolename}.String(),
	})
	if err == nil && len(crList.Items) > 0 {
		return &crList.Items[0], nil
	}

	cr, err := k8sClient.RbacV1().ClusterRoles().Get(ctx, rolename, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("clusterrole %q not found by name: %w", rolename, err)
	}
	return cr, nil
}
