package utils

import (
	"embed"
	"fmt"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/version"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"

	punqDtos "github.com/mogenius/punq/dtos"
)

const APP_NAME = "k8s"
const MOGENIUS_CONFIGMAP_DEFAULT_APPS_NAME = "mogenius-k8s-manager-default-apps"
const MOGENIUS_CONFIGMAP_DEFAULT_DEPLOYMENT_NAME = "mogenius-k8s-manager-default-deployment"

var YamlTemplatesFolder embed.FS

func MountPath(namespaceName string, volumeName string, defaultReturnValue string) string {
	if CONFIG.Kubernetes.RunInCluster {
		return fmt.Sprintf("%s/%s_%s", CONFIG.Misc.DefaultMountPath, namespaceName, volumeName)
	} else {
		pwd, err := os.Getwd()
		pwd += "/temp"
		if err != nil {
			logger.Log.Errorf("StatsMogeniusNfsVolume PWD Err: %s", err.Error())
		} else {
			return pwd
		}
	}
	return defaultReturnValue
}

func StorageClassForClusterProvider(clusterProvider punqDtos.KubernetesProvider) string {
	var nfsStorageClassStr string = "default"

	switch clusterProvider {
	case punqDtos.EKS:
		nfsStorageClassStr = "gp2"
	case punqDtos.GKE:
		nfsStorageClassStr = "standard-rwo"
	case punqDtos.AKS:
		nfsStorageClassStr = "default"
	case punqDtos.OTC:
		nfsStorageClassStr = "csi-disk"
	case punqDtos.BRING_YOUR_OWN:
		nfsStorageClassStr = "default"
	case punqDtos.DOCKER_DESKTOP, punqDtos.KIND:
		nfsStorageClassStr = "hostpath"
	case punqDtos.K3S:
		nfsStorageClassStr = "local-path"
	default:
		logger.Log.Errorf("CLUSTERPROVIDER '%s' HAS NOT BEEN TESTED YET! Returning 'default'.", clusterProvider)
		nfsStorageClassStr = "default"
	}

	return nfsStorageClassStr
}

func HttpHeader(additionalName string) http.Header {
	return http.Header{
		"x-authorization":  []string{CONFIG.Kubernetes.ApiKey},
		"x-cluster-mfa-id": []string{CONFIG.Kubernetes.ClusterMfaId},
		"x-app":            []string{fmt.Sprintf("%s%s", APP_NAME, additionalName)},
		"x-app-version":    []string{version.Ver},
		"x-cluster-name":   []string{CONFIG.Kubernetes.ClusterName}}
}

// parseIPs parses a slice of IP address strings into a slice of net.IP.
func parseIPs(ips []string) ([]net.IP, error) {
	var parsed []net.IP
	for _, ip := range ips {
		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			return nil, fmt.Errorf("invalid IP address: %s", ip)
		}
		parsed = append(parsed, parsedIP.To4())
	}
	return parsed, nil
}

func Prepend[T any](s []T, values ...T) []T {
	return append(values, s...)
}

func GetFunctionName() string {
	pc, _, _, _ := runtime.Caller(1)
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}

	// Split the name to get only the function name without the package path
	parts := strings.Split(fn.Name(), ".")
	return parts[len(parts)-1]
}
