package utils

import (
	"mogenius-k8s-manager/src/shutdown"

	v1 "k8s.io/api/apps/v1"
	v1job "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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
		shutdown.SendShutdownSignal(true)
		select {}
	}
	return app
}

func InitUpgradeJob() v1job.Job {
	file := "yaml-templates/upgrade-job.yaml"
	yaml := readYaml(file)

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app v1job.Job
	_, _, err := s.Decode([]byte(yaml), nil, &app)
	if err != nil {
		utilsLogger.Error("failed to decode job", "file", file, "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
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
		shutdown.SendShutdownSignal(true)
		select {}
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
		shutdown.SendShutdownSignal(true)
		select {}
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
		shutdown.SendShutdownSignal(true)
		select {}
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
		shutdown.SendShutdownSignal(true)
		select {}
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
		shutdown.SendShutdownSignal(true)
		select {}
	}
	return service
}

func InitSecret() corev1.Secret {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/secret.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app corev1.Secret
	_, _, err = s.Decode(yaml, nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitResourceTemplatesYaml() string {
	return readYaml("yaml-templates/resource-templates-configmap.yaml")
}

func readYaml(filePath string) string {
	yaml, err := YamlTemplatesFolder.ReadFile(filePath)
	if err != nil {
		utilsLogger.Error("failed to read embedded file from YamlTemplatesFolder", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	}
	return string(yaml)
}

func IndexHtml() string {
	html, err := HtmlFolder.ReadFile("html/index.html")
	if err != nil {
		utilsLogger.Error("failed to read embedded file from HtmlFolder", "error", err)
	}
	return string(html)
}

func NodeStatsHtml() string {
	html, err := HtmlFolder.ReadFile("html/node-stats.html")
	if err != nil {
		utilsLogger.Error("failed to read embedded file from HtmlFolder", "error", err)
	}
	return string(html)
}
