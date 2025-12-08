package valkeyclient

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/utils"
	"net"
	"path/filepath"
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
	SetObject(value any, expiration time.Duration, keys ...string) error
	SetObjectWithAutoincrementLimit(value any, limit int64, keys ...string) error
	Get(keys ...string) (string, error)
	GetObject(keys ...string) (any, error)
	List(limit int, keys ...string) ([]string, error)

	DeleteFromSortedListWithNsAndReleaseName(namespace string, releaseName string, keys ...string) error

	StoreSortedListEntry(data any, timestamp int64, keys ...string) error

	ClearNonEssentialKeys(includeTraffic bool, includePodStats bool, includeNodestats bool) (string, error)

	DeleteSingle(key ...string) error
	DeleteMultiple(patterns ...string) error
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
	MAX_RETENTION_SIZE = 10800
	MAX_RETENTION_TIME = 7 * 24 * time.Hour // 7 days
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

func (self *valkeyClient) SetObject(value any, expiration time.Duration, keys ...string) error {
	key := createKey(keys...)

	objStr, err := json.Marshal(value)
	if err != nil {
		self.logger.Error("Error marshalling object for Valkey", "key", key, "error", err)
		return err
	}

	return self.Set(string(objStr), expiration, key)
}

func (self *valkeyClient) SetObjectWithAutoincrementLimit(value any, limit int64, keys ...string) error {
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

func (self *valkeyClient) GetObject(keys ...string) (any, error) {
	key := createKey(keys...)
	var result any
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
		end := min(i+MAX_CHUNK_GET_SIZE, len(selectedKeys))
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

func (self *valkeyClient) DeleteFromSortedListWithNsAndReleaseName(namespace string, releaseName string, keys ...string) error {
	key := createKey(keys...)

	cmd := self.valkeyClient.B().Xrevrange().Key(key).End("+").Start("-").Build()
	result := self.valkeyClient.Do(self.ctx, cmd)
	if err := result.Error(); err != nil {
		return fmt.Errorf("failed to query stream: %w", err)
	}

	// Parse the stream messages
	messages, err := result.AsXRange()
	if err != nil {
		return fmt.Errorf("failed to parse stream result: %w", err)
	}

	for _, v := range messages {
		var obj map[string]any
		err := json.Unmarshal([]byte(v.FieldValues["data"]), &obj)
		if err != nil {
			return fmt.Errorf("error unmarshalling value from Redis, error: %v", err)
		}

		// Check if the object contains a "Payload" field
		payload, ok := obj["Payload"].(map[string]any)
		if !ok {
			continue
		}

		// Extract namespace and releaseName from the Payload
		if payload["namespace"] == namespace && payload["releaseName"] == releaseName {
			// Remove the specific entry from the sorted list
			delCmd := self.valkeyClient.B().Xdel().Key(key).Id(v.ID).Build()
			if err := self.valkeyClient.Do(self.ctx, delCmd).Error(); err != nil {
				return fmt.Errorf("failed to delete entry from stream: %w", err)
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

func (self *valkeyClient) DeleteMultiple(patterns ...string) error {
	if len(patterns) == 0 {
		return nil
	}

	cursor := uint64(0)
	batch := make([]string, 0, 100)
	totalDeleted := 0

	for {
		result := self.valkeyClient.Do(self.ctx,
			self.valkeyClient.B().Scan().Cursor(cursor).Count(1000).Build())

		if result.Error() != nil {
			return result.Error()
		}

		scanResult, err := result.AsScanEntry()
		if err != nil {
			return err
		}

		// Check each key against patterns
		for _, key := range scanResult.Elements {
			for _, pattern := range patterns {
				if matched, _ := filepath.Match(pattern, key); matched {
					batch = append(batch, key)
					break
				}
			}

			// Delete batch when full
			if len(batch) >= 100 {
				self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Del().Key(batch...).Build())
				totalDeleted += len(batch)
				batch = batch[:0]
			}
		}

		cursor = scanResult.Cursor
		if cursor == 0 {
			break
		}
	}

	// Delete remaining keys
	if len(batch) > 0 {
		self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Del().Key(batch...).Build())
		totalDeleted += len(batch)
	}

	self.logger.Info("Successfully deleted keys", "count", totalDeleted, "patterns", patterns)
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
		end := min(i+MAX_CHUNK_GET_SIZE, len(keyList))
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

func GetObjectsByPrefix[T any](store ValkeyClient, order SortOrder, keys ...string) ([]T, error) {
	var result []T
	pattern := createKey(keys...)

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
	key := strings.Join(parts, ":")

	// Remove trailing ":*:*" patterns
	for strings.HasSuffix(key, ":*:*") {
		key = strings.TrimSuffix(key, ":*")
	}

	return key
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

// trimStream removes old entries based on retention policy
func trimStream(self *valkeyClient, streamKey string) error {
	// Trim by time (remove entries older than retention period)
	if MAX_RETENTION_TIME > 0 {
		cutoffTime := time.Now().Add(-MAX_RETENTION_TIME)
		cutoffID := fmt.Sprintf("%d-0", cutoffTime.UnixMilli())

		// Build XTRIM MINID command
		cmd := self.valkeyClient.B().Xtrim().Key(streamKey).Minid().Threshold(cutoffID).Build()

		trimResult := self.valkeyClient.Do(self.ctx, cmd)
		if err := trimResult.Error(); err != nil {
			// Ignore "no such key" errors
			if err.Error() != "ERR no such key" {
				return fmt.Errorf("failed to trim by time: %w", err)
			}
			return nil
		}
	}

	if MAX_RETENTION_SIZE > 0 {
		// Build XTRIM MAXLEN command
		cmd := self.valkeyClient.B().Xtrim().Key(streamKey).Maxlen().Threshold(fmt.Sprintf("%d", MAX_RETENTION_SIZE)).Build()

		trimResult := self.valkeyClient.Do(self.ctx, cmd)
		if err := trimResult.Error(); err != nil {
			// Ignore "no such key" errors
			if err.Error() != "ERR no such key" {
				return fmt.Errorf("failed to trim by size: %w", err)
			}
			return nil
		}
	}

	return nil
}

func parseStreamMessages[T any](logger *slog.Logger, messages []valkey.XRangeEntry) ([]T, error) {
	var dataPoints []T

	for _, msg := range messages {
		if dataStr, ok := msg.FieldValues["data"]; ok {
			var dataPoint T
			err := json.Unmarshal([]byte(dataStr), &dataPoint)
			if err != nil {
				logger.Error("Failed to unmarshal stream data for ID %s: %v", msg.ID, err)
				continue
			}
			dataPoints = append(dataPoints, dataPoint)
		}
	}
	return dataPoints, nil
}

func (self *valkeyClient) StoreSortedListEntry(data any, timestamp int64, keys ...string) error {
	streamKey := createKey(keys...)

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	//  make sure it's in milliseconds because valkey requires it
	if timestamp < 10000000000 {
		timestamp = timestamp * 1000
	}

	id := fmt.Sprintf("%d-0", timestamp)
	// Build XADD command - removed the empty FieldValue() call
	cmd := self.valkeyClient.B().Xadd().Key(streamKey).Id(id).FieldValue().
		FieldValue("data", string(jsonData)).
		Build()

	result := self.valkeyClient.Do(self.ctx, cmd)
	if err := result.Error(); err != nil {
		errString := err.Error()

		if strings.Contains(errString, "The ID specified in XADD is equal or smaller than the target stream top item") {
			// This means we're trying to insert a duplicate entry
			// we dont care about duplicates
			return nil
		} else if errString == "WRONGTYPE Operation against a key holding the wrong kind of value" {
			// This means the key exists but is not a stream, delete it and try again
			self.logger.Warn("Wrong type for key, deleting it...", "key", streamKey)
			_, err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Del().Key(streamKey).Build()).AsInt64()
			if err != nil {
				return fmt.Errorf("failed to delete key: %w, key:%s", err, streamKey)
			}
			self.logger.Warn("Key deleted successfully", "key", streamKey)
			// Try adding the entry again
			cmd := self.valkeyClient.B().Xadd().Key(streamKey).Id(id).FieldValue().
				FieldValue("data", string(jsonData)).
				Build()
			result = self.valkeyClient.Do(self.ctx, cmd)
			if err := result.Error(); err != nil {
				return fmt.Errorf("failed to add to stream after deleting wrong type key: %w, key:%s", err, streamKey)
			}
		} else {
			return fmt.Errorf("failed to add to stream: %w, key:%s", err, streamKey)
		}
	}

	// Trim stream to maintain retention policy
	if err := trimStream(self, streamKey); err != nil {
		self.logger.Error("failed to trim stream", "key", streamKey, "error", err.Error())
	}

	// notify subscribers about the new entry
	err = self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Publish().Channel(createChannel(keys...)).Message(utils.PrintJson(data)).Build()).Error()
	if err != nil {
		return err
	}

	return nil
}

func GetObjectsFromSortedListWithDuration[T any](store ValkeyClient, duration int64, keys ...string) ([]T, error) {
	end := time.Now().UTC()
	start := end.Add(-time.Duration(duration) * time.Minute).UTC()
	return GetObjectsFromSortedListWithRange[T](store, start, end, keys...)
}

func GetObjectsFromSortedListWithRange[T any](store ValkeyClient, start, end time.Time, keys ...string) ([]T, error) {
	key := createKey(keys...)
	startID := fmt.Sprintf("%d-0", start.UnixMilli())
	endID := fmt.Sprintf("%d-0", end.UnixMilli())

	// Build XRANGE command using valkey-go command builder
	cmd := store.GetValkeyClient().B().Xrange().Key(key).Start(startID).End(endID).Build()
	result := store.GetValkeyClient().Do(store.GetContext(), cmd)

	if err := result.Error(); err != nil {
		return nil, fmt.Errorf("failed to query stream: %w", err)
	}

	// Parse the stream messages
	messages, err := result.AsXRange()
	if err != nil {
		return nil, fmt.Errorf("failed to parse stream result: %w", err)
	}

	return parseStreamMessages[T](store.GetLogger(), messages)
}

func GetLastObjectsFromSortedList[T any](store ValkeyClient, count int64, keys ...string) ([]T, error) {
	key := createKey(keys...)

	// Build XREVRANGE command to get entries in reverse order (newest first)
	// Using "+" as end (latest) and "-" as start (oldest), with COUNT to limit results
	cmd := store.GetValkeyClient().B().Xrevrange().Key(key).End("+").Start("-").Count(count).Build()
	result := store.GetValkeyClient().Do(store.GetContext(), cmd)
	if err := result.Error(); err != nil {
		return nil, fmt.Errorf("failed to query stream: %w", err)
	}

	// Parse the stream messages
	messages, err := result.AsXRange()
	if err != nil {
		return nil, fmt.Errorf("failed to parse stream result: %w", err)
	}

	return parseStreamMessages[T](store.GetLogger(), messages)
}

func createChannel(parts ...string) string {
	return strings.Join(parts, ":") + ":channel"
}
