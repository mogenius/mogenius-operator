package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UpgradeMyself(job *structs.Job, command string, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Upgrade mogenius platform ...", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start("Upgrade mogenius platform ...", c)

		kubeProvider := NewKubeProvider()
		jobClient := kubeProvider.ClientSet.BatchV1().Jobs(NAMESPACE)
		configmapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(NAMESPACE)

		configmap := utils.InitUpgradeConfigMap()
		configmap.Namespace = NAMESPACE
		configmap.Data["values.command"] = command

		job := utils.InitUpgradeJob()
		job.Namespace = NAMESPACE
		job.Name = fmt.Sprintf("%s-%s", job.Name, uuid.New().String())

		createOptions := metav1.CreateOptions{
			FieldManager: DEPLOYMENTNAME,
		}

		// CONFIGMAP
		_, err := configmapClient.Get(context.TODO(), configmap.Name, metav1.GetOptions{})
		if err != nil {
			// CREATE
			_, err = configmapClient.Create(context.TODO(), &configmap, createOptions)
			if err != nil {
				cmd.Fail(fmt.Sprintf("UpgradeMyself (configmap) ERROR: %s", err.Error()), c)
				return
			}
		} else {
			// UPDATE
			_, err = configmapClient.Update(context.TODO(), &configmap, metav1.UpdateOptions{})
			if err != nil {
				cmd.Fail(fmt.Sprintf("UpgradeMyself (update_configmap) ERROR: %s", err.Error()), c)
				return
			}
		}

		// JOB
		_, err = jobClient.Get(context.TODO(), job.Name, metav1.GetOptions{})
		if err != nil {
			// CREATE
			_, err = jobClient.Create(context.TODO(), &job, createOptions)
			if err != nil {
				cmd.Fail(fmt.Sprintf("UpgradeMyself (job) ERROR: %s", err.Error()), c)
				return
			}
		} else {
			// UPDATE
			_, err = jobClient.Update(context.TODO(), &job, metav1.UpdateOptions{})
			if err != nil {
				cmd.Fail(fmt.Sprintf("UpgradeMyself (update_job) ERROR: %s", err.Error()), c)
				return
			}
		}
		cmd.Success("Upgraded platform successfully.", c)
	}(cmd, wg)
	return cmd
}
