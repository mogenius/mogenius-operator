package utils

import (
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/kubectl/pkg/scheme"
)

func InitMogeniusNfsPersistentVolumeClaim() corev1.PersistentVolumeClaim {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/mo-storage-nfs-pvc.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app corev1.PersistentVolumeClaim
	_, _, err = s.Decode(yaml, nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitMogeniusNfsDeployment() v1.Deployment {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/mo-storage-nfs-deployment.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app v1.Deployment
	_, _, err = s.Decode(yaml, nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitMogeniusNfsService() corev1.Service {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/mo-storage-nfs-service.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var service corev1.Service
	_, _, err = s.Decode(yaml, nil, &service)
	if err != nil {
		panic(err)
	}
	return service
}
