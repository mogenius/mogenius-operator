package services

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"
	"time"

	v1cm "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	v1 "k8s.io/api/apps/v1"
	v2 "k8s.io/api/autoscaling/v2"
	v1job "k8s.io/api/batch/v1"
	coordination "k8s.io/api/coordination/v1"
	core "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	rbac "k8s.io/api/rbac/v1"
	scheduling "k8s.io/api/scheduling/v1"
	storage "k8s.io/api/storage/v1"
	"k8s.io/client-go/rest"

	punq "github.com/mogenius/punq/kubernetes"
	punqStructs "github.com/mogenius/punq/structs"
	punqUtils "github.com/mogenius/punq/utils"
)

func CreateService(r ServiceCreateRequest) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Create Service "+r.Project.DisplayName+"/"+r.Namespace.DisplayName, r.Project.Id, &r.Namespace.Id, &r.Service.Id)
	job.Start()

	// check if namespace exists and CREATE IT IF NOT
	nsExists, nsErr := punq.NamespaceExists(r.Namespace.Name, nil)
	if nsErr != nil {
		logger.Log.Warning(nsErr.Error())
	}
	if !nsExists {
		nsReq := NamespaceCreateRequest{
			Project:   r.Project,
			Namespace: r.Namespace,
		}
		job.AddCmds(CreateNamespaceCmds(&job, nsReq, &wg))
	}

	if r.Project.ContainerRegistryUser != nil && r.Project.ContainerRegistryPat != nil {
		job.AddCmd(mokubernetes.CreateOrUpdateContainerSecret(&job, r.Project, r.Namespace, &wg))
	}
	if r.Service.GetImageRepoSecretDecryptValue() != nil {
		job.AddCmd(mokubernetes.CreateOrUpdateContainerSecretForService(&job, r.Project, r.Namespace, r.Service, &wg))
	}

	job.AddCmd(mokubernetes.CreateSecret(&job, r.Namespace, r.Service, &wg))

	switch r.Service.Controller {
	case dtos.DEPLOYMENT:
		job.AddCmd(mokubernetes.CreateDeployment(&job, r.Namespace, r.Service, &wg))
	case dtos.CRON_JOB:
		job.AddCmd(mokubernetes.CreateCronJob(&job, r.Namespace, r.Service, &wg))
	}

	if r.Service.HasPorts() {
		job.AddCmd(mokubernetes.CreateService(&job, r.Namespace, r.Service, &wg))
		job.AddCmd(mokubernetes.CreateNetworkPolicyService(&job, r.Namespace, r.Service, &wg))
	}
	job.AddCmd(mokubernetes.UpdateIngress(&job, r.Namespace, r.Service, &wg))
	if r.Service.HasSeedRepo() {
		initDocker(r.Service, job)
	}
	if r.Service.HasContainerWithGitRepo() {
		updateInfrastructureYaml(r.Service, job)
	}
	wg.Wait()
	job.Finish()
	return job
}

