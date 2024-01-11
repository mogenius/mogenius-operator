package builder

import (
	"bufio"
	"context"
	"fmt"
	"mogenius-k8s-manager/db"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"os"
	"os/exec"
	"strings"
	"time"

	punqUtils "github.com/mogenius/punq/utils"
)

var currentBuildContext *context.Context
var currentBuildChannel chan structs.BuildJobStateEnum
var currentBuildJob *structs.BuildJob

func ProcessQueue() {
	jobsToBuild := db.GetJobsToBuildFromDb()

	// this must happen outside the transaction to avoid dead-locks
	logger.Log.Noticef("Queued %d jobs in build-queue.", len(jobsToBuild))
	for _, buildJob := range jobsToBuild {
		currentBuildChannel = make(chan structs.BuildJobStateEnum, 1)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(utils.CONFIG.Builder.BuildTimeout))
		currentBuildContext = &ctx
		currentBuildJob = &buildJob
		defer cancel()

		job := structs.CreateJob(fmt.Sprintf("Building '%s'", buildJob.ServiceName), buildJob.ProjectId, &buildJob.NamespaceId, nil)

		build(job, &buildJob, currentBuildChannel, &ctx)

		select {
		case <-ctx.Done():
			logger.Log.Errorf("BUILD TIMEOUT (after %ds)! (%s)", utils.CONFIG.Builder.BuildTimeout, ctx.Err())
			job.State = structs.BuildJobStateTimeout
			buildJob.State = structs.BuildJobStateTimeout
			saveJob(buildJob)
		case result := <-currentBuildChannel:
			switch result {
			case structs.BuildJobStateTimeout:
				logger.Log.Warningf("Build '%d' CANCELED successfuly. (Took: %dms)", buildJob.BuildId, buildJob.DurationMs)
			case structs.BuildJobStateFailed:
				logger.Log.Errorf("Build '%d' FAILDED. (Took: %dms)", buildJob.BuildId, buildJob.DurationMs)
			case structs.BuildJobStateSucceeded:
				logger.Log.Noticef("Build '%d' finished successfuly. (Took: %dms)", buildJob.BuildId, buildJob.DurationMs)
			default:
				logger.Log.Errorf("Unhandled channelMsg for '%d': %s", buildJob.BuildId, result)
			}

			job.State = result
			buildJob.EndTimestamp = time.Now().Format(time.RFC3339)
			buildJob.State = result
			job.Finish()
			saveJob(buildJob)
		}

		currentBuildContext = nil
		currentBuildChannel = nil
		currentBuildJob = nil
	}
}

