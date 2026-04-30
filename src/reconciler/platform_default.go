package reconciler

import (
	"fmt"
	"io"
	"net/http"
	"time"

	sigsyaml "sigs.k8s.io/yaml"
)

type componentDefaults struct {
	Kind       string               `json:"kind"`
	ApiVersion string               `json:"apiVersion"`
	Spec       componentDefaultSpec `json:"spec"`
}
type componentDefaultSpec struct {
	Version      string                 `json:"version"`
	ValuesObject map[string]interface{} `json:"valuesObject,omitempty"`
}

var defaultConfigHTTPClient = &http.Client{Timeout: 10 * time.Second}

func getDefaultConfig(source string, version string, component string) (componentDefaultSpec, error) {
	if source == "" {
		source = "https://raw.githubusercontent.com/mogenius/platform-defaults/refs/heads"
	}
	url := fmt.Sprintf("%s/%s/%s.yaml", source, version, component)

	resp, err := defaultConfigHTTPClient.Get(url)
	if err != nil {
		return componentDefaultSpec{}, fmt.Errorf("fetch default config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return componentDefaultSpec{}, fmt.Errorf("fetch default config: unexpected status %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return componentDefaultSpec{}, fmt.Errorf("read default config: %w", err)
	}

	var defaults componentDefaults
	if err := sigsyaml.Unmarshal(body, &defaults); err != nil {
		return componentDefaultSpec{}, fmt.Errorf("parse default config: %w", err)
	}

	return defaults.Spec, nil
}
