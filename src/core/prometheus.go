package core

import (
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/valkeyclient"
	"net/http"
	"net/url"
	"time"

	"encoding/json"
)

const (
	DB_PROMETHEUS_QUERIES = "prometheus_queries"

	// prometheusMaxPoints is Prometheus' hard limit on the number of resolution
	// steps a single range query may resolve to. Exceeding it makes the
	// /api/v1/query_range endpoint reject the request with HTTP 400.
	prometheusMaxPoints = 11000

	// prometheusTargetPoints is the resolution we aim for when the caller does
	// not provide a usable step.
	prometheusTargetPoints = 60
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

func IsPrometheusReachable(data PrometheusRequest, config cfg.ConfigModule, logger *slog.Logger) (bool, error) {
	data.Query = "up" // Default query to check if Prometheus is reachable
	urlString, header := prometheusUrlAndHeader(data, "", config, logger)
	if urlString == "" {
		return false, fmt.Errorf("prometheus API URL is not set or query is empty")
	}
	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header = header

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("prometheus reachability check failed", "url", urlString, "error", err)
		return false, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Warn("prometheus not reachable", "url", urlString, "statusCode", resp.StatusCode, "body", string(body))
		return false, fmt.Errorf("prometheus is not reachable: status %d", resp.StatusCode)
	}

	return true, nil
}

