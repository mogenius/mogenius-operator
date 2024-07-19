package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"mogenius-k8s-manager/dtos"
	iacmanager "mogenius-k8s-manager/iac-manager"
	"mogenius-k8s-manager/structs"

	punq "github.com/mogenius/punq/kubernetes"
	punqutils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"
	apipatchv1 "k8s.io/api/batch/v1"
	v1job "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	v1Core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	batchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"

	cron "github.com/robfig/cron/v3"
)

type JobInfoTileType string
type JobInfoStatusType string

const (
	JobInfoStatusTypeActive    JobInfoStatusType = "Active"
	JobInfoStatusTypeSucceeded JobInfoStatusType = "Succeeded"
	JobInfoStatusTypeFailed    JobInfoStatusType = "Failed"
	JobInfoStatusTypeSuspended JobInfoStatusType = "Suspended"
	JobInfoStatusTypeUnknown   JobInfoStatusType = "Unknown"
)

const (
	JobInfoTileTypeJob   JobInfoTileType = "Job"
	JobInfoTileTypeEmpty JobInfoTileType = "Empty"
)

type JobInfo struct {
	Schedule     time.Time         `json:"schedule"`
	Status       JobInfoStatusType `json:"status"`
	TileType     JobInfoTileType   `json:"tileType"`
	JobName      string            `json:"jobName,omitempty"`
	JobId        string            `json:"jobId,omitempty"`
	PodName      string            `json:"podName,omitempty"`
	DurationInMs int64             `json:"durationInMs,omitempty"`
	Message      *StatusMessage    `json:"message,omitempty"`
}

type ListJobInfoResponse struct {
	ControllerName string    `json:"controllerName"`
	NamespaceName  string    `json:"namespaceName"`
	ProjectId      string    `json:"projectId"`
	JobsInfo       []JobInfo `json:"jobsInfo"`
}

type StatusMessage struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

func TriggerJobFromCronjob(job *structs.Job, namespace string, controller string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("trigger", fmt.Sprintf("Trigger Job from CronJob '%s'.", namespace), job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Trigger Job from CronJob")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}

		// get cronjob
		cronjobs := provider.ClientSet.BatchV1().CronJobs(namespace)
		cronjob, err := cronjobs.Get(context.TODO(), controller, metav1.GetOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("Failed get CronJob for trigger ERROR: %s", err.Error()))
			return
		}

		// convert cronjob to job
		jobs := provider.ClientSet.BatchV1().Jobs(namespace)
		jobSpec := &v1job.Job{
			ObjectMeta: cronjob.Spec.JobTemplate.ObjectMeta,
			Spec:       cronjob.Spec.JobTemplate.Spec,
		}
		jobSpec.Name = fmt.Sprintf("%s-%s", controller, punqutils.NanoIdSmallLowerCase())

		// disable TTL to keep history limit
		// both, jobs and pods are keept then
		// otherwise we need to implement a custom JobReconciler which
		// deletes the jobs and keeps the pods with client.PropagationPolicy(metav1.DeletePropagationOrphan)
		jobSpec.Spec.TTLSecondsAfterFinished = nil

		// create job
		_, err = jobs.Create(context.TODO(), jobSpec, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("Failed create Job via CronJob trigger ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Triggered Job from CronJob")
		}
	}(wg)
}

// func CreateCronJob(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
// 	cmd := structs.CreateCommand("create", "Creating CronJob", job)
// 	wg.Add(1)
// 	go func(wg *sync.WaitGroup) {
// 		defer wg.Done()
// 		cmd.Start(job, "Creating CronJob")

// 		provider, err := punq.NewKubeProvider(nil)
// 		if err != nil {
// 			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
// 			return
// 		}

// 		if service.CronJobSettings == nil {
// 			cmd.Fail(job, "CronJobSettings is nil.")
// 			return
// 		}

// 		cronJobClient := provider.ClientSet.BatchV1().CronJobs(namespace.Name)
// 		newController, err := CreateControllerConfiguration(job.ProjectId, namespace, service, true, cronJobClient, createCronJobHandler)
// 		if err != nil {
// 			K8sLogger.Errorf("error: %s", err.Error())
// 		}

