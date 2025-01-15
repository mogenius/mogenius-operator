package utils

import (
	"bufio"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/version"
	"net"
	"net/http"
	"net/url"
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
var validate *validator.Validate

func Setup(logManagerModule logging.LogManagerModule, configModule cfg.ConfigModule) {
	utilsLogger = logManagerModule.CreateLogger("utils")
	config = configModule

	validate = validator.New(validator.WithRequiredStructEnabled())
}

const IMAGE_PLACEHOLDER = "PLACEHOLDER-UNTIL-BUILDSERVER-OVERWRITES-THIS-IMAGE"

const APP_NAME = "k8s"
const MOGENIUS_CONFIGMAP_DEFAULT_APPS_NAME = "mogenius-k8s-manager-default-apps"
const MOGENIUS_CONFIGMAP_DEFAULT_DEPLOYMENT_NAME = "mogenius-k8s-manager-default-deployment"

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
	HelmReleaseNameTrafficCollector     = "mogenius-traffic-collector"
	HelmReleaseNamePodStatsCollector    = "mogenius-pod-stats-collector"
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

// this includes the yaml-templates folder into the binary
//
//go:embed yaml-templates
var YamlTemplatesFolder embed.FS

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

func ConfirmTask(s string) bool {
	r := bufio.NewReader(os.Stdin)

	fmt.Printf("%s [Y/n]: ", s)

	res, err := r.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}

	// Empty input (i.e. "\n")
	if res == "\n" {
		return true
	}

	return strings.ToLower(strings.TrimSpace(res)) == "y"
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

func ExecuteShellCommandRealySilent(title string, shellCmd string) error {
	result, err := RunOnLocalShell(shellCmd).Output()
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
	case strings.Contains(u.Host, "bitbucket.org"):
		commitURL = fmt.Sprintf("%s/_git/%s/commits/%s", baseRepoURL, u.Path, commitHash)
	default:
		commitURL = fmt.Sprintf("%s/-/commit/%s", baseRepoURL, commitHash)
	}
	return &commitURL
}

