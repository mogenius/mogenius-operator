package utils

import (
	"mogenius-k8s-manager/shutdown"

	v1 "k8s.io/api/apps/v1"
	v1job "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/kubectl/pkg/scheme"
)

func InitUpgradeConfigMap() corev1.ConfigMap {
	file := "yaml-templates/upgrade-configmap.yaml"
	yaml := readYaml(file)

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app corev1.ConfigMap
	_, _, err := s.Decode([]byte(yaml), nil, &app)
	if err != nil {
		utilsLogger.Error("failed to decode configmap", "file", file, "error", err)
		shutdown.SendShutdownSignalAndBlockForever(true)
		panic("unreachable")
	}
	return app
}

func InitUpgradeConfigMapYaml() string {
	return readYaml("yaml-templates/upgrade-configmap.yaml")
}

func InitUpgradeJob() v1job.Job {
	file := "yaml-templates/upgrade-job.yaml"
	yaml := readYaml(file)

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app v1job.Job
	_, _, err := s.Decode([]byte(yaml), nil, &app)
	if err != nil {
		utilsLogger.Error("failed to decode job", "file", file, "error", err)
		shutdown.SendShutdownSignalAndBlockForever(true)
		panic("unreachable")
	}
	return app
}

func InitMogeniusNfsPersistentVolumeClaim() corev1.PersistentVolumeClaim {
	file := "yaml-templates/mo-storage-nfs-pvc.yaml"
	yaml := readYaml(file)

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app corev1.PersistentVolumeClaim
	_, _, err := s.Decode([]byte(yaml), nil, &app)
	if err != nil {
		utilsLogger.Error("failed to decode pvc", "file", file, "error", err)
		shutdown.SendShutdownSignalAndBlockForever(true)
		panic("unreachable")
	}
	return app
}

func InitMogeniusNfsPersistentVolumeClaimForService() corev1.PersistentVolumeClaim {
	file := "yaml-templates/mo-storage-service-pvc.yaml"
	yaml := readYaml(file)

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app corev1.PersistentVolumeClaim
	_, _, err := s.Decode([]byte(yaml), nil, &app)
	if err != nil {
		utilsLogger.Error("failed to decode pvc", "file", file, "error", err)
		shutdown.SendShutdownSignalAndBlockForever(true)
		panic("unreachable")
	}
	return app
}

func InitMogeniusNfsPersistentVolumeForService() corev1.PersistentVolume {
	file := "yaml-templates/mo-storage-service-pv.yaml"
	yaml := readYaml(file)

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app corev1.PersistentVolume
	_, _, err := s.Decode([]byte(yaml), nil, &app)
	if err != nil {
		utilsLogger.Error("failed to decode pvc", "file", file, "error", err)
		shutdown.SendShutdownSignalAndBlockForever(true)
		panic("unreachable")
	}
	return app
}

func InitMogeniusNfsDeployment() v1.Deployment {
	file := "yaml-templates/mo-storage-nfs-deployment.yaml"
	yaml := readYaml(file)

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app v1.Deployment
	_, _, err := s.Decode([]byte(yaml), nil, &app)
	if err != nil {
		utilsLogger.Error("failed to decode deployment", "file", file, "error", err)
		shutdown.SendShutdownSignalAndBlockForever(true)
		panic("unreachable")
	}
	return app
}

func InitMogeniusNfsService() corev1.Service {
	file := "yaml-templates/mo-storage-nfs-service.yaml"
	yaml := readYaml(file)

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var service corev1.Service
	_, _, err := s.Decode([]byte(yaml), nil, &service)
	if err != nil {
		utilsLogger.Error("failed to decode service", "file", file, "error", err)
		shutdown.SendShutdownSignalAndBlockForever(true)
		panic("unreachable")
	}
	return service
}

func InitMogeniusContainerRegistryIngress() netv1.Ingress {
	file := "yaml-templates/mo-container-registry-ingress.yaml"
	yaml := readYaml("yaml-templates/mo-container-registry-ingress.yaml")

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var ingress netv1.Ingress
	_, _, err := s.Decode([]byte(yaml), nil, &ingress)
	if err != nil {
		utilsLogger.Error("failed to decode ingress", "file", file, "error", err)
		shutdown.SendShutdownSignalAndBlockForever(true)
		panic("unreachable")
	}
	return ingress
}

func InitMogeniusContainerRegistrySecret(crt string, key string) corev1.Secret {
	file := "yaml-templates/mo-container-registry-tls-secret.yaml"
	yaml := readYaml(file)

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var secret corev1.Secret
	_, _, err := s.Decode([]byte(yaml), nil, &secret)
	if err != nil {
		utilsLogger.Error("failed to decode", "file", file, "error", err)
		shutdown.SendShutdownSignalAndBlockForever(true)
		panic("unreachable")
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

func InitResourceTemplatesYaml() string {
	return readYaml("yaml-templates/resource-templates-configmap.yaml")
}

func readYaml(filePath string) string {
	yaml, err := YamlTemplatesFolder.ReadFile(filePath)
	if err != nil {
		utilsLogger.Error("failed to read embedded file from YamlTemplatesFolder", "error", err)
		shutdown.SendShutdownSignalAndBlockForever(true)
		panic("unreachable")
	}
	return string(yaml)
}
