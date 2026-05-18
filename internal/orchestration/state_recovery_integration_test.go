package orchestration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orch.io/internal/adapters"
	"orch.io/pkg/events"
	"orch.io/pkg/logging"
	manifestcore "orch.io/pkg/manifest/core"
	"orch.io/pkg/runners"
	"orch.io/pkg/state"
	statebackends "orch.io/pkg/state/backends"
)

type testLogger struct{}

func TestMain(m *testing.M) {
	_ = os.RemoveAll(".orch")
	code := m.Run()
	_ = os.RemoveAll(".orch")
	os.Exit(code)
}

func (t testLogger) Debug(string, ...logging.Field) {}
func (t testLogger) Info(string, ...logging.Field)  {}
func (t testLogger) Warn(string, ...logging.Field)  {}
func (t testLogger) Error(string, ...logging.Field) {}
func (t testLogger) With(...logging.Field) logging.Logger {
	return t
}
func (t testLogger) AsDebugLogger() logging.DebugLogger {
	return &logging.NoopDebugLogger{}
}

type failingDestroyAdapter struct{}

func (a failingDestroyAdapter) Apply(ctx context.Context, c *manifestcore.Component, r runners.Runner) (adapters.ComponentApplyResult, error) {
	return adapters.ComponentApplyResult{State: state.EmptyComponentStateData(c.WorkDir)}, nil
}

func (a failingDestroyAdapter) Destroy(ctx context.Context, c *manifestcore.Component, r runners.Runner) error {
	return nil
}

func (a failingDestroyAdapter) DestroyFromState(ctx context.Context, c state.ComponentState, r runners.Runner) error {
	return errors.New("planned destroy failure")
}

func (a failingDestroyAdapter) RequiredCapabilities() runners.Capabilities {
	return runners.Capabilities{}
}

func (a failingDestroyAdapter) SupportedSources() adapters.ComponentSourceSupport {
	return adapters.ComponentSourceSupport{}
}

func (a failingDestroyAdapter) ValidateAndLoadConfig(ctx context.Context, c *manifestcore.Component) (adapters.ComponentConfig, []events.Event, error) {
	return nil, nil, nil
}

func TestRunUpPersistsFailedPostApplyStage(t *testing.T) {
	stateRoot := t.TempDir()
	workRoot := t.TempDir()
	envID := "post-apply-failure"

	manifest := testManifest(stateRoot, manifestcore.Component{
		Name:    "setup",
		Type:    "script",
		Runner:  "local",
		WorkDir: workRoot,
		Source:  manifestcore.ComponentSource{Embedded: "exit 0\n"},
		Hooks: manifestcore.Hooks{
			PostApply: []manifestcore.Hook{{Command: "exit 7"}},
		},
	})

	err := RunUp(envID, manifest, testLogger{}, nil)
	if err == nil {
		t.Fatal("expected post_apply failure")
	}

	componentState := loadComponentState(t, stateRoot, envID, "setup")
	if componentState.Status != state.StatusFailed {
		t.Fatalf("status = %q, want %q", componentState.Status, state.StatusFailed)
	}
	if componentState.Stage != state.StagePostApply {
		t.Fatalf("stage = %q, want %q", componentState.Stage, state.StagePostApply)
	}
	if componentState.WorkDir == "" || len(componentState.Payload) == 0 {
		t.Fatalf("expected destroyable state to be preserved: %#v", componentState)
	}
}

func TestRunUpSkipsAppliedAndReappliesWithOption(t *testing.T) {
	stateRoot := t.TempDir()
	workRoot := t.TempDir()
	countFile := filepath.Join(t.TempDir(), "count")
	envID := "reapply"

	manifest := testManifest(stateRoot, manifestcore.Component{
		Name:    "setup",
		Type:    "script",
		Runner:  "local",
		WorkDir: workRoot,
		Source: manifestcore.ComponentSource{
			Embedded: `echo run >> "$COUNT_FILE"` + "\n",
		},
		Env: map[string]string{
			"COUNT_FILE": countFile,
		},
	})

	if err := RunUp(envID, manifest, testLogger{}, nil); err != nil {
		t.Fatalf("first up failed: %v", err)
	}
	assertLineCount(t, countFile, 1)

	if err := RunUp(envID, manifest, testLogger{}, nil); err != nil {
		t.Fatalf("second up failed: %v", err)
	}
	assertLineCount(t, countFile, 1)

	if err := RunUpWithOptions(envID, manifest, testLogger{}, nil, UpOptions{Reapply: true}); err != nil {
		t.Fatalf("reapply failed: %v", err)
	}
	assertLineCount(t, countFile, 2)
}

