package valkeyclient

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/utils"
	"net"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/valkey-io/valkey-go"
	valkeyclient "github.com/valkey-io/valkey-go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type ValkeyClient interface {
	Connect() error
	Set(value string, expiration time.Duration, keys ...string) error
	SetObject(value interface{}, expiration time.Duration, keys ...string) error
	SetObjectWithAutoincrementLimit(value interface{}, limit int64, keys ...string) error
	Get(keys ...string) (string, error)
	GetObject(keys ...string) (interface{}, error)
	List(limit int, keys ...string) ([]string, error)

	AddToBucket(maxSize int64, value interface{}, bucketKey ...string) error
	ListFromBucket(start int64, stop int64, bucketKey ...string) ([]string, error)
	LastNEntryFromBucketWithType(number int64, bucketKey ...string) ([]string, error)
	DeleteFromBucketWithNsAndReleaseName(namespace string, releaseName string, bucketKey ...string) error

	ClearNonEssentialKeys(includeTraffic bool, includePodStats bool, includeNodestats bool) (string, error)

	DeleteSingle(key ...string) error
	DeleteMultiple(keys ...string) error
	Keys(pattern string) ([]string, error)
	Exists(keys ...string) (bool, error)

	GetValkeyClient() valkeyclient.Client
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
	// internal valkey client used to connect to a valkey instance
	valkeyClient valkeyclient.Client
}

func NewValkeyClient(logger *slog.Logger, configModule config.ConfigModule) ValkeyClient {
	self := &valkeyClient{}

	self.ctx = context.Background()
	self.logger = logger
	self.config = configModule

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

	client, err := valkeyclient.NewClient(valkeyclient.ClientOption{
		InitAddress:         []string{valkeyAddr},
		Password:            valkeyPwd,
		SelectDB:            0,
		DisableRetry:        true,
		ReadBufferEachConn:  2 * (1 << 20), // 2 MiB
		WriteBufferEachConn: 2 * (1 << 20), // 2 MiB
	})
	if err != nil {
		self.logger.Info("connection to Valkey failed", "valkeyAddr", valkeyAddr, "password", valkeyPwd, "error", err)
		return fmt.Errorf("could not connect to Valkey: %s", err)
	}
	self.valkeyClient = client
	err = self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Ping().Build()).Error()
	if err != nil {
		self.logger.Info("connection to Valkey failed", "addr", valkeyAddr, "password", valkeyPwd, "error", err)
		return fmt.Errorf("could not connect to Valkey: %v", err)
	}

	self.logger.Info("Connected to valkey", "addr", valkeyAddr)

	return nil
}

func (self *valkeyClient) GetValkeyClient() valkeyclient.Client {
	return self.valkeyClient
}

func (self *valkeyClient) GetContext() context.Context {
	return self.ctx
}

func (self *valkeyClient) GetLogger() *slog.Logger {
	return self.logger
}

func (self *valkeyClient) Set(value string, expiration time.Duration, keys ...string) error {
	key := createKey(keys...)

	cmd := self.valkeyClient.B().Set().Key(key).Value(value)
	if expiration != time.Duration(0) {
		cmd.Ex(expiration)
	}
	err := self.valkeyClient.Do(self.ctx, cmd.Build()).Error()
	if err != nil {
		self.logger.Error("Error setting value in Valkey", "key", key, "error", err)
		return err
	}

	return nil
}

func (self *valkeyClient) SetObject(value interface{}, expiration time.Duration, keys ...string) error {
	key := createKey(keys...)

	objStr, err := json.Marshal(value)
	if err != nil {
		self.logger.Error("Error marshalling object for Valkey", "key", key, "error", err)
		return err
	}

	return self.Set(string(objStr), expiration, key)
}

func (self *valkeyClient) SetObjectWithAutoincrementLimit(value interface{}, limit int64, keys ...string) error {
	ctx := context.Background()

	baseKey := strings.Join(keys, ":")
	pattern := baseKey + ":*"

	existingKeys, err := self.Keys(pattern)
	if err != nil {
		return fmt.Errorf("error while parsing keys: %w", err)
	}

	sort.Slice(existingKeys, func(i, j int) bool {
		numI := extractNumber(existingKeys[i], baseKey)
		numJ := extractNumber(existingKeys[j], baseKey)
		return numI < numJ
	})

	var nextNum int64 = 1
	if len(existingKeys) > 0 {
		lastKey := existingKeys[len(existingKeys)-1]
		lastNum := extractNumber(lastKey, baseKey)
		nextNum = lastNum + 1
	}

	newKey := fmt.Sprintf("%s:%d", baseKey, nextNum)

	jsonValue, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("error while serializing value: %w", err)
	}

	var cmds []valkey.Completed
	cmds = append(cmds, self.valkeyClient.B().Set().Key(newKey).Value(string(jsonValue)).Build())

	if int64(len(existingKeys)) >= limit {
		keysToDelete := int64(len(existingKeys)) - limit + 1
		for i := int64(0); i < keysToDelete; i++ {
			cmds = append(cmds, self.valkeyClient.B().Del().Key(existingKeys[i]).Build())
		}
	}

	for _, resp := range self.valkeyClient.DoMulti(ctx, cmds...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("error while executing commands: %w", err)
		}
	}

	return nil
}

