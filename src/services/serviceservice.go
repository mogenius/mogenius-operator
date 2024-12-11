package services

import (
	"fmt"
	"mogenius-k8s-manager/src/crds"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/gitmanager"
	"mogenius-k8s-manager/src/kubernetes"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"os"
	"sync"
	"time"

	"k8s.io/client-go/rest"
)

func UpdateService(r ServiceUpdateRequest) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Update Service "+r.Project.DisplayName+"/"+r.Namespace.DisplayName, r.Project.Id, r.Namespace.Name, r.Service.ControllerName)
	job.Start()

	// check if namespace exists and CREATE IT IF NOT
	nsExists, nsErr := kubernetes.NamespaceExists(r.Namespace.Name)
	if nsErr != nil {
		serviceLogger.Warn("failed to check if namespace exists", "error", nsErr)
	}
	if !nsExists {
		nsReq := NamespaceCreateRequest{
			Project:   r.Project,
			Namespace: r.Namespace,
		}
		CreateNamespaceCmds(job, nsReq, &wg)
	}

	mokubernetes.CreateOrUpdateClusterImagePullSecret(job, r.Project, r.Namespace, &wg)
	mokubernetes.CreateOrUpdateContainerImagePullSecret(job, r.Namespace, r.Service, &wg)
	mokubernetes.UpdateOrCreateControllerSecret(job, r.Namespace, r.Service, &wg)
	mokubernetes.UpdateService(job, r.Namespace, r.Service, &wg)
	// mokubernetes.CreateOrUpdateNetworkPolicyService(job, r.Namespace, r.Service, &wg)
	mokubernetes.UpdateIngress(job, r.Namespace, r.Service, &wg)

	switch r.Service.Controller {
	case dtos.DEPLOYMENT:
		mokubernetes.UpdateDeployment(job, r.Namespace, r.Service, &wg)
	case dtos.CRON_JOB:
		mokubernetes.UpdateCronJob(job, r.Namespace, r.Service, &wg)
	}

	if r.Service.HasContainerWithGitRepo() && serviceHasYamlSettings(r.Service) {
		updateInfrastructureYaml(job, r.Service, &wg)
	}

	// crds.CreateOrUpdateApplicationKitCmd(job, r.Namespace.Name, r.Service.ControllerName, crds.CrdApplicationKit{
	// 	Id:          r.Service.Id,
	// 	DisplayName: r.Service.DisplayName,
	// 	CreatedBy:   "MISSING_FIELD",
	// 	Controller:  r.Service.ControllerName,
	// 	AppId:       "MISSING_FIELD",
	// }, &wg)

	go func() {
		wg.Wait()
		job.Finish()
	}()

	return job
}

func DeleteService(r ServiceDeleteRequest) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Delete Service "+r.Project.DisplayName+"/"+r.Namespace.DisplayName, r.Project.Id, r.Namespace.Name, r.Service.ControllerName)
	job.Start()
	mokubernetes.DeleteService(job, r.Namespace, r.Service, &wg)
	mokubernetes.DeleteContainerImagePullSecret(job, r.Namespace, r.Service, &wg)
	mokubernetes.DeleteControllerSecret(job, r.Namespace, r.Service, &wg)

	switch r.Service.Controller {
	case dtos.DEPLOYMENT:
		mokubernetes.DeleteDeployment(job, r.Namespace, r.Service, &wg)
	case dtos.CRON_JOB:
		mokubernetes.DeleteCronJob(job, r.Namespace, r.Service, &wg)
	}

	// EXTERNAL SECRETS OPERATOR - cleanup unused secrets
	mokubernetes.DeleteUnusedSecretsForNamespace(job, r.Namespace, r.Service, &wg)

	mokubernetes.DeleteIngress(job, r.Namespace, r.Service, &wg)

	crds.DeleteApplicationKitCmd(clientProvider.DynamicClient(), job, r.Namespace.Name, r.Service.ControllerName, &wg)

	go func() {
		wg.Wait()
		job.Finish()

		time.Sleep(10 * time.Second)
		for _, container := range r.Service.Containers {
			serviceLogger.Info("Deleting build data", "namespace", r.Namespace.Name, "controller", r.Service.ControllerName, "container", container.Name)
			kubernetes.GetDb().DeleteAllBuildData(r.Namespace.Name, r.Service.ControllerName, container.Name)
		}
	}()

	return job
}

