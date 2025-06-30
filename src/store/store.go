package store

import (
	"errors"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeyclient"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "k8s.io/api/apps/v1"
	v1batch "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
	networkingV1 "k8s.io/api/networking/v1"
)

const (
	VALKEY_RESOURCE_PREFIX = "resources"
)

var ErrNotFound = errors.New("not found")

var storeLogger *slog.Logger
var valkeyClient valkeyclient.ValkeyClient

func Setup(
	logManagerModule logging.SlogManager,
	valkey valkeyclient.ValkeyClient,
) error {
	storeLogger = logManagerModule.CreateLogger("store")
	valkeyClient = valkey
	return nil
}

func GetByKeyParts[T any](valkeyClient valkeyclient.ValkeyClient, keys ...string) (*T, error) {
	value, err := valkeyclient.GetObjectForKey[T](valkeyClient, keys...)
	if err != nil {
		return nil, fmt.Errorf("failed to get value for key %s: %w", strings.Join(keys, ":"), err)
	}
	if value == nil {
		return nil, fmt.Errorf("got nil value from GetObjectForKey %s", strings.Join(keys, ":"))
	}
	return value, nil
}

func SearchByKeyParts(valkeyClient valkeyclient.ValkeyClient, parts ...string) ([]unstructured.Unstructured, error) {
	key := CreateKey(parts...)

	items, err := valkeyclient.GetObjectsByPrefix[unstructured.Unstructured](valkeyClient, valkeyclient.ORDER_NONE, key)

	if len(items) == 0 {
		return nil, ErrNotFound

	}

	return items, err
}

func SearchByNamespaceAndName(valkeyClient valkeyclient.ValkeyClient, namespace string, name string) ([]unstructured.Unstructured, error) {
	pattern := CreateKeyPattern(nil, nil, &namespace, &name)

	items, err := valkeyclient.GetObjectsByPattern[unstructured.Unstructured](valkeyClient, pattern, []string{})

	return items, err
}

func SearchByGroupKindNameNamespace(valkeyClient valkeyclient.ValkeyClient, group string, kind string, name string, namespace *string) ([]unstructured.Unstructured, error) {
	pattern := CreateKeyPattern(&group, &kind, namespace, &name)

	items, err := valkeyclient.GetObjectsByPattern[unstructured.Unstructured](valkeyClient, pattern, []string{})

	return items, err
}

func SearchByNamespace(valkeyClient valkeyclient.ValkeyClient, namespace string, whitelist []*utils.ResourceEntry) ([]unstructured.Unstructured, error) {
	pattern := CreateKeyPattern(nil, nil, &namespace, nil)

	var searchKeys []string
	if len(whitelist) > 0 {
		for _, item := range whitelist {
			searchKey := CreateKey(item.Group, item.Kind, namespace)
			searchKeys = append(searchKeys, searchKey)
		}
	}

	items, err := valkeyclient.GetObjectsByPattern[unstructured.Unstructured](valkeyClient, pattern, searchKeys)

	return items, err
}

func DropAllResourcesFromValkey(valkeyClient valkeyclient.ValkeyClient, logger *slog.Logger) error {
	keys, err := valkeyClient.Keys(VALKEY_RESOURCE_PREFIX + ":*")
	if err != nil {
		return fmt.Errorf("failed to get keys: %v", err)
	}
	err = valkeyClient.DeleteMultiple(keys...)
	if err != nil {
		logger.Error("failed to DropAllResourcesFromValkey", "error", err)
	}
	return err
}

func DropAllPodEventsFromValkey(valkeyClient valkeyclient.ValkeyClient, logger *slog.Logger) error {
	keys, err := valkeyClient.Keys("pod-events" + ":*")
	if err != nil {
		logger.Error("failed to get keys", "error", err)
		return err
	}
	err = valkeyClient.DeleteMultiple(keys...)
	if err != nil {
		logger.Error("failed to DropAllPodEventsFromValkey", "error", err)
	}
	return err
}

