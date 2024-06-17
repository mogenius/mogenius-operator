package utils

import (
	v1 "k8s.io/api/apps/v1"
	v1job "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/kubectl/pkg/scheme"
)

func InitUpgradeConfigMap() corev1.ConfigMap {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/upgrade-configmap.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app corev1.ConfigMap
	_, _, err = s.Decode(yaml, nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitUpgradeJob() v1job.Job {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/upgrade-job.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app v1job.Job
	_, _, err = s.Decode(yaml, nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

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

func InitMogeniusNfsPersistentVolumeClaimForService() corev1.PersistentVolumeClaim {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/mo-storage-service-pvc.yaml")
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

func InitMogeniusNfsPersistentVolumeForService() corev1.PersistentVolume {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/mo-storage-service-pv.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app corev1.PersistentVolume
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

func InitMogeniusContainerRegistryIngress() netv1.Ingress {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/mo-container-registry-ingress.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var ingress netv1.Ingress
	_, _, err = s.Decode(yaml, nil, &ingress)
	if err != nil {
		panic(err)
	}
	return ingress
}

func InitMogeniusContainerRegistrySecret(crt string, key string) corev1.Secret {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/mo-container-registry-tls-secret.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var secret corev1.Secret
	_, _, err = s.Decode(yaml, nil, &secret)
	if err != nil {
		panic(err)
	}

	secret.StringData = make(map[string]string)
	secret.StringData["tls.crt"] = crt
	secret.StringData["tls.key"] = key
	secret.Namespace = CONFIG.Kubernetes.OwnNamespace

	return secret
}

func InitMogeniusCrdProjectsYaml() string {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/crds-projects.yaml")
	if err != nil {
		panic(err.Error())
	}

	return string(yaml)
}

func InitMogeniusCrdEnvironmentsYaml() string {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/crds-environments.yaml")
	if err != nil {
		panic(err.Error())
	}

	return string(yaml)
}

func InitMogeniusCrdApplicationKitYaml() string {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/crds-applicationkit.yaml")
	if err != nil {
		panic(err.Error())
	}

	return string(yaml)
}

func InitExternalSecretsStoreYaml() string {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/external-secrets-store-vault.yml")
	if err != nil {
		panic(err.Error())
	}
	return string(yaml)
}
