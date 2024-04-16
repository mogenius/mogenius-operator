package db

import (
	"bytes"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"strconv"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	punqStructs "github.com/mogenius/punq/structs"
	punqUtils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

const (
	DB_SCHEMA_VERSION = "1"
)

const (
	BUILD_BUCKET_NAME     = "mogenius-builds"
	SCAN_BUCKET_NAME      = "mogenius-scans"
	LOG_BUCKET_NAME       = "mogenius-logs"
	MIGRATION_BUCKET_NAME = "mogenius-migrations"

	// PREFIX_GIT_CLONE = "clone"
	// PREFIX_LS        = "ls"
	// PREFIX_LOGIN     = "login"
	//PREFIX_BUILD     = "build"
	//PREFIX_PULL      = "pull"
	//PREFIX_PUSH  = "push"
	PREFIX_QUEUE = "queue"

	PREFIX_VUL_SCAN = "scan"

	PREFIX_CLEANUP = "cleanup"

	MAX_ENTRY_LENGTH = 1024 * 1024 * 50 // 50 MB
)

var db *bolt.DB

func Init() {
	dbPath := strings.ReplaceAll(utils.CONFIG.Kubernetes.BboltDbPath, ".db", fmt.Sprintf("-%s.db", DB_SCHEMA_VERSION))
	database, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		log.Errorf("Error opening bbolt database from '%s'.", dbPath)
		log.Fatal(err.Error())
	}
	// ### BUILD BUCKET ###
	db = database
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BUILD_BUCKET_NAME))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Errorf("Error creating bucket ('%s'): %s", BUILD_BUCKET_NAME, err)
	}
	// ### SCAN BUCKET ### create a new scan bucket on every startup
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(SCAN_BUCKET_NAME))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		err := db.Update(func(tx *bolt.Tx) error {
			err := tx.DeleteBucket([]byte(SCAN_BUCKET_NAME))
			if err != nil {
				return err
			}
			_, err = tx.CreateBucket([]byte(SCAN_BUCKET_NAME))
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			log.Errorf("Error recreating bucket ('%s'): %s", SCAN_BUCKET_NAME, err)
		}
	}
	// ### LOG BUCKET ###
	db = database
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(LOG_BUCKET_NAME))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Errorf("Error creating bucket ('%s'): %s", LOG_BUCKET_NAME, err)
	}

	// ### MIGRATION BUCKET ###
	db = database
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(MIGRATION_BUCKET_NAME))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Errorf("Error creating bucket ('%s'): %s", MIGRATION_BUCKET_NAME, err)
	}

	// RESET STARTED JOBS TO PENDING
	resetStartedJobsToPendingOnInit()

	log.Infof("bbold started 🚀 (Path: '%s')", dbPath)
}

func Close() {
	if db != nil {
		db.Close()
	}
}

// if a job was started and the server was restarted/crashed, we need to reset the state to pending to resume the builld
func resetStartedJobsToPendingOnInit() {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		c := bucket.Cursor()
		prefix := []byte(PREFIX_QUEUE)
		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
			job := structs.BuildJob{}
			err := structs.UnmarshalJob(&job, jobData)
			if err != nil {
				log.Errorf("Init (unmarshall) ERR: %s", err.Error())
				continue
			}
			if job.State == punqStructs.JobStateStarted {
				job.State = punqStructs.JobStatePending
				key := fmt.Sprintf("%s-%s", PREFIX_QUEUE, utils.SequenceToKey(job.BuildId))
				err := bucket.Put([]byte(key), []byte(punqStructs.PrettyPrintString(job)))
				if err != nil {
					log.Errorf("Init (update) ERR: %s", err.Error())
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Errorf("Init (db) ERR: %s", err.Error())
	}
}

func GetJobsToBuildFromDb() []structs.BuildJob {
	result := []structs.BuildJob{}
	err := db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(BUILD_BUCKET_NAME)).Cursor()
		prefix := []byte(PREFIX_QUEUE)
		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
			job := structs.BuildJob{}
			err := structs.UnmarshalJob(&job, jobData)
			if err == nil {
				if job.State == punqStructs.JobStatePending {
					result = append(result, job)
				}
			} else {
				log.Errorf("ProcessQueue (unmarshall) ERR: %s", err.Error())
			}
		}
		return nil
	})
	if err != nil {
		log.Errorf("GetJobsToBuildFromDb (db) ERR: %s", err.Error())
	}
	return result
}

