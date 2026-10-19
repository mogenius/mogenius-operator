package kubernetes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func newOwnedResource(kind, name string, uid types.UID, ownerUID types.UID) unstructured.Unstructured {
	u := unstructured.Unstructured{Object: map[string]any{}}
	u.SetAPIVersion("v1")
	u.SetKind(kind)
	u.SetName(name)
	u.SetNamespace("ns")
	u.SetUID(uid)
	if ownerUID != "" {
		u.SetOwnerReferences([]metav1.OwnerReference{{UID: ownerUID, Name: "owner"}})
	}
	return u
}

func TestIndexByOwnerUIDGroupsChildrenByEveryOwnerReference(t *testing.T) {
	podOwnedByA := newOwnedResource("Pod", "pod-a", "pod-a", "owner-a")
	podOwnedByB := newOwnedResource("Pod", "pod-b", "pod-b", "owner-b")
	podWithNoOwner := newOwnedResource("Pod", "pod-c", "pod-c", "")

	index := indexByOwnerUID([]unstructured.Unstructured{podOwnedByA, podOwnedByB, podWithNoOwner})

	assert.Equal(t, []unstructured.Unstructured{podOwnedByA}, index[types.UID("owner-a")])
	assert.Equal(t, []unstructured.Unstructured{podOwnedByB}, index[types.UID("owner-b")])
	assert.Len(t, index, 2)
}

// GetWorkloadStatusItems must only descend into children that actually list
// the parent as an owner, and must leave siblings/decoys out, even though the
// lookup now goes through a shared per-namespace index instead of a per-parent
// scan.
func TestGetWorkloadStatusItemsWalksOwnerIndexWithoutCrossMatching(t *testing.T) {
	deployment := newOwnedResource("Deployment", "web", "deployment-uid", "")
	deployment.Object["spec"] = map[string]any{"replicas": int64(1)}

	ownedReplicaSet := newOwnedResource("ReplicaSet", "web-rs1", "rs-owned", "deployment-uid")
	decoyReplicaSet := newOwnedResource("ReplicaSet", "web-rs2", "rs-decoy", "some-other-deployment")

	ownedPod := newOwnedResource("Pod", "web-rs1-pod", "pod-owned", "rs-owned")
	decoyPod := newOwnedResource("Pod", "web-rs2-pod", "pod-decoy", "rs-decoy")

	replicaSetsByOwnerCache := map[string]ownerIndex{
		"ns": indexByOwnerUID([]unstructured.Unstructured{ownedReplicaSet, decoyReplicaSet}),
	}
	podsByOwnerCache := map[string]ownerIndex{
		"ns": indexByOwnerUID([]unstructured.Unstructured{ownedPod, decoyPod}),
	}
	jobsByOwnerCache := map[string]ownerIndex{}

	items := GetWorkloadStatusItems(deployment, nil, false, replicaSetsByOwnerCache, jobsByOwnerCache, podsByOwnerCache)

	var uids []string
	for _, item := range items {
		uids = append(uids, item.UID)
	}
	assert.ElementsMatch(t, []string{"deployment-uid", "rs-owned", "pod-owned"}, uids)
}
