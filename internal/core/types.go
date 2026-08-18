// Package core contains the neutral types and the agent loop that Kite is
// built on. The root package kite re-exports these types behind a small
// façade, so consumers depend on the root package while the implementation
// stays here.
package core

import (
	"context"
	"io"
	"time"
)

// ContractVersion identifies the versioned wire contracts Kite speaks.
type ContractVersion string

const (
	// ContractEvent is the version of the event JSON contract.
	ContractEvent ContractVersion = "kite.event/v1"
	// ContractResult is the version of the result JSON contract.
	ContractResult ContractVersion = "kite.result/v1"
	// ContractRPCRequest is the version of the RPC request JSON contract.
	ContractRPCRequest ContractVersion = "kite.rpc.request/v1"
	// ContractRPCResponse is the version of the RPC response JSON contract.
	ContractRPCResponse ContractVersion = "kite.rpc.response/v1"
)

// Role identifies who produced a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
	RoleSystem    Role = "system"
)

// Message is a single turn in a conversation.
type Message struct {
	Role    Role
	Content string
	// ToolCallID links a tool result message back to the call that produced it.
	ToolCallID string
	// ToolCalls lists the tool calls an assistant message made. Tool result
	// messages that follow must reference one of these by ToolCallID.
	ToolCalls []ToolCall
}

// ToolCall is a request from the model to run a tool.
type ToolCall struct {
	ID    string
	Name  string
	Input string
}

// Tool is a named function the model can invoke. Input is a raw JSON
// string; each tool parses it itself. Schema returns the JSON schema for
// the tool's arguments object, which providers advertise to the model.
type Tool interface {
	Name() string
	Description() string
	Schema() any
	Run(ctx context.Context, input string) (string, error)
}

// Provider talks to a model. The provider-specific wire format stays inside
// the provider package; this interface is the small seam the agent loop
// depends on.
type Provider interface {
	// Complete sends the session messages and advertises the available
	// tools, then streams the model's reply back as events. The reply may
	// contain zero or more tool calls, and at most one text part.
	Complete(ctx context.Context, session *Session, tools []Tool, onEvent func(ProviderEvent)) error
}

// NoopProvider is a Provider that never produces output. It is useful for
// building context or inspecting a session without driving a model.
type NoopProvider struct{}

// Complete returns immediately with no events.
func (NoopProvider) Complete(ctx context.Context, session *Session, tools []Tool, onEvent func(ProviderEvent)) error {
	return nil
}

// ProviderEvent is a single streamed unit from a provider. Providers emit
// text deltas, completed tool calls, usage, and errors as they arrive.
type ProviderEvent struct {
	// Text is a delta of assistant text, if any.
	Text string
	// ToolCall is a completed tool call, if any. Fragmented tool-call
	// assembly happens inside the provider.
	ToolCall *ToolCall
	// Usage is the final token usage, if reported by the endpoint.
	Usage *Usage
	// Err is a structured provider error, if the stream failed.
	Err *Error
	// Done marks the end of the stream.
	Done bool
}

