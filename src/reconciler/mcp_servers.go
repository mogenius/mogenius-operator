package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/store"
	"net/url"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const mcpServerReadyCondition = "Ready"

// reconcileMcpServers validates McpServer CRs and updates their status.
// On a valid (non-deleted) CR it also triggers the AI manager to reconnect
// the underlying MCP server so the discovered tool list stays in sync.
func (d *reconcilerModule) reconcileMcpServers(_ context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	if op == deleteOperation {
		var server v1alpha1.McpServer
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &server); err == nil {
			d.aiManager.NotifyMcpServerDeleted(server.Name)
		}
		return nil
	}

	var server v1alpha1.McpServer
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &server); err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("failed to parse McpServer: %w", err)}}
	}

	conditionStatus, reason, message := d.evaluateMcpServer(&server)

	var results []ReconcileResult

	// When the spec is valid, (re)connect the MCP server and collect discovered tools.
	if conditionStatus == metav1.ConditionTrue {
		discoveredTools, connErr := d.aiManager.NotifyMcpServerChanged(server.Name)
		if connErr != nil {
			d.logger.Warn("McpServer: connection probe failed", "name", server.Name, "error", connErr)
			conditionStatus = metav1.ConditionFalse
			reason = "ConnectionFailed"
			message = connErr.Error()
		} else {
			server.Status.AvailableTools = discoveredTools
			server.Status.ToolCount = len(discoveredTools)
			server.Status.LastConnectedAt = time.Now().UTC().Format(time.RFC3339)
		}
	} else {
		// Spec invalid — clear stale tool lists so the status is accurate.
		server.Status.AvailableTools = nil
		server.Status.ToolCount = 0
	}

	current := apimeta.FindStatusCondition(server.Status.Conditions, mcpServerReadyCondition)
	upToDate := current != nil &&
		current.Status == conditionStatus &&
		current.Reason == reason &&
		current.Message == message &&
		server.Status.ObservedGeneration == server.Generation
	if !upToDate || conditionStatus == metav1.ConditionTrue {
		// Always re-write status after a successful connection (tool list and
		// lastConnectedAt change on every reconcile when the server is healthy).
		apimeta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               mcpServerReadyCondition,
			Status:             conditionStatus,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: server.Generation,
		})
		server.Status.ObservedGeneration = server.Generation
		if _, err := d.clientProvider.MogeniusClientSet().MogeniusV1alpha1.UpdateMcpServerStatus(&server); err != nil {
			return []ReconcileResult{{Err: fmt.Errorf("failed to update status of McpServer %q: %w", server.Name, err)}}
		}
	}

	if conditionStatus == metav1.ConditionFalse {
		results = append(results, ReconcileResult{Err: fmt.Errorf("McpServer %q is not ready: %s: %s", server.Name, reason, message), IsWarning: true})
	}
	return results
}

// evaluateMcpServer validates the McpServer spec without making any network calls.
func (d *reconcilerModule) evaluateMcpServer(server *v1alpha1.McpServer) (metav1.ConditionStatus, string, string) {
	ownNamespace := d.config.Get("MO_OWN_NAMESPACE")
	if server.Namespace != ownNamespace {
		return metav1.ConditionFalse, "IgnoredNamespace", fmt.Sprintf("McpServers are only processed in namespace %q", ownNamespace)
	}

	if server.Spec.URL == "" {
		return metav1.ConditionFalse, "InvalidSpec", "spec.url must not be empty"
	}
	if _, err := url.ParseRequestURI(server.Spec.URL); err != nil {
		return metav1.ConditionFalse, "InvalidSpec", fmt.Sprintf("spec.url is not a valid URL: %v", err)
	}

	switch server.Spec.Transport {
	case v1alpha1.McpTransportStreamableHTTP, v1alpha1.McpTransportSSE:
	default:
		return metav1.ConditionFalse, "InvalidSpec", fmt.Sprintf("unsupported transport %q (allowed: streamableHttp, sse)", server.Spec.Transport)
	}

	if a := server.Spec.Auth; a != nil && a.Type != v1alpha1.McpAuthNone {
		if a.SecretRef == nil {
			return metav1.ConditionFalse, "InvalidSpec", fmt.Sprintf("auth type %q requires secretRef", a.Type)
		}
		if store.GetSecret(server.Namespace, a.SecretRef.Name) == nil {
			return metav1.ConditionFalse, "SecretNotFound", fmt.Sprintf("auth.secretRef references Secret %q which does not exist", a.SecretRef.Name)
		}
	}

	for _, h := range server.Spec.Headers {
		if h.Name == "" {
			return metav1.ConditionFalse, "InvalidSpec", "headers entry has an empty name"
		}
		if h.ValueFrom != nil {
			if store.GetSecret(server.Namespace, h.ValueFrom.Name) == nil {
				return metav1.ConditionFalse, "SecretNotFound", fmt.Sprintf("header %q valueFrom references Secret %q which does not exist", h.Name, h.ValueFrom.Name)
			}
		}
	}

	return metav1.ConditionTrue, "Valid", "spec is valid"
}
