package builder

import (
	"bufio"
	"context"
	"fmt"
	"mogenius-k8s-manager/db"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"os"
	"os/exec"
	"strings"
	"time"

	punqStructs "github.com/mogenius/punq/structs"
	punqUtils "github.com/mogenius/punq/utils"

	log "github.com/sirupsen/logrus"
)

var DISABLEQUEUE bool = true

var currentBuildContext *context.Context
var currentBuildChannel chan punqStructs.JobStateEnum
var currentBuildJob *structs.BuildJob
var currentNumberOfRunningJobs int = 0

func ProcessQueue() {
	if DISABLEQUEUE || currentNumberOfRunningJobs >= utils.CONFIG.Builder.MaxConcurrentBuilds {
		time.Sleep(3 * time.Second)
		ProcessQueue()
		return
	}

	jobsToBuild := db.GetJobsToBuildFromDb()

	// this must happen outside the transaction to avoid dead-locks
	log.Infof("Queued %d/%d jobs in build-queue.", len(jobsToBuild), utils.CONFIG.Builder.MaxConcurrentBuilds)
	for _, buildJob := range jobsToBuild {
		for _, container := range buildJob.Service.Containers {
			// only build git-repositories
			if container.Type != dtos.CONTAINER_GIT_REPOSITORY {
				continue
			}

			currentBuildChannel = make(chan punqStructs.JobStateEnum, 1)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(utils.CONFIG.Builder.BuildTimeout))
			currentBuildContext = &ctx
			currentBuildJob = &buildJob
			defer cancel()

			currentNumberOfRunningJobs++
			job := structs.CreateJob(fmt.Sprintf("[%d/%d] Building '%s' (commit: %s)", currentNumberOfRunningJobs, utils.CONFIG.Builder.MaxConcurrentBuilds, buildJob.Service.ControllerName, *container.GitCommitHash), buildJob.Project.Id, &buildJob.Namespace.Id, &buildJob.Service.Id)

			go build(job, &buildJob, &container, currentBuildChannel, &ctx)

			select {
			case <-ctx.Done():
				log.Errorf("BUILD TIMEOUT (after %ds)! (%s)", utils.CONFIG.Builder.BuildTimeout, ctx.Err())
				job.State = punqStructs.JobStateTimeout
				buildJob.State = punqStructs.JobStateTimeout
				saveJob(buildJob)
			case result := <-currentBuildChannel:
				switch result {
				case punqStructs.JobStateTimeout:
					log.Warningf("Build '%d' CANCELED successfuly. (Took: %dms)", buildJob.BuildId, buildJob.DurationMs)
				case punqStructs.JobStateFailed:
					log.Errorf("Build '%d' FAILDED. (Took: %dms)", buildJob.BuildId, buildJob.DurationMs)
				case punqStructs.JobStateSucceeded:
					log.Infof("Build '%d' finished successfuly. (Took: %dms)", buildJob.BuildId, buildJob.DurationMs)
				default:
					log.Errorf("Unhandled channelMsg for '%d': %s", buildJob.BuildId, result)
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
			currentNumberOfRunningJobs--
		}
	}
}

func build(job structs.Job, buildJob *structs.BuildJob, container *dtos.K8sContainerDto, done chan punqStructs.JobStateEnum, timeoutCtx *context.Context) {
	job.Start()

	pwd, _ := os.Getwd()
	workingDir := fmt.Sprintf("%s/temp/%d", pwd, buildJob.BuildId)

	defer func() {
		// reset everything if done
		if !utils.CONFIG.Misc.Debug {
			executeCmd(nil, db.PREFIX_CLEANUP, buildJob, container, nil, false, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("rm -rf %s", workingDir))
		}
		done <- punqStructs.JobStateSucceeded
	}()

	updateState(*buildJob, punqStructs.JobStateStarted)

	imageName := fmt.Sprintf("%s-%s", buildJob.Namespace.Name, buildJob.Service.ControllerName)
	tagName := fmt.Sprintf("%s/%s:%d", *buildJob.Project.ContainerRegistryPath, imageName, buildJob.BuildId)
	latestTagName := fmt.Sprintf("%s/%s:latest", *buildJob.Project.ContainerRegistryPath, imageName)

	// overwrite images name for local builds
	if buildJob.Project.ContainerRegistryUser == nil && buildJob.Project.ContainerRegistryPat == nil {
		tagName = fmt.Sprintf("%s/%s:%d", utils.CONFIG.Kubernetes.LocalContainerRegistryHost, imageName, buildJob.BuildId)
		latestTagName = fmt.Sprintf("%s/%s:latest", utils.CONFIG.Kubernetes.LocalContainerRegistryHost, imageName)
	}

	// CLEANUP
	if !utils.CONFIG.Misc.Debug {
		executeCmd(nil, db.PREFIX_CLEANUP, buildJob, container, nil, false, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("rm -rf %s", workingDir))
	}

	// CLONE
	cloneCmd := structs.CreateCommandFromBuildJob("Clone repository", buildJob)
	err := executeCmd(cloneCmd, structs.PrefixGitClone, buildJob, container, nil, true, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("git clone --progress -b %s --single-branch %s %s", *container.GitBranch, *container.GitRepository, workingDir))
	if err != nil {
		log.Errorf("Error%s: %s", structs.PrefixGitClone, err.Error())
		done <- punqStructs.JobStateFailed
		return
	}

	// LS
	lsCmd := structs.CreateCommandFromBuildJob("List contents", buildJob)
	err = executeCmd(lsCmd, structs.PrefixLs, buildJob, container, nil, true, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("ls -lisa %s", workingDir))
	if err != nil {
		log.Errorf("Error%s: %s", structs.PrefixLs, err.Error())
		done <- punqStructs.JobStateFailed
		return
	}

	// LOGIN
	if buildJob.Project.ContainerRegistryUser != nil && buildJob.Project.ContainerRegistryPat != nil {
		loginCmd := structs.CreateCommandFromBuildJob("Authenticate with container registry", buildJob)
		err = executeCmd(loginCmd, structs.PrefixLogin, buildJob, container, nil, true, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("docker login %s -u %s -p %s", *buildJob.Project.ContainerRegistryUrl, *buildJob.Project.ContainerRegistryUser, *buildJob.Project.ContainerRegistryPat))
		if err != nil {
			log.Errorf("Error%s: %s", structs.PrefixLogin, err.Error())
			done <- punqStructs.JobStateFailed
			return
		}
	}

	// BUILD
	buildCmd := structs.CreateCommandFromBuildJob("Building container", buildJob)
	err = executeCmd(buildCmd, structs.PrefixBuild, buildJob, container, nil, true, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("cd %s; docker build --network host -f %s %s -t %s -t %s %s", workingDir, *container.DockerfileName, container.GetInjectDockerEnvVars(), tagName, latestTagName, *container.DockerContext))
	if err != nil {
		log.Errorf("Error%s: %s", structs.PrefixBuild, err.Error())
		done <- punqStructs.JobStateFailed
		return
	}

	// PUSH
	pushCmd := structs.CreateCommandFromBuildJob("Pushing container", buildJob)
	err = executeCmd(pushCmd, structs.PrefixPush, buildJob, container, nil, false, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("docker push %s", latestTagName))
	if err != nil {
		log.Errorf("Error%s: %s", structs.PrefixPush, err.Error())
		done <- punqStructs.JobStateFailed
		return
	}
	err = executeCmd(pushCmd, structs.PrefixPush, buildJob, container, nil, true, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("docker push %s", tagName))
	if err != nil {
		log.Errorf("Error%s: %s", structs.PrefixPush, err.Error())
		done <- punqStructs.JobStateFailed
		return
	}

	// UPDATE IMAGE
	setImageCmd := structs.CreateCommandFromBuildJob("Deploying image", buildJob)
	err = updateContainerImage(setImageCmd, buildJob, container.Name, tagName)
	if err != nil {
		log.Errorf("Error-%s: %s", "updateDeploymentImage", err.Error())
		done <- punqStructs.JobStateFailed
		return
	}
}

func Scan(req structs.ScanImageRequest) structs.BuildScanResult {
	if req.ContainerImage == "" {
		imagename, err := kubernetes.GetDeploymentImage(req.NamespaceName, req.ControllerName, req.ContainerName)
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
		log.Infof("Cache missed: %s", cacheMissed.Error())
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
					err := executeCmd(loginCmd, structs.PrefixLogin, nil, nil, &req.ContainerImage, false, false, &ctxTimeout, "/bin/sh", "-c", fmt.Sprintf("echo \"%s\" | docker login %s -u %s --password-stdin", *req.ContainerRegistryPat, *req.ContainerRegistryUrl, *req.ContainerRegistryUser))
					if err != nil {
						log.Errorf("Error%s: %s", structs.PrefixLogin, err.Error())
						result.Result.State = punqStructs.JobStateFailed
						result.Error = &loginCmd.Message
						return
					}
				}
			}
			buildJob := structs.BuildJobFrom(job.Id, req)

			// PULL IMAGE
			pullCmd := structs.CreateCommand("Pull image for vulnerabilities ...", &job)
			job.AddCmd(pullCmd)
			log.Errorf("TODO - pull image for vulnerabilities ...")
			err := executeCmd(pullCmd, structs.PrefixPull, &buildJob, nil, &req.ContainerImage, false, false, &ctxTimeout, "/bin/sh", "-c", fmt.Sprintf("docker pull %s", req.ContainerImage))
			if err != nil {
				log.Errorf("Error%s: %s", structs.PrefixPull, err.Error())
				result.Result.State = punqStructs.JobStateFailed
				result.Error = &pullCmd.Message
				return
			}

			// SCAN
			scanCmd := structs.CreateCommand("Scanning for vulnerabilities", &job)
			job.AddCmd(scanCmd)
			log.Errorf("TODO - pull image for vulnerabilities ...")
			err = executeCmd(scanCmd, db.PREFIX_VUL_SCAN, &buildJob, nil, &req.ContainerImage, true, false, &ctxTimeout, "/bin/sh", "-c", fmt.Sprintf("grype %s --add-cpes-if-none -q -o template -t %s", req.ContainerImage, grypeTemplate))
			if err != nil {
				log.Errorf("Error%s: %s", db.PREFIX_VUL_SCAN, err.Error())
				result.Result.State = punqStructs.JobStateFailed
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

func BuildJobInfos(buildId uint64) structs.BuildJobInfos {
	return db.GetBuildJobInfosFromDb(buildId)
}

func LastBuildInfos(data structs.LastBuildTaskListRequest) structs.BuildJobInfos {
	return db.GetLastBuildJobInfosFromDb(data)
}

func Add(buildJob structs.BuildJob) structs.BuildAddResult {
	nextBuildId, err := db.AddToDb(buildJob)
	if err != nil {
		log.Errorf("Error adding job for '%s/%s'. REASON: %s", buildJob.Namespace.Name, buildJob.Service.ControllerName, err.Error())
		return structs.BuildAddResult{BuildId: nextBuildId}
	}

	go ProcessQueue()

	return structs.BuildAddResult{BuildId: nextBuildId}
}

func Cancel(buildNo uint64) structs.BuildCancelResult {
	// CANCEL PROCESS
	if currentBuildContext != nil {
		if currentBuildJob != nil {
			if currentBuildJob.BuildId == buildNo {
				currentBuildChannel <- punqStructs.JobStateCanceled
				return structs.BuildCancelResult{Result: fmt.Sprintf("Build '%d' canceled successfuly.", buildNo)}
			} else {
				return structs.BuildCancelResult{Error: fmt.Sprintf("Error: Build '%d' not running.", buildNo)}
			}
		}
	}
	return structs.BuildCancelResult{Error: "Error: No active build jobs found."}
}

func Delete(buildNo uint64) structs.BuildDeleteResult {
	err := db.DeleteFromDb(db.BUILD_BUCKET_NAME, db.PREFIX_QUEUE, buildNo)
	if err != nil {
		errStr := fmt.Sprintf("Error deleting build '%d' in bucket. REASON: %s", buildNo, err.Error())
		log.Error(errStr)
		return structs.BuildDeleteResult{Error: errStr}
	}
	return structs.BuildDeleteResult{Result: fmt.Sprintf("Build '%d' deleted successfuly (or has been deleted before).", buildNo)}
}

func ListAll() []structs.BuildJob {
	return db.GetBuildJobListFromDb()
}

func ListByProjectId(projectId string) []structs.BuildJob {
	result := []structs.BuildJob{}

	list := ListAll()
	for _, queueEntry := range list {
		if queueEntry.Project.Id == projectId {
			result = append(result, queueEntry)
		}
	}
	return result
}

func ListByServiceId(serviceId string) []structs.BuildJob {
	result := []structs.BuildJob{}

	list := ListAll()
	for _, queueEntry := range list {
		if queueEntry.Service.Id == serviceId {
			result = append(result, queueEntry)
		}
	}
	return result
}

func ListByServiceByNamespaceAndControllerName(namespace, controllerName string) []structs.BuildJob {
	result := []structs.BuildJob{}

	list := ListAll()
	for _, queueEntry := range list {
		if queueEntry.Service.ControllerName == controllerName && queueEntry.Namespace.Name == namespace {
			result = append(result, queueEntry)
		}
	}
	return result
}

func ListByServiceIds(serviceIds []string) []structs.BuildJob {
	result := []structs.BuildJob{}

	list := ListAll()
	for _, queueEntry := range list {
		if punqUtils.Contains(serviceIds, queueEntry.Service.Id) {
			result = append(result, queueEntry)
		}
	}
	return result
}

func LastNJobsPerService(maxResults int, serviceId string) []structs.BuildJob {
	result := []structs.BuildJob{}

	list := ListByServiceId(serviceId)
	for i := len(list) - 1; i >= 0; i-- {
		if len(result) < maxResults {
			result = append(result, list[i])
		}
	}
	return result
}

func LastNJobsPerServices(maxResults int, serviceIds []string) []structs.BuildJob {
	result := []structs.BuildJob{}

	list := ListByServiceIds(serviceIds)
	for i := len(list) - 1; i >= 0; i-- {
		if len(result) < maxResults {
			result = append(result, list[i])
		}
	}
	return result
}

func LastJobForService(serviceId string) structs.BuildJob {
	result := structs.BuildJob{}

	list := ListByServiceId(serviceId)
	if len(list) > 0 {
		result = list[len(list)-1]
	}
	return result
}

func LastJobForNamespaceAndControllerName(namespace, controllerName string) structs.BuildJob {
	result := structs.BuildJob{}

	list := ListByServiceByNamespaceAndControllerName(namespace, controllerName)
	if len(list) > 0 {
		result = list[len(list)-1]
	}
	return result
}

func LastBuildForService(serviceId string) structs.BuildJobInfos {
	result := structs.BuildJobInfos{}

	var lastJob *structs.BuildJob
	list := ListByServiceId(serviceId)
	if len(list) > 0 {
		lastJob = &list[len(list)-1]
	}

	if lastJob != nil {
		result = BuildJobInfos(lastJob.BuildId)
	}

	return result
}

func LastBuildForNamespaceAndControllerName(namespace, controllerName string) structs.BuildJobInfos {
	result := structs.BuildJobInfos{}

	var lastJob *structs.BuildJob
	list := ListByServiceByNamespaceAndControllerName(namespace, controllerName)
	if len(list) > 0 {
		lastJob = &list[len(list)-1]
	}

	if lastJob != nil {
		result = BuildJobInfos(lastJob.BuildId)
	}

	return result
}

func executeCmd(reportCmd *structs.Command, prefix structs.BuildPrefixEnum, job *structs.BuildJob, container *dtos.K8sContainerDto, containerImageName *string, saveLog bool, enableTimestamp bool, timeoutCtx *context.Context, name string, arg ...string) error {
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
		log.Errorf("Failed to execute command (%s): %v", cmd.String(), execErr)
		log.Errorf("Error: %s", cmdOutput.String())
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
			processLine(enableTimestamp, saveLog, prefix, lineCounter, scanner.Text(), job, container, containerImageName, startTime, reportCmd, &cmdOutput)
			lineCounter++
		}
	}()
	go func() {
		scanner := bufio.NewScanner(stdErr)
		lineCounter := 0
		for scanner.Scan() {
			processLine(enableTimestamp, saveLog, prefix, lineCounter, scanner.Text(), job, container, containerImageName, startTime, reportCmd, &cmdOutput)
			lineCounter++
		}
	}()

	// Waiting for the command to finish
	waitErr := cmd.Wait()

	if waitErr != nil {
		log.Errorf("Failed wait for command (%s): %v", cmd.String(), waitErr)
		log.Errorf("Error: %s", cmdOutput.String())
		if reportCmd != nil {
			reportCmd.Fail(fmt.Sprintf("%s: %s", waitErr.Error(), cmdOutput.String()))
			processLine(enableTimestamp, saveLog, prefix, -1, "", job, container, containerImageName, startTime, reportCmd, &cmdOutput)
			return waitErr
		}
	}
	if utils.CONFIG.Misc.Debug && job != nil {
		elapsedTime := time.Since(startTime)
		job.DurationMs = int(elapsedTime.Milliseconds()) + job.DurationMs
		log.Infof("%s%d: %dms", prefix, job.BuildId, job.DurationMs)
		log.Infof("%s%d: %s", prefix, job.BuildId, cmd.String())
		log.Infof("%s%d: %s", prefix, job.BuildId, cmdOutput.String())
	}

	if reportCmd != nil {
		reportCmd.Success(reportCmd.Message)
		processLine(enableTimestamp, saveLog, prefix, -1, "", job, container, containerImageName, startTime, reportCmd, &cmdOutput)
	}
	return nil
}