// 		newCronJob := newController.(*v1job.CronJob)
// 		newCronJob.Labels = MoUpdateLabels(&newCronJob.Labels, nil, nil, &service)

// 		_, err = cronJobClient.Create(context.TODO(), newCronJob, MoCreateOptions())
// 		if err != nil {
// 			cmd.Fail(job, fmt.Sprintf("CreateCronJob ERROR: %s", err.Error()))
// 		} else {
// 			cmd.Success(job, "Created CronJob")
// 		}

// 	}(wg)
// }

func DeleteCronJob(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", fmt.Sprintf("Deleting CronJob '%s'.", service.ControllerName), job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting CronJob")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		cronJobClient := provider.ClientSet.BatchV1().CronJobs(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: punqutils.Pointer[int64](5),
		}

		err = cronJobClient.Delete(context.TODO(), service.ControllerName, deleteOptions)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("DeleteCronJob ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Deleted CronJob")
		}

	}(wg)
}

func UpdateCronJob(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("update", "Updating CronJob", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Updating CronJob")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		cronJobClient := provider.ClientSet.BatchV1().CronJobs(namespace.Name)
		newController, err := CreateControllerConfiguration(job.ProjectId, namespace, service, false, cronJobClient, createCronJobHandler)
		if err != nil {
			K8sLogger.Errorf("error: %s", err.Error())
		}

		newCronJob := newController.(*v1job.CronJob)

		_, err = cronJobClient.Update(context.TODO(), newCronJob, MoUpdateOptions())
		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err = cronJobClient.Create(context.TODO(), newCronJob, MoCreateOptions())
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("CreateCronJob ERROR: %s", err.Error()))
				} else {
					cmd.Success(job, "Created CronJob")
				}
			} else {
				cmd.Fail(job, fmt.Sprintf("Updating CronJob ERROR: %s", err.Error()))
			}
		} else {
			cmd.Success(job, "Updating CronJob")
		}

	}(wg)
}

func StartCronJob(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("start", "Start CronJob", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Starting CronJob")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}

		cronJobClient := provider.ClientSet.BatchV1().CronJobs(namespace.Name)
		newController, err := CreateControllerConfiguration(job.ProjectId, namespace, service, false, cronJobClient, createCronJobHandler)
		if err != nil {
			K8sLogger.Errorf("error: %s", err.Error())
		}

		cronJob := newController.(*v1job.CronJob)

		_, err = cronJobClient.Update(context.TODO(), cronJob, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("StartingCronJob ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Started CronJob")
		}
	}(wg)
}

func StopCronJob(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("stop", "Stopping CronJob", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Stopping CronJob")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		cronJobClient := provider.ClientSet.BatchV1().CronJobs(namespace.Name)
		newController, err := CreateControllerConfiguration(job.ProjectId, namespace, service, false, cronJobClient, createCronJobHandler)
		if err != nil {
			K8sLogger.Errorf("error: %s", err.Error())
		}
		cronJob := newController.(*v1job.CronJob)
		cronJob.Spec.Suspend = punqutils.Pointer(true)

		_, err = cronJobClient.Update(context.TODO(), cronJob, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("StopCronJob ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Stopped CronJob")
		}
	}(wg)
}

func RestartCronJob(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("restart", "Restart CronJob", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Restarting CronJob ")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		cronJobClient := provider.ClientSet.BatchV1().CronJobs(namespace.Name)
		newController, err := CreateControllerConfiguration(job.ProjectId, namespace, service, false, cronJobClient, createCronJobHandler)
		if err != nil {
			K8sLogger.Errorf("error: %s", err.Error())
		}
		cronJob := newController.(*v1job.CronJob)

		// KUBERNETES ISSUES A "rollout restart deployment" WHENETHER THE METADATA IS CHANGED.
		if cronJob.ObjectMeta.Annotations == nil {
			cronJob.ObjectMeta.Annotations = map[string]string{}
			cronJob.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
		} else {
			cronJob.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
		}

		_, err = cronJobClient.Update(context.TODO(), cronJob, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("RestartCronJob ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Restart CronJob")
		}
	}(wg)
}

