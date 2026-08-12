package store

import (
	"io"
	"log/slog"
	"mogenius-operator/src/config"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const testResourceTTL = time.Hour

// newIndexTestStore points the package-level client at an in-memory server and
// returns it, so a test can inspect and tamper with the raw keyspace.
func newIndexTestStore(t *testing.T) *miniredis.Miniredis {
	t.Helper()

	mr := miniredis.RunT(t)

	cfg := config.NewConfig()
	for key, value := range map[string]string{
		"MO_VALKEY_ADDR":                 mr.Addr(),
		"MO_VALKEY_USERNAME":             "",
		"MO_VALKEY_PASSWORD":             "",
		"MO_STATS_RETENTION_MAX_ENTRIES": "",
		"MO_STATS_RETENTION_HOURS":       "",
	} {
		cfg.Declare(config.ConfigDeclaration{Key: key, DefaultValue: &value})
	}

	client := valkeyclient.NewValkeyClient(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	require.NoError(t, client.Connect())

	previous := valkeyClient
	valkeyClient = client
	t.Cleanup(func() {
		client.Close()
		valkeyClient = previous
	})

	return mr
}

func writeResource(t *testing.T, descriptor utils.ResourceDescriptor, namespace, name string) {
	t.Helper()

	obj := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": name, "namespace": namespace},
	}}
	require.NoError(t, SetResourceWithIndex(
		valkeyClient, descriptor.ApiVersion, descriptor.Kind, namespace, name, obj, testResourceTTL))
}

func nodeNames(t *testing.T) []string {
	t.Helper()

	names := make([]string, 0)
	for _, node := range GetNodes() {
		names = append(names, node.Name)
	}
	return names
}

func TestGetNodesReturnsEveryNode(t *testing.T) {
	newIndexTestStore(t)

	for _, name := range []string{"node-a", "node-b", "node-c", "node-d", "node-e", "node-f", "node-g"} {
		writeResource(t, utils.NodeResource, "", name)
	}

	assert.ElementsMatch(t,
		[]string{"node-a", "node-b", "node-c", "node-d", "node-e", "node-f", "node-g"},
		nodeNames(t))
}

func TestGetNodesReadsTheIndexNotTheKeyspace(t *testing.T) {
	mr := newIndexTestStore(t)

	writeResource(t, utils.NodeResource, "", "node-a")

	// A primary key that never made it into the index is invisible to the list
	// helpers: the index, not the keyspace, is the enumeration source. The
	// watcher writes both in one pipeline and rewrites on every resync, so a
	// dropped index entry heals within one resync window.
	require.NoError(t, mr.Set(CreateResourceKey(
		utils.NodeResource.ApiVersion, utils.NodeResource.Kind, "", "node-orphan"), "{}"))

	assert.Equal(t, []string{"node-a"}, nodeNames(t))
}

func TestDeletedResourceLeavesTheList(t *testing.T) {
	newIndexTestStore(t)

	writeResource(t, utils.NodeResource, "", "node-a")
	writeResource(t, utils.NodeResource, "", "node-b")

	require.NoError(t, DeleteResourceWithIndex(
		valkeyClient, utils.NodeResource.ApiVersion, utils.NodeResource.Kind, "", "node-a", nil))

	assert.Equal(t, []string{"node-b"}, nodeNames(t))
}

func TestExpiredPrimaryKeyIsSkipped(t *testing.T) {
	mr := newIndexTestStore(t)

	writeResource(t, utils.NodeResource, "", "node-a")
	writeResource(t, utils.NodeResource, "", "node-b")

	// TTL expiry of the primary key ahead of its index member: the member is
	// still there, the object is gone, and the read must not fail over it.
	mr.Del(CreateResourceKey(utils.NodeResource.ApiVersion, utils.NodeResource.Kind, "", "node-a"))

	assert.Equal(t, []string{"node-b"}, nodeNames(t))
}

func TestGetPodsScopesToNamespace(t *testing.T) {
	newIndexTestStore(t)

	writeResource(t, utils.PodResource, "team-a", "web")
	writeResource(t, utils.PodResource, "team-a", "worker")
	writeResource(t, utils.PodResource, "team-b", "web")

	scoped := GetPods("team-a")
	names := make([]string, 0, len(scoped))
	for _, pod := range scoped {
		assert.Equal(t, "team-a", pod.Namespace)
		names = append(names, pod.Name)
	}
	assert.ElementsMatch(t, []string{"web", "worker"}, names)

	assert.Len(t, GetPods("*"), 3)
	assert.Empty(t, GetPods("team-c"))
}

