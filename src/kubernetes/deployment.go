package kubernetes

import (
	"fmt"
	"mogenius-operator/src/store"
	"strings"

	v1 "k8s.io/api/apps/v1"
)

func GetDeploymentsWithFieldSelector(namespace string, labelSelector string) ([]v1.Deployment, error) {
	all := store.GetDeployments(namespace, "*")
	if labelSelector == "" {
		return all, nil
	}
	var result []v1.Deployment
	for _, d := range all {
		if matchesLabelSelector(d.Labels, labelSelector) {
			result = append(result, d)
		}
	}
	return result, nil
}

// matchesLabelSelector matches equality-based label selectors (key=value, key!=value).
func matchesLabelSelector(labels map[string]string, selector string) bool {
	for req := range strings.SplitSeq(selector, ",") {
		req = strings.TrimSpace(req)
		if before, after, ok := strings.Cut(req, "!="); ok {
			if labels[before] == after {
				return false
			}
		} else if before, after, ok := strings.Cut(req, "="); ok {
			if labels[before] != after {
				return false
			}
		}
	}
	return true
}

func IsDeploymentInstalled(namespaceName string, name string) (string, error) {
	ownDeployment := store.GetDeployment(namespaceName, name)
	if ownDeployment == nil {
		return "", fmt.Errorf("deployment not found")
	}

	result := ""
	split := strings.Split(ownDeployment.Spec.Template.Spec.Containers[0].Image, ":")
	if len(split) > 1 {
		result = split[1]
	}

	return result, nil
}
