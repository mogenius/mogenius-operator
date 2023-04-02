package services

import (
	mokubernetes "mogenius-k8s-manager/kubernetes"

	"github.com/gorilla/websocket"
)

func K8sUpdateDeployment(r K8sUpdateDeploymentRequest, c *websocket.Conn) interface{} {
	return mokubernetes.UpdateK8sDeployment(*r.Data)
}

func K8sUpdateService(r K8sUpdateServiceRequest, c *websocket.Conn) interface{} {
	return mokubernetes.UpdateK8sService(*r.Data)
}

func K8sUpdatePod(r K8sUpdatePodRequest, c *websocket.Conn) interface{} {
	return mokubernetes.UpdateK8sPod(*r.Data)
}

func K8sUpdateIngress(r K8sUpdateIngressRequest, c *websocket.Conn) interface{} {
	return mokubernetes.UpdateK8sIngress(*r.Data)
}

func K8sUpdateConfigMap(r K8sUpdateConfigmapRequest, c *websocket.Conn) interface{} {
	return mokubernetes.UpdateK8sConfigMap(*r.Data)
}

func K8sUpdateSecret(r K8sUpdateSecretRequest, c *websocket.Conn) interface{} {
	return mokubernetes.UpdateK8sSecret(*r.Data)
}

func K8sUpdateDaemonSet(r K8sUpdateDaemonSetRequest, c *websocket.Conn) interface{} {
	return mokubernetes.UpdateK8sDaemonSet(*r.Data)
}

func K8sUpdateStatefulset(r K8sUpdateStatefulSetRequest, c *websocket.Conn) interface{} {
	return mokubernetes.UpdateK8sStatefulset(*r.Data)
}

func K8sUpdateJob(r K8sUpdateJobRequest, c *websocket.Conn) interface{} {
	return mokubernetes.UpdateK8sJob(*r.Data)
}

func K8sUpdateCronJob(r K8sUpdateCronJobRequest, c *websocket.Conn) interface{} {
	return mokubernetes.UpdateK8sCronJob(*r.Data)
}

func K8sUpdateReplicaSet(r K8sUpdateReplicaSetRequest, c *websocket.Conn) interface{} {
	return mokubernetes.UpdateK8sReplicaset(*r.Data)
}

func K8sDeleteNamespace(r K8sDeleteNamespaceRequest, c *websocket.Conn) interface{} {
	return mokubernetes.DeleteK8sNamespace(*r.Data)
}

func K8sDeleteDeployment(r K8sDeleteDeploymentRequest, c *websocket.Conn) interface{} {
	return mokubernetes.DeleteK8sDeployment(*r.Data)
}

func K8sDeleteService(r K8sDeleteServiceRequest, c *websocket.Conn) interface{} {
	return mokubernetes.DeleteK8sService(*r.Data)
}

func K8sDeletePod(r K8sDeletePodRequest, c *websocket.Conn) interface{} {
	return mokubernetes.DeleteK8sPod(*r.Data)
}

func K8sDeleteIngress(r K8sDeleteIngressRequest, c *websocket.Conn) interface{} {
	return mokubernetes.DeleteK8sIngress(*r.Data)
}

func K8sDeleteConfigMap(r K8sDeleteConfigmapRequest, c *websocket.Conn) interface{} {
	return mokubernetes.DeleteK8sConfigmap(*r.Data)
}

func K8sDeleteSecret(r K8sDeleteSecretRequest, c *websocket.Conn) interface{} {
	return mokubernetes.DeleteK8sSecret(*r.Data)
}

func K8sDeleteDaemonSet(r K8sDeleteDaemonsetRequest, c *websocket.Conn) interface{} {
	return mokubernetes.DeleteK8sDaemonSet(*r.Data)
}

func K8sDeleteStatefulset(r K8sDeleteStatefulsetRequest, c *websocket.Conn) interface{} {
	return mokubernetes.DeleteK8sStatefulset(*r.Data)
}

func K8sDeleteJob(r K8sDeleteJobRequest, c *websocket.Conn) interface{} {
	return mokubernetes.DeleteK8sJob(*r.Data)
}

func K8sDeleteCronJob(r K8sDeleteCronjobRequest, c *websocket.Conn) interface{} {
	return mokubernetes.DeleteK8sCronJob(*r.Data)
}

func K8sDeleteReplicaSet(r K8sDeleteReplicasetRequest, c *websocket.Conn) interface{} {
	return mokubernetes.DeleteK8sReplicaset(*r.Data)
}