func GetScannedImageFromCache(req structs.ScanImageRequest) (structs.BuildJobInfoEntry, error) {
	entry := structs.CreateBuildJobInfoEntryFromScanImageReq(req)
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(SCAN_BUCKET_NAME))
		rawData := string(bucket.Get([]byte(fmt.Sprintf("%s%s", PREFIX_VUL_SCAN, req.ContainerImage))))

		// FOUND SOMETHING IN BOLT DB, SEND IT TO SERVER
		if rawData != "" {
			err := structs.UnmarshalBuildJobInfoEntry(&entry, []byte(rawData))
			if err == nil && !isMoreThan24HoursAgo(entry.StartTime) {
				return nil
			}
		}
		return fmt.Errorf("Not cached data found in bold db for %s. Starting scan ...", req.ContainerImage)
	})
	return entry, err
}

func StartScanInCache(data structs.BuildJobInfoEntry, imageName string) {
	// FIRST CREATE A DB ENTRY TO AVOID MULTIPLE SCANS
	db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(SCAN_BUCKET_NAME))

		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		bytes, err := json.Marshal(data)
		if err != nil {
			log.Errorf("Error %s: %s", PREFIX_VUL_SCAN, err.Error())
		}
		bucket.Put([]byte(fmt.Sprintf("%s%s", PREFIX_VUL_SCAN, imageName)), bytes)
		return nil
	})
}

func GetBuilderStatus() structs.BuilderStatus {
	result := structs.BuilderStatus{}

	err := db.View(func(tx *bolt.Tx) error {
		cursorBuild := tx.Bucket([]byte(BUILD_BUCKET_NAME)).Cursor()
		prefix := []byte(PREFIX_QUEUE)
		for k, jobData := cursorBuild.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = cursorBuild.Next() {
			job := structs.BuildJob{}
			err := structs.UnmarshalJob(&job, jobData)
			if err == nil {
				result.TotalBuilds++
				result.TotalBuildTimeMs += job.DurationMs
				if job.State == punqStructs.JobStatePending {
					result.QueuedBuilds++
				}
				if job.State == punqStructs.JobStateFailed {
					result.FailedBuilds++
				}
				if job.State == punqStructs.JobStateCanceled {
					result.CanceledBuilds++
				}
				if job.State == punqStructs.JobStateSucceeded {
					result.FinishedBuilds++
				}
			}
		}
		cursorScan := tx.Bucket([]byte(SCAN_BUCKET_NAME)).Cursor()
		prefixScan := []byte(PREFIX_VUL_SCAN)
		for k, jobData := cursorScan.Seek(prefixScan); k != nil && bytes.HasPrefix(k, prefixScan); k, jobData = cursorScan.Next() {
			scan := structs.BuildScanResult{}
			err := structs.UnmarshalScan(&scan, jobData)
			if err == nil {
				result.TotalScans++
			}
		}

		return nil
	})
	if err != nil {
		log.Errorf("GetBuilderStatus (db) ERR: %s", err.Error())
	}
	return result
}

func GetLastBuildJobInfosFromDb(data structs.LastBuildTaskListRequest) structs.BuildJobInfos {
	result := structs.BuildJobInfos{}
	err := db.View(func(tx *bolt.Tx) error {

		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		cursorBuild := bucket.Cursor()

		suffix := structs.LastBuildJobInfosKeySuffix(data.Namespace, data.Controller, data.Container)
		var lastBuildKey string
		for k, _ := cursorBuild.Last(); k != nil; k, _ = cursorBuild.Prev() {
			if strings.HasSuffix(string(k), suffix) {
				lastBuildKey = string(k)
				break
			}
		}

		parts := strings.Split(lastBuildKey, "___")
		if len(parts) >= 3 {
			buildId := parts[1]
			lastBuildId, err := strconv.ParseUint(buildId, 10, 64)
			if err != nil {
				log.Errorf("GetLastBuildJobInfosFromDb: %s", err.Error())
				return err
			}
			result = GetBuildJobInfosFromDb(lastBuildId)
		}
		return nil
	})
	if err != nil {
		log.Errorf("GetBuildJobListFromDb: %s", err.Error())
	}
	return result

}
func GetBuildJobInfosFromDb(buildId uint64) structs.BuildJobInfos {
	result := structs.BuildJobInfos{}
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		cursorBuild := bucket.Cursor()

		//key := fmt.Sprintf("%s-%s", PREFIX_QUEUE, utils.SequenceToKey(buildId))
		//queueEntry := bucket.Get([]byte(key))
		//job := structs.BuildJob{}
		//err := structs.UnmarshalJob(&job, queueEntry)
		//if err != nil {
		//	return err
		//}
		//for k, value := cursorBuild.Last(); k != nil; k, _ = cursorBuild.Prev() {
		//	if strings.Contains(string(k), fmt.Sprintf("___%s___", id)) {
		//		log.Info(value)
		//		break
		//	}
		//}
		//
		//if namespace == "" || controller == "" || container == "" {
		//	return fmt.Errorf("Not build found for id '%d'.", buildId)
		//}

		namespace := ""
		controller := ""
		container := ""

		prefix := structs.GetBuildJobInfosPrefix(structs.PrefixBuild, buildId)
		for k, _ := cursorBuild.Last(); k != nil; k, _ = cursorBuild.Prev() {
			if strings.HasPrefix(string(k), prefix) {
				buildTemp := structs.CreateBuildJobEntryFromData(bucket.Get(k))
				namespace = buildTemp.Namespace
				controller = buildTemp.Controller
				container = buildTemp.Container
				break
			}
		}

		clone := bucket.Get([]byte(structs.BuildJobInfoEntryKey(structs.PrefixGitClone, buildId, namespace, controller, container)))
		ls := bucket.Get([]byte(structs.BuildJobInfoEntryKey(structs.PrefixLs, buildId, namespace, controller, container)))
		login := bucket.Get([]byte(structs.BuildJobInfoEntryKey(structs.PrefixLogin, buildId, namespace, controller, container)))
		build := bucket.Get([]byte(structs.BuildJobInfoEntryKey(structs.PrefixBuild, buildId, namespace, controller, container)))
		push := bucket.Get([]byte(structs.BuildJobInfoEntryKey(structs.PrefixPush, buildId, namespace, controller, container)))
		result = structs.CreateBuildJobInfos(clone, ls, login, build, push)
		return nil
	})
	if err != nil {
		log.Errorf("GetBuildJobFromDb (db) ERR: %s", err.Error())
	}

	return result
}

