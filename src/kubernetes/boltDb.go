package kubernetes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"strconv"
	"strings"
	"time"

	"go.etcd.io/bbolt"
	v1Core "k8s.io/api/core/v1"
)

const (
	BOLT_DB_SCHEMA_VERSION        = "3"
	BOLT_DB_BUILD_BUCKET_NAME     = "mogenius-builds"
	BOLT_DB_SCAN_BUCKET_NAME      = "mogenius-scans"
	BOLT_DB_LOG_BUCKET_NAME       = "mogenius-logs"
	BOLT_DB_MIGRATION_BUCKET_NAME = "mogenius-migrations"
	BOLT_DB_POD_EVENT_BUCKET_NAME = "mogenius-pod-event"
	BOLT_DB_PREFIX_QUEUE          = "queue"
	BOLT_DB_PREFIX_VUL_SCAN       = "scan"
	BOLT_DB_PREFIX_CLEANUP        = "cleanup"
)

type BoltDb interface {
	ExecuteMigrations()
	AddPodEvent(namespace string, controller string, event *v1Core.Event, maxSize int) error
	GetLastBuildJobInfosFromDb(data structs.BuildTaskRequest) structs.BuildJobInfo
	ListLogFromDb() []structs.Log
	GetJobsToBuildFromDb() []structs.BuildJob
	GetEventByKey(key string) []byte
	GetItemByKey(key string) []byte
	GetBuilderStatus() structs.BuilderStatus
	GetBuildJobInfosFromDb(buildId uint64) structs.BuildJobInfo
	GetBuildJobInfosListFromDb(namespace string, controller string, container string) []structs.BuildJobInfo
	DeleteAllBuildData(namespace string, controller string, container string)
	GetLastBuildForNamespaceAndControllerName(namespace string, controllerName string) structs.BuildJobInfo
	UpdateStateInDb(buildJob structs.BuildJob, newState structs.JobStateEnum)
	SaveBuildResult(
		state structs.JobStateEnum,
		prefix structs.BuildPrefixEnum,
		cmdOutput string,
		startTime time.Time,
		job *structs.BuildJob,
		container *dtos.K8sContainerDto,
	) error
	GetBuildJobListFromDb() []structs.BuildJob
	DeleteBuildJobFromDb(bucket string, buildId uint64) error
	AddToDb(buildJob structs.BuildJob) (int, error)
	ImageNamesFromBuildJob(buildJob structs.BuildJob) (imageName string, imageTag string, imageTagLatest string)
	SaveJobInDb(buildJob structs.BuildJob)
}

type boldDbModule struct {
	config interfaces.ConfigModule
	logger *slog.Logger
	db     *bbolt.DB
}

func NewBoltDbModule(
	config interfaces.ConfigModule,
	logger *slog.Logger,
) (BoltDb, error) {
	dbModule := boldDbModule{
		config: config,
		logger: logger,
	}
	err := dbModule.initializeBoltDb()
	if err != nil {
		return nil, err
	}
	return &dbModule, nil
}

func (m *boldDbModule) initializeBoltDb() error {
	dbPath := strings.ReplaceAll(m.config.Get("MO_BBOLT_DB_PATH"), ".db", fmt.Sprintf("-%s.db", BOLT_DB_SCHEMA_VERSION))
	database, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		m.logger.Error("Error opening bbolt database", "dbPath", dbPath, "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	}
	// ### BUILD BUCKET ###
	m.db = database
	err = m.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BOLT_DB_BUILD_BUCKET_NAME))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		m.logger.Error("failed to create bucket", "bucket", BOLT_DB_BUILD_BUCKET_NAME, "error", err)
	}
	// ### SCAN BUCKET ### create a new scan bucket on every startup
	err = m.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucket([]byte(BOLT_DB_SCAN_BUCKET_NAME))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		err := m.db.Update(func(tx *bbolt.Tx) error {
			err := tx.DeleteBucket([]byte(BOLT_DB_SCAN_BUCKET_NAME))
			if err != nil {
				return err
			}
			_, err = tx.CreateBucket([]byte(BOLT_DB_SCAN_BUCKET_NAME))
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			m.logger.Error("Error recreating bucket", "bucket", BOLT_DB_SCAN_BUCKET_NAME, "error", err)
		}
	}
	// ### LOG BUCKET ###
	err = m.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BOLT_DB_LOG_BUCKET_NAME))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		m.logger.Error("Error creating bucket", "bucket", BOLT_DB_LOG_BUCKET_NAME, "error", err)
	}

	// ### POD EVENT BUCKET ###
	err = m.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BOLT_DB_POD_EVENT_BUCKET_NAME))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		m.logger.Error("Error creating bucket", "bucket", BOLT_DB_POD_EVENT_BUCKET_NAME, "error", err)
	}

	// ### MIGRATION BUCKET ###
	err = m.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BOLT_DB_MIGRATION_BUCKET_NAME))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		m.logger.Error("Error creating bucket", "bucket", BOLT_DB_MIGRATION_BUCKET_NAME, "error", err)
	}

	// RESET STARTED JOBS TO PENDING
	m.resetStartedJobsToPendingOnInit()

	m.logger.Debug("bbolt started 🚀", "dbPath", dbPath)
	return nil
}

