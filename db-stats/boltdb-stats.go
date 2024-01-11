package dbstats

import (
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"time"

	punqStructs "github.com/mogenius/punq/structs"
	bolt "go.etcd.io/bbolt"
)

const (
	TRAFFIC_BUCKET_NAME   = "mogenius-traffic"
	POD_STATS_BUCKET_NAME = "mogenius-scans"
)

const MAX_DATA_POINTS = 10000

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
	err := dbStats.Update(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket([]byte(TRAFFIC_BUCKET_NAME))

		// CREATE A BUCKET FOR EACH POD
		bucket, err := mainBucket.CreateBucketIfNotExists([]byte(stats.PodName))
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
		logger.Log.Errorf("Error adding stats for '%s': %s", stats.PodName, err.Error())
	}
}

func GetLastEntryForPodName(podName string) *structs.InterfaceStats {
	result := &structs.InterfaceStats{}
	err := dbStats.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(TRAFFIC_BUCKET_NAME))
		bucket = bucket.Bucket([]byte(podName))
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
		logger.Log.Errorf("GetLastEntryForPodName: %s", err.Error())
	}
	return result
}

func cleanupStats() {
	err := dbStats.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(TRAFFIC_BUCKET_NAME))
		c := bucket.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			entry := GetLastEntryForPodName(string(k))
			if isMoreThan14DaysOld(entry.CreatedAt) {
				err := bucket.DeleteBucket(k)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		logger.Log.Errorf("cleanupStats: %s", err.Error())
	}
}

// func GetJobsToBuildFromDb() []structs.BuildJob {
// 	result := []structs.BuildJob{}
// 	err := db.View(func(tx *bolt.Tx) error {
// 		c := tx.Bucket([]byte(BUILD_BUCKET_NAME)).Cursor()
// 		prefix := []byte(PREFIX_QUEUE)
// 		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
// 			job := structs.BuildJob{}
// 			err := structs.UnmarshalJob(&job, jobData)
// 			if err == nil {
// 				if job.State == structs.BuildJobStatePending {
// 					result = append(result, job)
// 				}
// 			} else {
// 				logger.Log.Errorf("ProcessQueue (unmarshall) ERR: %s", err.Error())
// 			}
// 		}
// 		return nil
// 	})
// 	if err != nil {
// 		logger.Log.Errorf("GetJobsToBuildFromDb (db) ERR: %s", err.Error())
// 	}
// 	return result
// }

// func GetScannedImageFromCache(req structs.ScanImageRequest) (structs.BuildJobInfoEntry, error) {
// 	entry := structs.CreateBuildJobInfoEntryFromScanImageReq(req)
// 	err := db.View(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(SCAN_BUCKET_NAME))
// 		rawData := string(bucket.Get([]byte(fmt.Sprintf("%s%s", PREFIX_VUL_SCAN, req.ContainerImage))))

// 		// FOUND SOMETHING IN BOLT DB, SEND IT TO SERVER
// 		if rawData != "" {
// 			err := structs.UnmarshalBuildJobInfoEntry(&entry, []byte(rawData))
// 			if err == nil && !isMoreThan24HoursAgo(entry.StartTime) {
// 				return nil
// 			}
// 		}
// 		return fmt.Errorf("Not cached data found in bold db for %s. Starting scan ...", req.ContainerImage)
// 	})
// 	return entry, err
// }

// func StartScanInCache(data structs.BuildJobInfoEntry, imageName string) {
// 	// FIRST CREATE A DB ENTRY TO AVOID MULTIPLE SCANS
// 	db.Update(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(SCAN_BUCKET_NAME))

// 		var json = jsoniter.ConfigCompatibleWithStandardLibrary
// 		bytes, err := json.Marshal(data)
// 		if err != nil {
// 			logger.Log.Errorf("Error %s: %s", PREFIX_VUL_SCAN, err.Error())
// 		}
// 		bucket.Put([]byte(fmt.Sprintf("%s%s", PREFIX_VUL_SCAN, imageName)), bytes)
// 		return nil
// 	})
// }

