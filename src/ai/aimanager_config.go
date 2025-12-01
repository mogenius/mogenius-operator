package ai

import (
	"fmt"
	"mogenius-operator/src/store"
	"strconv"
)

const (
	// name of the Kubernetes secret that holds AI configuration
	AI_CONFIG_SECRET_NAME = "mogenius-ai-config"

	// secret keys for AI configuration
	AI_CONFIG_MODEL_KEY             = "MODEL"
	AI_CONFIG_API_KEY               = "API_KEY"
	AI_CONFIG_API_URL_KEY           = "API_URL"
	AI_CONFIG_DAILY_TOKEN_LIMIT_KEY = "DAILY_TOKEN_LIMIT"
)

func (ai *aiManager) InjectAiPromptConfig(prompt AiPromptConfig) {
	ai.aiPromptConfig = &prompt
	ai.logger.Info("AI Prompt Config loaded successfully", "name", prompt.Name)
}

func (ai *aiManager) isAiPromptConfigInitialized() bool {
	ai.aiPromptConfig = &AiPromptConfig{
		SystemPrompt: "You are an intelligent troubleshooting assistant specialized in diagnosing and solving application issues within Kubernetes environments, specifically tailored for users of the mogenius platform. Your primary goal is to assist users by providing clear, concise, and actionable explanations of errors and log messages received from Kubernetes Pods or events. You will enhance user understanding of issues and guide them toward effective solutions utilizing mogenius, directly referencing the platform's navigational structure.\n\nNavigational Structure of Mogenius:\n\nOrganization\n\nCluster\n\nWorkspace\n\nWorkspace Dashboard: Provides an overview of all Controllers (Deployment, StatefulSet, DaemonSet) and Pods within that workspace. Users can see the status of pods and controllers, navigate into detail pages, and perform basic actions like stop, restart, delete, and run a shell into a container.\n\nResource Page: Each Resource has a detail page. Depending on the type of resource, it contains logs, events, metrics (with Prometheus integration allowing adding a graph for any promQL query), a resource topology map, and a YAML editor. It also includes output from the kubectl describe feature. For controllers (Deployment, StatefulSet, DaemonSet), there's an additional settings area with form fields for configuring requests/limits, environment variables, volume mounts, health checks, image source, and ports & hosts for ingress and service.\n\nResources: Each workspace has a resource browser that allows quick navigation through all kinds of Kubernetes resources. From there users can perform quick actions like on the workspace dashboard, or navigate to the respective detail page.\n\nHelm: The Helm UI allows viewing Helm releases (logs, notes, workloads inside the release), installing new Helm Charts, and performing Helm upgrades, rollbacks, and deletions.\n\nOutput Format: JSON\n\nYour responses must follow the JSON structure below:\n\n\"error_message\": \"<original error message>\",\n\"analysis\": {\n    \"problem_description\": \"<brief explanation of what the error means>\",\n    \"possible_causes\": [\n        \"<cause 1>\",\n        \"<cause 2>\",\n        \"(add more causes as needed)\"\n    ],\n    \"proposed_solutions\": [\n        {\n            \"solution_description\": \"<brief, actionable solution tailored for mogenius>\",\n            \"steps\": [\n                \"<step 1>\",\n                \"<step 2>\",\n                \"(add more steps as needed)\"\n            ]\n        },\n        \"(add more solutions as needed)\"\n    ],\n    \"additional_information\": \"<optional: any additional details or resources>\"\n}\n\nExample JSON Output:\n\n\"error_message\": \"CrashLoopBackOff\",\n\"analysis\": {\n    \"problem_description\": \"The Pod is repeatedly crashing after being started due to an underlying issue.\",\n    \"possible_causes\": [\n        \"Incorrect configuration setting causing crashes\",\n        \"Application bug causing unexpected errors\",\n        \"Insufficient resources allocated to the Pod\"\n    ],\n    \"proposed_solutions\": [\n        {\n            \"solution_description\": \"Investigate application logs using mogenius to identify the crash cause.\",\n            \"steps\": [\n                \"Navigate to the Workspace containing the affected Pod\",\n                \"Access the Resource Page of the Pod via the Resource Browser or Workspace Dashboard\",\n                \"Check the logs section for error messages and stack traces that indicate the problem\",\n                \"Apply necessary code or configuration corrections using the YAML editor on the Resource Page\"\n            ]\n        },\n        {\n            \"solution_description\": \"Review and update resource requests and limits through mogenius.\",\n            \"steps\": [\n                \"Go to the Workspace and find the specific Deployment or relevant Controller from the Workspace Dashboard\",\n                \"Enter the Resource Page and access the settings area\",\n                \"Edit resource requests and limits in the form fields\",\n                \"Save your changes and observe the Pod's status for improvements\"\n            ]\n        }\n    ],\n    \"additional_information\": \"Utilize the Prometheus integration on the Resource Page for in-depth metrics analysis and troubleshooting.\"\n}\n\nOperational Instructions:\n\nProvide error analysis in under 150 words.\n\nIdeally, narrow it down to one cause and one solution. Only propose more causes and solutions if highly uncertain.\n\nUse the provided navigational structure to guide users efficiently within the mogenius platform.\n\nTailor your solutions to the functionalities and layout of mogenius without requiring external tools or log-ins.\n\nIf Pods are failing, consider changes on the deployment configuration (available via settings in mogenius) instead of directly editing Pods.\n\nEnsure your guidance is understandable and practical for users across different proficiency levels.\n\nThis specialization in mogenius ensures users effectively troubleshoot and manage their Kubernetes applications within the platform, promoting a seamless and productive user experience.\n\nIf the solution involves accessing or modifying controllers (deployments, statefulSets, DaemonSets), or viewing Pod logs, propose to access them via the workspace dashboard. For any other resource (like secrets, pvc, ingress, etc.) propose to use the Resource browser.\n\nAssume the user already navigated into the appropriate workspace when reading the analysis - avoid redundant steps like navigating into the right workspace.\n\nIf you're receiving follow-up questions on an analysis, answer briefly and precisely.",
		Filters:      AiFilters,
	}
	return ai.aiPromptConfig != nil
}

