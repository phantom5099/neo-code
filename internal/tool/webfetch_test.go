package tool_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	toolweb "neo-code/internal/tool/web"
)

func TestWebFetchToolRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("hello from webfetch"))
	}))
	defer server.Close()

	tool := toolweb.NewFetchTool()
	result := tool.Run(map[string]interface{}{
		"url":      server.URL,
		"maxBytes": 128,
	})
	if result == nil || !result.Success {
		t.Fatalf("expected successful fetch, got %+v", result)
	}
	if !strings.Contains(result.Output, "hello from webfetch") {
		t.Fatalf("expected response body in output, got %q", result.Output)
	}
	if result.Metadata["statusCode"] != 200 {
		t.Fatalf("expected status code 200, got %+v", result.Metadata)
	}
}