// if a job was started and the server was restarted/crashed, we need to reset the state to pending to resume the build
func (m *boldDbModule) resetStartedJobsToPendingOnInit() {
	err := m.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_BUILD_BUCKET_NAME))
		c := bucket.Cursor()
		prefix := []byte(BOLT_DB_PREFIX_QUEUE)
		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
			job := structs.BuildJob{}
			err := structs.UnmarshalJob(&job, jobData)
			if err != nil {
				m.logger.Error("Init (unmarshall)", "error", err)
				continue
			}
			if job.State == structs.JobStateStarted {
				job.State = structs.JobStatePending
				key := m.buildJobKey(job.BuildId)
				err := bucket.Put([]byte(key), []byte(utils.PrettyPrintString(job)))
				if err != nil {
					m.logger.Error("Init (update)", "error", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		m.logger.Error("Init (db)", "error", err)
	}
}

func (m *boldDbModule) buildJobKey(buildId uint64) string {
	return fmt.Sprintf("%s-%s", BOLT_DB_PREFIX_QUEUE, utils.SequenceToKey(buildId))
}

func (m *boldDbModule) AddPodEvent(namespace string, controller string, event *v1Core.Event, maxSize int) error {
	return m.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_POD_EVENT_BUCKET_NAME))

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

func (m *boldDbModule) GetLastBuildJobInfosFromDb(data structs.BuildTaskRequest) structs.BuildJobInfo {
	result := structs.BuildJobInfo{}
	err := m.db.View(func(tx *bbolt.Tx) error {

		bucket := tx.Bucket([]byte(BOLT_DB_BUILD_BUCKET_NAME))
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
				m.logger.Error("GetLastBuildJobInfosFromDb", "error", err)
				return err
			}
			result = m.getBuildJobInfosFromDb(lastBuildId)
		}
		return nil
	})
	if err != nil {
		m.logger.Error("GetBuildJobListFromDb", "error", err)
	}
	return result
}

func (m *boldDbModule) getBuildJobInfosFromDb(buildId uint64) structs.BuildJobInfo {
	result := structs.BuildJobInfo{}
	err := m.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_BUILD_BUCKET_NAME))
		cursorBuild := bucket.Cursor()

		key := fmt.Sprintf("%s-%s", BOLT_DB_PREFIX_QUEUE, utils.SequenceToKey(buildId))
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
		m.logger.Error("GetBuildJobFromDb (db)", "error", err)
	}

	return result
}

func (m *boldDbModule) ExecuteMigrations() {
	name, err := m._PvcMigration1()
	if err != nil {
		m.logger.Info("Migration", "name", name, "error", err)
	}
}

