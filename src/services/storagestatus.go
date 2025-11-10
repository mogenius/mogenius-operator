package services

import (
	"context"
	"errors"
	"fmt"
	"mogenius-operator/src/utils"
	"sort"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// milliseconds
	VolumeStatusTimeout = 250
)

type VolumeAPIObject int

const (
	VolumeTypePersistentVolume VolumeAPIObject = iota
	VolumeTypePersistentVolumeClaim
)

func (s VolumeAPIObject) String() string {
	return [...]string{"PersistentVolume", "PersistentVolumeClaim"}[s]
}

func StorageAPIObjectFromString(s string) VolumeAPIObject {
	switch s {
	case VolumeTypePersistentVolume.String():
		return VolumeTypePersistentVolume
	case VolumeTypePersistentVolumeClaim.String():
		return VolumeTypePersistentVolumeClaim
	default:
		return -1
	}
}

type VolumeStatusType string

const (
	VolumeStatusTypePending VolumeStatusType = "PENDING"
	VolumeStatusTypeBound   VolumeStatusType = "BOUND"
	VolumeStatusTypeError   VolumeStatusType = "ERROR"
	VolumeStatusTypeWarning VolumeStatusType = "WARNING"
)

type VolumeStatusMessageType string

const (
	VolumeStatusMessageTypeInfo    VolumeStatusMessageType = "INFO"
	VolumeStatusMessageTypeSuccess VolumeStatusMessageType = "SUCCESS"
	VolumeStatusMessageTypeError   VolumeStatusMessageType = "ERROR"
	VolumeStatusMessageTypeWarning VolumeStatusMessageType = "WARNING"
)

type VolumeProvisioningType int

const (
	VolumeProvisioningTypeDynamic VolumeProvisioningType = iota
	VolumeProvisioningTypeManual
)

func (p VolumeProvisioningType) String() string {
	return [...]string{"Dynamic", "Manual"}[p]
}

type VolumeStatus struct {
	client *kubernetes.Clientset
	//
	PersistentVolume            *v1.PersistentVolume      `json:"persistentVolume,omitempty"`
	PersistentVolumeClaim       *v1.PersistentVolumeClaim `json:"persistentVolumeClaim,omitempty"`
	UsedByPods                  []v1.Pod                  `json:"usedByPods,omitempty"`
	Namespace                   string                    `json:"namespace"`
	Provisioning                string                    `json:"provisioning"`
	PersistentVolumeEvents      []v1.Event                `json:"persistentVolumeEvents,omitempty"`
	PersistentVolumeClaimEvents []v1.Event                `json:"persistentVolumeClaimEvents,omitempty"`
	//
	persistentVolumeName      string
	persistentVolumeClaimName string
}

type VolumeStatusMessage struct {
	Type    VolumeStatusMessageType `json:"type"`
	Message string                  `json:"message"`
}

var statusMogeniusNfsDebounce = utils.NewDebounce("statusMogeniusNfsDebounce", 1000*time.Millisecond, 300*time.Millisecond)

func StatusMogeniusNfs(r NfsStatusRequest) NfsStatusResponse {
	key := fmt.Sprintf("%s-%s-%s", r.Name, r.Namespace, r.StorageAPIObject)
	result, _ := statusMogeniusNfsDebounce.CallFn(key, func() (any, error) {
		return StatusMogeniusNfs2(r), nil
	})
	return result.(NfsStatusResponse)
}

func StatusMogeniusNfs2(r NfsStatusRequest) NfsStatusResponse {
	prefix := fmt.Sprintf("%s-", utils.NFS_POD_PREFIX)
	// normalize name, convert no prefixed 'nfs-server-pod-' to prefixed and vice versa
	nonPrefixName := strings.TrimPrefix(r.Name, prefix)
	prefixName := prefix + nonPrefixName

	serviceLogger.Debug("Storage status", "storageAPIObject", r.StorageAPIObject, "nonPrefixName", nonPrefixName, "prefixName", prefixName)

	nfsStatusResponse := NfsStatusResponse{
		VolumeName: nonPrefixName,
		TotalBytes: 0,
		FreeBytes:  0,
		UsedBytes:  0,
	}

	if r.Namespace != "" {
		nfsStatusResponse.NamespaceName = r.Namespace
	}

	storageStatus := VolumeStatus{}
	storageStatus.SetClient(clientProvider.K8sClientSet())

	if StorageAPIObjectFromString(r.StorageAPIObject) == VolumeTypePersistentVolume {
		if _, err := storageStatus.GetByPVName(prefixName); err != nil {
			nfsStatusResponse.ProcessNfsStatusResponse(&storageStatus, err)
			return nfsStatusResponse
		}
	} else if StorageAPIObjectFromString(r.StorageAPIObject) == VolumeTypePersistentVolumeClaim {
		if _, err := storageStatus.GetByPVCName(r.Namespace, prefixName); err != nil {
			nfsStatusResponse.ProcessNfsStatusResponse(&storageStatus, err)
			return nfsStatusResponse
		}
	} else {
		nfsStatusResponse.ProcessNfsStatusResponse(nil, errors.New("invalid StorageAPIObject"))
		return nfsStatusResponse
	}

	nfsStatusResponse.ProcessNfsStatusResponse(&storageStatus, nil)
	return nfsStatusResponse
}