func build(job structs.Job, buildJob *structs.BuildJob, done chan structs.BuildJobStateEnum, timeoutCtx *context.Context) {
	job.Start()

	pwd, _ := os.Getwd()
	workingDir := fmt.Sprintf("%s/temp/%d", pwd, buildJob.BuildId)

	defer func() {
		// reset everything if done
		if !utils.CONFIG.Misc.Debug {
			executeCmd(nil, db.PREFIX_CLEANUP, buildJob, nil, false, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("rm -rf %s", workingDir))
		}
		done <- structs.BuildJobStateSucceeded
	}()

	updateState(*buildJob, structs.BuildJobStateStarted)

	imageName := fmt.Sprintf("%s-%s", buildJob.Namespace, buildJob.ServiceName)
	tagName := fmt.Sprintf("%s/%s:%d", buildJob.ContainerRegistryPath, imageName, buildJob.BuildId)
	latestTagName := fmt.Sprintf("%s/%s:latest", buildJob.ContainerRegistryPath, imageName)

	// overwrite images name for local builds
	if buildJob.ContainerRegistryUser == "" && buildJob.ContainerRegistryPat == "" {
		tagName = fmt.Sprintf("%s/%s:%d", utils.CONFIG.Kubernetes.LocalContainerRegistryHost, imageName, buildJob.BuildId)
		latestTagName = fmt.Sprintf("%s/%s:latest", utils.CONFIG.Kubernetes.LocalContainerRegistryHost, imageName)
	}

	// CLEANUP
	if !utils.CONFIG.Misc.Debug {
		executeCmd(nil, db.PREFIX_CLEANUP, buildJob, nil, false, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("rm -rf %s", workingDir))
	}

	// CLONE
	cloneCmd := structs.CreateCommandFromBuildJob("Clone repository", buildJob)
	err := executeCmd(cloneCmd, db.PREFIX_GIT_CLONE, buildJob, nil, true, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("git clone --progress -b %s --single-branch %s %s", buildJob.GitBranch, buildJob.GitRepo, workingDir))
	if err != nil {
		logger.Log.Errorf("Error%s: %s", db.PREFIX_GIT_CLONE, err.Error())
		done <- structs.BuildJobStateFailed
		return
	}

	// LS
	lsCmd := structs.CreateCommandFromBuildJob("List contents", buildJob)
	err = executeCmd(lsCmd, db.PREFIX_LS, buildJob, nil, true, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("ls -lisa %s", workingDir))
	if err != nil {
		logger.Log.Errorf("Error%s: %s", db.PREFIX_LS, err.Error())
		done <- structs.BuildJobStateFailed
		return
	}

	// LOGIN
	if buildJob.ContainerRegistryUser != "" && buildJob.ContainerRegistryPat != "" {
		loginCmd := structs.CreateCommandFromBuildJob("Authenticate with container registry", buildJob)
		err = executeCmd(loginCmd, db.PREFIX_LOGIN, buildJob, nil, true, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("docker login %s -u %s -p %s", buildJob.ContainerRegistryUrl, buildJob.ContainerRegistryUser, buildJob.ContainerRegistryPat))
		if err != nil {
			logger.Log.Errorf("Error%s: %s", db.PREFIX_LOGIN, err.Error())
			done <- structs.BuildJobStateFailed
			return
		}
	}

	// BUILD
	buildCmd := structs.CreateCommandFromBuildJob("Building container", buildJob)
	err = executeCmd(buildCmd, db.PREFIX_BUILD, buildJob, nil, true, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("cd %s; docker build --network host -f %s %s -t %s -t %s %s", workingDir, buildJob.DockerFile, buildJob.InjectDockerEnvVars, tagName, latestTagName, buildJob.DockerContext))
	if err != nil {
		logger.Log.Errorf("Error%s: %s", db.PREFIX_BUILD, err.Error())
		done <- structs.BuildJobStateFailed
		return
	}

	// PUSH
	pushCmd := structs.CreateCommandFromBuildJob("Pushing container", buildJob)
	err = executeCmd(pushCmd, db.PREFIX_PUSH, buildJob, nil, true, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("docker push %s", tagName))
	if err != nil {
		logger.Log.Errorf("Error%s: %s", db.PREFIX_PUSH, err.Error())
		done <- structs.BuildJobStateFailed
		return
	}
	err = executeCmd(pushCmd, db.PREFIX_PUSH, buildJob, nil, false, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("docker push %s", latestTagName))
	if err != nil {
		logger.Log.Errorf("Error%s: %s", db.PREFIX_PUSH, err.Error())
		done <- structs.BuildJobStateFailed
		return
	}

	// UPDATE IMAGE
	setImageCmd := structs.CreateCommandFromBuildJob("Deploying image", buildJob)
	err = updateDeploymentImage(setImageCmd, buildJob, tagName)
	if err != nil {
		logger.Log.Errorf("Error-%s: %s", "updateDeploymentImage", err.Error())
		done <- structs.BuildJobStateFailed
		return
	}
}