func UpdateSecrets(r ServiceUpdateRequest) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Update Secrets "+r.Project.DisplayName+"/"+r.Namespace.DisplayName, r.Project.Id, r.Namespace.Name, r.Service.ControllerName)
	job.Start()

	mokubernetes.UpdateOrCreateControllerSecret(job, r.Namespace, r.Service, &wg)
	mokubernetes.CreateOrUpdateClusterImagePullSecret(job, r.Project, r.Namespace, &wg)
	mokubernetes.CreateOrUpdateContainerImagePullSecret(job, r.Namespace, r.Service, &wg)

	go func() {
		wg.Wait()
		job.Finish()
	}()

	return job
}

// func SetImage(r ServiceSetImageRequest) interface{} {
// 	var wg sync.WaitGroup
// 	job := structs.CreateJob("Set new image for service "+r.ServiceDisplayName, r.ProjectId, &r.NamespaceId, &r.ServiceId)
// 	job.Start()

// 	switch r.ServiceType {
// 	case dtos.K8S_DEPLOYMENT:
// 		mokubernetes.SetDeploymentImage(job, r.NamespaceName, r.ControllerName, r.ImageName, &wg))
// 	case dtos.K8S_CRON_JOB_CONTAINER_IMAGE, dtos.K8S_CRON_JOB_CONTAINER_IMAGE_TEMPLATE:
// 		mokubernetes.SetCronJobImage(job, r.NamespaceName, r.ControllerName, r.ImageName, r.co&wg))
// 	}

// 	wg.Wait()
// 	job.Finish()
// 	return job
// }

func ServicePodIds(r ServiceGetPodIdsRequest) interface{} {
	return kubernetes.PodIdsFor(r.Namespace, &r.ServiceId)
}

func ServicePodExists(r ServicePodExistsRequest) interface{} {
	return kubernetes.PodExists(r.K8sNamespace, r.K8sPod)
}

func PodLog(r ServiceGetLogRequest) interface{} {
	return kubernetes.GetLog(r.Namespace, r.PodId, r.Timestamp)
}

func PodLogError(r ServiceGetLogRequest) interface{} {
	return kubernetes.GetLogError(r.Namespace, r.PodId)
}

func PodLogStream(r ServiceLogStreamRequest) (*rest.Request, error) {
	return kubernetes.StreamLog(r.Namespace, r.PodId, int64(r.SinceSeconds))
}

func PreviousPodLogStream(namespace, podName string) (*rest.Request, error) {
	return kubernetes.StreamPreviousLog(namespace, podName)
}

func PodStatus(r ServiceResourceStatusRequest) interface{} {
	return kubernetes.PodStatus(r.Namespace, r.Name, r.StatusOnly)
}

var servicePodStatusDebounce = utils.NewDebounce("servicePodStatusDebounce", 1000*time.Millisecond, 300*time.Millisecond)

func ServicePodStatus(r ServicePodsRequest) interface{} {
	key := fmt.Sprintf("%s-%s", r.Namespace, r.ControllerName)
	result, _ := servicePodStatusDebounce.CallFn(key, func() (interface{}, error) {
		return ServicePodStatus2(r), nil
	})
	return result
}

func ServicePodStatus2(r ServicePodsRequest) interface{} {
	return kubernetes.ServicePodStatus(r.Namespace, r.ControllerName)
}

func TriggerJobService(r ServiceTriggerJobRequest) interface{} {
	var wg sync.WaitGroup

	job := structs.CreateJob("Trigger Job Service "+r.NamespaceDisplayName, r.ProjectId, r.NamespaceName, r.ControllerName)
	job.Start()
	mokubernetes.TriggerJobFromCronjob(job, r.NamespaceName, r.ControllerName, &wg)

	go func() {
		wg.Wait()
		job.Finish()
	}()

	return job
}

func Restart(r ServiceRestartRequest) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Restart Service "+r.Namespace.DisplayName, r.Project.Id, r.Namespace.Name, r.Service.ControllerName)
	job.Start()

	mokubernetes.CreateOrUpdateClusterImagePullSecret(job, r.Project, r.Namespace, &wg)
	mokubernetes.CreateOrUpdateContainerImagePullSecret(job, r.Namespace, r.Service, &wg)
	mokubernetes.UpdateService(job, r.Namespace, r.Service, &wg)
	mokubernetes.UpdateOrCreateControllerSecret(job, r.Namespace, r.Service, &wg)
	// mokubernetes.CreateOrUpdateNetworkPolicyService(job, r.Namespace, r.Service, &wg)
	mokubernetes.UpdateIngress(job, r.Namespace, r.Service, &wg)

	switch r.Service.Controller {
	case dtos.DEPLOYMENT:
		mokubernetes.RestartDeployment(job, r.Namespace, r.Service, &wg)
	case dtos.CRON_JOB:
		mokubernetes.RestartCronJob(job, r.Namespace, r.Service, &wg)
	}

	go func() {
		wg.Wait()
		job.Finish()
	}()

	return job
}

