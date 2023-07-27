package builder

import (
	"bytes"
	"context"
	"fmt"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"os"
	"os/exec"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	BUCKET_NAME = "mogenius"

	PREFIX_GIT_CLONE = "git-clone-"
	PREFIX_LS        = "ls-"
	PREFIX_LOGIN     = "login-"
	PREFIX_BUILD     = "build-"
	PREFIX_PUSH      = "push-"
	PREFIX_SCAN      = "scan-"
	PREFIX_QUEUE     = "queue-"

	PREFIX_CLEANUP = "cleanup"
)

var db *bolt.DB
var currentBuildContext *context.Context
var currentBuildChannel chan string
var currentBuildJob *structs.BuildJob

func Init() {
	database, err := bolt.Open(utils.CONFIG.Kubernetes.BboltDbPath, 0600, nil)
	if err != nil {
		logger.Log.Fatal(err)
	}
	db = database
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BUCKET_NAME))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logger.Log.Errorf("Error creating bucket ('%s'): %s", BUCKET_NAME, err)
	}
	logger.Log.Noticef("bbold started 🚀 (Path: '%s')", utils.CONFIG.Kubernetes.BboltDbPath)
}

func ProcessQueue() {
	jobsToBuild := []structs.BuildJob{}
	db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(BUCKET_NAME)).Cursor()
		prefix := []byte(PREFIX_QUEUE)
		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
			job := structs.BuildJob{}
			err := structs.UnmarshalJob(&job, jobData)
			if err == nil {
				if job.State == structs.BUILD_STATE_PENDING {
					jobsToBuild = append(jobsToBuild, job)
				}
			} else {
				logger.Log.Errorf("ProcessQueue (unmarshall) ERR: %s", err.Error())
			}
		}
		return nil
	})
	// this must happen outside the transaction to avoid dead-locks
	logger.Log.Noticef("Queued %d jobs in build-queue.", len(jobsToBuild))
	for _, buildJob := range jobsToBuild {
		currentBuildChannel = make(chan string, 1)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(utils.CONFIG.Builder.BuildTimeout))
		currentBuildContext = &ctx
		currentBuildJob = &buildJob
		defer cancel()

		job := structs.CreateJob(fmt.Sprintf("Building '%s' ...", buildJob.ServiceName), buildJob.ProjectId, &buildJob.NamespaceId, nil)

		go build(job, &buildJob, currentBuildChannel, &ctx)

		select {
		case <-ctx.Done():
			logger.Log.Errorf("BUILD TIMEOUT (after %ds)! (%s)", utils.CONFIG.Builder.BuildTimeout, ctx.Err())
			job.State = structs.BUILD_STATE_TIMEOUT
			buildJob.State = structs.BUILD_STATE_TIMEOUT
			saveJob(buildJob)
		case result := <-currentBuildChannel:
			switch result {
			case structs.BUILD_STATE_CANCELED:
				logger.Log.Warningf("Build '%d' CANCELED successfuly. (Took: %dms)", buildJob.BuildId, buildJob.DurationMs)
			case structs.BUILD_STATE_FAILED:
				logger.Log.Errorf("Build '%d' FAILDED. (Took: %dms)", buildJob.BuildId, buildJob.DurationMs)
			case structs.BUILD_STATE_SUCCEEDED:
				logger.Log.Noticef("Build '%d' finished successfuly. (Took: %dms)", buildJob.BuildId, buildJob.DurationMs)
			default:
				logger.Log.Errorf("Unhandled channelMsg for '%d': %s", buildJob.BuildId, result)
			}

			job.State = result
			buildJob.State = result
			job.Finish()
			saveJob(buildJob)
		}

		currentBuildContext = nil
		currentBuildChannel = nil
		currentBuildJob = nil
	}
}