func GetItemByKey(key string) []byte {
	rawData := []byte{}
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		rawData = bucket.Get([]byte(key))
		return nil
	})
	if err != nil {
		log.Errorf("GetBuilderStatus (db) ERR: %s", err.Error())
	}
	return rawData
}

func GetBuildJobListFromDb() []structs.BuildJob {
	result := []structs.BuildJob{}
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		c := bucket.Cursor()
		prefix := []byte(PREFIX_QUEUE)
		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
			job := structs.BuildJob{}
			err := structs.UnmarshalJobListEntry(&job, jobData)
			if err != nil {
				return err
			}
			result = append(result, job)
		}
		// sort result array by buildId
		for resultIndex := range result {
			for i := range result {
				if result[resultIndex].BuildId < result[i].BuildId {
					result[resultIndex], result[i] = result[i], result[resultIndex]
				}
			}
		}

		return nil
	})
	if err != nil {
		log.Errorf("GetBuildJobListFromDb: %s", err.Error())
	}
	return result
}

func UpdateStateInDb(buildJob structs.BuildJob, newState punqStructs.JobStateEnum) {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		key := fmt.Sprintf("%s-%s", PREFIX_QUEUE, utils.SequenceToKey(buildJob.BuildId))
		jobData := bucket.Get([]byte(key))
		job := structs.BuildJob{}
		err := structs.UnmarshalJob(&job, jobData)
		if err == nil {
			job.State = newState
			return bucket.Put([]byte(key), []byte(punqStructs.PrettyPrintString(job)))
		}
		return err
	})
	if err != nil {
		errStr := fmt.Sprintf("Error updating state for build '%d'. REASON: %s", buildJob.BuildId, err.Error())
		log.Error(errStr)
	}
	log.Infof(fmt.Sprintf("State for build '%d' updated successfuly to '%s'.", buildJob.BuildId, newState))
}

func PositionInQueueFromDb(buildId uint64) int {
	positionInQueue := 0

	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))

		// FIRST: CHECK FOR DUPLICATES
		c := bucket.Cursor()
		prefix := []byte(PREFIX_QUEUE)
		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
			job := structs.BuildJob{}
			err := structs.UnmarshalJob(&job, jobData)
			if err == nil {
				if job.State == punqStructs.JobStatePending && job.BuildId != buildId {
					positionInQueue++
				}
			}
		}
		return nil
	})
	if err != nil {
		return -1
	}

	return positionInQueue
}

func SaveJobInDb(buildJob structs.BuildJob) {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		key := fmt.Sprintf("%s-%s", PREFIX_QUEUE, utils.SequenceToKey(buildJob.BuildId))
		return bucket.Put([]byte(key), []byte(punqStructs.PrettyPrintString(buildJob)))
	})
	if err != nil {
		log.Errorf("Error saving job '%d'.", buildJob.BuildId)
	}
}

func PrintAllEntriesFromDb(bucket string, prefix string) {
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucket))
		c := bucket.Cursor()
		prefix := []byte(prefix)
		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
			job := structs.BuildJob{}
			err := structs.UnmarshalJob(&job, jobData)
			if err != nil {
				log.Infof("bucket=%s, key=%s, value=%s\n", bucket, k, job.BuildId)
			}
		}
		return nil
	})
	if err != nil {
		log.Errorf("printAllEntries: %s", err.Error())
	}
}

