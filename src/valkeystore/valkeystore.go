package valkeystore

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/utils"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/go-redis/redis/v8"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type ValkeyStore interface {
	Connect() error
	Set(value interface{}, expiration time.Duration, keys ...string) error
	SetObject(value interface{}, expiration time.Duration, keys ...string) error
	Get(keys ...string) (string, error)
	GetObject(keys ...string) (interface{}, error)

	AddToBucket(maxSize int64, value interface{}, bucketKey ...string) error
	ListFromBucket(start int64, stop int64, bucketKey ...string) ([]string, error)
	LastNEntryFromBucketWithType(number int64, bucketKey ...string) ([]string, error)

	Delete(keys ...string) error
	Keys(pattern string) ([]string, error)
	Exists(keys ...string) (bool, error)

	GetRedisClient() *redis.Client
	GetContext() context.Context
	GetLogger() *slog.Logger
}

type SortOrder int

const (
	ORDER_NONE SortOrder = 0
	ORDER_ASC  SortOrder = 1
	ORDER_DESC SortOrder = 2
)

type valkeyStore struct {
	logger *slog.Logger
	config config.ConfigModule

	ctx context.Context
	// internal redis client used to connect to a valkey instance
	redisClient *redis.Client
}

func NewValkeyStore(logger *slog.Logger, configModule config.ConfigModule) ValkeyStore {
	self := &valkeyStore{}
	self.ctx = context.Background()
	self.logger = logger
	self.config = configModule
	self.redisClient = redis.NewClient(&redis.Options{})

	return self
}

func (self *valkeyStore) Connect() error {
	self.logger.Info("Connecting to valkey")

	valkeyHost := self.config.Get("MO_VALKEY_ADDR")
	valkeyHost, valkeyPort, err := net.SplitHostPort(valkeyHost)
	assert.Assert(err == nil, err)
	assert.Assert(valkeyHost != "")
	assert.Assert(valkeyPort != "")
	valkeyAddr := valkeyHost + ":" + valkeyPort
	valkeyPwd := self.config.Get("MO_VALKEY_PASSWORD")

	self.redisClient = redis.NewClient(&redis.Options{
		Addr:       valkeyAddr,
		Password:   valkeyPwd,
		DB:         0,
		MaxRetries: 0,
	})

	_, err = self.redisClient.Ping(self.ctx).Result()
	if err != nil {
		self.logger.Info("connection to Redis failed", "addr", valkeyAddr, "password", valkeyPwd, "error", err)
		return fmt.Errorf("could not connect to Redis: %v", err)
	}

	self.logger.Info("Connected to valkey", "addr", valkeyAddr)

	return nil
}

func (self *valkeyStore) GetRedisClient() *redis.Client {
	return self.redisClient
}

func (self *valkeyStore) GetContext() context.Context {
	return self.ctx
}

func (self *valkeyStore) GetLogger() *slog.Logger {
	return self.logger
}

func (self *valkeyStore) Set(value interface{}, expiration time.Duration, keys ...string) error {
	key := createKey(keys...)

	err := self.redisClient.Set(self.ctx, key, value, expiration).Err()
	if err != nil {
		self.logger.Error("Error setting value in Redis", "key", key, "error", err)
		return err
	}
	return nil
}

func (self *valkeyStore) SetObject(value interface{}, expiration time.Duration, keys ...string) error {
	key := createKey(keys...)

	objStr, err := json.Marshal(value)
	if err != nil {
		self.logger.Error("Error marshalling object for Redis", "key", key, "error", err)
		return err
	}
	return self.Set(objStr, expiration, key)
}

func (self *valkeyStore) Get(keys ...string) (string, error) {
	key := createKey(keys...)

	val, err := self.redisClient.Get(self.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			self.logger.Info("Key does not exist", "key", key)
			return "", nil
		}
		self.logger.Error("Error getting value from Redis", "key", key, "error", err)
		return "", err
	}
	return val, nil
}

func (self *valkeyStore) GetObject(keys ...string) (interface{}, error) {
	key := createKey(keys...)
	var result interface{}
	val, err := self.Get(key)
	if err != nil {
		return result, err
	}
	// Correct usage of Unmarshal
	err = json.Unmarshal([]byte(val), &result)
	if err != nil {
		self.logger.Error("Error unmarshalling value from Redis", "key", key, "error", err)
		return result, err
	}
	return result, nil
}

func (self *valkeyStore) AddToBucket(maxSize int64, value interface{}, bucketKey ...string) error {
	key := createKey(bucketKey...)
	// Add the new elements to the end of the list
	err := self.redisClient.RPush(self.ctx, key, utils.PrintJson(value)).Err()
	if err != nil {
		return err
	}

	// Trim the list to keep only the last maxSize elements
	err = self.redisClient.LTrim(self.ctx, key, -maxSize, -1).Err()
	if err != nil {
		return err
	}

	return nil
}

func (self *valkeyStore) ListFromBucket(start int64, stop int64, bucketKey ...string) ([]string, error) {
	key := createKey(bucketKey...)
	// start=0 stop=-1 to retrieve all elements from start to the end of the list

	elements, err := self.redisClient.LRange(self.ctx, key, start, stop).Result()

	return elements, err
}

