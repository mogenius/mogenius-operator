package db

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"strconv"
	"strings"
	"time"

	v1Core "k8s.io/api/core/v1"

	punqStructs "github.com/mogenius/punq/structs"
	bolt "go.etcd.io/bbolt"
)

const (
	DB_SCHEMA_VERSION = "3"
)

const (
	BUILD_BUCKET_NAME     = "mogenius-builds"
	SCAN_BUCKET_NAME      = "mogenius-scans"
	LOG_BUCKET_NAME       = "mogenius-logs"
	MIGRATION_BUCKET_NAME = "mogenius-migrations"
	POD_EVENT_BUCKET_NAME = "mogenius-pod-event"

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

func BuildJobKey(buildId uint64) string {
	return fmt.Sprintf("%s-%s", PREFIX_QUEUE, utils.SequenceToKey(buildId))
}

var db *bolt.DB

func Start() {
	dbPath := strings.ReplaceAll(config.Get("MO_BBOLT_DB_PATH"), ".db", fmt.Sprintf("-%s.db", DB_SCHEMA_VERSION))
	database, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		dbLogger.Error("Error opening bbolt database", "dbPath", dbPath, "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
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
		dbLogger.Error("failed to create bucket", "bucket", BUILD_BUCKET_NAME, "error", err)
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
			dbLogger.Error("Error recreating bucket", "bucket", SCAN_BUCKET_NAME, "error", err)
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
		dbLogger.Error("Error creating bucket", "bucket", LOG_BUCKET_NAME, "error", err)
	}

	// ### POD EVENT BUCKET ###
	db = database
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(POD_EVENT_BUCKET_NAME))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		dbLogger.Error("Error creating bucket", "bucket", POD_EVENT_BUCKET_NAME, "error", err)
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
		dbLogger.Error("Error creating bucket", "bucket", MIGRATION_BUCKET_NAME, "error", err)
	}

	// RESET STARTED JOBS TO PENDING
	resetStartedJobsToPendingOnInit()

	dbLogger.Debug("bbold started 🚀", "dbPath", dbPath)
}

func close() {
	dbLogger.Debug("Shutting down db...")
	if db != nil {
		db.Close()
	}
}

func DeleteAllBuildData(namespace string, controller string, container string) {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		c := bucket.Cursor()
		suffix := structs.BuildJobInfosKeySuffix(namespace, controller, container)
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			if strings.HasSuffix(string(k), suffix) {
				// delete all build data
				err := bucket.Delete(k)
				if err != nil {
					dbLogger.Error("DeleteAllBuildData delete build data", "error", err)
				}

				parts := strings.Split(string(k), "___")
				if len(parts) >= 3 {
					buildIdStr := parts[0]
					buildId, err := strconv.ParseUint(buildIdStr, 10, 64)
					if err != nil {
						dbLogger.Error("DeleteAllBuildData parse buildId", "error", err)
					}
					// Delete queue entry
					queueKey := fmt.Sprintf("%s-%s", PREFIX_QUEUE, utils.SequenceToKey(buildId))
					err = bucket.Delete([]byte(queueKey))
					if err != nil {
						dbLogger.Error("DeleteAllBuildData delete queue entry", "error", err)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		dbLogger.Error("DeleteAllBuildData", "error", err.Error())
	}
	// Delete event entry
	eventKey := fmt.Sprintf("%s-%s", namespace, controller)
	err = DeleteEventByKey(eventKey)
	if err != nil {
		dbLogger.Error("DeleteAllBuildData delete event entry", "error", err.Error())
	}
}

// if a job was started and the server was restarted/crashed, we need to reset the state to pending to resume the build
func resetStartedJobsToPendingOnInit() {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		c := bucket.Cursor()
		prefix := []byte(PREFIX_QUEUE)
		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
			job := structs.BuildJob{}
			err := structs.UnmarshalJob(&job, jobData)
			if err != nil {
				dbLogger.Error("Init (unmarshall)", "error", err)
				continue
			}
			if job.State == structs.JobStateStarted {
				job.State = structs.JobStatePending
				key := BuildJobKey(job.BuildId)
				err := bucket.Put([]byte(key), []byte(punqStructs.PrettyPrintString(job)))
				if err != nil {
					dbLogger.Error("Init (update)", "error", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		dbLogger.Error("Init (db)", "error", err)
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
				if job.State == structs.JobStatePending {
					result = append(result, job)
				}
			} else {
				dbLogger.Error("ProcessQueue (unmarshall)", "error", err)
			}
		}
		return nil
	})
	if err != nil {
		dbLogger.Error("GetJobsToBuildFromDb (db)", "error", err)
	}
	return result
}

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
// 	err := db.Update(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(SCAN_BUCKET_NAME))

// 		var json = jsoniter.ConfigCompatibleWithStandardLibrary
// 		bytes, err := json.Marshal(data)
// 		if err != nil {
// 			dblogger.Errorf("Error %s: %s", PREFIX_VUL_SCAN, err.Error())
// 		}
// 		return bucket.Put([]byte(fmt.Sprintf("%s%s", PREFIX_VUL_SCAN, imageName)), bytes)
// 	})
// 	if err != nil {
// 		dblogger.Errorf("Error saving scan data for '%s'.", imageName)
// 	}
// }

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
				if job.State == structs.JobStatePending {
					result.QueuedBuilds++
				}
				if job.State == structs.JobStateFailed {
					result.FailedBuilds++
				}
				if job.State == structs.JobStateCanceled {
					result.CanceledBuilds++
				}
				if job.State == structs.JobStateSucceeded {
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
		dbLogger.Error("GetBuilderStatus (db)", "error", err)
	}
	return result
}

