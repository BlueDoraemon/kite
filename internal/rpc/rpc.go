// Package rpc implements the NDJSON RPC protocol for Kite. Requests and
// responses are newline-delimited JSON with a 1 MiB line limit.
package rpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/BlueDoraemon/kite-core/internal/core"
)

// Request is a kite.rpc.request/v1 request.
type Request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Response is a kite.rpc.response/v1 response.
type Response struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
}

// Error is a structured RPC error.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// maxLineBytes is the 1 MiB line limit for the NDJSON protocol.
const maxLineBytes = 1 << 20

// Handler processes a single RPC request and returns its response.
type Handler interface {
	Handle(req *Request) (*Response, error)
}

// Server processes requests sequentially from a reader and writes responses
// to a writer.
type Server struct {
	handler Handler
}

// NewServer returns a Server backed by handler.
func NewServer(h Handler) *Server {
	return &Server{handler: h}
}

// Serve reads requests from r and writes responses to w until EOF.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	enc := json.NewEncoder(w)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			resp := &Response{ID: "", Method: "", OK: false, Error: &Error{Code: "bad_request", Message: "invalid JSON"}}
			if err := enc.Encode(resp); err != nil {
				return err
			}
			continue
		}
		resp, err := s.handler.Handle(&req)
		if err != nil {
			resp = &Response{ID: req.ID, Method: req.Method, OK: false, Error: &Error{Code: "internal", Message: err.Error()}}
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// Methods.
const (
	MethodPrompt   = "prompt"
	MethodResume   = "resume"
	MethodStatus   = "status"
	MethodInspect  = "inspect"
	MethodArtifact = "artifact"
	MethodContext  = "context"
)

// PromptParams is the params for the prompt method.
type PromptParams struct {
	Text string `json:"text"`
}

// ResumeParams is the params for the resume method.
type ResumeParams struct {
	SessionID string `json:"session_id"`
	Prompt    string `json:"prompt,omitempty"`
}

// StatusParams is the params for the status method.
type StatusParams struct {
	SessionID string `json:"session_id,omitempty"`
}

// InspectParams is the params for the inspect method.
type InspectParams struct {
	ToolID string `json:"tool_id"`
}

// ArtifactParams is the params for the artifact method.
type ArtifactParams struct {
	ArtifactID string `json:"artifact_id"`
	Offset     int    `json:"offset,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

// ContextParams is the params for the context method.
type ContextParams struct {
	SessionID string `json:"session_id,omitempty"`
	Full      bool   `json:"full,omitempty"`
}

// StatusResult is the result of the status method.
type StatusResult struct {
	SessionID   string          `json:"session_id"`
	Model       string          `json:"model"`
	Turn        int             `json:"turn"`
	Messages    int             `json:"messages"`
	Interrupted []core.ToolCall `json:"interrupted,omitempty"`
}

// ContextResult is the result of the context method.
type ContextResult struct {
	Messages []core.Message `json:"messages"`
}

// InspectResult is the result of the inspect method.
type InspectResult struct {
	ToolID      string `json:"tool_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Schema      any    `json:"schema"`
}

// ArtifactResult is the result of the artifact method.
type ArtifactResult struct {
	ArtifactID string `json:"artifact_id"`
	Offset     int    `json:"offset"`
	Limit      int    `json:"limit"`
	Content    string `json:"content"`
}

// ParseRequest decodes a request line.
func ParseRequest(line string) (*Request, error) {
	var req Request
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	return &req, nil
}
