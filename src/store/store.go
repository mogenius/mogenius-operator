package store

import (
	"errors"
	"log/slog"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeystore"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	VALKEY_RESOURCE_PREFIX = "resources"
)

var storeLogger *slog.Logger
var valkeyStore valkeystore.ValkeyStore

func Setup(logManagerModule logging.SlogManager, storeModule valkeystore.ValkeyStore) {
	storeLogger = logManagerModule.CreateLogger("store")
	valkeyStore = storeModule
}

var ErrNotFound = errors.New("not found")

func GetByKeyParts[T any](keys ...string) *T {
	value, err := valkeystore.GetObjectForKey[T](valkeyStore, keys...)
	if err != nil {
		storeLogger.Warn("failed to get value", "key", strings.Join(keys, ":"), "error", err)
		return nil
	}
	return value
}

func SearchByKeyParts(parts ...string) ([]unstructured.Unstructured, error) {
	key := CreateKey(parts...)

	items, err := valkeystore.GetObjectsByPrefix[unstructured.Unstructured](valkeyStore, valkeystore.ORDER_NONE, key)

	if len(items) == 0 {
		return nil, ErrNotFound

	}

	return items, err
}

func SearchByNamespaceAndName(namespace string, name string) ([]unstructured.Unstructured, error) {
	pattern := CreateKeyPattern(nil, nil, &namespace, &name)

	items, err := valkeystore.GetObjectsByPattern[unstructured.Unstructured](valkeyStore, pattern, []string{})

	return items, err
}

func SearchByGroupKindNameNamespace(group string, kind string, name string, namespace *string) ([]unstructured.Unstructured, error) {
	pattern := CreateKeyPattern(&group, &kind, namespace, &name)

	items, err := valkeystore.GetObjectsByPattern[unstructured.Unstructured](valkeyStore, pattern, []string{})

	return items, err
}

func SearchByNamespace(namespace string, whitelist []*utils.SyncResourceEntry) ([]unstructured.Unstructured, error) {
	pattern := CreateKeyPattern(nil, nil, &namespace, nil)

	var searchKeys []string
	if len(whitelist) > 0 {
		for _, item := range whitelist {
			searchKey := CreateKey(item.Group, item.Kind, namespace)
			searchKeys = append(searchKeys, searchKey)
		}
	}

	items, err := valkeystore.GetObjectsByPattern[unstructured.Unstructured](valkeyStore, pattern, searchKeys)

	return items, err
}

func DropAllResourcesFromValkey() error {
	keys, err := valkeyStore.Keys(VALKEY_RESOURCE_PREFIX + ":*")
	if err != nil {
		storeLogger.Error("failed to get keys", "error", err)
		return err
	}
	for _, v := range keys {
		err = valkeyStore.Delete(v)
		if err != nil {
			storeLogger.Error("failed to delete key", "key", v, "error", err)
		}
	}
	return err
}

func DropAllPodEventsFromValkey() error {
	keys, err := valkeyStore.Keys("pod-events" + ":*")
	if err != nil {
		storeLogger.Error("failed to get keys", "error", err)
		return err
	}
	for _, v := range keys {
		err = valkeyStore.Delete(v)
		if err != nil {
			storeLogger.Error("failed to delete key", "key", v, "error", err)
		}
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
