package services

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/kubernetes"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	v1 "k8s.io/api/apps/v1"
	v1job "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/rest"
)

func CreateService(r ServiceCreateRequest, c *websocket.Conn) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Create Service "+r.Namespace.DisplayName+"/"+r.Stage.DisplayName, r.Namespace.Id, &r.Stage.Id, &r.Service.Id, c)
	job.Start(c)
	job.AddCmd(mokubernetes.CreateSecret(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.CreateDeployment(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.CreateService(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.CreateNetworkPolicyService(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.UpdateIngress(&job, r.Namespace.ShortId, r.Stage, nil, nil, c, &wg))
	if r.Service.App.Type == "DOCKER_TEMPLATE" {
		initDocker(r.Service, job, c)
	}
	wg.Wait()
	job.Finish(c)
	return job
}

func DeleteService(r ServiceDeleteRequest, c *websocket.Conn) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Delete Service "+r.Namespace.DisplayName+"/"+r.Stage.DisplayName, r.Namespace.Id, &r.Stage.Id, &r.Service.Id, c)
	job.Start(c)
	job.AddCmd(mokubernetes.DeleteService(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.DeleteSecret(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.DeleteDeployment(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.UpdateIngress(&job, r.Namespace.ShortId, r.Stage, nil, nil, c, &wg))
	wg.Wait()
	job.Finish(c)
	return job
}

func SetImage(r ServiceSetImageRequest, c *websocket.Conn) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Set new image for service "+r.ServiceDisplayName, r.NamespaceId, &r.StageId, &r.ServiceId, c)
	job.Start(c)
	job.AddCmd(kubernetes.SetImage(&job, r.StageK8sName, r.ServiceK8sName, r.ImageName, c, &wg))
	wg.Wait()
	job.Finish(c)
	return job
}

func ServicePodIds(r ServiceGetPodIdsRequest, c *websocket.Conn) interface{} {
	return kubernetes.PodIdsFor(r.Namespace, &r.ServiceId)
}

func ServicePodExists(r ServicePodExistsRequest, c *websocket.Conn) interface{} {
	return kubernetes.PodExists(r.K8sNamespace, r.K8sPod)
}

func PodLog(r ServiceGetLogRequest, c *websocket.Conn) interface{} {
	return mokubernetes.GetLog(r.Namespace, r.PodId, r.Timestamp)
}

func PodLogError(r ServiceGetLogRequest, c *websocket.Conn) interface{} {
	return mokubernetes.GetLogError(r.Namespace, r.PodId)
}

func PodLogStream(r ServiceLogStreamRequest, c *websocket.Conn) (*rest.Request, error) {
	return mokubernetes.StreamLog(r.Namespace, r.PodId, int64(r.SinceSeconds))
}

func PodStatus(r ServiceResourceStatusRequest, c *websocket.Conn) interface{} {
	return mokubernetes.PodStatus(r.Resource, r.Namespace, r.Name, r.StatusOnly)
}

func Restart(r ServiceRestartRequest, c *websocket.Conn) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Restart Service "+r.Stage.DisplayName, r.Namespace.Id, &r.Stage.Id, &r.Service.Id, c)
	job.Start(c)
	job.AddCmd(mokubernetes.RestartDeployment(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.UpdateService(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.UpdateIngress(&job, r.Namespace.ShortId, r.Stage, nil, nil, c, &wg))
	wg.Wait()
	job.Finish(c)
	return job
}

func StopService(r ServiceStopRequest, c *websocket.Conn) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Stop Service "+r.Stage.DisplayName, r.NamespaceId, &r.Stage.Id, &r.Service.Id, c)
	job.Start(c)
	job.AddCmd(mokubernetes.StopDeployment(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.UpdateService(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.UpdateIngress(&job, r.NamespaceShortId, r.Stage, nil, nil, c, &wg))
	wg.Wait()
	job.Finish(c)
	return job
}

func StartService(r ServiceStartRequest, c *websocket.Conn) interface{} {
	var wg sync.WaitGroup

	job := structs.CreateJob("Start Service "+r.Stage.DisplayName, r.NamespaceId, &r.Stage.Id, &r.Service.Id, c)
	job.Start(c)
	job.AddCmd(mokubernetes.StartDeployment(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.UpdateService(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.UpdateDeployment(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.UpdateIngress(&job, r.NamespaceShortId, r.Stage, nil, nil, c, &wg))
	wg.Wait()
	job.Finish(c)
	return job
}

func UpdateService(r ServiceUpdateRequest, c *websocket.Conn) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Update Service "+r.Namespace.DisplayName+"/"+r.Stage.DisplayName, r.Namespace.Id, &r.Stage.Id, &r.Service.Id, c)
	job.Start(c)
	job.AddCmd(mokubernetes.UpdateService(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.UpdateSecrete(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.UpdateDeployment(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.UpdateIngress(&job, r.Namespace.ShortId, r.Stage, nil, nil, c, &wg))
	wg.Wait()
	job.Finish(c)
	return job
}

func BindSpectrum(r ServiceBindSpectrumRequest, c *websocket.Conn) (interface{}, error) {
	if r.ExternalPort < 9999 && r.ExternalPort > 65536 {
		return nil, fmt.Errorf("port must be >9999 and <65536")
	}
	if r.InternalPort <= 0 && r.InternalPort > 65536 {
		return nil, fmt.Errorf("port must be >9999 and <65536")
	}
	if r.Type != "TCP" && r.Type != "UDP" {
		return nil, fmt.Errorf("type musst be TCP or UDP")
	}

	configMapName := fmt.Sprintf("%s-services", strings.ToLower(r.Type))
	externalPortStr := fmt.Sprintf("%d", r.ExternalPort)
	fullServiceName := fmt.Sprintf("%s/%s:%d", r.K8sNamespaceName, r.K8sServiceName, r.InternalPort)

	var wg sync.WaitGroup
	job := structs.CreateJob(fmt.Sprintf("Bind: Port %d:%d/%s", r.InternalPort, r.ExternalPort, r.Type), r.NamespaceId, nil, nil, c)
	job.Start(c)
	job.AddCmd(mokubernetes.AddKeyToConfigMap(&job, "default", configMapName, externalPortStr, fullServiceName, c, &wg))
	job.AddCmd(mokubernetes.AddPortToService(&job, "default", "nginx-ingress-ingress-nginx-controller", int32(r.ExternalPort), r.Type, c, &wg))
	wg.Wait()
	job.Finish(c)
	return &job, nil
}

func UnbindSpectrum(r ServiceUnbindSpectrumRequest, c *websocket.Conn) (*structs.Job, error) {
	if r.ExternalPort < 9999 && r.ExternalPort > 65536 {
		return nil, fmt.Errorf("port must be >9999 and <65536")
	}
	if r.Type != "TCP" && r.Type != "UDP" {
		return nil, fmt.Errorf("type musst be TCP or UDP")
	}

	var wg sync.WaitGroup
	job := structs.CreateJob(fmt.Sprintf("Unbind: Port %d/%s", r.ExternalPort, r.Type), r.NamespaceId, nil, nil, c)
	job.Start(c)
	configMapName := fmt.Sprintf("%s-services", strings.ToLower(r.Type))
	externalPortStr := fmt.Sprintf("%d", r.ExternalPort)
	job.AddCmd(mokubernetes.RemoveKeyFromConfigMap(&job, "default", configMapName, externalPortStr, c, &wg))
	job.AddCmd(mokubernetes.RemovePortFromService(&job, "default", "nginx-ingress-ingress-nginx-controller", int32(r.ExternalPort), c, &wg))
	wg.Wait()
	job.Finish(c)
	return &job, nil
}

func SpectrumConfigmaps(c *websocket.Conn) dtos.SpectrumConfigmapDto {
	return dtos.SpectrumConfigmapDto{
		IngressServices: mokubernetes.ServiceFor("default", "nginx-ingress-ingress-nginx-controller"),
		TcpServices:     mokubernetes.ConfigMapFor("default", "tcp-services"),
		UdpServices:     mokubernetes.ConfigMapFor("default", "udp-services"),
	}
}

func initDocker(service dtos.K8sServiceDto, job structs.Job, c *websocket.Conn) []*structs.Command {
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
	structs.ExecuteBashCommandSilent("Commit", fmt.Sprintf(`cd %s; git add . ; git commit -m "[skip ci]: Add inital files."`, gitDir))
	structs.ExecuteBashCommandSilent("Push", fmt.Sprintf("cd %s; git push --set-upstream origin %s", gitDir, service.GitBranch))
	structs.ExecuteBashCommandSilent("Cleanup", fmt.Sprintf("rm -rf %s", gitDir))
	structs.ExecuteBashCommandSilent("Wait", "sleep 5")
	return cmds
}

type ServiceCreateRequest struct {
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Stage     dtos.K8sStageDto     `json:"stage"`
	Service   dtos.K8sServiceDto   `json:"service"`
}

func ServiceCreateRequestExample() ServiceCreateRequest {
	return ServiceCreateRequest{
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Stage:     dtos.K8sStageDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}

type ServiceDeleteRequest struct {
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Stage     dtos.K8sStageDto     `json:"stage"`
	Service   dtos.K8sServiceDto   `json:"service"`
}

func ServiceDeleteRequestExample() ServiceDeleteRequest {
	return ServiceDeleteRequest{
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Stage:     dtos.K8sStageDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}

type ServiceGetPodIdsRequest struct {
	Namespace string `json:"namespace"`
	ServiceId string `json:"serviceId"`
}

func ServiceGetPodIdsRequestExample() ServiceGetPodIdsRequest {
	return ServiceGetPodIdsRequest{
		Namespace: "default",
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

type ServiceSetImageRequest struct {
	NamespaceId        string `json:"namespaceId"`
	StageId            string `json:"stageId"`
	ServiceId          string `json:"serviceId"`
	StageK8sName       string `json:"stageK8sName"`
	ServiceK8sName     string `json:"serviceK8sName"`
	ServiceDisplayName string `json:"serviceDisplayName"`
	ImageName          string `json:"imageName"`
}

func ServiceSetImageRequestExample() ServiceSetImageRequest {
	return ServiceSetImageRequest{
		NamespaceId:        "NAMESPACEID",
		ServiceId:          "SERVICEID",
		StageId:            "STAGEID",
		StageK8sName:       "StageK8sName",
		ServiceK8sName:     "ServiceK8sName",
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
	Id           string `json:"id"`
	Namespace    string `json:"namespace"`
	PodId        string `json:"podId"`
	SinceSeconds int    `json:"sinceSeconds"`
}

func ServiceLogStreamRequestExample() ServiceLogStreamRequest {
	return ServiceLogStreamRequest{
		Id:           "6BBE797E-D559-48B9-9810-1D25A7FF9167",
		Namespace:    "gcp2-new-xrrllb-y0y3g6",
		PodId:        "nginx-63uleb-686867bb6c-bsdvl",
		SinceSeconds: -1,
	}
}

type K8sListRequest struct {
	NamespaceName string `json:"namespaceName"` // empty string for all namespaces
}

func K8sListRequestExample() K8sListRequest {
	return K8sListRequest{
		NamespaceName: "mo7-behrang-prod-vbtv0w",
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

type ServiceLogStreamResult struct {
	NamespaceId string `json:"namespaceId"`
	Message     string `json:"message"`
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
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Stage     dtos.K8sStageDto     `json:"stage"`
	Service   dtos.K8sServiceDto   `json:"service"`
}

func ServiceRestartRequestExample() ServiceRestartRequest {
	return ServiceRestartRequest{
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Stage:     dtos.K8sStageDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}

type ServiceStopRequest struct {
	NamespaceId      string             `json:"namespaceId"`
	NamespaceShortId string             `json:"namespaceShortId"`
	Stage            dtos.K8sStageDto   `json:"stage"`
	Service          dtos.K8sServiceDto `json:"service"`
}

func ServiceStopRequestExample() ServiceStopRequest {
	return ServiceStopRequest{
		NamespaceId:      "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		NamespaceShortId: "y123as",
		Stage:            dtos.K8sStageDtoExampleData(),
		Service:          dtos.K8sServiceDtoExampleData(),
	}
}

type ServiceStartRequest struct {
	NamespaceId      string             `json:"namespaceId"`
	NamespaceShortId string             `json:"namespaceShortId"`
	Stage            dtos.K8sStageDto   `json:"stage"`
	Service          dtos.K8sServiceDto `json:"service"`
}

func ServiceStartRequestExample() ServiceStartRequest {
	return ServiceStartRequest{
		NamespaceId:      "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		NamespaceShortId: "y123as",
		Stage:            dtos.K8sStageDtoExampleData(),
		Service:          dtos.K8sServiceDtoExampleData(),
	}
}

type ServiceUpdateRequest struct {
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Stage     dtos.K8sStageDto     `json:"stage"`
	Service   dtos.K8sServiceDto   `json:"service"`
}

func ServiceUpdateRequestExample() ServiceUpdateRequest {
	return ServiceUpdateRequest{
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Stage:     dtos.K8sStageDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}

type ServiceBindSpectrumRequest struct {
	K8sNamespaceName string `json:"k8sNamespaceName"`
	K8sServiceName   string `json:"k8sServiceName"`
	ExternalPort     int    `json:"externalPort"`
	InternalPort     int    `json:"internalPort"`
	Type             string `json:"type"`
	NamespaceId      string `json:"namespaceId"`
}

func ServiceBindSpectrumRequestExample() ServiceBindSpectrumRequest {
	return ServiceBindSpectrumRequest{
		K8sNamespaceName: "lalalal123",
		K8sServiceName:   "lulululu123",
		ExternalPort:     12345,
		InternalPort:     80,
		Type:             "TCP",
		NamespaceId:      "DAF08780-9C55-4A56-BF3C-471FEEE93C41",
	}
}

type ServiceUnbindSpectrumRequest struct {
	ExternalPort int    `json:"externalPort"`
	Type         string `json:"type"`
	NamespaceId  string `json:"namespaceId"`
}

func ServiceUnbindSpectrumRequestExample() ServiceUnbindSpectrumRequest {
	return ServiceUnbindSpectrumRequest{
		ExternalPort: 12345,
		Type:         "TCP",
		NamespaceId:  "DAF08780-9C55-4A56-BF3C-471FEEE93C41",
	}
}