func StopService(r ServiceStopRequest) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Stop Service "+r.Namespace.DisplayName, r.ProjectId, r.Namespace.Name, r.Service.ControllerName)
	job.Start()

	switch r.Service.Controller {
	case dtos.DEPLOYMENT:
		mokubernetes.StopDeployment(job, r.Namespace, r.Service, &wg)
	case dtos.CRON_JOB:
		mokubernetes.StopCronJob(job, r.Namespace, r.Service, &wg)
	}

	mokubernetes.UpdateService(job, r.Namespace, r.Service, &wg)
	mokubernetes.UpdateIngress(job, r.Namespace, r.Service, &wg)

	go func() {
		wg.Wait()
		job.Finish()
	}()

	return job
}

func StartService(r ServiceStartRequest) interface{} {
	var wg sync.WaitGroup

	job := structs.CreateJob("Start Service "+r.Service.DisplayName, r.Project.Id, r.Namespace.Name, r.Service.ControllerName)
	job.Start()

	mokubernetes.CreateOrUpdateClusterImagePullSecret(job, r.Project, r.Namespace, &wg)
	mokubernetes.CreateOrUpdateContainerImagePullSecret(job, r.Namespace, r.Service, &wg)
	mokubernetes.UpdateService(job, r.Namespace, r.Service, &wg)
	mokubernetes.UpdateOrCreateControllerSecret(job, r.Namespace, r.Service, &wg)
	// mokubernetes.CreateOrUpdateNetworkPolicyService(job, r.Namespace, r.Service, &wg)
	mokubernetes.UpdateIngress(job, r.Namespace, r.Service, &wg)

	switch r.Service.Controller {
	case dtos.DEPLOYMENT:
		mokubernetes.StartDeployment(job, r.Namespace, r.Service, &wg)
	case dtos.CRON_JOB:
		mokubernetes.StartCronJob(job, r.Namespace, r.Service, &wg)
	}

	go func() {
		wg.Wait()
		job.Finish()
	}()

	return job
}

func TcpUdpClusterConfiguration() dtos.TcpUdpClusterConfigurationDto {
	return dtos.TcpUdpClusterConfigurationDto{
		IngressServices: kubernetes.ServiceFor(config.Get("MO_OWN_NAMESPACE"), "mogenius-ingress-nginx-controller"),
		TcpServices:     kubernetes.ConfigMapFor(config.Get("MO_OWN_NAMESPACE"), "mogenius-ingress-nginx-tcp", false),
		UdpServices:     kubernetes.ConfigMapFor(config.Get("MO_OWN_NAMESPACE"), "mogenius-ingress-nginx-udp", false),
	}
}

func serviceHasYamlSettings(service dtos.K8sServiceDto) bool {
	for _, container := range service.Containers {
		if container.SettingsYaml != nil {
			return true
		}
	}
	return false
}