func processLine(
	enableTimestamp bool,
	saveLog bool,
	prefix structs.BuildPrefixEnum,
	lineNumber int,
	line string,
	job *structs.BuildJob,
	container *dtos.K8sContainerDto,
	containerImageName *string,
	startTime time.Time,
	reportCmd *structs.Command,
	cmdOutput *strings.Builder,
) {
	newLine := ""
	if enableTimestamp && lineNumber != -1 {
		newLine += time.Now().Format(time.DateTime) + ": "
	}
	newLine += line
	newLine += "\n\r"
	newLine = cleanPasswords(job, newLine)
	cmdOutput.Write([]byte(newLine))
	if saveLog {
		if job != nil {
			elapsedTime := time.Since(startTime)
			job.DurationMs = int(elapsedTime.Milliseconds()) + job.DurationMs
		} else {
			log.Infof("Notice: job is nil")
			return
		}
		if containerImageName == nil {
			db.SaveBuildResult(punqStructs.JobStateEnum(reportCmd.State), prefix, cmdOutput.String(), startTime, job, container)
		} else {
			log.Errorf("TODO - pull image for vulnerabilities ...")
			// db.SaveScanResult(punqStructs.JobStateEnum(reportCmd.State), cmdOutput.String(), startTime, *containerImageName, job)
		}

		// send notification
		//cleanPrefix := prefix
		//if strings.HasSuffix(prefix, "-") {
		//	cleanPrefix, _ = strings.CutSuffix(cleanPrefix, "-")
		//}
		// data := structs.CreateDatagramBuildLogs(prefix, job.Namespace.Name, job.Service.ControllerName, job.Project.Id, newLine, reportCmd.State)
		// send start-signal when first line is received
		if lineNumber == 0 {
			// structs.EventServerSendData(structs.CreateDatagramBuildLogs(prefix, job.Namespace.Name, job.Service.ControllerName, job.Project.Id, "####START####", reportCmd.State), "", "", "", 0)
			data := db.GetBuildJobInfosFromDb(job.BuildId)
			structs.EventServerSendData(structs.CreateDatagramBuildLogs(data), "", "", "", 0)
			//switch prefix {
			//case structs.PrefixGitClone:
			//	structs.EventServerSendData(structs.CreateDatagramBuildLogs(data.Clone), "", "", "", 0)
			//case structs.PrefixLs:
			//	structs.EventServerSendData(structs.CreateDatagramBuildLogs(data.Ls), "", "", "", 0)
			//case structs.PrefixLogin:
			//	structs.EventServerSendData(structs.CreateDatagramBuildLogs(data.Login), "", "", "", 0)
			//case structs.PrefixBuild:
			//	structs.EventServerSendData(structs.CreateDatagramBuildLogs(data.Build), "", "", "", 0)
			//case structs.PrefixPush:
			//	structs.EventServerSendData(structs.CreateDatagramBuildLogs(data.Push), "", "", "", 0)
			//	break
			// }
		}
		// structs.EventServerSendData(data, "", "", "", 0)
	}
}

