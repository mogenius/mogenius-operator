package ai

import (
	"context"
	"crypto/tls"
	"log/slog"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type ChatRequest struct {
	ChannelId       string `json:"channelId" validate:"required"`
	WebsocketScheme string `json:"websocketScheme" validate:"required"`
	WebsocketHost   string `json:"websocketHost" validate:"required"`
	IsAdmin         bool   `json:"isAdmin" validate:"boolean"`
}

type AiWebsocketConnection interface {
	LiveStreamAiManagerChatRequest(request ChatRequest, datagram structs.Datagram)
}

type aiWebsocketConnection struct {
	logger    *slog.Logger
	aiManager AiManager
}

func NewAiWebsocketConnection(logger *slog.Logger, aiManager AiManager) AiWebsocketConnection {
	self := &aiWebsocketConnection{}
	self.logger = logger
	self.aiManager = aiManager

	return self
}

// ToolUseRecord captures a single tool invocation for audit logging.
type ToolUseRecord struct {
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args,omitempty"`
	Result string         `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
}

// ChatTurnStats collects metadata about a single AI chat turn for audit logging.
type ChatTurnStats struct {
	ToolRecords  []ToolUseRecord `json:"tools,omitempty"`
	Model        string          `json:"model"`
	InputTokens  int64           `json:"inputTokens"`
	OutputTokens int64           `json:"outputTokens"`
	DurationMs   int             `json:"durationMs"`
}

// AuditEvent represents an auditable event from the AI chat session
type AuditEvent struct {
	Pattern string // e.g. "ai/chat"
	Payload any    // user message or tool call info
	Result  any    // AI response or tool result
	Error   string // error message if any
}

// AuditCallback is called by chat implementations to record audit log entries
type AuditCallback func(event AuditEvent)

// IOChatChannel represents a bidirectional channel for AI chat communication
type IOChatChannel struct {
	Input          <-chan string           // Incoming messages (user questions)
	Output         chan<- string           // Outgoing messages (AI responses)
	User           *structs.User           // Optional user information
	IsAdmin        bool                    // Indicates if the user has admin privileges
	WorkspaceSpec  *v1alpha1.WorkspaceSpec // Optional workspace information
	WorkspaceGrant *v1alpha1.GrantSpec     // Optional workspace grant information
	OnAudit        AuditCallback           // Optional callback for audit logging
}

// emitAuditEvent calls the OnAudit callback if configured on the channel.
func emitAuditEvent(ch IOChatChannel, pattern string, payload any, result any, errStr string) {
	if ch.OnAudit != nil {
		ch.OnAudit(AuditEvent{
			Pattern: pattern,
			Payload: payload,
			Result:  result,
			Error:   errStr,
		})
	}
}

// truncateToolResult limits tool result strings to a reasonable size for audit logging.
func truncateToolResult(s string) string {
	const maxLen = 2000
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}

func (self *aiWebsocketConnection) LiveStreamAiManagerChatRequest(request ChatRequest, datagram structs.Datagram) {
	logger := self.logger.With("scope", "LiveStreamAiManagerChatRequest")

	if request.WebsocketScheme == "" {
		logger.Error("WebsocketScheme is empty")
		return
	}

	if request.WebsocketHost == "" {
		logger.Error("WebsocketHost is empty")
		return
	}

	websocketUrl := url.URL{Scheme: request.WebsocketScheme, Host: request.WebsocketHost, Path: "/"}
	// context
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3600)
	defer cancel()
	// websocket connection
	conn, connWriteLock, connReadLock, err := self.GenerateWsConnection(websocketUrl, request.ChannelId)
	if err != nil {
		logger.Error("Unable to connect to websocket", "error", err)
		return
	}
	logger.Info("Successfully connected to WebSocket for AI Manager Chat")

	defer func() {
		cancel()
		if conn != nil {
			conn.Close()
		}
	}()

	// Create IO channels for AI chat
	inputChan := make(chan string)
	outputChan := make(chan string)

	chatChannel := IOChatChannel{
		Input:   inputChan,
		Output:  outputChan,
		User:    &datagram.User,
		IsAdmin: request.IsAdmin,
		OnAudit: func(event AuditEvent) {
			store.AddAiChatAuditLog(logger, event.Pattern, event.Payload, event.Result, event.Error, datagram.User, datagram.Workspace)
		},
	}

	// Resolve workspace and grant context if user and workspace are provided
	if datagram.User.Email != "" && datagram.Workspace != "" {
		workspaceSpec, grantSpec := self.aiManager.ResolveWorkspaceContext(datagram.User.Email, datagram.Workspace)
		chatChannel.WorkspaceSpec = workspaceSpec
		chatChannel.WorkspaceGrant = grantSpec
	}

	defer func() {
		logger.Info("AI Chat WebSocket connection closed")
		cancel() // cancel context first so Chat goroutine exits via ctx.Done() before channels close
		close(inputChan)
		close(outputChan)
	}()

	// Read from output channel and write to WebSocket
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case output, ok := <-outputChan:
				if !ok {
					return
				}
				connWriteLock.Lock()
				err := conn.WriteMessage(websocket.TextMessage, []byte(output))
				connWriteLock.Unlock()
				if err != nil {
					logger.Error("Failed to write to WebSocket", "error", err)
					return
				}
			}
		}
	}()

	// Run Chat with the channels
	go func() {
		//defer close(outputChan)
		err := self.aiManager.Chat(ctx, chatChannel)
		if err != nil {
			logger.Error("ChatTest error", "error", err)
		}
	}()

	// Main loop: Read from WebSocket and write to input channel
	for {
		select {
		case <-ctx.Done():
			return
		default:
			connReadLock.Lock()
			messageType, p, err := conn.ReadMessage()
			connReadLock.Unlock()

			if err != nil {
				if closeErr, ok := err.(*websocket.CloseError); ok {
					logger.Warn("WebSocket closed", "statusCode", closeErr.Code, "closeErr", closeErr.Text)
				} else {
					logger.Warn("Failed to read message. Connection closed.", "error", err)
				}
				return
			}

			// Only process text messages
			if messageType == websocket.TextMessage {
				userInput := string(p)

				// Send to input channel (non-blocking with timeout)
				select {
				case inputChan <- userInput:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func (self *aiWebsocketConnection) GenerateWsConnection(
	u url.URL,
	channelId string,
) (conn *websocket.Conn, connWriteLock *sync.Mutex, connReadLock *sync.Mutex, err error) {
	logger := self.logger.With("scope", "GenerateWsConnection")

	maxRetries := 6
	currentRetries := 0

	for {
		// add header
		headers := utils.HttpHeader("")
		headers.Add("x-channel-id", channelId)
		headers.Add("x-type", "k8s")

		dialer := &websocket.Dialer{}
		if strings.ToLower(os.Getenv("MO_SKIP_TLS_VERIFICATION")) == "true" {
			dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		}
		conn, _, err := dialer.Dial(u.String(), headers)
		connWriteLock := &sync.Mutex{}
		connReadLock := &sync.Mutex{}
		if err != nil {
			logger.Error("failed to connect, retrying in 5 seconds", "error", err.Error())
			if currentRetries >= maxRetries {
				logger.Error("Max retries reached, exiting.")
				return nil, nil, nil, err
			}
			time.Sleep(5 * time.Second)
			currentRetries++
			continue
		}

		// API send ack when it is ready to receive messages.
		err = conn.SetReadDeadline(time.Now().Add(30 * time.Minute))
		if err != nil {
			logger.Error("failed to set read deadline", "error", err)
		}
		connReadLock.Lock()
		_, _, err = conn.ReadMessage()
		connReadLock.Unlock()
		if err != nil {
			logger.Error("failed to receive ack-ready, retrying in 5 seconds", "error", err)
			time.Sleep(5 * time.Second)
			if currentRetries >= maxRetries {
				logger.Error("Max retries reached, exiting.")
				return conn, connWriteLock, connReadLock, err
			}
			currentRetries++
			continue
		}

		return conn, connWriteLock, connReadLock, nil
	}
}
