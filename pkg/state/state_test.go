package state

import (
	"testing"

	"orch.io/pkg/logging"
	manifestcore "orch.io/pkg/manifest/core"
)

func TestBeginComponentApplyPreservesDestroyData(t *testing.T) {
	current := New("env", "manifest", &logging.NoopDebugLogger{})
	component := &manifestcore.Component{
		Name:   "db",
		Type:   "terraform",
		Runner: "local",
		Config: map[string]interface{}{
			"name": "db",
		},
	}

	current.UpsertComponent(ComponentState{
		Name:    "db",
		Type:    "terraform",
		Runner:  RunnerRef{Name: "local", Type: "local"},
		WorkDir: "/tmp/db",
		Outputs: map[string]string{
			"url": "postgres://example",
		},
		Payload: map[string]interface{}{
			"workdir": "/tmp/db",
		},
		Artifacts: []Artifact{
			{Name: "tfstate", Path: "terraform.tfstate", Required: true},
		},
		Status: StatusApplied,
	})

	current.BeginComponentApply(component, "local", "/tmp/db")
	got, ok := current.FindComponent("db")
	if !ok {
		t.Fatal("component not found")
	}
	if got.Status != StatusApplying {
		t.Fatalf("status = %q, want %q", got.Status, StatusApplying)
	}
	if got.Payload["workdir"] != "/tmp/db" {
		t.Fatalf("payload was not preserved: %#v", got.Payload)
	}
	if len(got.Artifacts) != 1 {
		t.Fatalf("artifacts were not preserved: %#v", got.Artifacts)
	}
}

func TestMarkComponentStatus(t *testing.T) {
	current := New("env", "manifest", &logging.NoopDebugLogger{})
	component := &manifestcore.Component{Name: "app", Type: "script", Runner: "local"}
	current.BeginComponentApply(component, "local", "/tmp/app")

	current.MarkComponentFailed("app")
	got, ok := current.FindComponent("app")
	if !ok {
		t.Fatal("component not found")
	}
	if got.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", got.Status, StatusFailed)
	}

	current.MarkComponentDestroying("app")
	got, _ = current.FindComponent("app")
	if got.Status != StatusDestroying {
		t.Fatalf("status = %q, want %q", got.Status, StatusDestroying)
	}
}