func TestRunUpSensitiveOutputFeedsSameRunButIsNotPersisted(t *testing.T) {
	stateRoot := t.TempDir()
	workRoot := t.TempDir()
	envID := "sensitive-output"

	manifest := testManifest(stateRoot,
		manifestcore.Component{
			Name:    "producer",
			Type:    "script",
			Runner:  "local",
			WorkDir: workRoot,
			Source: manifestcore.ComponentSource{
				Embedded: `echo "token=abc" >> "$ORCH_OUTPUT_ENV"` + "\n",
			},
			Outputs: []manifestcore.Output{
				{Name: "token", Sensitive: true},
			},
		},
		manifestcore.Component{
			Name:      "consumer",
			Type:      "script",
			Runner:    "local",
			DependsOn: []string{"producer"},
			WorkDir:   workRoot,
			Source: manifestcore.ComponentSource{
				Embedded: `test "$TOKEN_FROM_PRODUCER" = "abc"` + "\n",
			},
			Env: map[string]string{
				"TOKEN_FROM_PRODUCER": "${producer.outputs.token}",
			},
		},
	)

	if err := RunUp(envID, manifest, testLogger{}, nil); err != nil {
		t.Fatalf("up failed: %v", err)
	}

	producerState := loadComponentState(t, stateRoot, envID, "producer")
	if _, ok := producerState.Outputs["token"]; ok {
		t.Fatal("expected sensitive producer output to be absent from persisted state")
	}

	consumerState := loadComponentState(t, stateRoot, envID, "consumer")
	if consumerState.Status != state.StatusApplied {
		t.Fatalf("consumer status = %q, want %q", consumerState.Status, state.StatusApplied)
	}
}

func TestRunUpBlocksOnFailedDestroyStage(t *testing.T) {
	stateRoot := t.TempDir()
	workRoot := t.TempDir()
	envID := "failed-destroy-blocks"
	manifest := testManifest(stateRoot, manifestcore.Component{
		Name:    "setup",
		Type:    "script",
		Runner:  "local",
		WorkDir: workRoot,
		Source:  manifestcore.ComponentSource{Embedded: "exit 0\n"},
	})

	current := state.New(envID, "test", &logging.NoopDebugLogger{})
	current.UpsertComponent(state.ComponentState{
		Name:    "setup",
		Type:    "script",
		Runner:  state.RunnerRef{Name: "local", Type: runners.RunnerTypeLocal},
		WorkDir: filepath.Join(workRoot, "orch", envID, "setup"),
		Status:  state.StatusFailed,
		Stage:   state.StageDestroy,
	})
	saveState(t, stateRoot, envID, current)

	err := RunUp(envID, manifest, testLogger{}, nil)
	if err == nil {
		t.Fatal("expected up to block")
	}
	if !strings.Contains(err.Error(), "run down again") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunDownPersistsFailedDestroyStage(t *testing.T) {
	stateRoot := t.TempDir()
	envID := "destroy-failure"
	adapterType := "test-failing-destroy"
	adapters.Register(adapterType, failingDestroyAdapter{})

	manifest := testManifest(stateRoot, manifestcore.Component{
		Name:   "setup",
		Type:   adapterType,
		Runner: "local",
	})

	current := state.New(envID, "test", &logging.NoopDebugLogger{})
	current.UpsertComponent(state.ComponentState{
		Name:    "setup",
		Type:    adapterType,
		Runner:  state.RunnerRef{Name: "local", Type: runners.RunnerTypeLocal},
		Status:  state.StatusApplied,
		Stage:   state.StagePostApply,
		Payload: map[string]interface{}{},
	})
	saveState(t, stateRoot, envID, current)

	err := RunDown(envID, manifest, testLogger{})
	if err == nil {
		t.Fatal("expected down to fail")
	}

	componentState := loadComponentState(t, stateRoot, envID, "setup")
	if componentState.Status != state.StatusFailed {
		t.Fatalf("status = %q, want %q", componentState.Status, state.StatusFailed)
	}
	if componentState.Stage != state.StageDestroy {
		t.Fatalf("stage = %q, want %q", componentState.Stage, state.StageDestroy)
	}
}

func testManifest(stateRoot string, components ...manifestcore.Component) *manifestcore.Manifest {
	return &manifestcore.Manifest{
		Version: "v0alpha1",
		Metadata: manifestcore.Metadata{
			ID: "test",
			Owner: manifestcore.Owner{
				Name:  "test",
				Email: "test@example.com",
			},
		},
		State: &manifestcore.StateConfig{
			Backend: "local",
			Config: map[string]interface{}{
				"path": stateRoot,
			},
		},
		Runners: map[string]manifestcore.RunnerManifest{
			"local": {
				Type:   "local",
				Config: map[string]interface{}{},
			},
		},
		Components: components,
	}
}

func loadComponentState(t *testing.T, stateRoot string, envID string, componentName string) state.ComponentState {
	t.Helper()
	backend := statebackends.NewLocal(stateRoot, &logging.NoopDebugLogger{})
	manager := state.NewManager(envID, backend)
	current, err := manager.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	componentState, ok := current.FindComponent(componentName)
	if !ok {
		t.Fatalf("component %q not found in state", componentName)
	}
	return componentState
}

func saveState(t *testing.T, stateRoot string, envID string, current *state.OrchState) {
	t.Helper()
	backend := statebackends.NewLocal(stateRoot, &logging.NoopDebugLogger{})
	manager := state.NewManager(envID, backend)
	if err := manager.Save(current); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}
}

func assertLineCount(t *testing.T, path string, want int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %q: %v", path, err)
	}
	got := strings.Count(strings.TrimSpace(string(data)), "\n") + 1
	if got != want {
		t.Fatalf("line count = %d, want %d; content:\n%s", got, want, data)
	}
}
