package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/valkeyclient"
	"net/http"
	"net/url"
	"time"
)

const (
	DB_PROMETHEUS_QUERIES = "prometheus_queries"
)

type PrometheusRequest struct {
	Query              string `json:"query"`
	Step               int    `json:"step"`
	TimeOffsetSeconds  int64  `json:"timeOffsetSeconds"`
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
	Step       int    `json:"step" validate:"required"`
}

type PrometheusRequestRedisList struct {
	Namespace  string `json:"namespace" validate:"required"`
	Controller string `json:"controller" validate:"required"`
}

type PrometheusQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []any  `json:"result"`
	} `json:"data"`
	ErrorType string `json:"errorType,omitempty"`
	Error     string `json:"error,omitempty"`
}

type PrometheusStoreObject struct {
	Query     string    `json:"query"`
	Step      int       `json:"step"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}
type PrometheusValuesResponse struct {
	Status string   `json:"status"`
	Data   []string `json:"data"`
}

func IsPrometheusReachable(data PrometheusRequest, config cfg.ConfigModule) (bool, error) {
	data.Query = "up" // Default query to check if Prometheus is reachable
	urlString, header := prometheusUrlAndHeader(data, "", config)
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

func PrometheusValues(data PrometheusRequest, config cfg.ConfigModule) ([]string, error) {
	urlString, header := prometheusUrlAndHeader(data, "/api/v1/label/__name__/values", config)
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

func ExecutePrometheusQuery(data PrometheusRequest, config cfg.ConfigModule) (*PrometheusQueryResponse, error) {
	urlString, header := prometheusUrlAndHeader(data, "", config)
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
	prometheusStoreObject := PrometheusStoreObject{
		Query:     req.Query,
		Step:      req.Step,
		CreatedAt: time.Now(),
	}
	err := valkey.SetObject(prometheusStoreObject, 0, DB_PROMETHEUS_QUERIES, req.Namespace, req.Controller, req.QueryName)
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

func PrometheusListQueriesFromRedis(valkey valkeyclient.ValkeyClient, req PrometheusRequestRedisList) (map[string]PrometheusStoreObject, error) {
	pattern := fmt.Sprintf("%s:%s:%s:*", DB_PROMETHEUS_QUERIES, req.Namespace, req.Controller)
	keys, err := valkey.Keys(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list queries from Redis: %v", err)
	}
	queries := make(map[string]PrometheusStoreObject)
	for _, key := range keys {
		query, err := valkey.Get(key)
		if err != nil {
			return nil, fmt.Errorf("failed to get query from Redis: %v", err)
		}
		var queryObj PrometheusStoreObject
		if err := json.Unmarshal([]byte(query), &queryObj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal query: %v", err)
		}
		queries[key] = queryObj
	}

	return queries, nil
}

func PrometheusGetQueryFromRedis(valkey valkeyclient.ValkeyClient, req PrometheusRequestRedis) (*PrometheusStoreObject, error) {
	query, err := valkey.Get(DB_PROMETHEUS_QUERIES, req.Namespace, req.Controller, req.QueryName)
	if err != nil {
		return nil, fmt.Errorf("failed to get query from Redis: %v", err)
	}
	var queryObj PrometheusStoreObject
	if err := json.Unmarshal([]byte(query), &queryObj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal query: %v", err)
	}

	return &queryObj, nil
}

func prometheusUrlAndHeader(data PrometheusRequest, customEndpoint string, config cfg.ConfigModule) (urlString string, header map[string][]string) {
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
		namespace, serviceName, port, err := kubernetes.FindPrometheusService()
		if err != nil {
			return "", header
		}
		clusterDomain, err := config.TryGet("CLUSTER_DOMAIN")
		if err != nil {
			clusterDomain = "cluster.local"
		}

		result = fmt.Sprintf("http://%s.%s.svc.%s:%d", serviceName, namespace, clusterDomain, port)
	}

	if customEndpoint != "" {
		result += customEndpoint
	} else {
		if data.TimeOffsetSeconds == 0 {
			result += fmt.Sprintf("/api/v1/query?query=%s", url.QueryEscape(data.Query))
		} else {
			if data.Step <= 0 {
				data.Step = 1 // Default step if not provided
			}
			if data.TimeOffsetSeconds < 60 {
				data.TimeOffsetSeconds = 60 // Minimum offset to avoid too frequent queries
			}
			start := time.Now().Add(-time.Duration(data.TimeOffsetSeconds) * time.Second).Unix()
			end := time.Now().Unix()
			result += fmt.Sprintf("/api/v1/query_range?query=%s&step=%d&start=%d&end=%d", url.QueryEscape(data.Query), data.Step, start, end)
		}
	}

	return result, header
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