func Scan(req structs.ScanImageRequest) structs.BuildScanResult {
	if req.ContainerImage == "" {
		imagename, err := kubernetes.GetDeploymentImage(req.NamespaceName, req.ServiceName)
		if err != nil || imagename == "" {
			return structs.CreateBuildScanResult("", "Error: No image found in deployment.")
		}
		req.ContainerImage = imagename
	}

	result := structs.CreateBuildScanResult(fmt.Sprintf("Scan of '%s' started ...", req.ContainerImage), "")

	// CHECK IF IMAGE HAS BEEN SCANNED BEFORE (CHECK BOLT DB)
	cachedEntry, cacheMissed := db.GetScannedImageFromCache(req)
	result.Result = &cachedEntry

	if cacheMissed != nil {
		logger.Log.Infof("Cache missed: %s", cacheMissed.Error())
		go func() {
			db.StartScanInCache(cachedEntry, req.ContainerImage)
			job := structs.CreateJob(fmt.Sprintf("Vulnerability scan: '%s'", req.ContainerImage), req.ProjectId, &req.NamespaceId, &req.ServiceId)
			job.Start()

			pwd, _ := os.Getwd()
			grypeTemplate := fmt.Sprintf("%s/grype-json-template", pwd)

			ctxTimeout, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(utils.CONFIG.Builder.BuildTimeout))

			defer func() {
				job.Finish()
				cancel()
			}()
			// LOGIN
			if req.ContainerRegistryUser != nil && req.ContainerRegistryPat != nil {
				if *req.ContainerRegistryUser != "" && *req.ContainerRegistryPat != "" {
					loginCmd := structs.CreateCommand("Authenticate with container registry ...", &job)
					job.AddCmd(loginCmd)
					err := executeCmd(loginCmd, db.PREFIX_LOGIN, nil, &req.ContainerImage, true, false, &ctxTimeout, "/bin/sh", "-c", fmt.Sprintf("echo \"%s\" | docker login %s -u %s --password-stdin", req.ContainerRegistryPat, req.ContainerRegistryUrl, req.ContainerRegistryUser))
					if err != nil {
						logger.Log.Errorf("Error%s: %s", db.PREFIX_LOGIN, err.Error())
						result.Result.State = structs.BuildJobStateFailed
						result.Error = &loginCmd.Message
						return
					}
				}
			}
			buildJob := structs.BuildJobFrom(job.Id, req)

			// PULL IMAGE
			pullCmd := structs.CreateCommand("Pull image for vulnerabilities ...", &job)
			job.AddCmd(pullCmd)
			err := executeCmd(pullCmd, db.PREFIX_PULL, &buildJob, &req.ContainerImage, false, false, &ctxTimeout, "/bin/sh", "-c", fmt.Sprintf("docker pull %s", req.ContainerImage))
			if err != nil {
				logger.Log.Errorf("Error%s: %s", db.PREFIX_PULL, err.Error())
				result.Result.State = structs.BuildJobStateFailed
				result.Error = &pullCmd.Message
				return
			}

			// SCAN
			scanCmd := structs.CreateCommand("Scanning for vulnerabilities", &job)
			job.AddCmd(scanCmd)
			err = executeCmd(scanCmd, db.PREFIX_VUL_SCAN, &buildJob, &req.ContainerImage, true, false, &ctxTimeout, "/bin/sh", "-c", fmt.Sprintf("grype %s --add-cpes-if-none -q -o template -t %s", req.ContainerImage, grypeTemplate))
			if err != nil {
				logger.Log.Errorf("Error%s: %s", db.PREFIX_VUL_SCAN, err.Error())
				result.Result.State = structs.BuildJobStateFailed
				result.Error = &scanCmd.Message
				return
			}
		}()
	}
	return result
}

func BuilderStatus() structs.BuilderStatus {
	return db.GetBuilderStatus()
}

func BuildJobInfos(buildId int) structs.BuildJobInfos {
	return db.GetBuildJobInfosFromDb(buildId)
}

func Add(buildJob structs.BuildJob) structs.BuildAddResult {
	if buildJob.InjectDockerEnvVars == "" {
		buildJob.InjectDockerEnvVars = "--build-arg PLACEHOLDER=MOGENIUS"
	}

	nextBuildId, err := db.AddToDb(buildJob)
	if err != nil {
		logger.Log.Errorf("Error adding job '%d' to bucket. REASON: %s", nextBuildId, err.Error())
		return structs.BuildAddResult{BuildId: nextBuildId}
	}

	go ProcessQueue()

	return structs.BuildAddResult{BuildId: nextBuildId}
}

func Cancel(buildNo int) structs.BuildCancelResult {
	// CANCEL PROCESS
	if currentBuildContext != nil {
		if currentBuildJob != nil {
			if currentBuildJob.BuildId == buildNo {
				currentBuildChannel <- structs.BuildJobStateCanceled
				return structs.BuildCancelResult{Result: fmt.Sprintf("Build '%d' canceled successfuly.", buildNo)}
			} else {
				return structs.BuildCancelResult{Error: fmt.Sprintf("Error: Build '%d' not running.", buildNo)}
			}
		}
	}
	return structs.BuildCancelResult{Error: "Error: No active build jobs found."}
}

func Delete(buildNo int) structs.BuildDeleteResult {
	err := db.DeleteFromDb(db.BUILD_BUCKET_NAME, db.PREFIX_QUEUE, buildNo)
	if err != nil {
		errStr := fmt.Sprintf("Error deleting build '%d' in bucket. REASON: %s", buildNo, err.Error())
		logger.Log.Error(errStr)
		return structs.BuildDeleteResult{Error: errStr}
	}
	return structs.BuildDeleteResult{Result: fmt.Sprintf("Build '%d' deleted successfuly (or has been deleted before).", buildNo)}
}

