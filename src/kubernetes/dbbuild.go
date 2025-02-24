package kubernetes

import (
	"fmt"
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/redisstore"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"time"
)

const (
	DB_BUILD_BUCKET_NAME = "mogenius-builds"
	DB_LOG_BUCKET_NAME   = "mogenius-logs"
	DB_PREFIX_QUEUE      = "queue"
	DB_PREFIX_CLEANUP    = "cleanup"
)

var DefaultBuildTtl = time.Hour * 24 * 30 * 6 // 6 months

type RedisBuildDb interface {
	Start() error
	GetLastBuildJobInfosFromDb(data structs.BuildTaskRequest) structs.BuildJobInfo
	ListLogFromDb() []structs.Log
	GetJobsToBuildFromDb() []structs.BuildJob
	GetBuildJobInfosFromDb(buildId int64) structs.BuildJobInfo
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
	DeleteBuildJobFromDb(bucket string, buildId int64) error
	AddToDb(buildJob structs.BuildJob) (int, error)
	ImageNamesFromBuildJob(buildJob structs.BuildJob) (imageName string, imageTag string, imageTagLatest string)
	SaveJobInDb(buildJob structs.BuildJob)
}

type redisBuildDbModule struct {
	config cfg.ConfigModule
	logger *slog.Logger
	redis  redisstore.RedisStore
}

func NewRedisBuildModule(
	config cfg.ConfigModule,
	logger *slog.Logger,
) RedisBuildDb {
	redisStore := redisstore.NewRedis(logger)
	dbModule := redisBuildDbModule{
		config: config,
		logger: logger,
		redis:  redisStore,
	}
	return &dbModule
}

func (self *redisBuildDbModule) Start() error {
	err := self.redis.Connect()
	if err != nil {
		self.logger.Error("could not connect to Redis", "error", err)
	}
	return err
}

func (self *redisBuildDbModule) buildJobKey(buildId int64) string {
	return fmt.Sprintf("%s:%s", DB_PREFIX_QUEUE, utils.SequenceToKey(buildId))
}

// TODO BROKEN
func (self *redisBuildDbModule) GetLastBuildJobInfosFromDb(data structs.BuildTaskRequest) structs.BuildJobInfo {
	result := structs.BuildJobInfo{}
	// err := self.db.View(func(tx *bbolt.Tx) error {

	// 	bucket := tx.Bucket([]byte(DB_BUILD_BUCKET_NAME))
	// 	cursorBuild := bucket.Cursor()

	// 	suffix := structs.BuildJobInfosKeySuffix(data.Namespace, data.Controller, data.Container)
	// 	var lastBuildKey string
	// 	for k, _ := cursorBuild.Last(); k != nil; k, _ = cursorBuild.Prev() {
	// 		if strings.HasSuffix(string(k), suffix) {
	// 			lastBuildKey = string(k)
	// 			break
	// 		}
	// 	}

	// 	parts := strings.Split(lastBuildKey, ":")
	// 	if len(parts) >= 3 {
	// 		buildId := parts[0]
	// 		lastBuildId, err := strconv.ParseInt(buildId, 10, 64)
	// 		if err != nil {
	// 			self.logger.Error("GetLastBuildJobInfosFromDb", "error", err)
	// 			return err
	// 		}
	// 		result = self.getBuildJobInfosFromDb(lastBuildId)
	// 	}
	// 	return nil
	// })
	// if err != nil {
	// 	self.logger.Error("GetBuildJobListFromDb", "error", err)
	// }
	return result
}

