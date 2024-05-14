package utils

import (
	"embed"
	"fmt"
	"io"
	"mogenius-k8s-manager/version"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	punqStructs "github.com/mogenius/punq/structs"
	"github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

const IMAGE_PLACEHOLDER = "PLACEHOLDER-UNTIL-BUILDSERVER-OVERWRITES-THIS-IMAGE"

const APP_NAME = "k8s"
const MOGENIUS_CONFIGMAP_DEFAULT_APPS_NAME = "mogenius-k8s-manager-default-apps"
const MOGENIUS_CONFIGMAP_DEFAULT_DEPLOYMENT_NAME = "mogenius-k8s-manager-default-deployment"

const (
	HelmReleaseNameTrafficCollector     = "mogenius-traffic-collector"
	HelmReleaseNamePodStatsCollector    = "mogenius-pod-stats-collector"
	HelmReleaseNameMetricsServer        = "metrics-server"
	HelmReleaseNameTraefik              = "traefik"
	HelmReleaseNameCertManager          = "cert-manager"
	HelmReleaseNameClusterIssuer        = "clusterissuer"
	HelmReleaseNameDistributionRegistry = "distribution-registry"
	HelmReleaseNameMetalLb              = "metallb"
	HelmReleaseNameKepler               = "kepler"
)

var YamlTemplatesFolder embed.FS

func MountPath(namespaceName string, volumeName string, defaultReturnValue string) string {
	if CONFIG.Kubernetes.RunInCluster {
		return fmt.Sprintf("%s/%s_%s", CONFIG.Misc.DefaultMountPath, namespaceName, volumeName)
	} else {
		pwd, err := os.Getwd()
		pwd += "/temp"
		if err != nil {
			log.Errorf("StatsMogeniusNfsVolume PWD Err: %s", err.Error())
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
// func parseIPs(ips []string) ([]net.IP, error) {
// 	var parsed []net.IP
// 	for _, ip := range ips {
// 		parsedIP := net.ParseIP(ip)
// 		if parsedIP == nil {
// 			return nil, fmt.Errorf("invalid IP address: %s", ip)
// 		}
// 		parsed = append(parsed, parsedIP.To4())
// 	}
// 	return parsed, nil
// }

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
	log.Infof("ExecuteShellCommandSilent: %s:\n%s", shellCmd, result)
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

func ExecuteShellCommandRealySilent(title string, shellCmd string) error {
	result, err := utils.RunOnLocalShell(shellCmd).Output()
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode := exitErr.ExitCode()
		errorMsg := string(exitErr.Stderr)
		return fmt.Errorf("%d: %s %s", exitCode, errorMsg, string(result))
	} else if err != nil {
		return err
	} else {
		return nil
	}
}

func GetVersionData(url string) (*punqStructs.HelmData, error) {
	response, err := http.Get(url)
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

func SequenceToKey(id uint64) []byte {
	return []byte(fmt.Sprintf("%020d", id))
}

func GitCommitLink(gitRepository string, commitHash string) *string {
	u, err := url.Parse(gitRepository)
	if err != nil {
		return nil
	}

	// remove the user from the URL
	u.User = nil
	// without authentication
	cleanedURL := u.String()
	baseRepoURL := strings.TrimSuffix(cleanedURL, ".git")

	var commitURL string
	switch {
	case strings.Contains(u.Host, "github.com"):
		commitURL = fmt.Sprintf("%s/commit/%s", baseRepoURL, commitHash)
	case strings.Contains(u.Host, "dev.azure.com"):
		commitURL = fmt.Sprintf("%s/_git/%s/commit/%s", baseRepoURL, u.Path, commitHash)
	default:
		commitURL = fmt.Sprintf("%s/-/commit/%s", baseRepoURL, commitHash)
	}
	return &commitURL
}

func ContainsUint64(slice []uint64, value uint64) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
func ContainsString(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

func IsFirstTimestampNewer(ts1, ts2 string) bool {
	// Parse the timestamps using RFC 3339 format
	t1, err := time.Parse(time.RFC3339, ts1)
	if err != nil {
		log.Error(fmt.Errorf("error parsing ts1: %w", err))
	}

	t2, err := time.Parse(time.RFC3339, ts2)
	if err != nil {
		log.Error(fmt.Errorf("error parsing ts2: %w", err))
	}

	// Check if the first timestamp is strictly newer than the second
	return t1.After(t2)
}
