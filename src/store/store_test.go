package store_test

import (
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/store"
	"reflect"
	"testing"
)

func TestStore(t *testing.T) {
	t.Parallel()
	// NEW STORE
	store, err := store.NewStore()
	assert.AssertT(t, err == nil, "store should be created", err)
	t.Log("Store created ✅")

	// SET
	err = store.Set("value", "key")
	assert.AssertT(t, err == nil, "value should be set", err)
	t.Log("Value set ✅")

	// GET
	resultType := reflect.TypeOf("value")
	value, err := store.Get("key", resultType)
	assert.AssertT(t, err == nil, "value should be available", err)
	t.Logf("Value retrieved: %s ✅", value)

	// GetByKeyParts
	err = store.Set("value1", "key1___key2")
	assert.AssertT(t, err == nil, err)
	value = store.GetByKeyParts(resultType, "key1", "key2")
	assert.AssertT(t, value != nil, "requesting the value should return something")
	t.Logf("Value retrieved by GetByKeyParts: %s ✅", value)

	// SearchByUUID
	data, err := store.SearchByUUID("uuid", resultType)
	assert.AssertT(t, err == nil, "value should be found", err)
	t.Logf("Value searched by UUID: %s ✅", data)

	// GetByKeyPart
	arrayValue := store.GetByKeyPart("key", resultType)
	assert.AssertT(t, len(arrayValue) == 1, "GetByKeyPart should return a single value")
	t.Logf("Value retrieved by GetByKeyPart: %s ✅", value)

	// SearchByNames
	data, err = store.SearchByNames("namespace", "name", interface{}("value"))
	assert.AssertT(t, err == nil, "value should be found", err)
	t.Logf("Value searched: %s ✅", data)

	// SearchByPrefix
	data, err = store.SearchByPrefix(resultType, "key")
	assert.AssertT(t, err == nil, "value should be found", err)
	t.Logf("Value searched by prefix: %s ✅", data)

	// DELETE
	err = store.Delete("key")
	assert.AssertT(t, err == nil, "value should be deleted", err)
	t.Log("Value deleted ✅")

	// CLOSE STORE
	err = store.Close()
	assert.AssertT(t, err == nil, "store should be closed", err)
	t.Log("Store closed ✅")
}
