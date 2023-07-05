package utils

import (
	v1 "k8s.io/api/apps/v1"
	v1job "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/kubectl/pkg/scheme"
)

func InitPersistentVolume() core.PersistentVolume {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/volume-nfs-pv.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app core.PersistentVolume
	_, _, err = s.Decode(yaml, nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitPersistentVolumeClaim() core.PersistentVolumeClaim {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/volumeclaim-cephfs.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app core.PersistentVolumeClaim
	_, _, err = s.Decode(yaml, nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitContainerSecret() core.Secret {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/container-secret.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app core.Secret
	_, _, err = s.Decode(yaml, nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

// func InitNfsStorageClassMogenius() storage.StorageClass {
// 	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/mo-nfs-storageclass.yaml")
// 	if err != nil {
// 		panic(err.Error())
// 	}

// 	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

// 	var app storage.StorageClass
// 	_, _, err = s.Decode(yaml, nil, &app)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return app
// }

func InitMogeniusNfsPersistentVolumeClaim() core.PersistentVolumeClaim {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/mo-storage-nfs-pvc.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app core.PersistentVolumeClaim
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

func InitMogeniusNfsService() core.Service {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/mo-storage-nfs-service.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var service core.Service
	_, _, err = s.Decode(yaml, nil, &service)
	if err != nil {
		panic(err)
	}
	return service
}

func InitMogeniusNfsK8sService() core.Service {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/mo-storage-k8s-nfs-service.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var service core.Service
	_, _, err = s.Decode(yaml, nil, &service)
	if err != nil {
		panic(err)
	}
	return service
}

func InitSecret() core.Secret {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/secret.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app core.Secret
	_, _, err = s.Decode(yaml, nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitConfigMap() core.ConfigMap {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/configmap.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app core.ConfigMap
	_, _, err = s.Decode(yaml, nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitDeployment() v1.Deployment {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/deployment.yaml")
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

func InitIngress() netv1.Ingress {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/ingress.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app netv1.Ingress
	_, _, err = s.Decode(yaml, nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitNetPolNamespace() netv1.NetworkPolicy {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/network-policy-namespace.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app netv1.NetworkPolicy
	_, _, err = s.Decode(yaml, nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitNetPolService() netv1.NetworkPolicy {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/network-policy-service.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app netv1.NetworkPolicy
	_, _, err = s.Decode(yaml, nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitService() core.Service {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/service.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app core.Service
	_, _, err = s.Decode(yaml, nil, &app)
	if err != nil {
		panic(err)
	}
	return app
}

func InitUpgradeConfigMap() core.ConfigMap {
	yaml, err := YamlTemplatesFolder.ReadFile("yaml-templates/upgrade-configmap.yaml")
	if err != nil {
		panic(err.Error())
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var app core.ConfigMap
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
