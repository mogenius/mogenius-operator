package kubernetes

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"strconv"
	"strings"
	"time"

	"go.etcd.io/bbolt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	BOLT_DB_STATS_SCHEMA_VERSION         = "3"
	BOLT_DB_STATS_TRAFFIC_BUCKET_NAME    = "traffic-stats"
	BOLT_DB_STATS_POD_STATS_BUCKET_NAME  = "pod-stats"
	BOLT_DB_STATS_NODE_STATS_BUCKET_NAME = "node-stats"
	BOLT_DB_STATS_SOCKET_STATS_BUCKET    = "socket-stats"
	BOLT_DB_STATS_CNI_BUCKET_NAME        = "cluster-cni-configuration"
)

type BoltDbStats interface {
	AddInterfaceStatsToDb(stats structs.InterfaceStats)
	AddNodeStatsToDb(stats structs.NodeStats)
	AddPodStatsToDb(stats structs.PodStats)
	GetCniData() ([]structs.CniData, error)
	GetLastPodStatsEntriesForNamespace(namespace string) []structs.PodStats
	GetLastPodStatsEntryForController(controller K8sController) *structs.PodStats
	GetPodStatsEntriesForController(controller K8sController) *[]structs.PodStats
	GetPodStatsEntriesForNamespace(namespace string) *[]structs.PodStats
	GetSocketConnectionsForController(controller K8sController) *structs.SocketConnections
	GetTrafficStatsEntriesForController(controller K8sController) *[]structs.InterfaceStats
	GetTrafficStatsEntriesForNamespace(namespace string) *[]structs.InterfaceStats
	GetTrafficStatsEntriesSumForNamespace(namespace string) []structs.InterfaceStats
	GetTrafficStatsEntrySumForController(controller K8sController, includeSocketConnections bool) *structs.InterfaceStats
	GetWorkspaceStatsCpuUtilization(req utils.WorkspaceStatsRequest, resources []unstructured.Unstructured) ([]GenericChartEntry, error)
	GetWorkspaceStatsMemoryUtilization(req utils.WorkspaceStatsRequest, resources []unstructured.Unstructured) ([]GenericChartEntry, error)
	GetWorkspaceStatsTrafficUtilization(req utils.WorkspaceStatsRequest, resources []unstructured.Unstructured) ([]GenericChartEntry, error)
	ReplaceCniData(data []structs.CniData)
}

type boldDbStatsModule struct {
	config cfg.ConfigModule
	logger *slog.Logger
	db     *bbolt.DB
	ctx    context.Context
	cancel context.CancelFunc
}

func NewBoltDbStatsModule(
	config cfg.ConfigModule,
	logger *slog.Logger,
) (BoltDbStats, error) {
	dbPath := strings.ReplaceAll(config.Get("MO_BBOLT_DB_STATS_PATH"), ".db", fmt.Sprintf("-%s.db", BOLT_DB_STATS_SCHEMA_VERSION))
	database, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("cant open bbolt database: %s", dbPath)
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	dbStatsModule := boldDbStatsModule{
		config: config,
		logger: logger,
		db:     database,
		ctx:    ctx,
		cancel: cancel,
	}

	err = dbStatsModule.initializeBoltDb()
	if err != nil {
		cancel()
		return nil, err
	}

	go func() {
		dbStatsModule.cleanupStats()

		ticker := time.NewTicker(1 * time.Minute)
		for {
			select {
			case <-ticker.C:
				dbStatsModule.cleanupStats()
			case <-dbStatsModule.ctx.Done():
				return
			}
		}
	}()

	shutdown.Add(func() {
		dbStatsModule.logger.Debug("Shutting down...")
		dbStatsModule.cancel()
		err := dbStatsModule.db.Close()
		if err != nil {
			dbStatsModule.logger.Error("failed to close db", "error", err)
		}
		dbStatsModule.logger.Debug("done")
	})

	logger.Info("bbold started 🚀", "dbPath", dbPath)

	return &dbStatsModule, nil
}

