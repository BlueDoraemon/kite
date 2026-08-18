package rpc

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// echoHandler echoes the request params back.
type echoHandler struct{}

func (echoHandler) Handle(req *Request) (*Response, error) {
	return &Response{ID: req.ID, Method: req.Method, OK: true, Result: req.Params}, nil
}

func TestServeSequentialRequests(t *testing.T) {
	var buf bytes.Buffer
	input := `{"id":"1","method":"prompt","params":{"text":"hi"}}
{"id":"2","method":"status"}
`
	srv := NewServer(echoHandler{})
	if err := srv.Serve(strings.NewReader(input), &buf); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("responses = %d, want 2", len(lines))
	}
	var r1 Response
	if err := json.Unmarshal([]byte(lines[0]), &r1); err != nil {
		t.Fatal(err)
	}
	if r1.ID != "1" || !r1.OK {
		t.Fatalf("response 1 = %+v", r1)
	}
}

func TestServeMalformedLine(t *testing.T) {
	var buf bytes.Buffer
	input := "not json\n{\"id\":\"2\",\"method\":\"status\"}\n"
	srv := NewServer(echoHandler{})
	if err := srv.Serve(strings.NewReader(input), &buf); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("responses = %d, want 2", len(lines))
	}
	var bad Response
	json.Unmarshal([]byte(lines[0]), &bad)
	if bad.OK || bad.Error == nil || bad.Error.Code != "bad_request" {
		t.Fatalf("bad request response = %+v", bad)
	}
}

func TestServeLineLimit(t *testing.T) {
	// A line over 1 MiB should not be read; the scanner returns an error.
	var buf bytes.Buffer
	input := strings.Repeat("x", maxLineBytes+10) + "\n"
	srv := NewServer(echoHandler{})
	err := srv.Serve(strings.NewReader(input), &buf)
	if err == nil {
		t.Fatal("expected error for oversized line")
	}
}

func TestParseRequest(t *testing.T) {
	req, err := ParseRequest(`{"id":"1","method":"prompt","params":{"text":"hi"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if req.ID != "1" || req.Method != MethodPrompt {
		t.Fatalf("req = %+v", req)
	}
}
