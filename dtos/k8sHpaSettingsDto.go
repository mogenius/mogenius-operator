package dtos

import v2 "k8s.io/api/autoscaling/v2"

type K8sHpaSettingsDto struct {
	*v2.HorizontalPodAutoscalerSpec `json:",inline"`
}
