package dbstats

import (
	"fmt"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"strings"
	"time"

	punqStructs "github.com/mogenius/punq/structs"
	bolt "go.etcd.io/bbolt"
)

const (
	DB_SCHEMA_VERSION = "3"
)

const (
	TRAFFIC_BUCKET_NAME    = "traffic-stats"
	POD_STATS_BUCKET_NAME  = "pod-stats"
	NODE_STATS_BUCKET_NAME = "node-stats"
	SOCKET_STATS_BUCKET    = "socket-stats"
)

var dbStats *bolt.DB
var cleanupTimer = time.NewTicker(1 * time.Minute)

func Start() {
	dbPath := strings.ReplaceAll(utils.CONFIG.Kubernetes.BboltDbStatsPath, ".db", fmt.Sprintf("-%s.db", DB_SCHEMA_VERSION))
	database, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		dbStatsLogger.Error("Error opening bbolt database.", "dbPath", dbPath, "error", err)
		panic(1)
	}
	dbStats = database

	// ### TRAFFIC BUCKET ###
	err = dbStats.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(TRAFFIC_BUCKET_NAME))
		if err == nil {
			dbStatsLogger.Info("Bucket created 🚀.", "bucket", TRAFFIC_BUCKET_NAME)
		}
		return err
	})
	if err != nil {
		dbStatsLogger.Error("Error creating bucket", "bucket", TRAFFIC_BUCKET_NAME, "error", err)
	}

	// ### POD STATS BUCKET ###
	err = dbStats.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(POD_STATS_BUCKET_NAME))
		if err == nil {
			dbStatsLogger.Info("Bucket created 🚀.", "bucket", POD_STATS_BUCKET_NAME)
		}
		return err
	})
	if err != nil {
		dbStatsLogger.Error("Error creating bucket", "bucket", POD_STATS_BUCKET_NAME, "error", err)
	}

	// ### NODE STATS BUCKET ###
	err = dbStats.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(NODE_STATS_BUCKET_NAME))
		if err == nil {
			dbStatsLogger.Info("Bucket created 🚀.", "bucket", NODE_STATS_BUCKET_NAME)
		}
		return err
	})
	if err != nil {
		dbStatsLogger.Error("Error creating bucket", "bucket", NODE_STATS_BUCKET_NAME, "error", err)
	}

	// ### SOCKET STATS BUCKET ###
	err = dbStats.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(SOCKET_STATS_BUCKET))
		if err == nil {
			dbStatsLogger.Info("Bucket created 🚀.", "bucket", SOCKET_STATS_BUCKET)
		}
		return err
	})
	if err != nil {
		dbStatsLogger.Error("Error creating bucket", "bucket", SOCKET_STATS_BUCKET, "error", err)
	}

	dbStatsLogger.Info("bbold started 🚀", "dbPath", dbPath)

	go func() {
		cleanupStats()
		for range cleanupTimer.C {
			cleanupStats()
		}
	}()
}

func close() {
	dbStatsLogger.Info("Shutting down db...")
	if dbStats != nil {
		dbStats.Close()
	}
}

// make function mockable
var getControllerFunc = kubernetes.ControllerForPod

func AddInterfaceStatsToDb(stats structs.InterfaceStats) {
	stats.CreatedAt = time.Now().Format(time.RFC3339)
	err := dbStats.Update(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket([]byte(TRAFFIC_BUCKET_NAME))

		// CREATE A BUCKET FOR EACH NAMESPACE
		namespaceBucket, err := mainBucket.CreateBucketIfNotExists([]byte(stats.Namespace))
		if err != nil {
			return err
		}

		// CREATE A BUCKET FOR EACH CONTROLLER
		controller := getControllerFunc(stats.Namespace, stats.PodName)
		if controller == nil {
			return fmt.Errorf("Controller not found for '%s/%s'", stats.Namespace, stats.PodName)
		}
		controllerBucket, err := namespaceBucket.CreateBucketIfNotExists([]byte(controller.Name))
		if err != nil {
			return err
		}

		// DELETE FIRST IF TO MANY DATA POINTS
		if controllerBucket.Stats().KeyN > utils.CONFIG.Stats.MaxDataPoints {
			c := controllerBucket.Cursor()
			k, _ := c.First()
			err := controllerBucket.Delete(k)
			if err != nil {
				return err
			}
		}

		// save socketConnections to separate bucket and remove from stats
		socketBucket := tx.Bucket([]byte(SOCKET_STATS_BUCKET))
		err = socketBucket.Put([]byte(stats.PodName), []byte(punqStructs.PrettyPrintString(cleanSocketConnections(stats.SocketConnections))))
		if err != nil {
			dbStatsLogger.Error("Error adding socket connections", "namespace", stats.Namespace, "podName", stats.PodName, "error", err.Error())
		}
		stats.SocketConnections = nil

		// add new Entry
		id, err := controllerBucket.NextSequence() // auto increment
		if err != nil {
			return fmt.Errorf("Cant create next id: %s", err.Error())
		}
		return controllerBucket.Put(utils.SequenceToKey(id), []byte(punqStructs.PrettyPrintString(stats)))
	})
	if err != nil {
		dbStatsLogger.Error("Error adding interface stats", "namespace", stats.Namespace, "podName", stats.PodName, "error", err.Error())
	}
}