func PrometheusValues(data PrometheusRequest, config cfg.ConfigModule, logger *slog.Logger) ([]string, error) {
	urlString, header := prometheusUrlAndHeader(data, "/api/v1/label/__name__/values", config, logger)
	if urlString == "" {
		return []string{}, fmt.Errorf("prometheus API URL is not set or query is empty")
	}
	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		return []string{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header = header

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("prometheus values request failed", "url", urlString, "error", err)
		return []string{}, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Warn("prometheus values request returned non-OK status", "url", urlString, "statusCode", resp.StatusCode, "body", string(body))
		return []string{}, fmt.Errorf("prometheus is not reachable: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var promResp PrometheusValuesResponse
	if err := json.Unmarshal(body, &promResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return promResp.Data, nil
}

func ExecutePrometheusQuery(data PrometheusRequest, config cfg.ConfigModule, logger *slog.Logger) (*PrometheusQueryResponse, error) {
	urlString, header := prometheusUrlAndHeader(data, "", config, logger)
	if urlString == "" {
		return nil, fmt.Errorf("prometheus API URL is not set or query is empty")
	}
	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header = header

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("prometheus query request failed", "url", urlString, "error", err)
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var promResp PrometheusQueryResponse
	if err := json.Unmarshal(body, &promResp); err != nil {
		// A non-OK status without a parseable Prometheus error envelope (e.g. an
		// HTML error page from a reverse proxy) lands here. Surface the raw body
		// so the failure is diagnosable instead of an opaque JSON parse error.
		logger.Warn("failed to parse prometheus response", "url", urlString, "statusCode", resp.StatusCode, "body", string(body), "error", err)
		return nil, fmt.Errorf("failed to parse prometheus response (status %d): %w", resp.StatusCode, err)
	}

	if promResp.Status != "success" {
		logger.Warn("prometheus query failed", "url", urlString, "statusCode", resp.StatusCode, "errorType", promResp.ErrorType, "error", promResp.Error)
		return nil, fmt.Errorf("prometheus query failed: %s - %s", promResp.ErrorType, promResp.Error)
	}

	// A successful query with an empty result is the symptom users see as an
	// "empty chart": the request reached Prometheus but matched no series for
	// the given query and time range. Log it explicitly so it is not mistaken
	// for a transport failure.
	if len(promResp.Data.Result) == 0 {
		logger.Info("prometheus query returned no data", "url", urlString, "query", data.Query, "resultType", promResp.Data.ResultType, "timeOffsetSeconds", data.TimeOffsetSeconds, "step", data.Step)
	} else {
		logger.Debug("prometheus query succeeded", "query", data.Query, "resultType", promResp.Data.ResultType, "seriesCount", len(promResp.Data.Result))
	}

	return &promResp, nil
}

func PrometheusSaveQueryToRedis(valkey valkeyclient.ValkeyClient, req PrometheusRequestRedis) (*string, error) {
	prometheusStoreObject := PrometheusStoreObject{
		Query:     req.Query,
		Step:      req.Step,
		CreatedAt: time.Now(),
	}
	err := valkey.SetObject(prometheusStoreObject, 30*24*time.Hour, DB_PROMETHEUS_QUERIES, req.Namespace, req.Controller, req.QueryName)
	if err != nil {
		return nil, fmt.Errorf("failed to save query to Redis: %w", err)
	}
	return &req.Query, nil
}

func PrometheusRemoveQueryFromRedis(valkey valkeyclient.ValkeyClient, req PrometheusRequestRedis) (*string, error) {
	err := valkey.DeleteSingle(DB_PROMETHEUS_QUERIES, req.Namespace, req.Controller, req.QueryName)
	if err != nil {
		return nil, fmt.Errorf("failed to remove query from Redis: %w", err)
	}

	return &req.QueryName, nil
}

func PrometheusListQueriesFromRedis(valkey valkeyclient.ValkeyClient, req PrometheusRequestRedisList) (map[string]PrometheusStoreObject, error) {
	pattern := fmt.Sprintf("%s:%s:%s:*", DB_PROMETHEUS_QUERIES, req.Namespace, req.Controller)
	keys, err := valkey.Keys(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list queries from Redis: %w", err)
	}
	queries := make(map[string]PrometheusStoreObject)
	for _, key := range keys {
		query, err := valkey.Get(key)
		if err != nil {
			return nil, fmt.Errorf("failed to get query from Redis: %w", err)
		}
		var queryObj PrometheusStoreObject
		if err := json.Unmarshal([]byte(query), &queryObj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal query: %w", err)
		}
		queries[key] = queryObj
	}

	return queries, nil
}

func PrometheusGetQueryFromRedis(valkey valkeyclient.ValkeyClient, req PrometheusRequestRedis) (*PrometheusStoreObject, error) {
	query, err := valkey.Get(DB_PROMETHEUS_QUERIES, req.Namespace, req.Controller, req.QueryName)
	if err != nil {
		return nil, fmt.Errorf("failed to get query from Redis: %w", err)
	}
	var queryObj PrometheusStoreObject
	if err := json.Unmarshal([]byte(query), &queryObj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal query: %w", err)
	}

	return &queryObj, nil
}

func prometheusUrlAndHeader(data PrometheusRequest, customEndpoint string, config cfg.ConfigModule, logger *slog.Logger) (urlString string, header map[string][]string) {
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
			logger.Warn("prometheus service auto-discovery failed; no prometheus URL was provided either", "error", err)
			return "", header
		}
		clusterDomain, err := config.TryGet("CLUSTER_DOMAIN")
		if err != nil {
			clusterDomain = "cluster.local"
		}

		result = fmt.Sprintf("http://%s.%s.svc.%s:%d", serviceName, namespace, clusterDomain, port)
		logger.Debug("prometheus auto-discovered", "service", serviceName, "namespace", namespace, "port", port, "url", result)
	}

	if customEndpoint != "" {
		result += customEndpoint
	} else if data.TimeOffsetSeconds == 0 {
		result += fmt.Sprintf("/api/v1/query?query=%s", url.QueryEscape(data.Query))
	} else {
		offset, step := resolvePrometheusRange(data.TimeOffsetSeconds, data.Step)
		start := time.Now().Add(-time.Duration(offset) * time.Second).Unix()
		end := time.Now().Unix()
		result += fmt.Sprintf("/api/v1/query_range?query=%s&step=%d&start=%d&end=%d", url.QueryEscape(data.Query), step, start, end)
	}

	return result, header
}

// resolvePrometheusRange normalizes the requested time range and step for a
// /api/v1/query_range request. It guarantees a sane resolution and, crucially,
// never lets the query resolve to more than prometheusMaxPoints points per
// series — Prometheus rejects such requests with HTTP 400, which surfaces to
// the user as an empty chart.
func resolvePrometheusRange(timeOffsetSeconds int64, step int) (resolvedOffset int64, resolvedStep int) {
	if timeOffsetSeconds < 60 {
		timeOffsetSeconds = 60 // Minimum offset to avoid too frequent queries
	}

	// Fall back to a ~prometheusTargetPoints resolution when the caller does not
	// provide a usable step, or provides one so large it would yield <2 points.
	if step <= 0 || int64(step) > timeOffsetSeconds/2 {
		calculatedStep := timeOffsetSeconds / prometheusTargetPoints
		if calculatedStep < 1 {
			calculatedStep = 1
		}
		step = int(calculatedStep)
	}

	// Clamp the resolution so we never exceed Prometheus' max points per series.
	// The step is rounded up (ceil division) so the resulting point count stays
	// at or below the limit even when the range is not an exact multiple.
	if timeOffsetSeconds/int64(step) > prometheusMaxPoints {
		minStep := (timeOffsetSeconds + prometheusMaxPoints - 1) / prometheusMaxPoints
		if minStep < 1 {
			minStep = 1
		}
		step = int(minStep)
	}

	return timeOffsetSeconds, step
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