// func GetBuilderStatus() structs.BuilderStatus {
// 	result := structs.BuilderStatus{}

// 	err := db.View(func(tx *bolt.Tx) error {
// 		cursorBuild := tx.Bucket([]byte(BUILD_BUCKET_NAME)).Cursor()
// 		prefix := []byte(PREFIX_QUEUE)
// 		for k, jobData := cursorBuild.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = cursorBuild.Next() {
// 			job := structs.BuildJob{}
// 			err := structs.UnmarshalJob(&job, jobData)
// 			if err == nil {
// 				result.TotalBuilds++
// 				result.TotalBuildTimeMs += job.DurationMs
// 				if job.State == structs.BuildJobStatePending {
// 					result.QueuedBuilds++
// 				}
// 				if job.State == structs.BuildJobStateFailed {
// 					result.FailedBuilds++
// 				}
// 				if job.State == structs.BuildJobStateCanceled {
// 					result.CanceledBuilds++
// 				}
// 				if job.State == structs.BuildJobStateSucceeded {
// 					result.FinishedBuilds++
// 				}
// 			}
// 		}
// 		cursorScan := tx.Bucket([]byte(SCAN_BUCKET_NAME)).Cursor()
// 		prefixScan := []byte(PREFIX_VUL_SCAN)
// 		for k, jobData := cursorScan.Seek(prefixScan); k != nil && bytes.HasPrefix(k, prefixScan); k, jobData = cursorScan.Next() {
// 			scan := structs.BuildScanResult{}
// 			err := structs.UnmarshalScan(&scan, jobData)
// 			if err == nil {
// 				result.TotalScans++
// 			}
// 		}

// 		return nil
// 	})
// 	if err != nil {
// 		logger.Log.Errorf("GetBuilderStatus (db) ERR: %s", err.Error())
// 	}
// 	return result
// }

// func GetBuildJobInfosFromDb(buildId int) structs.BuildJobInfos {
// 	result := structs.BuildJobInfos{}
// 	err := db.View(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))

// 		queueEntry := bucket.Get([]byte(fmt.Sprintf("%s%d", PREFIX_QUEUE, buildId)))
// 		job := structs.BuildJob{}
// 		err := structs.UnmarshalJob(&job, queueEntry)
// 		if err != nil {
// 			return err
// 		}

// 		clone := bucket.Get([]byte(fmt.Sprintf("%s%d", PREFIX_GIT_CLONE, buildId)))
// 		ls := bucket.Get([]byte(fmt.Sprintf("%s%d", PREFIX_LS, buildId)))
// 		login := bucket.Get([]byte(fmt.Sprintf("%s%d", PREFIX_LOGIN, buildId)))
// 		build := bucket.Get([]byte(fmt.Sprintf("%s%d", PREFIX_BUILD, buildId)))
// 		push := bucket.Get([]byte(fmt.Sprintf("%s%d", PREFIX_PUSH, buildId)))
// 		result = structs.CreateBuildJobInfos(job, clone, ls, login, build, push)
// 		return nil
// 	})
// 	if err != nil {
// 		logger.Log.Errorf("GetBuildJobFromDb (db) ERR: %s", err.Error())
// 	}

// 	return result
// }

// func GetBuildJobListFromDb() []structs.BuildJobListEntry {
// 	result := []structs.BuildJobListEntry{}
// 	err := db.View(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
// 		c := bucket.Cursor()
// 		prefix := []byte(PREFIX_QUEUE)
// 		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
// 			job := structs.BuildJobListEntry{}
// 			err := structs.UnmarshalJobListEntry(&job, jobData)
// 			if err != nil {
// 				return err
// 			}
// 			result = append(result, job)
// 		}
// 		return nil
// 	})
// 	if err != nil {
// 		logger.Log.Errorf("GetBuildJobListFromDb: %s", err.Error())
// 	}
// 	return result
// }

