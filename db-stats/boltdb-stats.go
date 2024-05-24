package dbstats

import (
	"fmt"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"strings"
	"time"

	punqStructs "github.com/mogenius/punq/structs"
	log "github.com/sirupsen/logrus"
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

func Init() {
	dbPath := strings.ReplaceAll(utils.CONFIG.Kubernetes.BboltDbStatsPath, ".db", fmt.Sprintf("-%s.db", DB_SCHEMA_VERSION))
	database, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		log.Errorf("Error opening bbolt database from '%s'.", dbPath)
		log.Fatal(err.Error())
	}
	dbStats = database

	// ### TRAFFIC BUCKET ###
	err = dbStats.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(TRAFFIC_BUCKET_NAME))
		if err == nil {
			log.Infof("Bucket '%s' created 🚀.", TRAFFIC_BUCKET_NAME)
		}
		return err
	})
	if err != nil {
		log.Errorf("Error creating bucket ('%s'): %s", TRAFFIC_BUCKET_NAME, err)
	}

	// ### POD STATS BUCKET ###
	err = dbStats.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(POD_STATS_BUCKET_NAME))
		if err == nil {
			log.Infof("Bucket '%s' created 🚀.", POD_STATS_BUCKET_NAME)
		}
		return err
	})
	if err != nil {
		log.Errorf("Error creating bucket ('%s'): %s", POD_STATS_BUCKET_NAME, err)
	}

	// ### NODE STATS BUCKET ###
	err = dbStats.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(NODE_STATS_BUCKET_NAME))
		if err == nil {
			log.Infof("Bucket '%s' created 🚀.", NODE_STATS_BUCKET_NAME)
		}
		return err
	})
	if err != nil {
		log.Errorf("Error creating bucket ('%s'): %s", NODE_STATS_BUCKET_NAME, err)
	}

	// ### SOCKET STATS BUCKET ###
	err = dbStats.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(SOCKET_STATS_BUCKET))
		if err == nil {
			log.Infof("Bucket '%s' created 🚀.", SOCKET_STATS_BUCKET)
		}
		return err
	})
	if err != nil {
		log.Errorf("Error creating bucket ('%s'): %s", SOCKET_STATS_BUCKET, err)
	}

	log.Infof("bbold started 🚀 (Path: '%s')", dbPath)

	go func() {
		cleanupStats()
		for range cleanupTimer.C {
			cleanupStats()
		}
	}()
}

func Close() {
	if dbStats != nil {
		dbStats.Close()
	}
}

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
		controller := kubernetes.ControllerForPod(stats.Namespace, stats.PodName)
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
			controllerBucket.Delete(k)
		}

		// save socketConnections to separate bucket and remove from stats
		socketBucket := tx.Bucket([]byte(SOCKET_STATS_BUCKET))
		err = socketBucket.Put([]byte(stats.PodName), []byte(punqStructs.PrettyPrintString(cleanSocketConnections(stats.SocketConnections))))
		if err != nil {
			log.Errorf("Error adding socket connections for '%s': %s", stats.PodName, err.Error())
		}
		stats.SocketConnections = nil

		// add new Entry
		id, _ := controllerBucket.NextSequence() // auto increment
		return controllerBucket.Put(utils.SequenceToKey(id), []byte(punqStructs.PrettyPrintString(stats)))
	})
	if err != nil {
		log.Errorf("Error adding interface stats for '%s': %s", stats.Namespace, err.Error())
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
		log.Errorf("GetSocketConnectionsForPod: %s", err.Error())
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
			nodeBucket.Delete(k)
		}

		// add new Entry
		id, _ := nodeBucket.NextSequence() // auto increment
		return nodeBucket.Put(utils.SequenceToKey(id), []byte(punqStructs.PrettyPrintString(stats)))
	})
	if err != nil {
		log.Errorf("Error adding node stats for '%s': %s", stats.Name, err.Error())
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
			controllerBucket.Delete(k)
		}

		// add new Entry
		id, _ := controllerBucket.NextSequence() // auto increment
		return controllerBucket.Put(utils.SequenceToKey(id), []byte(punqStructs.PrettyPrintString(stats)))
	})
	if err != nil {
		log.Errorf("Error adding pod stats for '%s': %s", stats.Namespace, err.Error())
	}
}

func GetTrafficStatsEntrySumForController(controller kubernetes.K8sController, includeSocketConnections bool) *structs.InterfaceStats {
	result := &structs.InterfaceStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket, err := GetSubBuckets(tx.Bucket([]byte(TRAFFIC_BUCKET_NAME)), []string{controller.Namespace, controller.Name})
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
		log.Errorf("GetTrafficStatsEntrySumForController: %s", err.Error())
	}
	result.PrintInfo()
	return result
}

