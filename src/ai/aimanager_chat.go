package ai

import (
	"context"
	"fmt"
	"strings"

	json "github.com/goccy/go-json"
)

const mogeniusCRDsPrompt = `## Mogenius Operator Custom Resource Definitions (CRDs)

The mogenius-operator manages three core CRDs that work together to provide workspace management and access control:

### 1. Workspace CRD

**API Details:**
- apiVersion: mogenius.com/v1alpha1
- kind: Workspace
- Scope: Namespaced

**Purpose:**
A Workspace is a logical organizational unit that groups and manages various Kubernetes resources as a cohesive entity. It serves as a meta-resource containing references to other resources.

**Spec Structure:**
- spec.name (string, optional): Human-readable name for the workspace
- spec.resources (array): List of managed resources with the following properties:
  - id (string): Name/identifier of the target resource
  - type (string): Resource type - allowed values: "namespace", "helm", "argocd"
  - namespace (string): 
    - When type="namespace": unused
    - When type="helm": Namespace in which the Helm chart was installed
    - When type="argocd": Namespace in which the ArgoCD application was installed

---

### 2. User CRD

**API Details:**
- apiVersion: mogenius.com/v1alpha1
- kind: User
- Scope: Namespaced

**Purpose:**
A User resource contains information about a user on the mogenius platform and maps the mogenius user to Kubernetes RBAC subjects (User, Group, or ServiceAccount).

**Spec Structure:**
- spec.email (string, optional): User's email address
- spec.firstName (string, optional): User's first name
- spec.lastName (string, optional): User's last name
- spec.subject (object, required): RBAC subject reference
  - kind (string, required): Type of subject - "User", "Group", or "ServiceAccount"
  - name (string, required): Name of the subject
  - apiGroup (string, optional): API group (defaults to "" for ServiceAccount, "rbac.authorization.k8s.io" for User/Group)
  - namespace (string, optional): Namespace for ServiceAccount subjects (must be empty for User/Group)

**Subject Types:**
- **User**: Human or system account authenticated by Kubernetes
- **Group**: Collection of users (membership established by authentication provider)
- **ServiceAccount**: Kubernetes resource acting as identity for processes in Pods

---

### 3. Grant CRD

**API Details:**
- apiVersion: mogenius.com/v1alpha1
- kind: Grant
- Scope: Namespaced

**Purpose:**
A Grant assigns permissions for mogenius User or Team resources to mogenius Workspace resources. It creates the link between users and the workspaces they can access.

**Spec Structure:**
- spec.grantee (string): Who is granted permission (user.metadata.name or team.metadata.name)
- spec.role (string): Which permissions are granted - allowed values: "viewer", "editor", "admin"
- spec.targetType (string): Type of target resource - currently only "workspace" is supported
- spec.targetName (string): Name of the specific Workspace resource (workspace.metadata.name)

**Role Definitions:**
- **viewer**: Read-only access to workspace resources
- **editor**: Can modify workspace resources
- **admin**: Full administrative access to the workspace

---

## How They Work Together

The three CRDs form a complete access control and resource management system:

1. **Workspaces** define logical groupings of Kubernetes resources (namespaces, Helm charts, ArgoCD applications)
2. **Users** represent platform users and map them to Kubernetes RBAC subjects
3. **Grants** connect Users to Workspaces with specific permission levels (viewer/editor/admin)

**Example Flow:**
- A Workspace named "production" contains a namespace, several Helm charts, and ArgoCD applications
- A User named "jane" is mapped to a Kubernetes User subject
- A Grant gives "jane" the "editor" role on the "production" workspace
- Result: Jane can modify resources within the production workspace according to her editor permissions

This architecture enables multi-tenant scenarios where different users have different levels of access to different workspaces, while maintaining clean separation of concerns.`

