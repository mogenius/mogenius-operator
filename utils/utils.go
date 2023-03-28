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

func Contains(s []string, str string) bool {
	for _, v := range s {
		if strings.Contains(str, v) {
			return true
		}
	}
	return false
}

func ContainsInt(v int, a []int) bool {
	for _, i := range a {
		if i == v {
			return true
		}
	}
	return false
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
