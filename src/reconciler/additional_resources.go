package reconciler

import (
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/utils"
	"regexp"
	"strings"
)

var repoNameSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

// repositorySecretName derives a stable, DNS-safe Kubernetes resource name from a repository URL.
func repositorySecretName(url string) string {
	s := strings.ToLower(url)
	for _, prefix := range []string{"https://", "http://", "ssh://", "git@"} {
		s = strings.TrimPrefix(s, prefix)
	}
	s = repoNameSanitizer.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	// "mo-repo-" prefix is 8 chars; Kubernetes names are max 63 chars.
	if len(s) > 55 {
		s = s[:55]
	}
	return "mo-repo-" + s
}

func externalSecretResource(name, namespace string, es v1alpha1.ExternalSecret, targetLabels map[string]string, extraData map[string]string) map[string]any {
	key := "token"

	remoteRef := map[string]any{
		"key": es.Path,
	}

	if es.Key != "" {
		key = es.Key
		remoteRef["property"] = es.Key
	}

	target := map[string]any{
		"creationPolicy": "Owner",
		"deletionPolicy": "Merge",
		"name":           name,
	}

	if len(targetLabels) > 0 || len(extraData) > 0 {
		tmpl := map[string]any{}
		if len(targetLabels) > 0 {
			tmpl["metadata"] = map[string]any{
				"labels": toStringAnyMap(targetLabels),
			}
		}
		if len(extraData) > 0 {
			tmpl["data"] = toStringAnyMap(extraData)
		}
		target["template"] = tmpl
	}

	return map[string]any{
		"apiVersion": utils.ExternalSecretResource.ApiVersion,
		"kind":       utils.ExternalSecretResource.Kind,
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]any{
			"data": []map[string]any{
				{
					"remoteRef": remoteRef,
					"secretKey": key,
				},
			},
			"secretStoreRef": map[string]any{
				"kind": utils.ClusterSecretStoreResource.Kind,
				"name": es.Vault,
			},
			"target": target,
		},
	}
}

func toStringAnyMap(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