func (m *boldDbModule) _PvcMigration1() (string, error) {
	migrationName := utils.GetFunctionName()
	if m.isMigrationAlreadyApplied(migrationName) {
		return migrationName, fmt.Errorf("Migration already applied.")
	}

	pvcs := AllPersistentVolumeClaims("")
	for _, pvc := range pvcs {
		if strings.HasPrefix(pvc.Name, utils.NFS_POD_PREFIX) {
			volumeName := strings.Replace(pvc.Name, fmt.Sprintf("%s-", utils.NFS_POD_PREFIX), "", 1)
			pvc.Labels = MoAddLabels(&pvc.Labels, map[string]string{
				LabelKeyVolumeIdentifier: pvc.Name,
				LabelKeyVolumeName:       volumeName,
			})
			UpdateK8sPersistentVolumeClaim(pvc)
			// now also update auto-created PVC
			connectedPvc, err := GetPersistentVolumeClaim(pvc.Namespace, volumeName)
			if err == nil && connectedPvc != nil {
				connectedPvc.Labels = MoAddLabels(&connectedPvc.Labels, map[string]string{
					LabelKeyVolumeIdentifier: pvc.Name,
					LabelKeyVolumeName:       volumeName,
				})
				UpdateK8sPersistentVolumeClaim(*connectedPvc)
			}

			m.logger.Info("Updated PVC", "name", pvc.Name)
		}
	}
	pvs := AllPersistentVolumesRaw()
	for _, pv := range pvs {
		if pv.Spec.ClaimRef != nil {
			if strings.HasPrefix(pv.Spec.ClaimRef.Name, utils.NFS_POD_PREFIX) {
				pv.Labels = MoAddLabels(&pv.Labels, map[string]string{
					LabelKeyVolumeIdentifier: pv.Spec.ClaimRef.Name,
					LabelKeyVolumeName: strings.Replace(
						pv.Spec.ClaimRef.Name,
						fmt.Sprintf("%s-", utils.NFS_POD_PREFIX),
						"",
						1,
					),
				})
				_, err := UpdateK8sPersistentVolume(pv)
				if err != nil {
					m.logger.Error("failed to update k8s persistent volume", "error", err)
				}
				m.logger.Info("Updated PV", "name", pv.Name)
			}
		}
	}

	m.logger.Info("Migration applied successfuly.", "migrationName", migrationName)
	err := m.addMigrationToDb(migrationName)
	if err != nil {
		return migrationName, fmt.Errorf("Migration '%s' applied successfuly, but could not be added to migrations table: %s", migrationName, err.Error())
	}
	return migrationName, nil
}

func (m *boldDbModule) isMigrationAlreadyApplied(name string) bool {
	err := m.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_MIGRATION_BUCKET_NAME))
		rawData := bucket.Get([]byte(name))
		if len(rawData) > 0 {
			return nil
		}
		return fmt.Errorf("Not migration found for name '%s'.", name)
	})
	return err == nil
}

func (m *boldDbModule) addMigrationToDb(name string) error {
	err := m.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_MIGRATION_BUCKET_NAME))
		id, _ := bucket.NextSequence() // auto increment
		entry := structs.CreateMigration(id, name)
		return bucket.Put([]byte(entry.Name), structs.MigrationBytes(entry))
	})
	if err != nil {
		m.logger.Error("Error adding migration.", "name", name, "error", err)
	}
	return err
}

func (m *boldDbModule) ListLogFromDb() []structs.Log {
	result := []structs.Log{}
	err := m.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_LOG_BUCKET_NAME))
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
		m.logger.Error("ListLog", "error", err)
	}
	return result
}

func (m *boldDbModule) GetJobsToBuildFromDb() []structs.BuildJob {
	result := []structs.BuildJob{}
	err := m.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket([]byte(BOLT_DB_BUILD_BUCKET_NAME)).Cursor()
		prefix := []byte(BOLT_DB_PREFIX_QUEUE)
		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
			job := structs.BuildJob{}
			err := structs.UnmarshalJob(&job, jobData)
			if err == nil {
				if job.State == structs.JobStatePending {
					result = append(result, job)
				}
			} else {
				m.logger.Error("ProcessQueue (unmarshall)", "error", err)
			}
		}
		return nil
	})
	if err != nil {
		m.logger.Error("GetJobsToBuildFromDb (db)", "error", err)
	}
	return result
}

func (m *boldDbModule) GetEventByKey(key string) []byte {
	rawData := []byte{}
	err := m.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_POD_EVENT_BUCKET_NAME))
		rawData = bucket.Get([]byte(key))
		return nil
	})
	if err != nil {
		m.logger.Error("GetEventByKey (db)", "error", err)
	}
	return rawData
}