const mogeniusGithubAiContextPrompt = `

---
## Extended Instructions for GitHub Integration

### Automatic GitHub Authentication
**Step 1: GitHub Login Check**
- At the start of a new conversation: Check if a GitHub PAT (Personal Access Token) is available
- If available: Automatically execute 'get_me' to confirm authentication and retrieve user information
- Store the GitHub username for later reference

### Repository as Knowledge Base
**Step 2: Repository Context Loading**
- When the repository '{{GITHUB_REPO}}' exists and provided by the user:
  1. Load the repository structure using 'get_file_contents' (root directory)
  2. Identify important files (README.md, docs/, important configuration files)
  3. **Load '.ai-context.md' file immediately if it exists** - this file contains critical conventions and preferences
  4. Load relevant documentation and context
  5. Use this repository content as the primary knowledge base for all subsequent questions
  6. In case of ambiguity: Reference specific files from the repository

**Step 3: Context Awareness**
- Maintain the repository context active throughout the entire conversation
- When answering questions: Prioritize information from the loaded repository
- For code examples: Follow the patterns and conventions from the repository
- **Apply conventions from '.ai-context.md' to all operations** (e.g., standard labels, naming conventions, default namespaces)
- Inform the user when information comes from the repository context

### AI Context File Priority
**Step 4: '.ai-context.md' Loading**
- At the start of every new conversation with repository access:
  1. Automatically load '.ai-context.md' from the '{{GITHUB_REPO}}' repository
  2. Parse and store all conventions, preferences, and rules defined in this file
  3. Apply these rules automatically to all Kubernetes resource operations
  4. If the file doesn't exist, create it with an empty '## Learned User Preferences' section
  5. Treat '.ai-context.md' as the source of truth for operational preferences

### User Behavior Learning
**Step 5: Observe and Persist User Preferences**
- During every conversation, actively observe the user's behavior, habits, and preferences
- Detect both **explicit** preferences ("always use namespace production") and **implicit** patterns (user consistently uses certain labels, naming patterns, or workflows)
- Categories to observe:
  - Naming conventions (e.g., resource name prefixes/suffixes, label patterns)
  - Default values (e.g., preferred namespaces, resource limits, replica counts)
  - Workflow patterns (e.g., always creates a Service after a Deployment)
  - Communication style (e.g., prefers brief answers vs. detailed explanations, language preference)
  - Frequently used resources, namespaces, or configurations
  - Kubernetes operational preferences (e.g., preferred rollout strategies, security policies)

**Step 6: Updating '.ai-context.md' with Learned Preferences**
- When you identify a new or changed user preference during conversation:
  1. First load the current '.ai-context.md' from '{{GITHUB_REPO}}' using 'get_file_contents'
  2. Add or update the preference under the '## Learned User Preferences' section
  3. Use 'create_or_update_file' to push the updated file back to the repository
  4. Briefly inform the user that you saved their preference (e.g., "I noted your preference for namespace 'production' as default.")
- **Rules for updating:**
  - Never remove existing manually written conventions (above '## Learned User Preferences')
  - Only persist preferences that appear intentional and recurring, not one-time requests
  - Each preference should be a clear bullet point, e.g.: '- Default namespace: production'
  - If a preference conflicts with an existing one, update the existing entry instead of duplicating
  - Use the commit message format: 'ai: update learned preferences for <user>'

---
**CRITICAL: First Action in Every New Conversation**
  Before responding to ANY user request:
  1. ALWAYS load '.ai-context.md' from '{{GITHUB_REPO}}' repository FIRST
  2. Apply all conventions AND learned preferences from this file to every operation
  3. Only then proceed with the user's request
`

