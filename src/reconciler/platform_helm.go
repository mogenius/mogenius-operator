package reconciler

import (
	"encoding/json"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
)

func helmChartName(reference *v1alpha1.HelmChartReference, defaultName string) string {
	if reference != nil && reference.Chart != "" {
		return reference.Chart
	}
	return defaultName
}
func helmReleaseName(reference *v1alpha1.HelmChartReference, defaultName string) string {
	if reference != nil && reference.Name != "" {
		return reference.Name
	}
	return defaultName
}
func helmRepository(reference *v1alpha1.HelmChartReference, defaultRepository string) string {
	if reference != nil && reference.Repository != "" {
		return reference.Repository
	}
	return defaultRepository
}
func helmVersion(reference *v1alpha1.HelmChartReference, defaultVersion string) string {
	if reference != nil && reference.Version != "" {
		return reference.Version
	}
	return defaultVersion
}

// mergeHelmValues builds a merged values map in three layers:
//  1. defaults.ValuesObject from getDefaultConfig
//  2. configValues derived from the component spec
//  3. patch values from a PlatformPatch (highest precedence)
func mergeHelmValues(defaults componentDefaultSpec, configValues map[string]interface{}, patch *v1alpha1.PlatformPatch) (map[string]interface{}, error) {
	result := map[string]interface{}{}

	mergeMaps(result, defaults.ValuesObject)

	mergeMaps(result, configValues)

	if patch != nil && patch.Spec.ValuesObject != nil {
		patchValues := map[string]interface{}{}
		if err := json.Unmarshal(patch.Spec.ValuesObject.Raw, &patchValues); err != nil {
			return nil, fmt.Errorf("parse patch values: %w", err)
		}
		mergeMaps(result, patchValues)
	}

	return result, nil
}

// mergeMaps deep-merges src into dst. Nested maps are merged recursively;
// all other values in src overwrite those in dst.
func mergeMaps(dst, src map[string]interface{}) {
	for k, srcVal := range src {
		if srcMap, ok := srcVal.(map[string]interface{}); ok {
			if dstMap, ok := dst[k].(map[string]interface{}); ok {
				mergeMaps(dstMap, srcMap)
				continue
			}
		}
		dst[k] = srcVal
	}
}
