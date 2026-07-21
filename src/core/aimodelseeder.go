package core

import (
	"context"
	"log/slog"
	"mogenius-operator/src/ai"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/k8sclient"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MOGENIUS_AI_DEFAULT_MODEL_MARKER_CONFIGMAP records that the legacy flat AI
// config secret was migrated into a default AiModel once. Migration never runs
// again while it exists, so deleting the seeded AiModel is permanent (until
// the marker itself is deleted).
const MOGENIUS_AI_DEFAULT_MODEL_MARKER_CONFIGMAP = "mogenius-ai-default-model-seeded"

// SeedDefaultAiModel migrates the legacy flat AI config secret
// (mogenius-ai-config) into a default AiModel CR exactly once per cluster
// (guarded by a marker ConfigMap). The API key stays in the legacy secret and
// is referenced from the AiModel; no new secret is created. Meant to run on
// the leader — concurrent replicas would only race into AlreadyExists errors,
// which are tolerated anyway.
func SeedDefaultAiModel(logger *slog.Logger, config cfg.ConfigModule, clientProvider k8sclient.K8sClientProvider) {
	namespace := config.Get("MO_OWN_NAMESPACE")
	client := clientProvider.K8sClientSet()

	ctx := context.Background()
	_, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, MOGENIUS_AI_DEFAULT_MODEL_MARKER_CONFIGMAP, metav1.GetOptions{})
	if err == nil {
		return // already migrated once
	}
	if !errors.IsNotFound(err) {
		logger.Error("seed default aimodel: failed to check marker configmap", "error", err)
		return
	}

	// Read the legacy secret. Missing or incomplete config means there is
	// nothing to migrate yet — do NOT write the marker, so a secret configured
	// later still gets migrated on a future leadership.
	legacySecret, err := client.CoreV1().Secrets(namespace).Get(ctx, ai.AI_CONFIG_SECRET_NAME, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Debug("seed default aimodel: legacy ai config secret not found, nothing to migrate", "secret", ai.AI_CONFIG_SECRET_NAME)
		} else {
			logger.Error("seed default aimodel: failed to read legacy ai config secret", "secret", ai.AI_CONFIG_SECRET_NAME, "error", err)
		}
		return
	}
	sdk := string(legacySecret.Data[ai.AI_CONFIG_SDK_KEY])
	model := string(legacySecret.Data[ai.AI_CONFIG_MODEL_KEY])
	if sdk == "" || model == "" {
		logger.Debug("seed default aimodel: legacy ai config secret lacks SDK or MODEL, nothing to migrate", "secret", ai.AI_CONFIG_SECRET_NAME)
		return
	}

	// If the customer already adopted AiModels, only write the marker. A list
	// error leaves us unable to tell — abort without the marker rather than
	// risking a duplicate default model.
	existing, err := clientProvider.MogeniusClientSet().MogeniusV1alpha1.ListAiModels(namespace)
	if err != nil {
		logger.Error("seed default aimodel: failed to list existing aimodels", "error", err)
		return // retry on next leadership without the marker
	}
	if len(existing) == 0 {
		spec := ai.NormalizeAiModelSpec(v1alpha1.AiModelSpec{
			DisplayName: "Default",
			Sdk:         sdk,
			Model:       model,
			ApiUrl:      string(legacySecret.Data[ai.AI_CONFIG_API_URL_KEY]),
			Default:     true,
		})
		// The API key stays in the legacy secret; the AiModel just points at
		// it. Ollama never authenticates, so it gets no reference at all.
		if sdk != string(ai.AiSdkTypeOllama) && len(legacySecret.Data[ai.AI_CONFIG_API_KEY]) > 0 {
			spec.ApiKeySecretRef = &v1alpha1.SecretKeyRef{
				Name: ai.AI_CONFIG_SECRET_NAME,
				Key:  ai.AI_CONFIG_API_KEY,
			}
		}
		if _, err := clientProvider.MogeniusClientSet().MogeniusV1alpha1.CreateAiModel(namespace, "default", spec); err != nil && !errors.IsAlreadyExists(err) {
			logger.Error("seed default aimodel: failed to create aimodel", "error", err)
			return // retry on next leadership without the marker
		}
	}

	marker := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MOGENIUS_AI_DEFAULT_MODEL_MARKER_CONFIGMAP,
			Namespace: namespace,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "mogenius-k8s-manager"},
		},
		Data: map[string]string{
			"seededAt": time.Now().UTC().Format(time.RFC3339),
		},
	}
	if _, err := client.CoreV1().ConfigMaps(namespace).Create(ctx, marker, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		logger.Error("seed default aimodel: failed to create marker configmap", "error", err)
		return
	}
	if len(existing) == 0 {
		logger.Info("seeded default AiModel from legacy ai config secret")
	}
}
