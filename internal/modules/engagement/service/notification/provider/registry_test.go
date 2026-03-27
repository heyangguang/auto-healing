package provider

import (
	"context"
	"testing"
)

type stubProvider struct {
	providerType string
}

func (s stubProvider) Type() string { return s.providerType }

func (s stubProvider) Send(context.Context, *SendRequest) (*SendResponse, error) {
	return &SendResponse{Success: true}, nil
}

func (s stubProvider) Test(context.Context, map[string]interface{}) error { return nil }

func TestNewRegistryRegistersDefaults(t *testing.T) {
	registry := NewRegistry()

	for _, want := range []string{"webhook", "dingtalk", "email"} {
		if _, ok := registry.Get(want); !ok {
			t.Fatalf("registry missing default provider %q", want)
		}
	}

	types := registry.List()
	if len(types) != 3 {
		t.Fatalf("List() len = %d, want 3", len(types))
	}
}

func TestRegistryRegisterOverridesAndListsCustomProvider(t *testing.T) {
	registry := NewRegistry()
	registry.Register(stubProvider{providerType: "custom"})

	got, ok := registry.Get("custom")
	if !ok {
		t.Fatal("Get(custom) ok = false, want true")
	}
	if got.Type() != "custom" {
		t.Fatalf("Get(custom).Type() = %q, want %q", got.Type(), "custom")
	}

	found := false
	for _, providerType := range registry.List() {
		if providerType == "custom" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("List() missing custom provider")
	}
}