// Hilfsfunktion zum Extrahieren der Nummer aus einem Key
func extractNumber(key, baseKey string) int64 {
	prefix := baseKey + ":"
	if !strings.HasPrefix(key, prefix) {
		return 0
	}

	numStr := strings.TrimPrefix(key, prefix)
	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0
	}

	return num
}

func (self *valkeyClient) Get(keys ...string) (string, error) {
	key := createKey(keys...)

	val, err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Get().Key(key).Build()).ToString()
	if err != nil {
		if err == valkeyclient.Nil {
			self.logger.Info("Key does not exist", "key", key)
			return "", nil
		}
		self.logger.Error("Error getting value from Valkey", "key", key, "error", err)
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
		self.logger.Error("Error unmarshalling value from Valkey", "key", key, "error", err)
		return result, err
	}
	return result, nil
}

func (self *valkeyClient) List(limit int, keys ...string) ([]string, error) {
	key := createKey(keys...)
	key = key + ":*"

	selectedKeys, err := self.Keys(key)
	if err != nil {
		self.logger.Error("Error listing keys from Valkey", "pattern", key, "error", err)
		return []string{}, err
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
	result := []string{}
	for _, v := range chunks {
		values, err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Mget().Key(v...).Build()).AsStrSlice()
		if err != nil {
			self.logger.Error("Error fetching values from Valkey", "keys", v, "error", err)
			return result, err
		}
		for index, v := range values {
			if limit > 0 && index >= limit {
				break
			}
			result = append(result, v)
		}
	}

	return result, nil
}

func (self *valkeyClient) AddToBucket(maxSize int64, value interface{}, bucketKey ...string) error {
	// key := createKey(bucketKey...)
	// // Add the new elements to the end of the list
	// err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Rpush().Key(key).Element(utils.PrintJson(value)).Build()).Error()
	// if err != nil {
	// 	return err
	// }

	// // Trim the list to keep only the last maxSize elements
	// err = self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Ltrim().Key(key).Start(-maxSize).Stop(-1).Build()).Error()
	// if err != nil {
	// 	return err
	// }

	// err = self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Publish().Channel(createChannel(bucketKey...)).Message(utils.PrintJson(value)).Build()).Error()
	// if err != nil {
	// 	return err
	// }

	// return nil

	key := createKey(bucketKey...)

	obj := utils.PrintJson(value)

	// Build all commands (same pipeline approach as before)
	rpushCmd := self.valkeyClient.B().Rpush().Key(key).Element(obj).Build()
	ltrimCmd := self.valkeyClient.B().Ltrim().Key(key).Start(-maxSize).Stop(-1).Build()
	publishCmd := self.valkeyClient.B().Publish().Channel(createChannel(bucketKey...)).Message(obj).Build()

	// Execute all commands in one pipeline
	results := self.valkeyClient.DoMulti(self.ctx, rpushCmd, ltrimCmd, publishCmd)

	// Check for errors
	for _, result := range results {
		if err := result.Error(); err != nil {
			return err
		}
	}

	return nil
}

func (self *valkeyClient) ListFromBucket(start int64, stop int64, bucketKey ...string) ([]string, error) {
	key := createKey(bucketKey...)
	// start=0 stop=-1 to retrieve all elements from start to the end of the list

	elements, err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Lrange().Key(key).Start(start).Stop(stop).Build()).AsStrSlice()
	if err != nil {
		return []string{}, err
	}

	return elements, nil
}

func (self *valkeyClient) LastNEntryFromBucketWithType(number int64, bucketKey ...string) ([]string, error) {
	key := createKey(bucketKey...)

	// Get the length of the list
	length, err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Llen().Key(key).Build()).AsInt64()
	if err != nil {
		return []string{}, err
	}

	// Calculate start index for LRANGE
	start := length - number
	if start < 0 {
		start = 0 // Ensure start index is not negative
	}

	// Use LRANGE to get the last N elements
	elements, err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Lrange().Key(key).Start(start).Stop(-1).Build()).AsStrSlice()
	if err != nil {
		return []string{}, err
	}

	return elements, nil
}

