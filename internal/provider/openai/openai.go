package openai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	sdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"neo-code/internal/config"
	domain "neo-code/internal/provider"
)

const DriverName = "openai"

type Provider struct {
	cfg        config.ResolvedProviderConfig
	client     sdk.Client
	httpClient *http.Client
}

type buildOptions struct {
	httpClient     *http.Client
	transport      http.RoundTripper
	maxRetries     *int
	requestTimeout *time.Duration
}

type BuildOption func(*buildOptions)

// WithTransport injects a custom HTTP transport for SDK requests.
func WithTransport(rt http.RoundTripper) BuildOption {
	return func(o *buildOptions) {
		o.transport = rt
	}
}

// WithHTTPClient injects a custom HTTP client for SDK requests.
func WithHTTPClient(client *http.Client) BuildOption {
	return func(o *buildOptions) {
		o.httpClient = client
	}
}

// WithMaxRetries overrides the SDK retry count.
func WithMaxRetries(retries int) BuildOption {
	return func(o *buildOptions) {
		o.maxRetries = &retries
	}
}

// WithRequestTimeout overrides the SDK request timeout.
func WithRequestTimeout(timeout time.Duration) BuildOption {
	return func(o *buildOptions) {
		o.requestTimeout = &timeout
	}
}

// Driver returns the OpenAI provider driver definition.
func Driver() domain.DriverDefinition {
	return domain.DriverDefinition{
		Name: DriverName,
		Build: func(ctx context.Context, cfg config.ResolvedProviderConfig) (domain.Provider, error) {
			return New(cfg)
		},
		Discover: func(ctx context.Context, cfg config.ResolvedProviderConfig) ([]config.ModelDescriptor, error) {
			provider, err := New(cfg)
			if err != nil {
				return nil, err
			}
			return provider.DiscoverModels(ctx)
		},
	}
}

// New constructs an OpenAI provider backed by the official openai-go SDK.
func New(cfg config.ResolvedProviderConfig, opts ...BuildOption) (*Provider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("openai provider: %w", err)
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("openai provider: api key is empty")
	}

	buildOpts := buildOptions{}
	for _, apply := range opts {
		apply(&buildOpts)
	}

	if buildOpts.maxRetries != nil && *buildOpts.maxRetries < 0 {
		return nil, errors.New("openai provider: max retries must be non-negative")
	}
	if buildOpts.requestTimeout != nil && *buildOpts.requestTimeout < 0 {
		return nil, errors.New("openai provider: request timeout must be non-negative")
	}

	requestOptions := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
		option.WithBaseURL(cfg.BaseURL),
	}

	httpClient := buildOpts.resolvedHTTPClient()
	if httpClient != nil {
		requestOptions = append(requestOptions, option.WithHTTPClient(httpClient))
	}
	if buildOpts.maxRetries != nil {
		requestOptions = append(requestOptions, option.WithMaxRetries(*buildOpts.maxRetries))
	}
	if buildOpts.requestTimeout != nil {
		requestOptions = append(requestOptions, option.WithRequestTimeout(*buildOpts.requestTimeout))
	}

	return &Provider{
		cfg:        cfg,
		client:     sdk.NewClient(requestOptions...),
		httpClient: httpClient,
	}, nil
}

func (o buildOptions) resolvedHTTPClient() *http.Client {
	if o.httpClient == nil && o.transport == nil {
		return nil
	}
	if o.httpClient == nil {
		return &http.Client{Transport: o.transport}
	}

	cloned := *o.httpClient
	if o.transport != nil {
		cloned.Transport = o.transport
	}
	return &cloned
}

func (p *Provider) DiscoverModels(ctx context.Context) ([]config.ModelDescriptor, error) {
	pager := p.client.Models.ListAutoPaging(ctx)
	descriptors := make([]config.ModelDescriptor, 0)

	for pager.Next() {
		model := pager.Current()
		if descriptor, ok := descriptorFromSDKModel(model); ok {
			descriptors = append(descriptors, descriptor)
			continue
		}

		id := strings.TrimSpace(model.ID)
		if id == "" {
			continue
		}
		descriptors = append(descriptors, config.ModelDescriptor{
			ID:   id,
			Name: id,
		})
	}

	if err := pager.Err(); err != nil {
		return nil, mapProviderError(err)
	}

	return config.MergeModelDescriptors(descriptors), nil
}

func (p *Provider) Chat(
	ctx context.Context,
	req domain.ChatRequest,
	events chan<- domain.StreamEvent,
) (domain.ChatResponse, error) {
	params, err := p.buildRequest(req)
	if err != nil {
		return domain.ChatResponse{}, err
	}

	stream := p.client.Chat.Completions.NewStreaming(ctx, params)
	defer func() {
		_ = stream.Close()
	}()

	return p.consumeStream(ctx, stream, events)
}
