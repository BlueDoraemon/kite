// Package tools implements the built-in repository tools that the agent can
// call: read, edit, and bash. They operate on a working directory.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BlueDoraemon/kite/internal/kite"
)

// Tool is a named function the model can call. It implements kite.Tool.
type Tool struct {
	name        string
	description string
	specs       []argSpec
	run         func(ctx context.Context, args map[string]any) (string, error)
}

func (t *Tool) Name() string        { return t.name }
func (t *Tool) Description() string { return t.description }
func (t *Tool) Schema() any         { return schemaOf(t.specs) }

// Run decodes the input arguments and invokes the tool's function.
func (t *Tool) Run(ctx context.Context, input string) (string, error) {
	args, err := parseInput(input, t.specs)
	if err != nil {
		return "", err
	}
	return t.run(ctx, args)
}

type argSpec struct {
	name, typ, desc string
	required        bool
}

func schemaOf(specs []argSpec) map[string]any {
	props := map[string]any{}
	required := []string{}
	for _, s := range specs {
		props[s.name] = map[string]any{"type": s.typ, "description": s.desc}
		if s.required {
			required = append(required, s.name)
		}
	}
	m := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

func parseInput(input string, specs []argSpec) (map[string]any, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON input: %w", err)
	}
	out := map[string]any{}
	for _, s := range specs {
		val, ok := raw[s.name]
		if !ok {
			if s.required {
				return nil, fmt.Errorf("missing required %q argument", s.name)
			}
			continue
		}
		switch s.typ {
		case "string":
			var v string
			if err := json.Unmarshal(val, &v); err != nil {
				return nil, fmt.Errorf("argument %q must be a string", s.name)
			}
			out[s.name] = v
		case "integer":
			var v int
			if err := json.Unmarshal(val, &v); err != nil {
				return nil, fmt.Errorf("argument %q must be an integer", s.name)
			}
			out[s.name] = v
		case "boolean":
			var v bool
			if err := json.Unmarshal(val, &v); err != nil {
				return nil, fmt.Errorf("argument %q must be a boolean", s.name)
			}
			out[s.name] = v
		}
	}
	return out, nil
}

func str(m map[string]any, k string) string { v, _ := m[k].(string); return v }
func boolv(m map[string]any, k string) bool { v, _ := m[k].(bool); return v }

// Set holds the tools that work against a working directory.
type Set struct {
	Dir string
}

// All returns the read, edit, and bash tools.
func (s *Set) All() []kite.Tool {
	return []kite.Tool{s.Read(), s.Edit(), s.Bash()}
}

// resolve returns the absolute path for a repository-relative path, safe
// against attempts to escape the working directory.
func (s *Set) resolve(path string) (string, error) {
	abs, err := filepath.Abs(filepath.Join(s.Dir, path))
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(s.Dir)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside the working directory", path)
	}
	return abs, nil
}