func (self *boldDbStatsModule) initializeBoltDb() error {
	// ### TRAFFIC BUCKET ###
	err := self.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BOLT_DB_STATS_TRAFFIC_BUCKET_NAME))
		if err == nil {
			self.logger.Debug("Bucket created 🚀.", "bucket", BOLT_DB_STATS_TRAFFIC_BUCKET_NAME)
		}
		return err
	})
	if err != nil {
		self.logger.Error("Error creating bucket", "bucket", BOLT_DB_STATS_TRAFFIC_BUCKET_NAME, "error", err)
	}

	// ### CNI BUCKET ###
	err = self.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BOLT_DB_STATS_CNI_BUCKET_NAME))
		if err == nil {
			self.logger.Debug("Bucket created 🚀.", "bucket", BOLT_DB_STATS_CNI_BUCKET_NAME)
		}
		return err
	})
	if err != nil {
		self.logger.Error("Error creating bucket", "bucket", BOLT_DB_STATS_CNI_BUCKET_NAME, "error", err)
	}

	// ### POD STATS BUCKET ###
	err = self.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BOLT_DB_STATS_POD_STATS_BUCKET_NAME))
		if err == nil {
			self.logger.Debug("Bucket created 🚀.", "bucket", BOLT_DB_STATS_POD_STATS_BUCKET_NAME)
		}
		return err
	})
	if err != nil {
		self.logger.Error("Error creating bucket", "bucket", BOLT_DB_STATS_POD_STATS_BUCKET_NAME, "error", err)
	}

	// ### NODE STATS BUCKET ###
	err = self.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BOLT_DB_STATS_NODE_STATS_BUCKET_NAME))
		if err == nil {
			self.logger.Debug("Bucket created 🚀.", "bucket", BOLT_DB_STATS_NODE_STATS_BUCKET_NAME)
		}
		return err
	})
	if err != nil {
		self.logger.Error("Error creating bucket", "bucket", BOLT_DB_STATS_NODE_STATS_BUCKET_NAME, "error", err)
	}

	// ### SOCKET STATS BUCKET ###
	err = self.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BOLT_DB_STATS_SOCKET_STATS_BUCKET))
		if err == nil {
			self.logger.Debug("Bucket created 🚀.", "bucket", BOLT_DB_STATS_SOCKET_STATS_BUCKET)
		}
		return err
	})
	if err != nil {
		self.logger.Error("Error creating bucket", "bucket", BOLT_DB_STATS_SOCKET_STATS_BUCKET, "error", err)
	}

	return nil
}

