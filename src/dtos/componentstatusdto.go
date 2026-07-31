package dtos

// ComponentStatus reports whether an integrated in-cluster component is installed.
// It is the presence counterpart to a live reachability probe.
// Namespace is the discovered service's namespace (empty when the component is absent).
type ComponentStatus struct {
	Installed bool   `json:"installed"`
	Namespace string `json:"namespace,omitempty"`
}