func ListAll() []structs.BuildJobListEntry {
	return db.GetBuildJobListFromDb()
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

func ListByServiceId(serviceId string) []structs.BuildJobListEntry {
	result := []structs.BuildJobListEntry{}

	list := ListAll()
	for _, queueEntry := range list {
		if queueEntry.ServiceId == serviceId {
			result = append(result, queueEntry)
		}
	}
	return result
}

func ListByServiceByNamespaceAndServiceName(namespace, serviceName string) []structs.BuildJobListEntry {
	result := []structs.BuildJobListEntry{}

	list := ListAll()
	for _, queueEntry := range list {
		if queueEntry.ServiceName == serviceName && queueEntry.Namespace == namespace {
			result = append(result, queueEntry)
		}
	}
	return result
}

func ListByServiceIds(serviceIds []string) []structs.BuildJobListEntry {
	result := []structs.BuildJobListEntry{}

	list := ListAll()
	for _, queueEntry := range list {
		if punqUtils.Contains(serviceIds, queueEntry.ServiceId) {
			result = append(result, queueEntry)
		}
	}
	return result
}

func LastNJobsPerService(maxResults int, serviceId string) []structs.BuildJobListEntry {
	result := []structs.BuildJobListEntry{}

	list := ListByServiceId(serviceId)
	for i := len(list) - 1; i >= 0; i-- {
		if len(result) < maxResults {
			result = append(result, list[i])
		}
	}
	return result
}

func LastNJobsPerServices(maxResults int, serviceIds []string) []structs.BuildJobListEntry {
	result := []structs.BuildJobListEntry{}

	list := ListByServiceIds(serviceIds)
	for i := len(list) - 1; i >= 0; i-- {
		if len(result) < maxResults {
			result = append(result, list[i])
		}
	}
	return result
}

func LastJobForService(serviceId string) structs.BuildJobListEntry {
	result := structs.BuildJobListEntry{}

	list := ListByServiceId(serviceId)
	if len(list) > 0 {
		result = list[len(list)-1]
	}
	return result
}

func LastJobForNamespaceAndServiceName(namespace, serviceName string) structs.BuildJobListEntry {
	result := structs.BuildJobListEntry{}

	list := ListByServiceByNamespaceAndServiceName(namespace, serviceName)
	if len(list) > 0 {
		result = list[len(list)-1]
	}
	return result
}

func LastBuildForService(serviceId string) structs.BuildJobInfos {
	result := structs.BuildJobInfos{}

	var lastJob *structs.BuildJobListEntry
	list := ListByServiceId(serviceId)
	if len(list) > 0 {
		lastJob = &list[len(list)-1]
	}

	if lastJob != nil {
		result = BuildJobInfos(lastJob.BuildId)
	}

	return result
}

func LastBuildForNamespaceAndServiceName(namespace, serviceName string) structs.BuildJobInfos {
	result := structs.BuildJobInfos{}

	var lastJob *structs.BuildJobListEntry
	list := ListByServiceByNamespaceAndServiceName(namespace, serviceName)
	if len(list) > 0 {
		lastJob = &list[len(list)-1]
	}

	if lastJob != nil {
		result = BuildJobInfos(lastJob.BuildId)
	}

	return result
}

func executeCmd(reportCmd *structs.Command, prefix string, job *structs.BuildJob, containerImageName *string, saveLog bool, enableTimestamp bool, timeoutCtx *context.Context, name string, arg ...string) error {
	startTime := time.Now()

	if reportCmd != nil {
		reportCmd.Start(reportCmd.Message)
	}

	// Prioritize the command to 10 (which is lower the default 0)
	// this means the command will get execution time after the paret process
	// arg = append([]string{"nice -n 10"}, arg...)

	// TIMESTAMP EVERY LINE
	// if utils.CONFIG.Misc.Stage != utils.STAGE_LOCAL {
	// 	// PREFIX LINE BUFFER (otherwise the timestamp will be set only in the first line)
	// 	arg[len(arg)-1] = fmt.Sprintf("%s %s", "stdbuf -oL", arg[len(arg)-1])
	// }
	// arg[len(arg)-1] = fmt.Sprintf("%s %s", arg[len(arg)-1], `| while IFS= read -r line; do printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$line"; done`)

	cmd := exec.CommandContext(*timeoutCtx, name, arg...)

	// Goroutine to read the command's standard output
	stdout, stdOutErr := cmd.StdoutPipe()
	if stdOutErr != nil {
		return stdOutErr
	}
	stdErr, stdErrErr := cmd.StderrPipe()
	if stdErrErr != nil {
		return stdErrErr
	}

	var cmdOutput strings.Builder
	execErr := cmd.Start()
	if execErr != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), execErr)
		logger.Log.Errorf("Error: %s", cmdOutput.String())
		if reportCmd != nil {
			reportCmd.Fail(fmt.Sprintf("%s: %s", execErr.Error(), cmdOutput.String()))
			return execErr
		}
	}

	// Collecting the output
	go func() {
		scanner := bufio.NewScanner(stdout)
		lineCounter := 0
		for scanner.Scan() {
			processLine(enableTimestamp, saveLog, prefix, lineCounter, scanner.Text(), job, containerImageName, startTime, reportCmd, &cmdOutput)
			lineCounter++
		}
	}()
	go func() {
		scanner := bufio.NewScanner(stdErr)
		lineCounter := 0
		for scanner.Scan() {
			processLine(enableTimestamp, saveLog, prefix, lineCounter, scanner.Text(), job, containerImageName, startTime, reportCmd, &cmdOutput)
			lineCounter++
		}
	}()

	// Waiting for the command to finish
	execErr = cmd.Wait()

	if execErr != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), execErr)
		logger.Log.Errorf("Error: %s", cmdOutput.String())
		if reportCmd != nil {
			reportCmd.Fail(fmt.Sprintf("%s: %s", execErr.Error(), cmdOutput.String()))
			processLine(enableTimestamp, saveLog, prefix, -1, "", job, containerImageName, startTime, reportCmd, &cmdOutput)
			return execErr
		}
	}
	if utils.CONFIG.Misc.Debug && job != nil {
		elapsedTime := time.Since(startTime)
		job.DurationMs = int(elapsedTime.Milliseconds()) + job.DurationMs
		logger.Log.Noticef("%s%d: %dms", prefix, job.BuildId, job.DurationMs)
		logger.Log.Noticef("%s%d: %s", prefix, job.BuildId, cmd.String())
		logger.Log.Infof("%s%d: %s", prefix, job.BuildId, cmdOutput.String())
	}

	if reportCmd != nil {
		reportCmd.Success(reportCmd.Message)
		processLine(enableTimestamp, saveLog, prefix, -1, "", job, containerImageName, startTime, reportCmd, &cmdOutput)
	}
	return nil
}