// TODO BROKEN
func (self *redisBuildDbModule) getBuildJobInfosFromDb(buildId int64) structs.BuildJobInfo {
	result := structs.BuildJobInfo{}
	// err := self.db.View(func(tx *bbolt.Tx) error {
	// 	bucket := tx.Bucket([]byte(DB_BUILD_BUCKET_NAME))
	// 	cursorBuild := bucket.Cursor()

	// 	key := fmt.Sprintf("%s-%s", DB_PREFIX_QUEUE, utils.SequenceToKey(buildId))
	// 	queueEntry := bucket.Get([]byte(key))
	// 	job := structs.BuildJob{}
	// 	err := structs.UnmarshalJob(&job, queueEntry)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	namespace := job.Namespace.Name
	// 	controller := job.Service.ControllerName
	// 	container := ""

	// 	prefixBuild := structs.GetBuildJobInfosPrefix(buildId, structs.PrefixBuild, namespace, controller)
	// 	prefixClone := structs.GetBuildJobInfosPrefix(buildId, structs.PrefixGitClone, namespace, controller)
	// 	for k, _ := cursorBuild.Last(); k != nil; k, _ = cursorBuild.Prev() {
	// 		if strings.HasPrefix(string(k), prefixBuild) || strings.HasPrefix(string(k), prefixClone) {
	// 			buildTemp := structs.CreateBuildJobEntryFromData(bucket.Get(k))
	// 			container = buildTemp.Container
	// 			break
	// 		}
	// 	}

	// 	containerObj := dtos.K8sContainerDto{}
	// 	for _, item := range job.Service.Containers {
	// 		if item.Name == container {
	// 			containerObj = item
	// 			break
	// 		}
	// 	}

	// 	clone := bucket.Get([]byte(structs.BuildJobInfoEntryKey(buildId, structs.PrefixGitClone, namespace, controller, container)))
	// 	ls := bucket.Get([]byte(structs.BuildJobInfoEntryKey(buildId, structs.PrefixLs, namespace, controller, container)))
	// 	login := bucket.Get([]byte(structs.BuildJobInfoEntryKey(buildId, structs.PrefixLogin, namespace, controller, container)))
	// 	build := bucket.Get([]byte(structs.BuildJobInfoEntryKey(buildId, structs.PrefixBuild, namespace, controller, container)))
	// 	push := bucket.Get([]byte(structs.BuildJobInfoEntryKey(buildId, structs.PrefixPush, namespace, controller, container)))
	// 	result = structs.CreateBuildJobInfo(job.Image, clone, ls, login, build, push)

	// 	result.BuildId = buildId
	// 	result.Namespace = namespace
	// 	result.Controller = controller
	// 	result.Container = container

	// 	if containerObj.GitCommitHash != nil {
	// 		result.CommitHash = *containerObj.GitCommitHash
	// 	}
	// 	if containerObj.GitRepository != nil && containerObj.GitCommitHash != nil {
	// 		result.CommitLink = *utils.GitCommitLink(*containerObj.GitRepository, *containerObj.GitCommitHash)
	// 	}
	// 	if containerObj.GitCommitAuthor != nil {
	// 		result.CommitAuthor = *containerObj.GitCommitAuthor
	// 	}
	// 	if containerObj.GitCommitMessage != nil {
	// 		result.CommitMessage = *containerObj.GitCommitMessage
	// 	}
	// 	return nil
	// })
	// if err != nil {
	// 	self.logger.Error("GetBuildJobFromDb (db)", "error", err)
	// }

	return result
}

func (self *redisBuildDbModule) ListLogFromDb() []structs.Log {
	allBuildJobs, err := redisstore.GetObjectsByPrefix[structs.Log](self.redis.GetContext(), self.redis.GetClient(), redisstore.ORDER_ASC, DB_LOG_BUCKET_NAME, "*")
	if err != nil {
		self.logger.Error("ListLogFromDb", "error", err)
		return []structs.Log{}
	}
	return allBuildJobs
}

func (self *redisBuildDbModule) GetJobsToBuildFromDb() []structs.BuildJob {
	result := []structs.BuildJob{}
	allBuildJobs, err := redisstore.GetObjectsByPrefix[structs.BuildJob](self.redis.GetContext(), self.redis.GetClient(), redisstore.ORDER_ASC, DB_BUILD_BUCKET_NAME)
	if err != nil {
		self.logger.Error("GetJobsToBuildFromDb", "error", err)
	}
	for _, job := range allBuildJobs {
		if job.State == structs.JobStatePending {
			result = append(result, job)
		}
	}
	return result
}

