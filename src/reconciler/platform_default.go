package reconciler

type componentDefaults struct {
	Kind       string               `json:"kind"`
	ApiVersion string               `json:"apiVersion"`
	Spec       componentDefaultSpec `json:"spec"`
}
type componentDefaultSpec struct {
	Version string `json:"version"`
	Values  string `json:"values"`
}

func getDefaultConfig(version string, component string, source string) componentDefaultSpec {
	return componentDefaultSpec{}
}