func build(job structs.Job, buildJob *structs.BuildJob, done chan string, timeoutCtx *context.Context) {
	job.Start()

	pwd, _ := os.Getwd()
	workingDir := fmt.Sprintf("%s/temp/%d", pwd, buildJob.BuildId)

	defer func() {
		// reset everything if done
		if !utils.CONFIG.Misc.Debug {
			executeCmd(nil, PREFIX_CLEANUP, buildJob, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("rm -rf %s", workingDir))
		}
		done <- structs.BUILD_STATE_SUCCEEDED
	}()

	logger.Log.Noticef("Build '%d' starting ...", buildJob.BuildId)

	updateState(*buildJob, structs.BUILD_STATE_STARTED)

	imageName := fmt.Sprintf("%s-%s", buildJob.Namespace, buildJob.ServiceName)
	tagName := fmt.Sprintf("%s/%s:%d", buildJob.ContainerRegistryPath, imageName, buildJob.BuildId)
	latestTagName := fmt.Sprintf("%s/%s:latest", buildJob.ContainerRegistryPath, imageName)

	// CLEANUP
	if !utils.CONFIG.Misc.Debug {
		executeCmd(nil, PREFIX_CLEANUP, buildJob, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("rm -rf %s", workingDir))
	}

	// CLONE
	cloneCmd := structs.CreateCommand("Cloning repository ...", &job)
	err := executeCmd(cloneCmd, PREFIX_GIT_CLONE, buildJob, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("git clone --progress -b %s --single-branch %s %s", buildJob.GitBranch, buildJob.GitRepo, workingDir))
	if err != nil {
		logger.Log.Errorf("Error%s: %s", PREFIX_GIT_CLONE, err.Error())
		done <- structs.BUILD_STATE_FAILED
		return
	}

	// LS
	lsCmd := structs.CreateCommand("Listing contents ...", &job)
	err = executeCmd(lsCmd, PREFIX_LS, buildJob, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("ls -lisa %s", workingDir))
	if err != nil {
		logger.Log.Errorf("Error%s: %s", PREFIX_LS, err.Error())
		done <- structs.BUILD_STATE_FAILED
		return
	}

	// LOGIN
	loginCmd := structs.CreateCommand("Authentificating with container registry ...", &job)
	err = executeCmd(loginCmd, PREFIX_LOGIN, buildJob, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("podman login %s -u %s -p %s", buildJob.ContainerRegistryUrl, buildJob.ContainerRegistryUser, buildJob.ContainerRegistryPat))
	if err != nil {
		logger.Log.Errorf("Error%s: %s", PREFIX_LOGIN, err.Error())
		done <- structs.BUILD_STATE_FAILED
		return
	}

	// BUILD
	buildCmd := structs.CreateCommand("Building container ...", &job)
	err = executeCmd(buildCmd, PREFIX_BUILD, buildJob, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("cd %s; podman build -f %s %s -t %s -t %s %s", workingDir, buildJob.DockerFile, buildJob.InjectDockerEnvVars, tagName, latestTagName, buildJob.DockerContext))
	if err != nil {
		logger.Log.Errorf("Error%s: %s", PREFIX_BUILD, err.Error())
		done <- structs.BUILD_STATE_FAILED
		return
	}

	// PUSH
	pushCmd := structs.CreateCommand("Pushing container ...", &job)
	err = executeCmd(pushCmd, PREFIX_PUSH, buildJob, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("podman push %s", tagName))
	if err != nil {
		logger.Log.Errorf("Error%s: %s", PREFIX_PUSH, err.Error())
		done <- structs.BUILD_STATE_FAILED
		return
	}
	err = executeCmd(pushCmd, PREFIX_PUSH, buildJob, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("podman push %s", latestTagName))
	if err != nil {
		logger.Log.Errorf("Error%s: %s", PREFIX_PUSH, err.Error())
		done <- structs.BUILD_STATE_FAILED
		return
	}

	// SCAN
	Scan(*buildJob, false)
}