func (v *NfsStatusResponse) ProcessNfsStatusResponse(s *VolumeStatus, err error) {
	if s == nil && err != nil {
		v.Status = VolumeStatusTypeError
		v.Messages = append(v.Messages, VolumeStatusMessage{Type: VolumeStatusMessageTypeError, Message: err.Error()})
		return
	}

	if s != nil {
		// Begin status logic

		// check pv and pvc
		bounded := false
		if s.PersistentVolumeClaim != nil && s.PersistentVolume != nil {

			v.NamespaceName = s.PersistentVolumeClaim.Namespace

			// pv phase 'failed or pvc phase 'lost'
			notOk := false
			notOk = notOk || s.PersistentVolume.Status.Phase == v1.VolumeFailed
			notOk = notOk || s.PersistentVolumeClaim.Status.Phase == v1.ClaimLost

			if notOk {
				v.Status = VolumeStatusTypeError
				v.Messages = append(v.Messages, VolumeStatusMessage{Type: VolumeStatusMessageTypeError, Message: "Volume phase failed or claim phase lost"})
				v.Messages = append(v.Messages, s.messages()...)
				return
			}

			bounded = true
			bounded = bounded && s.PersistentVolume.Status.Phase == v1.VolumeBound
			bounded = bounded && s.PersistentVolumeClaim.Status.Phase == v1.ClaimBound
		}

		// check nfs-pod

		// use index to remove our helper nfs-pod from usedPods array
		index := -1
		indexToRemove := -1

		// iterate over usedByPods and check if they are running and only if pod name start with pvc.Name
		boundedPodRunning := false
		for _, pod := range s.UsedByPods {
			index++

			if s.PersistentVolumeClaim != nil && strings.HasPrefix(pod.ObjectMeta.Name, s.PersistentVolumeClaim.ObjectMeta.Name) {
				indexToRemove = index

				// terminating
				if pod.ObjectMeta.DeletionTimestamp != nil {
					message := fmt.Sprintf("Terminating since %s. Grace period %v sec.", pod.ObjectMeta.DeletionTimestamp.Format(time.RFC3339), *pod.ObjectMeta.DeletionGracePeriodSeconds)
					v.Status = VolumeStatusTypeWarning
					v.Messages = append(v.Messages, VolumeStatusMessage{Type: VolumeStatusMessageTypeWarning, Message: message})
					break
				}

				// check if the pod is runnung
				if pod.Status.Phase == v1.PodRunning {
					if len(pod.Status.ContainerStatuses) > 0 {
						containerStatus := pod.Status.ContainerStatuses[0]
						// check if the container is ready and started and in state running
						if containerStatus.State.Running != nil && containerStatus.Ready && *containerStatus.Started {
							boundedPodRunning = true
						} else {
							// check if lastState exists and add warning message
							if containerStatus.LastTerminationState.Terminated != nil {
								message := fmt.Sprintf("Last termination state: %s, %s. Exit code: %v ", containerStatus.LastTerminationState.Terminated.Reason, containerStatus.LastTerminationState.Terminated.Message, containerStatus.LastTerminationState.Terminated.ExitCode)
								if containerStatus.LastTerminationState.Terminated.ExitCode != 0 {
									v.Status = VolumeStatusTypeError
									v.Messages = append(v.Messages, VolumeStatusMessage{Type: VolumeStatusMessageTypeError, Message: message})
								} else {
									v.Status = VolumeStatusTypeWarning
									v.Messages = append(v.Messages, VolumeStatusMessage{Type: VolumeStatusMessageTypeWarning, Message: message})
								}
							}

							// check if container is waiting and restarted add warning message
							if containerStatus.State.Waiting != nil && containerStatus.RestartCount > 0 {
								message := fmt.Sprintf("Container is waiting. Restarted %v. %s, %s", containerStatus.RestartCount, containerStatus.State.Waiting.Reason, containerStatus.State.Waiting.Message)

								v.Status = VolumeStatusTypeError
								v.Messages = append(v.Messages, VolumeStatusMessage{Type: VolumeStatusMessageTypeError, Message: message})
							}

							if len(v.Messages) > 0 {
								break
							}
						}
					}
				}
				break
			}
		}

		// remove our helper nfs-pod from usedPods array
		if indexToRemove > -1 {
			s.UsedByPods = append(s.UsedByPods[:indexToRemove], s.UsedByPods[indexToRemove+1:]...)
		} else {
			// add warning message if nfs-pod not found
			v.Status = VolumeStatusTypeWarning
			v.Messages = append(v.Messages, VolumeStatusMessage{Type: VolumeStatusMessageTypeWarning, Message: "NFS pod not found"})
		}

		// add usedByPods to response
		for _, pod := range s.UsedByPods {
			v.UsedByPods = append(v.UsedByPods, pod.ObjectMeta.Name)
		}

		// pv, pvc and nfs-pod are bounded and running
		if bounded && boundedPodRunning {
			if s.PersistentVolumeClaim != nil {
				mountPath := utils.MountPath(s.Namespace, v.VolumeName, "/", clientProvider.RunsInCluster())

				if utils.ClusterProviderCached == utils.DOCKER_DESKTOP || utils.ClusterProviderCached == utils.K3S {
					var usedBytes uint64 = sumAllBytesOfFolder(mountPath)
					v.FreeBytes = uint64(s.PersistentVolumeClaim.Spec.Resources.Requests.Storage().Value()) - usedBytes
					v.UsedBytes = usedBytes
					v.TotalBytes = uint64(s.PersistentVolumeClaim.Spec.Resources.Requests.Storage().Value())
				} else {
					free, used, total, _ := diskUsage(mountPath)
					v.FreeBytes = free
					v.UsedBytes = used
					v.TotalBytes = total
				}
			}

			v.Status = VolumeStatusTypeBound
			v.Messages = append(v.Messages, VolumeStatusMessage{Type: VolumeStatusMessageTypeSuccess, Message: "Volume is bound"})
			return
		}
	}

	// check if there are any messages
	if len(v.Messages) > 0 && (v.Status == VolumeStatusTypeError || v.Status == VolumeStatusTypeWarning) {
		return
	}

	if err != nil {
		v.Status = VolumeStatusTypeError
		v.Messages = append(v.Messages, VolumeStatusMessage{Type: VolumeStatusMessageTypeError, Message: err.Error()})
	} else {
		v.Status = VolumeStatusTypePending
		v.Messages = append(v.Messages, VolumeStatusMessage{Type: VolumeStatusMessageTypeInfo, Message: "Volume is processing"})
	}
}

