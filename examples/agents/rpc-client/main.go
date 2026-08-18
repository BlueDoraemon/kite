// Command rpc-client demonstrates the NDJSON RPC protocol: it sends requests
// to `kite rpc` over stdin and reads responses from stdout.
//
// Usage:
//
//	KITE_API_KEY=sk-... go run ./examples/agents/rpc-client
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// request is a kite.rpc.request/v1 request.
type request struct {
	ID     string `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

// response is a kite.rpc.response/v1 response.
type response struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func main() {
	// Start kite rpc as a subprocess.
	cmd := exec.Command("kite", "rpc")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Send an inspect request.
	req := request{ID: "1", Method: "inspect", Params: map[string]any{"tool_id": "read"}}
	data, _ := json.Marshal(req)
	fmt.Fprintln(stdin, string(data))
	stdin.Close()

	// Read the response.
	scanner := bufio.NewScanner(stdout)
	if scanner.Scan() {
		var resp response
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("id=%s method=%s ok=%v\n", resp.ID, resp.Method, resp.OK)
		if resp.Error != nil {
			fmt.Printf("error: %s: %s\n", resp.Error.Code, resp.Error.Message)
		} else {
			fmt.Printf("result: %s\n", string(resp.Result))
		}
	}
	_ = io.EOF
	_ = strings.TrimSpace
	cmd.Wait()
}
