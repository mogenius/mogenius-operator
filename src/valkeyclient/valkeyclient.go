package valkeyclient

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

type ValkeyClient interface {
	Connect() error
	Set(value interface{}, expiration time.Duration, keys ...string) error
	SetObject(value interface{}, expiration time.Duration, keys ...string) error
	Get(keys ...string) (string, error)
	GetObject(keys ...string) (interface{}, error)
	List(limit int, keys ...string) ([]string, error)

	AddToBucket(maxSize int64, value interface{}, bucketKey ...string) error
	ListFromBucket(start int64, stop int64, bucketKey ...string) ([]string, error)
	LastNEntryFromBucketWithType(number int64, bucketKey ...string) ([]string, error)
	DeleteFromBucketWithNsAndReleaseName(namespace string, releaseName string, bucketKey ...string) error
	SubscribeToBucket(bucketKey ...string) *redis.PubSub

	ClearNonEssentialKeys(includeTraffic bool, includePodStats bool, includeNodestats bool) (string, error)

	DeleteSingle(key ...string) error
	DeleteMultiple(keys ...string) error
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

	MAX_CHUNK_GET_SIZE = 100
)

type valkeyClient struct {
	logger *slog.Logger
	config config.ConfigModule

	ctx context.Context
	// internal redis client used to connect to a valkey instance
	redisClient *redis.Client
}

func NewValkeyClient(logger *slog.Logger, configModule config.ConfigModule) ValkeyClient {
	self := &valkeyClient{}

	self.ctx = context.Background()
	self.logger = logger
	self.config = configModule
	self.redisClient = redis.NewClient(&redis.Options{})

	return self
}