func (s *VolumeStatus) messages() []VolumeStatusMessage {
	// convert PersistentVolumeEvents and PersistentVolumeClaimEvents into messages
	var messages []VolumeStatusMessage

	sort.SliceStable(s.PersistentVolumeEvents, func(i, j int) bool {
		return s.PersistentVolumeEvents[i].LastTimestamp.Time.After(s.PersistentVolumeEvents[j].LastTimestamp.Time)
	})

	for _, event := range s.PersistentVolumeEvents {
		var messageType VolumeStatusMessageType
		if event.Type == "Warning" {
			messageType = VolumeStatusMessageTypeWarning
		} else {
			messageType = VolumeStatusMessageTypeInfo
		}

		messages = append(messages, VolumeStatusMessage{
			Type:    messageType,
			Message: fmt.Sprintf("PersitentVolume: %s", event.Message),
		})
	}

	sort.SliceStable(s.PersistentVolumeClaimEvents, func(i, j int) bool {
		return s.PersistentVolumeClaimEvents[i].LastTimestamp.Time.After(s.PersistentVolumeClaimEvents[j].LastTimestamp.Time)
	})

	for _, event := range s.PersistentVolumeClaimEvents {
		var messageType VolumeStatusMessageType
		if event.Type == "Warning" {
			messageType = VolumeStatusMessageTypeWarning
		} else {
			messageType = VolumeStatusMessageTypeInfo
		}

		messages = append(messages, VolumeStatusMessage{
			Type:    messageType,
			Message: fmt.Sprintf("PersitentVolumeClaim: %s", event.Message),
		})
	}

	return messages
}

