package dbstats

import (
	"fmt"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"time"

	log "github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

const (
	TRAFFIC_BUCKET_NAME    = "traffic-stats"
	POD_STATS_BUCKET_NAME  = "pod-stats"
	NODE_STATS_BUCKET_NAME = "node-stats"
)

var dbStats *bolt.DB
var cleanupTimer = time.NewTicker(1 * time.Minute)

func Init() {
	database, err := bolt.Open(utils.CONFIG.Kubernetes.BboltDbStatsPath, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		log.Errorf("Error opening bbolt database from '%s'.", utils.CONFIG.Kubernetes.BboltDbStatsPath)
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

	log.Infof("bbold started 🚀 (Path: '%s')", utils.CONFIG.Kubernetes.BboltDbStatsPath)

	go func() {
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
		if controllerBucket.Stats().KeyN > utils.CONFIG.Builder.MaxDataPoints {
			c := controllerBucket.Cursor()
			k, _ := c.First()
			controllerBucket.Delete(k)
		}

		// add new Entry
		id, _ := controllerBucket.NextSequence() // auto increment
		return controllerBucket.Put(sequenceToKey(id), []byte(utils.PrettyPrintInterface(stats)))
	})
	if err != nil {
		log.Errorf("Error adding interface stats for '%s': %s", stats.Namespace, err.Error())
	}
}

func sequenceToKey(id uint64) []byte {
	return []byte(fmt.Sprintf("%020d", id))
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
		if nodeBucket.Stats().KeyN > utils.CONFIG.Builder.MaxDataPoints {
			c := nodeBucket.Cursor()
			k, _ := c.First()
			nodeBucket.Delete(k)
		}

		// add new Entry
		id, _ := nodeBucket.NextSequence() // auto increment
		return nodeBucket.Put(sequenceToKey(id), []byte(utils.PrettyPrintInterface(stats)))
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
		if controllerBucket.Stats().KeyN > utils.CONFIG.Builder.MaxDataPoints {
			c := controllerBucket.Cursor()
			k, _ := c.First()
			controllerBucket.Delete(k)
		}

		// add new Entry
		id, _ := controllerBucket.NextSequence() // auto increment
		return controllerBucket.Put(sequenceToKey(id), []byte(utils.PrettyPrintInterface(stats)))
	})
	if err != nil {
		log.Errorf("Error adding pod stats for '%s': %s", stats.Namespace, err.Error())
	}
}

func GetLastTrafficStatsEntryForController(controller kubernetes.K8sController) *structs.InterfaceStats {
	result := &structs.InterfaceStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket([]byte(TRAFFIC_BUCKET_NAME))
		if mainBucket == nil {
			return fmt.Errorf("Bucket '%s' not found.", TRAFFIC_BUCKET_NAME)
		}
		namespaceBucket := mainBucket.Bucket([]byte(controller.Namespace))
		if namespaceBucket == nil {
			return fmt.Errorf("Bucket '%s' not found.", controller.Namespace)
		}

		controllerBucket := namespaceBucket.Bucket([]byte(controller.Name))

		c := controllerBucket.Cursor()
		k, v := c.Last()
		if k != nil {
			err := structs.UnmarshalInterfaceStats(result, v)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Errorf("GetLastTrafficStatsEntryForController: %s", err.Error())
	}
	return result
}

