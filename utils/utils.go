package utils

import (
	"bufio"
	"embed"
	"fmt"
	"log"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/version"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

const APP_NAME = "k8s"

var YamlTemplatesFolder embed.FS

func Pointer[K any](val K) *K {
	return &val
}

type ResponseError struct {
	Error string `json:"error,omitempty"`
}

func CreateError(err error) ResponseError {
	return ResponseError{
		Error: err.Error(),
	}
}

func Contains(s []string, str string) bool {
	for _, v := range s {
		if strings.Contains(str, v) {
			return true
		}
	}
	return false
}

func Diff(a []string, b []string) []string {
	diff := make([]string, 0)

	if len(a) != len(b) {
		return a
	}

	// Create a map to store the count of each string in array 'a'
	countMap := make(map[string]int)
	for _, str := range a {
		countMap[str]++
	}

	// Check if all strings in array 'b' are present in the map
	for _, str := range b {
		count, ok := countMap[str]
		if !ok || count == 0 {
			diff = append(diff, str)
		} else {
			countMap[str]--
		}
	}

	// Add any remaining items in countMap to the diff slice
	for str, count := range countMap {
		if count > 0 {
			diff = append(diff, str)
		}
	}

	return diff
}

func ContainsInt(v int, a []int) bool {
	for _, i := range a {
		if i == v {
			return true
		}
	}
	return false
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
	default:
		logger.Log.Errorf("CLUSTERPROVIDER '%s' HAS NOT BEEN TESTED YET! Returning 'default'.", clusterProvider)
		nfsStorageClassStr = "default"
	}
	return nfsStorageClassStr
}

func OpenBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	if err != nil {
		fmt.Errorf("error while opening browser, %v", err)
	}
}

func ConfirmTask(s string, tries int) bool {
	r := bufio.NewReader(os.Stdin)

	for ; tries > 0; tries-- {
		fmt.Printf("%s [y/n]: ", s)

		res, err := r.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}

		// Empty input (i.e. "\n")
		if len(res) < 2 {
			continue
		}

		return strings.ToLower(strings.TrimSpace(res))[0] == 'y'
	}

	return false
}

func FillWith(s string, targetLength int, chars string) string {
	if len(s) >= targetLength {
		return TruncateText(s, targetLength)
	}
	for i := 0; len(s) < targetLength; i++ {
		s = s + chars
	}

	return s
}

func TruncateText(s string, max int) string {
	if max < 4 || max > len(s) {
		return s
	}
	return s[:max-4] + " ..."
}

func FunctionName() string {
	counter, _, _, success := runtime.Caller(1)

	if !success {
		println("functionName: runtime.Caller: failed")
		os.Exit(1)
	}

	return runtime.FuncForPC(counter).Name()
}

func ParseJsonStringArray(input string) []string {
	val := []string{}
	var jsonOnSteroids = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := jsonOnSteroids.Unmarshal([]byte(input), &val); err != nil {
		logger.Log.Errorf("jsonStringArrayToStringArray: Failed to parse: '%s' to []string.", input)
	}
	return val
}

func Remove[T any](slice []T, s int) []T {
	return append(slice[:s], slice[s+1:]...)
}

func HttpHeader() http.Header {
	return http.Header{
		"x-authorization":  []string{CONFIG.Kubernetes.ApiKey},
		"x-cluster-mfa-id": []string{CONFIG.Kubernetes.ClusterMfaId},
		"x-app":            []string{APP_NAME},
		"x-app-version":    []string{version.Ver},
		"x-cluster-name":   []string{CONFIG.Kubernetes.ClusterName}}
}
