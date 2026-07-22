package openbaomanager

import (
	"context"
	"fmt"
	"strings"

	"mogenius-operator/src/utils"

	bao "github.com/openbao/openbao/api/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// configure brings OpenBao to the desired configuration: a KV v2 engine, the
// Kubernetes auth backend, a read policy, and a role bound to the ESO
// ServiceAccount. Every step tolerates "already exists" so it is safe to run on
// each tick. The client must already carry the root token.
func (m *openBaoManager) configure(ctx context.Context, client *bao.Client, tgt target) error {
	if err := m.ensureKVMount(ctx, client, tgt.kvMount); err != nil {
		return err
	}
	if err := m.ensureKubernetesAuth(ctx, client); err != nil {
		return err
	}
	if err := m.ensureESOPolicyAndRole(ctx, client, tgt.kvMount); err != nil {
		return err
	}
	return nil
}

// ensureKVMount enables a KV v2 secrets engine at path if not already mounted.
func (m *openBaoManager) ensureKVMount(ctx context.Context, client *bao.Client, path string) error {
	mounts, err := client.Sys().ListMountsWithContext(ctx)
	if err != nil {
		return fmt.Errorf("list mounts: %w", err)
	}
	if _, ok := mounts[path+"/"]; ok {
		return nil
	}
	m.logger.Info("enabling OpenBao KV v2 engine", "path", path)
	err = client.Sys().MountWithContext(ctx, path, &bao.MountInput{
		Type:    "kv",
		Options: map[string]string{"version": "2"},
	})
	if err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("mount kv at %q: %w", path, err)
	}
	return nil
}

// ensureKubernetesAuth enables and configures the Kubernetes auth backend. In
// cluster, OpenBao uses its own ServiceAccount token as the TokenReview
// reviewer, so only kubernetes_host needs to be set.
func (m *openBaoManager) ensureKubernetesAuth(ctx context.Context, client *bao.Client) error {
	auths, err := client.Sys().ListAuthWithContext(ctx)
	if err != nil {
		return fmt.Errorf("list auth: %w", err)
	}
	if _, ok := auths[kubernetesAuthPath+"/"]; !ok {
		m.logger.Info("enabling OpenBao Kubernetes auth", "path", kubernetesAuthPath)
		err = client.Sys().EnableAuthWithOptionsWithContext(ctx, kubernetesAuthPath, &bao.EnableAuthOptions{
			Type: "kubernetes",
		})
		if err != nil && !isAlreadyExists(err) {
			return fmt.Errorf("enable kubernetes auth: %w", err)
		}
	}

	_, err = client.Logical().WriteWithContext(ctx, fmt.Sprintf("auth/%s/config", kubernetesAuthPath), map[string]any{
		"kubernetes_host": "https://kubernetes.default.svc",
	})
	if err != nil {
		return fmt.Errorf("write kubernetes auth config: %w", err)
	}
	return nil
}

// ensureESOPolicyAndRole writes the read policy for the KV mount and binds a
// role to the ESO ServiceAccount. Both writes are idempotent.
func (m *openBaoManager) ensureESOPolicyAndRole(ctx context.Context, client *bao.Client, kvMount string) error {
	if err := client.Sys().PutPolicyWithContext(ctx, esoPolicyName, esoReadPolicy(kvMount)); err != nil {
		return fmt.Errorf("put policy %q: %w", esoPolicyName, err)
	}

	_, err := client.Logical().WriteWithContext(ctx, fmt.Sprintf("auth/%s/role/%s", kubernetesAuthPath, esoRoleName), map[string]any{
		"bound_service_account_names":      esoServiceAccountName,
		"bound_service_account_namespaces": esoNamespace,
		"policies":                         esoPolicyName,
		"ttl":                              "1h",
	})
	if err != nil {
		return fmt.Errorf("write kubernetes auth role %q: %w", esoRoleName, err)
	}
	return nil
}

// esoReadPolicy renders the OpenBao policy granting read access to the KV v2
// engine mounted at kvMount.
func esoReadPolicy(kvMount string) string {
	return fmt.Sprintf(`
path "%[1]s/data/*" {
  capabilities = ["read"]
}
path "%[1]s/metadata/*" {
  capabilities = ["read", "list"]
}
`, kvMount)
}

// clusterSecretStoreObject builds the ClusterSecretStore object pointing ESO at
// OpenBao via Kubernetes auth.
func clusterSecretStoreObject(tgt target) map[string]any {
	return map[string]any{
		"apiVersion": utils.ClusterSecretStoreResource.ApiVersion,
		"kind":       utils.ClusterSecretStoreResource.Kind,
		"metadata": map[string]any{
			"name":   tgt.storeName,
			"labels": map[string]any{managedByLabelKey: managedByLabelValue},
		},
		"spec": map[string]any{
			"provider": map[string]any{
				"vault": map[string]any{
					"server":  tgt.serverURL,
					"path":    tgt.kvMount,
					"version": "v2",
					"auth": map[string]any{
						"kubernetes": map[string]any{
							"mountPath": kubernetesAuthPath,
							"role":      esoRoleName,
							"serviceAccountRef": map[string]any{
								"name":      esoServiceAccountName,
								"namespace": esoNamespace,
							},
						},
					},
				},
			},
		},
	}
}

// ensureClusterSecretStore creates or updates the ClusterSecretStore that
// points ESO at OpenBao via Kubernetes auth.
func (m *openBaoManager) ensureClusterSecretStore(ctx context.Context, tgt target) error {
	gvr := gvrFor(utils.ClusterSecretStoreResource)
	client := m.clientProvider.DynamicClient().Resource(gvr)

	desired := &unstructured.Unstructured{Object: clusterSecretStoreObject(tgt)}
	if refs := ownerReferences(tgt.ownerRef); refs != nil {
		desired.SetOwnerReferences(refs)
	}

	_, err := client.Create(ctx, desired, metav1.CreateOptions{})
	if err == nil {
		m.logger.Info("created ClusterSecretStore for OpenBao", "name", tgt.storeName)
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return err
	}

	existing, err := client.Get(ctx, tgt.storeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	desired.SetResourceVersion(existing.GetResourceVersion())
	_, err = client.Update(ctx, desired, metav1.UpdateOptions{})
	return err
}

// isAlreadyExists reports whether an OpenBao API error indicates the target
// mount/backend is already in place. OpenBao returns these as 400s with a
// descriptive body rather than a typed error.
func isAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "already in use") || strings.Contains(msg, "already exists")
}
