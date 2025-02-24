package redisstore

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/utils"
	"os"
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
	ctx         context.Context
	logger      *slog.Logger
	redisClient *redis.Client
}

var Global redisStore

func NewRedis(logger *slog.Logger) RedisStore {
	redisStoreModule := &redisStore{
		ctx:    context.Background(),
		logger: logger,
	}
	return redisStoreModule
}

func StartGlobalRedis(logger *slog.Logger) {
	Global = redisStore{
		ctx:    context.Background(),
		logger: logger,
	}
	err := Global.Connect()
	if err != nil {
		logger.Error("could not connect to Redis (for global store)", "error", err)
		os.Exit(1)
	}
}

func (r *redisStore) Connect() error {
	r.logger.Info("Connecting to Redis")
	redisUrl := os.Getenv("MO_REDIS_HOST")
	redisPwd := os.Getenv("MO_REDIS_PASSWORD")

	r.redisClient = redis.NewClient(&redis.Options{
		Addr:       redisUrl,
		Password:   redisPwd,
		DB:         0,
		MaxRetries: 0,
	})

	_, err := r.redisClient.Ping(r.ctx).Result()
	if err != nil {
		return fmt.Errorf("could not connect to Redis: %v", err)
	}
	r.logger.Info("Connected to Redis", "hostUrl", redisUrl)
	return nil
}

func GetGlobalCtx() context.Context {
	return Global.ctx
}

func GetGlobalLogger() *slog.Logger {
	return Global.logger
}

func GetGlobalRedisClient() *redis.Client {
	return Global.redisClient
}

func (r *redisStore) GetClient() *redis.Client {
	return r.redisClient
}

func (r *redisStore) GetContext() context.Context {
	return r.ctx
}

func (r *redisStore) GetLogger() *slog.Logger {
	return r.logger
}

func (r *redisStore) Set(value interface{}, expiration time.Duration, keys ...string) error {
	key := CreateKey(keys...)

	err := r.redisClient.Set(r.ctx, key, value, expiration).Err()
	if err != nil {
		r.logger.Error("Error setting value in Redis", "key", key, "error", err)
		return err
	}
	return nil
}

func (r *redisStore) SetObject(value interface{}, expiration time.Duration, keys ...string) error {
	key := CreateKey(keys...)

	objStr, err := json.Marshal(value)
	if err != nil {
		r.logger.Error("Error marshalling object for Redis", "key", key, "error", err)
		return err
	}
	return r.Set(objStr, expiration, key)
}

func (r *redisStore) Get(keys ...string) (string, error) {
	key := CreateKey(keys...)

	val, err := r.redisClient.Get(r.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			r.logger.Info("Key does not exist", "key", key)
			return "", nil
		}
		r.logger.Error("Error getting value from Redis", "key", key, "error", err)
		return "", err
	}
	return val, nil
}

func (r *redisStore) GetObject(keys ...string) (interface{}, error) {
	key := CreateKey(keys...)
	var result interface{}
	val, err := r.Get(key)
	if err != nil {
		return result, err
	}
	// Correct usage of Unmarshal
	err = json.Unmarshal([]byte(val), &result)
	if err != nil {
		r.logger.Error("Error unmarshalling value from Redis", "key", key, "error", err)
		return result, err
	}
	return result, nil
}

func GetObjectsByPrefix[T any](ctx context.Context, r *redis.Client, order SortOrder, keys ...string) ([]T, error) {
	key := CreateKey(keys...)
	// var cursor uint64
	pattern := key + "*"
	// Get the keys
	keyList := r.Keys(ctx, pattern).Val()
	if len(keyList) == 0 {
		return nil, nil
	}

	// Sort keys
	SortStringsByTimestamp(keyList, order)

	// Fetch the values
	var objects []T
	values, err := r.MGet(ctx, keyList...).Result()
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

func GetObjectsByPattern[T any](ctx context.Context, r *redis.Client, pattern string, keywords []string) ([]T, error) {
	keyList := r.Keys(ctx, pattern).Val()

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
	values, err := r.MGet(ctx, keyList...).Result()
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

func GetObjectForKey[T any](ctx context.Context, r *redis.Client, keys ...string) (*T, error) {
	key := CreateKey(keys...)

	var obj *T
	data, err := r.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(data), obj)
	return obj, err
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

func CreateKey(parts ...string) string {
	return strings.Join(parts, ":")
}

func SortStringsByTimestamp(stringsToSort []string, order SortOrder) {
	if order == ORDER_NONE {
		return
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

func extractTimestamp(s string) (int, error) {
	parts := strings.Split(s, ":")
	return strconv.Atoi(parts[len(parts)-1])
}
