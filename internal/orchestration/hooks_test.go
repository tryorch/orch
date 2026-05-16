package orchestration

import (
	"context"
	"testing"

	manifestcore "orch.io/pkg/manifest/core"
	"orch.io/pkg/runners"
	"orch.io/pkg/varresolvers"
)

type fakeHookRunner struct {
	commands []runners.ExecCommand
}

func (f *fakeHookRunner) Name() string { return "local" }

func (f *fakeHookRunner) Type() runners.RunnerType { return runners.RunnerTypeLocal }

func (f *fakeHookRunner) ValidateAndInitialize() error { return nil }

func (f *fakeHookRunner) Capabilities() runners.Capabilities {
	return runners.Capabilities{Exec: true}
}

func (f *fakeHookRunner) Exec(ctx context.Context, command runners.ExecCommand) (*runners.ExecResult, error) {
	f.commands = append(f.commands, command)
	return &runners.ExecResult{ExitCode: 0}, nil
}

func (f *fakeHookRunner) CopyFile(ctx context.Context, req runners.FileCopyRequest) (*runners.FileCopyResult, error) {
	return &runners.FileCopyResult{}, nil
}

func (f *fakeHookRunner) UsesNonAmbientCredentials() (bool, []string) { return false, nil }

func (f *fakeHookRunner) Disconnect() error { return nil }

func TestRunLifecycleHooksInterpolatesCommandAndEnv(t *testing.T) {
	componentResolver := varresolvers.NewComponentResolver()
	componentResolver.RegisterPersistedComponentOutput("database", map[string]string{
		"url": "postgres://localhost:5432/app",
	})

	runner := &fakeHookRunner{}
	component := &manifestcore.Component{Name: "api", Type: "docker-compose"}
	err := runLifecycleHooks(context.Background(), runner, []manifestcore.Hook{
		{
			Command: `echo "${database.outputs.url}"`,
			Env: map[string]string{
				"DATABASE_URL": "${database.outputs.url}",
			},
		},
	}, lifecyclePreApply, hookExecutionContext{
		envID:        "test-env",
		componentRef: component,
		component:    "api",
		runner:       "local",
		workDir:      "/tmp/orch/test-env/api",
		baseEnv:      map[string]string{"BASE": "present"},
		resolver:     componentResolver,
	})
	if err != nil {
		t.Fatalf("runLifecycleHooks returned error: %v", err)
	}

	if len(runner.commands) != 2 {
		t.Fatalf("expected mkdir plus hook command, got %d commands", len(runner.commands))
	}

	hookCommand := runner.commands[1]
	if got := hookCommand.Command; len(got) != 3 || got[0] != "sh" || got[1] != "-c" || got[2] != `echo "postgres://localhost:5432/app"` {
		t.Fatalf("unexpected hook command: %#v", got)
	}
	if hookCommand.Env["DATABASE_URL"] != "postgres://localhost:5432/app" {
		t.Fatalf("expected interpolated hook env, got %q", hookCommand.Env["DATABASE_URL"])
	}
	if hookCommand.Env["ORCH_LIFECYCLE"] != string(lifecyclePreApply) {
		t.Fatalf("expected ORCH_LIFECYCLE, got %q", hookCommand.Env["ORCH_LIFECYCLE"])
	}
	if hookCommand.Env["ORCH_WORKDIR"] != "/tmp/orch/test-env/api" {
		t.Fatalf("expected ORCH_WORKDIR, got %q", hookCommand.Env["ORCH_WORKDIR"])
	}
	if hookCommand.Env["BASE"] != "present" {
		t.Fatalf("expected base env to be preserved")
	}
}

func TestRunLifecycleHooksFailsOnMissingInterpolation(t *testing.T) {
	runner := &fakeHookRunner{}
	err := runLifecycleHooks(context.Background(), runner, []manifestcore.Hook{
		{Command: `echo "${database.outputs.url}"`},
	}, lifecyclePreApply, hookExecutionContext{
		envID:     "test-env",
		component: "api",
		runner:    "local",
		resolver:  varresolvers.NewComponentResolver(),
	})
	if err == nil {
		t.Fatal("expected missing interpolation to fail")
	}
}