func Scan(buildJob structs.BuildJob, login bool) structs.BuildScanResult {
	job := structs.CreateJob(fmt.Sprintf("Vulnerability scan in build '%s' ...", buildJob.ServiceName), buildJob.ProjectId, &buildJob.NamespaceId, nil)

	imageName := fmt.Sprintf("%s-%s", buildJob.Namespace, buildJob.ServiceName)
	result := structs.BuildScanResult{Result: fmt.Sprintf("Scan of '%s' started ...", imageName), Error: ""}

	go func() {
		job.Start()

		latestTagName := fmt.Sprintf("podman:%s/%s:latest", buildJob.ContainerRegistryPath, imageName)
		pwd, _ := os.Getwd()
		grypeTemplate := fmt.Sprintf("%s/grype-json-template", pwd)

		ctxTimeout, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(utils.CONFIG.Builder.BuildTimeout))
		defer cancel()

		// LOGIN
		if login {
			loginCmd := structs.CreateCommand("Authentificating with container registry ...", &job)
			err := executeCmd(loginCmd, PREFIX_LOGIN, &buildJob, true, &ctxTimeout, "/bin/sh", "-c", fmt.Sprintf("podman login %s -u %s -p %s", buildJob.ContainerRegistryUrl, buildJob.ContainerRegistryUser, buildJob.ContainerRegistryPat))
			if err != nil {
				logger.Log.Errorf("Error%s: %s", PREFIX_LOGIN, err.Error())
				result.Error = err.Error()
				return
			}
		}

		// START PODMAN VM
		startVmCmd := structs.CreateCommand("Starting VM for podman ...", &job)
		err := executeCmd(startVmCmd, PREFIX_SCAN, &buildJob, true, &ctxTimeout, "/bin/sh", "-c", "podman machine start")
		if err != nil {
			logger.Log.Errorf("Error%s: %s", PREFIX_SCAN, err.Error())
			result.Error = err.Error()
			return
		}

		// SCAN
		scanCmd := structs.CreateCommand("Scanning for vulnerabilities ...", &job)
		err = executeCmd(scanCmd, PREFIX_SCAN, &buildJob, true, &ctxTimeout, "/bin/sh", "-c", fmt.Sprintf("grype %s --add-cpes-if-none -q -o template -t %s", latestTagName, grypeTemplate))
		if err != nil {
			logger.Log.Errorf("Error%s: %s", PREFIX_SCAN, err.Error())
			result.Error = err.Error()
			return
		}

		// STOP PODMAN VM
		stopVmCmd := structs.CreateCommand("Stopping VM for podman ...", &job)
		err = executeCmd(stopVmCmd, PREFIX_SCAN, &buildJob, true, &ctxTimeout, "/bin/sh", "-c", "podman machine stop")
		if err != nil {
			logger.Log.Errorf("Error%s: %s", PREFIX_SCAN, err.Error())
			result.Error = err.Error()
			return
		}

		// RETURN RESULT
		db.View(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(BUCKET_NAME))
			result.Result = string(bucket.Get([]byte(fmt.Sprintf("%s%d", PREFIX_SCAN, buildJob.BuildId))))
			return nil
		})
		job.Finish()
	}()
	return result
}

func BuilderStatus() structs.BuilderStatus {
	result := structs.BuilderStatus{}

	db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(BUCKET_NAME)).Cursor()
		prefix := []byte(PREFIX_QUEUE)
		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
			job := structs.BuildJob{}
			err := structs.UnmarshalJob(&job, jobData)
			if err == nil {
				result.TotalBuilds++
				result.TotalBuildTimeMs += job.DurationMs
				if job.State == structs.BUILD_STATE_PENDING {
					result.QueuedBuilds++
				}
			}
		}
		return nil
	})

	return result
}

func BuildJobInfos(buildId int) structs.BuildJobInfos {
	result := structs.BuildJobInfos{}
	db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUCKET_NAME))
		clone := bucket.Get([]byte(fmt.Sprintf("%s%d", PREFIX_GIT_CLONE, buildId)))
		ls := bucket.Get([]byte(fmt.Sprintf("%s%d", PREFIX_LS, buildId)))
		login := bucket.Get([]byte(fmt.Sprintf("%s%d", PREFIX_LOGIN, buildId)))
		build := bucket.Get([]byte(fmt.Sprintf("%s%d", PREFIX_BUILD, buildId)))
		push := bucket.Get([]byte(fmt.Sprintf("%s%d", PREFIX_PUSH, buildId)))
		scan := bucket.Get([]byte(fmt.Sprintf("%s%d", PREFIX_SCAN, buildId)))
		result = structs.CreateBuildJobInfos(buildId, clone, ls, login, build, push, scan)
		return nil
	})

	return result
}

