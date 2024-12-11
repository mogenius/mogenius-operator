package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/src/utils"
	"sort"
	"strings"
	"sync"
	"time"

	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/structs"

	apipatchv1 "k8s.io/api/batch/v1"
	v1job "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	v1core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	batchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"

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

		clientset := clientProvider.K8sClientSet()

		// get cronjob
		cronjobs := clientset.BatchV1().CronJobs(namespace)
		cronjob, err := cronjobs.Get(context.TODO(), controller, metav1.GetOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("Failed get CronJob for trigger ERROR: %s", err.Error()))
			return
		}

		// convert cronjob to job
		jobs := clientset.BatchV1().Jobs(namespace)
		jobSpec := &v1job.Job{
			ObjectMeta: cronjob.Spec.JobTemplate.ObjectMeta,
			Spec:       cronjob.Spec.JobTemplate.Spec,
		}
		jobSpec.Name = fmt.Sprintf("%s-%s", controller, utils.NanoIdSmallLowerCase())

		// set owner reference to cronjob
		ownerReference := metav1.OwnerReference{
			APIVersion:         "batch/v1",
			Kind:               "CronJob",
			Name:               cronjob.Name,
			UID:                cronjob.UID,
			Controller:         utils.Pointer(true),
			BlockOwnerDeletion: utils.Pointer(true),
		}
		jobSpec.SetOwnerReferences([]metav1.OwnerReference{ownerReference})

		// disable TTL to keep history limit
		// both, jobs and pods are keept then
		// otherwise we need to implement a custom JobReconciler which
		// deletes the jobs and keeps the pods with client.PropagationPolicy(metav1.DeletePropagationOrphan)
		jobSpec.Spec.TTLSecondsAfterFinished = nil
		// force pod restartPolicy: Never
		jobSpec.Spec.Template.Spec.RestartPolicy = v1core.RestartPolicyNever
		// set backofflimit=0 to avoid weird behavior for restartPolicy: Never
		jobSpec.Spec.BackoffLimit = utils.Pointer(int32(0))

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

