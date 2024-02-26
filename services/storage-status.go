package services

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/logger"
	"sort"
	"sync"
	"time"

	punq "github.com/mogenius/punq/kubernetes"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// milliseconds
	StorageStatusTimeout = 250
)

type StorageAPIObject int

const (
	StorageTypePersistentVolume StorageAPIObject = iota
	StorageTypePersistentVolumeClaim
)

func (s StorageAPIObject) String() string {
	return [...]string{"PersistentVolume", "PersistentVolumeClaim"}[s]
}

func StorageAPIObjectFromString(s string) StorageAPIObject {
	switch s {
	case "PersistentVolume":
		return StorageTypePersistentVolume
	case "PersistentVolumeClaim":
		return StorageTypePersistentVolumeClaim
	default:
		return -1
	}
}

type Provisioning int

const (
	Dynamic Provisioning = iota
	Manual
)

func (p Provisioning) String() string {
	return [...]string{"Dynamic", "Manual"}[p]
}

type StorageStatus struct {
	client *kubernetes.Clientset
	//
	PersistentVolumeObject     *v1.PersistentVolume      `json:"persistentVolumeObject,omitempty"`
	PersistenVolumeClaimObject *v1.PersistentVolumeClaim `json:"persistentVolumeClaimObject,omitempty"`
	UsedByPods                 []v1.Pod                  `json:"usedByPods,omitempty"`
	//
	Namespace             string     `json:"namespace"`
	Provisioning          string     `json:"provisioning"`
	PersistentVolume      string     `json:"persistentVolume"`
	PersistentVolumeClaim string     `json:"persistentVolumeClaims"`
	UsedBy                []string   `json:"usedBy,omitempty"`
	VolumeEvents          []v1.Event `json:"volumeEvents,omitempty"`
	VolumeClaimEvents     []v1.Event `json:"volumeClaimEvents,omitempty"`
}

func StatusMogeniusNfs(r NfsStatusRequest) NfsStatusResponse {
	logger.Log.Debugf("Storage status for (%s): %s %s", r.StorageAPIObject, r.Namespace, r.Name)

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		logger.Log.Warningf("Warningf: %s", err.Error())
		return NfsStatusResponse{Error: err.Error()}
	}

	storageStatus := StorageStatus{}
	storageStatus.SetClient(provider.ClientSet)

	if StorageAPIObjectFromString(r.StorageAPIObject) == StorageTypePersistentVolume {
		if _, err := storageStatus.GetByPVName(r.Name); err != nil {
			return NfsStatusResponse{Error: err.Error()}
		}
	} else if StorageAPIObjectFromString(r.StorageAPIObject) == StorageTypePersistentVolumeClaim {
		if _, err := storageStatus.GetByPVCName(r.Namespace, r.Name); err != nil {
			return NfsStatusResponse{Error: err.Error()}
		}
	} else {
		return NfsStatusResponse{Error: "Invalid StorageAPIObject"}
	}

	return NfsStatusResponse{
		Status: storageStatus,
	}
}

func (s *StorageStatus) SetClient(client *kubernetes.Clientset) {
	s.client = client
}

func (s *StorageStatus) GetByPVCName(namespace, name string) (*StorageStatus, error) {
	pvc, err := s.getPVC(namespace, name)
	if err != nil {
		fmt.Printf("Error getting PVC: %v\n", err)
		return nil, err
	}

	s.PersistenVolumeClaimObject = pvc
	s.PersistentVolumeClaim = pvc.Name
	s.Namespace = pvc.Namespace

	// Get the PV from volumeName
	pv, err := s.getPV(pvc.Spec.VolumeName)
	if err != nil {
		fmt.Printf("Error getting PV: %v\n", err)
		return s, err
	}
	s.PersistentVolumeObject = pv
	s.PersistentVolume = pv.Name

	s.provisioningType()
	if _, err := s.collectCommonData(); err != nil {
		return s, err
	}

	return s, nil
}

