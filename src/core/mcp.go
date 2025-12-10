package core

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type McpApi interface {
	McpRequest(method, url string, body *string, headers map[string]string) (string, error)
}
type mcpApi struct {
	logger *slog.Logger
}

func NewMcpApi(logger *slog.Logger) McpApi {
	self := &mcpApi{}

	self.logger = logger

	return self
}

func (mcp *mcpApi) McpRequest(method, url string, body *string, headers map[string]string) (string, error) {
	baseUrl := "http://kubernetes-mcp-server.kubernetes-mcp-server.svc.cluster.local:8080"

	if body == nil {
		body = new(string)
	}

	req, err := http.NewRequest(method, baseUrl+url, strings.NewReader(*body))
	if err != nil {
		mcp.logger.Error("Failed to create request", slog.Any("error", err))
		return "", err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		mcp.logger.Error("Failed to send request", slog.Any("error", err))
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		mcp.logger.Error("Received non-200 response", slog.Any("status", resp.StatusCode))
		return "", fmt.Errorf("non-200 response: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		mcp.logger.Error("Failed to read response body", slog.Any("error", err))
		return "", err
	}

	return string(bodyBytes), nil
}
