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

var storeLogger *slog.Logger

func Setup(logManagerModule logging.LogManagerModule) {
	storeLogger = logManagerModule.CreateLogger("store")
}

var ErrNotFound = errors.New("not found")

func GetByKeyParts(keys ...string) interface{} {
	key := CreateKey(keys...)

	value, err := redisstore.Global.GetObject(key)
	if err != nil {
		storeLogger.Warn("failed to get value", "key", key, "error", err)
		return nil
	}
	return value
}

func SearchByKeyParts(parts ...string) ([]unstructured.Unstructured, error) {
	key := CreateKey(parts...)

	items, err := redisstore.GetObjectsByPrefix[unstructured.Unstructured](redisstore.GetGlobalCtx(), redisstore.GetGlobalRedisClient(), redisstore.ORDER_NONE, key)

	if len(items) == 0 {
		return nil, ErrNotFound

	}

	return items, err
}

func SearchByNamespaceAndName(namespace string, name string) ([]unstructured.Unstructured, error) {
	pattern := CreateKeyPattern(nil, nil, &namespace, &name)

	items, err := redisstore.GetObjectsByPattern[unstructured.Unstructured](redisstore.GetGlobalCtx(), redisstore.GetGlobalRedisClient(), pattern, []string{})

	return items, err
}

func SearchByGroupKindNameNamespace(group string, kind string, name string, namespace *string) ([]unstructured.Unstructured, error) {
	pattern := CreateKeyPattern(&group, &kind, namespace, &name)

	items, err := redisstore.GetObjectsByPattern[unstructured.Unstructured](redisstore.GetGlobalCtx(), redisstore.GetGlobalRedisClient(), pattern, []string{})

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

	items, err := redisstore.GetObjectsByPattern[unstructured.Unstructured](redisstore.GetGlobalCtx(), redisstore.GetGlobalRedisClient(), pattern, searchKeys)

	return items, err
}

func CreateKey(parts ...string) string {
	return strings.Join(parts, ":")
}

func CreateKeyPattern(groupVersion, kind, namespace, name *string) string {
	parts := make([]string, 4)

	if groupVersion != nil && *groupVersion != "" {
		parts[0] = *groupVersion
	} else {
		parts[0] = "*"
	}

	if kind != nil && *kind != "" {
		parts[1] = *kind
	} else {
		parts[1] = "*"
	}

	if namespace != nil && *namespace != "" {
		parts[2] = *namespace
	} else {
		parts[2] = "*"
	}

	if name != nil && *name != "" {
		parts[3] = *name
	} else {
		parts[3] = "*"
	}

	pattern := strings.Join(parts, ":")
	return pattern
}