func GetLastBuildJobInfosFromDb(data structs.BuildTaskRequest) structs.BuildJobInfo {
	result := structs.BuildJobInfo{}
	err := db.View(func(tx *bolt.Tx) error {

		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		cursorBuild := bucket.Cursor()

		suffix := structs.BuildJobInfosKeySuffix(data.Namespace, data.Controller, data.Container)
		var lastBuildKey string
		for k, _ := cursorBuild.Last(); k != nil; k, _ = cursorBuild.Prev() {
			if strings.HasSuffix(string(k), suffix) {
				lastBuildKey = string(k)
				break
			}
		}

		parts := strings.Split(lastBuildKey, "___")
		if len(parts) >= 3 {
			buildId := parts[0]
			lastBuildId, err := strconv.ParseUint(buildId, 10, 64)
			if err != nil {
				dbLogger.Error("GetLastBuildJobInfosFromDb", "error", err)
				return err
			}
			result = GetBuildJobInfosFromDb(lastBuildId)
		}
		return nil
	})
	if err != nil {
		dbLogger.Error("GetBuildJobListFromDb", "error", err)
	}
	return result
}

func GetLastBuildForNamespaceAndControllerName(namespace string, controllerName string) structs.BuildJobInfo {
	result := structs.BuildJobInfo{}
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		cursorBuild := bucket.Cursor()

		suffix := structs.GetLastBuildJobInfosFromDbByNamespaceAndControllerName(namespace, controllerName)
		var lastBuildKey string
		for k, _ := cursorBuild.Last(); k != nil; k, _ = cursorBuild.Prev() {
			if strings.Contains(string(k), suffix) {
				lastBuildKey = string(k)
				break
			}
		}

		if lastBuildKey == "" {
			return nil
		}

		parts := strings.Split(lastBuildKey, "___")
		if len(parts) >= 3 {
			buildId := parts[0]
			lastBuildId, err := strconv.ParseUint(buildId, 10, 64)
			if err != nil {
				dbLogger.Error("GetLastBuildJobInfosFromDb", "error", err)
				return err
			}
			result = GetBuildJobInfosFromDb(lastBuildId)
		}
		return nil
	})
	if err != nil {
		dbLogger.Error("GetBuildJobListFromDb", "error", err)
	}
	return result
}