func (s *VolumeStatus) SetClient(client *kubernetes.Clientset) {
	s.client = client
}

func (s *VolumeStatus) GetByPVCName(namespace, name string) (*VolumeStatus, error) {
	pvc, err := s.getPVC(namespace, name)
	if err != nil {
		serviceLogger.Warn("failed to gett PVC", "error", err)
		return nil, err
	}

	s.PersistentVolumeClaim = pvc
	s.persistentVolumeClaimName = pvc.Name
	s.Namespace = pvc.Namespace

	// Get the PV from volumeName
	pv, err := s.getPV(pvc.Spec.VolumeName)
	if err != nil {
		serviceLogger.Warn("failed to gett PV", "error", err)
		return s, err
	}
	s.PersistentVolume = pv
	s.persistentVolumeName = pv.Name

	s.provisioningType()
	if _, err := s.collectEventsAndUsedByPods(); err != nil {
		return s, err
	}

	return s, nil
}

func (s *VolumeStatus) GetByPVName(name string) (*VolumeStatus, error) {
	pv, err := s.getPV(name)
	if err != nil {
		serviceLogger.Warn("failed to get PV", "error", err)
		return nil, err
	}

	s.PersistentVolume = pv
	s.persistentVolumeName = pv.Name

	// ClaimRef is only set when bounded, other states handled via pvc
	if pv.Spec.ClaimRef != nil && pv.Spec.ClaimRef.Kind == VolumeTypePersistentVolumeClaim.String() {
		pvc, err := s.getPVC(pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
		if err != nil {
			serviceLogger.Warn("failed to get PVC", "error", err)
			return s, err
		}

		s.PersistentVolumeClaim = pvc
		s.persistentVolumeClaimName = pvc.Name
		s.Namespace = pvc.Namespace
	} else {
		pvc, err := s.findPVCByPVName(pv.Name)
		if err != nil {
			serviceLogger.Warn("failed to gett PVC", "error", err)
			return s, err
		}

		s.PersistentVolumeClaim = pvc
		s.persistentVolumeClaimName = pvc.Name
		s.Namespace = pvc.Namespace
	}

	s.provisioningType()
	if _, err := s.collectEventsAndUsedByPods(); err != nil {
		return s, err
	}

	return s, nil
}

func (s *VolumeStatus) provisioningType() {
	if s.PersistentVolume != nil && s.PersistentVolume.Spec.StorageClassName != "" {
		s.Provisioning = VolumeProvisioningTypeDynamic.String()
	} else {
		s.Provisioning = VolumeProvisioningTypeManual.String()
	}
}

func (s *VolumeStatus) collectEventsAndUsedByPods() (*VolumeStatus, error) {
	pvcsEventsChan := make(chan []v1.Event, 1)
	pvsEventsChan := make(chan []v1.Event, 1)
	podsEventsChan := make(chan []string, 1)
	errorChan := make(chan error, 1)

	processedPvcs, processedPvs, processedPods := false, false, false

	var wg sync.WaitGroup

	// Context with timeout to handle cancellation and timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(VolumeStatusTimeout)*time.Millisecond)
	defer cancel()

	wg.Go(func() {
		s.getUsedByPods(ctx, podsEventsChan, errorChan)
	})

	if s.PersistentVolumeClaim.Name != "" {
		wg.Go(func() {
			s.getEvents(s.PersistentVolumeClaim.Name, VolumeTypePersistentVolumeClaim.String(), ctx, pvcsEventsChan, errorChan)
		})
	}

	if s.PersistentVolume.Name != "" {
		wg.Go(func() {
			s.getEvents(s.PersistentVolume.Name, VolumeTypePersistentVolume.String(), ctx, pvsEventsChan, errorChan)
		})
	}

	go func() {
		wg.Wait() // Wait for all goroutines to finish.

		// IMPORTANT!: Safely close channel after all sends are done.
		close(podsEventsChan)
		close(pvsEventsChan)
		close(pvcsEventsChan)
		close(errorChan)

		serviceLogger.Debug("All goroutines channels closed: podsEventsChan,pvsEventsChan,pvcsEventsChan,errorChan.")
	}()

	var chanError error

EventLoop:
	for {
		select {
		case events, ok := <-pvsEventsChan:
			processedPvs = true
			if !ok {
				serviceLogger.Debug("Warning PV event channel closed.")
				break
			}
			s.PersistentVolumeEvents = events

		case events, ok := <-pvcsEventsChan:
			processedPvcs = true
			if !ok {
				serviceLogger.Debug("Warning PVC event channel closed.")
				break
			}
			s.PersistentVolumeClaimEvents = events

		case _, ok := <-podsEventsChan:
			processedPods = true
			if !ok {
				serviceLogger.Debug("Warning Pods event channel closed.")
			}

		case chanError = <-errorChan:
			break EventLoop

		case <-ctx.Done():
			serviceLogger.Debug("Warning timeout waiting for events")
			break EventLoop

		default:
			if processedPvs && processedPvcs && processedPods {
				serviceLogger.Debug("EventLoop default break.")
				break EventLoop
			}

			serviceLogger.Debug("EventLoop default 15millis sleep.")

			time.Sleep(15 * time.Millisecond)
		}
	}

	if chanError != nil {
		return s, chanError
	}

	return s, nil
}

func (s *VolumeStatus) findPVCByPVName(name string) (*v1.PersistentVolumeClaim, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client is not set")
	}

	pvcs, err := s.client.CoreV1().PersistentVolumeClaims("").List(context.Background(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.volumeName=%s", name),
	})

	if err != nil {
		return nil, err
	}

	if len(pvcs.Items) == 0 || len(pvcs.Items) > 1 {
		return nil, fmt.Errorf("issue searching PVC for PV: %s", name)
	}

	return &pvcs.Items[0], nil
}

