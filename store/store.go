package store

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/interfaces"
	"mogenius-k8s-manager/shutdown"
	"reflect"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
)

var storeLogger *slog.Logger

func Setup(logManager interfaces.LogManager) {
	storeLogger = logManager.CreateLogger("store")
}

type Store struct {
	db         *badger.DB
	mu         sync.RWMutex
	indexStore *ReverseIndexStore
}

var GlobalStore *Store
var garbageCollectionTicker *time.Ticker

func Start() {
	var err error
	GlobalStore, err = NewStore()
	if err != nil {
		storeLogger.Error("failed to initialize store", "error", err)
		shutdown.SendShutdownSignalAndBlockForever(true)
		panic("unreachable")
	}

	// Run garbage collection every 5 minutes
	garbageCollectionTicker = time.NewTicker(5 * time.Minute)
	go func() {
		for range garbageCollectionTicker.C {
			storeLogger.Info("Run garbage collection DB ...")
			err := GlobalStore.RunGC()
			if err != nil {
				storeLogger.Debug("Error running GlobalStore.RunGC", "error", err)
			}
		}
	}()
}

func Defer() {
	if garbageCollectionTicker != nil {
		garbageCollectionTicker.Stop()
	}
	if GlobalStore != nil {
		GlobalStore.Close()
	}
}

func (s *Store) Close() error {
	return s.db.Close()
}

func NewStore() (*Store, error) {
	// Limit Memory Usage to
	opts := badger.DefaultOptions("").WithInMemory(true).
		WithMemTableSize(16 << 20).
		WithNumMemtables(2).
		WithNumLevelZeroTables(1).
		WithNumLevelZeroTablesStall(2).
		WithLogger(nil)
	db, err := badger.Open(opts)
	if err != nil {
		storeLogger.Error(err.Error())
		return nil, err
	}

	indexStore := NewReverseIndexStore()

	return &Store{db: db, indexStore: indexStore}, nil
}

func (s *Store) Set(value interface{}, keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return fmt.Errorf("database is not initialized")
	}

	key := CreateKey(keys...)

	s.indexStore.AddCompositeKey(key, keys...)

	keyBytes := []byte(key)

	valueBytes, err := s.serialize(value)
	if err != nil {
		return err
	}

	err = s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(keyBytes, valueBytes)
	})

	return err
}

func (s *Store) Get(key string, resultType reflect.Type) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	keyBytes := []byte(key)
	result := reflect.New(resultType).Interface()

	// var result interface{}
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(keyBytes)
		if err != nil {
			return err
		}

		valueBytes, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}

		err = s.deserialize(valueBytes, result)
		if err != nil {
			return err
		}

		return nil
	})
	return result, err
}

func (s *Store) GetByKeyParts(resultType reflect.Type, keys ...string) interface{} {
	key := CreateKey(keys...)
	value, err := s.Get(key, resultType)
	if err != nil {
		storeLogger.Error("failed to get value", "key", key, "error", err)
		return nil
	}
	return value
}

func (s *Store) GetByKeyPart(keyPart string, resultType reflect.Type) []interface{} {
	keys := s.indexStore.GetCompositeKeys(keyPart)
	values := make([]interface{}, 0, len(keys))
	for _, key := range keys {
		value, err := s.Get(key, resultType)
		if err != nil {
			storeLogger.Error("failed to get value", "key", key, "error", err)
			continue
		}
		values = append(values, value)
	}
	return values
}

func (s *Store) Delete(keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return fmt.Errorf("database is not initialized")
	}

	key := CreateKey(keys...)
	s.indexStore.DeleteCompositeKey(key, keys...)

	keyBytes := []byte(key)

	err := s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(keyBytes)
	})
	return err
}

func (s *Store) SearchByPrefix(resultType reflect.Type, parts ...string) ([]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	key := CreateKey(parts...)
	items := make([]interface{}, 0)

	// var result interface{}
	err := s.db.View(func(txn *badger.Txn) error {
		prefix := []byte(key)
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			// Create a new instance of the resultType
			result := reflect.New(resultType).Interface()

			err := item.Value(func(v []byte) error {
				return s.deserialize(v, result)
			})

			if err != nil {
				return err
			}

			items = append(items, result)
		}

		return nil
	})

	if len(items) == 0 {
		return nil, fmt.Errorf("No entry found for %s", key)
	}

	return items, err
}

func (s *Store) SearchByUUID(uuid string, result interface{}) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	// var result interface{}
	err := s.db.View(func(txn *badger.Txn) error {
		prefix := []byte(uuid)
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			err := item.Value(func(v []byte) error {
				return s.deserialize(v, result)
			})
			return err
		}

		return nil
	})

	if result == nil {
		return nil, fmt.Errorf("No entry found for %s", uuid)
	}

	return result, err
}

func (s *Store) SearchByNames(namespace string, name string, result interface{}) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	searchSuffix := fmt.Sprintf("___%s___%s", namespace, name)

	// var result interface{}
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := string(item.Key())

			if len(key) >= len(searchSuffix) && key[len(key)-len(searchSuffix):] == searchSuffix {
				err := item.Value(func(v []byte) error {
					return s.deserialize(v, result)
				})
				return err
			}
		}
		return nil
	})

	if result == nil {
		return nil, fmt.Errorf("No entry found for %s/%s", namespace, name)
	}

	return result, err
}

func (s *Store) serialize(value interface{}) ([]byte, error) {
	// use json for serialization to not lose data (pointer)
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (s *Store) deserialize(data []byte, value interface{}) error {
	return json.Unmarshal(data, value)
}

func (s *Store) RunGC() error {
	return s.db.RunValueLogGC(0.7)
}
