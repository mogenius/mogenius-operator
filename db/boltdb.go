package db

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"strconv"
	"strings"
	"time"

	v1Core "k8s.io/api/core/v1"

	punqStructs "github.com/mogenius/punq/structs"
	punqUtils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"
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

var dblogger = log.WithField("component", structs.ComponentDb)

func BuildJobKey(buildId uint64) string {
	return fmt.Sprintf("%s-%s", PREFIX_QUEUE, utils.SequenceToKey(buildId))
}

var db *bolt.DB

func Init() {
	dbPath := strings.ReplaceAll(utils.CONFIG.Kubernetes.BboltDbPath, ".db", fmt.Sprintf("-%s.db", DB_SCHEMA_VERSION))
	database, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		dblogger.Errorf("Error opening bbolt database from '%s'", dbPath)
		dblogger.Fatal(err.Error())
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
		dblogger.Errorf("Error creating bucket ('%s'): %s", BUILD_BUCKET_NAME, err)
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
			dblogger.Errorf("Error recreating bucket ('%s'): %s", SCAN_BUCKET_NAME, err)
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
		dblogger.Errorf("Error creating bucket ('%s'): %s", LOG_BUCKET_NAME, err)
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
		dblogger.Errorf("Error creating bucket ('%s'): %s", POD_EVENT_BUCKET_NAME, err)
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
		dblogger.Errorf("Error creating bucket ('%s'): %s", MIGRATION_BUCKET_NAME, err)
	}

	// RESET STARTED JOBS TO PENDING
	resetStartedJobsToPendingOnInit()

	dblogger.Infof("bbold started 🚀 (Path: '%s')", dbPath)
}

func Close() {
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
					dblogger.Errorf("DeleteAllBuildData delete build data: %s", err.Error())
				}

				parts := strings.Split(string(k), "___")
				if len(parts) >= 3 {
					buildIdStr := parts[0]
					buildId, err := strconv.ParseUint(buildIdStr, 10, 64)
					if err != nil {
						dblogger.Errorf("DeleteAllBuildData parse buildId: %s", err.Error())
					}
					// Delete queue entry
					queueKey := fmt.Sprintf("%s-%s", PREFIX_QUEUE, utils.SequenceToKey(buildId))
					err = bucket.Delete([]byte(queueKey))
					if err != nil {
						dblogger.Errorf("DeleteAllBuildData delete queue entry: %s", err.Error())
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		dblogger.Errorf("DeleteAllBuildData: %s", err.Error())
	}
	// Delete event entry
	eventKey := fmt.Sprintf("%s-%s", namespace, controller)
	err = DeleteEventByKey(eventKey)
	if err != nil {
		dblogger.Errorf("DeleteAllBuildData delete event entry: %s", err.Error())
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
				dblogger.Errorf("Init (unmarshall) ERR: %s", err.Error())
				continue
			}
			if job.State == structs.JobStateStarted {
				job.State = structs.JobStatePending
				key := BuildJobKey(job.BuildId)
				err := bucket.Put([]byte(key), []byte(punqStructs.PrettyPrintString(job)))
				if err != nil {
					dblogger.Errorf("Init (update) ERR: %s", err.Error())
				}
			}
		}
		return nil
	})
	if err != nil {
		dblogger.Errorf("Init (db) ERR: %s", err.Error())
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
				dblogger.Errorf("ProcessQueue (unmarshall) ERR: %s", err.Error())
			}
		}
		return nil
	})
	if err != nil {
		dblogger.Errorf("GetJobsToBuildFromDb (db) ERR: %s", err.Error())
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
		dblogger.Errorf("GetBuilderStatus (db) ERR: %s", err.Error())
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
				dblogger.Errorf("GetLastBuildJobInfosFromDb: %s", err.Error())
				return err
			}
			result = GetBuildJobInfosFromDb(lastBuildId)
		}
		return nil
	})
	if err != nil {
		dblogger.Errorf("GetBuildJobListFromDb: %s", err.Error())
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
				dblogger.Errorf("GetLastBuildJobInfosFromDb: %s", err.Error())
				return err
			}
			result = GetBuildJobInfosFromDb(lastBuildId)
		}
		return nil
	})
	if err != nil {
		dblogger.Errorf("GetBuildJobListFromDb: %s", err.Error())
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
		dblogger.Errorf("GetBuildJobFromDb (db) ERR: %s", err.Error())
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
		dblogger.Errorf("GetBuildJobInfosListFromDb (db) ERR: %s", err.Error())
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
		dblogger.Errorf("GetBuilderStatus (db) ERR: %s", err.Error())
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
		dblogger.Errorf("GetBuildJobListFromDb: %s", err.Error())
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
		dblogger.Error(errStr)
	}
	dblogger.Info(fmt.Sprintf("State for build '%d' updated successfuly to '%s'.", buildJob.BuildId, newState))
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
		dblogger.Errorf("Error saving job '%d'.", buildJob.BuildId)
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
		buildJob.JobId = punqUtils.NanoId()
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
				dblogger.Errorf("AddToDb (unmarshall) ERR: %s", err.Error())
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
		imageTag = fmt.Sprintf("%s/%s:%d", utils.CONFIG.Kubernetes.LocalContainerRegistryHost, imageName, buildJob.BuildId)
		imageTagLatest = fmt.Sprintf("%s/%s:latest", utils.CONFIG.Kubernetes.LocalContainerRegistryHost, imageName)
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
		dblogger.Errorf("Error saving build result for '%d'.", job.BuildId)
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
		dblogger.Errorf("Error adding log for '%s': %s", title, err.Error())
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
		dblogger.Errorf("ListLog: %s", err.Error())
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
		dblogger.Errorf("Error adding migration '%s'.", name)
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
		dblogger.Errorf("GetEventByKey (db) ERR: %s", err.Error())
	}
	return rawData
}

func DeleteEventByKey(key string) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(POD_EVENT_BUCKET_NAME))
		err := bucket.Delete([]byte(key))
		if err != nil {
			dblogger.Errorf("DeleteEventByKey: %s", err.Error())
		}
		return nil
	})
}