func (m *boldDbModule) GetItemByKey(key string) []byte {
	rawData := []byte{}
	err := m.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_BUILD_BUCKET_NAME))
		rawData = bucket.Get([]byte(key))
		return nil
	})
	if err != nil {
		m.logger.Error("GetBuilderStatus (db)", "error", err)
	}
	return rawData
}

func (m *boldDbModule) GetBuilderStatus() structs.BuilderStatus {
	result := structs.BuilderStatus{}

	err := m.db.View(func(tx *bbolt.Tx) error {
		cursorBuild := tx.Bucket([]byte(BOLT_DB_BUILD_BUCKET_NAME)).Cursor()
		prefix := []byte(BOLT_DB_PREFIX_QUEUE)
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
		cursorScan := tx.Bucket([]byte(BOLT_DB_SCAN_BUCKET_NAME)).Cursor()
		prefixScan := []byte(BOLT_DB_PREFIX_VUL_SCAN)
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
		m.logger.Error("GetBuilderStatus (db)", "error", err)
	}
	return result
}

func (m *boldDbModule) GetBuildJobInfosFromDb(buildId uint64) structs.BuildJobInfo {
	result := structs.BuildJobInfo{}
	err := m.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_BUILD_BUCKET_NAME))
		cursorBuild := bucket.Cursor()

		key := fmt.Sprintf("%s-%s", BOLT_DB_PREFIX_QUEUE, utils.SequenceToKey(buildId))
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
		m.logger.Error("GetBuildJobFromDb (db)", "error", err)
	}

	return result
}

func (m *boldDbModule) GetBuildJobInfosListFromDb(namespace string, controller string, container string) []structs.BuildJobInfo {
	results := []structs.BuildJobInfo{}
	err := m.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_BUILD_BUCKET_NAME))
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
						results = append(results, m.GetBuildJobInfosFromDb(buildId))
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
		m.logger.Error("GetBuildJobInfosListFromDb (db)", "error", err)
	}

	return results
}

func (m *boldDbModule) DeleteAllBuildData(namespace string, controller string, container string) {
	err := m.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_BUILD_BUCKET_NAME))
		c := bucket.Cursor()
		suffix := structs.BuildJobInfosKeySuffix(namespace, controller, container)
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			if strings.HasSuffix(string(k), suffix) {
				// delete all build data
				err := bucket.Delete(k)
				if err != nil {
					m.logger.Error("DeleteAllBuildData delete build data", "error", err)
				}

				parts := strings.Split(string(k), "___")
				if len(parts) >= 3 {
					buildIdStr := parts[0]
					buildId, err := strconv.ParseUint(buildIdStr, 10, 64)
					if err != nil {
						m.logger.Error("DeleteAllBuildData parse buildId", "error", err)
					}
					// Delete queue entry
					queueKey := fmt.Sprintf("%s-%s", BOLT_DB_PREFIX_QUEUE, utils.SequenceToKey(buildId))
					err = bucket.Delete([]byte(queueKey))
					if err != nil {
						m.logger.Error("DeleteAllBuildData delete queue entry", "error", err)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		m.logger.Error("DeleteAllBuildData", "error", err.Error())
	}
	// Delete event entry
	eventKey := fmt.Sprintf("%s-%s", namespace, controller)
	err = m.deleteEventByKey(eventKey)
	if err != nil {
		m.logger.Error("DeleteAllBuildData delete event entry", "error", err.Error())
	}
}

func (m *boldDbModule) deleteEventByKey(key string) error {
	return m.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_POD_EVENT_BUCKET_NAME))
		err := bucket.Delete([]byte(key))
		if err != nil {
			m.logger.Error("DeleteEventByKey", "error", err)
		}
		return nil
	})
}

func (m *boldDbModule) GetLastBuildForNamespaceAndControllerName(namespace string, controllerName string) structs.BuildJobInfo {
	result := structs.BuildJobInfo{}
	err := m.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_BUILD_BUCKET_NAME))
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
				m.logger.Error("GetLastBuildJobInfosFromDb", "error", err)
				return err
			}
			result = m.GetBuildJobInfosFromDb(lastBuildId)
		}
		return nil
	})
	if err != nil {
		m.logger.Error("GetBuildJobListFromDb", "error", err)
	}
	return result
}

