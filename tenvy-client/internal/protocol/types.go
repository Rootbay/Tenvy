package protocol

import (
	"encoding/json"
	"errors"
)

var ErrUnauthorized = errors.New("unauthorized")

type AgentConfig struct {
	PollIntervalMs int     `json:"pollIntervalMs"`
	MaxBackoffMs   int     `json:"maxBackoffMs"`
	JitterRatio    float64 `json:"jitterRatio"`
}

type AgentMetrics struct {
	MemoryBytes   uint64 `json:"memoryBytes,omitempty"`
	Goroutines    int    `json:"goroutines,omitempty"`
	UptimeSeconds uint64 `json:"uptimeSeconds,omitempty"`
}

type Command struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt string          `json:"createdAt"`
}

type CommandResult struct {
	CommandID   string `json:"commandId"`
	Success     bool   `json:"success"`
	Output      string `json:"output,omitempty"`
	Error       string `json:"error,omitempty"`
	CompletedAt string `json:"completedAt"`
}

type AgentMetadata struct {
	Hostname        string   `json:"hostname"`
	Username        string   `json:"username"`
	OS              string   `json:"os"`
	Architecture    string   `json:"architecture"`
	IPAddress       string   `json:"ipAddress,omitempty"`
	PublicIPAddress string   `json:"publicIpAddress,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	Version         string   `json:"version,omitempty"`
}

type AgentRegistrationRequest struct {
	Token    string        `json:"token,omitempty"`
	Metadata AgentMetadata `json:"metadata"`
}

type AgentRegistrationResponse struct {
	AgentID    string      `json:"agentId"`
	AgentKey   string      `json:"agentKey"`
	Config     AgentConfig `json:"config"`
	Commands   []Command   `json:"commands"`
	ServerTime string      `json:"serverTime"`
}

type AgentSyncRequest struct {
	Status    string          `json:"status"`
	Timestamp string          `json:"timestamp"`
	Metrics   *AgentMetrics   `json:"metrics,omitempty"`
	Results   []CommandResult `json:"results,omitempty"`
}

type AgentSyncResponse struct {
	AgentID    string      `json:"agentId"`
	Commands   []Command   `json:"commands"`
	Config     AgentConfig `json:"config"`
	ServerTime string      `json:"serverTime"`
}

type PingCommandPayload struct {
	Message string `json:"message,omitempty"`
}

type ShellCommandPayload struct {
	Command          string            `json:"command"`
	TimeoutSeconds   int               `json:"timeoutSeconds,omitempty"`
	WorkingDirectory string            `json:"workingDirectory,omitempty"`
	Elevated         bool              `json:"elevated,omitempty"`
	Environment      map[string]string `json:"environment,omitempty"`
}

type OpenURLCommandPayload struct {
	URL  string `json:"url"`
	Note string `json:"note,omitempty"`
}

type AgentControlCommandPayload struct {
	Action string `json:"action"`
	Reason string `json:"reason,omitempty"`
}

type ToolActivationCommandPayload struct {
	ToolID      string         `json:"toolId"`
	Action      string         `json:"action"`
	InitiatedBy string         `json:"initiatedBy,omitempty"`
	Timestamp   string         `json:"timestamp,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type RecoveryTargetSelection struct {
	Type      string   `json:"type"`
	Label     string   `json:"label,omitempty"`
	Path      string   `json:"path,omitempty"`
	Paths     []string `json:"paths,omitempty"`
	Recursive bool     `json:"recursive,omitempty"`
}

type RecoveryCommandPayload struct {
	RequestID   string                    `json:"requestId"`
	Selections  []RecoveryTargetSelection `json:"selections"`
	ArchiveName string                    `json:"archiveName,omitempty"`
	Notes       string                    `json:"notes,omitempty"`
}

type RecoveryManifestEntry struct {
	Path            string `json:"path"`
	Size            int64  `json:"size"`
	ModifiedAt      string `json:"modifiedAt"`
	Mode            string `json:"mode"`
	Type            string `json:"type"`
	Target          string `json:"target"`
	SourcePath      string `json:"sourcePath,omitempty"`
	Preview         string `json:"preview,omitempty"`
	PreviewEncoding string `json:"previewEncoding,omitempty"`
	Truncated       bool   `json:"truncated,omitempty"`
}

type ClientChatAliasConfiguration struct {
	Operator string `json:"operator,omitempty"`
	Client   string `json:"client,omitempty"`
}

type ClientChatFeatureFlags struct {
	Unstoppable        *bool `json:"unstoppable,omitempty"`
	AllowNotifications *bool `json:"allowNotifications,omitempty"`
	AllowFileTransfers *bool `json:"allowFileTransfers,omitempty"`
}

type ClientChatCommandMessage struct {
	ID        string `json:"id,omitempty"`
	Body      string `json:"body"`
	Timestamp string `json:"timestamp,omitempty"`
	Alias     string `json:"alias,omitempty"`
}

type ClientChatCommandPayload struct {
	Action    string                        `json:"action"`
	SessionID string                        `json:"sessionId,omitempty"`
	Message   *ClientChatCommandMessage     `json:"message,omitempty"`
	Aliases   *ClientChatAliasConfiguration `json:"aliases,omitempty"`
	Features  *ClientChatFeatureFlags       `json:"features,omitempty"`
}

type ClientChatMessage struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	Timestamp string `json:"timestamp"`
	Alias     string `json:"alias,omitempty"`
}

type ClientChatMessageEnvelope struct {
	SessionID string            `json:"sessionId"`
	Message   ClientChatMessage `json:"message"`
}
