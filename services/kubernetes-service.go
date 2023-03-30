package services

import (
	"fmt"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"sync"

	"github.com/gorilla/websocket"
)

func K8sUpdateDeployment(r K8sUpdateDeploymentRequest, c *websocket.Conn) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob(fmt.Sprintf("Update %s %s/%s", r.Data.Kind, r.Data.Namespace, r.Data.Name), r.NamespaceId, nil, nil, c)
	job.Start(c)
	job.AddCmd(mokubernetes.UpdateK8sDeployment(&job, *r.Data, c, &wg))
	wg.Wait()
	job.Finish(c)
	return job
}
