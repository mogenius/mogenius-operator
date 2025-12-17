package utils

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"mogenius-operator/src/assert"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/secrets"
	"mogenius-operator/src/version"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/go-playground/validator/v10"
	"github.com/patrickmn/go-cache"
)

var CURRENT_COUNTRY *CountryDetails

var config cfg.ConfigModule
var utilsLogger *slog.Logger
var validate = validator.New(validator.WithRequiredStructEnabled())

func Setup(logManagerModule logging.SlogManager, configModule cfg.ConfigModule) {
	utilsLogger = logManagerModule.CreateLogger("utils")
	config = configModule

	validate = validator.New(validator.WithRequiredStructEnabled())
}

const APP_NAME = "k8s"
const MOGENIUS_CONFIGMAP_DEFAULT_APPS_NAME = "mogenius-operator-default-apps"
const MOGENIUS_CONFIGMAP_DEFAULT_DEPLOYMENT_NAME = "mogenius-operator-default-deployment"

const MAX_NAME_LENGTH = 253

type Release struct {
	TagName    string `json:"tag_name"`
	Published  string `json:"published_at"`
	Prerelease bool   `json:"prerelease"`
}

type CountryDetails struct {
	Code              string   `json:"code"`
	Code3             string   `json:"code3"`
	IsoID             int      `json:"isoId"`
	Name              string   `json:"name"`
	Currency          string   `json:"currency"`
	CurrencyName      string   `json:"currencyName"`
	TaxPercent        float64  `json:"taxPercent"`
	Continent         string   `json:"continent"`
	CapitalCity       string   `json:"capitalCity"`
	CapitalCityLat    float64  `json:"capitalCityLat"`
	CapitalCityLng    float64  `json:"capitalCityLng"`
	IsEuMember        bool     `json:"isEuMember"`
	PhoneNumberPrefix string   `json:"phoneNumberPrefix"`
	DomainTld         string   `json:"domainTld"`
	Languages         []string `json:"languages"`
	IsActive          bool     `json:"isActive"`
}

const (
	HelmReleaseNameMetricsServer        = "metrics-server"
	HelmReleaseNameTraefik              = "traefik"
	HelmReleaseNameCertManager          = "cert-manager"
	HelmReleaseNameClusterIssuer        = "clusterissuer"
	HelmReleaseNameDistributionRegistry = "distribution-registry"
	HelmReleaseNameExternalSecrets      = "external-secrets"
	HelmReleaseNameMetalLb              = "metallb"
	HelmReleaseNameKepler               = "kepler"
)

// IacSecurity is an enum type for the different security treatments that can be applied to IaC data.
type IacSecurity string

const (
	IacSecurityNeedsNothing                     IacSecurity = "Nothing"                  // No encryption or decryption needed
	IacSecurityNeedsDecryption                  IacSecurity = "Decrypt"                  // Decrypt the data field values
	IacSecurityNeedsEncryption                  IacSecurity = "Encrypt"                  // Encrypt the data field values
	IacSecurityNeedsEncryptionButStateIsUnknown IacSecurity = "EncryptButStateIsUnknown" // Encrypt the data field values
)

// this includes the yaml-templates & html folder into the binary
//
//go:embed yaml-templates
var YamlTemplatesFolder embed.FS

//go:embed html
var HtmlFolder embed.FS

var helmDataVersion = cache.New(2*time.Hour, 30*time.Minute)

func MountPath(namespaceName string, volumeName string, defaultReturnValue string, runsInCluster bool) string {
	if runsInCluster {
		return fmt.Sprintf("%s/%s_%s", config.Get("MO_DEFAULT_MOUNT_PATH"), namespaceName, volumeName)
	} else {
		pwd, err := os.Getwd()
		pwd += "/temp"
		if err != nil {
			utilsLogger.Error("StatsMogeniusNfsVolume PWD", "error", err)
		} else {
			return pwd
		}
	}
	return defaultReturnValue
}

func HttpHeader(additionalName string) http.Header {
	return http.Header{
		"x-authorization":  []string{config.Get("MO_API_KEY")},
		"x-cluster-mfa-id": []string{config.Get("MO_CLUSTER_MFA_ID")},
		"x-app":            []string{fmt.Sprintf("%s%s", APP_NAME, additionalName)},
		"x-app-version":    []string{version.Ver},
		"x-cluster-name":   []string{config.Get("MO_CLUSTER_NAME")}}
}