func cleanPasswords(job *structs.BuildJob, line string) string {
	if job == nil {
		return line
	}
	if job.Project.GitAccessToken != nil {
		line = strings.ReplaceAll(line, *job.Project.GitAccessToken, "****")
	}
	if job.Project.ContainerRegistryPat != nil {
		line = strings.ReplaceAll(line, *job.Project.ContainerRegistryPat, "****")
	}
	if job.Project.ContainerRegistryUser != nil {
		line = strings.ReplaceAll(line, *job.Project.ContainerRegistryUser, "****")
	}
	for _, container := range job.Service.Containers {
		for _, v := range container.EnvVars {
			if v.Type == dtos.EnvVarKeyVault {
				line = strings.ReplaceAll(line, v.Value, "****")
			}
		}
	}
	return line
}

func updateContainerImage(reportCmd *structs.Command, job *structs.BuildJob, containerName string, imageName string) error {
	startTime := time.Now()
	if reportCmd != nil {
		reportCmd.Start(reportCmd.Message)
	}

	var err error
	switch job.Service.Controller {
	case dtos.CRON_JOB:
		err = kubernetes.UpdateCronjobImage(job.Namespace.Name, job.Service.ControllerName, containerName, imageName)
	default:
		err = kubernetes.UpdateDeploymentImage(job.Namespace.Name, job.Service.ControllerName, containerName, imageName)
	}

	elapsedTime := time.Since(startTime)
	job.DurationMs = int(elapsedTime.Milliseconds()) + job.DurationMs + 1
	if err != nil {
		reportCmd.Fail(err.Error())
	} else {
		reportCmd.Success(reportCmd.Message)
	}

	return err
}

func updateState(buildJob structs.BuildJob, newState punqStructs.JobStateEnum) {
	db.UpdateStateInDb(buildJob, newState)
}

func positionInQueue(buildId uint64) int {
	return db.PositionInQueueFromDb(buildId)
}

func saveJob(buildJob structs.BuildJob) {
	db.SaveJobInDb(buildJob)
}
