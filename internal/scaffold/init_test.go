package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInitWritesStarterManifest(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "orch.yaml")

	if err := RunInit(InitOptions{Path: manifestPath, ID: "demo"}); err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}
	content := string(data)
	for _, expected := range []string{
		"version: orch/1.0",
		"id: demo",
		"type: local",
		"type: script",
		`echo "message=hello from orch" >> "$ORCH_OUTPUT_ENV"`,
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("manifest missing %q:\n%s", expected, content)
		}
	}
}

func TestRunInitSanitizesExplicitID(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "orch.yaml")

	if err := RunInit(InitOptions{Path: manifestPath, ID: "My App!"}); err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}
	if !strings.Contains(string(data), "id: my-app") {
		t.Fatalf("manifest did not sanitize id:\n%s", data)
	}
}

func TestRunInitRefusesOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "orch.yaml")
	if err := os.WriteFile(manifestPath, []byte("existing"), 0644); err != nil {
		t.Fatalf("failed to seed manifest: %v", err)
	}

	err := RunInit(InitOptions{Path: manifestPath, ID: "demo"})
	if err == nil {
		t.Fatal("expected overwrite error")
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}
	if string(data) != "existing" {
		t.Fatalf("manifest was overwritten without force: %q", data)
	}
}

func TestRunInitOverwritesWithForce(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "orch.yaml")
	if err := os.WriteFile(manifestPath, []byte("existing"), 0644); err != nil {
		t.Fatalf("failed to seed manifest: %v", err)
	}

	if err := RunInit(InitOptions{Path: manifestPath, ID: "demo", Force: true}); err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}
	if !strings.Contains(string(data), "id: demo") {
		t.Fatalf("manifest was not overwritten with starter content:\n%s", data)
	}
}

func TestSanitizeManifestID(t *testing.T) {
	tests := map[string]string{
		"My App":       "my-app",
		"@personal":    "personal",
		"orch_demo--x": "orch-demo-x",
		"!!!":          "orch-demo",
	}

	for input, expected := range tests {
		if got := sanitizeManifestID(input); got != expected {
			t.Fatalf("sanitizeManifestID(%q) = %q, want %q", input, got, expected)
		}
	}
}