func ContainsString(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

func AppendIfNotExist(slice []string, str string) []string {
	for _, item := range slice {
		if item == str {
			return slice
		}
	}
	return append(slice, str)
}

func Escape(str string) string {
	var builder strings.Builder
	for _, char := range str {
		switch char {
		case '\'': // escape single quotes
			builder.WriteString("'\\''")
		case '"', '`', '\\', '$', '%', '&', '*', ';', '|', '<', '>', '?', '[', ']', '{', '}', '(', ')':
			builder.WriteString("\\" + string(char))
		case '\b':
			builder.WriteString("\\b")
		case '\f':
			builder.WriteString("\\f")
		case '\n':
			builder.WriteString("\\n")
		case '\r':
			builder.WriteString("\\r")
		case '\t':
			builder.WriteString("\\t")
		case '\u2028':
			builder.WriteString("\\u2028")
		case '\u2029':
			builder.WriteString("\\u2029")
		default:
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

const (
	SecretListSuffix      = "vault-secret-list"
	SecretStoreSuffix     = "vault-secret-store"
	ExternalSecretsSA     = "mo-eso-serviceaccount"
	StoreAnnotationPrefix = "used-by-mogenius/"
)

func GetServiceAccountName(role string) string {
	return fmt.Sprintf("%s-%s",
		strings.ToLower(ExternalSecretsSA),
		strings.ToLower(role),
	)
}

func GetSecretStoreName(namePrefix string) string {
	return fmt.Sprintf("%s-%s",
		strings.ToLower(namePrefix),
		strings.ToLower(SecretStoreSuffix),
	)
}

func GetSecretName(namePrefix, service, propertyName string) string {
	return fmt.Sprintf("%s-%s-%s",
		strings.ToLower(namePrefix),
		strings.ToLower(service),
		strings.ToLower(propertyName),
	)
}

func GetSecretListName(namePrefix string) string {
	return fmt.Sprintf("%s-%s",
		strings.ToLower(namePrefix),
		strings.ToLower(SecretListSuffix),
	)
}

func ParseK8sName(name string) string {
	if len(name) > MAX_NAME_LENGTH {
		name = name[:MAX_NAME_LENGTH]
	}
	return strings.ToLower(name)
}

func CleanYaml(data string, treatment IacSecurity) (string, error) {
	if data == "" {
		return "", nil
	}
	var dataMap map[string]interface{}
	err := yaml.Unmarshal([]byte(data), &dataMap)
	if err != nil {
		return "", fmt.Errorf("Error CleanYaml unmarshalling yaml: %s", err.Error())
	}

	dataMap, err = CleanObject(dataMap, treatment)
	if err != nil {
		return "", fmt.Errorf("Error cleaning yaml: %s", err.Error())
	}

	cleanedYaml, err := yaml.Marshal(dataMap)
	if err != nil {
		return "", fmt.Errorf("Error marshalling yaml: %s", err.Error())
	}
	return string(cleanedYaml), nil
}

func CleanObjectInterface(data interface{}, treatment IacSecurity) (interface{}, error) {
	dataInf := data.(map[string]interface{})
	dataInf, err := CleanObject(dataInf, treatment)
	return dataInf, err
}

func CleanObject(data map[string]interface{}, treatment IacSecurity) (map[string]interface{}, error) {
	removeFieldAtPath(data, "uid", []string{"metadata"}, []string{})
	removeFieldAtPath(data, "selfLink", []string{"metadata"}, []string{})
	removeFieldAtPath(data, "generation", []string{"metadata"}, []string{})
	removeFieldAtPath(data, "managedFields", []string{"metadata"}, []string{})
	removeFieldAtPath(data, "deployment.kubernetes.io/revision", []string{"annotations"}, []string{})
	removeFieldAtPath(data, "kubectl.kubernetes.io/last-applied-configuration", []string{"annotations"}, []string{})

	removeFieldAtPath(data, "creationTimestamp", []string{}, []string{})
	removeFieldAtPath(data, "resourceVersion", []string{}, []string{})
	removeFieldAtPath(data, "status", []string{}, []string{})

	// Remove Workload Specific Fields Which make no sens to be in the yaml for gitops
	switch expression := data["kind"]; expression {
	case "Service":
		removeFieldAtPath(data, "clusterIP", []string{"spec"}, []string{})
		removeFieldAtPath(data, "clusterIPs", []string{"spec"}, []string{})
	case "Secret":
		switch treatment {
		case IacSecurityNeedsEncryption:
			// Encrypt the data field values
			if data["data"] != nil {
				for k, v := range data["data"].(map[string]interface{}) {
					encryptStr, err := EncryptString(config.Get("MO_API_KEY"), v.(string))
					if err != nil {
						return nil, err
					}
					data["data"].(map[string]interface{})[k] = encryptStr
				}
			}
		case IacSecurityNeedsDecryption:
			// Encrypt the data field values
			if data["data"] != nil {
				for k, v := range data["data"].(map[string]interface{}) {
					decryptStr, err := DecryptString(config.Get("MO_API_KEY"), v.(string))
					if err != nil {
						return nil, err
					}
					data["data"].(map[string]interface{})[k] = decryptStr
				}
			}
		}
	}

	return data, nil
}

func EncryptSecretIfNecessary(filePath string) (changedFile bool, error error) {
	yamlData, err := os.ReadFile(filePath)
	if err != nil {
		return changedFile, err
	}

	var dataMap map[string]interface{}
	err = yaml.Unmarshal([]byte(yamlData), &dataMap)
	if err != nil {
		return changedFile, fmt.Errorf("Error CleanYaml unmarshalling yaml: %s", err.Error())
	}

	// Encrypt the data field values
	for k, v := range dataMap["data"].(map[string]interface{}) {
		isEncrypted := IsEncrypted(v.(string))
		if !isEncrypted {
			encryptStr, err := EncryptString(config.Get("MO_API_KEY"), v.(string))
			if err != nil {
				return false, err
			}
			dataMap["data"].(map[string]interface{})[k] = encryptStr
			changedFile = true
		}
		// do nothing if it is already encrypted
	}
	if !changedFile {
		return changedFile, nil
	}

	cleanedYaml, err := yaml.Marshal(dataMap)
	if err != nil {
		return false, fmt.Errorf("Error marshalling yaml: %s", err.Error())
	}

	// Write the cleaned yaml back to the file
	err = os.WriteFile(filePath, cleanedYaml, 0644)

	return changedFile, err
}

func removeFieldAtPath(data map[string]interface{}, field string, targetPath []string, currentPath []string) {
	// Check if the current path matches the target path for removal.
	if len(currentPath) >= len(targetPath) && strings.Join(currentPath[len(currentPath)-len(targetPath):], "/") == strings.Join(targetPath, "/") {
		delete(data, field)
	}
	// Continue searching within the map.
	for key, value := range data {
		switch v := value.(type) {
		case map[string]interface{}:
			removeFieldAtPath(v, field, targetPath, append(currentPath, key))
			// After processing the nested map, check if it's empty and remove it if so.
			if len(v) == 0 {
				delete(data, key)
			}
		case []interface{}:
			for i, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					// Construct a new path for each item in the list.
					newPath := append(currentPath, fmt.Sprintf("%s[%d]", key, i))
					removeFieldAtPath(itemMap, field, targetPath, newPath)
				}
			}
			// Clean up the slice if it becomes empty after deletion.
			if len(v) == 0 {
				delete(data, key)
			}
		default:
			// Check and delete empty values here.
			if isEmptyValue(value) {
				delete(data, key)
			}
		}
	}
}

// Helper function to determine if a value is "empty" for our purposes.
func isEmptyValue(value interface{}) bool {
	switch v := value.(type) {
	case string:
		return v == ""
	case []interface{}:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	case nil:
		return true
	default:
		return false
	}
}

func IsEncrypted(stringTocheck string) bool {
	_, err := DecryptString(config.Get("MO_API_KEY"), stringTocheck)
	return err == nil
}

func EncryptString(password string, plaintext string) (string, error) {
	key := []byte(password)
	// Ensure the key length is 16, 24, or 32 bytes
	key = adjustKeyLength(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("error creating cipher block: %v", err)
	}

	// Generate a random IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {

		return "", fmt.Errorf("error generating IV: %v", err)
	}

	// Pad plaintext to a multiple of block size
	paddedPlaintext := pad([]byte(plaintext), aes.BlockSize)

	// Encrypt using CBC mode
	mode := cipher.NewCBCEncrypter(block, iv)
	ciphertext := make([]byte, len(paddedPlaintext))
	mode.CryptBlocks(ciphertext, paddedPlaintext)

	// Prepend IV to ciphertext
	ciphertext = append(iv, ciphertext...)

	// Encode to base64 for safe transport/storage
	encodedCiphertext := base64.StdEncoding.EncodeToString(ciphertext)

	return encodedCiphertext, nil
}

func DecryptString(password string, encryptedString string) (string, error) {
	key := []byte(password)
	key = adjustKeyLength(key)

	ciphertext, err := base64.StdEncoding.DecodeString(encryptedString)
	if err != nil {
		return "", fmt.Errorf("error decoding base64: %v", err)
	}

	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	// Extract IV from ciphertext
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	if len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("ciphertext is not a multiple of the block size")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("error creating cipher block: %v", err)
	}

	// Decrypt using CBC mode
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// Unpad plaintext
	unpaddedPlaintext, err := unpad(plaintext, aes.BlockSize)
	if err != nil {
		return "", fmt.Errorf("error unpadding plaintext: %v", err)
	}

	return string(unpaddedPlaintext), nil
}