func ExecuteShellCommandSilent(title string, shellCmd string) error {
	result, err := RunOnLocalShell(shellCmd).Output()
	utilsLogger.Debug("ExecuteShellCommandSilent", "command", shellCmd, "result", result)
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

// // MeasureTime measures the execution time of a function and prints it in milliseconds.
// func MeasureTime(name string, fn func()) {
// 	start := time.Now()
// 	fn()
// 	elapsed := time.Since(start)
// 	log.Infof("%s took %s", name, elapsed)
// }

func GetVersionData(url string) (*HelmData, error) {
	// Check if the data is already in the cache
	if cachedData, found := helmDataVersion.Get(url); found {
		return cachedData.(*HelmData), nil
	}

	// If not in cache, fetch the data from the URL
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var helmData HelmData
	err = yaml.Unmarshal(data, &helmData)
	if err != nil {
		return nil, err
	}

	// Store the fetched data in the cache
	helmDataVersion.Set(url, &helmData, 2*time.Hour)

	return &helmData, nil
}

const (
	SecretListSuffix      = "vault-secret-list"
	SecretStoreSuffix     = "vault-secret-store"
	ExternalSecretsSA     = "mo-eso-serviceaccount"
	StoreAnnotationPrefix = "used-by-mogenius/"
)

func PrettyPrintInterface(i any) string {
	str := PrettyPrintString(i)
	return RedactString(str)
}

func RedactString(targetSring string) string {
	for _, secret := range secrets.SecretArray() {
		targetSring = strings.ReplaceAll(targetSring, secret, secrets.REDACTED)
	}
	return targetSring
}

func PrettyPrintString(i any) string {
	iJson, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		utilsLogger.Error(err.Error())
	}
	return string(iJson)
}

func PrintJson(i any) string {
	data, err := json.Marshal(i)
	if err != nil {
		utilsLogger.Error(err.Error())
	}
	return string(data)
}

type ResponseError struct {
	Error string `json:"error,omitempty"`
}

func CreateError(err error) ResponseError {
	return ResponseError{
		Error: err.Error(),
	}
}

func RunOnLocalShell(cmd string) *exec.Cmd {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("/bin/sh", "-c", cmd)
	case "windows":
		return exec.Command("cmd", "/C", cmd)
	case "darwin":
		return exec.Command("/bin/zsh", "-c", cmd)
	default:
		return exec.Command("/bin/sh", "-c", cmd)
	}
}

func ExecuteShellCommandWithResponse(title string, shellCmd string) string {
	var err error
	var returnStr []byte
	returnStr, err = RunOnLocalShell(shellCmd).Output()
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode := exitErr.ExitCode()
		errorMsg := string(exitErr.Stderr)
		utilsLogger.Error(shellCmd)
		utilsLogger.Error("Exitcode and errormsg", "exitCode", exitCode, "error", errorMsg)
		return errorMsg
	} else if err != nil {
		utilsLogger.Error("ERROR", "title", title, "error", err.Error())
		return err.Error()
	}
	return string(returnStr)
}

func MergeMaps(maplist ...map[string]string) map[string]string {
	resultMap := make(map[string]string)

	// Iterate over the slice of maps
	for _, m := range maplist {
		// Add all elements from each map, potentially overwriting
		maps.Copy(resultMap, m)
	}

	return resultMap
}

func GuessClusterCountry() (*CountryDetails, error) {
	if CURRENT_COUNTRY != nil {
		return CURRENT_COUNTRY, nil
	}

	allowCountryCheck, err := strconv.ParseBool(config.Get("MO_ALLOW_COUNTRY_CHECK"))
	assert.Assert(err == nil, err)
	if allowCountryCheck {
		resp, err := http.Get("https://platform-api.mogenius.com/country/location")
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("failed to fetch with status code: %d", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var country CountryDetails
		if err := json.Unmarshal(body, &country); err != nil {
			return CURRENT_COUNTRY, err
		} else {
			CURRENT_COUNTRY = &country
			return CURRENT_COUNTRY, err
		}
	}
	return nil, nil
}

func Remove[T any](slice []T, s int) []T {
	return append(slice[:s], slice[s+1:]...)
}

func CheckInternetAccess() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a custom resolver
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "udp", "1.1.1.1:53")
		},
	}

	// Attempt to resolve a known domain
	// If it succeeds, it means we have internet access
	_, err := r.LookupHost(ctx, "mogenius.com")
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return false, ctx.Err()
		}
		// Other errors
		return false, err
	}

	// success
	return true, nil
}

func CreateDirIfNotExist(dir string) {
	_, err := os.Stat(dir)

	// If directory does not exist create it
	if os.IsNotExist(err) {
		errDir := os.MkdirAll(dir, 0755)
		if errDir != nil {
			utilsLogger.Error(err.Error())
		}
	}
}

func DeleteDirIfExist(dir string) {
	_, err := os.Stat(dir)

	// If directory does not exist create it
	if os.IsExist(err) {
		errDir := os.RemoveAll(dir)
		if errDir != nil {
			utilsLogger.Error(err.Error())
		}
	}
}

func ContainsResourceDescriptor(resources []*ResourceDescriptor, target ResourceDescriptor) bool {
	for _, r := range resources {
		if r.Kind == target.Kind && r.ApiVersion == target.ApiVersion {
			return true
		}
	}
	return false
}
