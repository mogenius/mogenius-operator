package redisstore

// import (
// 	"log/slog"
// 	"os"
// 	"testing"
// 	"time"
// )

// func setupRedisStore(t *testing.T) RedisStore {
// 	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
// 	store := NewRedis(logger)

// 	err := store.Connect()
// 	if err != nil {
// 		t.Fatalf("could not connect to Redis: %v", err)
// 	}

// 	// Cleanup from previous runs if necessary
// 	// store.Delete("test-key")
// 	// store.Delete("test-object-key")

// 	return store
// }

// func TestSetGet(t *testing.T) {
// 	store := setupRedisStore(t)

// 	err := store.Set("test-key:test:lala0", "test-value", time.Minute)
// 	if err != nil {
// 		t.Fatalf("unexpected error setting value: %v", err)
// 	}

// 	err = store.Set("test-key:test1:lala", "test-value", time.Minute)
// 	if err != nil {
// 		t.Fatalf("unexpected error setting value: %v", err)
// 	}

// 	err = store.Set("test-key:test1:lala1", "test-value", time.Minute)
// 	if err != nil {
// 		t.Fatalf("unexpected error setting value: %v", err)
// 	}

// 	val, err := store.Get("test-key")
// 	if err != nil {
// 		t.Fatalf("unexpected error getting value: %v", err)
// 	}
// 	if val != "test-value" {
// 		t.Fatalf("expected value 'test-value', got '%s'", val)
// 	}
// }

// func TestSetGetObject(t *testing.T) {
// 	store := setupRedisStore(t)

// 	type TestObject struct {
// 		ID    int
// 		Name  string
// 		Value string
// 	}

// 	obj := TestObject{ID: 1, Name: "Test", Value: "This is a test"}

// 	err := store.SetObject("test-object-key", obj, time.Minute)
// 	if err != nil {
// 		t.Fatalf("unexpected error setting object: %v", err)
// 	}

// 	var retrievedObj TestObject
// 	err = store.GetObject("test-object-key", &retrievedObj)
// 	if err != nil {
// 		t.Fatalf("unexpected error getting object: %v", err)
// 	}
// 	if retrievedObj != obj {
// 		t.Fatalf("expected object %#v, got %#v", obj, retrievedObj)
// 	}
// }

// func TestExists(t *testing.T) {
// 	store := setupRedisStore(t)

// 	exists, err := store.Exists("test-key")
// 	if err != nil {
// 		t.Fatalf("unexpected error checking existence: %v", err)
// 	}
// 	if !exists {
// 		t.Fatalf("expected 'test-key' to exist")
// 	}

// 	exists, err = store.Exists("nonexistent-key")
// 	if err != nil {
// 		t.Fatalf("unexpected error checking existence: %v", err)
// 	}
// 	if exists {
// 		t.Fatalf("expected 'nonexistent-key' not to exist")
// 	}
// }

// func TestDelete(t *testing.T) {
// 	store := setupRedisStore(t)

// 	err := store.Set("test-key-delete", "temporary", time.Minute)
// 	if err != nil {
// 		t.Fatalf("unexpected error setting value for delete test: %v", err)
// 	}

// 	err = store.Delete("test-key-delete")
// 	if err != nil {
// 		t.Fatalf("unexpected error deleting key: %v", err)
// 	}

// 	val, err := store.Get("test-key-delete")
// 	if err != nil {
// 		t.Fatalf("unexpected error getting value after delete: %v", err)
// 	}
// 	if val != "" {
// 		t.Fatalf("expected no value after delete, got '%s'", val)
// 	}
// }

// func TestKeys(t *testing.T) {
// 	store := setupRedisStore(t)

// 	err := store.Set("test-key-1", "value1", time.Minute)
// 	if err != nil {
// 		t.Fatalf("unexpected error setting value: %v", err)
// 	}

// 	err = store.Set("test-key-2", "value2", time.Minute)
// 	if err != nil {
// 		t.Fatalf("unexpected error setting value: %v", err)
// 	}

// 	keys, err := store.Keys("test-key-*")
// 	if err != nil {
// 		t.Fatalf("unexpected error getting keys: %v", err)
// 	}
// 	if len(keys) != 2 {
// 		t.Fatalf("expected 2 keys, got %d", len(keys))
// 	}
// }