func GetTrafficStatsEntriesForController(controller kubernetes.K8sController) *[]structs.InterfaceStats {
	result := &[]structs.InterfaceStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket([]byte(TRAFFIC_BUCKET_NAME))
		if mainBucket == nil {
			return fmt.Errorf("Bucket '%s' not found.", TRAFFIC_BUCKET_NAME)
		}
		namespaceBucket := mainBucket.Bucket([]byte(controller.Namespace))
		if namespaceBucket == nil {
			return fmt.Errorf("Bucket '%s' not found.", controller.Namespace)
		}

		controllerBucket := namespaceBucket.Bucket([]byte(controller.Name))

		return controllerBucket.ForEach(func(k, v []byte) error {
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
		mainBucket := tx.Bucket([]byte(POD_STATS_BUCKET_NAME))
		if mainBucket == nil {
			return fmt.Errorf("Bucket '%s' not found.", POD_STATS_BUCKET_NAME)
		}
		namespaceBucket := mainBucket.Bucket([]byte(controller.Namespace))
		if namespaceBucket == nil {
			return fmt.Errorf("Bucket '%s' not found.", controller.Namespace)
		}

		controllerBucket := namespaceBucket.Bucket([]byte(controller.Name))

		c := controllerBucket.Cursor()
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
		mainBucket := tx.Bucket([]byte(POD_STATS_BUCKET_NAME))
		if mainBucket == nil {
			return fmt.Errorf("Bucket '%s' not found.", POD_STATS_BUCKET_NAME)
		}
		namespaceBucket := mainBucket.Bucket([]byte(controller.Namespace))
		if namespaceBucket == nil {
			return fmt.Errorf("Bucket '%s' not found.", controller.Namespace)
		}

		controllerBucket := namespaceBucket.Bucket([]byte(controller.Name))

		return controllerBucket.ForEach(func(k, v []byte) error {
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
		mainBucket := tx.Bucket([]byte(POD_STATS_BUCKET_NAME))
		if mainBucket == nil {
			return fmt.Errorf("Bucket '%s' not found.", POD_STATS_BUCKET_NAME)
		}
		namespaceBucket := mainBucket.Bucket([]byte(namespace))
		if namespaceBucket == nil {
			return fmt.Errorf("Bucket '%s' not found.", namespace)
		}
		return namespaceBucket.ForEach(func(k, v []byte) error {
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
		mainBucket := tx.Bucket([]byte(POD_STATS_BUCKET_NAME))
		if mainBucket == nil {
			return fmt.Errorf("Bucket '%s' not found.", POD_STATS_BUCKET_NAME)
		}
		namespaceBucket := mainBucket.Bucket([]byte(namespace))
		if namespaceBucket == nil {
			return fmt.Errorf("Bucket '%s' not found.", namespace)
		}
		return namespaceBucket.ForEach(func(k, v []byte) error {
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

func GetLastTrafficStatsEntriesForNamespace(namespace string) []structs.InterfaceStats {
	result := []structs.InterfaceStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket([]byte(TRAFFIC_BUCKET_NAME))
		if mainBucket == nil {
			return fmt.Errorf("Bucket '%s' not found.", TRAFFIC_BUCKET_NAME)
		}
		namespaceBucket := mainBucket.Bucket([]byte(namespace))
		if namespaceBucket == nil {
			return fmt.Errorf("Bucket '%s' not found.", namespace)
		}

		c := namespaceBucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			entry := structs.InterfaceStats{}
			err := structs.UnmarshalInterfaceStats(&entry, v)
			if err != nil {
				return err
			}
			var newEntry bool = true
			for i := 0; i < len(result); i++ {
				if entry.PodName == result[i].PodName {
					newEntry = false

					if isFirstTimestampNewer(entry.CreatedAt, result[i].CreatedAt) {
						result[i] = entry
					}
					break
				}
			}
			if newEntry {
				result = append(result, entry)
			}
		}
		return nil
	})
	if err != nil {
		log.Errorf("GetLastPodStatsEntriesForNamespace: %s", err.Error())
	}
	return result
}

func GetTrafficStatsEntriesForNamespace(namespace string) *[]structs.InterfaceStats {
	result := &[]structs.InterfaceStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket([]byte(TRAFFIC_BUCKET_NAME))
		if mainBucket == nil {
			return fmt.Errorf("Bucket '%s' not found.", TRAFFIC_BUCKET_NAME)
		}
		namespaceBucket := mainBucket.Bucket([]byte(namespace))
		if namespaceBucket == nil {
			return fmt.Errorf("Bucket '%s' not found.", namespace)
		}
		return namespaceBucket.ForEach(func(k, v []byte) error {
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
		c := bucketTraffic.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			subBucket := bucketTraffic.Bucket(k)
			for kSub, _ := subBucket.Cursor().First(); kSub != nil; kSub, _ = subBucket.Cursor().Next() {
				entry := structs.InterfaceStats{}
				err := structs.UnmarshalInterfaceStats(&entry, subBucket.Get(kSub))
				if err != nil {
					return fmt.Errorf("cleanupStatsTraffic: %s", err.Error())
				}
				if isMoreThan14DaysOld(entry.CreatedAt) {
					err := bucketTraffic.DeleteBucket(k)
					if err != nil {
						return fmt.Errorf("cleanupStatsTraffic: %s", err.Error())
					}
				}
			}
		}
		// PODS
		bucketPods := tx.Bucket([]byte(POD_STATS_BUCKET_NAME))
		c = bucketPods.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			subBucket := bucketPods.Bucket(k)
			for kSub, _ := subBucket.Cursor().First(); kSub != nil; kSub, _ = subBucket.Cursor().Next() {
				entry := structs.PodStats{}
				err := structs.UnmarshalPodStats(&entry, subBucket.Get(kSub))
				if err != nil {
					return fmt.Errorf("cleanupStatsPods: %s", err.Error())
				}
				if isMoreThan14DaysOld(entry.CreatedAt) {
					err := bucketPods.DeleteBucket(k)
					if err != nil {
						return fmt.Errorf("cleanupStatsPods: %s", err.Error())
					}
				}
			}
		}
		// Nodes
		bucketNodes := tx.Bucket([]byte(NODE_STATS_BUCKET_NAME))
		c = bucketNodes.Cursor()
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
		return nil
	})
	if err != nil {
		log.Errorf("cleanupStats: %s", err.Error())
	}
}

func isFirstTimestampNewer(ts1, ts2 string) bool {
	// Parse the timestamps using RFC 3339 format
	t1, err := time.Parse(time.RFC3339, ts1)
	if err != nil {
		log.Error(fmt.Errorf("error parsing ts1: %w", err))
	}

	t2, err := time.Parse(time.RFC3339, ts2)
	if err != nil {
		log.Error(fmt.Errorf("error parsing ts2: %w", err))
	}

	// Check if the first timestamp is strictly newer than the second
	return t1.After(t2)
}