func (self *boldDbStatsModule) cleanupStats() {
	err := self.db.Update(func(tx *bbolt.Tx) error {
		// TRAFFIC
		bucketTraffic := tx.Bucket([]byte(BOLT_DB_STATS_TRAFFIC_BUCKET_NAME))
		err := bucketTraffic.ForEach(func(k, v []byte) error {
			namespaceBucket := bucketTraffic.Bucket(k)
			err := namespaceBucket.ForEach(func(k, v []byte) error {
				controllerBucket := namespaceBucket.Bucket(k)
				err := controllerBucket.ForEach(func(k, v []byte) error {
					entry := structs.InterfaceStats{}
					err := structs.UnmarshalInterfaceStats(&entry, v)
					if err != nil {
						return fmt.Errorf("cleanupStatsTraffic: %s", err.Error())
					}
					if self.isMoreThan14DaysOld(entry.CreatedAt) {
						err := controllerBucket.DeleteBucket(k)
						if err != nil {
							return fmt.Errorf("cleanupStatsTraffic: %s", err.Error())
						}
					}
					return nil
				})
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
		// PODS
		bucketPods := tx.Bucket([]byte(BOLT_DB_STATS_POD_STATS_BUCKET_NAME))
		err = bucketPods.ForEach(func(k, v []byte) error {
			namespaceBucket := bucketPods.Bucket(k)
			err := namespaceBucket.ForEach(func(k, v []byte) error {
				controllerBucket := namespaceBucket.Bucket(k)
				err := controllerBucket.ForEach(func(k, v []byte) error {
					entry := structs.PodStats{}
					err := structs.UnmarshalPodStats(&entry, v)
					if err != nil {
						return fmt.Errorf("cleanupStatsPods: %s", err.Error())
					}
					if self.isMoreThan14DaysOld(entry.CreatedAt) {
						err := controllerBucket.DeleteBucket(k)
						if err != nil {
							return fmt.Errorf("cleanupStatsPods: %s", err.Error())
						}
					}
					return nil
				})
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
		// Nodes
		bucketNodes := tx.Bucket([]byte(BOLT_DB_STATS_NODE_STATS_BUCKET_NAME))
		c := bucketNodes.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			subBucket := bucketNodes.Bucket(k)
			for kSub, _ := subBucket.Cursor().First(); kSub != nil; kSub, _ = subBucket.Cursor().Next() {
				entry := structs.NodeStats{}
				err := structs.UnmarshalNodeStats(&entry, subBucket.Get(kSub))
				if err != nil {
					return fmt.Errorf("cleanupStatsNodes: %s", err.Error())
				}
				if self.isMoreThan14DaysOld(entry.CreatedAt) {
					err := bucketNodes.DeleteBucket(k)
					if err != nil {
						return fmt.Errorf("cleanupStatsNodes: %s", err.Error())
					}
				}
			}
		}
		// SOCKETS
		bucketSockets := tx.Bucket([]byte(BOLT_DB_STATS_SOCKET_STATS_BUCKET))
		err = bucketSockets.ForEach(func(k, v []byte) error {
			entry := structs.SocketConnections{}
			err := structs.UnmarshalSocketConnections(&entry, v)
			if err != nil {
				return fmt.Errorf("cleanupStatsSocketConnections: %s", err.Error())
			}
			if self.isMoreThan14DaysOld(entry.LastUpdate) {
				err := bucketSockets.Delete(k)
				if err != nil {
					return fmt.Errorf("cleanupStatsSocketConnections: %s", err.Error())
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		self.logger.Error("cleanupStats", "error", err)
	}
}

func (self *boldDbStatsModule) isMoreThan14DaysOld(startTimeStr string) bool {
	// Parse the start time string into a time.Time object
	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		return true
	}

	// Get the current time
	currentTime := time.Now()

	// Check if startTime is more than 14 days ago
	return startTime.Before(currentTime.Add(-14 * (time.Hour * 24)))
}
func (self *boldDbStatsModule) AddInterfaceStatsToDb(stats structs.InterfaceStats) {
	stats.CreatedAt = time.Now().Format(time.RFC3339)
	err := self.db.Update(func(tx *bbolt.Tx) error {
		mainBucket := tx.Bucket([]byte(BOLT_DB_STATS_TRAFFIC_BUCKET_NAME))

		// CREATE A BUCKET FOR EACH NAMESPACE
		namespaceBucket, err := mainBucket.CreateBucketIfNotExists([]byte(stats.Namespace))
		if err != nil {
			return err
		}

		// CREATE A BUCKET FOR EACH CONTROLLER
		controller := ControllerForPod(stats.Namespace, stats.PodName)
		if controller == nil {
			return fmt.Errorf("Controller not found for '%s/%s'", stats.Namespace, stats.PodName)
		}
		controllerBucket, err := namespaceBucket.CreateBucketIfNotExists([]byte(controller.Name))
		if err != nil {
			return err
		}

		// DELETE FIRST IF TO MANY DATA POINTS
		maxDataPoints, err := strconv.Atoi(config.Get("MO_BBOLT_DB_STATS_MAX_DATA_POINTS"))
		assert.Assert(err == nil, err)
		if controllerBucket.Stats().KeyN > maxDataPoints {
			c := controllerBucket.Cursor()
			k, _ := c.First()
			err := controllerBucket.Delete(k)
			if err != nil {
				return err
			}
		}

		// save socketConnections to separate bucket and remove from stats
		socketBucket := tx.Bucket([]byte(BOLT_DB_STATS_SOCKET_STATS_BUCKET))
		err = socketBucket.Put([]byte(stats.PodName), []byte(utils.PrettyPrintString(self.cleanSocketConnections(stats.SocketConnections))))
		if err != nil {
			self.logger.Error("Error adding socket connections", "namespace", stats.Namespace, "podName", stats.PodName, "error", err.Error())
		}
		stats.SocketConnections = nil

		// add new Entry
		id, err := controllerBucket.NextSequence() // auto increment
		if err != nil {
			return fmt.Errorf("Cant create next id: %s", err.Error())
		}
		return controllerBucket.Put(utils.SequenceToKey(id), []byte(utils.PrettyPrintString(stats)))
	})
	if err != nil {
		self.logger.Error("Error adding interface stats", "namespace", stats.Namespace, "podName", stats.PodName, "error", err.Error())
	}
}
func (self *boldDbStatsModule) cleanSocketConnections(cons map[string]uint64) structs.SocketConnections {
	result := structs.SocketConnections{}
	result.Connections = make(map[string]uint64)
	for k, v := range cons {
		if v > 5 {
			result.Connections[k] = v
		}
	}
	result.LastUpdate = time.Now().Format(time.RFC3339)
	return result
}

func (self *boldDbStatsModule) ReplaceCniData(data []structs.CniData) {
	err := self.db.Update(func(tx *bbolt.Tx) error {
		cniDataBucket := tx.Bucket([]byte(BOLT_DB_STATS_CNI_BUCKET_NAME))

		// CHECKS
		if len(data) == 0 {
			return nil
		}
		nodeName := data[0].Node
		if nodeName == "" {
			return fmt.Errorf("Node name is empty")
		}

		// update Entry
		return cniDataBucket.Put([]byte(nodeName), []byte(utils.PrettyPrintString(data)))
	})
	if err != nil {
		self.logger.Error("Error adding CNI data", "error", err)
	}
}

func (self *boldDbStatsModule) GetCniData() ([]structs.CniData, error) {
	result := []structs.CniData{}
	err := self.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_STATS_CNI_BUCKET_NAME))
		return bucket.ForEach(func(k, v []byte) error {
			entry := []structs.CniData{}
			err := structs.UnmarshalCniData(&entry, v)
			if err != nil {
				return err
			}
			result = append(result, entry...)
			return nil
		})
	})
	if err != nil {
		self.logger.Error("GetCniData", "error", err)
	}
	return result, err
}

func (self *boldDbStatsModule) GetPodStatsEntriesForController(controller K8sController) *[]structs.PodStats {
	result := &[]structs.PodStats{}
	err := self.db.View(func(tx *bbolt.Tx) error {
		bucket, err := self.getSubBucket(tx.Bucket([]byte(BOLT_DB_STATS_POD_STATS_BUCKET_NAME)), []string{controller.Namespace, controller.Name})
		if err != nil {
			return err
		}

		return bucket.ForEach(func(k, v []byte) error {
			entry := structs.PodStats{}
			err := structs.UnmarshalPodStats(&entry, v)
			if err != nil {
				return err
			}
			*result = append(*result, entry)
			return nil
		})
	})
	if err != nil {
		self.logger.Error("GetPodStatsEntriesForController", "error", err)
	}
	return result
}

func (self *boldDbStatsModule) getSubBucket(bucket *bbolt.Bucket, bucketNames []string) (*bbolt.Bucket, error) {
	if len(bucketNames) == 0 {
		return bucket, nil
	}
	bucketName := bucketNames[0]
	subBucket := bucket.Bucket([]byte(bucketName))
	if subBucket == nil {
		return nil, fmt.Errorf("Bucket '%s' not found.", "/"+strings.Join(bucketNames, "/"))
	}

	return self.getSubBucket(subBucket, bucketNames[1:])
}

func (self *boldDbStatsModule) GetLastPodStatsEntryForController(controller K8sController) *structs.PodStats {
	result := &structs.PodStats{}
	err := self.db.View(func(tx *bbolt.Tx) error {
		bucket, err := self.getSubBucket(tx.Bucket([]byte(BOLT_DB_STATS_POD_STATS_BUCKET_NAME)), []string{controller.Namespace, controller.Name})
		if err != nil {
			return err
		}

		c := bucket.Cursor()
		k, v := c.Last()
		if k != nil {
			err := structs.UnmarshalPodStats(result, v)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		self.logger.Error("GetLastPodStatsEntryForController", "error", err)
	}
	return result
}

func (self *boldDbStatsModule) GetTrafficStatsEntriesForController(controller K8sController) *[]structs.InterfaceStats {
	result := &[]structs.InterfaceStats{}
	err := self.db.View(func(tx *bbolt.Tx) error {
		bucket, err := self.getSubBucket(tx.Bucket([]byte(BOLT_DB_STATS_TRAFFIC_BUCKET_NAME)), []string{controller.Namespace, controller.Name})
		if err != nil {
			return err
		}

		return bucket.ForEach(func(k, v []byte) error {
			entry := structs.InterfaceStats{}
			err := structs.UnmarshalInterfaceStats(&entry, v)
			if err != nil {
				return err
			}
			*result = append(*result, entry)
			return nil
		})
	})
	if err != nil {
		self.logger.Error("GetTrafficStatsEntriesForController", "error", err)
	}
	return result
}

func (self *boldDbStatsModule) GetTrafficStatsEntrySumForController(controller K8sController, includeSocketConnections bool) *structs.InterfaceStats {
	result := &structs.InterfaceStats{}
	err := self.db.View(func(tx *bbolt.Tx) error {
		bucket, err := self.getSubBucket(tx.Bucket([]byte(BOLT_DB_STATS_TRAFFIC_BUCKET_NAME)), []string{controller.Namespace, controller.Name})
		if err != nil {
			return err
		}
		c := bucket.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			entry := structs.InterfaceStats{}
			if includeSocketConnections {
				err = structs.UnmarshalInterfaceStats(&entry, bucket.Get(k))
			} else {
				err = structs.UnmarshalInterfaceStatsWithoutSocketConnections(&entry, bucket.Get(k))
			}
			if err != nil {
				return err
			}
			// initialize result
			if result.PodName == "" {
				result = &entry
			}
			if result.PodName != entry.PodName {
				// everytime the podName changes, sum up the values
				result.Sum(&entry)
				result.PodName = entry.PodName
			} else {
				// if the podName is the same, replace the values because it will be newer
				result.SumOrReplace(&entry)
			}
		}
		return nil
	})
	if err != nil {
		self.logger.Error("GetTrafficStatsEntrySumForController", "error", err)
	}
	result.PrintInfo()
	return result
}

func (self *boldDbStatsModule) GetWorkspaceStatsCpuUtilization(req utils.WorkspaceStatsRequest, resources []unstructured.Unstructured) ([]GenericChartEntry, error) {
	result := make([]GenericChartEntry, req.TimeOffSetMinutes)

	for _, controller := range resources {
		_ = self.db.View(func(tx *bbolt.Tx) error {
			bucket, err := self.getSubBucket(tx.Bucket([]byte(BOLT_DB_STATS_POD_STATS_BUCKET_NAME)), []string{controller.GetNamespace(), controller.GetName()})
			if err != nil {
				return nil
			}

			cursor := bucket.Cursor()
			index := 0
			for key, value := cursor.Last(); key != nil; key, value = cursor.Prev() {
				entry := structs.PodStats{}
				_ = structs.UnmarshalPodStats(&entry, value)
				result[index].Time = entry.CreatedAt
				result[index].Value += float64(entry.Cpu)

				index++
				if index >= req.TimeOffSetMinutes {
					break
				}
			}
			return nil
		})
	}
	return result, nil
}

func (self *boldDbStatsModule) GetWorkspaceStatsMemoryUtilization(req utils.WorkspaceStatsRequest, resources []unstructured.Unstructured) ([]GenericChartEntry, error) {
	result := make([]GenericChartEntry, req.TimeOffSetMinutes)

	for _, controller := range resources {
		_ = self.db.View(func(tx *bbolt.Tx) error {
			bucket, err := self.getSubBucket(tx.Bucket([]byte(BOLT_DB_STATS_POD_STATS_BUCKET_NAME)), []string{controller.GetNamespace(), controller.GetName()})
			if err != nil {
				return nil
			}

			cursor := bucket.Cursor()
			index := 0
			for key, value := cursor.Last(); key != nil; key, value = cursor.Prev() {
				entry := structs.PodStats{}
				_ = structs.UnmarshalPodStats(&entry, value)
				result[index].Time = entry.CreatedAt
				result[index].Value += float64(entry.Memory)

				index++
				if index >= req.TimeOffSetMinutes {
					break
				}
			}
			return nil
		})
	}
	return result, nil
}

func (self *boldDbStatsModule) GetWorkspaceStatsTrafficUtilization(req utils.WorkspaceStatsRequest, resources []unstructured.Unstructured) ([]GenericChartEntry, error) {
	result := make([]GenericChartEntry, req.TimeOffSetMinutes)

	for _, controller := range resources {
		_ = self.db.View(func(tx *bbolt.Tx) error {
			bucket, err := self.getSubBucket(tx.Bucket([]byte(BOLT_DB_STATS_TRAFFIC_BUCKET_NAME)), []string{controller.GetNamespace(), controller.GetName()})
			if err != nil {
				return nil
			}

			cursor := bucket.Cursor()
			index := 0
			for key, value := cursor.Last(); key != nil; key, value = cursor.Prev() {
				entry := structs.InterfaceStats{}
				_ = structs.UnmarshalInterfaceStats(&entry, value)
				result[index].Time = entry.CreatedAt
				result[index].Value += float64(entry.ReceivedBytes + entry.TransmitBytes)

				index++
				if index >= req.TimeOffSetMinutes {
					break
				}
			}
			return nil
		})
	}
	return result, nil
}

func (self *boldDbStatsModule) GetSocketConnectionsForController(controller K8sController) *structs.SocketConnections {
	result := &structs.SocketConnections{}
	err := self.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_STATS_SOCKET_STATS_BUCKET))
		c := bucket.Cursor()
		for k, data := c.First(); k != nil; k, _ = c.Next() {
			if strings.HasPrefix(string(k), controller.Name) {
				err := structs.UnmarshalSocketConnections(result, data)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		self.logger.Error("GetSocketConnectionsForController", "error", err)
		return nil
	}
	return result
}

func (self *boldDbStatsModule) GetPodStatsEntriesForNamespace(namespace string) *[]structs.PodStats {
	result := &[]structs.PodStats{}
	err := self.db.View(func(tx *bbolt.Tx) error {
		bucket, err := self.getSubBucket(tx.Bucket([]byte(BOLT_DB_STATS_POD_STATS_BUCKET_NAME)), []string{namespace})
		if err != nil {
			return err
		}
		return bucket.ForEach(func(k, v []byte) error {
			entry := structs.PodStats{}
			err := structs.UnmarshalPodStats(&entry, v)
			if err != nil {
				return err
			}
			*result = append(*result, entry)
			return nil
		})
	})
	if err != nil {
		self.logger.Error("GetPodStatsEntriesForNamespace", "error", err)
	}
	return result
}

func (self *boldDbStatsModule) GetLastPodStatsEntriesForNamespace(namespace string) []structs.PodStats {
	result := []structs.PodStats{}
	err := self.db.View(func(tx *bbolt.Tx) error {
		bucket, err := self.getSubBucket(tx.Bucket([]byte(BOLT_DB_STATS_POD_STATS_BUCKET_NAME)), []string{namespace})
		if err != nil {
			return err
		}

		return bucket.ForEach(func(k, v []byte) error {
			entry := structs.PodStats{}
			err := structs.UnmarshalPodStats(&entry, v)
			if err != nil {
				return err
			}
			var newEntry bool = true
			for _, currentPod := range result {
				if entry.PodName == currentPod.PodName {
					newEntry = false
				}

			}
			if newEntry {
				result = append(result, entry)
			}
			return nil
		})
	})
	if err != nil {
		self.logger.Error("GetLastPodStatsEntriesForNamespace", "error", err)
	}
	return result
}

func (self *boldDbStatsModule) GetTrafficStatsEntriesForNamespace(namespace string) *[]structs.InterfaceStats {
	result := &[]structs.InterfaceStats{}
	err := self.db.View(func(tx *bbolt.Tx) error {
		bucket, err := self.getSubBucket(tx.Bucket([]byte(BOLT_DB_STATS_TRAFFIC_BUCKET_NAME)), []string{namespace})
		if err != nil {
			return err
		}
		return bucket.ForEach(func(k, v []byte) error {
			entry := structs.InterfaceStats{}
			err := structs.UnmarshalInterfaceStats(&entry, v)
			if err != nil {
				return err
			}
			*result = append(*result, entry)
			return nil
		})
	})
	if err != nil {
		self.logger.Error("GetTrafficStatsEntriesForNamespace", "error", err)
	}
	return result
}

func (self *boldDbStatsModule) GetTrafficStatsEntriesSumForNamespace(namespace string) []structs.InterfaceStats {
	result := []structs.InterfaceStats{}
	err := self.db.View(func(tx *bbolt.Tx) error {
		bucket, err := self.getSubBucket(tx.Bucket([]byte(BOLT_DB_STATS_TRAFFIC_BUCKET_NAME)), []string{namespace})
		if err != nil {
			return err
		}

		controllerCursor := bucket.Cursor()
		for controllerName, _ := controllerCursor.First(); controllerName != nil; controllerName, _ = controllerCursor.Next() {
			controller := NewK8sController("", string(controllerName), namespace)
			entry := self.GetTrafficStatsEntrySumForController(controller, false)
			if entry != nil {
				result = append(result, *entry)
			}
		}
		return nil
	})
	if err != nil {
		self.logger.Warn("GetTrafficStatsEntriesSumForNamespace", "error", err)
	}
	return result
}

func (self *boldDbStatsModule) AddPodStatsToDb(stats structs.PodStats) {
	stats.CreatedAt = time.Now().Format(time.RFC3339)
	err := self.db.Update(func(tx *bbolt.Tx) error {
		mainBucket := tx.Bucket([]byte(BOLT_DB_STATS_POD_STATS_BUCKET_NAME))

		// CREATE A BUCKET FOR EACH NAMESPACE
		namespaceBucket, err := mainBucket.CreateBucketIfNotExists([]byte(stats.Namespace))
		if err != nil {
			return err
		}

		// CREATE A BUCKET FOR EACH CONTROLLER
		controller := ControllerForPod(stats.Namespace, stats.PodName)
		if controller == nil && stats.Namespace != "kube-system" {
			k8sLogger.Debug("Controller not found for pod", "namespace", stats.Namespace, "podName", stats.PodName)
			return nil
		}
		ctrlName := stats.Namespace
		if stats.Namespace != "kube-system" {
			ctrlName = controller.Name
		}
		controllerBucket, err := namespaceBucket.CreateBucketIfNotExists([]byte(ctrlName))
		if err != nil {
			return err
		}

		// DELETE FIRST IF TO MANY DATA POINTS
		maxDataPoints, err := strconv.Atoi(config.Get("MO_BBOLT_DB_STATS_MAX_DATA_POINTS"))
		assert.Assert(err == nil, err)
		if controllerBucket.Stats().KeyN > maxDataPoints {
			c := controllerBucket.Cursor()
			k, _ := c.First()
			err := controllerBucket.Delete(k)
			if err != nil {
				return err
			}
		}

		// add new Entry
		id, err := controllerBucket.NextSequence() // auto increment
		if err != nil {
			return fmt.Errorf("Cant create next id: %s", err.Error())
		}
		return controllerBucket.Put(utils.SequenceToKey(id), []byte(utils.PrettyPrintString(stats)))
	})
	if err != nil {
		self.logger.Error("Error adding pod stats", "namespace", stats.Namespace, "podName", stats.PodName, "error", err)
	}
}

func (self *boldDbStatsModule) AddNodeStatsToDb(stats structs.NodeStats) {
	stats.CreatedAt = time.Now().Format(time.RFC3339)
	err := self.db.Update(func(tx *bbolt.Tx) error {
		mainBucket := tx.Bucket([]byte(BOLT_DB_STATS_NODE_STATS_BUCKET_NAME))

		// CREATE A BUCKET FOR EACH NODE
		nodeBucket, err := mainBucket.CreateBucketIfNotExists([]byte(stats.Name))
		if err != nil {
			return err
		}

		maxDataPoints, err := strconv.Atoi(config.Get("MO_BBOLT_DB_STATS_MAX_DATA_POINTS"))
		assert.Assert(err == nil, err)
		// DELETE FIRST IF TO MANY DATA POINTS
		if nodeBucket.Stats().KeyN > maxDataPoints {
			c := nodeBucket.Cursor()
			k, _ := c.First()
			err := nodeBucket.Delete(k)
			if err != nil {
				return err
			}
		}

		// add new Entry
		id, err := nodeBucket.NextSequence() // auto increment
		if err != nil {
			return fmt.Errorf("Cant create next id: %s", err.Error())
		}
		return nodeBucket.Put(utils.SequenceToKey(id), []byte(utils.PrettyPrintString(stats)))
	})
	if err != nil {
		self.logger.Error("Error adding node stats", "name", stats.Name, "error", err)
	}
}

type GenericChartEntry struct {
	Time  string  `json:"time"`
	Value float64 `json:"value"`
}
