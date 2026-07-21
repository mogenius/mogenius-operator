package ai

import (
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/store"
	"sort"
)

// DefaultApiKeySecretKey is the Secret data key holding the API key when an
// AiModel's apiKeySecretRef does not name one explicitly.
const DefaultApiKeySecretKey = "API_KEY"

// ResolvedModelConfig is one fully resolved model configuration — provider,
// model name, endpoint, credentials and per-run limits. It is resolved once
// at the start of a run/chat and passed down, so a single run never mixes
// settings from different sources.
type ResolvedModelConfig struct {
	// Source describes where this config came from ("modelRef", "default" or
	// "legacy-secret") for logs and status reporting.
	Source string

	Sdk     AiSdkType
	Model   string
	ApiKey  string
	BaseUrl string

	MaxToolCalls    int
	MaxTokensPerRun int64
}

// parseSdkType maps the wire value to a known SDK type.
func parseSdkType(value string) (AiSdkType, error) {
	switch value {
	case "openai":
		return AiSdkTypeOpenAI, nil
	case "anthropic":
		return AiSdkTypeAnthropic, nil
	case "ollama":
		return AiSdkTypeOllama, nil
	default:
		return "", fmt.Errorf("unsupported SDK type: %s", value)
	}
}

// ValidateAiModelSpec checks an AiModel spec for the invariants resolution
// relies on: a known SDK, a model name, an endpoint for providers without a
// public default, and credentials for providers that authenticate.
func ValidateAiModelSpec(spec v1alpha1.AiModelSpec) error {
	sdk, err := parseSdkType(spec.Sdk)
	if err != nil {
		return err
	}
	if spec.Model == "" {
		return fmt.Errorf("model must not be empty")
	}
	switch sdk {
	case AiSdkTypeOllama:
		if spec.ApiUrl == "" {
			return fmt.Errorf("apiUrl is required for sdk %q", spec.Sdk)
		}
	case AiSdkTypeOpenAI, AiSdkTypeAnthropic:
		if spec.ApiKeySecretRef == nil || spec.ApiKeySecretRef.Name == "" {
			return fmt.Errorf("apiKeySecretRef is required for sdk %q", spec.Sdk)
		}
	}
	return nil
}

// resolveApiKeyFromRef reads the API key an AiModel points at. Mirrors
// getAiSettingByKey: store cache first, then a direct API read.
func (ai *aiManager) resolveApiKeyFromRef(namespace string, ref *v1alpha1.SecretKeyRef) (string, error) {
	if ref == nil || ref.Name == "" {
		return "", nil
	}
	key := ref.Key
	if key == "" {
		key = DefaultApiKeySecretKey
	}
	secret := store.GetSecret(namespace, ref.Name)
	if secret == nil {
		var err error
		secret, err = ai.secretGetter(namespace, ref.Name)
		if err != nil {
			return "", fmt.Errorf("secret %q not found in namespace %q: %w", ref.Name, namespace, err)
		}
	}
	data, exists := secret.Data[key]
	if !exists || len(data) == 0 {
		return "", fmt.Errorf("key %q not found in secret %q", key, ref.Name)
	}
	return string(data), nil
}

// listAiModels returns all AiModel CRs in the operator namespace.
func (ai *aiManager) listAiModels() ([]v1alpha1.AiModel, error) {
	ownNamespace, err := ai.config.TryGet("MO_OWN_NAMESPACE")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve own namespace: %w", err)
	}
	return store.GetAllAiModels(ownNamespace)
}

// getDefaultAiModel returns the effective default AiModel or nil when none is
// marked.
func (ai *aiManager) getDefaultAiModel() *v1alpha1.AiModel {
	models, err := ai.listAiModels()
	if err != nil {
		ai.logger.Warn("getDefaultAiModel: failed to list AiModels", "error", err)
		return nil
	}
	return pickDefaultAiModel(models)
}

// pickDefaultAiModel selects the effective default from a list of AiModels.
// When several carry default=true the oldest wins (name as tie-breaker), so
// the result is deterministic.
func pickDefaultAiModel(models []v1alpha1.AiModel) *v1alpha1.AiModel {
	defaults := make([]v1alpha1.AiModel, 0, 1)
	for _, model := range models {
		if model.Spec.Default {
			defaults = append(defaults, model)
		}
	}
	if len(defaults) == 0 {
		return nil
	}
	sort.Slice(defaults, func(i, j int) bool {
		ti, tj := defaults[i].CreationTimestamp, defaults[j].CreationTimestamp
		if !ti.Equal(&tj) {
			return ti.Before(&tj)
		}
		return defaults[i].Name < defaults[j].Name
	})
	return &defaults[0]
}

