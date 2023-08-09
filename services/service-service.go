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
)

func CreateService(r ServiceCreateRequest) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Create Service "+r.Project.DisplayName+"/"+r.Namespace.DisplayName, r.Project.Id, &r.Namespace.Id, &r.Service.Id)
	job.Start()

	// check if namespace exists and CREATE IT IF NOT
	nsExists, nsErr := mokubernetes.NamespaceExists(r.Namespace.Name)
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

	if r.Project.ContainerRegistryUser != "" && r.Project.ContainerRegistryPat != "" {
		job.AddCmd(mokubernetes.CreateOrUpdateContainerSecret(&job, r.Project, r.Namespace, &wg))
	}

	job.AddCmd(mokubernetes.CreateSecret(&job, r.Namespace, r.Service, &wg))

	switch r.Service.ServiceType {
	case dtos.GitRepositoryTemplate, dtos.GitRepository, dtos.ContainerImageTemplate, dtos.ContainerImage, dtos.K8SDeployment:
		job.AddCmd(mokubernetes.CreateDeployment(&job, r.Namespace, r.Service, &wg))
	case dtos.K8SCronJob:
		job.AddCmd(mokubernetes.CreateCronJob(&job, r.Namespace, r.Service, &wg))
	}

	job.AddCmd(mokubernetes.CreateService(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.CreateNetworkPolicyService(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.UpdateIngress(&job, r.Namespace, nil, nil, &wg))
	if r.Service.App.Type == "DOCKER_TEMPLATE" {
		initDocker(r.Service, job)
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
	job.AddCmd(mokubernetes.DeleteDeployment(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.DeleteNetworkPolicyService(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.UpdateIngress(&job, r.Namespace, nil, nil, &wg))
	wg.Wait()
	job.Finish()
	return job
}

func SetImage(r ServiceSetImageRequest) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Set new image for service "+r.ServiceDisplayName, r.ProjectId, &r.NamespaceId, &r.ServiceId)
	job.Start()

	switch r.ServiceType {
	case dtos.GitRepositoryTemplate, dtos.GitRepository, dtos.ContainerImageTemplate, dtos.ContainerImage, dtos.K8SDeployment:
		job.AddCmd(mokubernetes.SetDeploymentImage(&job, r.NamespaceName, r.ServiceName, r.ImageName, &wg))
	case dtos.K8SCronJob:
		job.AddCmd(mokubernetes.SetCronJobImage(&job, r.NamespaceName, r.ServiceName, r.ImageName, &wg))
	}
	
	wg.Wait()
	job.Finish()
	return job
}

func ServicePodIds(r ServiceGetPodIdsRequest) interface{} {
	return mokubernetes.PodIdsFor(r.Namespace, &r.ServiceId)
}

func ServicePodExists(r ServicePodExistsRequest) interface{} {
	return mokubernetes.PodExists(r.K8sNamespace, r.K8sPod)
}

func PodLog(r ServiceGetLogRequest) interface{} {
	return mokubernetes.GetLog(r.Namespace, r.PodId, r.Timestamp)
}

func PodLogError(r ServiceGetLogRequest) interface{} {
	return mokubernetes.GetLogError(r.Namespace, r.PodId)
}

func PodLogStream(r ServiceLogStreamRequest) (*rest.Request, error) {
	return mokubernetes.StreamLog(r.Namespace, r.PodId, int64(r.SinceSeconds))
}

func PreviousPodLogStream(r ServiceLogStreamRequest) (*rest.Request, error) {
	return mokubernetes.StreamPreviousLog(r.Namespace, r.PodId)
}

func PodStatus(r ServiceResourceStatusRequest) interface{} {
	return mokubernetes.PodStatus(r.Namespace, r.Name, r.StatusOnly)
}

func ServicePodStatus(r ServicePodsRequest) interface{} {
	return mokubernetes.ServicePodStatus(r.Namespace, r.ServiceName)
}

func Restart(r ServiceRestartRequest) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Restart Service "+r.Namespace.DisplayName, r.ProjectId, &r.Namespace.Id, &r.Service.Id)
	job.Start()
	job.AddCmd(mokubernetes.RestartDeployment(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.UpdateService(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.UpdateIngress(&job, r.Namespace, nil, nil, &wg))
	wg.Wait()
	job.Finish()
	return job
}

func StopService(r ServiceStopRequest) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Stop Service "+r.Namespace.DisplayName, r.ProjectId, &r.Namespace.Id, &r.Service.Id)
	job.Start()
	job.AddCmd(mokubernetes.StopDeployment(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.UpdateService(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.UpdateIngress(&job, r.Namespace, nil, nil, &wg))
	wg.Wait()
	job.Finish()
	return job
}

func StartService(r ServiceStartRequest) interface{} {
	var wg sync.WaitGroup

	job := structs.CreateJob("Start Service "+r.Namespace.DisplayName, r.ProjectId, &r.Namespace.Id, &r.Service.Id)
	job.Start()
	job.AddCmd(mokubernetes.StartDeployment(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.UpdateService(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.UpdateDeployment(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.UpdateIngress(&job, r.Namespace, nil, nil, &wg))
	wg.Wait()
	job.Finish()
	return job
}

func UpdateService(r ServiceUpdateRequest) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Update Service "+r.Project.DisplayName+"/"+r.Namespace.DisplayName, r.Project.Id, &r.Namespace.Id, &r.Service.Id)
	job.Start()
	if r.Project.ContainerRegistryUser != "" && r.Project.ContainerRegistryPat != "" {
		job.AddCmd(mokubernetes.CreateOrUpdateContainerSecret(&job, r.Project, r.Namespace, &wg))
	}
	job.AddCmd(mokubernetes.UpdateService(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.UpdateSecrete(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.UpdateDeployment(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.UpdateIngress(&job, r.Namespace, nil, nil, &wg))
	wg.Wait()
	job.Finish()
	return job
}

func TcpUdpClusterConfiguration() dtos.TcpUdpClusterConfigurationDto {
	return dtos.TcpUdpClusterConfigurationDto{
		IngressServices: mokubernetes.ServiceFor(utils.CONFIG.Kubernetes.OwnNamespace, "mogenius-ingress-nginx-controller"),
		TcpServices:     mokubernetes.ConfigMapFor(utils.CONFIG.Kubernetes.OwnNamespace, "mogenius-ingress-nginx-tcp"),
		UdpServices:     mokubernetes.ConfigMapFor(utils.CONFIG.Kubernetes.OwnNamespace, "mogenius-ingress-nginx-udp"),
	}
}

func initDocker(service dtos.K8sServiceDto, job structs.Job) []*structs.Command {
	tempDir := "/temp"
	gitDir := fmt.Sprintf("%s/%s", tempDir, service.Id)

	fmt.Println(tempDir)
	fmt.Println(gitDir)

	cmds := []*structs.Command{}
	structs.ExecuteBashCommandSilent("Cleanup", fmt.Sprintf("mkdir %s; rm -rf %s", tempDir, gitDir))
	structs.ExecuteBashCommandSilent("Clone", fmt.Sprintf("cd %s; git clone %s %s; cd %s; git switch %s", tempDir, service.GitRepository, gitDir, gitDir, service.GitBranch))
	if service.App.SetupCommands != "" {
		structs.ExecuteBashCommandSilent("Run Setup Commands ...", fmt.Sprintf("cd %s; %s", gitDir, service.App.SetupCommands))
	}
	if service.App.RepositoryLink != "" {
		structs.ExecuteBashCommandSilent("Clone files from template ...", fmt.Sprintf("git clone %s %s/__TEMPLATE__; rm -rf %s/__TEMPLATE__/.git; cp -rf %s/__TEMPLATE__/. %s/.; rm -rf %s/__TEMPLATE__/", service.App.RepositoryLink, gitDir, gitDir, gitDir, gitDir, gitDir))
	}
	structs.ExecuteBashCommandSilent("Commit", fmt.Sprintf(`cd %s; git add . ; git commit -m "[skip ci]: Add initial files."`, gitDir))
	structs.ExecuteBashCommandSilent("Push", fmt.Sprintf("cd %s; git push --set-upstream origin %s", gitDir, service.GitBranch))
	structs.ExecuteBashCommandSilent("Cleanup", fmt.Sprintf("rm -rf %s", gitDir))
	structs.ExecuteBashCommandSilent("Wait", "sleep 5")
	return cmds
}

type ServiceCreateRequest struct {
	Project   dtos.K8sProjectDto   `json:"project"`
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Service   dtos.K8sServiceDto   `json:"service"`
}

func ServiceCreateRequestExample() ServiceCreateRequest {
	return ServiceCreateRequest{
		Project:   dtos.K8sProjectDtoExampleData(),
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}

type ServiceDeleteRequest struct {
	Project   dtos.K8sProjectDto   `json:"project"`
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Service   dtos.K8sServiceDto   `json:"service"`
}

func ServiceDeleteRequestExample() ServiceDeleteRequest {
	return ServiceDeleteRequest{
		Project:   dtos.K8sProjectDtoExampleData(),
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}

type ServiceGetPodIdsRequest struct {
	Namespace string `json:"namespace"`
	ServiceId string `json:"serviceId"`
}

func ServiceGetPodIdsRequestExample() ServiceGetPodIdsRequest {
	return ServiceGetPodIdsRequest{
		Namespace: "mogenius",
		ServiceId: "mo-",
	}
}

type ServicePodExistsRequest struct {
	K8sNamespace string `json:"k8sNamespace"`
	K8sPod       string `json:"k8sPod"`
}

func ServicePodExistsRequestExample() ServicePodExistsRequest {
	return ServicePodExistsRequest{
		K8sNamespace: "mogenius",
		K8sPod:       "mogenius-traffic-collector-jfnjw",
	}
}

type ServicePodsRequest struct {
	Namespace   string `json:"namespace"`
	ServiceName string `json:"serviceName"`
}

func ServicePodsRequestExample() ServicePodsRequest {
	return ServicePodsRequest{
		Namespace:   "mogenius",
		ServiceName: "k8s",
	}
}

type ServiceSetImageRequest struct {
	ProjectId          string                     `json:"projectId"`
	NamespaceId        string                     `json:"namespaceId"`
	ServiceId          string                     `json:"serviceId"`
	NamespaceName      string                     `json:"namespaceName"`
	ServiceName        string                     `json:"serviceName"`
	ServiceDisplayName string                     `json:"serviceDisplayName"`
	ImageName          string                     `json:"imageName"`
	ServiceType        dtos.K8sServiceTypeEnum    `json:"serviceType,omitempty"`
}

func (service *ServiceSetImageRequest) ApplyDefaults() {
	if service.ServiceType == "" {
		service.ServiceType = dtos.K8SDeployment
	} 
}


func ServiceSetImageRequestExample() ServiceSetImageRequest {
	return ServiceSetImageRequest{
		ProjectId:          "PROJECTID",
		ServiceId:          "SERVICEID",
		NamespaceId:        "NAMESPACEID",
		NamespaceName:      "NAMESPACENAMe",
		ServiceName:        "SERVICENAME",
		ServiceDisplayName: "ServiceDisplayName",
		ImageName:          "nginx:latest",
	}
}

type ServiceGetLogRequest struct {
	Namespace string     `json:"namespace"`
	PodId     string     `json:"podId"`
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
	Namespace    string `json:"namespace"`
	PodId        string `json:"podId"`
	SinceSeconds int    `json:"sinceSeconds"`
	PostTo       string `json:"postTo"`
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
	Data *v1.Deployment `json:"data"`
}

func K8sUpdateDeploymentRequestExample() K8sUpdateDeploymentRequest {
	return K8sUpdateDeploymentRequest{
		Data: nil,
	}
}

type K8sUpdateServiceRequest struct {
	Data *core.Service `json:"data"`
}

func K8sUpdateServiceRequestExample() K8sUpdateServiceRequest {
	return K8sUpdateServiceRequest{
		Data: nil,
	}
}

type K8sUpdatePodRequest struct {
	Data *core.Pod `json:"data"`
}

func K8sUpdatePodRequestExample() K8sUpdatePodRequest {
	return K8sUpdatePodRequest{
		Data: nil,
	}
}

type K8sUpdateIngressRequest struct {
	Data *netv1.Ingress `json:"data"`
}

func K8sUpdateIngressRequestExample() K8sUpdateIngressRequest {
	return K8sUpdateIngressRequest{
		Data: nil,
	}
}

type K8sUpdateConfigmapRequest struct {
	Data *core.ConfigMap `json:"data"`
}

func K8sUpdateConfigmapRequestExample() K8sUpdateConfigmapRequest {
	return K8sUpdateConfigmapRequest{
		Data: nil,
	}
}

type K8sUpdateSecretRequest struct {
	Data *core.Secret `json:"data"`
}

func K8sUpdateSecretRequestExample() K8sUpdateSecretRequest {
	return K8sUpdateSecretRequest{
		Data: nil,
	}
}

type K8sUpdateDaemonSetRequest struct {
	Data *v1.DaemonSet `json:"data"`
}

func K8sUpdateDaemonsetRequestExample() K8sUpdateDaemonSetRequest {
	return K8sUpdateDaemonSetRequest{
		Data: nil,
	}
}

type K8sUpdateStatefulSetRequest struct {
	Data *v1.StatefulSet `json:"data"`
}

func K8sUpdateStatefulSetRequestExample() K8sUpdateStatefulSetRequest {
	return K8sUpdateStatefulSetRequest{
		Data: nil,
	}
}

type K8sUpdateJobRequest struct {
	Data *v1job.Job `json:"data"`
}

func K8sUpdateJobRequestExample() K8sUpdateJobRequest {
	return K8sUpdateJobRequest{
		Data: nil,
	}
}

type K8sUpdateCronJobRequest struct {
	Data *v1job.CronJob `json:"data"`
}

func K8sUpdateCronJobRequestExample() K8sUpdateCronJobRequest {
	return K8sUpdateCronJobRequest{
		Data: nil,
	}
}

type K8sUpdateReplicaSetRequest struct {
	Data *v1.ReplicaSet `json:"data"`
}

func K8sUpdateReplicaSetRequestExample() K8sUpdateReplicaSetRequest {
	return K8sUpdateReplicaSetRequest{
		Data: nil,
	}
}

type K8sUpdatePersistentVolumeRequest struct {
	Data *core.PersistentVolume `json:"data"`
}

func K8sUpdatePersistentVolumeRequestExample() K8sUpdatePersistentVolumeRequest {
	return K8sUpdatePersistentVolumeRequest{
		Data: nil,
	}
}

type K8sUpdatePersistentVolumeClaimRequest struct {
	Data *core.PersistentVolumeClaim `json:"data"`
}

func K8sUpdatePersistentVolumeClaimRequestExample() K8sUpdatePersistentVolumeClaimRequest {
	return K8sUpdatePersistentVolumeClaimRequest{
		Data: nil,
	}
}

type K8sUpdateHPARequest struct {
	Data *v2.HorizontalPodAutoscaler `json:"data"`
}

func K8sUpdateHPARequestExample() K8sUpdateHPARequest {
	return K8sUpdateHPARequest{
		Data: nil,
	}
}

type K8sUpdateCertificateRequest struct {
	Data *cmapi.Certificate `json:"data"`
}

func K8sUpdateCertificateExample() K8sUpdateCertificateRequest {
	return K8sUpdateCertificateRequest{
		Data: nil,
	}
}

type K8sUpdateCertificateRequestRequest struct {
	Data *cmapi.CertificateRequest `json:"data"`
}

func K8sUpdateCertificateRequestExample() K8sUpdateCertificateRequestRequest {
	return K8sUpdateCertificateRequestRequest{
		Data: nil,
	}
}

type K8sUpdateOrderRequest struct {
	Data *v1cm.Order `json:"data"`
}

func K8sUpdateOrderExample() K8sUpdateOrderRequest {
	return K8sUpdateOrderRequest{
		Data: nil,
	}
}

type K8sUpdateIssuerRequest struct {
	Data *cmapi.Issuer `json:"data"`
}

func K8sUpdateIssuerExample() K8sUpdateIssuerRequest {
	return K8sUpdateIssuerRequest{
		Data: nil,
	}
}

type K8sUpdateClusterIssuerRequest struct {
	Data *cmapi.ClusterIssuer `json:"data"`
}

func K8sUpdateClusterIssuerExample() K8sUpdateClusterIssuerRequest {
	return K8sUpdateClusterIssuerRequest{
		Data: nil,
	}
}

type K8sUpdateServiceAccountRequest struct {
	Data *core.ServiceAccount `json:"data"`
}

func K8sUpdateServiceAccountExample() K8sUpdateServiceAccountRequest {
	return K8sUpdateServiceAccountRequest{
		Data: nil,
	}
}

type K8sUpdateRoleRequest struct {
	Data *rbac.Role `json:"data"`
}

func K8sUpdateRoleExample() K8sUpdateRoleRequest {
	return K8sUpdateRoleRequest{
		Data: nil,
	}
}

type K8sUpdateRoleBindingRequest struct {
	Data *rbac.RoleBinding `json:"data"`
}

func K8sUpdateRoleBindingExample() K8sUpdateRoleBindingRequest {
	return K8sUpdateRoleBindingRequest{
		Data: nil,
	}
}

type K8sUpdateClusterRoleRequest struct {
	Data *rbac.ClusterRole `json:"data"`
}

func K8sUpdateClusterRoleExample() K8sUpdateClusterRoleRequest {
	return K8sUpdateClusterRoleRequest{
		Data: nil,
	}
}

type K8sUpdateClusterRoleBindingRequest struct {
	Data *rbac.ClusterRoleBinding `json:"data"`
}

func K8sUpdateClusterRoleBindingExample() K8sUpdateClusterRoleBindingRequest {
	return K8sUpdateClusterRoleBindingRequest{
		Data: nil,
	}
}

type K8sUpdateVolumeAttachmentRequest struct {
	Data *storage.VolumeAttachment `json:"data"`
}

func K8sUpdateVolumeAttachmentExample() K8sUpdateVolumeAttachmentRequest {
	return K8sUpdateVolumeAttachmentRequest{
		Data: nil,
	}
}

type K8sUpdateNetworkPolicyRequest struct {
	Data *netv1.NetworkPolicy `json:"data"`
}

func K8sUpdateNetworkPolicyExample() K8sUpdateNetworkPolicyRequest {
	return K8sUpdateNetworkPolicyRequest{
		Data: nil,
	}
}

type K8sUpdateStorageClassRequest struct {
	Data *storage.StorageClass `json:"data"`
}

func K8sUpdateStorageClassExample() K8sUpdateStorageClassRequest {
	return K8sUpdateStorageClassRequest{
		Data: nil,
	}
}

type K8sUpdatePriorityClassRequest struct {
	Data *scheduling.PriorityClass `json:"data"`
}

func K8sUpdatePriorityClassExample() K8sUpdatePriorityClassRequest {
	return K8sUpdatePriorityClassRequest{
		Data: nil,
	}
}

type K8sUpdateEndpointRequest struct {
	Data *core.Endpoints `json:"data"`
}

func K8sUpdateEndpointExample() K8sUpdateEndpointRequest {
	return K8sUpdateEndpointRequest{
		Data: nil,
	}
}

type K8sUpdateLeaseRequest struct {
	Data *coordination.Lease `json:"data"`
}

func K8sUpdateLeaseExample() K8sUpdateLeaseRequest {
	return K8sUpdateLeaseRequest{
		Data: nil,
	}
}

type K8sUpdateResourceQuotaRequest struct {
	Data *core.ResourceQuota `json:"data"`
}

func K8sUpdateResourceQuotaExample() K8sUpdateResourceQuotaRequest {
	return K8sUpdateResourceQuotaRequest{
		Data: nil,
	}
}

type K8sDeleteNamespaceRequest struct {
	Data *core.Namespace `json:"data"`
}

func K8sDeleteNamespaceRequestExample() K8sDeleteNamespaceRequest {
	return K8sDeleteNamespaceRequest{
		Data: nil,
	}
}

type K8sDeleteDeploymentRequest struct {
	Data *v1.Deployment `json:"data"`
}

func K8sDeleteDeploymentRequestExample() K8sDeleteDeploymentRequest {
	return K8sDeleteDeploymentRequest{
		Data: nil,
	}
}

type K8sDeleteServiceRequest struct {
	Data *core.Service `json:"data"`
}

func K8sDeleteServiceRequestExample() K8sDeleteServiceRequest {
	return K8sDeleteServiceRequest{
		Data: nil,
	}
}

type K8sDeletePodRequest struct {
	Data *core.Pod `json:"data"`
}

func K8sDeletePodRequestExample() K8sDeletePodRequest {
	return K8sDeletePodRequest{
		Data: nil,
	}
}

type K8sDeleteIngressRequest struct {
	Data *netv1.Ingress `json:"data"`
}

func K8sDeleteIngressRequestExample() K8sDeleteIngressRequest {
	return K8sDeleteIngressRequest{
		Data: nil,
	}
}

type K8sDeleteConfigmapRequest struct {
	Data *core.ConfigMap `json:"data"`
}

func K8sDeleteConfigmapRequestExample() K8sDeleteConfigmapRequest {
	return K8sDeleteConfigmapRequest{
		Data: nil,
	}
}

type K8sDeleteSecretRequest struct {
	Data *core.Secret `json:"data"`
}

func K8sDeleteSecretRequestExample() K8sDeleteSecretRequest {
	return K8sDeleteSecretRequest{
		Data: nil,
	}
}

type K8sDeleteDaemonsetRequest struct {
	Data *v1.DaemonSet `json:"data"`
}

func K8sDeleteDaemonsetRequestExample() K8sDeleteDaemonsetRequest {
	return K8sDeleteDaemonsetRequest{
		Data: nil,
	}
}

type K8sDeleteStatefulsetRequest struct {
	Data *v1.StatefulSet `json:"data"`
}

func K8sDeleteStatefulsetRequestExample() K8sDeleteStatefulsetRequest {
	return K8sDeleteStatefulsetRequest{
		Data: nil,
	}
}

type K8sDeleteJobRequest struct {
	Data *v1job.Job `json:"data"`
}

func K8sDeleteJobRequestExample() K8sDeleteJobRequest {
	return K8sDeleteJobRequest{
		Data: nil,
	}
}

type K8sDeleteCronjobRequest struct {
	Data *v1job.CronJob `json:"data"`
}

func K8sDeleteCronjobRequestExample() K8sDeleteCronjobRequest {
	return K8sDeleteCronjobRequest{
		Data: nil,
	}
}

type K8sDeleteReplicasetRequest struct {
	Data *v1.ReplicaSet `json:"data"`
}

func K8sDeleteReplicaSetRequestExample() K8sDeleteReplicasetRequest {
	return K8sDeleteReplicasetRequest{
		Data: nil,
	}
}

type K8sDeletePersistentVolumeRequest struct {
	Data *core.PersistentVolume `json:"data"`
}

func K8sDeletePersistentVolumeRequestExample() K8sDeletePersistentVolumeRequest {
	return K8sDeletePersistentVolumeRequest{
		Data: nil,
	}
}

type K8sDeletePersistentVolumeClaimRequest struct {
	Data *core.PersistentVolumeClaim `json:"data"`
}

func K8sDeletePersistentVolumeClaimRequestExample() K8sDeletePersistentVolumeClaimRequest {
	return K8sDeletePersistentVolumeClaimRequest{
		Data: nil,
	}
}

type K8sDeleteHPARequest struct {
	Data *v2.HorizontalPodAutoscaler `json:"data"`
}

func K8sDeleteHPAExample() K8sDeleteHPARequest {
	return K8sDeleteHPARequest{
		Data: nil,
	}
}

type K8sDeleteCertificateRequest struct {
	Data *cmapi.Certificate `json:"data"`
}

func K8sDeleteCertificateExample() K8sDeleteCertificateRequest {
	return K8sDeleteCertificateRequest{
		Data: nil,
	}
}

type K8sDeleteCertificateRequestRequest struct {
	Data *cmapi.CertificateRequest `json:"data"`
}

func K8sDeleteCertificateRequestExample() K8sDeleteCertificateRequestRequest {
	return K8sDeleteCertificateRequestRequest{
		Data: nil,
	}
}

type K8sDeleteOrderRequest struct {
	Data *v1cm.Order `json:"data"`
}

func K8sDeleteOrderExample() K8sDeleteOrderRequest {
	return K8sDeleteOrderRequest{
		Data: nil,
	}
}

type K8sDeleteIssuerRequest struct {
	Data *cmapi.Issuer `json:"data"`
}

func K8sDeleteIssuerExample() K8sDeleteIssuerRequest {
	return K8sDeleteIssuerRequest{
		Data: nil,
	}
}

type K8sDeleteClusterIssuerRequest struct {
	Data *cmapi.ClusterIssuer `json:"data"`
}

func K8sDeleteClusterIssuerExample() K8sDeleteClusterIssuerRequest {
	return K8sDeleteClusterIssuerRequest{
		Data: nil,
	}
}

type K8sDeleteServiceAccountRequest struct {
	Data *core.ServiceAccount `json:"data"`
}

func K8sDeleteServiceAccountExample() K8sDeleteServiceAccountRequest {
	return K8sDeleteServiceAccountRequest{
		Data: nil,
	}
}

type K8sDeleteRoleRequest struct {
	Data *rbac.Role `json:"data"`
}

func K8sDeleteRoleExample() K8sDeleteRoleRequest {
	return K8sDeleteRoleRequest{
		Data: nil,
	}
}

type K8sDeleteRoleBindingRequest struct {
	Data *rbac.RoleBinding `json:"data"`
}

func K8sDeleteRoleBindingExample() K8sDeleteRoleBindingRequest {
	return K8sDeleteRoleBindingRequest{
		Data: nil,
	}
}

type K8sDeleteClusterRoleRequest struct {
	Data *rbac.ClusterRole `json:"data"`
}

func K8sDeleteClusterRoleExample() K8sDeleteClusterRoleRequest {
	return K8sDeleteClusterRoleRequest{
		Data: nil,
	}
}

type K8sDeleteClusterRoleBindingRequest struct {
	Data *rbac.ClusterRoleBinding `json:"data"`
}

func K8sDeleteClusterRoleBindingExample() K8sDeleteClusterRoleBindingRequest {
	return K8sDeleteClusterRoleBindingRequest{
		Data: nil,
	}
}

type K8sDeleteVolumeAttachmentRequest struct {
	Data *storage.VolumeAttachment `json:"data"`
}

func K8sDeleteVolumeAttachmentExample() K8sDeleteVolumeAttachmentRequest {
	return K8sDeleteVolumeAttachmentRequest{
		Data: nil,
	}
}

type K8sDeleteNetworkPolicyRequest struct {
	Data *netv1.NetworkPolicy `json:"data"`
}

func K8sDeleteNetworkPolicyExample() K8sDeleteNetworkPolicyRequest {
	return K8sDeleteNetworkPolicyRequest{
		Data: nil,
	}
}

type K8sDeleteStorageClassRequest struct {
	Data *storage.StorageClass `json:"data"`
}

func K8sDeleteStorageClassExample() K8sDeleteStorageClassRequest {
	return K8sDeleteStorageClassRequest{
		Data: nil,
	}
}

type K8sDeleteLeaseRequest struct {
	Data *coordination.Lease `json:"data"`
}

func K8sDeleteLeaseExample() K8sDeleteLeaseRequest {
	return K8sDeleteLeaseRequest{
		Data: nil,
	}
}

type K8sDeletePriorityClassRequest struct {
	Data *scheduling.PriorityClass `json:"data"`
}

func K8sDeletePriorityClassExample() K8sDeletePriorityClassRequest {
	return K8sDeletePriorityClassRequest{
		Data: nil,
	}
}

type K8sDeleteEndpointRequest struct {
	Data *core.Endpoints `json:"data"`
}

func K8sDeleteEndpointExample() K8sDeleteEndpointRequest {
	return K8sDeleteEndpointRequest{
		Data: nil,
	}
}

type K8sDeleteResourceQuotaRequest struct {
	Data *core.ResourceQuota `json:"data"`
}

func K8sDeleteResourceQuotaExample() K8sDeleteResourceQuotaRequest {
	return K8sDeleteResourceQuotaRequest{
		Data: nil,
	}
}

// type K8sDeleteVolumeSnapshotRequest struct {
// 	Data *storagesnap. `json:"data"`
// }

// func K8sDeleteVolumeSnapshotExample() K8sDeleteVolumeSnapshotRequest {
// 	return K8sDeleteVolumeSnapshotRequest{
// 		Data: nil,
// 	}
// }

type ServiceLogStreamResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type ServiceResourceStatusRequest struct {
	Resource   string `json:"resource"` // pods, services, deployments
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
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
	ProjectId string               `json:"projectId"`
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Service   dtos.K8sServiceDto   `json:"service"`
}

func ServiceRestartRequestExample() ServiceRestartRequest {
	return ServiceRestartRequest{
		ProjectId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}

type ServiceStopRequest struct {
	ProjectId string               `json:"projectId"`
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Service   dtos.K8sServiceDto   `json:"service"`
}

func ServiceStopRequestExample() ServiceStopRequest {
	return ServiceStopRequest{
		ProjectId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}

type ServiceStartRequest struct {
	ProjectId string               `json:"projectId"`
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Service   dtos.K8sServiceDto   `json:"service"`
}

func ServiceStartRequestExample() ServiceStartRequest {
	return ServiceStartRequest{
		ProjectId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}

type ServiceUpdateRequest struct {
	Project   dtos.K8sProjectDto   `json:"project"`
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Service   dtos.K8sServiceDto   `json:"service"`
}

func ServiceUpdateRequestExample() ServiceUpdateRequest {
	return ServiceUpdateRequest{
		Project:   dtos.K8sProjectDtoExampleData(),
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}