func (s *VolumeStatus) getPVC(namespace, name string) (*v1.PersistentVolumeClaim, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client is not set")
	}

	pvc, err := s.client.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return pvc, nil
}

func (s *VolumeStatus) getPV(name string) (*v1.PersistentVolume, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client is not set")
	}

	pv, err := s.client.CoreV1().PersistentVolumes().Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return pv, nil
}

func (s *VolumeStatus) getEvents(name, kind string, ctx context.Context, channel chan<- []v1.Event, errChannel chan<- error) {
	if s.client == nil {
		errChannel <- fmt.Errorf("client is not set")
		return
	}

	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=%s", name, kind)
	eventList, err := s.client.CoreV1().Events(s.Namespace).List(context.Background(), metav1.ListOptions{
		FieldSelector: fieldSelector,
	})

	if err != nil {
		errChannel <- err
		return
	}

	events := eventList.Items

	sort.SliceStable(events, func(i, j int) bool {
		return events[i].LastTimestamp.Time.After(events[j].LastTimestamp.Time)
	})

	// Push the events into the channel
	select {
	case <-ctx.Done():
		serviceLogger.Debug("Async: timeout waiting for events", "kind", kind)
		return
	case channel <- events:
		serviceLogger.Debug("Async: push the events into the channel", "kind", kind)
	}
}

func (s *VolumeStatus) getUsedByPods(ctx context.Context, channel chan<- []string, errChannel chan<- error) {
	if s.client == nil {
		errChannel <- fmt.Errorf("client is not set")
		return
	}

	if s.Namespace == "" {
		errChannel <- fmt.Errorf("namespace is not set")
		return
	}

	pods, err := s.client.CoreV1().Pods(s.Namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		errChannel <- err
		return
	}

	prefix := fmt.Sprintf("%s-", utils.NFS_POD_PREFIX)

	var usedBy []string
	for _, pod := range pods.Items {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				// normalize name, convert no prefixed 'nfs-server-pod-' to prefixed and vice versa
				nonPrefixName := strings.TrimPrefix(volume.PersistentVolumeClaim.ClaimName, prefix)
				prefixName := prefix + nonPrefixName

				if s.persistentVolumeClaimName == prefixName {
					usedBy = append(usedBy, pod.Name)
					s.UsedByPods = append(s.UsedByPods, pod)
				}
			}
		}
	}

	// Push the events into the channel
	select {
	case <-ctx.Done():
		serviceLogger.Debug("Async: timeout waiting for pods")
		return
	case channel <- usedBy:
		serviceLogger.Debug("Async: push pods into the channel")
	}
}
