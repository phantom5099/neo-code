package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStdIOClientListToolsAndCallTool(t *testing.T) {
	t.Parallel()

	client := newTestStdIOClient(t)
	defer func() { _ = client.Close() }()

	toolsList, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(toolsList) != 1 || toolsList[0].Name != "search" {
		t.Fatalf("unexpected tools list: %+v", toolsList)
	}

	result, err := client.CallTool(context.Background(), "search", []byte(`{"query":"mcp"}`))
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if !strings.Contains(result.Content, "search") {
		t.Fatalf("unexpected call result content: %q", result.Content)
	}
}

func TestStdIOClientHealthCheck(t *testing.T) {
	t.Parallel()

	client := newTestStdIOClient(t)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.HealthCheck(ctx); err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
}

func TestStdIOClientConcurrentCallTool(t *testing.T) {
	t.Parallel()

	client := newTestStdIOClient(t)
	defer func() { _ = client.Close() }()

	const workers = 16
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := client.CallTool(context.Background(), "search", []byte(`{"query":"mcp"}`))
			if err != nil {
				errCh <- err
				return
			}
			if !strings.Contains(result.Content, "search") {
				errCh <- fmt.Errorf("unexpected content: %q", result.Content)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent call failed: %v", err)
	}
}

func TestReadFramedMessageRejectsOversizedPayload(t *testing.T) {
	t.Parallel()

	payload := strings.Repeat("x", 32)
	raw := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", maxStdioFrameBytes+1, payload)
	reader := bufio.NewReader(strings.NewReader(raw))
	_, err := readFramedMessage(reader)
	if err == nil {
		t.Fatalf("expected oversized payload error")
	}
	if !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("expected exceeds limit error, got %v", err)
	}
}

func newTestStdIOClient(t *testing.T) *StdIOClient {
	t.Helper()

	client, err := NewStdIOClient(StdioClientConfig{
		Command:      os.Args[0],
		Args:         []string{"-test.run=TestHelperProcessMCPStdioServer", "--"},
		Env:          []string{"GO_WANT_MCP_STDIO_HELPER=1"},
		StartTimeout: 3 * time.Second,
		CallTimeout:  3 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewStdIOClient() error = %v", err)
	}
	return client
}

func TestHelperProcessMCPStdioServer(t *testing.T) {
	if os.Getenv("GO_WANT_MCP_STDIO_HELPER") != "1" {
		return
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		payload, err := readFramedMessage(reader)
		if err != nil {
			if err == io.EOF {
				os.Exit(0)
			}
			os.Exit(2)
		}

		var request map[string]any
		if err := json.Unmarshal(payload, &request); err != nil {
			os.Exit(3)
		}

		method, _ := request["method"].(string)
		requestID, _ := request["id"].(string)

		var response any
		switch method {
		case "tools/list":
			response = map[string]any{
				"jsonrpc": "2.0",
				"id":      requestID,
				"result": map[string]any{
					"tools": []map[string]any{
						{
							"name":        "search",
							"description": "search docs",
							"inputSchema": map[string]any{
								"type":       "object",
								"properties": map[string]any{"query": map[string]any{"type": "string"}},
							},
						},
					},
				},
			}
		case "tools/call":
			params, _ := request["params"].(map[string]any)
			name, _ := params["name"].(string)
			response = map[string]any{
				"jsonrpc": "2.0",
				"id":      requestID,
				"result": map[string]any{
					"content": fmt.Sprintf("ok:%s", name),
					"isError": false,
				},
			}
		default:
			response = map[string]any{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": map[string]any{
					"code":    -32601,
					"message": "method not found",
				},
			}
		}

		rawResponse, err := json.Marshal(response)
		if err != nil {
			os.Exit(4)
		}
		if err := writeFramedMessage(os.Stdout, rawResponse); err != nil {
			os.Exit(5)
		}
	}
}
