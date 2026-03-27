package web

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const defaultSearchEndpoint = "https://duckduckgo.com/html/?q={query}"

type FetchResult struct {
	URL         string
	StatusCode  int
	ContentType string
	Body        string
}

type SearchResult struct {
	Title   string
	URL     string
	Snippet string
}

func DefaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

func NormalizeHTTPURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return nil, fmt.Errorf("unsupported url scheme %q", parsed.Scheme)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return nil, fmt.Errorf("url host is required")
	}
	return parsed, nil
}

func EndpointHost(rawURL string) (string, error) {
	u, err := NormalizeHTTPURL(strings.ReplaceAll(rawURL, "{query}", "ping"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(u.Hostname()), nil
}

func Fetch(ctx context.Context, client *http.Client, rawURL string, maxBytes int) (*FetchResult, error) {
	u, err := NormalizeHTTPURL(rawURL)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = DefaultHTTPClient()
	}
	if maxBytes <= 0 {
		maxBytes = 20_000
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "NeoCode/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxBytes)))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	return &FetchResult{
		URL:         u.String(),
		StatusCode:  resp.StatusCode,
		ContentType: strings.TrimSpace(resp.Header.Get("Content-Type")),
		Body:        string(bodyBytes),
	}, nil
}

func Search(ctx context.Context, client *http.Client, endpointTemplate, query string, limit int) ([]SearchResult, string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, "", fmt.Errorf("query is required")
	}
	if client == nil {
		client = DefaultHTTPClient()
	}
	if limit <= 0 {
		limit = 5
	}
	if strings.TrimSpace(endpointTemplate) == "" {
		endpointTemplate = defaultSearchEndpoint
	}

	endpoint, err := buildSearchURL(endpointTemplate, query)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create search request: %w", err)
	}
	req.Header.Set("User-Agent", "NeoCode/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, endpoint, fmt.Errorf("send search request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 128_000))
	if err != nil {
		return nil, endpoint, fmt.Errorf("read search response: %w", err)
	}
	results := parseSearchResults(string(bodyBytes), limit)
	return results, endpoint, nil
}

func buildSearchURL(endpointTemplate, query string) (string, error) {
	trimmed := strings.TrimSpace(endpointTemplate)
	if strings.Contains(trimmed, "{query}") {
		trimmed = strings.ReplaceAll(trimmed, "{query}", url.QueryEscape(query))
	}
	u, err := NormalizeHTTPURL(trimmed)
	if err != nil {
		return "", err
	}
	if !strings.Contains(endpointTemplate, "{query}") {
		values := u.Query()
		if strings.TrimSpace(values.Get("q")) == "" {
			values.Set("q", query)
		}
		u.RawQuery = values.Encode()
	}
	return u.String(), nil
}

var (
	duckDuckGoLinkPattern = regexp.MustCompile(`(?is)<a[^>]*class="[^"]*result__a[^"]*"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	anchorPattern         = regexp.MustCompile(`(?is)<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	tagPattern            = regexp.MustCompile(`(?is)<[^>]+>`)
)

func parseSearchResults(body string, limit int) []SearchResult {
	if limit <= 0 {
		limit = 5
	}

	results := parseAnchors(duckDuckGoLinkPattern.FindAllStringSubmatch(body, limit))
	if len(results) > 0 {
		return results
	}
	return parseAnchors(anchorPattern.FindAllStringSubmatch(body, limit))
}

func parseAnchors(matches [][]string) []SearchResult {
	results := make([]SearchResult, 0, len(matches))
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		link := normalizeResultURL(match[1])
		title := cleanHTML(match[2])
		if strings.TrimSpace(link) == "" || strings.TrimSpace(title) == "" {
			continue
		}
		results = append(results, SearchResult{
			Title: title,
			URL:   link,
		})
	}
	return results
}

func normalizeResultURL(raw string) string {
	raw = html.UnescapeString(strings.TrimSpace(raw))
	if strings.Contains(raw, "uddg=") {
		if parsed, err := url.Parse(raw); err == nil {
			if target := parsed.Query().Get("uddg"); strings.TrimSpace(target) != "" {
				raw = target
			}
		}
	}
	return raw
}

func cleanHTML(input string) string {
	withoutTags := tagPattern.ReplaceAllString(input, " ")
	return strings.Join(strings.Fields(html.UnescapeString(withoutTags)), " ")
}