func (self *valkeyClient) DeleteFromBucketWithNsAndReleaseName(namespace string, releaseName string, bucketKey ...string) error {
	key := createKey(bucketKey...)
	// Use LRANGE to get all elements in the bucket
	elements, err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Lrange().Key(key).Start(0).Stop(-1).Build()).AsStrSlice()
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
			err = self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Lrem().Key(key).Count(1).Element(v).Build()).Error()
			if err != nil {
				return fmt.Errorf("error removing entry from Valkey bucket, error: %v", err)
			}
		}
	}

	return nil
}

func (self *valkeyClient) ClearNonEssentialKeys(includeTraffic bool, includePodStats bool, includeNodestats bool) (string, error) {
	// Get all keys
	keys, err := self.Keys("*")
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

	self.logger.Info("Deleting non-essential keys from Valkey", "includeTraffic", includeTraffic, "includePodStats", includePodStats, "includeNodestats", includeNodestats)

	// Iterate over the keys and delete them
	cacheDeleteCounter := 0
	for _, key := range keys {
		for _, keyToDelete := range prefixesToDelete {
			if strings.HasPrefix(key, keyToDelete) {
				err = self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Del().Key(key).Build()).Error()
				if err != nil {
					return "", fmt.Errorf("error deleting non-essential key from Valkey, error: %v", err)
				}
				cacheDeleteCounter++
			}
		}
	}
	resultMsg := fmt.Sprintf("Deleted %d non-essential keys from Valkey", cacheDeleteCounter)
	self.logger.Info(resultMsg, "deletedKeys", cacheDeleteCounter)

	return resultMsg, nil
}

func (self *valkeyClient) DeleteSingle(keys ...string) error {
	key := createKey(keys...)
	err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Del().Key(key).Build()).Error()
	if err != nil {
		self.logger.Error("Error deleting key from Valkey", "key", keys, "error", err)
		return err
	}

	return nil
}

func (self *valkeyClient) DeleteMultiple(keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Del().Key(keys...).Build()).Error()
	if err != nil {
		self.logger.Error("Error deleting key from Valkey", "key", keys, "error", err)
		return err
	}

	return nil
}

func (self *valkeyClient) Keys(pattern string) ([]string, error) {
	// Pre-allocate with reasonable capacity to reduce reallocations
	allKeys := make([]string, 0, 1000)
	cursor := uint64(0)

	for {
		// Use larger count for fewer network round trips
		cmd := self.valkeyClient.B().Scan().Cursor(cursor).Match(pattern).Count(5000).Build()
		result, err := self.valkeyClient.Do(self.ctx, cmd).ToArray()
		if err != nil {
			self.logger.Error("Error scanning keys from Valkey", "pattern", pattern, "cursor", cursor, "error", err)
			return nil, err
		}

		if len(result) != 2 {
			self.logger.Error("Unexpected SCAN response format", "pattern", pattern, "result_length", len(result))
			return nil, fmt.Errorf("unexpected SCAN response format")
		}

		// Get new cursor
		newCursor, err := result[0].AsUint64()
		if err != nil {
			self.logger.Error("Error parsing cursor from SCAN response", "pattern", pattern, "error", err)
			return nil, err
		}

		// Get keys from this iteration
		keys, err := result[1].AsStrSlice()
		if err != nil {
			self.logger.Error("Error parsing keys from SCAN response", "pattern", pattern, "error", err)
			return nil, err
		}

		// Filter out empty strings
		for _, key := range keys {
			if key != "" {
				allKeys = append(allKeys, key)
			}
		}

		cursor = newCursor
		if cursor == 0 {
			break // Scan completed
		}
	}

	return allKeys, nil
}

func (self *valkeyClient) Exists(keys ...string) (bool, error) {
	key := createKey(keys...)

	exists, err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Exists().Key(key).Build()).AsInt64()
	if err != nil {
		self.logger.Error("Error checking if key exists in Valkey", "key", key, "error", err)
		return false, err
	}

	return exists > 0, nil
}

