package utils

import (
	"embed"
	"fmt"
	"io/ioutil"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/version"
	"net"
	"net/http"
	"os"

	jsoniter "github.com/json-iterator/go"
)

const APP_NAME = "k8s"

var YamlTemplatesFolder embed.FS

type NamespaceDisplayName struct {
	DisplayName string `json:"displayName"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Volume struct {
	Namespace  NamespaceDisplayName `json:"namespace"`
	VolumeName string               `json:"volumeName"`
	SizeInGb   int                  `json:"sizeInGb"`
}

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

func GetVolumeMountsForK8sManager() ([]Volume, error) {
	result := []Volume{}

	// Create an http client
	client := &http.Client{}

	// Create a new request using http
	url := fmt.Sprintf("%s/storage/k8s/cluster-project-storage/list", CONFIG.ApiServer.Http_Server)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return result, err
	}

	// Add headers to the http request
	req.Header = HttpHeader("")
	// TODO: REMOVE - THIS IS JUST FOR DEBUGGING
	// if CONFIG.Misc.Debug && CONFIG.Misc.Stage == "local" {
	// 	req.Header["x-authorization"] = []string{"mo_7bf5c2b5-d7bc-4f0e-b8fc-b29d09108928_0hkga6vjum3p1mvezith"}
	// 	req.Header["x-cluster-mfa-id"] = []string{"a141bd85-c986-402c-9475-5bdc4679293b"}
	// }

	// Send the request and get a response
	resp, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(body, &result)
	return result, err
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