func createCronJobHandler(namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, freshlyCreated bool, client interface{}) (*metav1.ObjectMeta, HasSpec, interface{}, error) {
	var previousSpec *v1job.CronJobSpec
	previousCronjob, err := client.(batchv1.CronJobInterface).Get(context.TODO(), service.ControllerName, metav1.GetOptions{})
	if err != nil {
		previousCronjob = nil
	} else {
		previousSpec = &(*previousCronjob).Spec
	}

	newCronJob := punqutils.InitCronJob()

	objectMeta := &newCronJob.ObjectMeta
	spec := &newCronJob.Spec

	// LABELS
	if objectMeta.Labels == nil {
		objectMeta.Labels = map[string]string{}
	}
	objectMeta.Labels["app"] = service.ControllerName
	objectMeta.Labels["ns"] = namespace.Name

	if spec.JobTemplate.Spec.Template.ObjectMeta.Labels == nil {
		spec.JobTemplate.Spec.Template.ObjectMeta.Labels = map[string]string{}
	}
	spec.JobTemplate.Spec.Template.ObjectMeta.Labels["app"] = service.ControllerName
	spec.JobTemplate.Spec.Template.ObjectMeta.Labels["ns"] = namespace.Name

	// INIT CONTAINER
	if len(spec.JobTemplate.Spec.Template.Spec.Containers) == 0 {
		spec.JobTemplate.Spec.Template.Spec.Containers = []core.Container{}
		spec.JobTemplate.Spec.Template.Spec.Containers = append(spec.JobTemplate.Spec.Template.Spec.Containers, core.Container{})
	}

	// SUSPEND -> PAUSE
	if freshlyCreated && service.HasContainerWithGitRepo() {
		spec.Suspend = punqutils.Pointer(true)
	} else {
		spec.Suspend = punqutils.Pointer(!(service.ReplicaCount > 0))
	}

	// CRON_JOB SETTINGS
	spec.Schedule = service.CronJobSettings.Schedule

	if service.CronJobSettings.ActiveDeadlineSeconds > 0 {
		spec.JobTemplate.Spec.ActiveDeadlineSeconds = punqutils.Pointer(service.CronJobSettings.ActiveDeadlineSeconds)
	}
	if service.CronJobSettings.BackoffLimit > 0 {
		spec.JobTemplate.Spec.BackoffLimit = punqutils.Pointer(service.CronJobSettings.BackoffLimit)
	}

	// HISTORY LIMITS
	if service.CronJobSettings.FailedJobsHistoryLimit > 0 {
		spec.FailedJobsHistoryLimit = punqutils.Pointer(service.CronJobSettings.FailedJobsHistoryLimit)
	}
	if service.CronJobSettings.SuccessfulJobsHistoryLimit > 0 {
		spec.SuccessfulJobsHistoryLimit = punqutils.Pointer(service.CronJobSettings.SuccessfulJobsHistoryLimit)
	}

	// disable TTL to keep history limit
	// both, jobs and pods are keept then
	// otherwise we need to implement a custom JobReconciler which
	// deletes the jobs and keeps the pods with client.PropagationPolicy(metav1.DeletePropagationOrphan)
	spec.JobTemplate.Spec.TTLSecondsAfterFinished = nil

	return objectMeta, &SpecCronJob{spec, previousSpec}, &newCronJob, nil
}

func UpdateCronjobImage(namespaceName string, controllerName string, containerName string, imageName string) error {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return err
	}
	client := provider.ClientSet.BatchV1().CronJobs(namespaceName)
	crontjobToUpdate, err := client.Get(context.TODO(), controllerName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// SET NEW IMAGE
	for index, container := range crontjobToUpdate.Spec.JobTemplate.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			crontjobToUpdate.Spec.JobTemplate.Spec.Template.Spec.Containers[index].Image = imageName
		}
	}
	crontjobToUpdate.Spec.Suspend = punqutils.Pointer(false)

	_, err = client.Update(context.TODO(), crontjobToUpdate, metav1.UpdateOptions{})
	return err
}

