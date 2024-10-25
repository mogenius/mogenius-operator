package interfaces

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

type KubernetesWatcher interface {
	// Register a watcher for the given resource
	Watch(resource ResourceIdentifier, onAdd func(resource ResourceIdentifier, obj *unstructured.Unstructured), onUpdate func(resource ResourceIdentifier, oldObj *unstructured.Unstructured, newObj *unstructured.Unstructured), onDelete func(resource ResourceIdentifier, obj *unstructured.Unstructured)) error
	// Stop the watcher for the given resource
	Unwatch(resource ResourceIdentifier) error
	// Query the status of the resource
	State(resource ResourceIdentifier) (ResourceState, error)
	// List all currently watched resources
	ListWatchedResources() []ResourceIdentifier
}

type ResourceIdentifier struct {
	Name         string
	Kind         string
	Version      string
	GroupVersion string
	Namespaced   bool
}

type ResourceState string

const (
	Unknown             ResourceState = "Unknown"
	Watching            ResourceState = "Watching"
	WatcherInitializing ResourceState = "WatcherInitializing"
	WatchingFailed      ResourceState = "WatchingFailed"
)
