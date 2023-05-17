package services

import (
	mokubernetes "mogenius-k8s-manager/kubernetes"
)

func K8sUpdateDeployment(r K8sUpdateDeploymentRequest) interface{} {
	return mokubernetes.UpdateK8sDeployment(*r.Data)
}

func K8sUpdateService(r K8sUpdateServiceRequest) interface{} {
	return mokubernetes.UpdateK8sService(*r.Data)
}

func K8sUpdatePod(r K8sUpdatePodRequest) interface{} {
	return mokubernetes.UpdateK8sPod(*r.Data)
}

func K8sUpdateIngress(r K8sUpdateIngressRequest) interface{} {
	return mokubernetes.UpdateK8sIngress(*r.Data)
}

func K8sUpdateConfigMap(r K8sUpdateConfigmapRequest) interface{} {
	return mokubernetes.UpdateK8sConfigMap(*r.Data)
}

func K8sUpdateSecret(r K8sUpdateSecretRequest) interface{} {
	return mokubernetes.UpdateK8sSecret(*r.Data)
}

func K8sUpdateDaemonSet(r K8sUpdateDaemonSetRequest) interface{} {
	return mokubernetes.UpdateK8sDaemonSet(*r.Data)
}

func K8sUpdateStatefulset(r K8sUpdateStatefulSetRequest) interface{} {
	return mokubernetes.UpdateK8sStatefulset(*r.Data)
}

func K8sUpdateJob(r K8sUpdateJobRequest) interface{} {
	return mokubernetes.UpdateK8sJob(*r.Data)
}

func K8sUpdateCronJob(r K8sUpdateCronJobRequest) interface{} {
	return mokubernetes.UpdateK8sCronJob(*r.Data)
}

func K8sUpdateReplicaSet(r K8sUpdateReplicaSetRequest) interface{} {
	return mokubernetes.UpdateK8sReplicaset(*r.Data)
}

func K8sDeleteNamespace(r K8sDeleteNamespaceRequest) interface{} {
	return mokubernetes.DeleteK8sNamespace(*r.Data)
}

func K8sDeleteDeployment(r K8sDeleteDeploymentRequest) interface{} {
	return mokubernetes.DeleteK8sDeployment(*r.Data)
}

func K8sDeleteService(r K8sDeleteServiceRequest) interface{} {
	return mokubernetes.DeleteK8sService(*r.Data)
}

func K8sDeletePod(r K8sDeletePodRequest) interface{} {
	return mokubernetes.DeleteK8sPod(*r.Data)
}

func K8sDeleteIngress(r K8sDeleteIngressRequest) interface{} {
	return mokubernetes.DeleteK8sIngress(*r.Data)
}

func K8sDeleteConfigMap(r K8sDeleteConfigmapRequest) interface{} {
	return mokubernetes.DeleteK8sConfigmap(*r.Data)
}

func K8sDeleteSecret(r K8sDeleteSecretRequest) interface{} {
	return mokubernetes.DeleteK8sSecret(*r.Data)
}

func K8sDeleteDaemonSet(r K8sDeleteDaemonsetRequest) interface{} {
	return mokubernetes.DeleteK8sDaemonSet(*r.Data)
}

func K8sDeleteStatefulset(r K8sDeleteStatefulsetRequest) interface{} {
	return mokubernetes.DeleteK8sStatefulset(*r.Data)
}

func K8sDeleteJob(r K8sDeleteJobRequest) interface{} {
	return mokubernetes.DeleteK8sJob(*r.Data)
}

func K8sDeleteCronJob(r K8sDeleteCronjobRequest) interface{} {
	return mokubernetes.DeleteK8sCronJob(*r.Data)
}

func K8sDeleteReplicaSet(r K8sDeleteReplicasetRequest) interface{} {
	return mokubernetes.DeleteK8sReplicaset(*r.Data)
}