func DeleteFromDb(bucket string, prefix string, buildNo uint64) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucket))
		key := fmt.Sprintf("%s-%s", PREFIX_QUEUE, utils.SequenceToKey(buildNo))
		return bucket.Delete([]byte(key))
	})
}

func AddToDb(buildJob structs.BuildJob) (int, error) {
	// setup usefull defaults
	if buildJob.JobId == "" {
		buildJob.JobId = punqUtils.NanoId()
	}
	if buildJob.State == "" {
		buildJob.State = punqStructs.JobStatePending
	}

	var nextBuildId uint64 = 0
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))

		// FIRST: CHECK FOR DUPLICATES
		c := bucket.Cursor()
		prefix := []byte(PREFIX_QUEUE)
		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
			job := structs.BuildJob{}
			err := structs.UnmarshalJob(&job, jobData)
			if err != nil {
				log.Errorf("AddToDb (unmarshall) ERR: %s", err.Error())
				continue
			}
			//
			if (job.State == punqStructs.JobStatePending || job.State == punqStructs.JobStateStarted) && job.Service.ControllerName == buildJob.Service.ControllerName {
				for _, container := range job.Service.Containers {
					for _, jobContainer := range buildJob.Service.Containers {
						if *jobContainer.GitCommitHash == *container.GitCommitHash {
							return fmt.Errorf("Duplicate Commit-Hash (%s) found skipping build.", *container.GitCommitHash)
						}
					}
				}
			}
		}
		nextBuildId, _ = bucket.NextSequence() // auto increment
		buildJob.BuildId = nextBuildId
		key := fmt.Sprintf("%s-%s", PREFIX_QUEUE, utils.SequenceToKey(nextBuildId))
		return bucket.Put([]byte(key), []byte(punqStructs.PrettyPrintString(buildJob)))
	})
	return int(nextBuildId), err
}

func SaveBuildResult(
	state punqStructs.JobStateEnum,
	prefix structs.BuildPrefixEnum,
	cmdOutput string,
	startTime time.Time,
	job *structs.BuildJob,
	container *dtos.K8sContainerDto,
) error {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		entry := structs.CreateBuildJobInfoEntryBytes(state, cmdOutput, startTime, time.Now(), prefix, job, container)
		key := structs.BuildJobInfoEntryKey(prefix, job.BuildId, job.Namespace.Name, job.Service.ControllerName, container.Name)
		return bucket.Put([]byte(key), entry)
	})
	if err != nil {
		log.Errorf("Error saving build result for '%d'.", job.BuildId)
	}
	return err
}

func AddLogToDb(title string, message string, category structs.Category, logType structs.LogType) error {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(LOG_BUCKET_NAME))
		id, _ := bucket.NextSequence() // auto increment
		entry := structs.CreateLog(id, title, message, category, logType)
		return bucket.Put([]byte(fmt.Sprintf("%s_%s_%s", entry.CreatedAt, entry.Category, entry.Type)), structs.LogBytes(entry))
	})
	if err != nil {
		log.Errorf("Error adding log for '%s'.", title)
	}
	return err
}

func ListLogFromDb() []structs.Log {
	result := []structs.Log{}
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(LOG_BUCKET_NAME))
		c := bucket.Cursor()
		prefix := []byte("")
		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
			entry := structs.Log{}
			err := structs.UnmarshalLog(&entry, jobData)
			if err != nil {
				return err
			}
			result = append(result, entry)
		}
		return nil
	})
	if err != nil {
		log.Errorf("ListLog: %s", err.Error())
	}
	return result
}

func AddMigrationToDb(name string) error {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(MIGRATION_BUCKET_NAME))
		id, _ := bucket.NextSequence() // auto increment
		entry := structs.CreateMigration(id, name)
		return bucket.Put([]byte(entry.Name), structs.MigrationBytes(entry))
	})
	if err != nil {
		log.Errorf("Error adding migration '%s'.", name)
	}
	return err
}

func IsMigrationAlreadyApplied(name string) bool {
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(MIGRATION_BUCKET_NAME))
		rawData := bucket.Get([]byte(name))
		if len(rawData) > 0 {
			return nil
		}
		return fmt.Errorf("Not migration found for name '%s'.", name)
	})
	return err == nil
}

func AppendToKey(bucket string, key string, value string) error {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucket))
		rawData := bucket.Get([]byte(key))
		if len(rawData) > 0 {
			if len(rawData) > MAX_ENTRY_LENGTH {
				return fmt.Errorf("Entry for key '%s' is too long (%d is the limit).", key, MAX_ENTRY_LENGTH)
			}
			rawData = append(rawData, []byte(value)...)
			return bucket.Put([]byte(key), rawData)
		}
		return fmt.Errorf("Not key found for name '%s'.", key)
	})
	return err
}