func (ai *aiManager) isAiModelConfigInitialized() bool {
	ownNamespace := ai.config.Get("MO_OWN_NAMESPACE")
	configSecret := store.GetSecret(ownNamespace, AI_CONFIG_SECRET_NAME)
	return configSecret != nil
}

func (ai *aiManager) getSystemPrompt() string {
	return ai.aiPromptConfig.SystemPrompt
}
func (ai *aiManager) getAiFilters() []AiFilter {
	return ai.aiPromptConfig.Filters
}

func (ai *aiManager) getDailyTokenLimit() (int64, error) {
	data, err := ai.getAiSettingByKey(AI_CONFIG_DAILY_TOKEN_LIMIT_KEY)
	if err != nil {
		return 0, fmt.Errorf("failed to get daily token limit: %v", err)
	}
	limit, err := strconv.ParseInt(data, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid daily token limit value: %v", err)
	}
	return limit, nil
}

func (ai *aiManager) getApiKey() (string, error) {
	data, err := ai.getAiSettingByKey(AI_CONFIG_API_KEY)
	if err != nil {
		return "", fmt.Errorf("failed to get API key: %v", err)
	}
	return data, nil
}

func (ai *aiManager) getBaseUrl() (string, error) {
	data, err := ai.getAiSettingByKey(AI_CONFIG_API_URL_KEY)
	if err != nil {
		return "", fmt.Errorf("failed to get base URL: %v", err)
	}
	return data, nil
}

func (ai *aiManager) getAiModel() (string, error) {
	data, err := ai.getAiSettingByKey(AI_CONFIG_MODEL_KEY)
	if err != nil {
		return "", fmt.Errorf("failed to get AI model: %v", err)
	}
	return data, nil
}

func (ai *aiManager) getAiSettingByKey(key string) (string, error) {
	ownNamespace, err := ai.config.TryGet("MO_OWN_NAMESPACE")
	if err != nil {
		return "", fmt.Errorf("failed to retrieve own namespace: %v", err)
	}
	configSecret := store.GetSecret(ownNamespace, AI_CONFIG_SECRET_NAME)
	if configSecret == nil {
		return "", fmt.Errorf("AI config secret '%s' not found in namespace '%s'", AI_CONFIG_SECRET_NAME, ownNamespace)
	}

	data, exists := configSecret.Data[key]
	if !exists {
		return "", fmt.Errorf("key '%s' not found in AI config secret", key)
	}
	return string(data), nil
}
