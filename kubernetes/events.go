package kubernetes

import (
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/db"
	"mogenius-k8s-manager/dtos"
	iacmanager "mogenius-k8s-manager/iac-manager"
	"mogenius-k8s-manager/utils"
	"strings"

	"mogenius-k8s-manager/structs"
	"time"

	punq "github.com/mogenius/punq/kubernetes"
	v1Core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/client-go/util/retry"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"
)

const RETRYTIMEOUT time.Duration = 3
const CONCURRENTCONNECTIONS = 1

var EventChannels = make(map[string]chan string)

func EventWatcher() {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		log.Fatalf("Error creating provider for watcher. Cannot continue because it is vital: %s", err.Error())
		return
	}

	// Retry watching events with exponential backoff in case of failures
	err = retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}, apierrors.IsServiceUnavailable, func() error {
		return watchEvents(provider)
	})
	if err != nil {
		log.Fatalf("Error watching events: %s", err.Error())
	}

	// Wait forever
	select {}
}

func ResourceWatcher() {
	// if !iacmanager.ShouldWatchResources() {
	// 	log.Warn("Nor Pull nor Push enabled. Skip watching resources.")
	// 	return
	// }
	// if resourceWatcherRunning {
	// 	log.Warn("Resource watcher already running.")
	// 	return
	// }

	log.Infof("Starting watchers for resources: %s", strings.Join(utils.CONFIG.Iac.SyncWorkloads, ", "))
	for _, workload := range utils.CONFIG.Iac.SyncWorkloads {
		switch workload {
		case dtos.KindConfigMaps:
			go WatchConfigmaps()
		case dtos.KindDeployments:
			go WatchDeployments()
		case dtos.KindPods:
			go WatchPods()
		case dtos.KindIngresses:
			go WatchIngresses()
		case dtos.KindSecrets:
			go WatchSecrets()
		case dtos.KindServices:
			go WatchServices()
		case dtos.KindNamespaces:
			go WatchNamespaces()
		case dtos.KindNetworkPolicies:
			go WatchNetworkPolicies()
		case dtos.KindJobs:
			go WatchJobs()
		case dtos.KindCronJobs:
			go WatchCronJobs()
		case dtos.KindDaemonSets:
			go WatchDaemonSets()
		case dtos.KindStatefulSets:
			go WatchStatefulSets()
		case dtos.KindHorizontalPodAutoscalers:
			go WatchHpas()
		default:
			log.Fatalf("🚫 Unknown resource type: %s", workload)
		}
		log.Infof("Started watching %s 🚀.", workload)
	}
}

func InitAllWorkloads() {
	if !iacmanager.ShouldWatchResources() {
		return
	}
	for _, workload := range utils.CONFIG.Iac.SyncWorkloads {
		switch workload {
		case dtos.KindConfigMaps:
			ressources := punq.AllConfigmaps("", nil)
			for _, res := range ressources {
				iacmanager.WriteResourceYaml(dtos.KindConfigMaps, res.Namespace, res.Name, res)
			}
		case dtos.KindDeployments:
			ressources := punq.AllDeployments("", nil)
			for _, res := range ressources {
				iacmanager.WriteResourceYaml(dtos.KindDeployments, res.Namespace, res.Name, res)
			}
		case dtos.KindPods:
			ressources := punq.AllPods("", nil)
			for _, res := range ressources {
				iacmanager.WriteResourceYaml(dtos.KindPods, res.Namespace, res.Name, res)
			}
		case dtos.KindIngresses:
			ressources := punq.AllIngresses("", nil)
			for _, res := range ressources {
				iacmanager.WriteResourceYaml(dtos.KindIngresses, res.Namespace, res.Name, res)
			}
		case dtos.KindSecrets:
			ressources := punq.AllSecrets("", nil)
			for _, res := range ressources {
				iacmanager.WriteResourceYaml(dtos.KindSecrets, res.Namespace, res.Name, res)
			}
		case dtos.KindServices:
			ressources := punq.AllServices("", nil)
			for _, res := range ressources {
				iacmanager.WriteResourceYaml(dtos.KindServices, res.Namespace, res.Name, res)
			}
		case dtos.KindNamespaces:
			ressources := punq.ListAllNamespace(nil)
			for _, res := range ressources {
				iacmanager.WriteResourceYaml(dtos.KindNamespaces, res.Namespace, res.Name, res)
			}
		case dtos.KindNetworkPolicies:
			ressources := punq.AllNetworkPolicies("", nil)
			for _, res := range ressources {
				iacmanager.WriteResourceYaml(dtos.KindNetworkPolicies, res.Namespace, res.Name, res)
			}
		case dtos.KindJobs:
			ressources := punq.AllJobs("", nil)
			for _, res := range ressources {
				iacmanager.WriteResourceYaml(dtos.KindJobs, res.Namespace, res.Name, res)
			}
		case dtos.KindCronJobs:
			ressources := punq.AllCronjobs("", nil)
			for _, res := range ressources {
				iacmanager.WriteResourceYaml(dtos.KindCronJobs, res.Namespace, res.Name, res)
			}
		case dtos.KindDaemonSets:
			ressources := punq.AllDaemonsets("", nil)
			for _, res := range ressources {
				iacmanager.WriteResourceYaml(dtos.KindDaemonSets, res.Namespace, res.Name, res)
			}
		case dtos.KindStatefulSets:
			ressources := punq.AllStatefulSets("", nil)
			for _, res := range ressources {
				iacmanager.WriteResourceYaml(dtos.KindStatefulSets, res.Namespace, res.Name, res)
			}
		case dtos.KindHorizontalPodAutoscalers:
			ressources := punq.AllHpas("", nil)
			for _, res := range ressources {
				iacmanager.WriteResourceYaml(dtos.KindHorizontalPodAutoscalers, res.Namespace, res.Name, res)
			}
		default:
			log.Fatalf("🚫 Unknown resource type: %s", workload)
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
				log.Errorf("Error adding event to db: %s", err.Error())
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
		log.Errorf("malformed event received")
	}
}

func watchEvents(provider *punq.KubeProvider) error {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			event := obj.(*v1Core.Event)
			processEvent(event)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			event := newObj.(*v1Core.Event)
			processEvent(event)
		},
		DeleteFunc: func(obj interface{}) {
			event := obj.(*v1Core.Event)
			processEvent(event)
		},
	}
	listWatch := cache.NewListWatchFromClient(
		provider.ClientSet.CoreV1().RESTClient(),
		"events",
		v1Core.NamespaceAll,
		fields.Nothing(),
	)
	eventInformer := cache.NewSharedInformer(listWatch, &v1Core.Event{}, 0)
	eventInformer.AddEventHandler(handler)

	stopCh := make(chan struct{})
	go eventInformer.Run(stopCh)

	// Wait for the informer to sync and start processing events
	if !cache.WaitForCacheSync(stopCh, eventInformer.HasSynced) {
		return fmt.Errorf("failed to sync cache")
	}

	// This loop will keep the function alive as long as the stopCh is not closed
	for {
		select {
		case <-stopCh:
			// stopCh closed, return from the function
			return nil
		case <-time.After(30 * time.Second):
			// This is to avoid a tight loop in case stopCh is never closed.
			// You can adjust the time as per your needs.
		}
	}
}
