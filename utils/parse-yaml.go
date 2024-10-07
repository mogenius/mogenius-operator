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
	yaml := readYaml("yaml-templates/upgrade-configmap.yaml")

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app corev1.ConfigMap
	_, _, err := s.Decode([]byte(yaml), nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitUpgradeConfigMapYaml() string {
	return readYaml("yaml-templates/upgrade-configmap.yaml")
}

func InitUpgradeJob() v1job.Job {
	yaml := readYaml("yaml-templates/upgrade-job.yaml")

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app v1job.Job
	_, _, err := s.Decode([]byte(yaml), nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitMogeniusNfsPersistentVolumeClaim() corev1.PersistentVolumeClaim {
	yaml := readYaml("yaml-templates/mo-storage-nfs-pvc.yaml")

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app corev1.PersistentVolumeClaim
	_, _, err := s.Decode([]byte(yaml), nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitMogeniusNfsPersistentVolumeClaimForService() corev1.PersistentVolumeClaim {
	yaml := readYaml("yaml-templates/mo-storage-service-pvc.yaml")

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app corev1.PersistentVolumeClaim
	_, _, err := s.Decode([]byte(yaml), nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitMogeniusNfsPersistentVolumeForService() corev1.PersistentVolume {
	yaml := readYaml("yaml-templates/mo-storage-service-pv.yaml")

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app corev1.PersistentVolume
	_, _, err := s.Decode([]byte(yaml), nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitMogeniusNfsDeployment() v1.Deployment {
	yaml := readYaml("yaml-templates/mo-storage-nfs-deployment.yaml")

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app v1.Deployment
	_, _, err := s.Decode([]byte(yaml), nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitMogeniusNfsService() corev1.Service {
	yaml := readYaml("yaml-templates/mo-storage-nfs-service.yaml")

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var service corev1.Service
	_, _, err := s.Decode([]byte(yaml), nil, &service)
	if err != nil {
		panic(err)
	}
	return service
}

func InitMogeniusContainerRegistryIngress() netv1.Ingress {
	yaml := readYaml("yaml-templates/mo-container-registry-ingress.yaml")

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var ingress netv1.Ingress
	_, _, err := s.Decode([]byte(yaml), nil, &ingress)
	if err != nil {
		panic(err)
	}
	return ingress
}

func InitMogeniusContainerRegistrySecret(crt string, key string) corev1.Secret {
	yaml := readYaml("yaml-templates/mo-container-registry-tls-secret.yaml")

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var secret corev1.Secret
	_, _, err := s.Decode([]byte(yaml), nil, &secret)
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
	return readYaml("yaml-templates/crds-projects.yaml")
}

func InitMogeniusCrdEnvironmentsYaml() string {
	return readYaml("yaml-templates/crds-environments.yaml")
}

func InitMogeniusCrdApplicationKitYaml() string {
	return readYaml("yaml-templates/crds-applicationkit.yaml")
}

func InitExternalSecretsStoreYaml() string {
	return readYaml("yaml-templates/external-secrets-store-vault.yaml")
}

func InitExternalSecretListYaml() string {
	return readYaml("yaml-templates/external-secret-list-available-kvs.yaml")
}

func InitExternalSecretYaml() string {
	return readYaml("yaml-templates/external-secret.yaml")
}

func InitNetworkPolicyDefaultsYaml() string {
	return readYaml("yaml-templates/networkpolicies-default-ports.yaml")
}

func readYaml(filePath string) string {
	yaml, err := YamlTemplatesFolder.ReadFile(filePath)
	if err != nil {
		panic(err.Error())
	}
	return string(yaml)
}
