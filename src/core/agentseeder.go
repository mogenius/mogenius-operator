package core

import (
	"context"
	"fmt"
	"log/slog"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/k8sclient"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DEFAULT_AGENTS_MARKER_CONFIGMAP records that the default agents were seeded
// once. Seeding never runs again while it exists, so deleting a default agent
// is permanent (until the marker itself is deleted).
const DEFAULT_AGENTS_MARKER_CONFIGMAP = "mogenius-ai-default-agents-seeded"

// defaultAgents are seeded disabled: the customer explicitly opts in per
// agent. All of them scope to "*" (resolved to an explicit namespace
// allow-list at run time) and stay strictly read-only like any other agent.
func defaultAgents() []v1alpha1.Agent {
	allNamespaces := v1alpha1.AgentScope{Namespaces: []string{"*"}}
	return []v1alpha1.Agent{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster-cleanup"},
			Spec: v1alpha1.AgentSpec{
				DisplayName: "Cluster Cleanup",
				Description: "Finds leftover resources like finished jobs, empty replica sets and unused config that can be safely removed.",
				Icon:        "fa-broom",
				Instruction: "You are a cluster janitor. Look for leftovers that accumulate over time: completed or failed Jobs older than a week, ReplicaSets scaled to zero that were superseded by newer revisions, and pods in Succeeded or Failed phase that were never cleaned up. Pick the single most obvious leftover and propose deleting it. Never propose deleting anything that is running, referenced by other resources, or that you are not certain about.",
				Enabled:     false,
				Scope:       allNamespaces,
				Triggers: v1alpha1.AgentTriggers{
					Cron:   "0 6 * * 1",
					Manual: true,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "resource-optimizer"},
			Spec: v1alpha1.AgentSpec{
				DisplayName: "Resource Optimizer",
				Description: "Reviews CPU and memory requests/limits and proposes right-sizing for over- or under-provisioned workloads.",
				Icon:        "fa-gauge-high",
				Instruction: "You are a resource right-sizing expert. Inspect deployments and stateful sets for missing resource requests or limits, requests that are drastically higher than typical usage for that kind of workload, and containers without memory limits that risk destabilizing nodes. Propose an update for the single most impactful workload, changing only its resources section and keeping every other field untouched. Be conservative — when in doubt, prefer adding missing requests/limits over shrinking existing ones.",
				Enabled:     false,
				Scope:       allNamespaces,
				Triggers: v1alpha1.AgentTriggers{
					Cron:   "0 7 * * 1",
					Manual: true,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "workload-doctor"},
			Spec: v1alpha1.AgentSpec{
				DisplayName: "Workload Doctor",
				Description: "Reacts to crash-looping and image-pull failures, diagnoses the root cause and proposes a concrete fix.",
				Icon:        "fa-stethoscope",
				Instruction: "You are a Kubernetes troubleshooter. Diagnose the failing workload: read its events, container statuses and logs, identify the root cause (bad image tag, failing command, missing config or secret, OOM kills) and propose a minimal fix on the owning controller. Only propose a change you are confident fixes the root cause; otherwise report your diagnosis without a proposed operation.",
				Enabled:     false,
				Scope:       allNamespaces,
				Triggers: v1alpha1.AgentTriggers{
					Events: []v1alpha1.AgentEventFilter{
						{
							Id:   "workload-doctor/crashloop",
							Name: "CrashLoopBackOff",
							Kind: "Pod",
							Contains: map[string]string{
								".status.containerStatuses[*].state.waiting.reason": "CrashLoopBackOff",
							},
							Prompt: "This pod is crash-looping. Diagnose the root cause and, if possible, propose a concrete fix on the owning controller.",
						},
						{
							Id:   "workload-doctor/imagepull",
							Name: "ImagePullBackOff",
							Kind: "Pod",
							Contains: map[string]string{
								".status.containerStatuses[*].state.waiting.reason": "ImagePullBackOff",
							},
							Prompt: "This pod cannot pull its image. Check the image reference and pull configuration and propose a fix if the cause is evident.",
						},
					},
					Manual: true,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "security-auditor"},
			Spec: v1alpha1.AgentSpec{
				DisplayName: "Security Auditor",
				Description: "Audits workloads for risky settings like privileged containers, host mounts, latest tags and missing security contexts.",
				Icon:        "fa-shield-halved",
				Instruction: "You are a Kubernetes security auditor. Look for privileged containers, hostPath volumes, hostNetwork/hostPID usage, containers running as root without need, images pinned to the latest tag, and missing securityContext hardening. Report the most severe finding and, when the fix is safe and mechanical (e.g. pinning a tag, adding runAsNonRoot), propose the corresponding update. Never propose changes that could break a workload whose requirements you do not know.",
				Enabled:     false,
				Scope:       allNamespaces,
				Triggers: v1alpha1.AgentTriggers{
					Cron:   "0 8 * * 1",
					Manual: true,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "best-practices-advisor"},
			Spec: v1alpha1.AgentSpec{
				DisplayName: "Best Practices Advisor",
				Description: "Checks workloads against operational best practices: probes, replica counts, update strategies and labels.",
				Icon:        "fa-clipboard-check",
				Instruction: "You are a Kubernetes reliability reviewer. Check workloads for missing liveness/readiness probes, single-replica deployments serving traffic, missing pod disruption budgets for multi-replica workloads, and absent standard labels. Pick the workload where a fix has the highest operational value and propose the corresponding update — for example adding a readiness probe that matches an exposed port. Keep proposals minimal and safe.",
				Enabled:     false,
				Scope:       allNamespaces,
				Triggers: v1alpha1.AgentTriggers{
					Cron:   "0 9 * * 1",
					Manual: true,
				},
			},
		},
	}
}

// SeedDefaultAgents creates the built-in default agents exactly once per
// cluster (guarded by a marker ConfigMap, so customer deletions stick). Meant
// to run on the leader — concurrent replicas would only race into
// AlreadyExists errors, which are tolerated anyway.
func SeedDefaultAgents(logger *slog.Logger, config cfg.ConfigModule, clientProvider k8sclient.K8sClientProvider, workspaceManager WorkspaceManager) {
	namespace := config.Get("MO_OWN_NAMESPACE")
	client := clientProvider.K8sClientSet()

	ctx := context.Background()
	_, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, DEFAULT_AGENTS_MARKER_CONFIGMAP, metav1.GetOptions{})
	if err == nil {
		return // already seeded once
	}
	if !errors.IsNotFound(err) {
		logger.Error("seed default agents: failed to check marker configmap", "error", err)
		return
	}

	seeded := 0
	for _, agent := range defaultAgents() {
		_, err := workspaceManager.CreateAgent(agent.Name, agent.Spec)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				continue
			}
			logger.Error("seed default agents: failed to create agent", "agent", agent.Name, "error", err)
			return // retry on next leadership without the marker
		}
		seeded++
	}

	marker := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DEFAULT_AGENTS_MARKER_CONFIGMAP,
			Namespace: namespace,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "mogenius-k8s-manager"},
		},
		Data: map[string]string{
			"seededAt": time.Now().UTC().Format(time.RFC3339),
			"agents":   fmt.Sprintf("%d", seeded),
		},
	}
	if _, err := client.CoreV1().ConfigMaps(namespace).Create(ctx, marker, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		logger.Error("seed default agents: failed to create marker configmap", "error", err)
		return
	}
	logger.Info("seeded default AI agents", "count", seeded)
}