func Add(buildJob structs.BuildJob) structs.BuildAddResult {
	nextBuildId := -1

	if buildJob.InjectDockerEnvVars == "" {
		buildJob.InjectDockerEnvVars = "--build-arg PLACEHOLDER=MOGENIUS"
	}

	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUCKET_NAME))

		// FIRST: CHECK FOR DUPLICATES
		c := bucket.Cursor()
		prefix := []byte(PREFIX_QUEUE)
		nextBuildId = 1
		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
			job := structs.BuildJob{}
			err := structs.UnmarshalJob(&job, jobData)
			if err == nil {
				// THIS IS A FILTER TO HANDLE DUPLICATED REQUESTS
				// if job.GitCommitHash == buildJob.GitCommitHash {
				// 	err = fmt.Errorf("Duplicate BuildJob '%s (%s)' found. Not adding to Queue.", job.ServiceName, job.GitCommitHash)
				// 	logger.Log.Error(err.Error())
				// 	return err
				// }
				nextBuildId++
			}
		}
		buildJob.BuildId = int(nextBuildId)
		return bucket.Put([]byte(fmt.Sprintf("%s%d", PREFIX_QUEUE, nextBuildId)), []byte(structs.PrettyPrintString(buildJob)))
	})
	if err != nil {
		logger.Log.Errorf("Error adding job '%d' to bucket. REASON: %s", nextBuildId, err.Error())
		return structs.BuildAddResult{BuildId: -1}
	}

	go ProcessQueue()

	return structs.BuildAddResult{BuildId: buildJob.BuildId}
}

func Cancel(buildNo int) structs.BuildCancelResult {
	// CANCEL PROCESS
	if currentBuildContext != nil {
		if currentBuildJob != nil {
			if currentBuildJob.BuildId == buildNo {
				currentBuildChannel <- structs.BUILD_STATE_CANCELED
				return structs.BuildCancelResult{Result: fmt.Sprintf("Build '%d' canceled successfuly.", buildNo)}
			} else {
				return structs.BuildCancelResult{Error: fmt.Sprintf("Error: Build '%d' not running.", buildNo)}
			}
		}
	}
	return structs.BuildCancelResult{Error: "Error: No active build jobs found."}
}

func Delete(buildNo int) structs.BuildDeleteResult {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUCKET_NAME))
		return bucket.Delete([]byte(fmt.Sprintf("%s%d", PREFIX_QUEUE, buildNo)))
	})
	if err != nil {
		errStr := fmt.Sprintf("Error deleting build '%d' in bucket. REASON: %s", buildNo, err.Error())
		logger.Log.Error(errStr)
		return structs.BuildDeleteResult{Error: errStr}
	}
	return structs.BuildDeleteResult{Result: fmt.Sprintf("Build '%d' deleted successfuly (or has been deleted before).", buildNo)}
}

func ListAll() []structs.BuildJobListEntry {
	result := []structs.BuildJobListEntry{}
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUCKET_NAME))
		c := bucket.Cursor()
		prefix := []byte(PREFIX_QUEUE)
		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
			job := structs.BuildJobListEntry{}
			err := structs.UnmarshalJobListEntry(&job, jobData)
			if err != nil {
				return err
			}
			result = append(result, job)
		}
		return nil
	})
	if err != nil {
		logger.Log.Errorf("list: %s", err.Error())
	}
	return result
}

func ListByProjectId(projectId string) []structs.BuildJobListEntry {
	result := []structs.BuildJobListEntry{}

	list := ListAll()
	for _, queueEntry := range list {
		if queueEntry.ProjectId == projectId {
			result = append(result, queueEntry)
		}
	}
	return result
}