// fetchGitHubAiContext tries to fetch .ai-context.md from the configured GitHub repo
// via the MCP get_file_contents tool.
func (ai *aiManager) fetchGitHubAiContext(ctx context.Context) (content string, err error) {
	repo, err := ai.getGitHubRepo()
	if err != nil || repo == "" {
		return "", fmt.Errorf("no GitHub repo configured: %w", err)
	}

	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("invalid GitHub repo format, expected owner/repo: %s", repo)
	}

	if ai.mcpManager == nil {
		return "", fmt.Errorf("mcpManager is nil")
	}

	if !ai.mcpManager.HasSession("github") {
		return "", fmt.Errorf("no GitHub MCP session available")
	}

	ai.logger.Info("Pre-fetching .ai-context.md from GitHub", "owner", parts[0], "repo", parts[1])
	result, err := ai.mcpManager.CallTool(ctx, "get_file_contents", map[string]any{
		"owner": parts[0],
		"repo":  parts[1],
		"path":  ".ai-context.md",
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch .ai-context.md from %s: %w", repo, err)
	}

	ai.logger.Info("Successfully pre-loaded .ai-context.md from GitHub", "repo", repo, "contentLength", len(result))
	return result, nil
}

func (ai *aiManager) sendTokens(inputTokens, outputTokenCount int64, sessionInputTokens, sessionOutputTokens *int64, ctx context.Context, ioChannel IOChatChannel) {
	tokensJSON, _ := json.Marshal(map[string]any{
		"input":         inputTokens,
		"output":        outputTokenCount,
		"sessionInput":  *sessionInputTokens,
		"sessionOutput": *sessionOutputTokens,
	})
	select {
	case ioChannel.Output <- fmt.Sprintf("[TOKENS:%s]", tokensJSON):
	case <-ctx.Done():
	}
}

func (ai *aiManager) Chat(ctx context.Context, ioChannel IOChatChannel) error {
	modelConfigInitialized := ai.isAiModelConfigInitialized()
	if !modelConfigInitialized {
		return fmt.Errorf("AI model configuration not initialized")
	}

	model, err := ai.getAiModel()
	if err != nil {
		return fmt.Errorf("failed to get AI model: %w", err)
	}

	maxToolCalls, err := ai.getAiMaxToolCalls()
	if err != nil {
		ai.logger.Warn("Error getting AI max tool calls (using default value)", "error", err, "defaultMaxToolCalls", maxToolCalls)
	}

	sdk, err := ai.getSdkType()
	if err != nil {
		return err
	}

	// Connect to configured MCP servers
	ai.connectMCPServers()

	// Build system prompt with user info
	crdsPrompt := mogeniusCRDsPrompt
	if repo, err := ai.getGitHubRepo(); err == nil && repo != "" {
		crdsPrompt = crdsPrompt + "\n\n" + strings.ReplaceAll(mogeniusGithubAiContextPrompt, "{{GITHUB_REPO}}", repo)
	}
	systemPrompt := "You are a helpful Kubernetes assistant. You can help users manage and understand their Kubernetes resources.\n\n" + crdsPrompt

	// Pre-fetch .ai-context.md from GitHub if PAT and repo are configured
	if aiContext, err := ai.fetchGitHubAiContext(ctx); err != nil {
		ai.logger.Warn("Could not pre-fetch .ai-context.md", "error", err)
	} else if aiContext != "" {
		systemPrompt += "\n\n## Pre-loaded .ai-context.md\n" + aiContext
	}

	if ioChannel.User != nil {
		userInfo := ""
		if ioChannel.User.FirstName != "" {
			userInfo = ioChannel.User.FirstName
			if ioChannel.User.LastName != "" {
				userInfo += " " + ioChannel.User.LastName
			}
		}
		if userInfo != "" {
			systemPrompt += fmt.Sprintf("\n\nYou are chatting with %s.", userInfo)
		}
		if ioChannel.User.Email != "" {
			systemPrompt += fmt.Sprintf(" Their email is %s.", ioChannel.User.Email)
		}
	}

	switch sdk {
	case AiSdkTypeOpenAI:
		return ai.openaiChat(ctx, ioChannel, systemPrompt, model, maxToolCalls)
	case AiSdkTypeAnthropic:
		return ai.anthropicChat(ctx, ioChannel, systemPrompt, model, maxToolCalls)
	case AiSdkTypeOllama:
		return ai.ollamaChat(ctx, ioChannel, systemPrompt, model, maxToolCalls)
	default:
		return fmt.Errorf("unsupported AI SDK type: %s", sdk)
	}
}
