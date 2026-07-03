package reconciler

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	sigsyaml "sigs.k8s.io/yaml"
)

type componentDefaults struct {
	Kind       string               `json:"kind"`
	ApiVersion string               `json:"apiVersion"`
	Spec       componentDefaultSpec `json:"spec"`
}
type componentDefaultSpec struct {
	Version      string         `json:"version"`
	ValuesObject map[string]any `json:"valuesObject,omitempty"`
}

var defaultConfigHTTPClient = &http.Client{Timeout: 10 * time.Second}

// defaultConfigCache caches raw default-config responses per URL. Every
// component of every reconcile would otherwise hit the remote host again.
// Raw bytes (not the parsed spec) are cached because mergeHelmValues inserts
// nested maps by reference and would mutate a shared parsed result.
const defaultConfigCacheTTL = 5 * time.Minute

var (
	defaultConfigCacheMu sync.Mutex
	defaultConfigCache   = map[string]cachedDefaultConfig{}
)

type cachedDefaultConfig struct {
	body      []byte
	fetchedAt time.Time
}

func getDefaultConfig(source string, version string, component string) (componentDefaultSpec, error) {
	if source == "" {
		source = "https://raw.githubusercontent.com/mogenius/platform-defaults/refs/heads"
	}
	url := fmt.Sprintf("%s/%s/%s.yaml", source, version, component)

	body, err := fetchDefaultConfigCached(url)
	if err != nil {
		return componentDefaultSpec{}, err
	}

	var defaults componentDefaults
	if err := sigsyaml.Unmarshal(body, &defaults); err != nil {
		return componentDefaultSpec{}, fmt.Errorf("parse default config: %w", err)
	}

	return defaults.Spec, nil
}

func fetchDefaultConfigCached(url string) ([]byte, error) {
	defaultConfigCacheMu.Lock()
	cached, ok := defaultConfigCache[url]
	defaultConfigCacheMu.Unlock()
	if ok && time.Since(cached.fetchedAt) < defaultConfigCacheTTL {
		return cached.body, nil
	}

	resp, err := defaultConfigHTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch default config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch default config: unexpected status %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read default config: %w", err)
	}

	defaultConfigCacheMu.Lock()
	defaultConfigCache[url] = cachedDefaultConfig{body: body, fetchedAt: time.Now()}
	defaultConfigCacheMu.Unlock()

	return body, nil
}
