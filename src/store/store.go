package store

import (
	"errors"
	"log/slog"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/redisstore"
	"mogenius-k8s-manager/src/utils"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	REDIS_KEY_PREFIX = "resources"
)

var storeLogger *slog.Logger
var redisStore redisstore.RedisStore

func Setup(logManagerModule logging.LogManagerModule, storeModule redisstore.RedisStore) {
	storeLogger = logManagerModule.CreateLogger("store")
	redisStore = storeModule
}

var ErrNotFound = errors.New("not found")

func GetByKeyParts(keys ...string) interface{} {
	key := CreateKey(keys...)

	value, err := redisStore.GetObject(key)
	if err != nil {
		storeLogger.Warn("failed to get value", "key", key, "error", err)
		return nil
	}
	return value
}

func SearchByKeyParts(parts ...string) ([]unstructured.Unstructured, error) {
	key := CreateKey(parts...)

	items, err := redisstore.GetObjectsByPrefix[unstructured.Unstructured](redisStore, redisstore.ORDER_NONE, key)

	if len(items) == 0 {
		return nil, ErrNotFound

	}

	return items, err
}

func SearchByNamespaceAndName(namespace string, name string) ([]unstructured.Unstructured, error) {
	pattern := CreateKeyPattern(nil, nil, &namespace, &name)

	items, err := redisstore.GetObjectsByPattern[unstructured.Unstructured](redisStore, pattern, []string{})

	return items, err
}

func SearchByGroupKindNameNamespace(group string, kind string, name string, namespace *string) ([]unstructured.Unstructured, error) {
	pattern := CreateKeyPattern(&group, &kind, namespace, &name)

	items, err := redisstore.GetObjectsByPattern[unstructured.Unstructured](redisStore, pattern, []string{})

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

	items, err := redisstore.GetObjectsByPattern[unstructured.Unstructured](redisStore, pattern, searchKeys)

	return items, err
}

func DropAllResourcesFromRedis() error {
	keys, err := redisStore.Keys(REDIS_KEY_PREFIX + ":*")
	if err != nil {
		storeLogger.Error("failed to get keys", "error", err)
		return err
	}
	for _, v := range keys {
		err = redisStore.Delete(v)
		if err != nil {
			storeLogger.Error("failed to delete key", "key", v, "error", err)
		}
	}
	return err
}

func DropAllPodEventsFromRedis() error {
	keys, err := redisStore.Keys("pod-events" + ":*")
	if err != nil {
		storeLogger.Error("failed to get keys", "error", err)
		return err
	}
	for _, v := range keys {
		err = redisStore.Delete(v)
		if err != nil {
			storeLogger.Error("failed to delete key", "key", v, "error", err)
		}
	}
	return err
}

func CreateKey(parts ...string) string {
	parts = append([]string{REDIS_KEY_PREFIX}, parts...)
	return strings.Join(parts, ":")
}

func CreateKeyPattern(groupVersion, kind, namespace, name *string) string {
	parts := make([]string, 5)

	parts[0] = REDIS_KEY_PREFIX

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
