package dtos

import v2 "k8s.io/api/autoscaling/v2"

type K8sHpaSettingsDto struct {
	Name      string                      `json:"name" validate:"required"`
	Namespace string                      `json:"namespace" validate:"required"`
	Data      *v2.HorizontalPodAutoscaler `json:"data" validate:"required"`
}

func K8sHpaSettingsDtoExampleData() *K8sHpaSettingsDto {
	max := int32(2)
	return &K8sHpaSettingsDto{
		Name:      "testhpa",
		Namespace: "testnamespace",
		Data: &v2.HorizontalPodAutoscaler{
			Spec: v2.HorizontalPodAutoscalerSpec{
				MaxReplicas: max,
				Metrics:     nil,
			},
		},
	}
}
