package core

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/kubernetes"
	"net/http"
	"time"

	json "github.com/goccy/go-json"
)

type AlertmanagerService interface {
	GetAlerts() ([]Alert, error)
	SendAlert(alerts []SendAlertRequest) error
	SilenceAlert(silence SilenceRequest) (string, error)
	GetSilences() ([]Silence, error)
	DeleteSilence(silenceID string) error
}

type alertmanagerService struct {
	logger *slog.Logger
	config cfg.ConfigModule
}

func NewAlertmanagerService(logger *slog.Logger, config cfg.ConfigModule) AlertmanagerService {
	return &alertmanagerService{logger: logger, config: config}
}

// Alert represents a single alert returned by Alertmanager.
type Alert struct {
	Annotations  map[string]string       `json:"annotations"`
	EndsAt       time.Time               `json:"endsAt"`
	Fingerprint  string                  `json:"fingerprint"`
	Labels       map[string]string       `json:"labels"`
	Receivers    []struct{ Name string } `json:"receivers"`
	StartsAt     time.Time               `json:"startsAt"`
	Status       AlertStatus             `json:"status"`
	UpdatedAt    time.Time               `json:"updatedAt"`
	GeneratorURL string                  `json:"generatorURL"`
}

// AlertStatus holds the current state of an alert.
type AlertStatus struct {
	InhibitedBy []string `json:"inhibitedBy"`
	SilencedBy  []string `json:"silencedBy"`
	State       string   `json:"state"` // "active", "suppressed", "unprocessed"
}

// AlertMatcher is used when creating a silence to select which alerts it covers.
type AlertMatcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
	IsEqual bool   `json:"isEqual"`
}

// SendAlertRequest is the payload for pushing an alert to Alertmanager.
type SendAlertRequest struct {
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	StartsAt     time.Time         `json:"startsAt,omitempty"`
	EndsAt       time.Time         `json:"endsAt,omitempty"`
	GeneratorURL string            `json:"generatorURL,omitempty"`
}

// Silence represents an existing silence returned by Alertmanager.
type Silence struct {
	ID        string         `json:"id"`
	Matchers  []AlertMatcher `json:"matchers"`
	StartsAt  time.Time      `json:"startsAt"`
	EndsAt    time.Time      `json:"endsAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	CreatedBy string         `json:"createdBy"`
	Comment   string         `json:"comment"`
	Status    struct {
		State string `json:"state"` // "active", "expired", "pending"
	} `json:"status"`
}

// SilenceRequest is the payload for creating a new silence.
type SilenceRequest struct {
	Matchers  []AlertMatcher `json:"matchers"`
	StartsAt  time.Time      `json:"startsAt"`
	EndsAt    time.Time      `json:"endsAt"`
	CreatedBy string         `json:"createdBy"`
	Comment   string         `json:"comment"`
}

// GetAlerts returns all current alerts from Alertmanager.
func (s *alertmanagerService) GetAlerts() ([]Alert, error) {
	baseUrl, err := alertmanagerUrl(s.config)
	if err != nil {
		return nil, fmt.Errorf("alertmanager: get base URL: %w", err)
	}

	url := fmt.Sprintf("%s/api/v2/alerts", baseUrl)
	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("alertmanager: create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("alertmanager: get alerts: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("alertmanager: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alertmanager: get alerts returned %d: %s", resp.StatusCode, body)
	}

	var alerts []Alert
	if err := json.Unmarshal(body, &alerts); err != nil {
		return nil, fmt.Errorf("alertmanager: parse alerts: %w", err)
	}

	s.logger.Debug("Fetched alerts from Alertmanager", "count", len(alerts), "url", url)
	return alerts, nil
}

// SendAlert pushes one or more alerts to Alertmanager via POST /api/v2/alerts.
func (s *alertmanagerService) SendAlert(alerts []SendAlertRequest) error {
	baseUrl, err := alertmanagerUrl(s.config)
	if err != nil {
		return fmt.Errorf("alertmanager: get base URL: %w", err)
	}

	payload, err := json.Marshal(alerts)
	if err != nil {
		return fmt.Errorf("alertmanager: marshal alerts: %w", err)
	}

	url := fmt.Sprintf("%s/api/v2/alerts", baseUrl)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("alertmanager: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("alertmanager: send alerts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("alertmanager: send alerts returned %d: %s", resp.StatusCode, body)
	}

	s.logger.Debug("Sent alerts to Alertmanager", "count", len(alerts), "url", url)
	return nil
}

// SilenceAlert creates a silence in Alertmanager and returns the new silence ID.
func (s *alertmanagerService) SilenceAlert(silence SilenceRequest) (string, error) {
	baseUrl, err := alertmanagerUrl(s.config)
	if err != nil {
		return "", fmt.Errorf("alertmanager: get base URL: %w", err)
	}

	payload, err := json.Marshal(silence)
	if err != nil {
		return "", fmt.Errorf("alertmanager: marshal silence: %w", err)
	}

	url := fmt.Sprintf("%s/api/v2/silences", baseUrl)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("alertmanager: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("alertmanager: silence alert: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("alertmanager: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("alertmanager: create silence returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		SilenceID string `json:"silenceID"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("alertmanager: parse silence response: %w", err)
	}

	s.logger.Debug("Created Alertmanager silence", "silenceID", result.SilenceID, "url", url)
	return result.SilenceID, nil
}

// GetSilences returns all silences from Alertmanager.
func (s *alertmanagerService) GetSilences() ([]Silence, error) {
	baseUrl, err := alertmanagerUrl(s.config)
	if err != nil {
		return nil, fmt.Errorf("alertmanager: get base URL: %w", err)
	}
	url := fmt.Sprintf("%s/api/v2/silences", baseUrl)
	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("alertmanager: create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("alertmanager: get silences: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("alertmanager: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alertmanager: get silences returned %d: %s", resp.StatusCode, body)
	}

	var silences []Silence
	if err := json.Unmarshal(body, &silences); err != nil {
		return nil, fmt.Errorf("alertmanager: parse silences: %w", err)
	}

	s.logger.Debug("Fetched silences from Alertmanager", "count", len(silences), "url", url)
	return silences, nil
}

// DeleteSilence removes a silence by ID from Alertmanager.
func (s *alertmanagerService) DeleteSilence(silenceID string) error {
	baseUrl, err := alertmanagerUrl(s.config)
	if err != nil {
		return fmt.Errorf("alertmanager: get base URL: %w", err)
	}
	url := fmt.Sprintf("%s/api/v2/silence/%s", baseUrl, silenceID)
	httpReq, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("alertmanager: create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("alertmanager: delete silence: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("alertmanager: delete silence returned %d: %s", resp.StatusCode, body)
	}

	s.logger.Debug("Deleted Alertmanager silence", "silenceID", silenceID, "url", url)
	return nil
}

func alertmanagerUrl(config cfg.ConfigModule) (string, error) {
	namespace, serviceName, port, err := kubernetes.FindAlertmanagerService()
	if err != nil {
		return "", err
	}
	clusterDomain, err := config.TryGet("CLUSTER_DOMAIN")
	if err != nil {
		clusterDomain = "cluster.local"
	}

	return fmt.Sprintf("http://%s.%s.svc.%s:%d", serviceName, namespace, clusterDomain, port), nil
}
