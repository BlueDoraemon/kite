// Package kite is the public façade for the Kite agent runtime. It re-exports
// the neutral types and the session API backed by internal/core, so
// consumers depend on this root package.
//
// Public contracts:
//   - NewSession(Config) (*Session, error)
//   - LoadSession(Config, id string) (*Session, error)
//   - (*Session).Prompt(ctx, text) (<-chan Event, error)
//   - (*Session).BuildContext() []Message
//   - Provider, Tool, Message, Event, Artifact, Usage, Result, and Error types.
package kite

import (
	"context"

	"github.com/BlueDoraemon/kite-core/internal/core"
	_ "github.com/BlueDoraemon/kite-core/internal/tools"
)

// Role identifies who produced a message.
type Role = core.Role

// Message is a single turn in a conversation.
type Message = core.Message

// ToolCall is a request from the model to run a tool.
type ToolCall = core.ToolCall

// Tool is a named function the model can invoke.
type Tool = core.Tool

// Provider talks to a model.
type Provider = core.Provider

// NoopProvider is a Provider that never produces output.
type NoopProvider = core.NoopProvider

// ProviderEvent is a single streamed unit from a provider.
type ProviderEvent = core.ProviderEvent

// Usage tracks token consumption.
type Usage = core.Usage

// Error is a structured, sanitised error.
type Error = core.Error

// Session is the public handle to a Kite agent session.
type Session = core.Session

// Result is the structured outcome of a completed prompt.
type Result = core.Result

// Verification describes a bash verification run.
type Verification = core.Verification

// Event is a durable, sequence-numbered unit emitted by the agent loop.
type Event = core.Event

// Artifact is a stored large output.
type Artifact = core.Artifact

// Event payload types.
type (
	SessionStartedPayload   = core.SessionStartedPayload
	SessionCompletedPayload = core.SessionCompletedPayload
	SessionFailedPayload    = core.SessionFailedPayload
	ModelStartedPayload     = core.ModelStartedPayload
	ModelCompletedPayload   = core.ModelCompletedPayload
	TextDeltaPayload        = core.TextDeltaPayload
	ToolStartedPayload      = core.ToolStartedPayload
	ToolFinishedPayload     = core.ToolFinishedPayload
	ArtifactCreatedPayload  = core.ArtifactCreatedPayload
	UserMessagePayload      = core.UserMessagePayload
	UsagePayload            = core.UsagePayload
	ResumePayload           = core.ResumePayload
	VerificationPayload     = core.VerificationPayload
	InterruptedToolPayload  = core.InterruptedToolPayload
)

// Config configures a new session.
type Config = core.Config

// SessionStore persists sessions and artifacts.
type SessionStore = core.SessionStore

// Contract versions.
const (
	ContractEvent       = core.ContractEvent
	ContractResult      = core.ContractResult
	ContractRPCRequest  = core.ContractRPCRequest
	ContractRPCResponse = core.ContractRPCResponse
)

// Roles.
const (
	RoleUser      = core.RoleUser
	RoleAssistant = core.RoleAssistant
	RoleTool      = core.RoleTool
	RoleSystem    = core.RoleSystem
)

// Event types.
const (
	EventSessionStarted   = core.EventSessionStarted
	EventSessionCompleted = core.EventSessionCompleted
	EventSessionFailed    = core.EventSessionFailed
	EventModelStarted     = core.EventModelStarted
	EventModelCompleted   = core.EventModelCompleted
	EventTextDelta        = core.EventTextDelta
	EventToolStarted      = core.EventToolStarted
	EventToolFinished     = core.EventToolFinished
	EventArtifactCreated  = core.EventArtifactCreated
	EventUserMessage      = core.EventUserMessage
	EventUsage            = core.EventUsage
	EventResume           = core.EventResume
	EventVerification     = core.EventVerification
	EventInterruptedTool  = core.EventInterruptedTool
)

// NewSession creates a new session from cfg.
func NewSession(cfg Config) (*Session, error) {
	return core.NewSession(cfg)
}

// LoadSession loads a persisted session by id.
func LoadSession(cfg Config, id string) (*Session, error) {
	return core.LoadSession(cfg, id)
}

// SystemInstructions is the fixed system prompt Kite sends to the model.
const SystemInstructions = core.SystemInstructions

// LoadInstructions loads the nearest AGENTS.md between the working directory
// and the repository root.
func LoadInstructions(dir string) (core.Instructions, error) {
	return core.LoadInstructions(dir)
}

// Context carries a context value.
type Context = context.Context