func GetBuildJobInfosFromDb(buildId uint64) structs.BuildJobInfo {
	result := structs.BuildJobInfo{}
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		cursorBuild := bucket.Cursor()

		key := fmt.Sprintf("%s-%s", PREFIX_QUEUE, utils.SequenceToKey(buildId))
		queueEntry := bucket.Get([]byte(key))
		job := structs.BuildJob{}
		err := structs.UnmarshalJob(&job, queueEntry)
		if err != nil {
			return err
		}

		namespace := job.Namespace.Name
		controller := job.Service.ControllerName
		container := ""

		prefixBuild := structs.GetBuildJobInfosPrefix(buildId, structs.PrefixBuild, namespace, controller)
		prefixClone := structs.GetBuildJobInfosPrefix(buildId, structs.PrefixGitClone, namespace, controller)
		for k, _ := cursorBuild.Last(); k != nil; k, _ = cursorBuild.Prev() {
			if strings.HasPrefix(string(k), prefixBuild) || strings.HasPrefix(string(k), prefixClone) {
				buildTemp := structs.CreateBuildJobEntryFromData(bucket.Get(k))
				container = buildTemp.Container
				break
			}
		}

		containerObj := dtos.K8sContainerDto{}
		for _, item := range job.Service.Containers {
			if item.Name == container {
				containerObj = item
				break
			}
		}

		clone := bucket.Get([]byte(structs.BuildJobInfoEntryKey(buildId, structs.PrefixGitClone, namespace, controller, container)))
		ls := bucket.Get([]byte(structs.BuildJobInfoEntryKey(buildId, structs.PrefixLs, namespace, controller, container)))
		login := bucket.Get([]byte(structs.BuildJobInfoEntryKey(buildId, structs.PrefixLogin, namespace, controller, container)))
		build := bucket.Get([]byte(structs.BuildJobInfoEntryKey(buildId, structs.PrefixBuild, namespace, controller, container)))
		push := bucket.Get([]byte(structs.BuildJobInfoEntryKey(buildId, structs.PrefixPush, namespace, controller, container)))
		result = structs.CreateBuildJobInfo(job.Image, clone, ls, login, build, push)

		result.BuildId = buildId
		result.Namespace = namespace
		result.Controller = controller
		result.Container = container

		if containerObj.GitCommitHash != nil {
			result.CommitHash = *containerObj.GitCommitHash
		}
		if containerObj.GitRepository != nil && containerObj.GitCommitHash != nil {
			result.CommitLink = *utils.GitCommitLink(*containerObj.GitRepository, *containerObj.GitCommitHash)
		}
		if containerObj.GitCommitAuthor != nil {
			result.CommitAuthor = *containerObj.GitCommitAuthor
		}
		if containerObj.GitCommitMessage != nil {
			result.CommitMessage = *containerObj.GitCommitMessage
		}
		return nil
	})
	if err != nil {
		dbLogger.Error("GetBuildJobFromDb (db)", "error", err)
	}

	return result
}

func GetBuildJobInfosListFromDb(namespace string, controller string, container string) []structs.BuildJobInfo {
	results := []structs.BuildJobInfo{}
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		cursorBuild := bucket.Cursor()

		suffix := structs.BuildJobInfosKeySuffix(namespace, controller, container)
		buildIdStrings := []string{}
		for k, _ := cursorBuild.Last(); k != nil; k, _ = cursorBuild.Prev() {
			if strings.HasSuffix(string(k), suffix) {
				parts := strings.Split(string(k), "___")
				if len(parts) >= 3 {
					buildIdString := parts[0]
					if utils.ContainsString(buildIdStrings, buildIdString) {
						continue
					}
					buildIdStrings = append(buildIdStrings, buildIdString)
					buildId, err := strconv.ParseUint(buildIdString, 10, 64)
					if err == nil {
						results = append(results, GetBuildJobInfosFromDb(buildId))
					}
				}
				if len(results) > 20 {
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		dbLogger.Error("GetBuildJobInfosListFromDb (db)", "error", err)
	}

	return results
}

func GetItemByKey(key string) []byte {
	rawData := []byte{}
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		rawData = bucket.Get([]byte(key))
		return nil
	})
	if err != nil {
		dbLogger.Error("GetBuilderStatus (db)", "error", err)
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
		dbLogger.Error("GetBuildJobListFromDb", "error", err)
	}
	return result
}

func UpdateStateInDb(buildJob structs.BuildJob, newState structs.JobStateEnum) {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		key := BuildJobKey(buildJob.BuildId)
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
		dbLogger.Error(errStr)
	}
	dbLogger.Info("State for build updated successfuly.", "buildId", buildJob.BuildId, "newState", newState)
}

// func PositionInQueueFromDb(buildId uint64) int {
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
// 				if job.State == structs.JobStatePending && job.BuildId != buildId {
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

func SaveJobInDb(buildJob structs.BuildJob) {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		key := BuildJobKey(buildJob.BuildId)
		return bucket.Put([]byte(key), []byte(punqStructs.PrettyPrintString(buildJob)))
	})
	if err != nil {
		dbLogger.Error("Error saving job.", "buildId", buildJob.BuildId, "error", err)
	}
}

// func PrintAllEntriesFromDb(bucketName string, prefix string) {
// 	err := db.View(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket([]byte(bucketName))
// 		c := bucket.Cursor()
// 		prefix := []byte(prefix)
// 		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
// 			job := structs.BuildJob{}
// 			err := structs.UnmarshalJob(&job, jobData)
// 			if err != nil {
// 				dblogger.Infof("bucket=%s, key=%s, value=%d\n", bucketName, string(k), job.BuildId)
// 			}
// 		}
// 		return nil
// 	})
// 	if err != nil {
// 		dblogger.Errorf("printAllEntries: %s", err.Error())
// 	}
// }

func DeleteBuildJobFromDb(bucket string, buildId uint64) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucket))
		key := BuildJobKey(buildId)
		return bucket.Delete([]byte(key))
	})
}

