package redisstore

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/utils"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/go-redis/redis/v8"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type RedisStore interface {
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

	GetClient() *redis.Client
	GetContext() context.Context
	GetLogger() *slog.Logger
}

type SortOrder int

const (
	ORDER_NONE SortOrder = 0
	ORDER_ASC  SortOrder = 1
	ORDER_DESC SortOrder = 2
)

type redisStore struct {
	logger *slog.Logger
	config config.ConfigModule

	ctx         context.Context
	redisClient *redis.Client
}

func NewRedisStore(logger *slog.Logger, configModule config.ConfigModule) RedisStore {
	self := &redisStore{}
	self.ctx = context.Background()
	self.logger = logger
	self.config = configModule
	self.redisClient = redis.NewClient(&redis.Options{})

	return self
}

func (self *redisStore) Connect() error {
	self.logger.Info("Connecting to valkey")

	valkeyHost := self.config.Get("MO_VALKEY_HOST")
	valkeyUrl, err := url.Parse(valkeyHost)
	assert.Assert(err == nil, err)
	valkeyPwd := self.config.Get("MO_VALKEY_PASSWORD")

	self.redisClient = redis.NewClient(&redis.Options{
		Addr:       valkeyUrl.String(),
		Password:   valkeyPwd,
		DB:         0,
		MaxRetries: 0,
	})

	_, err = self.redisClient.Ping(self.ctx).Result()
	if err != nil {
		self.logger.Info("valkey connection failed", "url", valkeyUrl.String(), "password", valkeyPwd, "error", err)
		return fmt.Errorf("could not connect to valkey: %v", err)
	}
	self.logger.Info("Connected to valkey", "hostUrl", valkeyUrl)
	return nil
}

func (self *redisStore) GetClient() *redis.Client {
	return self.redisClient
}

func (self *redisStore) GetContext() context.Context {
	return self.ctx
}

func (self *redisStore) GetLogger() *slog.Logger {
	return self.logger
}

func (self *redisStore) Set(value interface{}, expiration time.Duration, keys ...string) error {
	key := CreateKey(keys...)

	err := self.redisClient.Set(self.ctx, key, value, expiration).Err()
	if err != nil {
		self.logger.Error("Error setting value in Redis", "key", key, "error", err)
		return err
	}
	return nil
}

func (self *redisStore) SetObject(value interface{}, expiration time.Duration, keys ...string) error {
	key := CreateKey(keys...)

	objStr, err := json.Marshal(value)
	if err != nil {
		self.logger.Error("Error marshalling object for Redis", "key", key, "error", err)
		return err
	}
	return self.Set(objStr, expiration, key)
}

func (self *redisStore) Get(keys ...string) (string, error) {
	key := CreateKey(keys...)

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

func (self *redisStore) GetObject(keys ...string) (interface{}, error) {
	key := CreateKey(keys...)
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

func (self *redisStore) AddToBucket(maxSize int64, value interface{}, bucketKey ...string) error {
	return AddToBucket(self.ctx, self.redisClient, maxSize, value, bucketKey...)
}

func (r *redisStore) ListFromBucket(start int64, stop int64, bucketKey ...string) ([]string, error) {
	key := CreateKey(bucketKey...)
	// start=0 stop=-1 to retrieve all elements from start to the end of the list
	elements, err := r.redisClient.LRange(r.ctx, key, start, stop).Result()
	return elements, err
}

func (r *redisStore) LastNEntryFromBucketWithType(number int64, bucketKey ...string) ([]string, error) {
	key := CreateKey(bucketKey...)

	// Get the length of the list
	length, err := r.redisClient.LLen(r.ctx, key).Result()
	if err != nil {
		return nil, err
	}

	// Calculate start index for LRANGE
	start := length - number
	if start < 0 {
		start = 0 // Ensure start index is not negative
	}

	// Use LRANGE to get the last N elements
	elements, err := r.redisClient.LRange(r.ctx, key, start, -1).Result()
	if err != nil {
		return nil, err
	}

	return elements, nil
}

func (r *redisStore) Delete(keys ...string) error {
	key := CreateKey(keys...)

	_, err := r.redisClient.Del(r.ctx, key).Result()
	if err != nil {
		r.logger.Error("Error deleting key from Redis", "key", key, "error", err)
		return err
	}
	return nil
}

func (r *redisStore) Keys(pattern string) ([]string, error) {
	keys, err := r.redisClient.Keys(r.ctx, pattern).Result()
	if err != nil {
		r.logger.Error("Error listing keys from Redis", "pattern", pattern, "error", err)
		return nil, err
	}
	return keys, nil
}

func (r *redisStore) Exists(keys ...string) (bool, error) {
	key := CreateKey(keys...)

	exists, err := r.redisClient.Exists(r.ctx, key).Result()
	if err != nil {
		r.logger.Error("Error checking if key exists in Redis", "key", key, "error", err)
		return false, err
	}
	return exists > 0, nil
}

func GetObjectsByPattern[T any](store RedisStore, pattern string, keywords []string) ([]T, error) {
	keyList := store.GetClient().Keys(store.GetContext(), pattern).Val()

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
	values, err := store.GetClient().MGet(store.GetContext(), keyList...).Result()
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

func AddToBucket(ctx context.Context, r *redis.Client, maxSize int64, value interface{}, bucketKey ...string) error {
	key := CreateKey(bucketKey...)
	// Add the new elements to the end of the list
	if err := r.RPush(ctx, key, utils.PrettyPrintInterface(value)).Err(); err != nil {
		return err
	}

	// Trim the list to keep only the last maxSize elements
	if err := r.LTrim(ctx, key, -maxSize, -1).Err(); err != nil {
		return err
	}

	return nil
}

func ListFromBucketWithType[T any](store RedisStore, start int64, stop int64, bucketKey ...string) ([]T, error) {
	key := CreateKey(bucketKey...)
	// Use -1 as end index to retrieve all elements from start to the end of the list
	elements, err := store.GetClient().LRange(store.GetContext(), key, start, stop).Result()
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

func LastNEntryFromBucketWithType[T any](store RedisStore, number int64, bucketKey ...string) ([]T, error) {
	key := CreateKey(bucketKey...)

	// Get the length of the list
	length, err := store.GetClient().LLen(store.GetContext(), key).Result()
	if err != nil {
		return nil, err
	}

	// Calculate start index for LRANGE
	start := length - number
	if start < 0 {
		start = 0 // Ensure start index is not negative
	}

	// Use LRANGE to get the last N elements
	elements, err := store.GetClient().LRange(store.GetContext(), key, start, -1).Result()
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

func GetObjectsByPrefix[T any](redisStore RedisStore, order SortOrder, keys ...string) ([]T, error) {
	key := CreateKey(keys...)
	// var cursor uint64
	pattern := key + "*"
	// Get the keys
	keyList := redisStore.GetClient().Keys(redisStore.GetContext(), pattern).Val()
	if len(keyList) == 0 {
		return nil, nil
	}

	// Sort keys
	SortStringsByTimestamp(keyList, order)

	// Fetch the values
	var objects []T
	values, err := redisStore.GetClient().MGet(redisStore.GetContext(), keyList...).Result()
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

func GetObjectForKey[T any](store RedisStore, keys ...string) (*T, error) {
	key := CreateKey(keys...)

	var obj *T
	data, err := store.GetClient().Get(store.GetContext(), key).Result()
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(data), obj)
	return obj, err
}

func CreateKey(parts ...string) string {
	return strings.Join(parts, ":")
}

func SortStringsByTimestamp(stringsToSort []string, order SortOrder) {
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
