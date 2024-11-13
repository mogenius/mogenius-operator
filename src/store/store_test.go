package store_test

import (
	"mogenius-k8s-manager/src/store"
	"reflect"
	"testing"
)

func TestStore(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	// NEW STORE
	store, err := store.NewStore()
	if err != nil {
		t.Errorf("Error creating store: %s", err.Error())
	} else {
		t.Log("Store created ✅")
	}

	// SET
	err = store.Set("value", "key")
	if err != nil {
		t.Errorf("Error setting value: %s", err.Error())
	} else {
		t.Log("Value set ✅")
	}

	// GET
	resultType := reflect.TypeOf("value")
	value, err := store.Get("key", resultType)
	if err != nil {
		t.Errorf("Error getting value: %s", err.Error())
	} else {
		t.Logf("Value retrieved: %s ✅", value)
	}

	// GetByKeyParts
	err = store.Set("value1", "key1___key2")
	if err != nil {
		t.Error(err)
	}
	value = store.GetByKeyParts(resultType, "key1", "key2")
	if value == nil {
		t.Errorf("Error getting GetByKeyParts")
	} else {
		t.Logf("Value retrieved by GetByKeyParts: %s ✅", value)
	}

	// SearchByUUID
	data, err := store.SearchByUUID("uuid", resultType)
	if err != nil {
		t.Errorf("Error searching value: %s", err.Error())
	} else {
		t.Logf("Value searched by UUID: %s ✅", data)
	}

	// GetByKeyPart
	arrayValue := store.GetByKeyPart("key", resultType)
	if len(arrayValue) == 1 {
		t.Logf("Value retrieved by GetByKeyPart: %s ✅", value)
	} else {
		t.Errorf("Error getting GetByKeyPart")
	}

	// SearchByNames
	data, err = store.SearchByNames("namespace", "name", interface{}("value"))
	if err != nil {
		t.Errorf("Error searching value: %s", err.Error())
	} else {
		t.Logf("Value searched: %s ✅", data)
	}

	// SearchByPrefix
	data, err = store.SearchByPrefix(resultType, "key")
	if err != nil {
		t.Errorf("Error searching value: %s", err.Error())
	} else {
		t.Logf("Value searched by prefix: %s ✅", data)
	}

	// DELETE
	err = store.Delete("key")
	if err != nil {
		t.Errorf("Error deleting value: %s", err.Error())
	} else {
		t.Log("Value deleted ✅")
	}

	// CLOSE STORE
	err = store.Close()
	if err != nil {
		t.Errorf("Error closing store: %s", err.Error())
	} else {
		t.Log("Store closed ✅")
	}
}