// resolveAiModel turns one AiModel CR into a usable config, reading the
// referenced API key Secret. Per-model limits fall back to the global ones.
func (ai *aiManager) resolveAiModel(model *v1alpha1.AiModel, source string) (*ResolvedModelConfig, error) {
	if err := ValidateAiModelSpec(model.Spec); err != nil {
		return nil, fmt.Errorf("AiModel %q has an invalid spec: %w", model.Name, err)
	}
	sdk, err := parseSdkType(model.Spec.Sdk)
	if err != nil {
		return nil, fmt.Errorf("AiModel %q: %w", model.Name, err)
	}
	apiKey, err := ai.resolveApiKeyFromRef(model.Namespace, model.Spec.ApiKeySecretRef)
	if err != nil {
		return nil, fmt.Errorf("AiModel %q: %w", model.Name, err)
	}

	resolved := &ResolvedModelConfig{
		Source:  source,
		Sdk:     sdk,
		Model:   model.Spec.Model,
		ApiKey:  apiKey,
		BaseUrl: model.Spec.ApiUrl,
	}

	if model.Spec.MaxToolCalls != nil {
		resolved.MaxToolCalls = *model.Spec.MaxToolCalls
	} else {
		maxToolCalls, err := ai.getAiMaxToolCalls()
		if err != nil {
			ai.logger.Debug("AiModel resolution: no global max tool calls, using default", "model", model.Name, "default", maxToolCalls)
		}
		resolved.MaxToolCalls = maxToolCalls
	}

	if model.Spec.MaxTokensPerRun != nil {
		resolved.MaxTokensPerRun = *model.Spec.MaxTokensPerRun
	} else {
		resolved.MaxTokensPerRun = ai.getAiMaxTokensPerRun()
	}

	return resolved, nil
}

// resolveLegacyModelConfig builds the config from the flat mogenius-ai-config
// secret — the pre-AiModel configuration path, kept as fallback so existing
// installations keep working until they are migrated.
func (ai *aiManager) resolveLegacyModelConfig() (*ResolvedModelConfig, error) {
	sdk, err := ai.getSdkType()
	if err != nil {
		return nil, err
	}
	model, err := ai.getAiModel()
	if err != nil {
		return nil, err
	}
	baseUrl, err := ai.getBaseUrl()
	if err != nil {
		return nil, err
	}
	apiKey := ""
	if sdk != AiSdkTypeOllama {
		apiKey, err = ai.getApiKey()
		if err != nil {
			return nil, err
		}
	}
	maxToolCalls, err := ai.getAiMaxToolCalls()
	if err != nil {
		ai.logger.Debug("Legacy model resolution: no max tool calls configured, using default", "default", maxToolCalls)
	}
	return &ResolvedModelConfig{
		Source:          "legacy-secret",
		Sdk:             sdk,
		Model:           model,
		ApiKey:          apiKey,
		BaseUrl:         baseUrl,
		MaxToolCalls:    maxToolCalls,
		MaxTokensPerRun: ai.getAiMaxTokensPerRun(),
	}, nil
}

// resolveModelConfig resolves the model configuration for one run or chat.
// Precedence: the agent's explicit modelRef, then the AiModel marked default,
// then the legacy mogenius-ai-config secret. An explicit modelRef that cannot
// be resolved fails the run (fail closed) instead of silently degrading to a
// different provider. The deprecated agentSpec.Model name-only override is
// honored when no modelRef is set, preserving the old behavior.
func (ai *aiManager) resolveModelConfig(agentSpec *v1alpha1.AgentSpec) (*ResolvedModelConfig, error) {
	if agentSpec != nil && agentSpec.ModelRef != "" {
		ownNamespace, err := ai.config.TryGet("MO_OWN_NAMESPACE")
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve own namespace: %w", err)
		}
		model, err := store.GetAiModel(ownNamespace, agentSpec.ModelRef)
		if err != nil {
			return nil, fmt.Errorf("failed to get AiModel %q: %w", agentSpec.ModelRef, err)
		}
		if model == nil {
			return nil, fmt.Errorf("AiModel %q not found in namespace %q", agentSpec.ModelRef, ownNamespace)
		}
		return ai.resolveAiModel(model, "modelRef")
	}

	var resolved *ResolvedModelConfig
	if defaultModel := ai.getDefaultAiModel(); defaultModel != nil {
		var err error
		resolved, err = ai.resolveAiModel(defaultModel, "default")
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		resolved, err = ai.resolveLegacyModelConfig()
		if err != nil {
			// The raw legacy error ("secret mogenius-ai-config not found") is
			// misleading when the customer already uses AiModels but forgot to
			// mark one as default — say so explicitly.
			if models, listErr := ai.listAiModels(); listErr == nil && len(models) > 0 {
				return nil, fmt.Errorf("no AiModel is marked as default (spec.default: true) and no legacy AI config is available: %w", err)
			}
			return nil, err
		}
	}

	if agentSpec != nil && agentSpec.Model != "" {
		resolved.Model = agentSpec.Model
	}
	return resolved, nil
}
