package kubernetes

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"

	punq "github.com/mogenius/punq/kubernetes"
	punqutils "github.com/mogenius/punq/utils"
	v1job "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TriggerJobFromCronjob(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Trigger Job from CronJob '%s'.", namespace.Name), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Trigger Job from CronJob '%s'.", namespace.Name))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}

		// get cronjob
		cronjobs := provider.ClientSet.BatchV1().CronJobs(namespace.Name)
		cronjob, err := cronjobs.Get(context.TODO(), service.Name, metav1.GetOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("Failed get CronJob for trigger ERROR: %s", err.Error()))
			return
		}

		// convert cronjob to job
		jobs := provider.ClientSet.BatchV1().Jobs(namespace.Name)
		jobSpec := &v1job.Job{
			ObjectMeta: cronjob.Spec.JobTemplate.ObjectMeta,
			Spec:       cronjob.Spec.JobTemplate.Spec,
		}
		jobSpec.Name = fmt.Sprintf("%s-7r1663rd", service.Name)
		jobSpec.Spec.TTLSecondsAfterFinished = punqutils.Pointer(int32(60))

		// create job
		_, err = jobs.Create(context.TODO(), jobSpec, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("Failed create Job via CronJob trigger ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Triggered Job from CronJob '%s'.", namespace.Name))
		}
	}(cmd, wg)
	return cmd
}

