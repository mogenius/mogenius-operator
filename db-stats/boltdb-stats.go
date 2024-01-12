package dbstats

import (
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"time"

	punqStructs "github.com/mogenius/punq/structs"
	bolt "go.etcd.io/bbolt"
)

const (
	TRAFFIC_BUCKET_NAME   = "traffic-stats"
	POD_STATS_BUCKET_NAME = "pod-stats"
)

const MAX_DATA_POINTS = 100

var dbStats *bolt.DB
var cleanupTimer = time.NewTicker(1 * time.Minute)

func Init() {
	database, err := bolt.Open(utils.CONFIG.Kubernetes.BboltDbStatsPath, 0600, nil)
	if err != nil {
		logger.Log.Errorf("Error opening bbolt database from '%s'.", utils.CONFIG.Kubernetes.BboltDbStatsPath)
		logger.Log.Fatal(err.Error())
	}
	dbStats = database

	// ### TRAFFIC BUCKET ###
	err = dbStats.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(TRAFFIC_BUCKET_NAME))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logger.Log.Errorf("Error creating bucket ('%s'): %s", TRAFFIC_BUCKET_NAME, err)
	}

	// ### STATS BUCKET ###
	err = dbStats.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(POD_STATS_BUCKET_NAME))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logger.Log.Errorf("Error creating bucket ('%s'): %s", POD_STATS_BUCKET_NAME, err)
	}

	logger.Log.Noticef("bbold started 🚀 (Path: '%s')", utils.CONFIG.Kubernetes.BboltDbStatsPath)

	go func() {
		for range cleanupTimer.C {
			cleanupStats()
		}
	}()
}

func AddInterfaceStatsToDb(stats structs.InterfaceStats) {
	stats.CreatedAt = time.Now().Format(time.RFC3339)
	controller := kubernetes.ControllerForPod(stats.Namespace, stats.PodName)
	if controller == nil {
		return
	}
	controllerIdentifier := controller.Kind + "-" + controller.NameSpace + "-" + controller.Name
	err := dbStats.Update(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket([]byte(TRAFFIC_BUCKET_NAME))

		// CREATE A BUCKET FOR EACH POD
		bucket, err := mainBucket.CreateBucketIfNotExists([]byte(controllerIdentifier))
		if err != nil {
			return err
		}

		// DELETE FIRST IF TO MANY DATA POINTS
		if bucket.Stats().KeyN > MAX_DATA_POINTS {
			c := bucket.Cursor()
			k, _ := c.First()
			bucket.Delete(k)
		}

		// add new Entry
		id, _ := bucket.NextSequence() // auto increment
		return bucket.Put([]byte(string(id)), []byte(punqStructs.PrettyPrintString(stats)))
	})
	if err != nil {
		logger.Log.Errorf("Error adding stats for '%s': %s", controllerIdentifier, err.Error())
	}
}

func AddPodStatsToDb(stats structs.PodStats) {
	stats.CreatedAt = time.Now().Format(time.RFC3339)
	controller := kubernetes.ControllerForPod(stats.Namespace, stats.PodName)
	if controller == nil {
		return
	}
	err := dbStats.Update(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket([]byte(POD_STATS_BUCKET_NAME))

		// CREATE A BUCKET FOR EACH POD
		bucket, err := mainBucket.CreateBucketIfNotExists([]byte(controller.Identifier()))
		if err != nil {
			return err
		}

		// DELETE FIRST IF TO MANY DATA POINTS
		if bucket.Stats().KeyN > MAX_DATA_POINTS {
			c := bucket.Cursor()
			k, _ := c.First()
			bucket.Delete(k)
		}

		// add new Entry
		id, _ := bucket.NextSequence() // auto increment
		return bucket.Put([]byte(string(id)), []byte(punqStructs.PrettyPrintString(stats)))
	})
	if err != nil {
		logger.Log.Errorf("Error adding stats for '%s': %s", controller.Identifier(), err.Error())
	}
}

func GetLastTrafficStatsEntryForController(controller kubernetes.K8sController) *structs.InterfaceStats {
	result := &structs.InterfaceStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(TRAFFIC_BUCKET_NAME))
		bucket = bucket.Bucket([]byte(controller.Identifier()))
		c := bucket.Cursor()
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
		logger.Log.Errorf("GetLastTrafficStatsEntryForController: %s", err.Error())
	}
	return result
}

func GetTrafficStatsEntriesForController(controller kubernetes.K8sController) *[]structs.InterfaceStats {
	result := &[]structs.InterfaceStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(TRAFFIC_BUCKET_NAME))
		bucket = bucket.Bucket([]byte(controller.Identifier()))
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
		logger.Log.Errorf("GetTrafficStatsEntriesForController: %s", err.Error())
	}
	return result
}

func GetLastPodStatsEntryForController(controller kubernetes.K8sController) *structs.PodStats {
	result := &structs.PodStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(POD_STATS_BUCKET_NAME))
		bucket = bucket.Bucket([]byte(controller.Identifier()))
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
		logger.Log.Errorf("GetLastPodStatsEntryForController: %s", err.Error())
	}
	return result
}

func GetPodStatsEntriesForController(controller kubernetes.K8sController) *[]structs.PodStats {
	result := &[]structs.PodStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(POD_STATS_BUCKET_NAME))
		bucket = bucket.Bucket([]byte(controller.Identifier()))
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
		logger.Log.Errorf("GetPodStatsEntriesForController: %s", err.Error())
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
					return err
				}
				if isMoreThan14DaysOld(entry.CreatedAt) {
					err := bucketTraffic.DeleteBucket(k)
					if err != nil {
						return err
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
					return err
				}
				if isMoreThan14DaysOld(entry.CreatedAt) {
					err := bucketPods.DeleteBucket(k)
					if err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		logger.Log.Errorf("cleanupStats: %s", err.Error())
	}
}
