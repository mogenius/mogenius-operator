package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/valkeyclient"
	"net/http"
	"net/url"
)

const (
	DB_PROMETHEUS_QUERIES = "prometheus_queries"
)

type PrometheusRequest struct {
	Query              string `json:"query"`
	Prometheus_API_URL string `json:"prometheusUrl"`
	Prometheus_User    string `json:"prometheusUser"`
	Prometheus_Pass    string `json:"prometheusPass"`
	Prometheus_Token   string `json:"prometheusToken"`
}

type PrometheusRequestRedis struct {
	Query      string `json:"query" validate:"required"`
	QueryName  string `json:"queryName" validate:"required"`
	Namespace  string `json:"namespace" validate:"required"`
	Controller string `json:"controller" validate:"required"`
}

type PrometheusRequestRedisList struct {
	Namespace  string `json:"namespace" validate:"required"`
	Controller string `json:"controller" validate:"required"`
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
	urlString, header := PrometheusUrlAndHeader(data, "")
	if urlString == "" {
		return false, fmt.Errorf("Prometheus API URL is not set or query is empty")
	}
	req, err := http.NewRequest("GET", urlString, nil)
	req.Header = header

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
	urlString, header := PrometheusUrlAndHeader(data, "/api/v1/label/__name__/values")
	if urlString == "" {
		return []string{}, fmt.Errorf("Prometheus API URL is not set or query is empty")
	}
	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		return []string{}, fmt.Errorf("Failed to create request: %v", err)
	}
	req.Header = header

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
	urlString, header := PrometheusUrlAndHeader(data, "")
	if urlString == "" {
		return nil, fmt.Errorf("Prometheus API URL is not set or query is empty")
	}
	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create request: %v", err)
	}
	req.Header = header

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

func PrometheusSaveQueryToRedis(valkey valkeyclient.ValkeyClient, req PrometheusRequestRedis) (*string, error) {
	err := valkey.SetObject(req.Query, 0, DB_PROMETHEUS_QUERIES, req.Namespace, req.Controller, req.QueryName)
	if err != nil {
		return nil, fmt.Errorf("failed to save query to Redis: %v", err)
	}
	return &req.Query, nil
}

func PrometheusRemoveQueryFromRedis(valkey valkeyclient.ValkeyClient, req PrometheusRequestRedis) (*string, error) {
	err := valkey.DeleteSingle(DB_PROMETHEUS_QUERIES, req.Namespace, req.Controller, req.QueryName)
	if err != nil {
		return nil, fmt.Errorf("failed to remove query from Redis: %v", err)
	}

	return &req.QueryName, nil
}

func PrometheusListQueriesFromRedis(valkey valkeyclient.ValkeyClient, req PrometheusRequestRedisList) (map[string]string, error) {
	pattern := fmt.Sprintf("%s:%s:%s:*", DB_PROMETHEUS_QUERIES, req.Namespace, req.Controller)
	keys, err := valkey.Keys(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list queries from Redis: %v", err)
	}
	queries := make(map[string]string)
	for _, key := range keys {
		query, err := valkey.Get(key)
		if err != nil {
			return nil, fmt.Errorf("failed to get query from Redis: %v", err)
		}
		queries[key] = query
	}
	return queries, nil
}

func PrometheusGetQueryFromRedis(valkey valkeyclient.ValkeyClient, req PrometheusRequestRedis) (*string, error) {
	query, err := valkey.Get(DB_PROMETHEUS_QUERIES, req.Namespace, req.Controller, req.QueryName)
	if err != nil {
		return nil, fmt.Errorf("failed to get query from Redis: %v", err)
	}
	return &query, nil
}

func PrometheusUrlAndHeader(data PrometheusRequest, customEndpoint string) (urlString string, header map[string][]string) {
	header = make(map[string][]string)

	if data.Prometheus_User != "" && data.Prometheus_Pass != "" {
		username := data.Prometheus_User
		password := data.Prometheus_Pass
		header["Authorization"] = []string{"Basic " + basicAuth(username, password)}
	} else if data.Prometheus_Token != "" {
		token := data.Prometheus_Token
		header["Authorization"] = []string{"Bearer " + token}
	}

	result := data.Prometheus_API_URL
	if data.Prometheus_API_URL == "" {
		namespace, serviceName, _, err := kubernetes.FindPrometheusService()
		if err != nil {
			return "", header
		}
		result = fmt.Sprintf("http://%s.%s.svc.cluster.local", serviceName, namespace)
	}

	if customEndpoint != "" {
		result += customEndpoint
	} else {
		result += fmt.Sprintf("/api/v1/query?query=%s", url.QueryEscape(data.Query))
	}

	return result, header
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
