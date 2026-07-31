package ai

import (
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/store"
	"sort"
	"strings"
)

// DefaultApiKeySecretKey is the Secret data key holding the API key when an
// AiModel's apiKeySecretRef does not name one explicitly.
const DefaultApiKeySecretKey = "API_KEY"

// Public default endpoints of the provider SDKs (see WithBaseURL handling in
// newOpenAIClient/newAnthropicClient: empty BaseUrl selects these). Kept here
// so clients can show them and stored specs restating them can be normalized
// back to empty.
const (
	openAIDefaultApiUrl    = "https://api.openai.com/v1"
	anthropicDefaultApiUrl = "https://api.anthropic.com"
)

// AiSdkInfo describes one supported provider SDK for clients rendering model
// configuration forms: which endpoint is used when apiUrl stays empty and
// which spec fields the SDK requires.
type AiSdkInfo struct {
	Sdk         string `json:"sdk"`
	DisplayName string `json:"displayName"`

	// DefaultApiUrl is the public endpoint used when spec.apiUrl is empty;
	// empty when the SDK has no public default (ollama). Meant as a
	// placeholder — clients should leave apiUrl empty unless the customer
	// uses a proxy or compatible endpoint.
	DefaultApiUrl string `json:"defaultApiUrl,omitempty"`

	ApiUrlRequired bool `json:"apiUrlRequired"`
	ApiKeyRequired bool `json:"apiKeyRequired"`
}

// SupportedAiSdks lists the provider SDKs an AiModel may declare, in UI order.
func SupportedAiSdks() []AiSdkInfo {
	return []AiSdkInfo{
		{
			Sdk:            string(AiSdkTypeOpenAI),
			DisplayName:    "OpenAI",
			DefaultApiUrl:  openAIDefaultApiUrl,
			ApiKeyRequired: true,
		},
		{
			Sdk:            string(AiSdkTypeAnthropic),
			DisplayName:    "Anthropic",
			DefaultApiUrl:  anthropicDefaultApiUrl,
			ApiKeyRequired: true,
		},
		{
			Sdk:            string(AiSdkTypeOllama),
			DisplayName:    "Ollama",
			ApiUrlRequired: true,
		},
	}
}

// defaultApiUrlForSdk returns the SDK's public default endpoint, or "" when it
// has none.
func defaultApiUrlForSdk(sdk string) string {
	switch sdk {
	case string(AiSdkTypeOpenAI):
		return openAIDefaultApiUrl
	case string(AiSdkTypeAnthropic):
		return anthropicDefaultApiUrl
	default:
		return ""
	}
}

// NormalizeAiModelSpec clears an apiUrl that merely restates the SDK's public
// default endpoint. An empty apiUrl keeps following the SDK default across
// updates, while a stored literal URL would silently pin it forever.
func NormalizeAiModelSpec(spec v1alpha1.AiModelSpec) v1alpha1.AiModelSpec {
	spec.ApiUrl = strings.TrimSpace(spec.ApiUrl)
	if defaultUrl := defaultApiUrlForSdk(spec.Sdk); defaultUrl != "" &&
		strings.TrimSuffix(spec.ApiUrl, "/") == defaultUrl {
		spec.ApiUrl = ""
	}
	return spec
}

// Built-in fallbacks applied when neither the agent nor the AiModel spec set
// a limit. They replace the former global settings from the legacy
// mogenius-ai-config secret — limits are configured per model/agent now.
const (
	defaultMaxToolCalls    = 50
	defaultMaxTokensPerRun = int64(30_000)
	defaultDailyTokenLimit = int64(300_000)
)

// ResolvedModelConfig is one fully resolved model configuration — provider,
// model name, endpoint, credentials and per-run limits. It is resolved once
// at the start of a run/chat and passed down, so a single run never mixes
// settings from different sources.
type ResolvedModelConfig struct {
	// Source describes where this config came from ("modelRef" or "default")
	// for logs and status reporting.
	Source string

	// ModelCrName is the name of the AiModel CR this config was resolved
	// from; token usage is accounted against it.
	ModelCrName string

	Sdk     AiSdkType
	Model   string
	ApiKey  string
	BaseUrl string

	MaxToolCalls    int
	MaxTokensPerRun int64

	// DailyTokenLimit is the model's daily token budget; 0 means unlimited.
	DailyTokenLimit int64
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
	return PickDefaultAiModel(models)
}

// ValidateAiModelDefaultUnique rejects marking a model as default while a
// different AiModel already carries the flag. Guards the API write path;
// duplicates created behind its back (kubectl/GitOps) are surfaced by the
// reconciler's DuplicateDefault condition and resolved deterministically by
// PickDefaultAiModel.
func ValidateAiModelDefaultUnique(name string, spec v1alpha1.AiModelSpec, existing []v1alpha1.AiModel) error {
	if !spec.Default {
		return nil
	}
	for _, model := range existing {
		if model.Name != name && model.Spec.Default {
			return fmt.Errorf("AiModel %q is already marked as default — exactly one model may be the cluster default; unset its default flag first", model.Name)
		}
	}
	return nil
}