// func SetCronJobImage(job *structs.Job, namespaceName string, controllerName string, containerName string, imageName string, wg *sync.WaitGroup) {
// 	cmd := structs.CreateCommand("setImage", "Set CronJob Image", job)
// 	wg.Add(1)
// 	go func(wg *sync.WaitGroup) {
// 		defer wg.Done()
// 		cmd.Start(job, "Set Image in CronJob")

// 		provider, err := punq.NewKubeProvider(nil)
// 		if err != nil {
// 			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
// 			return
// 		}
// 		cronjobClient := provider.ClientSet.BatchV1().CronJobs(namespaceName)
// 		cronjobToUpdate, err := cronjobClient.Get(context.TODO(), controllerName, metav1.GetOptions{})
// 		if err != nil {
// 			cmd.Fail(job, fmt.Sprintf("SetCronJobImage ERROR: %s", err.Error()))
// 			return
// 		}

// 		// SET NEW IMAGE
// 		for index, container := range cronjobToUpdate.Spec.JobTemplate.Spec.Template.Spec.Containers {
// 			if container.Name == containerName {
// 				cronjobToUpdate.Spec.JobTemplate.Spec.Template.Spec.Containers[index].Image = imageName
// 			}
// 		}
// 		cronjobToUpdate.Spec.Suspend = punqutils.Pointer(false)

// 		_, err = cronjobClient.Update(context.TODO(), cronjobToUpdate, metav1.UpdateOptions{})
// 		if err != nil {
// 			cmd.Fail(job, fmt.Sprintf("SetCronJobImage ERROR: %s", err.Error()))
// 		} else {
// 			cmd.Success(job, "Set new image in CronJob")
// 		}
// 	}(wg)
// }

// func AllCronjobs(namespaceName string) K8sWorkloadResult {
// 	result := []v1job.CronJob{}

// 	provider, err := punq.NewKubeProvider(nil)
// 	if err != nil {
// 		return WorkloadResult(nil, err)
// 	}
// 	cronJobList, err := provider.ClientSet.BatchV1().CronJobs(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
// 	if err != nil {
// 		K8sLogger.Errorf("AllCronjobs ERROR: %s", err.Error())
// 		return WorkloadResult(nil, err)
// 	}

// 	for _, cronJob := range cronJobList.Items {
// 		if !punqutils.Contains(punqutils.CONFIG.Misc.IgnoreNamespaces, cronJob.ObjectMeta.Namespace) {
// 			result = append(result, cronJob)
// 		}
// 	}
// 	return WorkloadResult(result, nil)
// }

// func UpdateK8sCronJob(data v1job.CronJob) K8sWorkloadResult {
// 	provider, err := punq.NewKubeProvider(nil)
// 	if provider == nil || err != nil {
// 		return WorkloadResult(nil, err)
// 	}
// 	cronJobClient := provider.ClientSet.BatchV1().CronJobs(data.Namespace)
// 	_, err = cronJobClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
// 	if err != nil {
// 		return WorkloadResult(nil, err)
// 	}
// 	return WorkloadResult(nil, nil)
// }

// func DeleteK8sCronJob(data v1job.CronJob) K8sWorkloadResult {
// 	provider, err := punq.NewKubeProvider(nil)
// 	if provider == nil || err != nil {
// 		return WorkloadResult(nil, err)
// 	}
// 	jobClient := provider.ClientSet.BatchV1().CronJobs(data.Namespace)
// 	err = jobClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
// 	if err != nil {
// 		return WorkloadResult(nil, err)
// 	}
// 	return WorkloadResult(nil, nil)
// }

// func DescribeK8sCronJob(namespace string, name string) K8sWorkloadResult {
// 	cmd := exec.Command("kubectl", "describe", "cronjob", name, "-n", namespace)