// TODO BROKEN
func (self *redisBuildDbModule) GetBuildJobInfosFromDb(buildId int64) structs.BuildJobInfo {
	lastBuildJob, err := redisstore.GetObjectsByPattern[structs.BuildJobInfo](self.redis.GetContext(), self.redis.GetClient(), DB_BUILD_BUCKET_NAME+":*", []string{string(buildId)})
	if err != nil || len(lastBuildJob) == 0 {
		self.logger.Error("GetLastBuildJobInfosFromDb", "error", err)
		return structs.BuildJobInfo{}
	}
	return lastBuildJob[0]

	// result := structs.BuildJobInfo{}
	// err := self.db.View(func(tx *bbolt.Tx) error {
	// 	bucket := tx.Bucket([]byte(DB_BUILD_BUCKET_NAME))
	// 	cursorBuild := bucket.Cursor()

	// 	key := fmt.Sprintf("%s-%s", DB_PREFIX_QUEUE, utils.SequenceToKey(buildId))
	// 	queueEntry := bucket.Get([]byte(key))
	// 	job := structs.BuildJob{}
	// 	err := structs.UnmarshalJob(&job, queueEntry)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	namespace := job.Namespace.Name
	// 	controller := job.Service.ControllerName
	// 	container := ""

	// 	prefixBuild := structs.GetBuildJobInfosPrefix(buildId, structs.PrefixBuild, namespace, controller)
	// 	prefixClone := structs.GetBuildJobInfosPrefix(buildId, structs.PrefixGitClone, namespace, controller)
	// 	for k, _ := cursorBuild.Last(); k != nil; k, _ = cursorBuild.Prev() {
	// 		if strings.HasPrefix(string(k), prefixBuild) || strings.HasPrefix(string(k), prefixClone) {
	// 			buildTemp := structs.CreateBuildJobEntryFromData(bucket.Get(k))
	// 			container = buildTemp.Container
	// 			break
	// 		}
	// 	}

	// 	containerObj := dtos.K8sContainerDto{}
	// 	for _, item := range job.Service.Containers {
	// 		if item.Name == container {
	// 			containerObj = item
	// 			break
	// 		}
	// 	}

	// 	clone := bucket.Get([]byte(structs.BuildJobInfoEntryKey(buildId, structs.PrefixGitClone, namespace, controller, container)))
	// 	ls := bucket.Get([]byte(structs.BuildJobInfoEntryKey(buildId, structs.PrefixLs, namespace, controller, container)))
	// 	login := bucket.Get([]byte(structs.BuildJobInfoEntryKey(buildId, structs.PrefixLogin, namespace, controller, container)))
	// 	build := bucket.Get([]byte(structs.BuildJobInfoEntryKey(buildId, structs.PrefixBuild, namespace, controller, container)))
	// 	push := bucket.Get([]byte(structs.BuildJobInfoEntryKey(buildId, structs.PrefixPush, namespace, controller, container)))
	// 	result = structs.CreateBuildJobInfo(job.Image, clone, ls, login, build, push)

	// 	result.BuildId = buildId
	// 	result.Namespace = namespace
	// 	result.Controller = controller
	// 	result.Container = container

	// 	if containerObj.GitCommitHash != nil {
	// 		result.CommitHash = *containerObj.GitCommitHash
	// 	}
	// 	if containerObj.GitRepository != nil && containerObj.GitCommitHash != nil {
	// 		result.CommitLink = *utils.GitCommitLink(*containerObj.GitRepository, *containerObj.GitCommitHash)
	// 	}
	// 	if containerObj.GitCommitAuthor != nil {
	// 		result.CommitAuthor = *containerObj.GitCommitAuthor
	// 	}
	// 	if containerObj.GitCommitMessage != nil {
	// 		result.CommitMessage = *containerObj.GitCommitMessage
	// 	}
	// 	return nil
	// })
	// if err != nil {
	// 	self.logger.Error("GetBuildJobFromDb (db)", "error", err)
	// }

	// return result
}

func (self *redisBuildDbModule) GetBuildJobInfosListFromDb(namespace string, controller string, container string) []structs.BuildJobInfo {
	results, err := redisstore.GetObjectsByPrefix[structs.BuildJobInfo](self.redis.GetContext(), self.redis.GetClient(), redisstore.ORDER_ASC, DB_BUILD_BUCKET_NAME, namespace, controller, container)
	if err != nil {
		self.logger.Error("GetBuildJobInfosListFromDb", "error", err)
	}

	if len(results) > 20 {
		results = results[:20]
	}
	return results
}

func (self *redisBuildDbModule) DeleteAllBuildData(namespace string, controller string, container string) {
	err := self.redis.Delete(DB_BUILD_BUCKET_NAME, namespace, controller, container)
	if err != nil {
		self.logger.Error("DeleteAllBuildData", "error", err.Error())
	}
}

