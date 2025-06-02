package store

import (
	"errors"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeyclient"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	coreV1 "k8s.io/api/core/v1"
	networkingV1 "k8s.io/api/networking/v1"
)

const (
	VALKEY_RESOURCE_PREFIX = "resources"
)

var ErrNotFound = errors.New("not found")

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

func SearchByNamespace(valkeyClient valkeyclient.ValkeyClient, namespace string, whitelist []*utils.SyncResourceEntry) ([]unstructured.Unstructured, error) {
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
