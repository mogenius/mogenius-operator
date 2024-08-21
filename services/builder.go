package services

import (
	"bufio"
	"context"
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"mogenius-k8s-manager/db"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"mogenius-k8s-manager/xterm"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	punqUtils "github.com/mogenius/punq/utils"
)

var DISABLEQUEUE bool = true

var currentBuildContext *context.Context
var currentBuildChannel chan structs.JobStateEnum
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
	ServiceLogger.Infof("Queued %d/%d jobs in build-queue.", len(jobsToBuild), utils.CONFIG.Builder.MaxConcurrentBuilds)
	for _, buildJob := range jobsToBuild {
		for _, container := range buildJob.Service.Containers {
			// only build git-repositories
			if container.Type != dtos.CONTAINER_GIT_REPOSITORY {
				continue
			}

			currentBuildChannel = make(chan structs.JobStateEnum, 1)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(utils.CONFIG.Builder.BuildTimeout))
			currentBuildContext = &ctx
			currentBuildJob = &buildJob
			defer cancel()

			currentNumberOfRunningJobs++
			job := structs.CreateJob(fmt.Sprintf("Building '%s'", buildJob.Service.ControllerName), buildJob.Project.Id, buildJob.Namespace.Name, buildJob.Service.ControllerName)
			job.BuildId = buildJob.BuildId
			job.ContainerName = container.Name
			container.GitCommitAuthor = punqUtils.Pointer(utils.Escape(*container.GitCommitAuthor))
			container.GitCommitMessage = punqUtils.Pointer(utils.Escape(*container.GitCommitMessage))

			go build(job, &buildJob, &container, currentBuildChannel, &ctx)

			select {
			case <-ctx.Done():
				ServiceLogger.Errorf("BUILD TIMEOUT (after %ds)! (%s)", utils.CONFIG.Builder.BuildTimeout, ctx.Err())
				job.State = structs.JobStateTimeout
				buildJob.State = structs.JobStateTimeout
				saveJob(buildJob)
			case result := <-currentBuildChannel:
				switch result {
				case structs.JobStateTimeout:
					ServiceLogger.Warningf("Build '%d' CANCELED successfuly. (Took: %dms)", buildJob.BuildId, buildJob.DurationMs)
				case structs.JobStateFailed:
					ServiceLogger.Errorf("Build '%d' FAILDED. (Took: %dms)", buildJob.BuildId, buildJob.DurationMs)
				case structs.JobStateSucceeded:
					ServiceLogger.Infof("Build '%d' finished successfuly. (Took: %dms)", buildJob.BuildId, buildJob.DurationMs)
				default:
					ServiceLogger.Errorf("Unhandled channelMsg for '%d': %s", buildJob.BuildId, result)
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

	pwd, _ := os.Getwd()
	workingDir := fmt.Sprintf("%s/temp/%s", pwd, punqUtils.NanoId())

	defer func() {
		// reset everything if done
		if !utils.CONFIG.Misc.Debug {
			executeCmd(job, nil, db.PREFIX_CLEANUP, buildJob, container, false, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("rm -rf %s", workingDir))
		}
		done <- structs.JobStateSucceeded
	}()

	updateState(*buildJob, structs.JobStateStarted)

	_, tagName, latestTagName := db.ImageNamesFromBuildJob(*buildJob)

	// CLEANUP
	if !utils.CONFIG.Misc.Debug {
		executeCmd(job, nil, db.PREFIX_CLEANUP, buildJob, container, false, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("rm -rf %s", workingDir))
	}

	// CLONE
	cloneCmd := structs.CreateCommand(string(structs.PrefixGitClone), "Clone repository", job)
	err := executeCmd(job, cloneCmd, structs.PrefixGitClone, buildJob, container, true, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("git clone --progress -b %s --single-branch %s %s", *container.GitBranch, *container.GitRepository, workingDir))
	if err != nil {
		ServiceLogger.Errorf("Error%s: %s", structs.PrefixGitClone, err.Error())
		done <- structs.JobStateFailed
		return
	}

	// TAG
	getGitTagCmd := exec.Command("/bin/sh", "-c", "git tag --contains HEAD")
	getGitTagCmd.Dir = workingDir
	gitTagData, _ := getGitTagCmd.CombinedOutput()
	// in this case we dont care if the tag retrieval fails

	// LS
	lsCmd := structs.CreateCommand(string(structs.PrefixLs), "List contents", job)
	err = executeCmd(job, lsCmd, structs.PrefixLs, buildJob, container, true, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("cd %s; echo 'ℹ️  Current directory contents:'; ls -lisa; echo '\nℹ️  Git Log: '; git log -1 --decorate; echo '\nℹ️  Following ARGs are available for Docker build:'; echo '%s'", workingDir, container.AvailableDockerBuildArgs(job.BuildId, string(gitTagData))))
	if err != nil {
		ServiceLogger.Errorf("Error%s: %s", structs.PrefixLs, err.Error())
		done <- structs.JobStateFailed
		return
	}

	// LOGIN
	if buildJob.Project.ContainerRegistryUser != nil && buildJob.Project.ContainerRegistryPat != nil && *buildJob.Project.ContainerRegistryUser != "" && *buildJob.Project.ContainerRegistryPat != "" {
		loginCmd := structs.CreateCommand(string(structs.PrefixLogin), "Authenticate with container registry", job)
		err = executeCmd(job, loginCmd, structs.PrefixLogin, buildJob, container, true, false, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("docker login %s -u %s -p %s", *buildJob.Project.ContainerRegistryUrl, *buildJob.Project.ContainerRegistryUser, *buildJob.Project.ContainerRegistryPat))
		if err != nil {
			ServiceLogger.Errorf("Error%s: %s", structs.PrefixLogin, err.Error())
			done <- structs.JobStateFailed
			return
		}
	}

	// BUILD
	buildCmd := structs.CreateCommand(string(structs.PrefixBuild), "Building container", job)
	err = executeCmd(job, buildCmd, structs.PrefixBuild, buildJob, container, true, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("cd %s; docker build --network host -f %s %s -t %s -t %s %s", workingDir, *container.DockerfileName, container.GetInjectDockerEnvVars(job.BuildId, string(gitTagData)), tagName, latestTagName, *container.DockerContext))
	if err != nil {
		ServiceLogger.Errorf("Error%s: %s", structs.PrefixBuild, err.Error())
		done <- structs.JobStateFailed
		return
	}

	// PUSH
	pushCmd := structs.CreateCommand(string(structs.PrefixPush), "Pushing container", job)
	err = executeCmd(job, pushCmd, structs.PrefixPush, buildJob, container, false, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("docker push %s", latestTagName))
	if err != nil {
		ServiceLogger.Errorf("Error%s: %s", structs.PrefixPush, err.Error())
		done <- structs.JobStateFailed
		return
	}
	err = executeCmd(job, pushCmd, structs.PrefixPush, buildJob, container, true, true, timeoutCtx, "/bin/sh", "-c", fmt.Sprintf("docker push %s", tagName))
	if err != nil {
		ServiceLogger.Errorf("Error%s: %s", structs.PrefixPush, err.Error())
		done <- structs.JobStateFailed
		return
	}

	// create
	if buildJob.CreateAndStart {
		r := ServiceUpdateRequest{
			Project:   buildJob.Project,
			Namespace: buildJob.Namespace,
			Service:   buildJob.Service,
		}
		UpdateService(r)
	} else {
		// UPDATE IMAGE
		setImageCmd := structs.CreateCommand("setImage", "Deploying image", job)
		err = updateContainerImage(job, setImageCmd, buildJob, container.Name, tagName)
		if err != nil {
			ServiceLogger.Errorf("Error-%s: %s", "updateDeploymentImage", err.Error())
			done <- structs.JobStateFailed
			return
		}
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
		ServiceLogger.Errorf("Error adding job for '%s/%s'. REASON: %s", buildJob.Namespace.Name, buildJob.Service.ControllerName, err.Error())
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
		ServiceLogger.Error(errStr)
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

// func ListByServiceId(serviceId string) []structs.BuildJob {
// 	result := []structs.BuildJob{}

// 	list := ListAll()
// 	for _, queueEntry := range list {
// 		if queueEntry.Service.Id == serviceId {
// 			result = append(result, queueEntry)
// 		}
// 	}
// 	return result
// }

// TODO Remove this code
//func ListByServiceByNamespaceAndControllerName(namespace, controllerName string) []structs.BuildJob {
//	result := []structs.BuildJob{}
//
//	list := ListAll()
//	for _, queueEntry := range list {
//		if queueEntry.Service.ControllerName == controllerName && queueEntry.Namespace.Name == namespace {
//			result = append(result, queueEntry)
//		}
//	}
//	return result
//}

// func ListByServiceIds(serviceIds []string) []structs.BuildJob {
// 	result := []structs.BuildJob{}

// 	list := ListAll()
// 	for _, queueEntry := range list {
// 		if punqUtils.Contains(serviceIds, queueEntry.Service.Id) {
// 			result = append(result, queueEntry)
// 		}
// 	}
// 	return result
// }

//func LastNJobsPerService(maxResults int, serviceId string) []structs.BuildJob {
//	result := []structs.BuildJob{}
//
//	list := ListByServiceId(serviceId)
//	for i := len(list) - 1; i >= 0; i-- {
//		if len(result) < maxResults {
//			result = append(result, list[i])
//		}
//	}
//	return result
//}

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
		ServiceLogger.Errorf("Failed to execute command (%s): %v", cmd.String(), execErr)
		ServiceLogger.Errorf("Error: %s", cmdOutput.String())
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
		ServiceLogger.Errorf("Failed wait for command (%s): %v", cmd.String(), waitErr)
		ServiceLogger.Errorf("Error: %s", cmdOutput.String())
		ServiceLogger.Info(reportCmd == nil)
		if prefix == structs.PrefixPush {
			saveLog = true
		}
		if reportCmd != nil {
			reportCmd.Fail(job, fmt.Sprintf("%s: %s", waitErr.Error(), cmdOutput.String()))
			processLine(enableTimestamp, saveLog, prefix, -1, "", buildJob, container, startTime, reportCmd, &cmdOutput)
			return waitErr
		}
	}
	if utils.CONFIG.Misc.Debug && buildJob != nil {
		elapsedTime := time.Since(startTime)
		buildJob.DurationMs = int(elapsedTime.Milliseconds()) + buildJob.DurationMs
		ServiceLogger.Infof("%s%d: %dms", prefix, buildJob.BuildId, buildJob.DurationMs)
		ServiceLogger.Infof("%s%d: %s", prefix, buildJob.BuildId, cmd.String())
		ServiceLogger.Infof("%s%d: %s", prefix, buildJob.BuildId, cmdOutput.String())
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
			ServiceLogger.Infof("Notice: job is nil")
			return
		}
		nextLine := applyErrorSuggestions(cmdOutput.String())
		db.SaveBuildResult(structs.JobStateEnum(reportCmd.State), prefix, nextLine, startTime, job, container)

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
		line = line + infoStr() + color.GreenString("✅  Please run 'mocli cluster local-dev-setup' to prepare everything for local registries.\r\n✅  Make sure you have installed the Local Container Registry in the cluster settings.\r\n✅  Please restart your k8s-manager pod to retrieve the new certificate.")
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
			if v.Type == dtos.EnvVarKeyVault {
				line = strings.ReplaceAll(line, v.Value, "****")
			}
		}
	}
	return line
}

func updateContainerImage(job *structs.Job, reportCmd *structs.Command, buildJob *structs.BuildJob, containerName string, imageName string) error {
	startTime := time.Now()
	if reportCmd != nil {
		reportCmd.Start(job, reportCmd.Message)
	}

	var err error
	switch buildJob.Service.Controller {
	case dtos.CRON_JOB:
		err = kubernetes.UpdateCronjobImage(buildJob.Namespace.Name, buildJob.Service.ControllerName, containerName, imageName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				var wg sync.WaitGroup
				kubernetes.UpdateCronJob(job, buildJob.Namespace, buildJob.Service, &wg)
				err = nil
				wg.Wait()
			}
		}
	default:
		err = kubernetes.UpdateDeploymentImage(buildJob.Namespace.Name, buildJob.Service.ControllerName, containerName, imageName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				var wg sync.WaitGroup
				kubernetes.UpdateDeployment(job, buildJob.Namespace, buildJob.Service, &wg)
				err = nil
				wg.Wait()
			}
		}
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
