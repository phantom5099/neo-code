package config

import "testing"

func TestProviderIdentityKeyIncludesDriverSpecificFields(t *testing.T) {
	t.Parallel()

	identity := ProviderIdentity{
		Driver:         "openaicompat",
		BaseURL:        "https://api.example.com/v1",
		APIStyle:       "responses",
		DeploymentMode: "ignored",
		APIVersion:     "ignored",
	}

	if got, want := identity.Key(), "openaicompat|https://api.example.com/v1|responses|ignored|ignored"; got != want {
		t.Fatalf("expected identity key %q, got %q", want, got)
	}
}

func TestNewProviderIdentityFromConfigUsesDriverSpecificNormalization(t *testing.T) {
	t.Parallel()

	provider := ProviderConfig{
		Name:      "gateway",
		Driver:    " OpenAICompat ",
		BaseURL:   "https://API.EXAMPLE.COM/v1/",
		Model:     "gpt-4.1",
		APIKeyEnv: "GATEWAY_API_KEY",
		APIStyle:  " Responses ",
		Source:    ProviderSourceCustom,
	}

	identity, err := NewProviderIdentityFromConfig(provider)
	if err != nil {
		t.Fatalf("NewProviderIdentityFromConfig() error = %v", err)
	}

	if identity.Driver != "openaicompat" {
		t.Fatalf("expected normalized driver %q, got %q", "openaicompat", identity.Driver)
	}
	if identity.BaseURL != "https://api.example.com/v1" {
		t.Fatalf("expected normalized base url %q, got %q", "https://api.example.com/v1", identity.BaseURL)
	}
	if identity.APIStyle != "responses" {
		t.Fatalf("expected normalized api_style %q, got %q", "responses", identity.APIStyle)
	}
}

func TestNormalizeProviderIdentityPreservesDriverSpecificFields(t *testing.T) {
	t.Parallel()

	identity, err := NormalizeProviderIdentity(ProviderIdentity{
		Driver:         " Gemini ",
		BaseURL:        "https://API.EXAMPLE.COM/v1/",
		DeploymentMode: " Vertex ",
	})
	if err != nil {
		t.Fatalf("NormalizeProviderIdentity() error = %v", err)
	}

	if identity.Driver != "gemini" {
		t.Fatalf("expected normalized driver %q, got %q", "gemini", identity.Driver)
	}
	if identity.BaseURL != "https://api.example.com/v1" {
		t.Fatalf("expected normalized base url %q, got %q", "https://api.example.com/v1", identity.BaseURL)
	}
	if identity.DeploymentMode != "vertex" {
		t.Fatalf("expected normalized deployment_mode %q, got %q", "vertex", identity.DeploymentMode)
	}
}