// 	output, err := cmd.CombinedOutput()
// 	if err != nil {
// 		K8sLogger.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
// 		K8sLogger.Errorf("Error: %s", string(output))
// 		return WorkloadResult(nil, string(output))
// 	}
// 	return WorkloadResult(string(output), nil)
// }

// func NewK8sCronJob() K8sNewWorkload {
// 	return NewWorkload(
// 		punq.RES_CRON_JOB,
// 		punqutils.InitCronJobYaml(),
// 		"A CronJob creates Jobs on a repeating schedule, like the cron utility in Unix-like systems. In this example, a CronJob named 'my-cronjob' is created. It runs a Job every minute. Each Job creates a Pod with a single container from the 'my-cronjob-image' image.")
// }

func getNextSchedule(cronExpr string, lastScheduleTime time.Time) (time.Time, error) {
	sched, err := cron.ParseStandard(cronExpr)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(lastScheduleTime), nil
}

func getJobStatus(conditions []apipatchv1.JobCondition) JobInfoStatusType {
	for _, condition := range conditions {
		switch condition.Type {
		case apipatchv1.JobSuspended:
			return JobInfoStatusTypeSuspended
		case apipatchv1.JobComplete:
			return JobInfoStatusTypeSucceeded
		case apipatchv1.JobFailed:
			return JobInfoStatusTypeFailed
		case apipatchv1.JobFailureTarget:
			return JobInfoStatusTypeFailed
		case apipatchv1.JobSuccessCriteriaMet:
			return JobInfoStatusTypeSucceeded
		}
	}
	return JobInfoStatusTypeUnknown
}

func hasLabel(labels map[string]string, labelKey string, labelValue string) bool {
	_, exists := labels[labelKey]
	return exists && labels[labelKey] == labelValue
}

func ListCronjobJobs(controllerName, namespaceName, projectId string) ListJobInfoResponse {
	list := ListJobInfoResponse{
		ControllerName: controllerName,
		NamespaceName:  namespaceName,
		ProjectId:      projectId,
		JobsInfo:       []JobInfo{},
	}

	var jobInfos []JobInfo

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		log.Warnf("Error creating provider for ListJobs: %s", err.Error())
		return list
	}

	// Get the CronJob
	cronJob, err := provider.ClientSet.BatchV1().CronJobs(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})

	if err != nil {
		log.Warnf("Error getting cronjob %s: %s", controllerName, err.Error())
		return list
	}

	jobLabelSelectors := []string{
		fmt.Sprintf("mo-app=%s", controllerName),
		fmt.Sprintf("mo-ns=%s", namespaceName),
		fmt.Sprintf("mo-project-id=%s", projectId),
	}

	// Get the list of Jobs for each CronJob using multiple label selectors
	jobs, err := provider.ClientSet.BatchV1().Jobs(namespaceName).List(context.TODO(), metav1.ListOptions{
		LabelSelector: strings.Join(jobLabelSelectors, ","),
	})
	if err != nil {
		log.Warnf("Error getting jobs for cronjob %s: %s", cronJob.Name, err.Error())
		return list
	}

	podLabelSelectors := []string{}
	for _, job := range jobs.Items {
		podLabelSelectors = append(podLabelSelectors, job.Name)
	}

	// Get the Pods associated with the Job
	pods, err := provider.ClientSet.CoreV1().Pods(namespaceName).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name in (%s)", strings.Join(podLabelSelectors, ",")),
	})
	if err != nil {
		log.Warnf("Error getting pods for cronjob %s: %s", cronJob.Name, err.Error())
		return list
	}

	for _, job := range jobs.Items {
		jobInfo := JobInfo{
			JobName:  job.Name,
			JobId:    string(job.UID),
			PodName:  "",
			TileType: JobInfoTileTypeJob,
		}

		if job.Status.StartTime != nil {
			jobInfo.Schedule = job.Status.StartTime.Time
			if job.Status.CompletionTime != nil {
				duration := job.Status.CompletionTime.Sub(job.Status.StartTime.Time).Abs().Milliseconds()
				jobInfo.DurationInMs = duration
			}
		}

		if len(job.Status.Conditions) > 0 {
			jobInfo.Status = getJobStatus(job.Status.Conditions)
			condition := job.Status.Conditions[0]

			if condition.Message != "" && condition.Reason != "" {
				jobInfo.Message = &StatusMessage{
					Reason:  condition.Reason,
					Message: condition.Message,
				}
			}
		} else if job.Status.CompletionTime == nil {
			jobInfo.Status = JobInfoStatusTypeActive
		} else {
			jobInfo.Status = JobInfoStatusTypeUnknown
		}

		for _, pod := range pods.Items {
			labelKey := "job-name"
			labelValue := job.Name

			if hasLabel(pod.Labels, labelKey, labelValue) {
				jobInfo.PodName = pod.Name
				jobInfos = append(jobInfos, jobInfo)
			}
		}

	}

	sort.Slice(jobInfos, func(i, j int) bool {
		return jobInfos[i].Schedule.After(jobInfos[j].Schedule)
	})

	// Add an empty item for the next schedule
	if cronJob.Status.LastScheduleTime != nil {
		nextScheduleTime, err := getNextSchedule(cronJob.Spec.Schedule, cronJob.Status.LastScheduleTime.Time)
		if err != nil {
			log.Warnf("Error getting next schedule for cronjob %s: %s", cronJob.Name, err.Error())
			list.JobsInfo = jobInfos
			return list
		}
		// add an empty item to the beginning of the list
		jobInfos = append([]JobInfo{{
			Schedule: nextScheduleTime,
			TileType: JobInfoTileTypeEmpty,
			Status:   JobInfoStatusTypeUnknown,
		}}, jobInfos...)
	}

	list.JobsInfo = jobInfos

	return list
}

