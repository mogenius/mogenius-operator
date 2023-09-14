package services

import (
	punq "github.com/mogenius/punq/kubernetes"
)

func K8sUpdateDeployment(r K8sUpdateDeploymentRequest) interface{} {
	return punq.UpdateK8sDeployment(*r.Data, nil)
}

func K8sUpdateService(r K8sUpdateServiceRequest) interface{} {
	return punq.UpdateK8sService(*r.Data, nil)
}

func K8sUpdatePod(r K8sUpdatePodRequest) interface{} {
	return punq.UpdateK8sPod(*r.Data, nil)
}

func K8sUpdateIngress(r K8sUpdateIngressRequest) interface{} {
	return punq.UpdateK8sIngress(*r.Data, nil)
}

func K8sUpdateConfigMap(r K8sUpdateConfigmapRequest) interface{} {
	return punq.UpdateK8sConfigMap(*r.Data, nil)
}

func K8sUpdateSecret(r K8sUpdateSecretRequest) interface{} {
	return punq.UpdateK8sSecret(*r.Data, nil)
}

func K8sUpdateDaemonSet(r K8sUpdateDaemonSetRequest) interface{} {
	return punq.UpdateK8sDaemonSet(*r.Data, nil)
}

func K8sUpdateStatefulset(r K8sUpdateStatefulSetRequest) interface{} {
	return punq.UpdateK8sStatefulset(*r.Data, nil)
}

func K8sUpdateJob(r K8sUpdateJobRequest) interface{} {
	return punq.UpdateK8sJob(*r.Data, nil)
}

func K8sUpdateCronJob(r K8sUpdateCronJobRequest) interface{} {
	return punq.UpdateK8sCronJob(*r.Data, nil)
}

func K8sUpdateReplicaSet(r K8sUpdateReplicaSetRequest) interface{} {
	return punq.UpdateK8sReplicaset(*r.Data, nil)
}

func K8sDeleteNamespace(r K8sDeleteNamespaceRequest) interface{} {
	return punq.DeleteK8sNamespace(*r.Data, nil)
}

func K8sDeleteDeployment(r K8sDeleteDeploymentRequest) interface{} {
	return punq.DeleteK8sDeployment(*r.Data, nil)
}

func K8sDeleteService(r K8sDeleteServiceRequest) interface{} {
	return punq.DeleteK8sService(*r.Data, nil)
}

func K8sDeletePod(r K8sDeletePodRequest) interface{} {
	return punq.DeleteK8sPod(*r.Data, nil)
}

func K8sDeleteIngress(r K8sDeleteIngressRequest) interface{} {
	return punq.DeleteK8sIngress(*r.Data, nil)
}

func K8sDeleteConfigMap(r K8sDeleteConfigmapRequest) interface{} {
	return punq.DeleteK8sConfigmap(*r.Data, nil)
}

func K8sDeleteSecret(r K8sDeleteSecretRequest) interface{} {
	return punq.DeleteK8sSecret(*r.Data, nil)
}

func K8sDeleteDaemonSet(r K8sDeleteDaemonsetRequest) interface{} {
	return punq.DeleteK8sDaemonSet(*r.Data, nil)
}

func K8sDeleteStatefulset(r K8sDeleteStatefulsetRequest) interface{} {
	return punq.DeleteK8sStatefulset(*r.Data, nil)
}

func K8sDeleteJob(r K8sDeleteJobRequest) interface{} {
	return punq.DeleteK8sJob(*r.Data, nil)
}

func K8sDeleteCronJob(r K8sDeleteCronjobRequest) interface{} {
	return punq.DeleteK8sCronJob(*r.Data, nil)
}

func K8sDeleteReplicaSet(r K8sDeleteReplicasetRequest) interface{} {
	return punq.DeleteK8sReplicaset(*r.Data, nil)
}
