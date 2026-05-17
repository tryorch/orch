package varresolvers

import (
	"context"
	"strings"
	"testing"

	manifestcore "orch.io/pkg/manifest/core"
)

func TestComponentResolverUnavailableSensitiveOutput(t *testing.T) {
	resolver := NewComponentResolver()
	resolver.RegisterPersistedComponentOutput("db", map[string]string{
		"url": "postgres://example",
	})
	resolver.RegisterUnavailableSensitiveOutputs("db", []manifestcore.Output{
		{Name: "url"},
		{Name: "password", Sensitive: true},
	}, map[string]string{
		"url": "postgres://example",
	})

	got, err := resolver.Resolve(context.Background(), "db.outputs.url")
	if err != nil {
		t.Fatalf("expected non-sensitive output to resolve: %v", err)
	}
	if got != "postgres://example" {
		t.Fatalf("got %q", got)
	}

	_, err = resolver.Resolve(context.Background(), "db.outputs.password")
	if err == nil {
		t.Fatal("expected sensitive output error")
	}
	if !strings.Contains(err.Error(), "sensitive output") || !strings.Contains(err.Error(), "not persisted in state") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComponentResolverFreshOutputClearsUnavailableSensitiveOutput(t *testing.T) {
	resolver := NewComponentResolver()
	outputs := []manifestcore.Output{{Name: "password", Sensitive: true}}

	resolver.RegisterUnavailableSensitiveOutputs("db", outputs, nil)
	resolver.RegisterComponentOutput("db", outputs, map[string]string{
		"password": "abc",
	})

	got, err := resolver.Resolve(context.Background(), "db.outputs.password")
	if err != nil {
		t.Fatalf("expected fresh sensitive output to resolve: %v", err)
	}
	if got != "abc" {
		t.Fatalf("got %q", got)
	}
}