func updateInfrastructureYaml(job *structs.Job, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("update", "Update infrastructure YAML", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Update infrastructure YAML")

		// dont do this in local environment
		// if config.Get("MO_STAGE") == "local" {
		// 	cmd.Success(job, "Skipping infrastructure YAML update")
		// 	return
		// }

		for _, container := range service.Containers {
			if container.SettingsYaml != nil && container.GitBranch != nil && container.GitRepository != nil {
				if container.GitRepository == nil {
					serviceLogger.Error("GitRepository cannot be nil", "controller", service.ControllerName)
					continue
				}
				if container.GitBranch == nil {
					serviceLogger.Error("GitBranch cannot be nil", "controller", service.ControllerName)
					continue
				}
				if container.SettingsYaml == nil {
					serviceLogger.Error("SettingsYaml cannot be nil", "controller", service.ControllerName)
					continue
				}

				tempDir := os.TempDir()
				gitDir := fmt.Sprintf("%s/%s", tempDir, utils.NanoId())

				err := utils.ExecuteShellCommandSilent("Cleanup", fmt.Sprintf("mkdir %s; rm -rf %s", tempDir, gitDir))
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("Error cleaning up before: %s", err.Error()))
					return
				}
				err = gitmanager.CloneFast(*container.GitRepository, gitDir, *container.GitBranch)
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("Error cloning: %s", err.Error()))
					return
				}

				err = utils.ExecuteShellCommandSilent("Update infrastructure YAML", fmt.Sprintf("cd %s; mkdir -p .mogenius; echo '%s' > .mogenius/%s.yaml", gitDir, *container.SettingsYaml, *container.GitBranch))
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("Error updating file: %s", err.Error()))
					return
				}

				err = gitmanager.Commit(
					gitDir,
					[]string{fmt.Sprintf(".mogenius/%s.yaml", *container.GitBranch)},
					[]string{},
					"[skip ci]: Update infrastructure yaml.",
					config.Get("MO_GIT_USER_NAME"),
					config.Get("MO_GIT_USER_EMAIL"),
				)
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("Error commiting: %s", err.Error()))
					return
				}
				err = gitmanager.Push(gitDir, "origin")
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("Error pushing: %s", err.Error()))
					return
				}
				err = utils.ExecuteShellCommandSilent("Cleanup", fmt.Sprintf("rm -rf %s", gitDir))
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("Error cleaning up after: %s", err.Error()))
					return
				}
			}
		}
		cmd.Success(job, "Update infrastructure YAML")
	}(wg)
}

type ServiceDeleteRequest struct {
	Project   dtos.K8sProjectDto   `json:"project" validate:"required"`
	Namespace dtos.K8sNamespaceDto `json:"namespace" validate:"required"`
	Service   dtos.K8sServiceDto   `json:"service" validate:"required"`
}