func executeCmd(reportCmd *structs.Command, prefix string, job *structs.BuildJob, saveLog bool, timeoutCtx *context.Context, name string, arg ...string) error {
	startTime := time.Now()

	if reportCmd != nil {
		reportCmd.Start(reportCmd.Message)
	}

	cmd := exec.CommandContext(*timeoutCtx, name, arg...)
	cmdOutput, execErr := cmd.CombinedOutput()
	elapsedTime := time.Since(startTime)

	job.DurationMs = int(elapsedTime.Milliseconds()) + job.DurationMs + 1 // adding one ms as default penelty for every step (sometimes the steps take lass time. only microseconds)
	if execErr != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), execErr)
		logger.Log.Errorf("Error: %s", string(cmdOutput))
		if reportCmd != nil {
			reportCmd.Fail(execErr.Error())
		}
	}
	if utils.CONFIG.Misc.Debug {
		logger.Log.Noticef("%s%d: %dms", prefix, job.BuildId, job.DurationMs)
		logger.Log.Noticef("%s%d: %s", prefix, job.BuildId, cmd.String())
		logger.Log.Infof("%s%d: %s", prefix, job.BuildId, string(cmdOutput))
	}
	if saveLog {
		err := db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(BUCKET_NAME))
			if reportCmd != nil && execErr == nil {
				reportCmd.Success(reportCmd.Message)
			}
			if reportCmd != nil {
				entry := structs.CreateBuildJobInfoEntryBytes(reportCmd.State, cmdOutput)
				return bucket.Put([]byte(fmt.Sprintf("%s%d", prefix, job.BuildId)), entry)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	if execErr != nil {
		logger.Log.Errorf("ErrorExecuteCmd: %s", execErr.Error())
		return execErr
	}

	return execErr
}

func updateState(buildJob structs.BuildJob, newState string) {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUCKET_NAME))
		jobData := bucket.Get([]byte(fmt.Sprintf("%s%d", PREFIX_QUEUE, buildJob.BuildId)))
		job := structs.BuildJob{}
		err := structs.UnmarshalJob(&job, jobData)
		if err == nil {
			job.State = newState
			return bucket.Put([]byte(fmt.Sprintf("%s%d", PREFIX_QUEUE, buildJob.BuildId)), []byte(structs.PrettyPrintString(job)))
		}
		return err
	})
	if err != nil {
		errStr := fmt.Sprintf("Error updating state for build '%d'. REASON: %s", buildJob.BuildId, err.Error())
		logger.Log.Error(errStr)
	}
	logger.Log.Infof(fmt.Sprintf("State for build '%d' updated successfuly to '%s'.", buildJob.BuildId, newState))
}

func positionInQueue(buildId int) int {
	positionInQueue := 0

	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUCKET_NAME))

		// FIRST: CHECK FOR DUPLICATES
		c := bucket.Cursor()
		prefix := []byte(PREFIX_QUEUE)
		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
			job := structs.BuildJob{}
			err := structs.UnmarshalJob(&job, jobData)
			if err == nil {
				if job.State == structs.BUILD_STATE_PENDING && job.BuildId != buildId {
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

func saveJob(buildJob structs.BuildJob) {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUCKET_NAME))
		return bucket.Put([]byte(fmt.Sprintf("%s%d", PREFIX_QUEUE, buildJob.BuildId)), []byte(structs.PrettyPrintString(buildJob)))
	})
	if err != nil {
		logger.Log.Errorf("Error saving job '%d'.", buildJob.BuildId)
	}
}

func printAllEntries(prefix string) {
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUCKET_NAME))
		c := bucket.Cursor()
		prefix := []byte(prefix)
		for k, jobData := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, jobData = c.Next() {
			job := structs.BuildJob{}
			err := structs.UnmarshalJob(&job, jobData)
			if err != nil {
				logger.Log.Noticef("key=%s, value=%s\n", k, job.BuildId)
			}
		}
		return nil
	})
	if err != nil {
		logger.Log.Errorf("printAllEntries: %s", err.Error())
	}
}