func pad(src []byte, blockSize int) []byte {
	padding := blockSize - len(src)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(src, padtext...)
}

func unpad(src []byte, blockSize int) ([]byte, error) {
	length := len(src)
	if length == 0 || length%blockSize != 0 {
		return nil, fmt.Errorf("invalid padded plaintext")
	}

	padding := int(src[length-1])
	if padding > blockSize || padding == 0 {
		return nil, fmt.Errorf("invalid padding")
	}

	for i := length - padding; i < length; i++ {
		if src[i] != byte(padding) {
			return nil, fmt.Errorf("invalid padding")
		}
	}

	return src[:length-padding], nil
}

func adjustKeyLength(key []byte) []byte {
	// AES key lengths can be 16, 24, or 32 bytes
	if len(key) < 16 {
		key = append(key, bytes.Repeat([]byte{0}, 16-len(key))...)
	} else if len(key) > 16 && len(key) < 24 {
		key = append(key, bytes.Repeat([]byte{0}, 24-len(key))...)
	} else if len(key) > 24 && len(key) < 32 {
		key = append(key, bytes.Repeat([]byte{0}, 32-len(key))...)
	} else if len(key) > 32 {
		key = key[:32]
	}
	return key
}

func PrettyPrintInterface(i interface{}) string {
	str := PrettyPrintString(i)
	return RedactString(str)
}

func RedactString(targetSring string) string {
	for _, secret := range logging.SecretArray() {
		targetSring = strings.ReplaceAll(targetSring, secret, logging.REDACTED)
	}
	return targetSring
}

func PrettyPrintString(i interface{}) string {
	iJson, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		utilsLogger.Error(err.Error())
	}
	return string(iJson)
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

func MilliSecSince(since time.Time) int64 {
	return time.Since(since).Milliseconds()
}

func MicroSecSince(since time.Time) int64 {
	return time.Since(since).Microseconds()
}

func DurationStrSince(since time.Time) string {
	duration := MilliSecSince(since)
	durationStr := fmt.Sprintf("%d ms", duration)
	if duration <= 0 {
		duration = MicroSecSince(since)
		durationStr = fmt.Sprintf("%d μs", duration)
	}
	return durationStr
}

func MergeMaps(maps ...map[string]string) map[string]string {
	resultMap := make(map[string]string)

	// Iterate over the slice of maps
	for _, m := range maps {
		// Add all elements from each map, potentially overwriting
		for key, value := range m {
			resultMap[key] = value
		}
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

func ContainsToLowercase(s []string, str string) bool {
	for _, v := range s {
		if strings.Contains(strings.ToLower(str), strings.ToLower(v)) {
			return true
		}
	}
	return false
}

func ContainsEqual(s []string, str string) bool {
	for _, v := range s {
		if str == v {
			return true
		}
	}
	return false
}
func ContainsPattern(key []byte, pattern string) bool {
	return strings.Contains(string(key), pattern)
}

func IsProduction() bool {
	stage := config.Get("MO_STAGE")
	return Equals([]string{"prod", "production"}, strings.ToLower(stage))
}

func Equals(s []string, str string) bool {
	for _, v := range s {
		if str == v {
			return true
		}
	}
	return false
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

func ParseJsonStringArray(input string) []string {
	val := []string{}
	if err := json.Unmarshal([]byte(input), &val); err != nil {
		utilsLogger.Error("jsonStringArrayToStringArray: Failed to parse into []string.", "input", input)
	}
	return val
}
