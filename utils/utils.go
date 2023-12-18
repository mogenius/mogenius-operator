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
)

const APP_NAME = "k8s"

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

func StorageClassForClusterProvider(clusterProvider string) string {
	var nfsStorageClassStr string = "default"
	// TODO: "DOCKER_ENTERPRISE", "DOKS", "LINODE", "IBM", "ACK", "OKE", "OPEN_SHIFT"
	switch clusterProvider {
	case "EKS":
		nfsStorageClassStr = "gp2"
	case "GKE":
		nfsStorageClassStr = "standard-rwo"
	case "AKS":
		nfsStorageClassStr = "default"
	case "OTC":
		nfsStorageClassStr = "csi-disk"
	case "BRING_YOUR_OWN":
		nfsStorageClassStr = "default"
	case "DOCKER_DESKTOP":
		nfsStorageClassStr = "hostpath"
	case "K3S":
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