// Only save socket connections if more than 5 connections have been made
func cleanSocketConnections(cons map[string]uint64) structs.SocketConnections {
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

func GetSocketConnectionsForPod(podName string) structs.SocketConnections {
	result := structs.SocketConnections{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(SOCKET_STATS_BUCKET))
		data := bucket.Get([]byte(podName))
		if data != nil {
			err := structs.UnmarshalSocketConnections(&result, data)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		dbStatsLogger.Error("GetSocketConnectionsForPod", "error", err)
	}
	return result
}

func AddNodeStatsToDb(stats structs.NodeStats) {
	stats.CreatedAt = time.Now().Format(time.RFC3339)
	err := dbStats.Update(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket([]byte(NODE_STATS_BUCKET_NAME))

		// CREATE A BUCKET FOR EACH NODE
		nodeBucket, err := mainBucket.CreateBucketIfNotExists([]byte(stats.Name))
		if err != nil {
			return err
		}

		// DELETE FIRST IF TO MANY DATA POINTS
		if nodeBucket.Stats().KeyN > utils.CONFIG.Stats.MaxDataPoints {
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
		return nodeBucket.Put(utils.SequenceToKey(id), []byte(punqStructs.PrettyPrintString(stats)))
	})
	if err != nil {
		dbStatsLogger.Error("Error adding node stats", "name", stats.Name, "error", err)
	}
}

func AddPodStatsToDb(stats structs.PodStats) {
	stats.CreatedAt = time.Now().Format(time.RFC3339)
	err := dbStats.Update(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket([]byte(POD_STATS_BUCKET_NAME))

		// CREATE A BUCKET FOR EACH NAMESPACE
		namespaceBucket, err := mainBucket.CreateBucketIfNotExists([]byte(stats.Namespace))
		if err != nil {
			return err
		}

		// CREATE A BUCKET FOR EACH CONTROLLER
		controller := kubernetes.ControllerForPod(stats.Namespace, stats.PodName)
		if controller == nil && stats.Namespace != "kube-system" {
			return fmt.Errorf("Controller not found for '%s/%s'", stats.Namespace, stats.PodName)
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
		if controllerBucket.Stats().KeyN > utils.CONFIG.Stats.MaxDataPoints {
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
		return controllerBucket.Put(utils.SequenceToKey(id), []byte(punqStructs.PrettyPrintString(stats)))
	})
	if err != nil {
		dbStatsLogger.Error("Error adding pod stats", "namespace", stats.Namespace, "podName", stats.PodName, "error", err)
	}
}

func GetSocketConnectionsForController(controller kubernetes.K8sController) *structs.SocketConnections {
	result := &structs.SocketConnections{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(SOCKET_STATS_BUCKET))
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
		dbStatsLogger.Error("GetSocketConnectionsForController", "error", err)
		return nil
	}
	return result
}

func GetTrafficStatsEntrySumForController(controller kubernetes.K8sController, includeSocketConnections bool) *structs.InterfaceStats {
	result := &structs.InterfaceStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket, err := GetSubBucket(tx.Bucket([]byte(TRAFFIC_BUCKET_NAME)), []string{controller.Namespace, controller.Name})
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
		dbStatsLogger.Error("GetTrafficStatsEntrySumForController", "error", err)
	}
	result.PrintInfo()
	return result
}

func GetTrafficStatsEntriesForController(controller kubernetes.K8sController) *[]structs.InterfaceStats {
	result := &[]structs.InterfaceStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket, err := GetSubBucket(tx.Bucket([]byte(TRAFFIC_BUCKET_NAME)), []string{controller.Namespace, controller.Name})
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
		dbStatsLogger.Error("GetTrafficStatsEntriesForController", "error", err)
	}
	return result
}

func GetLastPodStatsEntryForController(controller kubernetes.K8sController) *structs.PodStats {
	result := &structs.PodStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket, err := GetSubBucket(tx.Bucket([]byte(POD_STATS_BUCKET_NAME)), []string{controller.Namespace, controller.Name})
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
		dbStatsLogger.Error("GetLastPodStatsEntryForController", "error", err)
	}
	return result
}