func (self *valkeyStore) LastNEntryFromBucketWithType(number int64, bucketKey ...string) ([]string, error) {
	key := createKey(bucketKey...)

	// Get the length of the list
	length, err := self.redisClient.LLen(self.ctx, key).Result()
	if err != nil {
		return nil, err
	}

	// Calculate start index for LRANGE
	start := length - number
	if start < 0 {
		start = 0 // Ensure start index is not negative
	}

	// Use LRANGE to get the last N elements
	elements, err := self.redisClient.LRange(self.ctx, key, start, -1).Result()
	if err != nil {
		return nil, err
	}

	return elements, nil
}

func (self *valkeyStore) Delete(keys ...string) error {
	key := createKey(keys...)

	_, err := self.redisClient.Del(self.ctx, key).Result()
	if err != nil {
		self.logger.Error("Error deleting key from Redis", "key", key, "error", err)
		return err
	}

	return nil
}

func (self *valkeyStore) Keys(pattern string) ([]string, error) {
	keys, err := self.redisClient.Keys(self.ctx, pattern).Result()
	if err != nil {
		self.logger.Error("Error listing keys from Redis", "pattern", pattern, "error", err)
		return nil, err
	}

	return keys, nil
}

func (self *valkeyStore) Exists(keys ...string) (bool, error) {
	key := createKey(keys...)

	exists, err := self.redisClient.Exists(self.ctx, key).Result()
	if err != nil {
		self.logger.Error("Error checking if key exists in Redis", "key", key, "error", err)
		return false, err
	}

	return exists > 0, nil
}

func GetObjectsByPattern[T any](store ValkeyStore, pattern string, keywords []string) ([]T, error) {
	keyList := store.GetRedisClient().Keys(store.GetContext(), pattern).Val()

	// filter for keywords
	if len(keywords) > 0 {
		for i := 0; i < len(keyList); {
			if !utils.ContainsPatterns(keyList[i], keywords) {
				keyList = append(keyList[:i], keyList[i+1:]...)
			} else {
				i++
			}
		}
	}
	if len(keyList) == 0 {
		return nil, nil
	}

	// Fetch the values for these keys
	var objects []T
	values, err := store.GetRedisClient().MGet(store.GetContext(), keyList...).Result()
	if err != nil {
		return nil, err
	}
	for _, v := range values {
		var obj T
		if err := json.Unmarshal([]byte(v.(string)), &obj); err != nil {
			return nil, fmt.Errorf("error unmarshalling value from Redis, error: %v", err)
		}
		objects = append(objects, obj)
	}

	return objects, nil
}

func LastNEntryFromBucketWithType[T any](store ValkeyStore, number int64, bucketKey ...string) ([]T, error) {
	key := createKey(bucketKey...)

	// Get the length of the list
	length, err := store.GetRedisClient().LLen(store.GetContext(), key).Result()
	if err != nil {
		return nil, err
	}

	// Calculate start index for LRANGE
	start := length - number
	if start < 0 {
		start = 0 // Ensure start index is not negative
	}

	// Use LRANGE to get the last N elements
	elements, err := store.GetRedisClient().LRange(store.GetContext(), key, start, -1).Result()
	if err != nil {
		return nil, err
	}

	var objects []T
	for _, v := range elements {
		var obj T
		if err := json.Unmarshal([]byte(v), &obj); err != nil {
			return nil, fmt.Errorf("error unmarshalling value from valkey bucket, error: %v", err)
		}
		objects = append(objects, obj)
	}

	return objects, nil
}

func GetObjectsByPrefix[T any](redisStore ValkeyStore, order SortOrder, keys ...string) ([]T, error) {
	key := createKey(keys...)
	// var cursor uint64
	pattern := key + "*"
	// Get the keys
	keyList := redisStore.GetRedisClient().Keys(redisStore.GetContext(), pattern).Val()
	if len(keyList) == 0 {
		return nil, nil
	}

	// Sort keys
	sortStringsByTimestamp(keyList, order)

	// Fetch the values
	var objects []T
	values, err := redisStore.GetRedisClient().MGet(redisStore.GetContext(), keyList...).Result()
	if err != nil {
		return nil, err
	}
	for _, v := range values {
		if v != nil {
			var obj T
			if err := json.Unmarshal([]byte(v.(string)), &obj); err != nil {
				return nil, fmt.Errorf("error unmarshalling value from Redis, error: %v", err)
			}
			objects = append(objects, obj)
		}
	}
	return objects, nil
}

func GetObjectForKey[T any](store ValkeyStore, keys ...string) (*T, error) {
	key := createKey(keys...)

	var obj *T
	data, err := store.GetRedisClient().Get(store.GetContext(), key).Result()
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(data), obj)
	return obj, err
}

func createKey(parts ...string) string {
	return strings.Join(parts, ":")
}

func sortStringsByTimestamp(stringsToSort []string, order SortOrder) {
	if order == ORDER_NONE {
		return
	}

	extractTimestamp := func(s string) (int, error) {
		parts := strings.Split(s, ":")
		return strconv.Atoi(parts[len(parts)-1])
	}

	sort.Slice(stringsToSort, func(i, j int) bool {
		// Extract the timestamp parts from the strings
		timestamp1, err1 := extractTimestamp(stringsToSort[i])
		timestamp2, err2 := extractTimestamp(stringsToSort[j])

		// Handle potential errors
		if err1 != nil || err2 != nil {
			// If parsing fails for either, assume i < j to avoid changing order unpredictably
			return false
		}

		if order == ORDER_ASC {
			return timestamp1 < timestamp2
		}
		return timestamp1 > timestamp2
	})
}