func TestEmptyNamespaceFilterMeansEveryNamespace(t *testing.T) {
	newIndexTestStore(t)

	writeResource(t, utils.PodResource, "team-a", "web")
	writeResource(t, utils.PodResource, "team-b", "web")

	// The key patterns this replaced mapped an empty segment to "*", and
	// callers still pass "" for "any namespace".
	assert.Len(t, GetPods(""), 2)

	items, err := GetResourcesByNamespaceAndKinds(valkeyClient, "", []utils.ResourceDescriptor{utils.PodResource})
	require.NoError(t, err)
	assert.Len(t, items, 2)
}

func TestGetSecretsFiltersByName(t *testing.T) {
	newIndexTestStore(t)

	writeResource(t, utils.SecretResource, "team-a", "tls-cert")
	writeResource(t, utils.SecretResource, "team-a", "tls-key")
	writeResource(t, utils.SecretResource, "team-a", "registry")

	assert.Len(t, GetSecrets("team-a", "*"), 3)
	assert.Len(t, GetSecrets("team-a", "tls-*"), 2)

	exact := GetSecrets("team-a", "registry")
	require.Len(t, exact, 1)
	assert.Equal(t, "registry", exact[0].Name)
}

// GetResourceByKindAndNamespace is the lookup helm's release-workload matching
// and the workload-status prefetch both rely on to find every ReplicaSet in a
// namespace.
func TestGetResourceByKindAndNamespaceReturnsEveryResource(t *testing.T) {
	newIndexTestStore(t)

	writeResource(t, utils.ReplicaSetResource, "team-a", "web-1")
	writeResource(t, utils.ReplicaSetResource, "team-a", "web-2")
	writeResource(t, utils.ReplicaSetResource, "team-b", "web-3")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	scoped := GetResourceByKindAndNamespace(
		valkeyClient, utils.ReplicaSetResource.ApiVersion, utils.ReplicaSetResource.Kind, "team-a", logger)

	names := make([]string, 0, len(scoped))
	for _, item := range scoped {
		names = append(names, item.GetName())
	}
	assert.ElementsMatch(t, []string{"web-1", "web-2"}, names)

	assert.Empty(t, GetResourceByKindAndNamespace(
		valkeyClient, utils.ReplicaSetResource.ApiVersion, utils.ReplicaSetResource.Kind, "team-c", logger))
}

func TestSearchByGroupKindNameNamespace(t *testing.T) {
	newIndexTestStore(t)

	writeResource(t, utils.PodResource, "team-a", "web")
	writeResource(t, utils.PodResource, "team-b", "web")
	writeResource(t, utils.PodResource, "team-a", "worker")

	scoped := "team-a"
	items, err := SearchByGroupKindNameNamespace(
		valkeyClient, utils.PodResource.ApiVersion, utils.PodResource.Kind, "web", &scoped)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "team-a", items[0].GetNamespace())

	// nil namespace searches every namespace the kind lives in.
	items, err = SearchByGroupKindNameNamespace(
		valkeyClient, utils.PodResource.ApiVersion, utils.PodResource.Kind, "web", nil)
	require.NoError(t, err)
	assert.Len(t, items, 2)

	items, err = SearchByGroupKindNameNamespace(
		valkeyClient, utils.PodResource.ApiVersion, utils.PodResource.Kind, "absent", nil)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestGetResourcesByNamespaceAndKindsFiltersKinds(t *testing.T) {
	newIndexTestStore(t)

	writeResource(t, utils.PodResource, "team-a", "web")
	writeResource(t, utils.SecretResource, "team-a", "tls-cert")
	writeResource(t, utils.PodResource, "team-b", "web")

	items, err := GetResourcesByNamespaceAndKinds(valkeyClient, "team-a", []utils.ResourceDescriptor{utils.PodResource})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "web", items[0].GetName())
	assert.Equal(t, "Pod", items[0].GetKind())
}