// TODO: potentially broken
func (self *redisBuildDbModule) GetLastBuildForNamespaceAndControllerName(namespace string, controllerName string) structs.BuildJobInfo {
	result := structs.BuildJobInfo{}
	lastBuildJob, err := redisstore.GetObjectForKey[structs.BuildJobInfo](self.redis.GetContext(), self.redis.GetClient(), DB_BUILD_BUCKET_NAME, namespace, controllerName)
	if err != nil {
		self.logger.Error("GetLastBuildJobInfosFromDb", "error", err)
		return result
	}
	result = self.GetBuildJobInfosFromDb(lastBuildJob.BuildId)
	return result
}

func (self *redisBuildDbModule) UpdateStateInDb(buildJob structs.BuildJob, newState structs.JobStateEnum) {
	buildJob.State = newState
	err := self.redis.SetObject(buildJob, DefaultBuildTtl, DB_BUILD_BUCKET_NAME, self.buildJobKey(buildJob.BuildId))
	if err != nil {
		self.logger.Error("Error saving job.", "buildId", buildJob.BuildId, "error", err)
	}
}

func (self *redisBuildDbModule) SaveBuildResult(
	state structs.JobStateEnum,
	prefix structs.BuildPrefixEnum,
	cmdOutput string,
	startTime time.Time,
	job *structs.BuildJob,
	container *dtos.K8sContainerDto,
) error {
	entry := structs.CreateBuildJobInfoEntry(state, cmdOutput, startTime, time.Now(), prefix, job, container)
	return self.redis.SetObject(entry, DefaultBuildTtl, DB_BUILD_BUCKET_NAME, self.buildJobKey(job.BuildId), job.Namespace.Name, job.Service.ControllerName, container.Name)
}

func (self *redisBuildDbModule) GetBuildJobListFromDb() []structs.BuildJob {
	result, err := redisstore.GetObjectsByPrefix[structs.BuildJob](self.redis.GetContext(), self.redis.GetClient(), redisstore.ORDER_ASC, DB_BUILD_BUCKET_NAME, DB_PREFIX_QUEUE)
	if err != nil {
		self.logger.Error("GetBuildJobListFromDb", "error", err)
	}
	return result
}

func (self *redisBuildDbModule) DeleteBuildJobFromDb(bucket string, buildId int64) error {
	return self.redis.Delete(DB_BUILD_BUCKET_NAME, self.buildJobKey(buildId))
}

func (self *redisBuildDbModule) AddToDb(buildJob structs.BuildJob) (int, error) {
	// setup usefull defaults
	if buildJob.JobId == "" {
		buildJob.JobId = utils.NanoId()
	}
	if buildJob.State == "" {
		buildJob.State = structs.JobStatePending
	}

	nextBuildId, err := self.getNextBuildID()
	if err != nil {
		return 0, err
	}

	buildJob.BuildId = nextBuildId
	err = self.redis.SetObject(buildJob, DefaultBuildTtl, DB_BUILD_BUCKET_NAME, self.buildJobKey(buildJob.BuildId))
	if err != nil {
		self.logger.Error("Error saving job.", "buildId", buildJob.BuildId, "error", err)
	}
	return int(nextBuildId), err
}

func (self *redisBuildDbModule) getNextBuildID() (int64, error) {
	buildID, err := self.redis.GetClient().Incr(self.redis.GetContext(), "AUTO_INCREMENTS:current-build-id").Result()
	if err != nil {
		return 0, err
	}
	return buildID, nil
}

func (self *redisBuildDbModule) ImageNamesFromBuildJob(buildJob structs.BuildJob) (imageName string, imageTag string, imageTagLatest string) {
	imageName = fmt.Sprintf("%s-%s", buildJob.Namespace.Name, buildJob.Service.ControllerName)
	// overwrite images name for local builds
	if buildJob.Project.ContainerRegistryPat == nil {
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

func (self *redisBuildDbModule) SaveJobInDb(buildJob structs.BuildJob) {
	err := self.redis.SetObject(buildJob, DefaultBuildTtl, DB_BUILD_BUCKET_NAME, self.buildJobKey(buildJob.BuildId))
	if err != nil {
		self.logger.Error("Error saving job.", "buildId", buildJob.BuildId, "error", err)
	}
}
