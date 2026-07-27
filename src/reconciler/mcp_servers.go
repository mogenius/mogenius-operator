package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/store"
	"net/url"
	"slices"
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
func (d *reconcilerModule) reconcileMcpServers(ctx context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
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

	// When the spec is valid, connect or probe the MCP server.
	// Full reconnect only when the spec changed (generation bump) or the session
	// is gone (operator restart, transient network drop). Otherwise a lightweight
	// ListTools probe refreshes the tool list without tearing down the live
	// session — avoiding state loss for in-flight agent runs and keeping the
	// reconcile slots free from 30s blocking connects.
	var discoveredTools []string
	didConnect := false
	if conditionStatus == metav1.ConditionTrue {
		sessionExists := d.aiManager.HasMcpSession(server.Name)
		generationChanged := server.Generation != server.Status.ObservedGeneration

		if !sessionExists || generationChanged {
			var connErr error
			discoveredTools, connErr = d.aiManager.NotifyMcpServerChanged(server.Name)
			if connErr != nil {
				conditionStatus = metav1.ConditionFalse
				reason = "ConnectionFailed"
				message = connErr.Error()
			} else {
				didConnect = true
			}
		} else {
			var probeErr error
			discoveredTools, probeErr = d.aiManager.ProbeMcpSession(ctx, server.Name)
			if probeErr != nil {
				// A probe failure on an otherwise-valid spec is logged but does
				// not flip the condition — the session may still be usable for
				// tool calls; we keep the last-known tool list.
				d.logger.Warn("McpServer: tool refresh probe failed", "name", server.Name, "error", probeErr)
				discoveredTools = server.Status.AvailableTools
			}
		}

		if conditionStatus == metav1.ConditionTrue {
			toolsChanged := !slices.Equal(server.Status.AvailableTools, discoveredTools)
			server.Status.AvailableTools = discoveredTools
			server.Status.ToolCount = len(discoveredTools)
			if didConnect {
				server.Status.LastConnectedAt = time.Now().UTC().Format(time.RFC3339)
			}
			// Force a status write when the tool list changed even if the
			// condition fields are unchanged (e.g. server added/removed tools).
			if toolsChanged {
				results = append(results, ReconcileResult{}) // marker: handled below
			}
		}
	} else {
		// Spec invalid — clear stale tool lists so the status is accurate.
		server.Status.AvailableTools = nil
		server.Status.ToolCount = 0
	}

	current := apimeta.FindStatusCondition(server.Status.Conditions, mcpServerReadyCondition)
	wasReady := current != nil && current.Status == metav1.ConditionTrue
	upToDate := current != nil &&
		current.Status == conditionStatus &&
		current.Reason == reason &&
		current.Message == message &&
		server.Status.ObservedGeneration == server.Generation
	toolsChanged := conditionStatus == metav1.ConditionTrue && len(results) > 0
	results = nil // clear the marker; real errors accumulate below

	if !upToDate || toolsChanged || didConnect {
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

		// When a server transitions to Ready, requeue agents that reference it
		// so they reflect the new status immediately rather than waiting up to
		// 15 minutes for the next background sweep.
		if conditionStatus == metav1.ConditionTrue && !wasReady {
			d.requeueAgentsReferencingMcpServer(server.Namespace, server.Name)
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
