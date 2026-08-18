// Package schemagen generates the versioned JSON schemas for Kite's machine
// contracts. Run it with:
//
//	go run ./cmd/schemagen
//
// The generated schemas are committed under docs/schemas/v1. A generation
// check fails when the committed schemas differ from the runtime types.
package schemagen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Schema is a JSON Schema document.
type Schema struct {
	Schema     string         `json:"$schema"`
	Title      string         `json:"title"`
	Version    string         `json:"version"`
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Required   []string       `json:"required,omitempty"`
}

// Generate writes all schemas into dir and returns the files written.
func Generate(dir string) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	schemas := map[string]Schema{
		"event.json": {
			Schema:  "https://json-schema.org/draft/2020-12/schema",
			Title:   "Kite Event",
			Version: "kite.event/v1",
			Type:    "object",
			Properties: map[string]any{
				"id":         map[string]any{"type": "string"},
				"seq":        map[string]any{"type": "integer"},
				"session_id": map[string]any{"type": "string"},
				"type":       map[string]any{"type": "string"},
				"time":       map[string]any{"type": "string", "format": "date-time"},
				"payload":    map[string]any{"type": "object"},
			},
			Required: []string{"id", "seq", "session_id", "type", "time"},
		},
		"result.json": {
			Schema:  "https://json-schema.org/draft/2020-12/schema",
			Title:   "Kite Result",
			Version: "kite.result/v1",
			Type:    "object",
			Properties: map[string]any{
				"status":                 map[string]any{"type": "string", "enum": []string{"completed", "failed"}},
				"text":                   map[string]any{"type": "string"},
				"changed_files":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"changed_files_complete": map[string]any{"type": "boolean"},
				"verification": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command":   map[string]any{"type": "string"},
						"status":    map[string]any{"type": "string", "enum": []string{"passed", "failed"}},
						"exit_code": map[string]any{"type": "integer"},
						"artifacts": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"stale":     map[string]any{"type": "boolean"},
					},
				},
				"usage": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"prompt_tokens":     map[string]any{"type": "integer"},
						"completion_tokens": map[string]any{"type": "integer"},
						"total_tokens":      map[string]any{"type": "integer"},
					},
				},
			},
			Required: []string{"status"},
		},
		"rpc-request.json": {
			Schema:  "https://json-schema.org/draft/2020-12/schema",
			Title:   "Kite RPC Request",
			Version: "kite.rpc.request/v1",
			Type:    "object",
			Properties: map[string]any{
				"id":     map[string]any{"type": "string"},
				"method": map[string]any{"type": "string"},
				"params": map[string]any{"type": "object"},
			},
			Required: []string{"id", "method"},
		},
		"rpc-response.json": {
			Schema:  "https://json-schema.org/draft/2020-12/schema",
			Title:   "Kite RPC Response",
			Version: "kite.rpc.response/v1",
			Type:    "object",
			Properties: map[string]any{
				"id":     map[string]any{"type": "string"},
				"method": map[string]any{"type": "string"},
				"ok":     map[string]any{"type": "boolean"},
				"result": map[string]any{},
				"error": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"code":    map[string]any{"type": "string"},
						"message": map[string]any{"type": "string"},
					},
				},
			},
			Required: []string{"id", "method", "ok"},
		},
	}

	var written []string
	for name, s := range schemas {
		data, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			return nil, err
		}
		data = append(data, '\n')
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return nil, err
		}
		written = append(written, path)
	}
	return written, nil
}

// Check compares the committed schemas against freshly generated ones and
// reports whether they differ.
func Check(dir string) error {
	generated, err := Generate(filepath.Join(os.TempDir(), "kite-schemas"))
	if err != nil {
		return err
	}
	for _, g := range generated {
		name := filepath.Base(g)
		committed := filepath.Join(dir, name)
		gData, err := os.ReadFile(g)
		if err != nil {
			return err
		}
		cData, err := os.ReadFile(committed)
		if err != nil {
			return fmt.Errorf("schema %s is missing; run go run ./internal/schemagen", name)
		}
		if string(gData) != string(cData) {
			return fmt.Errorf("schema %s is out of date; run go run ./internal/schemagen", name)
		}
	}
	return nil
}