func DeleteService(r ServiceDeleteRequest) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Delete Service "+r.Project.DisplayName+"/"+r.Namespace.DisplayName, r.Project.Id, &r.Namespace.Id, &r.Service.Id)
	job.Start()
	job.AddCmd(mokubernetes.DeleteService(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.DeleteSecret(&job, r.Namespace, r.Service, &wg))

	switch r.Service.Controller {
	case dtos.DEPLOYMENT:
		job.AddCmd(mokubernetes.DeleteDeployment(&job, r.Namespace, r.Service, &wg))
	case dtos.CRON_JOB:
		job.AddCmd(mokubernetes.DeleteCronJob(&job, r.Namespace, r.Service, &wg))
	}

	job.AddCmd(mokubernetes.DeleteNetworkPolicyService(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.DeleteIngress(&job, r.Namespace, r.Service, &wg))
	wg.Wait()
	job.Finish()
	return job
}

// func SetImage(r ServiceSetImageRequest) interface{} {
// 	var wg sync.WaitGroup
// 	job := structs.CreateJob("Set new image for service "+r.ServiceDisplayName, r.ProjectId, &r.NamespaceId, &r.ServiceId)
// 	job.Start()

// 	switch r.ServiceType {
// 	case dtos.K8S_DEPLOYMENT:
// 		job.AddCmd(mokubernetes.SetDeploymentImage(&job, r.NamespaceName, r.ControllerName, r.ImageName, &wg))
// 	case dtos.K8S_CRON_JOB_CONTAINER_IMAGE, dtos.K8S_CRON_JOB_CONTAINER_IMAGE_TEMPLATE:
// 		job.AddCmd(mokubernetes.SetCronJobImage(&job, r.NamespaceName, r.ControllerName, r.ImageName, r.co&wg))
// 	}

// 	wg.Wait()
// 	job.Finish()
// 	return job
// }

func ServicePodIds(r ServiceGetPodIdsRequest) interface{} {
	return punq.PodIdsFor(r.Namespace, &r.ServiceId, nil)
}

func ServicePodExists(r ServicePodExistsRequest) interface{} {
	return punq.PodExists(r.K8sNamespace, r.K8sPod, nil)
}

func PodLog(r ServiceGetLogRequest) interface{} {
	return punq.GetLog(r.Namespace, r.PodId, r.Timestamp, nil)
}

func PodLogError(r ServiceGetLogRequest) interface{} {
	return punq.GetLogError(r.Namespace, r.PodId, nil)
}

func PodLogStream(r ServiceLogStreamRequest) (*rest.Request, error) {
	return punq.StreamLog(r.Namespace, r.PodId, int64(r.SinceSeconds), nil)
}

func PreviousPodLogStream(namespace, podName string) (*rest.Request, error) {
	return punq.StreamPreviousLog(namespace, podName, nil)
}

func PodStatus(r ServiceResourceStatusRequest) interface{} {
	return punq.PodStatus(r.Namespace, r.Name, r.StatusOnly, nil)
}

func ServicePodStatus(r ServicePodsRequest) interface{} {
	return punq.ServicePodStatus(r.Namespace, r.ControllerName, nil)
}

func TriggerJobService(r ServiceTriggerJobRequest) interface{} {
	var wg sync.WaitGroup

	job := structs.CreateJob("Trigger Job Service "+r.NamespaceDisplayName, r.ProjectId, &r.NamespaceId, &r.ServiceId)
	job.Start()
	job.AddCmd(mokubernetes.TriggerJobFromCronjob(&job, r.NamespaceName, r.ControllerName, &wg))

	wg.Wait()
	job.Finish()
	return job
}

func Restart(r ServiceRestartRequest) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Restart Service "+r.Namespace.DisplayName, r.ProjectId, &r.Namespace.Id, &r.Service.Id)
	job.Start()

	switch r.Service.Controller {
	case dtos.DEPLOYMENT:
		job.AddCmd(mokubernetes.RestartDeployment(&job, r.Namespace, r.Service, &wg))
	case dtos.CRON_JOB:
		job.AddCmd(mokubernetes.RestartCronJob(&job, r.Namespace, r.Service, &wg))
	}

	job.AddCmd(mokubernetes.UpdateService(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.UpdateIngress(&job, r.Namespace, r.Service, &wg))
	wg.Wait()
	job.Finish()
	return job
}

func StopService(r ServiceStopRequest) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Stop Service "+r.Namespace.DisplayName, r.ProjectId, &r.Namespace.Id, &r.Service.Id)
	job.Start()

	switch r.Service.Controller {
	case dtos.DEPLOYMENT:
		job.AddCmd(mokubernetes.StopDeployment(&job, r.Namespace, r.Service, &wg))
	case dtos.CRON_JOB:
		job.AddCmd(mokubernetes.StopCronJob(&job, r.Namespace, r.Service, &wg))
	}

	job.AddCmd(mokubernetes.UpdateService(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.UpdateIngress(&job, r.Namespace, r.Service, &wg))
	wg.Wait()
	job.Finish()
	return job
}

func StartService(r ServiceStartRequest) interface{} {
	var wg sync.WaitGroup

	job := structs.CreateJob("Start Service "+r.Namespace.DisplayName, r.ProjectId, &r.Namespace.Id, &r.Service.Id)
	job.Start()

	switch r.Service.Controller {
	case dtos.DEPLOYMENT:
		job.AddCmd(mokubernetes.StartDeployment(&job, r.Namespace, r.Service, &wg))
	case dtos.CRON_JOB:
		job.AddCmd(mokubernetes.StartCronJob(&job, r.Namespace, r.Service, &wg))
	}

	job.AddCmd(mokubernetes.UpdateService(&job, r.Namespace, r.Service, &wg))

	switch r.Service.Controller {
	case dtos.DEPLOYMENT:
		job.AddCmd(mokubernetes.UpdateDeployment(&job, r.Namespace, r.Service, &wg))
	case dtos.CRON_JOB:
		job.AddCmd(mokubernetes.UpdateCronJob(&job, r.Namespace, r.Service, &wg))
	}

	job.AddCmd(mokubernetes.UpdateIngress(&job, r.Namespace, r.Service, &wg))
	wg.Wait()
	job.Finish()
	return job
}

func UpdateService(r ServiceUpdateRequest) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Update Service "+r.Project.DisplayName+"/"+r.Namespace.DisplayName, r.Project.Id, &r.Namespace.Id, &r.Service.Id)
	job.Start()
	if r.Project.ContainerRegistryUser != nil && r.Project.ContainerRegistryPat != nil {
		job.AddCmd(mokubernetes.CreateOrUpdateContainerSecret(&job, r.Project, r.Namespace, &wg))
	}
	if r.Service.GetImageRepoSecretDecryptValue() != nil {
		job.AddCmd(mokubernetes.CreateOrUpdateContainerSecretForService(&job, r.Project, r.Namespace, r.Service, &wg))
	}
	job.AddCmd(mokubernetes.UpdateService(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.UpdateSecrete(&job, r.Namespace, r.Service, &wg))

	switch r.Service.Controller {
	case dtos.DEPLOYMENT:
		job.AddCmd(mokubernetes.UpdateDeployment(&job, r.Namespace, r.Service, &wg))
	case dtos.CRON_JOB:
		job.AddCmd(mokubernetes.UpdateCronJob(&job, r.Namespace, r.Service, &wg))
	}

	updateInfrastructureYaml(r.Service, job)

	job.AddCmd(mokubernetes.UpdateIngress(&job, r.Namespace, r.Service, &wg))
	wg.Wait()
	job.Finish()
	return job
}

func TcpUdpClusterConfiguration() dtos.TcpUdpClusterConfigurationDto {
	return dtos.TcpUdpClusterConfigurationDto{
		IngressServices: punq.ServiceFor(utils.CONFIG.Kubernetes.OwnNamespace, "mogenius-ingress-nginx-controller", nil),
		TcpServices:     punq.ConfigMapFor(utils.CONFIG.Kubernetes.OwnNamespace, "mogenius-ingress-nginx-tcp", false, nil),
		UdpServices:     punq.ConfigMapFor(utils.CONFIG.Kubernetes.OwnNamespace, "mogenius-ingress-nginx-udp", false, nil),
	}
}

func initDocker(service dtos.K8sServiceDto, job structs.Job) []*structs.Command {
	tempDir := "/temp"
	gitDir := fmt.Sprintf("%s/%s", tempDir, service.Id)

	for _, container := range service.Containers {
		if container.GitRepository == nil {
			logger.Log.Errorf("%s: GitRepository cannot be nil", container.Name)
			continue
		}
		if container.GitBranch == nil {
			logger.Log.Errorf("%s: GitBranch cannot be nil", container.Name)
			continue
		}
		punqStructs.ExecuteShellCommandSilent("Cleanup", fmt.Sprintf("mkdir %s; rm -rf %s", tempDir, gitDir))
		punqStructs.ExecuteShellCommandSilent("Clone", fmt.Sprintf("cd %s; git clone %s %s; cd %s; git switch %s", tempDir, *container.GitRepository, gitDir, gitDir, *container.GitBranch))
		if container.AppSetupCommands != nil {
			punqStructs.ExecuteShellCommandSilent("Run Setup Commands", fmt.Sprintf("cd %s; %s", gitDir, *container.AppSetupCommands))
		}
		if container.AppGitRepositoryCloneUrl != nil {
			punqStructs.ExecuteShellCommandSilent("Clone files from template", fmt.Sprintf("git clone %s %s/__TEMPLATE__; rm -rf %s/__TEMPLATE__/.git; cp -rf %s/__TEMPLATE__/. %s/.; rm -rf %s/__TEMPLATE__/", *container.AppGitRepositoryCloneUrl, gitDir, gitDir, gitDir, gitDir, gitDir))
		}
		punqStructs.ExecuteShellCommandSilent("Commit", fmt.Sprintf(`cd %s; git add . ; git commit -m "[skip ci]: Add initial files."`, gitDir))
		punqStructs.ExecuteShellCommandSilent("Push", fmt.Sprintf("cd %s; git push --set-upstream origin %s", gitDir, *container.GitBranch))
		punqStructs.ExecuteShellCommandSilent("Cleanup", fmt.Sprintf("rm -rf %s", gitDir))
		punqStructs.ExecuteShellCommandSilent("Wait", "sleep 5")
	}
	return []*structs.Command{}
}

func updateInfrastructureYaml(service dtos.K8sServiceDto, job structs.Job) []*structs.Command {
	for _, container := range service.Containers {
		if container.SettingsYaml != nil && container.GitBranch != nil && container.GitRepository != nil {
			if container.GitRepository == nil {
				logger.Log.Errorf("%s: GitRepository cannot be nil", container.Name)
				continue
			}
			if container.GitBranch == nil {
				logger.Log.Errorf("%s: GitBranch cannot be nil", container.Name)
				continue
			}
			if container.SettingsYaml == nil {
				logger.Log.Errorf("%s: SettingsYaml cannot be nil", container.Name)
				continue
			}

			tempDir := "/temp"
			gitDir := fmt.Sprintf("%s/%s", tempDir, service.Id)

			punqStructs.ExecuteShellCommandSilent("Cleanup", fmt.Sprintf("mkdir %s; rm -rf %s", tempDir, gitDir))
			punqStructs.ExecuteShellCommandSilent("Clone", fmt.Sprintf("cd %s; git clone %s %s; cd %s; git switch %s", tempDir, *container.GitRepository, gitDir, gitDir, *container.GitBranch))

			punqStructs.ExecuteShellCommandSilent("Update infrastructure YAML", fmt.Sprintf("cd %s; mkdir -p .mogenius; echo '%s' > .mogenius/%s-%s.yaml", gitDir, *container.SettingsYaml, container.Name, *container.GitBranch))

			punqStructs.ExecuteShellCommandSilent("Commit", fmt.Sprintf(`cd %s; git add .mogenius/%s-%s.yaml ; git commit -m "[skip ci]: Update infrastructure yaml."`, gitDir, container.Name, *container.GitBranch))
			punqStructs.ExecuteShellCommandSilent("Push", fmt.Sprintf("cd %s; git push --set-upstream origin %s", gitDir, *container.GitBranch))
			punqStructs.ExecuteShellCommandSilent("Cleanup", fmt.Sprintf("rm -rf %s", gitDir))
		}
	}

	return []*structs.Command{}
}

type ServiceCreateRequest struct {
	Project   dtos.K8sProjectDto   `json:"project" validate:"required"`
	Namespace dtos.K8sNamespaceDto `json:"namespace" validate:"required"`
	Service   dtos.K8sServiceDto   `json:"service" validate:"required"`
}

func ServiceCreateRequestExample() ServiceCreateRequest {
	return ServiceCreateRequest{
		Project:   dtos.K8sProjectDtoExampleData(),
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
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
		Timestamp: punqUtils.Pointer(time.Now()),
	}
}

type ServiceLogStreamRequest struct {
	Namespace    string `json:"namespace" validate:"required"`
	PodId        string `json:"podId" validate:"required"`
	SinceSeconds int    `json:"sinceSeconds" validate:"required"`
	PostTo       string `json:"postTo" validate:"required"`
}

type PodCmdConnectionRequest struct {
	Namespace     string                     `json:"namespace" validate:"required"`
	Pod           string                     `json:"pod" validate:"required"`
	Container     string                     `json:"container" validate:"required"`
	CmdConnection utils.CmdConnectionRequest `json:"cmdConnectionRequest" validate:"required"`
	LogTail       string                     `json:"logTail"`
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

type K8sListRequest struct {
	NamespaceName string `json:"namespaceName"` // empty string for all namespaces
}

func K8sListRequestExample() K8sListRequest {
	return K8sListRequest{
		NamespaceName: "",
	}
}

type K8sDescribeRequest struct {
	NamespaceName string `json:"namespaceName"`
	ResourceName  string `json:"resourceName"`
}

func K8sDescribeRequestExample() K8sDescribeRequest {
	return K8sDescribeRequest{
		NamespaceName: "mogenius",
		ResourceName:  "mogenius-k8s-manager",
	}
}

type K8sUpdateDeploymentRequest struct {
	Data *v1.Deployment `json:"data" validate:"required"`
}

func K8sUpdateDeploymentRequestExample() K8sUpdateDeploymentRequest {
	return K8sUpdateDeploymentRequest{
		Data: nil,
	}
}

type K8sUpdateServiceRequest struct {
	Data *core.Service `json:"data" validate:"required"`
}

func K8sUpdateServiceRequestExample() K8sUpdateServiceRequest {
	return K8sUpdateServiceRequest{
		Data: nil,
	}
}

type K8sUpdatePodRequest struct {
	Data *core.Pod `json:"data" validate:"required"`
}

func K8sUpdatePodRequestExample() K8sUpdatePodRequest {
	return K8sUpdatePodRequest{
		Data: nil,
	}
}

type K8sUpdateIngressRequest struct {
	Data *netv1.Ingress `json:"data" validate:"required"`
}

func K8sUpdateIngressRequestExample() K8sUpdateIngressRequest {
	return K8sUpdateIngressRequest{
		Data: nil,
	}
}

type K8sUpdateConfigmapRequest struct {
	Data *core.ConfigMap `json:"data" validate:"required"`
}

func K8sUpdateConfigmapRequestExample() K8sUpdateConfigmapRequest {
	return K8sUpdateConfigmapRequest{
		Data: nil,
	}
}

type K8sUpdateSecretRequest struct {
	Data *core.Secret `json:"data" validate:"required"`
}

func K8sUpdateSecretRequestExample() K8sUpdateSecretRequest {
	return K8sUpdateSecretRequest{
		Data: nil,
	}
}

type K8sUpdateDaemonSetRequest struct {
	Data *v1.DaemonSet `json:"data" validate:"required"`
}

func K8sUpdateDaemonsetRequestExample() K8sUpdateDaemonSetRequest {
	return K8sUpdateDaemonSetRequest{
		Data: nil,
	}
}

type K8sUpdateStatefulSetRequest struct {
	Data *v1.StatefulSet `json:"data" validate:"required"`
}

func K8sUpdateStatefulSetRequestExample() K8sUpdateStatefulSetRequest {
	return K8sUpdateStatefulSetRequest{
		Data: nil,
	}
}

type K8sUpdateJobRequest struct {
	Data *v1job.Job `json:"data" validate:"required"`
}

func K8sUpdateJobRequestExample() K8sUpdateJobRequest {
	return K8sUpdateJobRequest{
		Data: nil,
	}
}

type K8sUpdateCronJobRequest struct {
	Data *v1job.CronJob `json:"data" validate:"required"`
}

func K8sUpdateCronJobRequestExample() K8sUpdateCronJobRequest {
	return K8sUpdateCronJobRequest{
		Data: nil,
	}
}

type K8sUpdateReplicaSetRequest struct {
	Data *v1.ReplicaSet `json:"data" validate:"required"`
}

func K8sUpdateReplicaSetRequestExample() K8sUpdateReplicaSetRequest {
	return K8sUpdateReplicaSetRequest{
		Data: nil,
	}
}

type K8sUpdatePersistentVolumeRequest struct {
	Data *core.PersistentVolume `json:"data" validate:"required"`
}

func K8sUpdatePersistentVolumeRequestExample() K8sUpdatePersistentVolumeRequest {
	return K8sUpdatePersistentVolumeRequest{
		Data: nil,
	}
}

type K8sUpdatePersistentVolumeClaimRequest struct {
	Data *core.PersistentVolumeClaim `json:"data" validate:"required"`
}

func K8sUpdatePersistentVolumeClaimRequestExample() K8sUpdatePersistentVolumeClaimRequest {
	return K8sUpdatePersistentVolumeClaimRequest{
		Data: nil,
	}
}

type K8sUpdateHPARequest struct {
	Data *v2.HorizontalPodAutoscaler `json:"data" validate:"required"`
}

func K8sUpdateHPARequestExample() K8sUpdateHPARequest {
	return K8sUpdateHPARequest{
		Data: nil,
	}
}

type K8sUpdateCertificateRequest struct {
	Data *cmapi.Certificate `json:"data" validate:"required"`
}

func K8sUpdateCertificateExample() K8sUpdateCertificateRequest {
	return K8sUpdateCertificateRequest{
		Data: nil,
	}
}

type K8sUpdateCertificateRequestRequest struct {
	Data *cmapi.CertificateRequest `json:"data" validate:"required"`
}

func K8sUpdateCertificateRequestExample() K8sUpdateCertificateRequestRequest {
	return K8sUpdateCertificateRequestRequest{
		Data: nil,
	}
}

type K8sUpdateOrderRequest struct {
	Data *v1cm.Order `json:"data" validate:"required"`
}

func K8sUpdateOrderExample() K8sUpdateOrderRequest {
	return K8sUpdateOrderRequest{
		Data: nil,
	}
}

type K8sUpdateIssuerRequest struct {
	Data *cmapi.Issuer `json:"data" validate:"required"`
}

func K8sUpdateIssuerExample() K8sUpdateIssuerRequest {
	return K8sUpdateIssuerRequest{
		Data: nil,
	}
}

type K8sUpdateClusterIssuerRequest struct {
	Data *cmapi.ClusterIssuer `json:"data" validate:"required"`
}

func K8sUpdateClusterIssuerExample() K8sUpdateClusterIssuerRequest {
	return K8sUpdateClusterIssuerRequest{
		Data: nil,
	}
}

type K8sUpdateServiceAccountRequest struct {
	Data *core.ServiceAccount `json:"data" validate:"required"`
}

func K8sUpdateServiceAccountExample() K8sUpdateServiceAccountRequest {
	return K8sUpdateServiceAccountRequest{
		Data: nil,
	}
}

type K8sUpdateRoleRequest struct {
	Data *rbac.Role `json:"data" validate:"required"`
}

func K8sUpdateRoleExample() K8sUpdateRoleRequest {
	return K8sUpdateRoleRequest{
		Data: nil,
	}
}

type K8sUpdateRoleBindingRequest struct {
	Data *rbac.RoleBinding `json:"data" validate:"required"`
}

func K8sUpdateRoleBindingExample() K8sUpdateRoleBindingRequest {
	return K8sUpdateRoleBindingRequest{
		Data: nil,
	}
}

type K8sUpdateClusterRoleRequest struct {
	Data *rbac.ClusterRole `json:"data" validate:"required"`
}

func K8sUpdateClusterRoleExample() K8sUpdateClusterRoleRequest {
	return K8sUpdateClusterRoleRequest{
		Data: nil,
	}
}

type K8sUpdateClusterRoleBindingRequest struct {
	Data *rbac.ClusterRoleBinding `json:"data" validate:"required"`
}

func K8sUpdateClusterRoleBindingExample() K8sUpdateClusterRoleBindingRequest {
	return K8sUpdateClusterRoleBindingRequest{
		Data: nil,
	}
}

type K8sUpdateVolumeAttachmentRequest struct {
	Data *storage.VolumeAttachment `json:"data" validate:"required"`
}

func K8sUpdateVolumeAttachmentExample() K8sUpdateVolumeAttachmentRequest {
	return K8sUpdateVolumeAttachmentRequest{
		Data: nil,
	}
}

type K8sUpdateNetworkPolicyRequest struct {
	Data *netv1.NetworkPolicy `json:"data" validate:"required"`
}

func K8sUpdateNetworkPolicyExample() K8sUpdateNetworkPolicyRequest {
	return K8sUpdateNetworkPolicyRequest{
		Data: nil,
	}
}

type K8sUpdateStorageClassRequest struct {
	Data *storage.StorageClass `json:"data" validate:"required"`
}

func K8sUpdateStorageClassExample() K8sUpdateStorageClassRequest {
	return K8sUpdateStorageClassRequest{
		Data: nil,
	}
}

type K8sUpdatePriorityClassRequest struct {
	Data *scheduling.PriorityClass `json:"data" validate:"required"`
}

func K8sUpdatePriorityClassExample() K8sUpdatePriorityClassRequest {
	return K8sUpdatePriorityClassRequest{
		Data: nil,
	}
}

type K8sUpdateEndpointRequest struct {
	Data *core.Endpoints `json:"data" validate:"required"`
}

func K8sUpdateEndpointExample() K8sUpdateEndpointRequest {
	return K8sUpdateEndpointRequest{
		Data: nil,
	}
}

type K8sUpdateLeaseRequest struct {
	Data *coordination.Lease `json:"data" validate:"required"`
}

func K8sUpdateLeaseExample() K8sUpdateLeaseRequest {
	return K8sUpdateLeaseRequest{
		Data: nil,
	}
}

type K8sUpdateResourceQuotaRequest struct {
	Data *core.ResourceQuota `json:"data" validate:"required"`
}

func K8sUpdateResourceQuotaExample() K8sUpdateResourceQuotaRequest {
	return K8sUpdateResourceQuotaRequest{
		Data: nil,
	}
}

type K8sDeleteResourceRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func K8sDeleteResourceRequestExample() K8sDeleteResourceRequest {
	return K8sDeleteResourceRequest{
		Namespace: "",
		Name:      "",
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
	ProjectId string               `json:"projectId" validate:"required"`
	Namespace dtos.K8sNamespaceDto `json:"namespace" validate:"required"`
	Service   dtos.K8sServiceDto   `json:"service" validate:"required"`
}

func ServiceRestartRequestExample() ServiceRestartRequest {
	return ServiceRestartRequest{
		ProjectId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
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
	ProjectId string               `json:"projectId" validate:"required"`
	Namespace dtos.K8sNamespaceDto `json:"namespace" validate:"required"`
	Service   dtos.K8sServiceDto   `json:"service" validate:"required"`
}

func ServiceStartRequestExample() ServiceStartRequest {
	return ServiceStartRequest{
		ProjectId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
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
