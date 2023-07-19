package builder

import (
	"bytes"
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

func Init() {
	// Open the my.db data file in your current directory.
	// It will be created if it doesn't exist.
	database, err := bolt.Open("mogenius.db", 0600, nil)
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
			}
		}
		return nil
	})
	// this must happen outside the transaction to avoid dead-locks
	logger.Log.Noticef("Queued %d jobs in build-queue.", len(jobsToBuild))
	for _, job := range jobsToBuild {
		go build(job)
	}
}

func build(job structs.BuildJob) {
	pwd, _ := os.Getwd()
	workingDir := fmt.Sprintf("%s/%d", pwd, job.BuildId)

	defer func() {
		// reset everything if done
		executeCmd(PREFIX_CLEANUP, &job, false, "/bin/sh", "-c", "rm -rf", workingDir)
	}()

	updateState(job, structs.BUILD_STATE_STARTED)

	imageName := fmt.Sprintf("%s-%s", job.Namespace, job.ServiceName)
	tagName := fmt.Sprintf("%s/%s:%d", job.ContainerRegistryPath, imageName, job.BuildId)
	latestTagName := fmt.Sprintf("%s/%s:latest", job.ContainerRegistryPath, imageName)

	// CLONE
	// gitOutput=$(git clone --progress -b ${{ parameters.gitBranch }} --single-branch '${{ parameters.gitRepo }}' ${{ parameters.jobId }} 2>&1)
	// cd ${{ parameters.jobId }}
	// ls -lisa
	executeCmd(PREFIX_CLEANUP, &job, false, "/bin/sh", "-c", "rm -rf", workingDir) // CLEANUP FIRST
	err := executeCmd(PREFIX_GIT_CLONE, &job, true, "git", "clone", "--progress", "-b", job.GitBranch, "--single-branch", job.GitRepo, fmt.Sprint(job.BuildId))
	if err != nil {
		logger.Log.Errorf("Error%s: %s", PREFIX_GIT_CLONE, err.Error())
		return
	}

	// LS
	err = executeCmd(PREFIX_LS, &job, true, "/bin/sh", "-c", "ls", "-lisa", workingDir)
	if err != nil {
		logger.Log.Errorf("Error%s: %s", PREFIX_LS, err.Error())
		return
	}

	// LOGIN
	// login=$(echo ${{ parameters.containerRegistryPat }} | docker login ${{ parameters.containerRegistryUrl }} -u ${{ parameters.containerRegistryUser }} --password-stdin 2>&1)
	// echo "XXX" | buildah login docker.io -u biltisberger --password-stdin
	//err = executeCmd(PREFIX_LOGIN, job.BuildId, true, "/bin/sh", "-c", "echo", job.ContainerRegistryPat, "|", "buildah", "login", job.ContainerRegistryUrl, "-u", job.ContainerRegistryUser, "--password-stdin")
	if utils.CONFIG.Misc.Stage == "local" {
		err = executeCmd(PREFIX_LOGIN, &job, true, "/bin/sh", "-c", fmt.Sprintf("podman login %s -u %s -p %s", job.ContainerRegistryUrl, job.ContainerRegistryUser, job.ContainerRegistryPat))
		if err != nil {
			logger.Log.Errorf("Error%s: %s", PREFIX_LOGIN, err.Error())
			return
		}
	} else {
		err = executeCmd(PREFIX_LOGIN, &job, true, "/bin/sh", "-c", fmt.Sprintf("buildah login %s -u %s -p %s", job.ContainerRegistryUrl, job.ContainerRegistryUser, job.ContainerRegistryPat))
		if err != nil {
			logger.Log.Errorf("Error%s: %s", PREFIX_LOGIN, err.Error())
			return
		}
	}

	// BUILD
	// build=$(docker build -f ${{ parameters.dockerFile }} ${{ parameters.injectDockerEnvVars }}-t ${{ parameters.containerRegistryPath }}/$(imageName):$(tag) -t ${{ parameters.containerRegistryPath }}/$(imageName):latest ${{ parameters.dockerContext }} 2>&1)
	// buildah bud -f Dockerfile .
	if utils.CONFIG.Misc.Stage == "local" {
		err = executeCmd(PREFIX_BUILD, &job, true, "/bin/sh", "-c", fmt.Sprintf("podman build -f %s %s -t %s -t %s %s", job.DockerFile, job.InjectDockerEnvVars, tagName, latestTagName, job.DockerContext))
		if err != nil {
			logger.Log.Errorf("Error%s: %s", PREFIX_BUILD, err.Error())
			return
		}
	} else {
		err = executeCmd(PREFIX_BUILD, &job, true, "/bin/sh", "-c", "buildah", "bud", "-f", job.DockerFile, job.InjectDockerEnvVars, "-t", fmt.Sprintf("%s/%s:%d", job.ContainerRegistryPath, imageName, job.BuildId), "-t", fmt.Sprintf("%s/%s:latest", job.ContainerRegistryPath, imageName), job.DockerContext)
		if err != nil {
			logger.Log.Errorf("Error%s: %s", PREFIX_BUILD, err.Error())
			return
		}
	}

	// PUSH
	// push=$(docker image push ${{ parameters.containerRegistryPath }}/$(imageName):$(tag);docker image push ${{ parameters.containerRegistryPath }}/$(imageName):latest 2>&1)
	// buildah push localhost/benetest biltisberger/lalala:latest
	if utils.CONFIG.Misc.Stage == "local" {
		err = executeCmd(PREFIX_PUSH, &job, true, "/bin/sh", "-c", fmt.Sprintf("podman push %s", tagName))
		if err != nil {
			logger.Log.Errorf("Error%s: %s", PREFIX_PUSH, err.Error())
			return
		}
		err = executeCmd(PREFIX_PUSH, &job, true, "/bin/sh", "-c", fmt.Sprintf("podman push %s", latestTagName))
		if err != nil {
			logger.Log.Errorf("Error%s: %s", PREFIX_PUSH, err.Error())
			return
		}
	} else {
		err = executeCmd(PREFIX_PUSH, &job, true, "/bin/sh", "-c", fmt.Sprintf("buildah push %s %s", tagName, latestTagName))
		if err != nil {
			logger.Log.Errorf("Error%s: %s", PREFIX_PUSH, err.Error())
			return
		}
	}
}

