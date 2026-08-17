// Package kite contains the core types and the agent loop for Kite,
// a minimal command-line agent that can explain and modify a repository.
package kite

import (
	"context"
	"io"
)

// Role identifies who produced a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
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

// Session carries the conversation and the model used to drive it.
type Session struct {
	Model    string
	Messages []Message
}

// ToolCall is a request from the model to run a tool.
type ToolCall struct {
	ID    string
	Name  string
	Input string
}

// ToolResult is the outcome of running a tool.
type ToolResult struct {
	CallID string
	Output string
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
// depends on on.
type Provider interface {
	// Complete sends the session messages and advertises the available
	// tools, then returns the model's reply. The reply may contain zero
	// or more tool calls, and at most one text part. When tool calls are
	// present, Text may be empty.
	Complete(ctx context.Context, session *Session, tools []Tool) (Reply, error)
}

// Reply is one model turn.
type Reply struct {
	Text       string
	ToolCalls  []ToolCall
	Finish     bool
	FinishText string
}

// Event is emitted by the agent loop as it runs, so a caller can observe
// progress without reaching into the loop's internals.
type Event struct {
	Type    string // "message", "tool_call", "tool_result", "finish"
	Message *Message
	Call    *ToolCall
	Result  *ToolResult
	Text    string
}

// EventFunc receives events from a run. Returning an error aborts the run.
type EventFunc func(Event) error

// RunOptions configures a single agent run.
type RunOptions struct {
	Session *Session
	Tools   []Tool
	// OnEvent is called for each event as the run progresses.
	OnEvent EventFunc
	// Stdout receives the model's text output and tool results when
	// Print is true.
	Stdout io.Writer
	// Print mirrors the model's text and tool output to Stdout as it
	// happens. When false, output is only available through OnEvent.
	Print bool
	// MaxTurns caps the number of agent turns to guard against runaway
	// loops. Zero means no limit.
	MaxTurns int
}