func AddToDb(buildJob structs.BuildJob) (int, error) {
	// setup usefull defaults
	if buildJob.JobId == "" {
		buildJob.JobId = utils.NanoId()
	}
	if buildJob.State == "" {
		buildJob.State = structs.JobStatePending
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
				dbLogger.Error("AddToDb (unmarshall)", "error", err)
				continue
			}
			//
			if (job.State == structs.JobStatePending || job.State == structs.JobStateStarted) && job.Service.ControllerName == buildJob.Service.ControllerName {
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
		_, imageTag, _ := ImageNamesFromBuildJob(buildJob)
		buildJob.Image = imageTag
		key := BuildJobKey(nextBuildId)
		return bucket.Put([]byte(key), []byte(punqStructs.PrettyPrintString(buildJob)))
	})
	return int(nextBuildId), err
}

func ImageNamesFromBuildJob(buildJob structs.BuildJob) (imageName string, imageTag string, imageTagLatest string) {
	imageName = fmt.Sprintf("%s-%s", buildJob.Namespace.Name, buildJob.Service.ControllerName)
	// overwrite images name for local builds
	if /*buildJob.Project.ContainerRegistryUser == nil &&*/ buildJob.Project.ContainerRegistryPat == nil {
		imageTag = fmt.Sprintf("%s/%s:%d", config.Get("MO_LOCAL_CONTAINER_REGISTRY_HOST"), imageName, buildJob.BuildId)
		imageTagLatest = fmt.Sprintf("%s/%s:latest", config.Get("MO_LOCAL_CONTAINER_REGISTRY_HOST"), imageName)
	} else {
		if *buildJob.Project.ContainerRegistryPath == "docker.io" {
			imageName = fmt.Sprintf("%s/%s", *buildJob.Project.ContainerRegistryUser, imageName)
		}
		imageTag = fmt.Sprintf("%s/%s:%d", *buildJob.Project.ContainerRegistryPath, imageName, buildJob.BuildId)
		imageTagLatest = fmt.Sprintf("%s/%s:latest", *buildJob.Project.ContainerRegistryPath, imageName)
	}
	return imageName, imageTag, imageTagLatest
}

func SaveBuildResult(
	state structs.JobStateEnum,
	prefix structs.BuildPrefixEnum,
	cmdOutput string,
	startTime time.Time,
	job *structs.BuildJob,
	container *dtos.K8sContainerDto,
) error {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUILD_BUCKET_NAME))
		entry := structs.CreateBuildJobInfoEntryBytes(state, cmdOutput, startTime, time.Now(), prefix, job, container)
		key := structs.BuildJobInfoEntryKey(job.BuildId, prefix, job.Namespace.Name, job.Service.ControllerName, container.Name)
		return bucket.Put([]byte(key), entry)
	})
	if err != nil {
		dbLogger.Error("Error saving build result.", "buildId", job.BuildId)
	}
	return err
}

func AddLogToDb(title string, message string, category structs.Category, logType structs.LogType) {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(LOG_BUCKET_NAME))
		id, _ := bucket.NextSequence() // auto increment
		entry := structs.CreateLog(id, title, message, category, logType)
		return bucket.Put([]byte(fmt.Sprintf("%s_%s_%s", entry.CreatedAt, entry.Category, entry.Type)), structs.LogBytes(entry))
	})
	if err != nil {
		dbLogger.Error("Error adding log", "title", title, "error", err)
	}
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
		dbLogger.Error("ListLog", "error", err)
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
		dbLogger.Error("Error adding migration.", "name", name, "error", err)
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

func AddPodEvent(namespace string, controller string, event *v1Core.Event, maxSize int) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(POD_EVENT_BUCKET_NAME))

		key := fmt.Sprintf("%s-%s", namespace, controller)
		existing := bucket.Get([]byte(key))
		var events []*v1Core.Event

		if existing != nil {
			if err := json.Unmarshal(existing, &events); err != nil {
				return err
			}
		}

		for _, e := range events {
			if e.UID == event.UID || e.UID == event.ObjectMeta.UID {
				return nil
			}
		}

		if len(events) >= maxSize {
			events = events[1:]
		}

		events = append(events, event)

		updatedData, err := json.Marshal(events)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(key), updatedData)
	})
}

func GetEventByKey(key string) []byte {
	rawData := []byte{}
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(POD_EVENT_BUCKET_NAME))
		rawData = bucket.Get([]byte(key))
		return nil
	})
	if err != nil {
		dbLogger.Error("GetEventByKey (db)", "error", err)
	}
	return rawData
}

func DeleteEventByKey(key string) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(POD_EVENT_BUCKET_NAME))
		err := bucket.Delete([]byte(key))
		if err != nil {
			dbLogger.Error("DeleteEventByKey", "error", err)
		}
		return nil
	})
}
