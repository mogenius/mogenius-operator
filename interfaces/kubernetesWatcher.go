package interfaces

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

type KubernetesWatcher interface {
	// Register a watcher for the given resource
	Watch(resource KubernetesWatcherResourceIdentifier, onAdd KubernetesWatcherOnAdd, onUpdate KubernetesWatcherOnUpdate, onDelete KubernetesWatcherOnDelete) error
	// Stop the watcher for the given resource
	Unwatch(resource KubernetesWatcherResourceIdentifier) error
	// Query the status of the resource
	State(resource KubernetesWatcherResourceIdentifier) (KubernetesWatcherResourceState, error)
	// List all currently watched resources
	ListWatchedResources() []KubernetesWatcherResourceIdentifier
}

type KubernetesWatcherOnAdd func(resource KubernetesWatcherResourceIdentifier, obj *unstructured.Unstructured)
type KubernetesWatcherOnUpdate func(resource KubernetesWatcherResourceIdentifier, oldObj *unstructured.Unstructured, newObj *unstructured.Unstructured)
type KubernetesWatcherOnDelete func(resource KubernetesWatcherResourceIdentifier, obj *unstructured.Unstructured)

type KubernetesWatcherResourceIdentifier struct {
	Name         string
	Kind         string
	Version      string
	GroupVersion string
	Namespaced   bool
}

type KubernetesWatcherResourceState string

const (
	Unknown             KubernetesWatcherResourceState = "Unknown"
	Watching            KubernetesWatcherResourceState = "Watching"
	WatcherInitializing KubernetesWatcherResourceState = "WatcherInitializing"
	WatchingFailed      KubernetesWatcherResourceState = "WatchingFailed"
)
