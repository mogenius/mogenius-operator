package kubernetes

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"mogenius-k8s-manager/dtos"
	iacmanager "mogenius-k8s-manager/iac-manager"
	"mogenius-k8s-manager/structs"

	punq "github.com/mogenius/punq/kubernetes"
	punqutils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"
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
)

func TriggerJobFromCronjob(job *structs.Job, namespace string, controller string, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Trigger Job from CronJob '%s'.", namespace), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Trigger Job from CronJob '%s'.", namespace))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}

		// get cronjob
		cronjobs := provider.ClientSet.BatchV1().CronJobs(namespace)
		cronjob, err := cronjobs.Get(context.TODO(), controller, metav1.GetOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("Failed get CronJob for trigger ERROR: %s", err.Error()))
			return
		}

		// convert cronjob to job
		jobs := provider.ClientSet.BatchV1().Jobs(namespace)
		jobSpec := &v1job.Job{
			ObjectMeta: cronjob.Spec.JobTemplate.ObjectMeta,
			Spec:       cronjob.Spec.JobTemplate.Spec,
		}
		jobSpec.Name = fmt.Sprintf("%s-%s", controller, punqutils.NanoIdSmallLowerCase())
		jobSpec.Spec.TTLSecondsAfterFinished = punqutils.Pointer(int32(60))

		// create job
		_, err = jobs.Create(context.TODO(), jobSpec, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("Failed create Job via CronJob trigger ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Triggered Job from CronJob '%s'.", namespace))
		}
	}(cmd, wg)
	return cmd
}

func CreateCronJob(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Creating CronJob '%s'.", namespace.Name), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating CronJob '%s'.", namespace.Name))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}

		if service.CronJobSettings == nil {
			cmd.Fail("CronJobSettings is nil.")
			return
		}

		cronJobClient := provider.ClientSet.BatchV1().CronJobs(namespace.Name)
		newController, err := CreateControllerConfiguration(namespace, service, true, cronJobClient, createCronJobHandler)
		if err != nil {
			log.Errorf("error: %s", err.Error())
		}

		newCronJob := newController.(*v1job.CronJob)
		newCronJob.Labels = MoUpdateLabels(&newCronJob.Labels, job.ProjectId, &namespace, &service)

		_, err = cronJobClient.Create(context.TODO(), newCronJob, MoCreateOptions())
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateCronJob ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Created CronJob '%s'.", namespace.Name))
		}

	}(cmd, wg)
	return cmd
}

func DeleteCronJob(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Deleting CronJob '%s'.", service.ControllerName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting CronJob '%s'.", service.ControllerName))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		cronJobClient := provider.ClientSet.BatchV1().CronJobs(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: punqutils.Pointer[int64](5),
		}

		err = cronJobClient.Delete(context.TODO(), service.ControllerName, deleteOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteCronJob ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted CronJob '%s'.", service.ControllerName))
		}

	}(cmd, wg)
	return cmd
}

func UpdateCronJob(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Updating CronJob '%s'.", namespace.Name), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Updating CronJob '%s'.", namespace.Name))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		cronJobClient := provider.ClientSet.BatchV1().CronJobs(namespace.Name)
		newController, err := CreateControllerConfiguration(namespace, service, false, cronJobClient, createCronJobHandler)
		if err != nil {
			log.Errorf("error: %s", err.Error())
		}

		newCronJob := newController.(*v1job.CronJob)

		updateOptions := metav1.UpdateOptions{
			FieldManager: DEPLOYMENTNAME,
		}

		_, err = cronJobClient.Update(context.TODO(), newCronJob, updateOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("UpdatingCronJob ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Updating CronJob '%s'.", namespace.Name))
		}

	}(cmd, wg)
	return cmd
}

func StartCronJob(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Starting CronJob", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Starting CronJob '%s'.", service.ControllerName))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}

		cronJobClient := provider.ClientSet.BatchV1().CronJobs(namespace.Name)
		newController, err := CreateControllerConfiguration(namespace, service, false, cronJobClient, createCronJobHandler)
		if err != nil {
			log.Errorf("error: %s", err.Error())
		}

		cronJob := newController.(*v1job.CronJob)

		_, err = cronJobClient.Update(context.TODO(), cronJob, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("StartingCronJob ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Started CronJob '%s'.", service.ControllerName))
		}
	}(cmd, wg)
	return cmd
}