func CreateKey(parts ...string) string {
	parts = append([]string{VALKEY_RESOURCE_PREFIX}, parts...)
	return strings.Join(parts, ":")
}

func CreateKeyPattern(groupVersion, kind, namespace, name *string) string {
	parts := make([]string, 5)

	parts[0] = VALKEY_RESOURCE_PREFIX

	if groupVersion != nil && *groupVersion != "" {
		parts[1] = *groupVersion
	} else {
		parts[1] = "*"
	}

	if kind != nil && *kind != "" {
		parts[2] = *kind
	} else {
		parts[2] = "*"
	}

	if namespace != nil && *namespace != "" {
		parts[3] = *namespace
	} else {
		parts[3] = "*"
	}

	if name != nil && *name != "" {
		parts[4] = *name
	} else {
		parts[4] = "*"
	}

	pattern := strings.Join(parts, ":")
	return pattern
}

func ListNetworkPolicies(valkeyClient valkeyclient.ValkeyClient, namespace string) ([]networkingV1.NetworkPolicy, error) {
	result := []networkingV1.NetworkPolicy{}

	policies, err := valkeyclient.GetObjectsByPrefix[networkingV1.NetworkPolicy](valkeyClient, valkeyclient.ORDER_NONE, VALKEY_RESOURCE_PREFIX, utils.NetworkPolicyResource.Group, "NetworkPolicy", namespace)
	if err != nil {
		return result, err
	}

	for _, ref := range policies {
		if namespace != "" && ref.Namespace != namespace {
			continue
		}

		result = append(result, ref)
	}

	return result, nil
}

func ListEvents(valkeyClient valkeyclient.ValkeyClient, namespace string) ([]coreV1.Event, error) {
	result := []coreV1.Event{}

	events, err := valkeyclient.GetObjectsByPrefix[coreV1.Event](valkeyClient, valkeyclient.ORDER_DESC, VALKEY_RESOURCE_PREFIX, "v1", "Event", namespace)
	if err != nil {
		return result, err
	}

	for _, ref := range events {
		if namespace != "" && ref.Namespace != namespace {
			continue
		}

		result = append(result, ref)
	}

	return result, nil
}

func ListPods(valkeyClient valkeyclient.ValkeyClient, parts ...string) ([]coreV1.Pod, error) {
	result := []coreV1.Pod{}

	args := append([]string{VALKEY_RESOURCE_PREFIX, utils.PodResource.Group, "Pod"}, parts...)
	pods, err := valkeyclient.GetObjectsByPrefix[coreV1.Pod](valkeyClient, valkeyclient.ORDER_NONE, args...)
	if err != nil {
		return result, err
	}

	return pods, nil
}

func ListAllNamespaces(valkeyClient valkeyclient.ValkeyClient) ([]coreV1.Namespace, error) {
	result := []coreV1.Namespace{}

	namespaces, err := valkeyclient.GetObjectsByPrefix[coreV1.Namespace](valkeyClient, valkeyclient.ORDER_NONE, VALKEY_RESOURCE_PREFIX, utils.NamespaceResource.Group, "Namespace")
	if err != nil {
		return result, err
	}

	return namespaces, nil
}

func GetNamespace(valkeyClient valkeyclient.ValkeyClient, name string, logger *slog.Logger) *coreV1.Namespace {
	namespace, err := valkeyclient.GetObjectForKey[coreV1.Namespace](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.NamespaceResource.Group, utils.NamespaceResource.Kind, "", name)
	if err != nil {
		logger.Error("failed to get namespace", "name", name, "error", err)
		return nil
	}
	return namespace
}