func GetTrafficStatsEntriesForController(controller kubernetes.K8sController) *[]structs.InterfaceStats {
	result := &[]structs.InterfaceStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket, err := GetSubBuckets(tx.Bucket([]byte(TRAFFIC_BUCKET_NAME)), []string{controller.Namespace, controller.Name})
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
		log.Errorf("GetTrafficStatsEntriesForController: %s", err.Error())
	}
	return result
}

func GetLastPodStatsEntryForController(controller kubernetes.K8sController) *structs.PodStats {
	result := &structs.PodStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket, err := GetSubBuckets(tx.Bucket([]byte(POD_STATS_BUCKET_NAME)), []string{controller.Namespace, controller.Name})
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
		log.Errorf("GetLastPodStatsEntryForController: %s", err.Error())
	}
	return result
}

func GetPodStatsEntriesForController(controller kubernetes.K8sController) *[]structs.PodStats {
	result := &[]structs.PodStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket, err := GetSubBuckets(tx.Bucket([]byte(POD_STATS_BUCKET_NAME)), []string{controller.Namespace, controller.Name})
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
		log.Errorf("GetPodStatsEntriesForController: %s", err.Error())
	}
	return result
}

func GetLastPodStatsEntriesForNamespace(namespace string) []structs.PodStats {
	result := []structs.PodStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket, err := GetSubBuckets(tx.Bucket([]byte(POD_STATS_BUCKET_NAME)), []string{namespace})
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
		log.Errorf("GetLastPodStatsEntriesForNamespace: %s", err.Error())
	}
	return result
}

func GetPodStatsEntriesForNamespace(namespace string) *[]structs.PodStats {
	result := &[]structs.PodStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket, err := GetSubBuckets(tx.Bucket([]byte(POD_STATS_BUCKET_NAME)), []string{namespace})
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
		log.Errorf("GetPodStatsEntriesForNamespace: %s", err.Error())
	}
	return result
}

func GetTrafficStatsEntriesSumForNamespace(namespace string) []structs.InterfaceStats {
	result := []structs.InterfaceStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket, err := GetSubBuckets(tx.Bucket([]byte(TRAFFIC_BUCKET_NAME)), []string{namespace})
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
		log.Errorf("GetTrafficStatsEntriesSumForNamespace: %s", err.Error())
	}
	return result
}

func GetTrafficStatsEntriesForNamespace(namespace string) *[]structs.InterfaceStats {
	result := &[]structs.InterfaceStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket, err := GetSubBuckets(tx.Bucket([]byte(TRAFFIC_BUCKET_NAME)), []string{namespace})
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
		log.Errorf("GetTrafficStatsEntriesForNamespace: %s", err.Error())
	}
	return result
}

func cleanupStats() {
	err := dbStats.Update(func(tx *bolt.Tx) error {
		// TRAFFIC
		bucketTraffic := tx.Bucket([]byte(TRAFFIC_BUCKET_NAME))
		bucketTraffic.ForEach(func(k, v []byte) error {
			namespaceBucket := bucketTraffic.Bucket(k)
			namespaceBucket.ForEach(func(k, v []byte) error {
				controllerBucket := namespaceBucket.Bucket(k)
				controllerBucket.ForEach(func(k, v []byte) error {
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
				return nil
			})
			return nil
		})
		// PODS
		bucketPods := tx.Bucket([]byte(POD_STATS_BUCKET_NAME))
		bucketPods.ForEach(func(k, v []byte) error {
			namespaceBucket := bucketPods.Bucket(k)
			namespaceBucket.ForEach(func(k, v []byte) error {
				controllerBucket := namespaceBucket.Bucket(k)
				controllerBucket.ForEach(func(k, v []byte) error {
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
				return nil
			})
			return nil
		})
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
		bucketSockets.ForEach(func(k, v []byte) error {
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
		return nil
	})
	if err != nil {
		log.Errorf("cleanupStats: %s", err.Error())
	}
}

func GetSubBuckets(bucket *bolt.Bucket, bucketNames []string) (*bolt.Bucket, error) {
	path := ""
	for _, v := range bucketNames {
		path += "/" + v
		subBucket := bucket.Bucket([]byte(v))
		if subBucket == nil {
			return nil, fmt.Errorf("Bucket '%s' not found.", path)
		}
		return GetSubBuckets(subBucket, bucketNames[1:])

	}
	return bucket, nil
}
