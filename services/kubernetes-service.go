package services

import (
	punq "github.com/mogenius/punq/kubernetes"
)

func K8sUpdateNamespace(r K8sUpdateNamespaceRequest) interface{} {
	return punq.UpdateK8sNamespace(*r.Data, nil)
}

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

func K8sDeleteNamespace(r K8sDeleteResourceRequest) interface{} {
	return punq.DeleteK8sNamespaceBy(r.Namespace, nil)
}

func K8sDeleteDeployment(r K8sDeleteResourceRequest) interface{} {
	return punq.DeleteK8sDeploymentBy(r.Namespace, r.Name, nil)
}

func K8sDeleteService(r K8sDeleteResourceRequest) interface{} {
	return punq.DeleteK8sServiceBy(r.Namespace, r.Name, nil)
}

func K8sDeletePod(r K8sDeleteResourceRequest) interface{} {
	return punq.DeleteK8sPodBy(r.Namespace, r.Name, nil)
}

func K8sDeleteIngress(r K8sDeleteResourceRequest) interface{} {
	return punq.DeleteK8sIngressBy(r.Namespace, r.Name, nil)
}

func K8sDeleteConfigMap(r K8sDeleteResourceRequest) interface{} {
	return punq.DeleteK8sConfigmapBy(r.Namespace, r.Name, nil)
}

func K8sDeleteSecret(r K8sDeleteResourceRequest) interface{} {
	return punq.DeleteK8sSecretBy(r.Namespace, r.Name, nil)
}

func K8sDeleteDaemonSet(r K8sDeleteResourceRequest) interface{} {
	return punq.DeleteK8sDaemonSetBy(r.Namespace, r.Name, nil)
}

func K8sDeleteStatefulset(r K8sDeleteResourceRequest) interface{} {
	return punq.DeleteK8sStatefulsetBy(r.Namespace, r.Name, nil)
}

func K8sDeleteJob(r K8sDeleteResourceRequest) interface{} {
	return punq.DeleteK8sJobBy(r.Namespace, r.Name, nil)
}

func K8sDeleteCronJob(r K8sDeleteResourceRequest) interface{} {
	return punq.DeleteK8sCronJobBy(r.Namespace, r.Name, nil)
}

func K8sDeleteReplicaSet(r K8sDeleteResourceRequest) interface{} {
	return punq.DeleteK8sReplicasetBy(r.Namespace, r.Name, nil)
}

func K8sDeleteHpa(r K8sDeleteResourceRequest) interface{} {
	return punq.DeleteK8sHpaBy(r.Namespace, r.Name, nil)
}