func (s *StorageStatus) GetByPVName(name string) (*StorageStatus, error) {
	pv, err := s.getPV(name)
	if err != nil {
		fmt.Printf("Error getting PV: %v\n", err)
		return nil, err
	}

	s.PersistentVolumeObject = pv
	s.PersistentVolume = pv.Name

	// ClaimRef is only set when bounded, other states handled via pvc
	if pv.Spec.ClaimRef != nil && pv.Spec.ClaimRef.Kind == StorageTypePersistentVolumeClaim.String() {
		pvc, err := s.getPVC(pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
		if err != nil {
			fmt.Printf("Error getting PVC: %v\n", err)
			return s, err
		}

		s.PersistenVolumeClaimObject = pvc
		s.PersistentVolumeClaim = pvc.Name
		s.Namespace = pvc.Namespace
	} else {
		pvc, err := s.findPVCByPVName(pv.Name)
		if err != nil {
			fmt.Printf("Error getting PVC: %v\n", err)
			return s, err
		}

		s.PersistenVolumeClaimObject = pvc
		s.PersistentVolumeClaim = pvc.Name
		s.Namespace = pvc.Namespace
	}

	s.provisioningType()
	if _, err := s.collectCommonData(); err != nil {
		return s, err
	}

	return s, nil
}

func (s *StorageStatus) provisioningType() {
	if s.PersistentVolumeObject != nil && s.PersistentVolumeObject.Spec.StorageClassName != "" {
		s.Provisioning = Dynamic.String()
	} else {
		s.Provisioning = Manual.String()
	}
}

func (s *StorageStatus) collectCommonData() (*StorageStatus, error) {
	pvcsEventsChan := make(chan []v1.Event, 1)
	pvsEventsChan := make(chan []v1.Event, 1)
	podsEventsChan := make(chan []string, 1)
	errorChan := make(chan error, 1)

	processedPvcs, processedPvs, processedPods := false, false, false

	var wg sync.WaitGroup

	// Context with timeout to handle cancellation and timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(StorageStatusTimeout)*time.Millisecond)
	defer cancel()

	wg.Add(1)
	go s.getUsedBy(ctx, &wg, podsEventsChan, errorChan)

	if s.PersistenVolumeClaimObject.Name != "" {
		wg.Add(1)
		go s.getEvents(s.PersistenVolumeClaimObject.Name, StorageTypePersistentVolumeClaim.String(), ctx, &wg, pvcsEventsChan, errorChan)
	}

	if s.PersistentVolumeObject.Name != "" {
		wg.Add(1)
		go s.getEvents(s.PersistentVolumeObject.Name, StorageTypePersistentVolume.String(), ctx, &wg, pvsEventsChan, errorChan)
	}

	go func() {
		wg.Wait() // Wait for all goroutines to finish.

		// IMPORTANT!: Safely close channel after all sends are done.
		close(podsEventsChan)
		close(pvsEventsChan)
		close(pvcsEventsChan)
		close(errorChan)

		fmt.Println("All goroutines finished")
	}()

	var chanError error

EventLoop:
	for {
		select {
		case events, ok := <-pvsEventsChan:
			processedPvs = true
			if !ok {
				fmt.Println("Warning PV event channel closed.")
				break
			}
			s.VolumeEvents = events

		case events, ok := <-pvcsEventsChan:
			processedPvcs = true
			if !ok {
				fmt.Println("Warning PVC event channel closed.")
				break
			}
			s.VolumeClaimEvents = events

		case pods, ok := <-podsEventsChan:
			processedPods = true
			if !ok {
				fmt.Println("Warning Pods event channel closed.")
				break
			}
			s.UsedBy = pods

		case chanError = <-errorChan:
			break EventLoop

		case <-ctx.Done():
			fmt.Println("Warning timeout waiting for events")
			break EventLoop
		}

		if processedPvs && processedPvcs && processedPods {
			break EventLoop
		}

		fmt.Println("Channel timeout")
		time.Sleep(15 * time.Millisecond)
	}

	if chanError != nil {
		return s, chanError
	}

	return s, nil
}

func (s *StorageStatus) findPVCByPVName(name string) (*v1.PersistentVolumeClaim, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client is not set")
	}

	pvcs, err := s.client.CoreV1().PersistentVolumeClaims("").List(context.TODO(), metav1.ListOptions{
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

func (s *StorageStatus) getPVC(namespace, name string) (*v1.PersistentVolumeClaim, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client is not set")
	}

	pvc, err := s.client.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return pvc, nil
}

func (s *StorageStatus) getPV(name string) (*v1.PersistentVolume, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client is not set")
	}

	pv, err := s.client.CoreV1().PersistentVolumes().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return pv, nil
}

func (s *StorageStatus) getEvents(name, kind string, ctx context.Context, wg *sync.WaitGroup, channel chan<- []v1.Event, errChannel chan<- error) {
	defer wg.Done()

	if s.client == nil {
		errChannel <- fmt.Errorf("client is not set")
		return
	}

	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=%s", name, kind)
	eventList, err := s.client.CoreV1().Events(s.Namespace).List(context.TODO(), metav1.ListOptions{
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
		fmt.Printf("go: timeout waiting for events: %s\n", kind)
		return
	case channel <- events:
		fmt.Printf("go: push the events into the channel: %s\n", kind)
	}
}

func (s *StorageStatus) getUsedBy(ctx context.Context, wg *sync.WaitGroup, channel chan<- []string, errChannel chan<- error) {
	defer wg.Done()

	if s.client == nil {
		errChannel <- fmt.Errorf("client is not set")
		return
	}

	if s.Namespace == "" {
		errChannel <- fmt.Errorf("namespace is not set")
		return
	}

	pods, err := s.client.CoreV1().Pods(s.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		errChannel <- err
		return
	}

	var usedBy []string
	for _, pod := range pods.Items {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == s.PersistentVolumeClaim {
				usedBy = append(usedBy, pod.Name)
				s.UsedByPods = append(s.UsedByPods, pod)
			}
		}
	}

	// Push the events into the channel
	select {
	case <-ctx.Done():
		fmt.Println("go: timeout waiting for events: Pods")
		return
	case channel <- usedBy:
		fmt.Println("go: push the events into the channel: Pods")
	}
}
