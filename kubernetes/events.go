package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/db"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/store"
	"mogenius-k8s-manager/utils"
	"strings"

	"mogenius-k8s-manager/structs"
	"time"

	punq "github.com/mogenius/punq/kubernetes"
	"github.com/mogenius/punq/logger"
	punqutils "github.com/mogenius/punq/utils"
	v1Core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const RETRYTIMEOUT time.Duration = 3
const CONCURRENTCONNECTIONS = 1

var EventChannels = make(map[string]chan string)

func MapIacSyncWorkloadIntoConfigMap() {
	// init all with false
	utils.IacWorkloadConfigMap = make(map[string]bool)
	for _, kind := range dtos.AvailableSyncWorkloadKinds {
		utils.IacWorkloadConfigMap[kind] = false
	}
	// set to true for the ones we want to watch
	for _, workload := range utils.CONFIG.Iac.SyncWorkloads {
		utils.IacWorkloadConfigMap[strings.TrimSpace(workload)] = true
	}
}

func InitAllWorkloads() {
	MapIacSyncWorkloadIntoConfigMap()

	deployments := punq.AllDeployments("", nil)
	for _, res := range deployments {
		if IacManagerShouldWatchResources() && utils.IacWorkloadConfigMap[dtos.KindDeployments] {
			IacManagerWriteResourceYaml(dtos.KindDeployments, res.Namespace, res.Name, res)
		}

		err := store.GlobalStore.Set(res, "Deployment", res.Namespace, res.Name)
		if err != nil {
			K8sLogger.Error(err)
		}
	}

	replicasets := punq.AllReplicasets("", nil)
	for _, res := range replicasets {
		err := store.GlobalStore.Set(res, "ReplicaSet", res.Namespace, res.Name)
		if err != nil {
			K8sLogger.Error(err)
		}
	}

	cronjobs := punq.AllCronjobs("", nil)
	for _, res := range cronjobs {
		if IacManagerShouldWatchResources() && utils.IacWorkloadConfigMap[dtos.KindCronJobs] {
			IacManagerWriteResourceYaml(dtos.KindCronJobs, res.Namespace, res.Name, res)
		}

		err := store.GlobalStore.Set(res, "CronJob", res.Namespace, res.Name)
		if err != nil {
			K8sLogger.Error(err)
		}
	}

	jobs := punq.AllJobs("", nil)
	for _, res := range jobs {
		if IacManagerShouldWatchResources() && utils.IacWorkloadConfigMap[dtos.KindJobs] {
			IacManagerWriteResourceYaml(dtos.KindJobs, res.Namespace, res.Name, res)
		}

		err := store.GlobalStore.Set(res, "Job", res.Namespace, res.Name)
		if err != nil {
			K8sLogger.Error(err)
		}
	}

	pods := punq.AllPods("", nil)
	for _, res := range pods {
		if IacManagerShouldWatchResources() && utils.IacWorkloadConfigMap[dtos.KindPods] {
			IacManagerWriteResourceYaml(dtos.KindPods, res.Namespace, res.Name, res)
		}

		err := store.GlobalStore.Set(res, "Pod", res.Namespace, res.Name)
		if err != nil {
			K8sLogger.Error(err)
		}
	}

	if !IacManagerShouldWatchResources() {
		return
	}

	for _, workload := range utils.CONFIG.Iac.SyncWorkloads {
		switch strings.TrimSpace(workload) {
		case dtos.KindConfigMaps:
			ressources := punq.AllConfigmaps("", nil)
			for _, res := range ressources {
				IacManagerWriteResourceYaml(dtos.KindConfigMaps, res.Namespace, res.Name, res)
			}
		case dtos.KindDeployments:
			// ressources := punq.AllDeployments("", nil)
			// for _, res := range ressources {
			// 	IacManagerWriteResourceYaml(dtos.KindDeployments, res.Namespace, res.Name, res)
			// }
		case dtos.KindPods:
			// ressources := punq.AllPods("", nil)
			// for _, res := range ressources {
			// 	IacManagerWriteResourceYaml(dtos.KindPods, res.Namespace, res.Name, res)
			// }
		case dtos.KindIngresses:
			ressources := punq.AllIngresses("", nil)
			for _, res := range ressources {
				IacManagerWriteResourceYaml(dtos.KindIngresses, res.Namespace, res.Name, res)
			}
		case dtos.KindSecrets:
			ressources := punq.AllSecrets("", nil)
			for _, res := range ressources {
				IacManagerWriteResourceYaml(dtos.KindSecrets, res.Namespace, res.Name, res)
			}
		case dtos.KindServices:
			ressources := punq.AllServices("", nil)
			for _, res := range ressources {
				IacManagerWriteResourceYaml(dtos.KindServices, res.Namespace, res.Name, res)
			}
		case dtos.KindNamespaces:
			ressources := punq.ListAllNamespace(nil)
			for _, res := range ressources {
				IacManagerWriteResourceYaml(dtos.KindNamespaces, res.Namespace, res.Name, res)
			}
		case dtos.KindNetworkPolicies:
			ressources := punq.AllNetworkPolicies("", nil)
			for _, res := range ressources {
				IacManagerWriteResourceYaml(dtos.KindNetworkPolicies, res.Namespace, res.Name, res)
			}
		case dtos.KindJobs:
			// ressources := punq.AllJobs("", nil)
			// for _, res := range ressources {
			// 	IacManagerWriteResourceYaml(dtos.KindJobs, res.Namespace, res.Name, res)
			// }
		case dtos.KindCronJobs:
			// ressources := punq.AllCronjobs("", nil)
			// for _, res := range ressources {
			// 	IacManagerWriteResourceYaml(dtos.KindCronJobs, res.Namespace, res.Name, res)
			// }
		case dtos.KindDaemonSets:
			ressources := punq.AllDaemonsets("", nil)
			for _, res := range ressources {
				IacManagerWriteResourceYaml(dtos.KindDaemonSets, res.Namespace, res.Name, res)
			}
		case dtos.KindStatefulSets:
			ressources := punq.AllStatefulSets("", nil)
			for _, res := range ressources {
				IacManagerWriteResourceYaml(dtos.KindStatefulSets, res.Namespace, res.Name, res)
			}
		case dtos.KindHorizontalPodAutoscalers:
			ressources := punq.AllHpas("", nil)
			for _, res := range ressources {
				IacManagerWriteResourceYaml(dtos.KindHorizontalPodAutoscalers, res.Namespace, res.Name, res)
			}
		default:
			K8sLogger.Fatalf("🚫 Unknown resource type: %s", workload)
		}
	}
}