func (self *valkeyClient) Connect() error {
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

func (self *valkeyClient) GetRedisClient() *redis.Client {
	return self.redisClient
}

func (self *valkeyClient) GetContext() context.Context {
	return self.ctx
}

func (self *valkeyClient) GetLogger() *slog.Logger {
	return self.logger
}

func (self *valkeyClient) Set(value interface{}, expiration time.Duration, keys ...string) error {
	key := createKey(keys...)

	err := self.redisClient.Set(self.ctx, key, value, expiration).Err()
	if err != nil {
		self.logger.Error("Error setting value in Redis", "key", key, "error", err)
		return err
	}
	return nil
}

func (self *valkeyClient) SetObject(value interface{}, expiration time.Duration, keys ...string) error {
	key := createKey(keys...)

	objStr, err := json.Marshal(value)
	if err != nil {
		self.logger.Error("Error marshalling object for Redis", "key", key, "error", err)
		return err
	}
	return self.Set(objStr, expiration, key)
}

func (self *valkeyClient) Get(keys ...string) (string, error) {
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

func (self *valkeyClient) GetObject(keys ...string) (interface{}, error) {
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

func (self *valkeyClient) List(limit int, keys ...string) ([]string, error) {
	key := createKey(keys...)
	key = key + ":*"

	selectedKeys, err := self.redisClient.Keys(self.ctx, key).Result()
	if err != nil {
		self.logger.Error("Error listing keys from Redis", "pattern", key, "error", err)
		return nil, err
	}
	if len(selectedKeys) == 0 {
		self.logger.Debug("No keys found for pattern", "pattern", key)
		return selectedKeys, nil
	}

	// apply limit to the number of keys to fetch
	if limit > 0 && len(selectedKeys) > limit {
		selectedKeys = selectedKeys[:limit]
	}

	// Fetch the values in 100 chunks to avoid memory issues with large datasets
	var chunks [][]string
	for i := 0; i < len(selectedKeys); i += MAX_CHUNK_GET_SIZE {
		end := i + MAX_CHUNK_GET_SIZE
		if end > len(selectedKeys) {
			end = len(selectedKeys)
		}
		chunks = append(chunks, selectedKeys[i:end])
	}

	// Fetch the values for these keys
	var result []string
	for _, v := range chunks {
		values, err := self.redisClient.MGet(self.ctx, v...).Result()
		if err != nil {
			self.logger.Error("Error fetching values from Redis", "keys", v, "error", err)
			return nil, err
		}
		// Convert the values to a slice of strings
		for index, v := range values {
			if limit > 0 && index >= limit {
				break
			}
			if v != nil {
				result = append(result, v.(string))
			}
		}
	}
	return result, nil
}

func (self *valkeyClient) AddToBucket(maxSize int64, value interface{}, bucketKey ...string) error {
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

	err = self.redisClient.Publish(self.ctx, createChannel(bucketKey...), utils.PrintJson(value)).Err()
	if err != nil {
		return err
	}

	return nil
}

func (self *valkeyClient) ListFromBucket(start int64, stop int64, bucketKey ...string) ([]string, error) {
	key := createKey(bucketKey...)
	// start=0 stop=-1 to retrieve all elements from start to the end of the list

	elements, err := self.redisClient.LRange(self.ctx, key, start, stop).Result()

	return elements, err
}

func (self *valkeyClient) LastNEntryFromBucketWithType(number int64, bucketKey ...string) ([]string, error) {
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

func (self *valkeyClient) DeleteFromBucketWithNsAndReleaseName(namespace string, releaseName string, bucketKey ...string) error {
	key := createKey(bucketKey...)
	// Use LRANGE to get all elements in the bucket
	elements, err := self.redisClient.LRange(self.ctx, key, 0, -1).Result()
	if err != nil {
		return err
	}

	for _, v := range elements {
		var obj map[string]interface{}
		err := json.Unmarshal([]byte(v), &obj)
		if err != nil {
			return fmt.Errorf("error unmarshalling value from Redis, error: %v", err)
		}

		// Check if the object contains a "Payload" field
		payload, ok := obj["Payload"].(map[string]interface{})
		if !ok {
			continue
		}

		// Extract namespace and releaseName from the Payload
		if payload["namespace"] == namespace && payload["releaseName"] == releaseName {
			// Remove the specific entry from the bucket
			err = self.redisClient.LRem(self.ctx, key, 1, v).Err()
			if err != nil {
				return fmt.Errorf("error removing entry from Redis bucket, error: %v", err)
			}
		}
	}

	return nil
}

func (self *valkeyClient) ClearNonEssentialKeys(includeTraffic bool, includePodStats bool, includeNodestats bool) (string, error) {
	// Get all keys
	keys, err := self.redisClient.Keys(self.ctx, "*").Result()
	if err != nil {
		return "", err
	}

	// resources & helm have to be kept
	prefixesToDelete := []string{
		"live-stats:",
		"logs:cmd",
		"logs:core",
		"logs:client-provider",
		"logs:db-stats",
		"logs:http",
		"logs:klog",
		"logs:pod-stats-collector",
		"logs:kubernetes",
		"logs:leader-elector",
		"logs:mokubernetes",
		"logs:socketapi",
		"logs:structs",
		"logs:utils",
		"logs:xterm",
		"logs:traffic-collector",
		"logs:valkey",
		"logs:websocket-events-client",
		"logs:websocket-job-client",
		"maschine-stats:",
		"pod-events:",
		"status:",
	}

	if includeTraffic {
		prefixesToDelete = append(prefixesToDelete, "traffic-stats:")
	}
	if includePodStats {
		prefixesToDelete = append(prefixesToDelete, "pod-stats:")
	}
	if includeNodestats {
		prefixesToDelete = append(prefixesToDelete, "node-stats:")
	}

	self.logger.Info("Deleting non-essential keys from Redis", "includeTraffic", includeTraffic, "includePodStats", includePodStats, "includeNodestats", includeNodestats)

	// Iterate over the keys and delete them
	cacheDeleteCounter := 0
	for _, key := range keys {
		for _, keyToDelete := range prefixesToDelete {
			if strings.HasPrefix(key, keyToDelete) {
				err = self.redisClient.Del(self.ctx, key).Err()
				if err != nil {
					return "", fmt.Errorf("error deleting non-essential key from Redis, error: %v", err)
				}
				cacheDeleteCounter++
			}
		}
	}
	resultMsg := fmt.Sprintf("Deleted %d non-essential keys from Redis", cacheDeleteCounter)
	self.logger.Info(resultMsg, "deletedKeys", cacheDeleteCounter)

	return resultMsg, nil
}

func (self *valkeyClient) SubscribeToBucket(bucketKey ...string) *redis.PubSub {
	keyName := createChannel(bucketKey...)
	return self.redisClient.Subscribe(self.ctx, keyName)
}

func (self *valkeyClient) DeleteSingle(keys ...string) error {
	key := createKey(keys...)
	_, err := self.redisClient.Del(self.ctx, key).Result()
	if err != nil {
		self.logger.Error("Error deleting key from Redis", "key", keys, "error", err)
		return err
	}

	return nil
}

func (self *valkeyClient) DeleteMultiple(keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	_, err := self.redisClient.Del(self.ctx, keys...).Result()
	if err != nil {
		self.logger.Error("Error deleting key from Redis", "key", keys, "error", err)
		return err
	}

	return nil
}

func (self *valkeyClient) Keys(pattern string) ([]string, error) {
	keys, err := self.redisClient.Keys(self.ctx, pattern).Result()
	if err != nil {
		self.logger.Error("Error listing keys from Redis", "pattern", pattern, "error", err)
		return nil, err
	}

	return keys, nil
}

func (self *valkeyClient) Exists(keys ...string) (bool, error) {
	key := createKey(keys...)

	exists, err := self.redisClient.Exists(self.ctx, key).Result()
	if err != nil {
		self.logger.Error("Error checking if key exists in Redis", "key", key, "error", err)
		return false, err
	}

	return exists > 0, nil
}

func GetObjectsByPattern[T any](store ValkeyClient, pattern string, keywords []string) ([]T, error) {
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

	// Fetch the values in 100 chunks to avoid memory issues with large datasets
	var chunks [][]string
	for i := 0; i < len(keyList); i += MAX_CHUNK_GET_SIZE {
		end := i + MAX_CHUNK_GET_SIZE
		if end > len(keyList) {
			end = len(keyList)
		}
		chunks = append(chunks, keyList[i:end])
	}

	// Fetch the values for these keys
	var objects []T
	for _, v := range chunks {
		values, err := store.GetRedisClient().MGet(store.GetContext(), v...).Result()
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
	}

	return objects, nil
}

func LastNEntryFromBucketWithType[T any](store ValkeyClient, number int64, bucketKey ...string) ([]T, error) {
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
	for i := len(elements) - 1; i >= 0; i-- {
		var obj T
		if err := json.Unmarshal([]byte(elements[i]), &obj); err != nil {
			return nil, fmt.Errorf("error unmarshalling value from valkey bucket, error: %v", err)
		}
		objects = append(objects, obj)
	}

	return objects, nil
}

func GetObjectsByPrefix[T any](redisStore ValkeyClient, order SortOrder, keys ...string) ([]T, error) {
	key := createKey(keys...)
	pattern := key + "*"
	// Get the keys
	keyList := redisStore.GetRedisClient().Keys(redisStore.GetContext(), pattern).Val()
	if len(keyList) == 0 {
		return nil, nil
	}

	// Sort keys
	sortStringsByTimestamp(keyList, order)

	// Fetch the values in 100 chunks to avoid memory issues with large datasets
	var chunks [][]string
	// Loop over the original array and divide it into chunks
	for i := 0; i < len(keyList); i += MAX_CHUNK_GET_SIZE {
		end := i + MAX_CHUNK_GET_SIZE
		if end > len(keyList) {
			end = len(keyList)
		}
		chunks = append(chunks, keyList[i:end])
	}

	var objects []T
	for _, v := range chunks {
		values, err := redisStore.GetRedisClient().MGet(redisStore.GetContext(), v...).Result()
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
	}
	return objects, nil
}

func GetObjectForKey[T any](store ValkeyClient, keys ...string) (*T, error) {
	key := createKey(keys...)

	data, err := store.GetRedisClient().Get(store.GetContext(), key).Result()
	if err != nil {
		return nil, err
	}
	var obj T
	err = json.Unmarshal([]byte(data), &obj)
	return &obj, err
}

func createKey(parts ...string) string {
	return strings.Join(parts, ":")
}

func createChannel(parts ...string) string {
	return strings.Join(parts, ":") + ":channel"
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