func GetResourceByKindAndNamespace(valkeyClient valkeyclient.ValkeyClient, groupVersion string, kind string, namespace string, logger *slog.Logger) []unstructured.Unstructured {
	var results []unstructured.Unstructured

	storeResults, err := valkeyclient.GetObjectsByPrefix[unstructured.Unstructured](valkeyClient, valkeyclient.ORDER_NONE, VALKEY_RESOURCE_PREFIX, groupVersion, kind, namespace)
	if err != nil {
		logger.Error("failed to get resources by kind and namespace", "groupVersion", groupVersion, "kind", kind, "namespace", namespace, "error", err)
		return results
	}

	for _, ref := range storeResults {
		if (namespace != "" && ref.GetNamespace() != namespace) || (kind != "" && ref.GetKind() != kind) {
			continue
		}

		results = append(results, ref)
	}
	return results
}

func GetPod(namespace string, name string) *coreV1.Pod {
	pod, err := valkeyclient.GetObjectForKey[coreV1.Pod](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.PodResource.Group, utils.PodResource.Kind, namespace, name)
	if err != nil || pod == nil {
		return nil
	}
	pod.Kind = utils.PodResource.Kind
	pod.APIVersion = utils.PodResource.Group

	return pod
}

func GetReplicaset(namespace string, name string) *v1.ReplicaSet {
	replicaSet, err := valkeyclient.GetObjectForKey[v1.ReplicaSet](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.ReplicaSetResource.Group, utils.ReplicaSetResource.Kind, namespace, name)
	if err != nil || replicaSet == nil {
		return nil
	}
	replicaSet.Kind = utils.ReplicaSetResource.Kind
	replicaSet.APIVersion = utils.ReplicaSetResource.Group

	return replicaSet
}

func GetDeployment(namespace string, name string) *v1.Deployment {
	deployment, err := valkeyclient.GetObjectForKey[v1.Deployment](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.DeploymentResource.Group, utils.DeploymentResource.Kind, namespace, name)
	if err != nil || deployment == nil {
		return nil
	}
	deployment.Kind = utils.DeploymentResource.Kind
	deployment.APIVersion = utils.DeploymentResource.Group

	return deployment
}

func GetStatefulSet(namespace string, name string) *v1.StatefulSet {
	statefulSet, err := valkeyclient.GetObjectForKey[v1.StatefulSet](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.StatefulSetResource.Group, utils.StatefulSetResource.Kind, namespace, name)
	if err != nil || statefulSet == nil {
		return nil
	}
	statefulSet.Kind = utils.StatefulSetResource.Kind
	statefulSet.APIVersion = utils.StatefulSetResource.Group

	return statefulSet
}

func GetDaemonSet(namespace string, name string) *v1.DaemonSet {
	daemonSet, err := valkeyclient.GetObjectForKey[v1.DaemonSet](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.DaemonSetResource.Group, utils.DaemonSetResource.Kind, namespace, name)
	if err != nil || daemonSet == nil {
		return nil
	}
	daemonSet.Kind = utils.DaemonSetResource.Kind
	daemonSet.APIVersion = utils.DaemonSetResource.Group

	return daemonSet
}

func GetJob(namespace string, name string) *v1batch.Job {
	job, err := valkeyclient.GetObjectForKey[v1batch.Job](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.JobResource.Group, utils.JobResource.Kind, namespace, name)
	if err != nil || job == nil {
		return nil
	}
	job.Kind = utils.JobResource.Kind
	job.APIVersion = utils.JobResource.Group

	return job
}

func GetCronJob(namespace string, name string) *v1batch.CronJob {
	cronJob, err := valkeyclient.GetObjectForKey[v1batch.CronJob](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.CronJobResource.Group, utils.CronJobResource.Kind, namespace, name)
	if err != nil || cronJob == nil {
		return nil
	}
	cronJob.Kind = utils.CronJobResource.Kind
	cronJob.APIVersion = utils.CronJobResource.Group

	return cronJob
}

func GetNode(name string) *coreV1.Node {
	node, err := valkeyclient.GetObjectForKey[coreV1.Node](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.NodeResource.Group, utils.NodeResource.Kind, "", name)
	if err != nil || node == nil {
		return nil
	}
	node.Kind = utils.NodeResource.Kind
	node.APIVersion = utils.NodeResource.Group

	return node
}