// func UpdateStateInDb(buildJob structs.BuildJob, newState structs.BuildJobStateEnum) {
// 	err := db.Update(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
// 		jobData := bucket.Get([]byte(fmt.Sprintf("%s%d", PREFIX_QUEUE, buildJob.BuildId)))
// 		job := structs.BuildJob{}
// 		err := structs.UnmarshalJob(&job, jobData)
// 		if err == nil {
// 			job.State = newState
// 			return bucket.Put([]byte(fmt.Sprintf("%s%d", PREFIX_QUEUE, buildJob.BuildId)), []byte(punqStructs.PrettyPrintString(job)))
// 		}
// 		return err
// 	})
// 	if err != nil {
// 		errStr := fmt.Sprintf("Error updating state for build '%d'. REASON: %s", buildJob.BuildId, err.Error())
// 		logger.Log.Error(errStr)
// 	}
// 	logger.Log.Infof(fmt.Sprintf("State for build '%d' updated successfuly to '%s'.", buildJob.BuildId, newState))
// }

// func PositionInQueueFromDb(buildId int) int {
// 	positionInQueue := 0

// 	err := db.View(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))

// 		// FIRST: CHECK FOR DUPLICATES
// 		c := bucket.Cursor()
// 		prefix := []byte(PREFIX_QUEUE)
// 		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
// 			job := structs.BuildJob{}
// 			err := structs.UnmarshalJob(&job, jobData)
// 			if err == nil {
// 				if job.State == structs.BuildJobStatePending && job.BuildId != buildId {
// 					positionInQueue++
// 				}
// 			}
// 		}
// 		return nil
// 	})
// 	if err != nil {
// 		return -1
// 	}

// 	return positionInQueue
// }

// func SaveJobInDb(buildJob structs.BuildJob) {
// 	err := db.Update(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
// 		return bucket.Put([]byte(fmt.Sprintf("%s%d", PREFIX_QUEUE, buildJob.BuildId)), []byte(punqStructs.PrettyPrintString(buildJob)))
// 	})
// 	if err != nil {
// 		logger.Log.Errorf("Error saving job '%d'.", buildJob.BuildId)
// 	}
// }

// func PrintAllEntriesFromDb(bucket string, prefix string) {
// 	err := db.View(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(bucket))
// 		c := bucket.Cursor()
// 		prefix := []byte(prefix)
// 		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
// 			job := structs.BuildJob{}
// 			err := structs.UnmarshalJob(&job, jobData)
// 			if err != nil {
// 				logger.Log.Noticef("bucket=%s, key=%s, value=%s\n", bucket, k, job.BuildId)
// 			}
// 		}
// 		return nil
// 	})
// 	if err != nil {
// 		logger.Log.Errorf("printAllEntries: %s", err.Error())
// 	}
// }

// func DeleteFromDb(bucket string, prefix string, buildNo int) error {
// 	return db.Update(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(bucket))
// 		return bucket.Delete([]byte(fmt.Sprintf("%s%d", PREFIX_QUEUE, buildNo)))
// 	})
// }

// func AddToDb(buildJob structs.BuildJob) (int, error) {
// 	var nextBuildId uint64 = 0
// 	err := db.Update(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
// 		nextBuildId, _ = bucket.NextSequence() // auto increment

// 		// FIRST: CHECK FOR DUPLICATES
// 		c := bucket.Cursor()
// 		prefix := []byte(PREFIX_QUEUE)
// 		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
// 			job := structs.BuildJob{}
// 			err := structs.UnmarshalJob(&job, jobData)
// 			if err == nil {
// 				// THIS IS A FILTER TO HANDLE DUPLICATED REQUESTS
// 				// if job.GitCommitHash == buildJob.GitCommitHash {
// 				// 	err = fmt.Errorf("Duplicate BuildJob '%s (%s)' found. Not adding to Queue.", job.ServiceName, job.GitCommitHash)
// 				// 	logger.Log.Error(err.Error())
// 				// 	return err
// 				// }
// 			}
// 		}
// 		buildJob.BuildId = int(nextBuildId)
// 		return bucket.Put([]byte(fmt.Sprintf("%s%d", PREFIX_QUEUE, nextBuildId)), []byte(punqStructs.PrettyPrintString(buildJob)))
// 	})
// 	return int(nextBuildId), err
// }