func WatchCronJobs() {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		K8sLogger.Fatalf("Error creating provider for watcher. Cannot continue because it is vital: %s", err.Error())
		return
	}

	// Retry watching resources with exponential backoff in case of failures
	err = retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}, apierrors.IsServiceUnavailable, func() error {
		return watchCronJobs(provider, "cronjobs")
	})
	if err != nil {
		K8sLogger.Fatalf("Error watching cronjobs: %s", err.Error())
	}

	// Wait forever
	select {}
}

func watchCronJobs(provider *punq.KubeProvider, kindName string) error {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			castedObj := obj.(*v1job.CronJob)
			castedObj.Kind = "CronJob"
			castedObj.APIVersion = "batch/v1"
			iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			castedObj := newObj.(*v1job.CronJob)
			castedObj.Kind = "CronJob"
			castedObj.APIVersion = "batch/v1"
			iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		DeleteFunc: func(obj interface{}) {
			castedObj := obj.(*v1job.CronJob)
			castedObj.Kind = "CronJob"
			castedObj.APIVersion = "batch/v1"
			iacmanager.DeleteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, obj)
		},
	}
	listWatch := cache.NewListWatchFromClient(
		provider.ClientSet.BatchV1().RESTClient(),
		kindName,
		v1Core.NamespaceAll,
		fields.Nothing(),
	)
	resourceInformer := cache.NewSharedInformer(listWatch, &v1job.CronJob{}, 0)
	_, err := resourceInformer.AddEventHandler(handler)
	if err != nil {
		return err
	}

	stopCh := make(chan struct{})
	go resourceInformer.Run(stopCh)

	// Wait for the informer to sync and start processing events
	if !cache.WaitForCacheSync(stopCh, resourceInformer.HasSynced) {
		return fmt.Errorf("failed to sync cache")
	}

	// This loop will keep the function alive as long as the stopCh is not closed
	for {
		select {
		case <-stopCh:
			// stopCh closed, return from the function
			return nil
		case <-time.After(30 * time.Second):
			// This is to avoid a tight loop in case stopCh is never closed.
			// You can adjust the time as per your needs.
		}
	}
}
