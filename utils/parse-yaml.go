package utils

import (
	"os"

	v1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/kubectl/pkg/scheme"
)

func InitPersistentVolume() core.PersistentVolume {
	pwd, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}
	path := pwd + "/yaml-templates/volume-nfs-pv.yaml"

	yaml, err := os.ReadFile(path)
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
	pwd, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}
	path := pwd + "/yaml-templates/volumeclaim-nfs-pvc.yaml"

	yaml, err := os.ReadFile(path)
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
	pwd, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}
	path := pwd + "/yaml-templates/container-secret.yaml"

	yaml, err := os.ReadFile(path)
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

func InitSecret() core.Secret {
	pwd, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}
	path := pwd + "/yaml-templates/secret.yaml"

	yaml, err := os.ReadFile(path)
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

func InitDeployment() v1.Deployment {
	pwd, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}
	path := pwd + "/yaml-templates/deployment.yaml"

	yaml, err := os.ReadFile(path)
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
	pwd, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}
	path := pwd + "/yaml-templates/ingress.yaml"

	yaml, err := os.ReadFile(path)
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
	pwd, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}
	path := pwd + "/yaml-templates/network-policy-namespace.yaml"

	yaml, err := os.ReadFile(path)
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
	pwd, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}
	path := pwd + "/yaml-templates/network-policy-service.yaml"

	yaml, err := os.ReadFile(path)
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
	pwd, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}
	path := pwd + "/yaml-templates/service.yaml"

	yaml, err := os.ReadFile(path)
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