func processEvent(event *v1Core.Event) {
	if event != nil {
		eventDto := dtos.CreateEvent(string(event.Type), event)
		datagram := structs.CreateDatagramFrom("KubernetesEvent", eventDto)
		message := event.Message
		kind := event.InvolvedObject.Kind
		reason := event.Reason
		count := event.Count
		structs.EventServerSendData(datagram, kind, reason, message, count)

		// deployment events
		ignoreKind := []string{"CertificateRequest", "Certificate"}
		ignoreNamespaces := []string{"kube-system", "kube-public", "default", "mogenius"}
		if event.InvolvedObject.Kind == "Pod" &&
			!utils.ContainsString(ignoreNamespaces, event.InvolvedObject.Namespace) &&
			!utils.ContainsString(ignoreKind, event.InvolvedObject.Kind) {

			//personJSON, err := json.Marshal(event)
			//if err == nil {
			//	fmt.Println("event as JSON:", string(personJSON))
			//}
			parts := strings.Split(event.InvolvedObject.Name, "-")

			if len(parts) >= 2 {
				parts = parts[:len(parts)-2]
			}
			controllerName := strings.Join(parts, "-")
			err := db.AddPodEvent(event.InvolvedObject.Namespace, controllerName, event, 150)
			if err != nil {
				K8sLogger.Errorf("Error adding event to db: %s", err.Error())
			}

			key := fmt.Sprintf("%s-%s", event.InvolvedObject.Namespace, controllerName)
			ch, exists := EventChannels[key]
			if exists {
				var events []*v1Core.Event
				events = append(events, event)
				updatedData, err := json.Marshal(events)
				if err == nil {
					ch <- string(updatedData)
				}
			}

		}
	} else {
		K8sLogger.Errorf("malformed event received")
	}
}

var allEventsForNamespaceDebounce = utils.NewDebounce("allEventsForNamespaceDebounce", 1000*time.Millisecond, 300*time.Millisecond)

func AllEventsForNamespace(namespaceName string) []v1Core.Event {
	result, _ := allEventsForNamespaceDebounce.CallFn(namespaceName, func() (interface{}, error) {
		return AllEventsForNamespace2(namespaceName), nil
	})
	return result.([]v1Core.Event)
}

func AllEventsForNamespace2(namespaceName string) []v1Core.Event {
	result := []v1Core.Event{}

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return result
	}
	eventList, err := provider.ClientSet.CoreV1().Events(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllEvents ERROR: %s", err.Error())
		return result
	}

	for _, event := range eventList.Items {
		if !punqutils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, event.ObjectMeta.Namespace) {
			event.Kind = "Event"
			event.APIVersion = "v1"
			result = append(result, event)
		}
	}
	return result
}
