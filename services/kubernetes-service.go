package services

import (
	mokubernetes "mogenius-k8s-manager/kubernetes"

	"github.com/gorilla/websocket"
)

func K8sUpdateDeployment(r K8sUpdateDeploymentRequest, c *websocket.Conn) interface{} {
	// var wg sync.WaitGroup
	// job := structs.CreateJob(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), r.NamespaceId, nil, nil, c)
	// job.Start(c)
	// //job.AddCmd(structs.CreateCommand(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), &job, c))
	// wg.Wait()
	// job.Finish(c)
	return mokubernetes.UpdateK8sDeployment(*r.Data)
}

func K8sUpdateService(r K8sUpdateServiceRequest, c *websocket.Conn) interface{} {
	// var wg sync.WaitGroup
	// job := structs.CreateJob(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), r.NamespaceId, nil, nil, c)
	// job.Start(c)
	// //job.AddCmd(structs.CreateCommand(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), &job, c))
	// wg.Wait()
	// job.Finish(c)
	return mokubernetes.UpdateK8sService(*r.Data)
}

func K8sUpdatePod(r K8sUpdatePodRequest, c *websocket.Conn) interface{} {
	// var wg sync.WaitGroup
	// job := structs.CreateJob(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), r.NamespaceId, nil, nil, c)
	// job.Start(c)
	// //job.AddCmd(structs.CreateCommand(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), &job, c))
	// wg.Wait()
	// job.Finish(c)
	return mokubernetes.UpdateK8sPod(*r.Data)
}

func K8sUpdateIngress(r K8sUpdateIngressRequest, c *websocket.Conn) interface{} {
	// var wg sync.WaitGroup
	// job := structs.CreateJob(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), r.NamespaceId, nil, nil, c)
	// job.Start(c)
	// //job.AddCmd(structs.CreateCommand(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), &job, c))
	// wg.Wait()
	// job.Finish(c)
	return mokubernetes.UpdateK8sIngress(*r.Data)
}

func K8sUpdateConfigMap(r K8sUpdateConfigmapRequest, c *websocket.Conn) interface{} {
	// var wg sync.WaitGroup
	// job := structs.CreateJob(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), r.NamespaceId, nil, nil, c)
	// job.Start(c)
	// //job.AddCmd(structs.CreateCommand(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), &job, c))
	// wg.Wait()
	// job.Finish(c)
	return mokubernetes.UpdateK8sConfigMap(*r.Data)
}

func K8sUpdateSecret(r K8sUpdateSecretRequest, c *websocket.Conn) interface{} {
	// var wg sync.WaitGroup
	// job := structs.CreateJob(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), r.NamespaceId, nil, nil, c)
	// job.Start(c)
	// //job.AddCmd(structs.CreateCommand(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), &job, c))
	// wg.Wait()
	// job.Finish(c)
	return mokubernetes.UpdateK8sSecret(*r.Data)
}

func K8sUpdateDaemonSet(r K8sUpdateDaemonSetRequest, c *websocket.Conn) interface{} {
	// var wg sync.WaitGroup
	// job := structs.CreateJob(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), r.NamespaceId, nil, nil, c)
	// job.Start(c)
	// //job.AddCmd(structs.CreateCommand(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), &job, c))
	// wg.Wait()
	// job.Finish(c)
	return mokubernetes.UpdateK8sDaemonSet(*r.Data)
}

func K8sUpdateStatefulset(r K8sUpdateStatefulSetRequest, c *websocket.Conn) interface{} {
	// var wg sync.WaitGroup
	// job := structs.CreateJob(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), r.NamespaceId, nil, nil, c)
	// job.Start(c)
	// //job.AddCmd(structs.CreateCommand(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), &job, c))
	// wg.Wait()
	// job.Finish(c)
	return mokubernetes.UpdateK8sStatefulset(*r.Data)
}

func K8sUpdateJob(r K8sUpdateJobRequest, c *websocket.Conn) interface{} {
	// var wg sync.WaitGroup
	// job := structs.CreateJob(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), r.NamespaceId, nil, nil, c)
	// job.Start(c)
	// //job.AddCmd(structs.CreateCommand(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), &job, c))
	// wg.Wait()
	// job.Finish(c)
	return mokubernetes.UpdateK8sJob(*r.Data)
}

func K8sUpdateCronJob(r K8sUpdateCronJobRequest, c *websocket.Conn) interface{} {
	// var wg sync.WaitGroup
	// job := structs.CreateJob(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), r.NamespaceId, nil, nil, c)
	// job.Start(c)
	// //job.AddCmd(structs.CreateCommand(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), &job, c))
	// wg.Wait()
	// job.Finish(c)
	return mokubernetes.UpdateK8sCronJob(*r.Data)
}

func K8sUpdateReplicaSet(r K8sUpdateReplicaSetRequest, c *websocket.Conn) interface{} {
	// var wg sync.WaitGroup
	// job := structs.CreateJob(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), r.NamespaceId, nil, nil, c)
	// job.Start(c)
	// //job.AddCmd(structs.CreateCommand(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), &job, c))
	// wg.Wait()
	// job.Finish(c)
	return mokubernetes.UpdateK8sReplicaset(*r.Data)
}