func GetObjectsByPattern[T any](store ValkeyClient, pattern string, keywords []string) ([]T, error) {
	var result []T
	client := store.GetValkeyClient()
	keyList, err := store.Keys(pattern)
	if err != nil {
		return result, err
	}

	// filter for keywords
	if len(keywords) > 0 {
		for i := 0; i < len(keyList); {
			if !slices.Contains(keywords, keyList[i]) {
				keyList = append(keyList[:i], keyList[i+1:]...)
			} else {
				i++
			}
		}
	}
	if len(keyList) == 0 {
		return result, nil
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
	for _, v := range chunks {
		values, err := client.Do(store.GetContext(), client.B().Mget().Key(v...).Build()).AsStrSlice()
		if err != nil {
			return result, err
		}
		for _, v := range values {
			var obj T
			if err := json.Unmarshal([]byte(v), &obj); err != nil {
				return result, fmt.Errorf("error unmarshalling value from Valkey, error: %v", err)
			}
			result = append(result, obj)
		}
	}

	return result, nil
}

func RangeFromEndOfBucketWithType[T any](store ValkeyClient, numberOfElements int64, offset int64, bucketKey ...string) ([]T, error) {
	var result []T
	key := createKey(bucketKey...)
	client := store.GetValkeyClient()

	// Get the length of the list
	length, err := client.Do(store.GetContext(), client.B().Llen().Key(key).Build()).AsInt64()
	if err != nil {
		return result, err
	}

	// Calculate start index for LRANGE
	start := length - (numberOfElements + offset)
	if start < 0 {
		start = 0 // Ensure start index is not negative
	}
	stop := length - offset

	// Use LRANGE to get the last N elements
	elements, err := client.Do(store.GetContext(), client.B().Lrange().Key(key).Start(start).Stop(stop).Build()).AsStrSlice()
	if err != nil {
		return result, err
	}

	result = make([]T, len(elements))
	for i := 0; i < len(elements); i++ {
		var obj T
		if err := json.Unmarshal([]byte(elements[i]), &obj); err != nil {
			return result, fmt.Errorf("error unmarshalling value from valkey bucket, error: %v", err)
		}
		result[i] = obj
	}

	return result, nil
}

func GetObjectsByPrefix[T any](store ValkeyClient, order SortOrder, keys ...string) ([]T, error) {
	var result []T
	key := createKey(keys...)
	pattern := key + "*"
	client := store.GetValkeyClient()

	// Get the keys
	keyList, err := store.Keys(pattern)
	if err != nil {
		return result, err
	}
	if len(keyList) == 0 {
		return result, nil
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

	for _, v := range chunks {
		values, err := client.Do(store.GetContext(), client.B().Mget().Key(v...).Build()).AsStrSlice()
		if err != nil {
			return result, err
		}
		for _, v := range values {
			var obj T
			if err := json.Unmarshal([]byte(v), &obj); err != nil {
				return result, fmt.Errorf("error unmarshalling value from Valkey, error: %v", err)
			}
			result = append(result, obj)
		}
	}
	return result, nil
}

func GetObjectsByPrefixWithSizeAndNs[T any](store ValkeyClient, limit int, offset int, namespaces []string, clusterWide bool, keys ...string) ([]T, int, error) {
	var result []T
	key := createKey(keys...)
	pattern := key + "*"
	client := store.GetValkeyClient()

	// Get the keys
	keyList, err := store.Keys(pattern)
	if err != nil {
		return result, 0, err
	}

	// remove elements which are not in the namespaces
	if !clusterWide {
		for i := len(keyList) - 1; i >= 0; i-- {
			split := strings.Split(keyList[i], ":")
			if len(split) < 3 {
				continue
			}
			namespace := split[1]
			if !slices.Contains(namespaces, namespace) {
				keyList = append(keyList[:i], keyList[i+1:]...)
			}
		}
	}

	for i := 0; i < len(keyList); i++ {
		if i < offset {
			keyList = append(keyList[:i], keyList[i+1:]...)
		}
		if i >= offset+limit {
			keyList = append(keyList[:i], keyList[i+1:]...)
		}
	}

	if len(keyList) <= 0 {
		return result, 0, nil
	}

	values, err := client.Do(store.GetContext(), client.B().Mget().Key(keyList...).Build()).AsStrSlice()
	if err != nil {
		return result, 0, err
	}
	for _, v := range values {
		var obj T
		if err := json.Unmarshal([]byte(v), &obj); err != nil {
			return result, 0, fmt.Errorf("error unmarshalling value from Valkey, error: %v", err)
		}
		result = append(result, obj)
	}
	return result, len(keyList), nil
}

func GetObjectForKey[T any](store ValkeyClient, keys ...string) (*T, error) {
	key := createKey(keys...)
	client := store.GetValkeyClient()
	data, err := client.Do(store.GetContext(), client.B().Get().Key(key).Build()).ToString()
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
