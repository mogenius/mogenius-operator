package kubernetes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var RunsInCluster bool = false

type KubeProvider struct {
	ClientSet     *kubernetes.Clientset
	DynamicClient *dynamic.DynamicClient
	ClientConfig  rest.Config
}

func NewKubeProvider(contextId *string) (*KubeProvider, error) {
	var provider *KubeProvider
	var err error
	if RunsInCluster {
		provider, err = newKubeProviderInCluster(contextId)
	} else {
		provider, err = newKubeProviderLocal(contextId)
	}

	if err != nil {
		K8sLogger.Errorf("ERROR: %s", err.Error())
	}
	return provider, err
}

func newKubeProviderLocal(contextId *string) (*KubeProvider, error) {
	config, err := ContextSwitcher(contextId)
	if err != nil {
		return nil, err
	}

	clientSet, errClientSet := kubernetes.NewForConfig(config)
	if errClientSet != nil {
		return nil, errClientSet
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &KubeProvider{
		ClientSet:     clientSet,
		DynamicClient: dynamicClient,
		ClientConfig:  *config,
	}, nil
}

func newKubeProviderInCluster(contextId *string) (*KubeProvider, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	if contextId != nil {
		config, err = ContextSwitcher(contextId)
		if err != nil {
			return nil, err
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &KubeProvider{
		ClientSet:     clientset,
		DynamicClient: dynamicClient,
		ClientConfig:  *config,
	}, nil
}

func ContextSwitcher(contextId *string) (*rest.Config, error) {
	if contextId != nil && *contextId != "" {
		return ContextConfigLoader(contextId)
	} else {
		var kubeconfigs []string = GetDefaultKubeConfig()
		var config *rest.Config
		var err error
		for _, kubeconfig := range kubeconfigs {
			config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err == nil {
				return config, nil
			}
		}

		return nil, fmt.Errorf("Error loading kubeconfig: %s", err.Error())
	}
}

func ContextConfigLoader(contextId *string) (*rest.Config, error) {
	ctx := ContextForId(*contextId)
	if ctx == nil {
		return nil, fmt.Errorf("context not found for id: %s", *contextId)
	}

	configFromString, err := clientcmd.NewClientConfigFromBytes([]byte(ctx.Context))
	if err != nil {
		K8sLogger.Errorf("Error creating client config from string: %s", err.Error())
		return nil, err
	}

	config, err := configFromString.ClientConfig()
	return config, err
}

func ContextForId(id string) *K8sContext {
	for _, ctx := range allContexts {
		if ctx.Id == id {
			return &ctx
		}
	}
	return nil
}

type K8sContext struct {
	Id          string      `json:"id" validate:"required"`
	Name        string      `json:"name" validate:"required"`
	ContextHash string      `json:"contextHash" validate:"required"`
	Context     string      `json:"context" validate:"required"`
	Provider    string      `json:"provider" validate:"required"`
	Reachable   bool        `json:"reachable" validate:"required"`
	Users       []string    `json:"users" validate:"required"`
	AccessLevel AccessLevel `json:"accessLevel" validate:"required"`
}

type AccessLevel int

const (
	UNKNOWNACCESS AccessLevel = iota
	READER
	USER
	ADMIN
	// and so on...
)

var allContexts []K8sContext = []K8sContext{}

func GetDefaultKubeConfig() []string {
	var kubeconfig string = os.Getenv("KUBECONFIG")
	var kubeconfigs []string

	if kubeconfig == "" {
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
			kubeconfigs = []string{kubeconfig}
		}
	} else {
		kubeconfigs = strings.Split(kubeconfig, ":")
	}

	// Check that at least one kubeconfig file exists
	validConfigs := []string{}
	for _, singleConfig := range kubeconfigs {
		if _, err := os.Stat(singleConfig); os.IsNotExist(err) {
			K8sLogger.Warningf("Warning: kubeconfig file '%s' does not exist", singleConfig)
		} else {
			validConfigs = append(validConfigs, singleConfig)
		}
	}

	if len(validConfigs) == 0 {
		K8sLogger.Fatal("Error: No valid kubeconfig file found. Ensure that the $KUBECONFIG environment variable or the default kubeconfig is set correctly. For more info, see https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/.")
	}

	return validConfigs
}
