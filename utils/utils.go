package utils

import (
	"embed"
	"fmt"
	"io"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/version"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	punqStructs "github.com/mogenius/punq/structs"
	"github.com/mogenius/punq/utils"
	"gopkg.in/yaml.v2"
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

func ExecuteShellCommandSilent(title string, shellCmd string) error {
	result, err := utils.RunOnLocalShell(shellCmd).Output()
	fmt.Printf("ExecuteShellCommandSilent:\n%s\n", result)
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode := exitErr.ExitCode()
		errorMsg := string(exitErr.Stderr)
		return fmt.Errorf("%d: %s", exitCode, errorMsg)
	} else if err != nil {
		return err
	} else {
		return nil
	}
}

func GetVersionData() (*punqStructs.HelmData, error) {
	response, err := http.Get(CONFIG.Misc.HelmIndex)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	data, _ := io.ReadAll(response.Body)
	var helmData punqStructs.HelmData
	err = yaml.Unmarshal(data, &helmData)
	if err != nil {
		return nil, err
	}
	return &helmData, nil
}
