package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mogenius-k8s-manager/src/kubernetes"
	"net/http"
	"net/url"
)

type PrometheusRequest struct {
	Query              string `json:"query"`
	Prometheus_API_URL string `json:"prometheusUrl"`
	Prometheus_User    string `json:"prometheusUser"`
	Prometheus_Pass    string `json:"prometheusPass"`
	Prometheus_Token   string `json:"prometheusToken"`
}

type PrometheusQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
	ErrorType string `json:"errorType,omitempty"`
	Error     string `json:"error,omitempty"`
}

type PrometheusValuesResponse struct {
	Status string   `json:"status"`
	Data   []string `json:"data"`
}

func IsPrometheusReachable(data PrometheusRequest) (bool, error) {
	data.Query = "up" // Default query to check if Prometheus is reachable
	urlString := PrometheusUrlString(data, "")
	if urlString == "" {
		return false, fmt.Errorf("Prometheus API URL is not set or query is empty")
	}
	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		return false, fmt.Errorf("Failed to create request: %v", err)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("Prometheus is not reachable")
	}

	return true, nil
}

func PrometheusValues(data PrometheusRequest) ([]string, error) {
	urlString := PrometheusUrlString(data, "/api/v1/label/__name__/values")
	if urlString == "" {
		return []string{}, fmt.Errorf("Prometheus API URL is not set or query is empty")
	}
	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		return []string{}, fmt.Errorf("Failed to create request: %v", err)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return []string{}, fmt.Errorf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return []string{}, fmt.Errorf("Prometheus is not reachable")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read response: %v", err)
	}

	var promResp PrometheusValuesResponse
	if err := json.Unmarshal(body, &promResp); err != nil {
		return nil, fmt.Errorf("Failed to parse JSON: %v", err)
	}

	return promResp.Data, nil
}

func ExecutePrometheusQuery(data PrometheusRequest) (*PrometheusQueryResponse, error) {
	urlString := PrometheusUrlString(data, "")
	if urlString == "" {
		return nil, fmt.Errorf("Prometheus API URL is not set or query is empty")
	}
	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create request: %v", err)
	}

	if data.Prometheus_User != "" && data.Prometheus_Pass != "" {
		username := data.Prometheus_User
		password := data.Prometheus_Pass
		req.Header.Add("Authorization", "Basic "+basicAuth(username, password))
	} else if data.Prometheus_Token != "" {
		token := data.Prometheus_Token
		req.Header.Add("Authorization", "Bearer "+token)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read response: %v", err)
	}

	var promResp PrometheusQueryResponse
	if err := json.Unmarshal(body, &promResp); err != nil {
		return nil, fmt.Errorf("Failed to parse JSON: %v", err)
	}

	if promResp.Status != "success" {
		return nil, fmt.Errorf("Prometheus query failed: %s - %s", promResp.ErrorType, promResp.Error)
	}

	return &promResp, nil
}

func PrometheusUrlString(data PrometheusRequest, customEndpoint string) string {
	result := data.Prometheus_API_URL
	if data.Prometheus_API_URL == "" {
		namespace, serviceName, _, err := kubernetes.FindPrometheusService()
		if err != nil {
			return ""
		}
		result = fmt.Sprintf("http://%s.%s.svc.cluster.local", serviceName, namespace)
	}

	if customEndpoint != "" {
		result += customEndpoint
	} else {
		result += fmt.Sprintf("/api/v1/query?query=%s", url.QueryEscape(data.Query))
	}

	return result
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
