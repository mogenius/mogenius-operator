package store

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/dgraph-io/badger/v4"
	log "github.com/sirupsen/logrus"
	"mogenius-k8s-manager/structs"
	"reflect"
	"strings"
	"sync"
)

var storeLogger = log.WithField("component", structs.Store)
var registeredTypes = make(map[reflect.Type]struct{})
var registeredTypesMutex sync.Mutex

type Store struct {
	db         *badger.DB
	mu         sync.RWMutex
	indexStore *ReverseIndexStore
}

func NewStore() (*Store, error) {
	opts := badger.DefaultOptions("").WithInMemory(true)
	db, err := badger.Open(opts)
	if err != nil {
		log.Errorf(err.Error())
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

	key := strings.Join(keys, "___")
	log.Info("-----------------------KEY added: ", key)

	s.indexStore.AddCompositeKey(key, keys...)

	keyBytes := []byte(key)
	s.registerGobType(value)

	valueBytes, err := s.serialize(value)
	if err != nil {
		return err
	}

	err = s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(keyBytes, valueBytes)
	})
	return err
}

func (s *Store) Get(key string, result interface{}) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	keyBytes := []byte(key)

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

		err = s.deserialize(valueBytes, &result)
		if err != nil {
			return err
		}

		return nil
	})
	return result, err
}

func (s *Store) GetByKeyPart(keyPart string, result interface{}) []interface{} {
	keys := s.indexStore.GetCompositeKeys(keyPart)
	values := make([]interface{}, 0, len(keys))
	for _, key := range keys {
		value, err := s.Get(key, result)
		if err != nil {
			log.Errorf("Error getting value for key %s: %s", key, err.Error())
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

	key := strings.Join(keys, "___")
	s.indexStore.DeleteCompositeKey(key, keys...)

	keyBytes := []byte(key)

	err := s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(keyBytes)
	})
	return err
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
				var err error
				err = s.deserialize(v, &result)
				return err
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
					var err error
					err = s.deserialize(v, &result)
					return err
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
func (s *Store) registerGobType(value interface{}) {
	typ := reflect.TypeOf(value)

	if gobTypeAlreadyRegistered(typ) {
		return
	}

	gob.Register(value)
}

func gobTypeAlreadyRegistered(typ reflect.Type) bool {
	registeredTypesMutex.Lock()
	defer registeredTypesMutex.Unlock()

	_, alreadyRegistered := registeredTypes[typ]
	if !alreadyRegistered {
		registeredTypes[typ] = struct{}{}
	}
	return alreadyRegistered
}

func (s *Store) serialize(value interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(value)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *Store) deserialize(data []byte, value interface{}) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	return dec.Decode(value)
}

//	func (s *Store) serialize(data interface{}) ([]byte, error) {
//		var buf bytes.Buffer
//		enc := gob.NewEncoder(&buf)
//		err := enc.Encode(data)
//		if err != nil {
//			return nil, err
//		}
//		return buf.Bytes(), nil
//	}
//
//	func (s *Store) deserialize(data []byte, result interface{}) (interface{}, error) {
//		var buf bytes.Buffer
//		buf.Write(data)
//		dec := gob.NewDecoder(&buf)
//		// var result interface{}
//		err := dec.Decode(&result)
//		if err != nil {
//			return nil, err
//		}
//		return result, nil
//	}
func (s *Store) Close() error {
	return s.db.Close()
}

var GlobalStore *Store

func Init() {
	var err error
	GlobalStore, err = NewStore()
	if err != nil {
		storeLogger.Errorf("Error initializing store: %s", err.Error())
		storeLogger.Fatal(err.Error())
	}
}