func CreateCronJob(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	logger.Log.Infof("CreateCronJob K8sServiceDto: %s", service)
	
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
		cronJobClient := provider.ClientSet.BatchV1().CronJobs(namespace.Name)
		// newCronJob := generateCronJob(namespace, service, true, cronJobClient)
		newController, err := GenerateController(namespace, service, false, cronJobClient, generateCronJobHandler)
		if  err != nil {
			logger.Log.Errorf("error: %s", err.Error())
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
	cmd := structs.CreateCommand(fmt.Sprintf("Deleting CronJob '%s'.", service.Name), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting CronJob '%s'.", service.Name))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		cronJobClient := provider.ClientSet.BatchV1().CronJobs(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: punqutils.Pointer[int64](5),
		}

		err = cronJobClient.Delete(context.TODO(), service.Name, deleteOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteCronJob ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted CronJob '%s'.", service.Name))
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
		// newCronJob := generateCronJob(namespace, service, true, cronJobClient)
		newController, err := GenerateController(namespace, service, false, cronJobClient, generateCronJobHandler)
		if  err != nil {
			logger.Log.Errorf("error: %s", err.Error())
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
		cmd.Start(fmt.Sprintf("Starting CronJob '%s'.", service.Name))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}

		cronJobClient := provider.ClientSet.BatchV1().CronJobs(namespace.Name)
		// newCronJob := generateCronJob(namespace, service, true, cronJobClient)
		newController, err := GenerateController(namespace, service, false, cronJobClient, generateCronJobHandler)
		if  err != nil {
			logger.Log.Errorf("error: %s", err.Error())
		}
		
		cronJob := newController.(*v1job.CronJob)


		_, err = cronJobClient.Update(context.TODO(), cronJob, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("StartingCronJob ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Started CronJob '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}

func StopCronJob(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Stopping CronJob", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Stopping CronJob '%s'.", service.Name))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		cronJobClient := provider.ClientSet.BatchV1().CronJobs(namespace.Name)
		// cronJob := generateCronJob(namespace, service, true, cronJobClient)
		newController, err := GenerateController(namespace, service, false, cronJobClient, generateCronJobHandler)
		if  err != nil {
			logger.Log.Errorf("error: %s", err.Error())
		}
		cronJob := newController.(*v1job.CronJob)
		cronJob.Spec.Suspend = punqutils.Pointer(true)

		_, err = cronJobClient.Update(context.TODO(), cronJob, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("StopCronJob ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Stopped CronJob '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}

func RestartCronJob(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Restart CronJob", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Restarting CronJob '%s'.", service.Name))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		cronJobClient := provider.ClientSet.BatchV1().CronJobs(namespace.Name)
		// cronJob := generateCronJob(namespace, service, true, cronJobClient)
		newController, err := GenerateController(namespace, service, false, cronJobClient, generateCronJobHandler)
		if  err != nil {
			logger.Log.Errorf("error: %s", err.Error())
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
			cmd.Success(fmt.Sprintf("Restart CronJob '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}

func generateCronJobHandler(namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, freshlyCreated bool, client interface{}) (*metav1.ObjectMeta, HasSpec, interface{}, error) {
	newCronJob := punqutils.InitCronJob()
	
	objectMeta := &newCronJob.ObjectMeta
	spec := &newCronJob.Spec

	// SUSPEND -> PAUSE
	if freshlyCreated &&
		(service.K8sSettings.K8sCronJobSettingsDto.SourceType == dtos.GIT_REPOSITORY ||
			service.K8sSettings.K8sCronJobSettingsDto.SourceType == dtos.GIT_REPOSITORY_TEMPLATE) {
				spec.Suspend = punqutils.Pointer(true)
	} else {
		spec.Suspend = punqutils.Pointer(!service.SwitchedOn)
	}

	// CRON_JOB SETTINGS
	spec.Schedule = service.K8sSettings.K8sCronJobSettingsDto.Schedule

	if service.K8sSettings.K8sCronJobSettingsDto.ActiveDeadlineSeconds > 0 {
		spec.JobTemplate.Spec.ActiveDeadlineSeconds = punqutils.Pointer(service.K8sSettings.K8sCronJobSettingsDto.ActiveDeadlineSeconds)
	}
	if service.K8sSettings.K8sCronJobSettingsDto.BackoffLimit > 0 {
		spec.JobTemplate.Spec.BackoffLimit = punqutils.Pointer(service.K8sSettings.K8sCronJobSettingsDto.BackoffLimit)
	}

	return objectMeta, &SpecCronJob{*spec}, &newCronJob, nil
}

// func generateCronJob(namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, freshlyCreated bool, cronjobclient batchv1.CronJobInterface) v1job.CronJob {
// 	previousCronjob, err := cronjobclient.Get(context.TODO(), service.Name, metav1.GetOptions{})
// 	if err != nil {
// 		//logger.Log.Infof("No previous cronjob found for %s/%s.", namespace.Name, service.Name)
// 		previousCronjob = nil
// 	}

// 	newCronJob := punqutils.InitCronJob()
	
// 	objectMeta := &newCronJob.ObjectMeta
// 	objectMeta.Name = service.Name
// 	objectMeta.Namespace = namespace.Name
	
// 	spec := &newCronJob.Spec.JobTemplate.Spec
// 	if spec.Selector == nil {
// 		spec.Selector = &metav1.LabelSelector{}
// 	}
// 	if spec.Selector.MatchLabels == nil {
// 		spec.Selector.MatchLabels = map[string]string{}
// 	}
// 	spec.Selector.MatchLabels["app"] = service.Name
// 	spec.Selector.MatchLabels["ns"] = namespace.Name
// 	if spec.Template.ObjectMeta.Labels == nil {
// 		spec.Template.ObjectMeta.Labels = map[string]string{}
// 	}
// 	spec.Template.ObjectMeta.Labels["app"] = service.Name
// 	spec.Template.ObjectMeta.Labels["ns"] = namespace.Name

// 	// not supported for cron job
// 	// newCronJob.Spec.JobTemplate.Spec.Selector.MatchLabels["app"] = service.Name
// 	// newCronJob.Spec.JobTemplate.Spec.Selector.MatchLabels["ns"] = namespace.Name
	
// 	// spec.Template.ObjectMeta.Labels["app"] = service.Name
// 	// spec.Template.ObjectMeta.Labels["ns"] = namespace.Name

// 	// STRATEGY
// 	// not implemented

// 	// SUSPEND -> SWITCHED ON
// 	// newCronJob.Spec.Suspend = utils.Pointer(!service.SwitchedOn)

// 	// SUSPEND -> PAUSE
// 	if freshlyCreated &&
// 		(service.K8sSettings.K8sCronJobSettingsDto.SourceType == dtos.GIT_REPOSITORY ||
// 			service.K8sSettings.K8sCronJobSettingsDto.SourceType == dtos.GIT_REPOSITORY_TEMPLATE) {
// 		newCronJob.Spec.Suspend = punqutils.Pointer(true)
// 	} else {
// 		newCronJob.Spec.Suspend = punqutils.Pointer(!service.SwitchedOn)
// 	}

// 	// CRON_JOB SETTINGS
// 	newCronJob.Spec.Schedule = service.K8sSettings.K8sCronJobSettingsDto.Schedule

// 	if service.K8sSettings.K8sCronJobSettingsDto.ActiveDeadlineSeconds > 0 {
// 		newCronJob.Spec.JobTemplate.Spec.ActiveDeadlineSeconds = punqutils.Pointer(service.K8sSettings.K8sCronJobSettingsDto.ActiveDeadlineSeconds)
// 	}
// 	if service.K8sSettings.K8sCronJobSettingsDto.BackoffLimit > 0 {
// 		newCronJob.Spec.JobTemplate.Spec.BackoffLimit = punqutils.Pointer(service.K8sSettings.K8sCronJobSettingsDto.BackoffLimit)
// 	}

// 	// PORTS
// 	if len(service.Ports) > 0 {
// 		newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Ports = []core.ContainerPort{}
// 		// newDeployment.Spec.Template.Spec.Containers[0].Ports = []core.ContainerPort{}
// 		for _, port := range service.Ports {
// 			if port.Expose {
// 				newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Ports = append(newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Ports, core.ContainerPort{
// 					ContainerPort: int32(port.InternalPort),
// 				})
// 			}
// 		}
// 	} else {
// 		newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Ports = nil
// 	}

// 	// RESOURCES
// 	if service.K8sSettings.IsLimitSetup() {
// 		limits := core.ResourceList{}
// 		requests := core.ResourceList{}
// 		limits["cpu"] = resource.MustParse(fmt.Sprintf("%.2fm", service.K8sSettings.LimitCpuCores*1000))
// 		limits["memory"] = resource.MustParse(fmt.Sprintf("%dMi", service.K8sSettings.LimitMemoryMB))
// 		limits["ephemeral-storage"] = resource.MustParse(fmt.Sprintf("%dMi", service.K8sSettings.EphemeralStorageMB))
// 		requests["cpu"] = resource.MustParse("1m")
// 		requests["memory"] = resource.MustParse("1Mi")
// 		requests["ephemeral-storage"] = resource.MustParse(fmt.Sprintf("%dMi", service.K8sSettings.EphemeralStorageMB))
// 		newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources.Limits = limits
// 		newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources.Requests = requests
// 	} else {
// 		newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources.Limits = nil
// 		newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources.Requests = nil
// 	}

// 	newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Name = service.Name

// 	newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command = []string{}
// 	newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Args = []string{}

// 	// IMAGE
// 	if service.ContainerImage != "" {
// 		newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image = service.ContainerImage
// 		if service.ContainerImageCommand != "" {
// 			newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command = punqutils.ParseJsonStringArray(service.ContainerImageCommand)
// 		}
// 		if service.ContainerImageCommandArgs != "" {
// 			newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Args = punqutils.ParseJsonStringArray(service.ContainerImageCommandArgs)
// 		}
// 		if service.ContainerImageRepoSecretDecryptValue != "" {
// 			newCronJob.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets = []core.LocalObjectReference{}
// 			newCronJob.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets = append(newCronJob.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets, core.LocalObjectReference{
// 				Name: fmt.Sprintf("%s-container-secret", service.Name),
// 			})
// 		}
// 	} else {
// 		// this will be setup UNTIL the buildserver overwrites the image with the real one.
// 		if previousCronjob != nil {
// 			newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image = previousCronjob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image
// 		} else {
// 			newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image = "ghcr.io/mogenius/mo-default-backend:latest"
// 		}
// 	}

// 	// ENV VARS
// 	newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env = []core.EnvVar{}
// 	newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts = []core.VolumeMount{}
// 	newCronJob.Spec.JobTemplate.Spec.Template.Spec.Volumes = []core.Volume{}
// 	for _, envVar := range service.EnvVars {
// 		if envVar.Type == "KEY_VAULT" ||
// 			envVar.Type == "PLAINTEXT" ||
// 			envVar.Type == "HOSTNAME" {
// 			newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env = append(newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env, core.EnvVar{
// 				Name: envVar.Name,
// 				ValueFrom: &core.EnvVarSource{
// 					SecretKeyRef: &core.SecretKeySelector{
// 						Key: envVar.Name,
// 						LocalObjectReference: core.LocalObjectReference{
// 							Name: service.Name,
// 						},
// 					},
// 				},
// 			})
// 		}
// 		if envVar.Type == "VOLUME_MOUNT" {
// 			// VOLUMEMOUNT
// 			// EXAMPLE FOR value CONTENTS: VOLUME_NAME:/LOCATION_CONTAINER_DIR
// 			components := strings.Split(envVar.Value, ":")
// 			if len(components) == 3 {
// 				volumeName := components[0]    // e.g. MY_COOL_NAME
// 				srcPath := components[1]       // e.g. subpath/to/heaven
// 				containerPath := components[2] // e.g. /mo-data

// 				// subPath must be relative
// 				if strings.HasPrefix(srcPath, "/") {
// 					srcPath = strings.Replace(srcPath, "/", "", 1)
// 				}
// 				newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts = append(newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts, core.VolumeMount{
// 					MountPath: containerPath,
// 					SubPath:   srcPath,
// 					Name:      volumeName,
// 				})

// 				// VOLUME
// 				nfsService := ServiceForNfsVolume(namespace.Name, volumeName)
// 				if nfsService != nil {
// 					newCronJob.Spec.JobTemplate.Spec.Template.Spec.Volumes = append(newCronJob.Spec.JobTemplate.Spec.Template.Spec.Volumes, core.Volume{
// 						Name: volumeName,
// 						VolumeSource: core.VolumeSource{
// 							NFS: &core.NFSVolumeSource{
// 								Path:   "/exports",
// 								Server: nfsService.Spec.ClusterIP,
// 							},
// 						},
// 					})
// 				} else {
// 					logger.Log.Errorf("No Volume found for  '%s/%s'!!!", namespace.Name, volumeName)
// 				}
// 			} else {
// 				logger.Log.Errorf("SKIPPING ENVVAR '%s' because value '%s' must conform to pattern XXX:YYY:ZZZ", envVar.Type, envVar.Value)
// 			}
// 		}
// 	}

// 	// IMAGE PULL SECRET
// 	if ContainerSecretDoesExistForStage(namespace) {
// 		containerSecretName := "container-secret-" + namespace.Name
// 		newCronJob.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets = []core.LocalObjectReference{}
// 		newCronJob.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets = append(newCronJob.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets, core.LocalObjectReference{Name: containerSecretName})
// 	}

// 	// PROBES OFF
// 	// not implemented

// 	// SECURITY CONTEXT
// 	// TODO wieder in betrieb nehmen
// 	//structs.StateDebugLog(fmt.Sprintf("securityContext of '%s' removed from deployment. BENE MUST SOLVE THIS!", service.K8sName))
// 	newCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].SecurityContext = nil

// 	return newCronJob
// }

func SetCronJobImage(job *structs.Job, namespaceName string, serviceName string, imageName string, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Set CronJob Image '%s'", imageName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Set Image in CronJob '%s'.", serviceName))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		cronjobClient := provider.ClientSet.BatchV1().CronJobs(namespaceName)
		cronjobToUpdate, err := cronjobClient.Get(context.TODO(), serviceName, metav1.GetOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("SetCronJobImage ERROR: %s", err.Error()))
			return
		}

		// SET NEW IMAGE
		cronjobToUpdate.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image = imageName
		cronjobToUpdate.Spec.Suspend = punqutils.Pointer(false)

		_, err = cronjobClient.Update(context.TODO(), cronjobToUpdate, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("SetCronJobImage ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Set new image in CronJob '%s'.", serviceName))
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
		logger.Log.Errorf("AllCronjobs ERROR: %s", err.Error())
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
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		logger.Log.Errorf("Error: %s", string(output))
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