// Usage tracks token consumption for a single model turn.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Error is a structured, sanitised error that crosses the public boundary.
// It never carries secrets, raw credentials, or full request bodies.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	// Retryable hints whether the caller may retry the operation.
	Retryable bool `json:"retryable"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Result is the structured outcome of a completed prompt.
type Result struct {
	// Status is "completed" or "failed".
	Status string `json:"status"`
	// Text is the final assistant text.
	Text string `json:"text,omitempty"`
	// ChangedFiles lists files modified during the current prompt.
	ChangedFiles []string `json:"changed_files,omitempty"`
	// ChangedFilesComplete is false outside a Git repository, where the
	// changed-files list may be incomplete.
	ChangedFilesComplete bool `json:"changed_files_complete"`
	// Verification describes the last verification run, if any.
	Verification *Verification `json:"verification,omitempty"`
	// Usage aggregates token usage across all model turns.
	Usage Usage `json:"usage"`
}

// Verification describes a bash verification run.
type Verification struct {
	Command  string `json:"command"`
	Status   string `json:"status"` // "passed" or "failed"
	ExitCode int    `json:"exit_code"`
	// Artifacts holds artifact IDs produced by the verification command.
	Artifacts []string `json:"artifacts,omitempty"`
	// Stale is true when the worktree changed after this verification ran.
	Stale bool `json:"stale"`
}

// Event is a durable, sequence-numbered unit emitted by the agent loop.
type Event struct {
	// ID is a globally unique prefixed identifier.
	ID string `json:"id"`
	// Seq is the durable sequence number within the session.
	Seq int `json:"seq"`
	// SessionID is the session that produced the event.
	SessionID string `json:"session_id"`
	// Type is the event type, for example "session.started".
	Type string `json:"type"`
	// Time is when the event was emitted.
	Time time.Time `json:"time"`
	// Payload carries the type-specific fields.
	Payload any `json:"payload,omitempty"`
}

// Event types.
const (
	EventSessionStarted   = "session.started"
	EventSessionCompleted = "session.completed"
	EventSessionFailed    = "session.failed"
	EventModelStarted     = "model.started"
	EventModelCompleted   = "model.completed"
	EventTextDelta        = "text.delta"
	EventToolStarted      = "tool.started"
	EventToolFinished     = "tool.finished"
	EventArtifactCreated  = "artifact.created"
	EventUserMessage      = "user-message"
	EventUsage            = "usage"
	EventResume           = "resume"
	EventVerification     = "verification"
	EventInterruptedTool  = "interrupted-tool"
)

// EventPayloads are the typed payloads carried by events.

// SessionStartedPayload is emitted when a session begins a prompt.
type SessionStartedPayload struct {
	Prompt string `json:"prompt"`
}

// SessionCompletedPayload is emitted when a prompt completes.
type SessionCompletedPayload struct {
	Result *Result `json:"result"`
}

// SessionFailedPayload is emitted when a prompt fails at runtime.
type SessionFailedPayload struct {
	Error *Error `json:"error"`
}

// ModelStartedPayload is emitted when a model turn begins.
type ModelStartedPayload struct {
	Turn int `json:"turn"`
}

// ModelCompletedPayload is emitted when a model turn ends.
type ModelCompletedPayload struct {
	Turn  int    `json:"turn"`
	Usage *Usage `json:"usage,omitempty"`
}

// TextDeltaPayload is emitted for each fragment of assistant text.
type TextDeltaPayload struct {
	Text string `json:"text"`
}

// ToolStartedPayload is emitted when a tool begins running.
type ToolStartedPayload struct {
	CallID string `json:"call_id"`
	Name   string `json:"name"`
	Input  string `json:"input"`
}

// ToolFinishedPayload is emitted when a tool finishes.
type ToolFinishedPayload struct {
	CallID string `json:"call_id"`
	Name   string `json:"name"`
	Output string `json:"output"`
	// Error is set when the tool failed.
	Error *Error `json:"error,omitempty"`
}

// ArtifactCreatedPayload is emitted when a large output is stored.
type ArtifactCreatedPayload struct {
	Artifact *Artifact `json:"artifact"`
}

// UserMessagePayload is emitted when a user message is added to a session.
type UserMessagePayload struct {
	Text string `json:"text"`
}

// UsagePayload is emitted when usage is reported.
type UsagePayload struct {
	Usage Usage `json:"usage"`
}

// ResumePayload is the payload shape reserved for resume events.
type ResumePayload struct {
	Prompt string `json:"prompt"`
}

// VerificationPayload is emitted when a verification runs.
type VerificationPayload struct {
	Verification *Verification `json:"verification"`
}

// InterruptedToolPayload is emitted when a tool call is interrupted.
type InterruptedToolPayload struct {
	Call *ToolCall `json:"call"`
}

// Artifact is a stored large output.
type Artifact struct {
	// ID is a globally unique prefixed identifier.
	ID string `json:"id"`
	// SessionID is the session that created the artifact.
	SessionID string `json:"session_id"`
	// Size is the full size in bytes.
	Size int64 `json:"size"`
	// MediaType is the media type of the stored content.
	MediaType string `json:"media_type"`
	// Truncated is true when the stored content was truncated.
	Truncated bool `json:"truncated"`
	// Preview is a head/tail preview, up to 8 KiB.
	Preview string `json:"preview"`
}

// Config configures a new session.
type Config struct {
	// Provider is the model provider. Required.
	Provider Provider
	// Model is the model identifier. Required.
	Model string
	// WorkingDir is the repository working directory.
	WorkingDir string
	// DataDir is where sessions and artifacts are persisted. Empty means
	// the platform default.
	DataDir string
	// Tools are the tools available to the model. Nil installs the
	// built-ins.
	Tools []Tool
	// MaxInline is the maximum bytes inlined in a tool result before the
	// output is stored as an artifact. Zero means 16 KiB.
	MaxInline int
	// MaxPreview is the maximum bytes of an artifact preview. Zero means
	// 8 KiB.
	MaxPreview int
	// Stdout is reserved for human-readable progress output. Session output is
	// currently delivered through events instead.
	Stdout io.Writer
	// Print is reserved for requesting progress mirroring by frontends. The
	// core runtime currently leaves presentation to event consumers.
	Print bool
	// MaxTurns caps the number of agent turns. Zero means no limit.
	MaxTurns int
}

// SessionStore persists sessions and artifacts.
type SessionStore interface {
	// AppendEvent durably appends an event to the session log.
	AppendEvent(sessionID string, ev *Event) error
	// LoadEvents returns all durable events for a session in order.
	LoadEvents(sessionID string) ([]*Event, error)
	// StoreArtifact writes an artifact's content.
	StoreArtifact(sessionID, artifactID string, content []byte) error
	// LoadArtifact reads up to limit bytes of an artifact starting at
	// offset.
	LoadArtifact(sessionID, artifactID string, offset, limit int64) ([]byte, error)
	// ArtifactSize returns the stored size of an artifact.
	ArtifactSize(sessionID, artifactID string) (int64, error)
	// ListSessions returns the session IDs known to the store.
	ListSessions() ([]string, error)
}