func (m *boldDbModule) UpdateStateInDb(buildJob structs.BuildJob, newState structs.JobStateEnum) {
	err := m.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_BUILD_BUCKET_NAME))
		key := m.buildJobKey(buildJob.BuildId)
		jobData := bucket.Get([]byte(key))
		job := structs.BuildJob{}
		err := structs.UnmarshalJob(&job, jobData)
		if err == nil {
			job.State = newState
			return bucket.Put([]byte(key), []byte(utils.PrettyPrintString(job)))
		}
		return err
	})
	if err != nil {
		errStr := fmt.Sprintf("Error updating state for build '%d'. REASON: %s", buildJob.BuildId, err.Error())
		m.logger.Error(errStr)
	}
	m.logger.Info("State for build updated successfuly.", "buildId", buildJob.BuildId, "newState", newState)
}

func (m *boldDbModule) SaveBuildResult(
	state structs.JobStateEnum,
	prefix structs.BuildPrefixEnum,
	cmdOutput string,
	startTime time.Time,
	job *structs.BuildJob,
	container *dtos.K8sContainerDto,
) error {
	err := m.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_BUILD_BUCKET_NAME))
		entry := structs.CreateBuildJobInfoEntryBytes(state, cmdOutput, startTime, time.Now(), prefix, job, container)
		key := structs.BuildJobInfoEntryKey(job.BuildId, prefix, job.Namespace.Name, job.Service.ControllerName, container.Name)
		return bucket.Put([]byte(key), entry)
	})
	if err != nil {
		m.logger.Error("Error saving build result.", "buildId", job.BuildId)
	}
	return err
}

func (m *boldDbModule) GetBuildJobListFromDb() []structs.BuildJob {
	result := []structs.BuildJob{}
	err := m.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_BUILD_BUCKET_NAME))
		c := bucket.Cursor()
		prefix := []byte(BOLT_DB_PREFIX_QUEUE)
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
		m.logger.Error("GetBuildJobListFromDb", "error", err)
	}
	return result
}

func (m *boldDbModule) DeleteBuildJobFromDb(bucket string, buildId uint64) error {
	return m.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucket))
		key := m.buildJobKey(buildId)
		return bucket.Delete([]byte(key))
	})
}

func (m *boldDbModule) AddToDb(buildJob structs.BuildJob) (int, error) {
	// setup usefull defaults
	if buildJob.JobId == "" {
		buildJob.JobId = utils.NanoId()
	}
	if buildJob.State == "" {
		buildJob.State = structs.JobStatePending
	}

	var nextBuildId uint64 = 0
	err := m.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_BUILD_BUCKET_NAME))

		// FIRST: CHECK FOR DUPLICATES
		c := bucket.Cursor()
		prefix := []byte(BOLT_DB_PREFIX_QUEUE)
		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
			job := structs.BuildJob{}
			err := structs.UnmarshalJob(&job, jobData)
			if err != nil {
				m.logger.Error("AddToDb (unmarshall)", "error", err)
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
		_, imageTag, _ := m.ImageNamesFromBuildJob(buildJob)
		buildJob.Image = imageTag
		key := m.buildJobKey(nextBuildId)
		return bucket.Put([]byte(key), []byte(utils.PrettyPrintString(buildJob)))
	})
	return int(nextBuildId), err
}

func (m *boldDbModule) ImageNamesFromBuildJob(buildJob structs.BuildJob) (imageName string, imageTag string, imageTagLatest string) {
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

func (m *boldDbModule) SaveJobInDb(buildJob structs.BuildJob) {
	err := m.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BOLT_DB_BUILD_BUCKET_NAME))
		key := m.buildJobKey(buildJob.BuildId)
		return bucket.Put([]byte(key), []byte(utils.PrettyPrintString(buildJob)))
	})
	if err != nil {
		m.logger.Error("Error saving job.", "buildId", buildJob.BuildId, "error", err)
	}
}