func GetPodStatsEntriesForController(controller kubernetes.K8sController) *[]structs.PodStats {
	result := &[]structs.PodStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket, err := GetSubBucket(tx.Bucket([]byte(POD_STATS_BUCKET_NAME)), []string{controller.Namespace, controller.Name})
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
		dbStatsLogger.Error("GetPodStatsEntriesForController", "error", err)
	}
	return result
}

func GetLastPodStatsEntriesForNamespace(namespace string) []structs.PodStats {
	result := []structs.PodStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket, err := GetSubBucket(tx.Bucket([]byte(POD_STATS_BUCKET_NAME)), []string{namespace})
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
		dbStatsLogger.Error("GetLastPodStatsEntriesForNamespace", "error", err)
	}
	return result
}

func GetPodStatsEntriesForNamespace(namespace string) *[]structs.PodStats {
	result := &[]structs.PodStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket, err := GetSubBucket(tx.Bucket([]byte(POD_STATS_BUCKET_NAME)), []string{namespace})
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
		dbStatsLogger.Error("GetPodStatsEntriesForNamespace", "error", err)
	}
	return result
}

func GetTrafficStatsEntriesSumForNamespace(namespace string) []structs.InterfaceStats {
	result := []structs.InterfaceStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket, err := GetSubBucket(tx.Bucket([]byte(TRAFFIC_BUCKET_NAME)), []string{namespace})
		if err != nil {
			return err
		}

		controllerCursor := bucket.Cursor()
		for controllerName, _ := controllerCursor.First(); controllerName != nil; controllerName, _ = controllerCursor.Next() {
			controller := kubernetes.NewK8sController("", string(controllerName), namespace)
			entry := GetTrafficStatsEntrySumForController(controller, false)
			if entry != nil {
				result = append(result, *entry)
			}
		}
		return nil
	})
	if err != nil {
		dbStatsLogger.Error("GetTrafficStatsEntriesSumForNamespace", "error", err)
	}
	return result
}

func GetTrafficStatsEntriesForNamespace(namespace string) *[]structs.InterfaceStats {
	result := &[]structs.InterfaceStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket, err := GetSubBucket(tx.Bucket([]byte(TRAFFIC_BUCKET_NAME)), []string{namespace})
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
		dbStatsLogger.Error("GetTrafficStatsEntriesForNamespace", "error", err)
	}
	return result
}

func cleanupStats() {
	err := dbStats.Update(func(tx *bolt.Tx) error {
		// TRAFFIC
		bucketTraffic := tx.Bucket([]byte(TRAFFIC_BUCKET_NAME))
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
					if isMoreThan14DaysOld(entry.CreatedAt) {
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
		bucketPods := tx.Bucket([]byte(POD_STATS_BUCKET_NAME))
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
					if isMoreThan14DaysOld(entry.CreatedAt) {
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
		bucketNodes := tx.Bucket([]byte(NODE_STATS_BUCKET_NAME))
		c := bucketNodes.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			subBucket := bucketNodes.Bucket(k)
			for kSub, _ := subBucket.Cursor().First(); kSub != nil; kSub, _ = subBucket.Cursor().Next() {
				entry := structs.NodeStats{}
				err := structs.UnmarshalNodeStats(&entry, subBucket.Get(kSub))
				if err != nil {
					return fmt.Errorf("cleanupStatsNodes: %s", err.Error())
				}
				if isMoreThan14DaysOld(entry.CreatedAt) {
					err := bucketNodes.DeleteBucket(k)
					if err != nil {
						return fmt.Errorf("cleanupStatsNodes: %s", err.Error())
					}
				}
			}
		}
		// SOCKETS
		bucketSockets := tx.Bucket([]byte(SOCKET_STATS_BUCKET))
		err = bucketSockets.ForEach(func(k, v []byte) error {
			entry := structs.SocketConnections{}
			err := structs.UnmarshalSocketConnections(&entry, v)
			if err != nil {
				return fmt.Errorf("cleanupStatsSocketConnections: %s", err.Error())
			}
			if isMoreThan14DaysOld(entry.LastUpdate) {
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
		dbStatsLogger.Error("cleanupStats", "error", err)
	}
}

func GetSubBucket(bucket *bolt.Bucket, bucketNames []string) (*bolt.Bucket, error) {
	if len(bucketNames) == 0 {
		return bucket, nil
	}
	bucketName := bucketNames[0]
	subBucket := bucket.Bucket([]byte(bucketName))
	if subBucket == nil {
		return nil, fmt.Errorf("Bucket '%s' not found.", "/"+strings.Join(bucketNames, "/"))
	}
	return GetSubBucket(subBucket, bucketNames[1:])
}
