package interfaces

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// A generic kubernetes resource watcher
type WatcherModule interface {
	// Register a watcher for the given resource
	Watch(resource WatcherResourceIdentifier, onAdd WatcherOnAdd, onUpdate WatcherOnUpdate, onDelete WatcherOnDelete) error
	// Stop the watcher for the given resource
	Unwatch(resource WatcherResourceIdentifier) error
	// Query the status of the resource
	State(resource WatcherResourceIdentifier) (WatcherResourceState, error)
	// List all currently watched resources
	ListWatchedResources() []WatcherResourceIdentifier
}

type WatcherOnAdd func(resource WatcherResourceIdentifier, obj *unstructured.Unstructured)
type WatcherOnUpdate func(resource WatcherResourceIdentifier, oldObj *unstructured.Unstructured, newObj *unstructured.Unstructured)
type WatcherOnDelete func(resource WatcherResourceIdentifier, obj *unstructured.Unstructured)

type WatcherResourceIdentifier struct {
	Name         string
	Kind         string
	Version      string
	GroupVersion string
	Namespaced   bool
}

type WatcherResourceState string

const (
	Unknown             WatcherResourceState = "Unknown"
	Watching            WatcherResourceState = "Watching"
	WatcherInitializing WatcherResourceState = "WatcherInitializing"
	WatchingFailed      WatcherResourceState = "WatchingFailed"
)