func StopCronJob(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Stopping CronJob", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Stopping CronJob '%s'.", service.ControllerName))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		cronJobClient := provider.ClientSet.BatchV1().CronJobs(namespace.Name)
		newController, err := CreateControllerConfiguration(namespace, service, false, cronJobClient, createCronJobHandler)
		if err != nil {
			log.Errorf("error: %s", err.Error())
		}
		cronJob := newController.(*v1job.CronJob)
		cronJob.Spec.Suspend = punqutils.Pointer(true)

		_, err = cronJobClient.Update(context.TODO(), cronJob, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("StopCronJob ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Stopped CronJob '%s'.", service.ControllerName))
		}
	}(cmd, wg)
	return cmd
}

func RestartCronJob(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Restart CronJob", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Restarting CronJob '%s'.", service.ControllerName))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		cronJobClient := provider.ClientSet.BatchV1().CronJobs(namespace.Name)
		newController, err := CreateControllerConfiguration(namespace, service, false, cronJobClient, createCronJobHandler)
		if err != nil {
			log.Errorf("error: %s", err.Error())
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
			cmd.Fail(fmt.Sprintf("RestartCronJob ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Restart CronJob '%s'.", service.ControllerName))
		}
	}(cmd, wg)
	return cmd
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

func SetCronJobImage(job *structs.Job, namespaceName string, controllerName string, containerName string, imageName string, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Set CronJob Image '%s %s'", containerName, imageName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Set Image in CronJob '%s'.", controllerName))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		cronjobClient := provider.ClientSet.BatchV1().CronJobs(namespaceName)
		cronjobToUpdate, err := cronjobClient.Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("SetCronJobImage ERROR: %s", err.Error()))
			return
		}

		// SET NEW IMAGE
		for index, container := range cronjobToUpdate.Spec.JobTemplate.Spec.Template.Spec.Containers {
			if container.Name == containerName {
				cronjobToUpdate.Spec.JobTemplate.Spec.Template.Spec.Containers[index].Image = imageName
			}
		}
		cronjobToUpdate.Spec.Suspend = punqutils.Pointer(false)

		_, err = cronjobClient.Update(context.TODO(), cronjobToUpdate, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("SetCronJobImage ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Set new image in CronJob '%s'.", controllerName))
		}
	}(cmd, wg)
	return cmd
}

func AllCronjobs(namespaceName string) K8sWorkloadResult {
	result := []v1job.CronJob{}

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return WorkloadResult(nil, err)
	}
	cronJobList, err := provider.ClientSet.BatchV1().CronJobs(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		log.Errorf("AllCronjobs ERROR: %s", err.Error())
		return WorkloadResult(nil, err)
	}

	for _, cronJob := range cronJobList.Items {
		if !punqutils.Contains(punqutils.CONFIG.Misc.IgnoreNamespaces, cronJob.ObjectMeta.Namespace) {
			result = append(result, cronJob)
		}
	}
	return WorkloadResult(result, nil)
}

func UpdateK8sCronJob(data v1job.CronJob) K8sWorkloadResult {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		return WorkloadResult(nil, err)
	}
	cronJobClient := provider.ClientSet.BatchV1().CronJobs(data.Namespace)
	_, err = cronJobClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DeleteK8sCronJob(data v1job.CronJob) K8sWorkloadResult {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		return WorkloadResult(nil, err)
	}
	jobClient := provider.ClientSet.BatchV1().CronJobs(data.Namespace)
	err = jobClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DescribeK8sCronJob(namespace string, name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "cronjob", name, "-n", namespace)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		log.Errorf("Error: %s", string(output))
		return WorkloadResult(nil, string(output))
	}
	return WorkloadResult(string(output), nil)
}

func NewK8sCronJob() K8sNewWorkload {
	return NewWorkload(
		punq.RES_CRON_JOB,
		punqutils.InitCronJobYaml(),
		"A CronJob creates Jobs on a repeating schedule, like the cron utility in Unix-like systems. In this example, a CronJob named 'my-cronjob' is created. It runs a Job every minute. Each Job creates a Pod with a single container from the 'my-cronjob-image' image.")
}

func WatchCronJobs() {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		log.Fatalf("Error creating provider for watcher. Cannot continue because it is vital: %s", err.Error())
		return
	}

	// Retry watching resources with exponential backoff in case of failures
	retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}, apierrors.IsServiceUnavailable, func() error {
		return watchCronJobs(provider, "cronjobs")
	})

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
			iacmanager.DeleteResourceYaml(kindName, castedObj.Namespace, castedObj.Name)
		},
	}
	listWatch := cache.NewListWatchFromClient(
		provider.ClientSet.BatchV1().RESTClient(),
		kindName,
		v1Core.NamespaceAll,
		fields.Nothing(),
	)
	resourceInformer := cache.NewSharedInformer(listWatch, &v1job.CronJob{}, 0)
	resourceInformer.AddEventHandler(handler)

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
