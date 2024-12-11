package structs

import jsoniter "github.com/json-iterator/go"

type CniData struct {
	Name       string   `json:"name"`
	Node       string   `json:"node"`
	CNIVersion string   `json:"cniVersion"`
	Plugins    []Plugin `json:"plugins"`
}

type Plugin struct {
	Type          string           `json:"type"`
	LogLevel      string           `json:"log_level,omitempty"`
	LogFilePath   string           `json:"log_file_path,omitempty"`
	DatastoreType string           `json:"datastore_type,omitempty"`
	Nodename      string           `json:"nodename,omitempty"`
	MTU           int              `json:"mtu,omitempty"`
	IPAM          *CniIPAM         `json:"ipam,omitempty"`
	Policy        *CniPolicy       `json:"policy,omitempty"`
	Kubernetes    *CniKubernetes   `json:"kubernetes,omitempty"`
	SNAT          *bool            `json:"snat,omitempty"`
	Capabilities  *CniCapabilities `json:"capabilities,omitempty"`
}

type CniIPAM struct {
	Type string `json:"type"`
}

type CniPolicy struct {
	Type string `json:"type"`
}

type CniKubernetes struct {
	Kubeconfig string `json:"kubeconfig"`
}

type CniCapabilities struct {
	PortMappings bool `json:"portMappings,omitempty"`
	Bandwidth    bool `json:"bandwidth,omitempty"`
}

func UnmarshalCniData(dst *[]CniData, data []byte) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, dst)
	if err != nil {
		return err
	}
	return nil
}