func executeCmd(prefix string, job *structs.BuildJob, saveLog bool, name string, arg ...string) error {
	startTime := time.Now()
	cmd := exec.Command(name, arg...)
	cmdOutput, err := cmd.CombinedOutput()
	elapsedTime := time.Since(startTime)

	job.DurationMs = int(elapsedTime.Milliseconds()) + job.DurationMs + 1 // adding one ms as default penelty for every step (sometimes the steps take lass time. only microseconds)
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		logger.Log.Errorf("Error: %s", string(cmdOutput))
	}
	if utils.CONFIG.Misc.Debug {
		logger.Log.Noticef("%s%d: %dms", prefix, job.BuildId, job.DurationMs)
		logger.Log.Noticef("%s%d: %s", prefix, job.BuildId, cmd.String())
		logger.Log.Infof("%s%d: %s", prefix, job.BuildId, string(cmdOutput))
	}
	if saveLog {
		err = db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(BUCKET_NAME))
			return bucket.Put([]byte(fmt.Sprintf("%s%d", prefix, job.BuildId)), []byte(cmdOutput))
		})
	}
	if err != nil {
		logger.Log.Errorf("ErrorExecuteCmd: %s", err.Error())
		return err
	}
	return nil
}

func Scan(job structs.BuildJob) structs.BuildScanResult {
	// scanresult=$(grype ${{ parameters.containerRegistryPath }}/$(imageName):$(tag) --add-cpes-if-none -t /home/build/jsontemplate -o template | base64 -w 0)
	//     echo '{
	//       "projectId": "${{ parameters.projectId }}",
	//       "serviceId": "${{ parameters.serviceId }}",
	//       "buildId": "$(tag)",
	//       "buildState": "FINISHED",
	//       "vulnerabilities": '\""$scanresult"\"'
	//     }' > vulnadata.json
	//     cat vulnadata.json
	//     curl -v --header "Content-Type: application/json" \
	//     --header "apitoken: $(API_KEY)" \
	//     --data @vulnadata.json \
	//     https://platform-api.mogenius.com/azure-pipeline-event/scan-from-pipeline

	imageName := fmt.Sprintf("%s-%s", job.Namespace, job.ServiceName)
	scan := exec.Command("grype", fmt.Sprintf("%s/%s:latest", job.ContainerRegistryPath, imageName), "--add-cpes-if-none", "-t", "/app/grype-json-template", "-o", "template")
	scanOutput, err := scan.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", scan.String(), err)
		logger.Log.Errorf("Error: %s", string(err.Error()))
		return structs.BuildScanResult{Error: err.Error()}
	}
	err = db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUCKET_NAME))
		return bucket.Put([]byte(fmt.Sprintf("%s%d", PREFIX_SCAN, job.BuildId)), []byte(scanOutput))
	})
	if err != nil {
		logger.Log.Errorf("ErrorScan: %s", err.Error())
		return structs.BuildScanResult{Error: err.Error()}
	}

	return structs.BuildScanResult{Result: string(scanOutput)}
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

	ProcessQueue()

	return structs.BuildAddResult{BuildId: buildJob.BuildId}
}

func Cancel(buildNo int) structs.BuildCancelResult {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUCKET_NAME))
		jobData := bucket.Get([]byte(fmt.Sprintf("%s%d", PREFIX_QUEUE, buildNo)))
		job := structs.BuildJob{}
		err := structs.UnmarshalJob(&job, jobData)
		if err != nil {
			job.State = structs.BUILD_STATE_CANCELED
			return bucket.Put([]byte(fmt.Sprintf("%s%d", PREFIX_QUEUE, buildNo)), []byte(structs.PrettyPrintString(job)))
		}
		return err
	})
	if err != nil {
		errStr := fmt.Sprintf("Error canceling build'%d' in bucket. REASON: %s", buildNo, err.Error())
		logger.Log.Error(errStr)
		return structs.BuildCancelResult{Result: errStr}
	}
	return structs.BuildCancelResult{Result: fmt.Sprintf("Build '%d' canceled successfuly.", buildNo)}
}

func Delete(buildNo int) structs.BuildDeleteResult {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUCKET_NAME))
		return bucket.Delete([]byte(fmt.Sprintf("%s%d", PREFIX_QUEUE, buildNo)))
	})
	if err != nil {
		errStr := fmt.Sprintf("Error deleting build '%d' in bucket. REASON: %s", buildNo, err.Error())
		logger.Log.Error(errStr)
		return structs.BuildDeleteResult{Result: errStr}
	}
	return structs.BuildDeleteResult{Result: fmt.Sprintf("Build '%d' deleted successfuly.", buildNo)}
}

func List() []structs.BuildJobListEntry {
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
