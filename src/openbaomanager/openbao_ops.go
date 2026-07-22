package openbaomanager

import (
	"context"
	"fmt"

	bao "github.com/openbao/openbao/api/v2"
)

// OpenBao lifecycle/seal status values reported by Status.
const (
	StatusNotFound = "not_found" // not enabled, unreachable, or uninitialized
	StatusSealed   = "sealed"    // initialized but sealed
	StatusUnsealed = "unsealed"  // initialized and unsealed (usable)
)

// KVMount describes a KV secrets engine mount.
type KVMount struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

// Status reports the current OpenBao state as one of StatusNotFound,
// StatusSealed, or StatusUnsealed. Absence of an enabled config or an
// unreachable/uninitialized instance all map to StatusNotFound.
func (m *openBaoManager) Status(ctx context.Context) (string, error) {
	cfg, _, err := m.enabledOpenBaoConfig(ctx)
	if err != nil {
		return "", err
	}
	if cfg == nil {
		return StatusNotFound, nil
	}
	tgt := resolveTarget(cfg)
	client, err := newBaoClient(tgt.serverURL)
	if err != nil {
		return "", fmt.Errorf("create OpenBao client: %w", err)
	}
	status, err := client.Sys().SealStatusWithContext(ctx)
	if err != nil {
		return StatusNotFound, nil // unreachable: pods not up yet
	}
	if !status.Initialized {
		return StatusNotFound, nil
	}
	if status.Sealed {
		return StatusSealed, nil
	}
	return StatusUnsealed, nil
}

// ListKVMounts returns the KV v2 secrets engines currently mounted in OpenBao.
func (m *openBaoManager) ListKVMounts(ctx context.Context) ([]KVMount, error) {
	client, _, err := m.authenticatedClient(ctx)
	if err != nil {
		return nil, err
	}
	mounts, err := client.Sys().ListMountsWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list mounts: %w", err)
	}
	return filterKVMounts(mounts), nil
}

// filterKVMounts keeps only KV secrets engines and normalizes their path and
// version.
func filterKVMounts(mounts map[string]*bao.MountOutput) []KVMount {
	result := make([]KVMount, 0)
	for path, mount := range mounts {
		if mount == nil || mount.Type != "kv" {
			continue
		}
		version := mount.Options["version"]
		if version == "" {
			version = "1"
		}
		result = append(result, KVMount{
			Path:    trimSlash(path),
			Version: version,
		})
	}
	return result
}

// ListSecrets returns the secret names directly under path in the KV v2 engine
// mounted at mount. Use an empty path to list the root.
func (m *openBaoManager) ListSecrets(ctx context.Context, mount, path string) ([]string, error) {
	client, _, err := m.authenticatedClient(ctx)
	if err != nil {
		return nil, err
	}
	list, err := client.KVv2(mount).List(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("list secrets at %q: %w", path, err)
	}
	if list == nil {
		return []string{}, nil
	}
	return list.Keys, nil
}

// GetSecret returns the key-value data of the secret at path in the KV v2
// engine mounted at mount.
func (m *openBaoManager) GetSecret(ctx context.Context, mount, path string) (map[string]any, error) {
	client, _, err := m.authenticatedClient(ctx)
	if err != nil {
		return nil, err
	}
	secret, err := client.KVv2(mount).Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("read secret at %q: %w", path, err)
	}
	if secret == nil {
		return nil, fmt.Errorf("secret at %q not found", path)
	}
	return secret.Data, nil
}

// PutSecret creates or updates the secret at path in the KV v2 engine mounted
// at mount, writing a new version with the given data.
func (m *openBaoManager) PutSecret(ctx context.Context, mount, path string, data map[string]any) error {
	client, _, err := m.authenticatedClient(ctx)
	if err != nil {
		return err
	}
	if _, err := client.KVv2(mount).Put(ctx, path, data); err != nil {
		return fmt.Errorf("write secret at %q: %w", path, err)
	}
	return nil
}

// DeleteSecret permanently removes the secret at path (all versions and
// metadata) from the KV v2 engine mounted at mount.
func (m *openBaoManager) DeleteSecret(ctx context.Context, mount, path string) error {
	client, _, err := m.authenticatedClient(ctx)
	if err != nil {
		return err
	}
	if err := client.KVv2(mount).DeleteMetadata(ctx, path); err != nil {
		return fmt.Errorf("delete secret at %q: %w", path, err)
	}
	return nil
}

// authenticatedClient resolves the enabled OpenBao target, builds a client, and
// authenticates it with the stored root token. It errors when OpenBao is not
// enabled or has no keys secret yet.
func (m *openBaoManager) authenticatedClient(ctx context.Context) (*bao.Client, target, error) {
	cfg, _, err := m.enabledOpenBaoConfig(ctx)
	if err != nil {
		return nil, target{}, err
	}
	if cfg == nil {
		return nil, target{}, fmt.Errorf("OpenBao is not enabled")
	}
	tgt := resolveTarget(cfg)
	client, err := newBaoClient(tgt.serverURL)
	if err != nil {
		return nil, target{}, fmt.Errorf("create OpenBao client: %w", err)
	}
	rootToken, _, err := m.readKeysSecret(ctx)
	if err != nil {
		return nil, target{}, fmt.Errorf("read keys secret: %w", err)
	}
	if rootToken == "" {
		return nil, target{}, fmt.Errorf("OpenBao is not initialized yet")
	}
	client.SetToken(rootToken)
	return client, tgt, nil
}

func trimSlash(s string) string {
	if len(s) > 0 && s[len(s)-1] == '/' {
		return s[:len(s)-1]
	}
	return s
}
