package tool_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	toolweb "neo-code/internal/tool/web"
)

func TestWebSearchToolRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
			<html><body>
				<a class="result__a" href="https://example.com/one">First Result</a>
				<a class="result__a" href="https://example.com/two">Second Result</a>
			</body></html>
		`))
	}))
	defer server.Close()

	tool := toolweb.NewSearchTool()
	result := tool.Run(map[string]interface{}{
		"query":    "neocode",
		"endpoint": server.URL + "?q={query}",
		"limit":    2,
	})
	if result == nil || !result.Success {
		t.Fatalf("expected successful search, got %+v", result)
	}
	if !strings.Contains(result.Output, "First Result") || !strings.Contains(result.Output, "https://example.com/two") {
		t.Fatalf("expected parsed results, got %q", result.Output)
	}
	if result.Metadata["resultCount"] != 2 {
		t.Fatalf("expected result count 2, got %+v", result.Metadata)
	}
}