// PickDefaultAiModel selects the effective default from a list of AiModels.
// When several carry default=true the oldest wins (name as tie-breaker), so
// the result is deterministic.
func PickDefaultAiModel(models []v1alpha1.AiModel) *v1alpha1.AiModel {
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
// referenced API key Secret. Limits the spec omits fall back to the built-in
// defaults.
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
		Source:          source,
		ModelCrName:     model.Name,
		Sdk:             sdk,
		Model:           model.Spec.Model,
		ApiKey:          apiKey,
		BaseUrl:         model.Spec.ApiUrl,
		MaxToolCalls:    defaultMaxToolCalls,
		MaxTokensPerRun: defaultMaxTokensPerRun,
		DailyTokenLimit: defaultDailyTokenLimit,
	}
	if model.Spec.MaxToolCalls != nil {
		resolved.MaxToolCalls = *model.Spec.MaxToolCalls
	}
	if model.Spec.MaxTokensPerRun != nil {
		resolved.MaxTokensPerRun = *model.Spec.MaxTokensPerRun
	}
	if model.Spec.DailyTokenLimit != nil {
		resolved.DailyTokenLimit = *model.Spec.DailyTokenLimit
	}
	return resolved, nil
}

// resolveModelConfig resolves the model configuration for one run or chat.
// Precedence: the agent's explicit modelRef, then the AiModel marked default.
// An explicit modelRef that cannot be resolved fails the run (fail closed)
// instead of silently degrading to a different provider. Per-run budget
// overrides from the agent spec beat the model's values.
func (ai *aiManager) resolveModelConfig(agentSpec *v1alpha1.AgentSpec) (*ResolvedModelConfig, error) {
	var resolved *ResolvedModelConfig
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
		resolved, err = ai.resolveAiModel(model, "modelRef")
		if err != nil {
			return nil, err
		}
	} else if defaultModel := ai.getDefaultAiModel(); defaultModel != nil {
		var err error
		resolved, err = ai.resolveAiModel(defaultModel, "default")
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("no AiModel is marked as default (spec.default: true) — create an AiModel or mark an existing one as default")
	}

	applyAgentBudgetOverrides(resolved, agentSpec)
	return resolved, nil
}

// resolveChatModelConfig resolves the model for one chat session. Chat is
// driven purely by the per-model chatEnabled flag; see pickChatModel for the
// selection rules.
func (ai *aiManager) resolveChatModelConfig(modelRef string) (*ResolvedModelConfig, error) {
	models, err := ai.listAiModels()
	if err != nil {
		return nil, fmt.Errorf("failed to list AI models: %w", err)
	}
	model, err := pickChatModel(models, modelRef)
	if err != nil {
		return nil, err
	}
	return ai.resolveAiModel(model, "chat")
}

// pickChatModel selects the chat model: an explicit modelRef must name a
// chat-enabled model (fail closed), otherwise the default-flagged model wins
// if it is chat-enabled, else the oldest chat-enabled one (name as
// tie-breaker — the same deterministic ordering as the default election).
// No chat-enabled model at all is a user-facing configuration error.
func pickChatModel(models []v1alpha1.AiModel, modelRef string) (*v1alpha1.AiModel, error) {
	chatModels := make([]v1alpha1.AiModel, 0, len(models))
	for _, model := range models {
		if model.Spec.ChatEnabled {
			chatModels = append(chatModels, model)
		}
	}

	if modelRef != "" {
		for i := range chatModels {
			if chatModels[i].Name == modelRef {
				return &chatModels[i], nil
			}
		}
		for i := range models {
			if models[i].Name == modelRef {
				return nil, fmt.Errorf("AI model %q is not enabled for chat — enable it in Cluster Settings → AI", modelRef)
			}
		}
		return nil, fmt.Errorf("AI model %q not found", modelRef)
	}

	if len(chatModels) == 0 {
		return nil, fmt.Errorf("no AI model is enabled for chat — enable a model for chat in Cluster Settings → AI")
	}
	for i := range chatModels {
		if chatModels[i].Spec.Default {
			return &chatModels[i], nil
		}
	}
	sort.Slice(chatModels, func(i, j int) bool {
		ti, tj := chatModels[i].CreationTimestamp, chatModels[j].CreationTimestamp
		if !ti.Equal(&tj) {
			return ti.Before(&tj)
		}
		return chatModels[i].Name < chatModels[j].Name
	})
	return &chatModels[0], nil
}

// applyAgentBudgetOverrides lets an agent's per-run budget fields beat the
// model's values (precedence: agent > model spec > built-in defaults).
func applyAgentBudgetOverrides(resolved *ResolvedModelConfig, agentSpec *v1alpha1.AgentSpec) {
	if agentSpec == nil {
		return
	}
	if agentSpec.MaxToolCalls != nil {
		resolved.MaxToolCalls = *agentSpec.MaxToolCalls
	}
	if agentSpec.MaxTokensPerRun != nil {
		resolved.MaxTokensPerRun = *agentSpec.MaxTokensPerRun
	}
}