// 		provider, err := NewKubeProvider()
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

		clientset := clientProvider.K8sClientSet()
		cronJobClient := clientset.BatchV1().CronJobs(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: utils.Pointer[int64](5),
		}

		err := cronJobClient.Delete(context.TODO(), service.ControllerName, deleteOptions)
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

		clientset := clientProvider.K8sClientSet()
		cronJobClient := clientset.BatchV1().CronJobs(namespace.Name)
		newController, err := CreateControllerConfiguration(job.ProjectId, namespace, service, false, cronJobClient, createCronJobHandler)
		if err != nil {
			k8sLogger.Error("Failed to create controller configuration", "error", err)
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

		clientset := clientProvider.K8sClientSet()
		cronJobClient := clientset.BatchV1().CronJobs(namespace.Name)

		newController, err := CreateControllerConfiguration(job.ProjectId, namespace, service, false, cronJobClient, createCronJobHandler)
		if err != nil {
			k8sLogger.Error("Failed to create controller configuration", "error", err)
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

		clientset := clientProvider.K8sClientSet()
		cronJobClient := clientset.BatchV1().CronJobs(namespace.Name)
		newController, err := CreateControllerConfiguration(job.ProjectId, namespace, service, false, cronJobClient, createCronJobHandler)
		if err != nil {
			k8sLogger.Error("Failed to create controller configuration", "error", err)
		}
		cronJob := newController.(*v1job.CronJob)
		cronJob.Spec.Suspend = utils.Pointer(true)

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

		clientset := clientProvider.K8sClientSet()
		cronJobClient := clientset.BatchV1().CronJobs(namespace.Name)

		newController, err := CreateControllerConfiguration(job.ProjectId, namespace, service, false, cronJobClient, createCronJobHandler)
		if err != nil {
			k8sLogger.Error("Failed to create controller configuration", "error", err)
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
	if err == nil {
		previousSpec = &(*previousCronjob).Spec
	}

	newCronJob := utils.InitCronJob()

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
		spec.Suspend = utils.Pointer(true)
	} else {
		spec.Suspend = utils.Pointer(!(service.ReplicaCount > 0))
	}

	// CRON_JOB SETTINGS
	spec.Schedule = service.CronJobSettings.Schedule

	if service.CronJobSettings.ActiveDeadlineSeconds > 0 {
		spec.JobTemplate.Spec.ActiveDeadlineSeconds = utils.Pointer(service.CronJobSettings.ActiveDeadlineSeconds)
	}

	// HISTORY LIMITS
	if service.CronJobSettings.FailedJobsHistoryLimit > 0 {
		spec.FailedJobsHistoryLimit = utils.Pointer(service.CronJobSettings.FailedJobsHistoryLimit)
	}
	if service.CronJobSettings.SuccessfulJobsHistoryLimit > 0 {
		spec.SuccessfulJobsHistoryLimit = utils.Pointer(service.CronJobSettings.SuccessfulJobsHistoryLimit)
	}

	// disable TTL to keep history limit
	// both, jobs and pods are keept then
	// otherwise we need to implement a custom JobReconciler which
	// deletes the jobs and keeps the pods with client.PropagationPolicy(metav1.DeletePropagationOrphan)
	spec.JobTemplate.Spec.TTLSecondsAfterFinished = nil
	// force pod restartPolicy: Never
	spec.JobTemplate.Spec.Template.Spec.RestartPolicy = v1core.RestartPolicyNever
	// set backofflimit=0 to avoid weird behavior for restartPolicy: Never
	spec.JobTemplate.Spec.BackoffLimit = utils.Pointer(int32(0))

	return objectMeta, &SpecCronJob{spec, previousSpec}, &newCronJob, nil
}

func UpdateCronjobImage(namespaceName string, controllerName string, containerName string, imageName string) error {
	clientset := clientProvider.K8sClientSet()
	client := clientset.BatchV1().CronJobs(namespaceName)
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
	// crontjobToUpdate.Spec.Suspend = utils.Pointer(false)

	_, err = client.Update(context.TODO(), crontjobToUpdate, metav1.UpdateOptions{})
	return err
}

func GetCronJob(namespaceName string, controllerName string) (*v1job.CronJob, error) {
	clientset := clientProvider.K8sClientSet()
	client := clientset.BatchV1().CronJobs(namespaceName)
	return client.Get(context.TODO(), controllerName, metav1.GetOptions{})
}

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

var listCronjobJobsDebounce = utils.NewDebounce("listCronjobJobsDebounce", 1000*time.Millisecond, 300*time.Millisecond)

func ListCronjobJobs(controllerName string, namespaceName string, projectId string) interface{} {
	key := fmt.Sprintf("%s-%s-%s", controllerName, namespaceName, projectId)
	result, _ := listCronjobJobsDebounce.CallFn(key, func() (interface{}, error) {
		return ListCronjobJobs2(controllerName, namespaceName, projectId), nil
	})
	return result
}

func ListCronjobJobs2(controllerName string, namespaceName string, projectId string) ListJobInfoResponse {
	list := ListJobInfoResponse{
		ControllerName: controllerName,
		NamespaceName:  namespaceName,
		ProjectId:      projectId,
		JobsInfo:       []JobInfo{},
	}

	var jobInfos []JobInfo

	clientset := clientProvider.K8sClientSet()

	// Get the CronJob
	cronJob, err := clientset.BatchV1().CronJobs(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})

	if err != nil {
		k8sLogger.Warn("Error getting cronjob", "controller", controllerName, "error", err)
		return list
	}

	jobLabelSelectors := []string{
		fmt.Sprintf("mo-app=%s", controllerName),
		fmt.Sprintf("mo-ns=%s", namespaceName),
		fmt.Sprintf("mo-project-id=%s", projectId),
	}

	// Get the list of Jobs for each CronJob using multiple label selectors
	jobs, err := clientset.BatchV1().Jobs(namespaceName).List(context.TODO(), metav1.ListOptions{
		LabelSelector: strings.Join(jobLabelSelectors, ","),
	})
	if err != nil {
		k8sLogger.Warn("Error getting jobs for cronjob %s: %s", cronJob.Name, err.Error())
		return list
	}

	podLabelSelectors := []string{}
	for _, job := range jobs.Items {
		podLabelSelectors = append(podLabelSelectors, job.Name)
	}

	// Get the Pods associated with the Job
	pods, err := clientset.CoreV1().Pods(namespaceName).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name in (%s)", strings.Join(podLabelSelectors, ",")),
	})
	if err != nil {
		k8sLogger.Warn("Error getting pods for cronjob %s: %s", cronJob.Name, err.Error())
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
	if cronJob.Spec.Suspend != nil && !*cronJob.Spec.Suspend {
		var lastTime time.Time
		if cronJob.Status.LastScheduleTime != nil {
			lastTime = cronJob.Status.LastScheduleTime.Time
		} else {
			lastTime = time.Now()
		}

		nextScheduleTime, err := getNextSchedule(cronJob.Spec.Schedule, lastTime)
		if err != nil {
			k8sLogger.Warn("Error getting next schedule for cronjob", "cronjob", cronJob.Name, "error", err)
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
