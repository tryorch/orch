package orchestration

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"orch.io/pkg/state"
)

func TestRenderStateInspectTable(t *testing.T) {
	current := &state.OrchState{
		EnvID:      "pr-123",
		ManifestID: "preview",
		UpdatedAt:  "2026-05-17T12:00:00Z",
		Components: []state.ComponentState{
			{
				Name:      "db",
				Type:      "terraform",
				Runner:    state.RunnerRef{Name: "ionos"},
				Status:    state.StatusApplied,
				Stage:     state.StagePostApply,
				UpdatedAt: "2026-05-17T12:00:00Z",
				Outputs:   map[string]string{"url": "postgres://example"},
				Payload:   map[string]interface{}{"workdir": "/tmp/db"},
			},
			{
				Name:      "api",
				Type:      "script",
				Runner:    state.RunnerRef{Name: "ionos"},
				Status:    state.StatusFailed,
				Stage:     state.StageOutputs,
				UpdatedAt: "2026-05-17T12:01:00Z",
			},
		},
	}

	var out bytes.Buffer
	if err := renderStateInspectTable(&out, current); err != nil {
		t.Fatalf("render failed: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"Environment: pr-123",
		"COMPONENT",
		"db",
		"applied",
		"post_apply",
		"api failed during outputs",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "postgres://example") || strings.Contains(got, "/tmp/db") {
		t.Fatalf("table output leaked outputs or payload:\n%s", got)
	}
}

func TestRenderStateInspectJSON(t *testing.T) {
	current := &state.OrchState{
		EnvID:      "pr-123",
		ManifestID: "preview",
	}

	var out bytes.Buffer
	if err := renderStateInspectJSON(&out, current); err != nil {
		t.Fatalf("render failed: %v", err)
	}

	var decoded state.OrchState
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if decoded.EnvID != "pr-123" {
		t.Fatalf("env id = %q", decoded.EnvID)
	}
}

func TestRecoveryHints(t *testing.T) {
	hints := recoveryHints([]state.ComponentState{
		{Name: "api", Status: state.StatusFailed, Stage: state.StageApply},
		{Name: "db", Status: state.StatusFailed, Stage: state.StageDestroy},
		{Name: "worker", Status: state.StatusDestroying, Stage: state.StageDestroy},
	})

	if len(hints) != 3 {
		t.Fatalf("expected 3 hints, got %d", len(hints))
	}
	if !strings.Contains(hints[0], "orch up") {
		t.Fatalf("expected apply-side hint to mention up: %q", hints[0])
	}
	if !strings.Contains(hints[1], "orch down") || strings.Contains(hints[1], "orch up") {
		t.Fatalf("expected destroy-side hint to mention only down: %q", hints[1])
	}
}