func processLine(enableTimestamp bool, saveLog bool, prefix string, lineNumber int, line string, job *structs.BuildJob, containerImageName *string, startTime time.Time, reportCmd *structs.Command, cmdOutput *strings.Builder) {
	newLine := ""
	if enableTimestamp && lineNumber != -1 {
		newLine += time.Now().Format(time.DateTime) + ": "
	}
	newLine += line
	newLine += "\n"
	cmdOutput.Write([]byte(newLine))
	if saveLog {
		if job != nil {
			elapsedTime := time.Since(startTime)
			job.DurationMs = int(elapsedTime.Milliseconds()) + job.DurationMs
		}
		if containerImageName == nil {
			db.SaveBuildResult(structs.BuildJobStateEnum(reportCmd.State), prefix, cmdOutput.String(), startTime, job)
		} else {
			db.SaveScanResult(structs.BuildJobStateEnum(reportCmd.State), cmdOutput.String(), startTime, *containerImageName, job)
		}

		// send notification
		cleanPrefix := prefix
		if strings.HasSuffix(prefix, "-") {
			cleanPrefix, _ = strings.CutSuffix(cleanPrefix, "-")
		}
		data := structs.CreateDatagramBuildLogs(cleanPrefix, job.Namespace, job.ServiceName, job.ProjectId, newLine, reportCmd.State)
		// send start-signal when first line is received
		if lineNumber == 0 {
			structs.EventServerSendData(structs.CreateDatagramBuildLogs(cleanPrefix, job.Namespace, job.ServiceName, job.ProjectId, "####START####", reportCmd.State), "", "", "", 0)
		}
		structs.EventServerSendData(data, "", "", "", 0)
	}
}

func updateDeploymentImage(reportCmd *structs.Command, job *structs.BuildJob, imageName string) error {
	startTime := time.Now()
	if reportCmd != nil {
		reportCmd.Start(reportCmd.Message)
	}

	err := kubernetes.UpdateDeploymentImage(job.Namespace, job.ServiceName, imageName)

	elapsedTime := time.Since(startTime)
	job.DurationMs = int(elapsedTime.Milliseconds()) + job.DurationMs + 1
	if err != nil {
		reportCmd.Fail(err.Error())
	} else {
		reportCmd.Success(reportCmd.Message)
	}

	return err
}

func updateState(buildJob structs.BuildJob, newState structs.BuildJobStateEnum) {
	db.UpdateStateInDb(buildJob, newState)
}

func positionInQueue(buildId int) int {
	return db.PositionInQueueFromDb(buildId)
}

func saveJob(buildJob structs.BuildJob) {
	db.SaveJobInDb(buildJob)
}
