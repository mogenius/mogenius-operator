package services

import (
	"bufio"
	"context"
	"fmt"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/db"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/gitmanager"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/shell"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/xterm"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/apps/v1"
	v1job "k8s.io/api/batch/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var DISABLEQUEUE bool = true

var currentBuildContext *context.Context
var currentBuildChannel chan structs.JobStateEnum
var currentBuildJob *structs.BuildJob
var currentNumberOfRunningJobs int = 0

func ProcessQueue() {
	maxConcurrentBuilds, err := strconv.Atoi(config.Get("MO_BUILDER_MAX_CONCURRENT_BUILDS"))
	assert.Assert(err == nil)
	buildTimeout, err := strconv.Atoi(config.Get("MO_BUILDER_BUILD_TIMEOUT"))
	assert.Assert(err == nil)

	if DISABLEQUEUE || currentNumberOfRunningJobs >= maxConcurrentBuilds {
		time.Sleep(3 * time.Second)
		ProcessQueue()
		return
	}

	jobsToBuild := db.GetJobsToBuildFromDb()

	// this must happen outside the transaction to avoid dead-locks
	serviceLogger.Info("Queued jobs in build-queue", "current", len(jobsToBuild), "max", maxConcurrentBuilds)
	for _, buildJob := range jobsToBuild {
		for _, container := range buildJob.Service.Containers {
			// only build git-repositories
			if container.Type != dtos.CONTAINER_GIT_REPOSITORY {
				continue
			}

			currentBuildChannel = make(chan structs.JobStateEnum, 1)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(buildTimeout))
			currentBuildContext = &ctx
			currentBuildJob = &buildJob
			defer cancel()

			currentNumberOfRunningJobs++
			job := structs.CreateJob(fmt.Sprintf("Building '%s'", buildJob.Service.ControllerName), buildJob.Project.Id, buildJob.Namespace.Name, buildJob.Service.ControllerName)
			job.BuildId = buildJob.BuildId
			job.ContainerName = container.Name
			container.GitCommitAuthor = utils.Pointer(utils.Escape(*container.GitCommitAuthor))
			container.GitCommitMessage = utils.Pointer(utils.Escape(*container.GitCommitMessage))

			go build(job, &buildJob, &container, currentBuildChannel, &ctx)

			select {
			case <-ctx.Done():
				buildTimeout, err := strconv.Atoi(config.Get("MO_BUILDER_BUILD_TIMEOUT"))
				assert.Assert(err == nil)
				serviceLogger.Error("BUILD TIMEOUT!", "duration", buildTimeout, "error", ctx.Err())
				job.State = structs.JobStateTimeout
				buildJob.State = structs.JobStateTimeout
				saveJob(buildJob)
			case result := <-currentBuildChannel:
				switch result {
				case structs.JobStateTimeout:
					serviceLogger.Warn("Build CANCELED successfuly.", "buildId", buildJob.BuildId, "duration", buildJob.DurationMs)
				case structs.JobStateFailed:
					serviceLogger.Error("Build FAILED.", "buildId", buildJob.BuildId, "duration", buildJob.DurationMs)
				case structs.JobStateSucceeded:
					serviceLogger.Info("Build finished successfuly.", "buildId", buildJob.BuildId, "duration", buildJob.DurationMs)
				default:
					serviceLogger.Error("Unhandled channelMsg", "buildId", buildJob.BuildId, "result", result)
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

func build(job *structs.Job, buildJob *structs.BuildJob, container *dtos.K8sContainerDto, done chan structs.JobStateEnum, timeoutCtx *context.Context) {
	job.Start()

	// update secrets
	r := ServiceUpdateRequest{
		Project:   buildJob.Project,
		Namespace: buildJob.Namespace,
		Service:   buildJob.Service,
	}
	UpdateSecrets(r)

	pwd, _ := os.Getwd()
	workingDir := fmt.Sprintf("%s/temp/%s", pwd, utils.NanoId())

	moDebug, err := strconv.ParseBool(config.Get("MO_DEBUG"))
	assert.Assert(err == nil)

	defer func() {
		// reset everything if done
		if !moDebug {
			err := executeCmd(job, nil, db.PREFIX_CLEANUP, buildJob, container, false, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("rm -rf %s", workingDir))
			if err != nil {
				serviceLogger.Error("Error cleaning up prefix", "error", err)
			}
		}
		done <- structs.JobStateSucceeded
	}()

	updateState(*buildJob, structs.JobStateStarted)

	_, tagName, latestTagName := db.ImageNamesFromBuildJob(*buildJob)

	// CLEANUP
	if !moDebug {
		err := executeCmd(job, nil, db.PREFIX_CLEANUP, buildJob, container, false, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("rm -rf %s", workingDir))
		if err != nil {
			serviceLogger.Error("Error cleaning up prefix", "error", err)
		}
	}

	// CLONE
	cloneCmd := structs.CreateCommand(string(structs.PrefixGitClone), "Clone repository", job)
	err = gitmanager.CloneFast(*container.GitRepository, workingDir, *container.GitBranch)
	cloneCmd.Success(job, cloneCmd.Message)
	if err != nil {
		serviceLogger.Error("failed to git clone", "error", err)
		cloneCmd.Fail(job, err.Error())
		done <- structs.JobStateFailed
		return
	}

	// TAG
	gitTagData, _ := gitmanager.GetHeadTag(workingDir)
	// in this case we dont care if the tag retrieval fails

	// LS
	lsCmd := structs.CreateCommand(string(structs.PrefixLs), "List contents", job)
	err = executeCmd(job, lsCmd, structs.PrefixLs, buildJob, container, true, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("cd %s; echo 'ℹ️  Current directory contents:'; ls -lisa; echo '\nℹ️  Following ARGs are available for Docker build:'; echo '%s'", workingDir, container.AvailableDockerBuildArgs(job.BuildId, string(gitTagData))))
	if err != nil {
		serviceLogger.Error("failed to list", "error", err)
		done <- structs.JobStateFailed
		return
	}

	// LOGIN
	if buildJob.Project.ContainerRegistryUser != nil && buildJob.Project.ContainerRegistryPat != nil && *buildJob.Project.ContainerRegistryUser != "" && *buildJob.Project.ContainerRegistryPat != "" {
		loginCmd := structs.CreateCommand(string(structs.PrefixLogin), "Authenticate with container registry", job)
		err = executeCmd(job, loginCmd, structs.PrefixLogin, buildJob, container, true, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("docker login %s -u %s -p %s", *buildJob.Project.ContainerRegistryUrl, *buildJob.Project.ContainerRegistryUser, *buildJob.Project.ContainerRegistryPat))
		if err != nil {
			serviceLogger.Error("failed to docker login", "error", err)
			done <- structs.JobStateFailed
			return
		}
	}

	// BUILD
	buildCmd := structs.CreateCommand(string(structs.PrefixBuild), "Building container", job)
	// add dynamic mo-... labels to image metadata
	labels := fmt.Sprintf("--label \"mo-app=%s\" --label \"mo-ns=%s\" --label \"mo-service-id=%s\" --label \"mo-project-id=%s\"", buildJob.Service.ControllerName, buildJob.Namespace.Name, buildJob.Service.Id, buildJob.Project.Id)
	err = executeCmd(job, buildCmd, structs.PrefixBuild, buildJob, container, true, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("cd %s; docker build %s --network host -f %s %s -t %s -t %s %s", workingDir, labels, *container.DockerfileName, container.GetInjectDockerEnvVars(job.NamespaceName, job.BuildId, string(gitTagData)), tagName, latestTagName, *container.DockerContext))
	if err != nil {
		serviceLogger.Error("failed to docker build", "cmd", buildCmd, "error", err)
		done <- structs.JobStateFailed
		return
	}

	//
	// PUSH
	pushCmd := structs.CreateCommand(string(structs.PrefixPush), "Pushing container", job)
	err = executeCmd(job, pushCmd, structs.PrefixPush, buildJob, container, false, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("docker push %s", latestTagName))
	if err != nil {
		serviceLogger.Error("failed to docker push", "cmd", pushCmd, "error", err)
		done <- structs.JobStateFailed
		return
	}
	err = executeCmd(job, pushCmd, structs.PrefixPush, buildJob, container, true, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("docker push %s", tagName))
	if err != nil {
		serviceLogger.Error("failed to docker push", "cmd", pushCmd, "error", err)
		done <- structs.JobStateFailed
		return
	}

	// Update controller
	setImageCmd := structs.CreateCommand("update", "Deploying image", job)
	err = updateController(job, setImageCmd, buildJob, container.Name, tagName, buildJob.CreateAndStart)
	if err != nil {
		serviceLogger.Error("updateDeploymentImage", "error", err)
		done <- structs.JobStateFailed
		return
	}
}

func BuilderStatus() structs.BuilderStatus {
	return db.GetBuilderStatus()
}

func BuildJobInfos(buildId uint64) structs.BuildJobInfo {
	return db.GetBuildJobInfosFromDb(buildId)
}

func LastBuildInfos(data structs.BuildTaskRequest) structs.BuildJobInfo {
	return db.GetLastBuildJobInfosFromDb(data)
}
func LastBuildForNamespaceAndControllerName(namespace string, controllerName string) structs.BuildJobInfo {
	return db.GetLastBuildForNamespaceAndControllerName(namespace, controllerName)
}
func BuildInfosList(data structs.BuildTaskRequest) []structs.BuildJobInfo {
	return db.GetBuildJobInfosListFromDb(data.Namespace, data.Controller, data.Container)
}
func DeleteAllBuildData(data structs.BuildTaskRequest) {
	db.DeleteAllBuildData(data.Namespace, data.Controller, data.Container)
}

func AddBuildJob(buildJob structs.BuildJob) structs.BuildAddResult {
	nextBuildId, err := db.AddToDb(buildJob)
	if err != nil {
		serviceLogger.Error("failed to add job", "namespace", buildJob.Namespace.Name, "controller", buildJob.Service.ControllerName, "error", err)
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
				currentBuildChannel <- structs.JobStateCanceled
				return structs.BuildCancelResult{Result: fmt.Sprintf("Build '%d' canceled successfuly.", buildNo)}
			} else {
				return structs.BuildCancelResult{Error: fmt.Sprintf("Error: Build '%d' not running.", buildNo)}
			}
		}
	}
	return structs.BuildCancelResult{Error: "Error: No active build jobs found."}
}

func DeleteBuild(buildId uint64) structs.BuildDeleteResult {
	err := db.DeleteBuildJobFromDb(db.BUILD_BUCKET_NAME, buildId)
	if err != nil {
		errStr := fmt.Sprintf("Error deleting build '%d' in bucket. REASON: %s", buildId, err.Error())
		serviceLogger.Error(errStr)
		return structs.BuildDeleteResult{Error: errStr}
	}
	return structs.BuildDeleteResult{Result: fmt.Sprintf("Build '%d' deleted successfuly (or has been deleted before).", buildId)}
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

func LastBuildInfosOfServices(data structs.BuildTaskListOfServicesRequest) []structs.BuildJobInfo {
	results := []structs.BuildJobInfo{}

	for _, request := range data.Requests {
		result := db.GetLastBuildJobInfosFromDb(request)
		results = append(results, result)
	}

	return results
}

// func LastJobForService(serviceId string) structs.BuildJob {
// 	result := structs.BuildJob{}

// 	list := ListByServiceId(serviceId)
// 	if len(list) > 0 {
// 		result = list[len(list)-1]
// 	}
// 	return result
// }

// func LastJobForNamespaceAndControllerName(namespace, controllerName string) structs.BuildJob {
// 	result := structs.BuildJob{}

// 	list := ListByServiceByNamespaceAndControllerName(namespace, controllerName)
// 	if len(list) > 0 {
// 		result = list[len(list)-1]
// 	}
// 	return result
// }

// func LastBuildForService(serviceId string) structs.BuildJobInfo {
// 	result := structs.BuildJobInfo{}

// 	var lastJob *structs.BuildJob
// 	list := ListByServiceId(serviceId)
// 	if len(list) > 0 {
// 		lastJob = &list[len(list)-1]
// 	}

// 	if lastJob != nil {
// 		result = BuildJobInfos(lastJob.BuildId)
// 	}

// 	return result
// }

// TODO Remove this code
//var lastBuildForNamespaceAndControllerNameDebounce = utils.NewDebounce("lastBuildForNamespaceAndControllerNameDebounce", 1000*time.Millisecond)
// TODO Remove this code
//func LastBuildForNamespaceAndControllerName(namespace string, controllerName string) structs.BuildJobInfo {
//	key := fmt.Sprintf("%s-%s", namespace, controllerName)
//	result, _ := lastBuildForNamespaceAndControllerNameDebounce.CallFn(key, func() (interface{}, error) {
//		return LastBuildForNamespaceAndControllerName2(namespace, controllerName), nil
//	})
//	return result.(structs.BuildJobInfo)
//}
// TODO Remove this code
//func LastBuildForNamespaceAndControllerName2(namespace string, controllerName string) structs.BuildJobInfo {
//	result := structs.BuildJobInfo{}
//
//	var lastJob *structs.BuildJob
//	list := ListByServiceByNamespaceAndControllerName(namespace, controllerName)
//	if len(list) > 0 {
//		lastJob = &list[len(list)-1]
//	}
//
//	if lastJob != nil {
//		result = BuildJobInfos(lastJob.BuildId)
//	}
//
//	return result
//}

func executeCmd(job *structs.Job, reportCmd *structs.Command, prefix structs.BuildPrefixEnum, buildJob *structs.BuildJob, container *dtos.K8sContainerDto, saveLog bool, enableTimestamp bool, timeoutCtx *context.Context, name string, arg ...string) error {
	startTime := time.Now()

	if reportCmd != nil {
		reportCmd.Start(job, reportCmd.Message)
	}

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
		serviceLogger.Error("Failed to execute command", "cmd", cmd.String(), "cmdOutput", cmdOutput.String(), "error", execErr)
		if reportCmd != nil {
			reportCmd.Fail(job, fmt.Sprintf("%s: %s", execErr.Error(), cmdOutput.String()))
			return execErr
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Collecting the output
	go func() {
		scanner := bufio.NewScanner(stdout)
		lineCounter := 0
		for scanner.Scan() {
			processLine(enableTimestamp, saveLog, prefix, lineCounter, scanner.Text(), buildJob, container, startTime, reportCmd, &cmdOutput)
			lineCounter++
		}
		wg.Done()
	}()
	go func() {
		scanner := bufio.NewScanner(stdErr)
		lineCounter := 0
		for scanner.Scan() {
			processLine(enableTimestamp, saveLog, prefix, lineCounter, scanner.Text(), buildJob, container, startTime, reportCmd, &cmdOutput)
			lineCounter++
		}
		wg.Done()
	}()

	// IMPORTANT: This is blocking the process until the command is finished ON PURPOSE. We were losing logs otherwise.
	// scanner := bufio.NewScanner(stdErr)
	//for scanner.Scan() {
	//	processLine(enableTimestamp, saveLog, prefix, 0, scanner.Text(), job, container, startTime, JobStateEnum(reportCmd.State), &cmdOutput)
	//}

	wg.Wait()
	// Waiting for the command to finish
	waitErr := cmd.Wait()

	if waitErr != nil {
		serviceLogger.Error("failed to wait for command", "cmd", cmd.String(), "cmdOutput", cmdOutput.String(), "error", waitErr)
		if prefix == structs.PrefixPush {
			saveLog = true
		}
		if reportCmd != nil {
			reportCmd.Fail(job, fmt.Sprintf("%s: %s", waitErr.Error(), cmdOutput.String()))
			processLine(enableTimestamp, saveLog, prefix, -1, "", buildJob, container, startTime, reportCmd, &cmdOutput)
			return waitErr
		}
	}

	moDebug, err := strconv.ParseBool(config.Get("MO_DEBUG"))
	assert.Assert(err == nil)
	if moDebug && buildJob != nil {
		elapsedTime := time.Since(startTime)
		buildJob.DurationMs = int(elapsedTime.Milliseconds()) + buildJob.DurationMs
		serviceLogger.Info("build info",
			"prefix", prefix,
			"BuildId", buildJob.BuildId,
			"DurationMs", buildJob.DurationMs,
			"cmd", cmd.String(),
			"cmdOutput", cmdOutput.String(),
		)
	}

	if reportCmd != nil {
		reportCmd.Success(job, reportCmd.Message)
		processLine(enableTimestamp, saveLog, prefix, -1, "", buildJob, container, startTime, reportCmd, &cmdOutput)
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
			serviceLogger.Info("Notice: job is nil")
			return
		}
		nextLine := applyErrorSuggestions(cmdOutput.String())
		err := db.SaveBuildResult(structs.JobStateEnum(reportCmd.State), prefix, nextLine, startTime, job, container)
		if err != nil {
			serviceLogger.Error("failed to save build result", "error", err)
		}

		ch, exists := xterm.LogChannels[structs.BuildJobInfoEntryKey(job.BuildId, prefix, job.Namespace.Name, job.Service.ControllerName, container.Name)]
		if exists {
			ch <- newLine
		}

		// send notification
		// send start-signal when first line is received
		if lineNumber == 0 || lineNumber == -1 {
			data := db.GetBuildJobInfosFromDb(job.BuildId)
			structs.EventServerSendData(structs.CreateDatagramBuildLogs(data), "", "", "", 0)
		}
	}
}

func applyErrorSuggestions(line string) string {
	if strings.Contains(line, "Get \"https://mocr.local.mogenius.io/v2/\": tls: failed to verify certificate: x509: certificate is valid for") {
		line = line + infoStr() + shell.Colorize("✅  Please run 'mocli cluster local-dev-setup' to prepare everything for local registries.\r\n✅  Make sure you have installed the Local Container Registry in the cluster settings.\r\n✅  Please restart your k8s-manager pod to retrieve the new certificate.", shell.Green)
	}
	return line
}

func infoStr() string {
	return "\r\n    🤓\r\n  🤓  🤓\r\n🤓  🤓  🤓\r\n\r\n"
}

func cleanPasswords(job *structs.BuildJob, line string) string {
	if job == nil {
		return line
	}
	if job.Project.GitAccessToken != nil && *job.Project.GitAccessToken != "" {
		line = strings.ReplaceAll(line, *job.Project.GitAccessToken, "****")
	}
	if job.Project.ContainerRegistryPat != nil && *job.Project.ContainerRegistryPat != "" {
		line = strings.ReplaceAll(line, *job.Project.ContainerRegistryPat, "****")
	}
	if job.Project.ContainerRegistryUser != nil && *job.Project.ContainerRegistryUser != "" {
		line = strings.ReplaceAll(line, *job.Project.ContainerRegistryUser, "****")
	}
	for _, container := range job.Service.Containers {
		for _, v := range container.EnvVars {
			if v.Type == dtos.EnvVarKeyVault && v.Data.VaultType == dtos.EnvVarVaultTypeMogeniusVault {
				if v.Value != "" {
					line = strings.ReplaceAll(line, v.Value, "****")
				}
			}
		}
	}
	return line
}

func updateController(job *structs.Job, reportCmd *structs.Command, buildJob *structs.BuildJob, containerName string, imageName string, createAndStart bool) error {
	startTime := time.Now()
	if reportCmd != nil {
		reportCmd.Start(job, reportCmd.Message)
	}

	var err error
	updateService := false

	switch buildJob.Service.Controller {
	case dtos.CRON_JOB:
		var cronJob *v1job.CronJob
		cronJob, err = kubernetes.GetCronJob(buildJob.Namespace.Name, buildJob.Service.ControllerName)
		if err != nil && apierrors.IsNotFound(err) {
			err = nil
			if createAndStart {
				updateService = true
			}
		} else {
			if err == nil {
				if cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend {
					// cron job not running, update image
					err = kubernetes.UpdateCronjobImage(buildJob.Namespace.Name, buildJob.Service.ControllerName, containerName, imageName)
				} else {
					// cron job running, update Service
					updateService = true
				}
			}
		}
	case dtos.DEPLOYMENT:
		var deployment *v1.Deployment
		deployment, err = kubernetes.GetDeployment(buildJob.Namespace.Name, buildJob.Service.ControllerName)
		// if deployment not found. Create it if createAndStart is true
		if err != nil && apierrors.IsNotFound(err) {
			err = nil
			if createAndStart {
				updateService = true
			}
		} else {
			if err == nil {
				if deployment.Spec.Paused || (deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == 0) {
					// deployment not running, update image
					err = kubernetes.UpdateDeploymentImage(buildJob.Namespace.Name, buildJob.Service.ControllerName, containerName, imageName)
				} else {
					// deployment running, update Service
					updateService = true
				}
			}
		}
	default:
		err = fmt.Errorf("Unsupported controller type: %s", buildJob.Service.Controller)
	}

	// update service
	if updateService && err == nil {
		r := ServiceUpdateRequest{
			Project:   buildJob.Project,
			Namespace: buildJob.Namespace,
			Service:   buildJob.Service,
		}
		UpdateService(r)
	}

	elapsedTime := time.Since(startTime)
	buildJob.DurationMs = int(elapsedTime.Milliseconds()) + buildJob.DurationMs + 1
	if err != nil {
		reportCmd.Fail(job, err.Error())
	} else {
		reportCmd.Success(job, reportCmd.Message)
	}

	return err
}

func updateState(buildJob structs.BuildJob, newState structs.JobStateEnum) {
	db.UpdateStateInDb(buildJob, newState)
}

// func positionInQueue(buildId uint64) int {
// 	return db.PositionInQueueFromDb(buildId)
// }

func saveJob(buildJob structs.BuildJob) {
	db.SaveJobInDb(buildJob)
}