// func SaveScanResult(state structs.BuildJobStateEnum, cmdOutput string, startTime time.Time, containerImageName string, job *structs.BuildJob) error {
// 	err := db.Update(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(SCAN_BUCKET_NAME))
// 		entry := structs.CreateBuildJobInfoEntryBytes(state, cmdOutput, startTime, time.Now(), job)
// 		return bucket.Put([]byte(fmt.Sprintf("%s%s", PREFIX_VUL_SCAN, containerImageName)), entry)
// 	})
// 	if err != nil {
// 		logger.Log.Errorf("Error saving scan result for '%s'.", containerImageName)
// 	}
// 	return err
// }

// func SaveBuildResult(state structs.BuildJobStateEnum, prefix string, cmdOutput string, startTime time.Time, job *structs.BuildJob) error {
// 	err := db.Update(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
// 		entry := structs.CreateBuildJobInfoEntryBytes(state, cmdOutput, startTime, time.Now(), job)
// 		return bucket.Put([]byte(fmt.Sprintf("%s%d", prefix, job.BuildId)), entry)
// 	})
// 	if err != nil {
// 		logger.Log.Errorf("Error saving build result for '%d'.", job.BuildId)
// 	}
// 	return err
// }

// func AddLogToDb(title string, message string, category structs.Category, logType structs.LogType) error {
// 	err := db.Update(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(LOG_BUCKET_NAME))
// 		id, _ := bucket.NextSequence() // auto increment
// 		entry := structs.CreateLog(id, title, message, category, logType)
// 		return bucket.Put([]byte(fmt.Sprintf("%s_%s_%s", entry.CreatedAt, entry.Category, entry.Type)), structs.LogBytes(entry))
// 	})
// 	if err != nil {
// 		logger.Log.Errorf("Error adding log for '%s'.", title)
// 	}
// 	return err
// }

// func ListLogFromDb() []structs.Log {
// 	result := []structs.Log{}
// 	err := db.View(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(LOG_BUCKET_NAME))
// 		c := bucket.Cursor()
// 		prefix := []byte("")
// 		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
// 			entry := structs.Log{}
// 			err := structs.UnmarshalLog(&entry, jobData)
// 			if err != nil {
// 				return err
// 			}
// 			result = append(result, entry)
// 		}
// 		return nil
// 	})
// 	if err != nil {
// 		logger.Log.Errorf("ListLog: %s", err.Error())
// 	}
// 	return result
// }

// func AddMigrationToDb(name string) error {
// 	err := db.Update(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(MIGRATION_BUCKET_NAME))
// 		id, _ := bucket.NextSequence() // auto increment
// 		entry := structs.CreateMigration(id, name)
// 		return bucket.Put([]byte(entry.Name), structs.MigrationBytes(entry))
// 	})
// 	if err != nil {
// 		logger.Log.Errorf("Error adding migration '%s'.", name)
// 	}
// 	return err
// }

// func IsMigrationAlreadyApplied(name string) bool {
// 	err := db.View(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(MIGRATION_BUCKET_NAME))
// 		rawData := bucket.Get([]byte(name))
// 		if len(rawData) > 0 {
// 			return nil
// 		}
// 		return fmt.Errorf("Not migration found for name '%s'.", name)
// 	})
// 	return err == nil
// }

// func AppendToKey(bucket string, key string, value string) error {
// 	err := db.Update(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(bucket))
// 		rawData := bucket.Get([]byte(key))
// 		if len(rawData) > 0 {
// 			if len(rawData) > MAX_ENTRY_LENGTH {
// 				return fmt.Errorf("Entry for key '%s' is too long (%d is the limit).", key, MAX_ENTRY_LENGTH)
// 			}
// 			rawData = append(rawData, []byte(value)...)
// 			return bucket.Put([]byte(key), rawData)
// 		}
// 		return fmt.Errorf("Not key found for name '%s'.", key)
// 	})
// 	return err
// }