func ServiceDeleteRequestExample() ServiceDeleteRequest {
	return ServiceDeleteRequest{
		Project:   dtos.K8sProjectDtoExampleData(),
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}

type ServiceGetPodIdsRequest struct {
	Namespace string `json:"namespace" validate:"required"`
	ServiceId string `json:"serviceId" validate:"required"`
}

func ServiceGetPodIdsRequestExample() ServiceGetPodIdsRequest {
	return ServiceGetPodIdsRequest{
		Namespace: "mogenius",
		ServiceId: "mo-",
	}
}

type ServicePodExistsRequest struct {
	K8sNamespace string `json:"k8sNamespace" validate:"required"`
	K8sPod       string `json:"k8sPod" validate:"required"`
}

func ServicePodExistsRequestExample() ServicePodExistsRequest {
	return ServicePodExistsRequest{
		K8sNamespace: "mogenius",
		K8sPod:       "mogenius-traffic-collector-jfnjw",
	}
}

type ServicePodsRequest struct {
	Namespace      string `json:"namespace" validate:"required"`
	ControllerName string `json:"controllerName" validate:"required"`
}

func ServicePodsRequestExample() ServicePodsRequest {
	return ServicePodsRequest{
		Namespace:      "mogenius",
		ControllerName: "k8s",
	}
}

// type ServiceSetImageRequest struct {
// 	ProjectId          string                  `json:"projectId" validate:"required"`
// 	NamespaceId        string                  `json:"namespaceId" validate:"required"`
// 	ServiceId          string                  `json:"serviceId" validate:"required"`
// 	NamespaceName      string                  `json:"namespaceName" validate:"required"`
// 	ControllerName     string                  `json:"controllerName" validate:"required"`
// 	ServiceDisplayName string                  `json:"serviceDisplayName" validate:"required"`
// 	ImageName          string                  `json:"imageName" validate:"required"`
// 	ServiceType        dtos.K8sServiceTypeEnum `json:"serviceType,omitempty"`
// }

// func ServiceSetImageRequestExample() ServiceSetImageRequest {
// 	return ServiceSetImageRequest{
// 		ProjectId:          "PROJECTID",
// 		ServiceId:          "SERVICEID",
// 		NamespaceId:        "NAMESPACEID",
// 		NamespaceName:      "NAMESPACENAMe",
// 		ControllerName:     "ControllerNAME",
// 		ServiceDisplayName: "ServiceDisplayName",
// 		ImageName:          "nginx:latest",
// 	}
// }

type ServiceGetLogRequest struct {
	Namespace string     `json:"namespace" validate:"required"`
	PodId     string     `json:"podId" validate:"required"`
	Timestamp *time.Time `json:"timestamp"`
}

func ServiceGetLogRequestExample() ServiceGetLogRequest {
	return ServiceGetLogRequest{
		Namespace: "gcp2-new-xrrllb-y0y3g6",
		PodId:     "nginx-63uleb-686867bb6c-bsdvl",
		Timestamp: utils.Pointer(time.Now()),
	}
}

type ServiceLogStreamRequest struct {
	Namespace    string `json:"namespace" validate:"required"`
	PodId        string `json:"podId" validate:"required"`
	SinceSeconds int    `json:"sinceSeconds" validate:"required"`
	PostTo       string `json:"postTo" validate:"required"`
}

type CmdWindowSize struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

func ServiceLogStreamRequestExample() ServiceLogStreamRequest {
	return ServiceLogStreamRequest{
		Namespace:    "mogenius",
		PodId:        "mogenius-k8s-manager-8576c46478-lv6gn",
		SinceSeconds: -1,
		PostTo:       "ws://localhost:8080/path/to/send/data?id=E694180D-4E18-41EC-A4CC-F402EA825D60",
	}
}

type ServiceLogStreamResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type ServiceResourceStatusRequest struct {
	Resource   string `json:"resource" validate:"required"` // pods, services, deployments
	Namespace  string `json:"namespace" validate:"required"`
	Name       string `json:"name" validate:"required"`
	StatusOnly bool   `json:"statusOnly"`
}

func ServiceResourceStatusRequestExample() ServiceResourceStatusRequest {
	return ServiceResourceStatusRequest{
		Resource:   "pods",
		Namespace:  "mogenius",
		Name:       "mogenius-k8s-manager-gcp2-6c969cb878-tcksq",
		StatusOnly: true,
	}
}

type ServiceRestartRequest struct {
	Project   dtos.K8sProjectDto   `json:"project" validate:"required"`
	Namespace dtos.K8sNamespaceDto `json:"namespace" validate:"required"`
	Service   dtos.K8sServiceDto   `json:"service" validate:"required"`
}

func ServiceRestartRequestExample() ServiceRestartRequest {
	return ServiceRestartRequest{
		Project:   dtos.K8sProjectDtoExampleData(),
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}

type ServiceStopRequest struct {
	ProjectId string               `json:"projectId" validate:"required"`
	Namespace dtos.K8sNamespaceDto `json:"namespace" validate:"required"`
	Service   dtos.K8sServiceDto   `json:"service" validate:"required"`
}

func ServiceStopRequestExample() ServiceStopRequest {
	return ServiceStopRequest{
		ProjectId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}

type ServiceStartRequest struct {
	Project   dtos.K8sProjectDto   `json:"project" validate:"required"`
	Namespace dtos.K8sNamespaceDto `json:"namespace" validate:"required"`
	Service   dtos.K8sServiceDto   `json:"service" validate:"required"`
}

func ServiceStartRequestExample() ServiceStartRequest {
	return ServiceStartRequest{
		Project:   dtos.K8sProjectDtoExampleData(),
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}

type ServiceUpdateRequest struct {
	Project   dtos.K8sProjectDto   `json:"project" validate:"required"`
	Namespace dtos.K8sNamespaceDto `json:"namespace" validate:"required"`
	Service   dtos.K8sServiceDto   `json:"service" validate:"required"`
}

func ServiceUpdateRequestExample() ServiceUpdateRequest {
	return ServiceUpdateRequest{
		Project:   dtos.K8sProjectDtoExampleData(),
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}

type ServiceTriggerJobRequest struct {
	ProjectId            string `json:"projectId" validate:"required"`
	NamespaceName        string `json:"namespaceName" validate:"required"`
	NamespaceDisplayName string `json:"namespaceDisplayName" validate:"required"`
	NamespaceId          string `json:"namespaceId" validate:"required"`
	ControllerName       string `json:"controllerName" validate:"required"`
	ServiceId            string `json:"serviceId" validate:"required"`
}

func ServiceTriggerJobRequestExample() ServiceTriggerJobRequest {
	return ServiceTriggerJobRequest{
		ProjectId:      "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		NamespaceName:  "my-namespace",
		ControllerName: "my-service",
	}
}

type ListCronjobJobsRequest struct {
	ProjectId      string `json:"projectId" validate:"required"`
	NamespaceName  string `json:"namespaceName" validate:"required"`
	ControllerName string `json:"controllerName" validate:"required"`
}

func ListJobsByCronJobByServiceRequestExample() ListCronjobJobsRequest {
	return ListCronjobJobsRequest{
		ProjectId:      "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		NamespaceName:  "my-namespace",
		ControllerName: "my-service",
	}
}
