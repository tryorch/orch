package orchestration

import (
	"context"
	"fmt"
	"os"
	"time"

	"orch.io/pkg/events"
	manifestcore "orch.io/pkg/manifest/core"
	"orch.io/pkg/runners"
	"orch.io/pkg/state"
	"orch.io/pkg/utils"
	"orch.io/pkg/varresolvers"
)

type lifecycleHookPhase string

const (
	lifecyclePreApply    lifecycleHookPhase = "pre_apply"
	lifecyclePostApply   lifecycleHookPhase = "post_apply"
	lifecyclePreDestroy  lifecycleHookPhase = "pre_destroy"
	lifecyclePostDestroy lifecycleHookPhase = "post_destroy"
)

type hookExecutionContext struct {
	envID           string
	componentRef    *manifestcore.Component
	component       string
	runner          string
	workDir         string
	baseEnv         map[string]string
	resolver        varresolvers.Resolver
	commandResolver varresolvers.Resolver
	emitter         events.Emitter
}

func runLifecycleHooks(ctx context.Context, t runners.Runner, hooks []manifestcore.Hook, phase lifecycleHookPhase, hookCtx hookExecutionContext) error {
	if len(hooks) == 0 {
		return nil
	}
	if !t.Capabilities().Exec {
		return fmt.Errorf("runner %q does not support Exec required by %s hooks", t.Name(), phase)
	}
	if hookCtx.componentRef == nil {
		return fmt.Errorf("component reference is required to execute %s hooks for component %q", phase, hookCtx.component)
	}
	if err := ensureLifecycleHookWorkDir(ctx, t, hookCtx); err != nil {
		return err
	}

	for i, hook := range hooks {
		if hook.Command == "" {
			return fmt.Errorf("%s hook %d for component %q has an empty command", phase, i+1, hookCtx.component)
		}
		emitHookEvent(hookCtx, events.EventStart, phase, i+1, "hook started", nil, 0)

		commandResolver := hookCtx.commandResolver
		if commandResolver == nil {
			commandResolver = hookCtx.resolver
		}
		command, err := varresolvers.InterpolateString(ctx, hook.Command, commandResolver)
		if err != nil {
			emitHookEvent(hookCtx, events.EventFailure, phase, i+1, "hook interpolation failed", err, 0)
			return fmt.Errorf("failed to interpolate %s hook %d for component %q: %w", phase, i+1, hookCtx.component, err)
		}

		env, err := lifecycleHookEnv(ctx, hookCtx, phase, hook.Env)
		if err != nil {
			emitHookEvent(hookCtx, events.EventFailure, phase, i+1, "hook env interpolation failed", err, 0)
			return fmt.Errorf("failed to interpolate env for %s hook %d on component %q: %w", phase, i+1, hookCtx.component, err)
		}

		res, err := t.Exec(ctx, runners.ExecCommand{
			WorkingDir: hookCtx.workDir,
			Command:    utils.ShellCommand(hook.Shell, command),
			Env:        env,
			Timeout:    0,
			Stderr:     utils.NewPrefixWriter(os.Stderr, utils.RunnerComponentPrefix(hookCtx.runner, fmt.Sprintf("%s.%s", hookCtx.component, phase))),
			Stdout:     utils.NewPrefixWriter(os.Stdout, utils.RunnerComponentPrefix(hookCtx.runner, fmt.Sprintf("%s.%s", hookCtx.component, phase))),
		})
		if err != nil {
			emitHookEvent(hookCtx, events.EventFailure, phase, i+1, "hook failed", err, 0)
			return fmt.Errorf("failed to execute %s hook %d for component %q: %w", phase, i+1, hookCtx.component, err)
		}
		if res.Error != nil || res.ExitCode != 0 {
			hookErr := res.Error
			if hookErr == nil {
				hookErr = fmt.Errorf("exit code %d", res.ExitCode)
			}
			emitHookEvent(hookCtx, events.EventFailure, phase, i+1, "hook failed", hookErr, res.Duration)
			return fmt.Errorf("%s hook %d for component %q failed with exit code %d: %v", phase, i+1, hookCtx.component, res.ExitCode, res.Error)
		}
		emitHookEvent(hookCtx, events.EventSuccess, phase, i+1, "hook completed", nil, res.Duration)
	}

	return nil
}

func emitHookEvent(hookCtx hookExecutionContext, eventType events.Type, phase lifecycleHookPhase, index int, message string, err error, duration time.Duration) {
	if hookCtx.emitter == nil {
		return
	}
	hookCtx.emitter.Emit(events.Event{
		Type:      eventType,
		Component: hookCtx.component,
		Adapter:   "hook",
		Runner:    hookCtx.runner,
		Stage:     string(phase),
		Message:   fmt.Sprintf("%s %d %s", phase, index, message),
		Err:       err,
		Duration:  duration,
	})
}

func ensureLifecycleHookWorkDir(ctx context.Context, t runners.Runner, hookCtx hookExecutionContext) error {
	if hookCtx.workDir == "" {
		return nil
	}

	res, err := t.Exec(ctx, runners.ExecCommand{
		Command: []string{"mkdir", "-p", hookCtx.workDir},
		Timeout: 0,
		Stderr:  utils.NewPrefixWriter(os.Stderr, utils.RunnerComponentPrefix(hookCtx.runner, hookCtx.component)),
		Stdout:  utils.NewPrefixWriter(os.Stdout, utils.RunnerComponentPrefix(hookCtx.runner, hookCtx.component)),
	})
	if err != nil {
		return fmt.Errorf("failed to create hook workdir %q for component %q: %w", hookCtx.workDir, hookCtx.component, err)
	}
	if res.Error != nil || res.ExitCode != 0 {
		return fmt.Errorf("failed to create hook workdir %q for component %q with exit code %d: %v", hookCtx.workDir, hookCtx.component, res.ExitCode, res.Error)
	}
	return nil
}

func lifecycleHookEnv(ctx context.Context, hookCtx hookExecutionContext, phase lifecycleHookPhase, hookEnv map[string]string) (map[string]string, error) {
	env, err := interpolateEnv(ctx, hookCtx.baseEnv, hookCtx.resolver)
	if err != nil {
		return nil, err
	}

	for key, value := range hookEnv {
		resolved, err := varresolvers.InterpolateString(ctx, value, hookCtx.resolver)
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", key, err)
		}
		env[key] = resolved
	}

	env = componentExecutionEnv(hookCtx.envID, hookCtx.componentRef, hookCtx.runner, hookCtx.workDir, env)
	env["ORCH_LIFECYCLE"] = string(phase)

	return env, nil
}

func validateLifecycleHooksForRunner(componentName string, hooks manifestcore.Hooks, runner runners.Runner) error {
	if hooks.HasAny() && !runner.Capabilities().Exec {
		return fmt.Errorf("component %q defines lifecycle hooks, but runner %q does not support Exec", componentName, runner.Name())
	}
	return nil
}

func validateDestroyHooksForRunner(componentName string, hooks state.DestroyHooks, runner runners.Runner) error {
	if (len(hooks.PreDestroy) > 0 || len(hooks.PostDestroy) > 0) && !runner.Capabilities().Exec {
		return fmt.Errorf("component %q defines lifecycle hooks, but runner %q does not support Exec", componentName, runner.Name())
	}
	return nil
}
