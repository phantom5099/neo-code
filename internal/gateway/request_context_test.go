package gateway

import (
	"context"
	"log"
	"os"
	"testing"
)

type stubTokenAuthenticator struct {
	token string
}

func (a stubTokenAuthenticator) ValidateToken(token string) bool {
	return token == a.token
}

func TestConnectionAuthState(t *testing.T) {
	state := NewConnectionAuthState()
	if state.IsAuthenticated() {
		t.Fatal("new state should be unauthenticated")
	}
	state.MarkAuthenticated()
	if !state.IsAuthenticated() {
		t.Fatal("state should be authenticated")
	}
}

func TestRequestContextHelpers(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestSource(ctx, RequestSourceHTTP)
	ctx = WithRequestToken(ctx, " token-1 ")

	state := NewConnectionAuthState()
	ctx = WithConnectionAuthState(ctx, state)

	authenticator := stubTokenAuthenticator{token: "token-1"}
	ctx = WithTokenAuthenticator(ctx, authenticator)
	ctx = WithRequestACL(ctx, NewStrictControlPlaneACL())
	metrics := NewGatewayMetrics()
	ctx = WithGatewayMetrics(ctx, metrics)
	logger := log.New(os.Stderr, "", 0)
	ctx = WithGatewayLogger(ctx, logger)

	if source := RequestSourceFromContext(ctx); source != RequestSourceHTTP {
		t.Fatalf("source = %q, want %q", source, RequestSourceHTTP)
	}
	if token := RequestTokenFromContext(ctx); token != "token-1" {
		t.Fatalf("token = %q, want %q", token, "token-1")
	}
	if loadedState, ok := ConnectionAuthStateFromContext(ctx); !ok || loadedState != state {
		t.Fatal("expected to load connection auth state")
	}
	if loadedAuthenticator, ok := TokenAuthenticatorFromContext(ctx); !ok || !loadedAuthenticator.ValidateToken("token-1") {
		t.Fatal("expected to load token authenticator")
	}
	if acl, ok := RequestACLFromContext(ctx); !ok || acl == nil {
		t.Fatal("expected to load acl")
	}
	if loadedMetrics, ok := GatewayMetricsFromContext(ctx); !ok || loadedMetrics != metrics {
		t.Fatal("expected to load metrics")
	}
	if loadedLogger, ok := GatewayLoggerFromContext(ctx); !ok || loadedLogger != logger {
		t.Fatal("expected to load logger")
	}
}
