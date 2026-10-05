package reconciler

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
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

const defaultPlatformSource = "https://raw.githubusercontent.com/mogenius/platform-defaults"

// releaseVersionPattern matches release tags of platform-defaults (v1.0.0,
// 1.2.3, v1.0.0-rc.1). Those live under refs/tags; everything else is
// treated as a branch name.
var releaseVersionPattern = regexp.MustCompile(`^v?\d+\.\d+\.\d+([-+].*)?$`)

func getDefaultConfig(source string, version string, component string) (componentDefaultSpec, error) {
	url := defaultConfigURL(source, version, component)

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

// defaultConfigURL builds the URL of a component's default config. For the
// default source, release versions resolve to refs/tags/<version> and
// anything else to refs/heads/<version>; a custom source is used as base URL
// as is.
func defaultConfigURL(source string, version string, component string) string {
	// An empty version would build ".../refs/heads//traefik.yaml", so every
	// component fetch would 404 and no component could be installed at all --
	// a confusing failure for a field the UI does not even offer.
	if version == "" {
		version = "main"
	}
	if source == "" {
		ref := "heads"
		if releaseVersionPattern.MatchString(version) {
			ref = "tags"
		}
		source = fmt.Sprintf("%s/refs/%s", defaultPlatformSource, ref)
	}
	return fmt.Sprintf("%s/%s/%s.yaml", source, version, component)
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
	defer func() { _ = resp.Body.Close() }()

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
