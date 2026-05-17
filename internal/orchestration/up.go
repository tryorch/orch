package orchestration

import (
	"context"
	"fmt"
	"strings"

	"orch.io/internal/adapters"
	"orch.io/pkg/events"
	"orch.io/pkg/logging"
	manifestcore "orch.io/pkg/manifest/core"
	"orch.io/pkg/runners"
	"orch.io/pkg/state"
	statebackends "orch.io/pkg/state/backends"
	"orch.io/pkg/varresolvers"
)

type job struct {
	c *manifestcore.Component
	r *runners.Runner
	a *adapters.Adapter
}

func RunUp(envID string, m *manifestcore.Manifest, logger logging.Logger, inputs map[string]string) error {
	componentResolver := varresolvers.NewComponentResolver()
	inputsResolver, err := varresolvers.NewInputsResolver(inputs, m.Inputs)
	if err != nil {
		return fmt.Errorf("failed to resolve inputs: %w", err)
	}

	resolvers := &varresolvers.ChainResolver{
		Resolvers: []varresolvers.Resolver{
			inputsResolver,
			varresolvers.NewEnvResolver(),
			componentResolver,
		},
	}

	emitter := events.NewRendererEmitter()
	debugLogger := logger.AsDebugLogger()
	adapterCtx := adapters.NewAdapterContext(envID, debugLogger, emitter)
	ctx := adapters.WithAdapterContext(context.Background(), adapterCtx)
	stateBackend, err := statebackends.FromManifestContext(context.Background(), m.State, debugLogger)
	if err != nil {
		return fmt.Errorf("failed to configure state backend: %w", err)
	}
	stateManager := state.NewManager(envID, stateBackend)
	currentState, err := stateManager.LoadOrNew(m.Metadata.ID, debugLogger)
	if err != nil {
		return fmt.Errorf("failed to initialize state: %w", err)
	}

	for key, value := range m.Runners {
		cfg, err := varresolvers.DeepInterpolate(ctx, value.Config, resolvers)
		if err != nil {
			return fmt.Errorf("failed to interpolate runner \"%s\" config: %w", key, err)
		}
		value.Config = cfg
		m.Runners[key] = value
	}

	allRunners, err := runners.FromManifestRunnersMap(m.Runners)
	if err != nil {
		return err
	}
	defer disconnectAllRunners(allRunners, emitter)

	componentsInOrder, err := TopologicallySortComponents(m.Components)
	if err != nil {
		return fmt.Errorf("failed to establish logical apply order: %w", err)
	}

	// Registry of all jobs to be executed, with already validated configs, runners, and adapters
	var jobs []job
	for _, c := range componentsInOrder {
		t, ok := allRunners[c.Runner]
		if !ok {
			return fmt.Errorf("component \"%s\" references an unknown runner \"%s\"",
				c.Name, c.Runner)
		}

		if err := validateOutputDeclarations(c); err != nil {
			return err
		}

		if yes, list := t.UsesNonAmbientCredentials(); yes {
			emitter.Emit(events.Event{
				Type: events.EventWarning,
				Message: fmt.Sprintf(
					"Runner uses non-ambient credentials (%v). This component cannot be reliably torn down by Orch.",
					strings.Join(list, ", "),
				),
				Hint:      "Use ambient authentication for the runner to enable safe teardown of this component. Learn more at https://orch.io/docs/guides/authentication",
				Runner:    t.Name(),
				Component: c.Name,
				Adapter:   c.Type,
			})
		}

		adapter, err := adapters.Get(c.Type)
		if err != nil {
			return err
		}

		if err := validateComponentSource(c, adapter); err != nil {
			return err
		}

		if !adapter.RequiredCapabilities().SatisfiedBy(t.Capabilities()) {
			return fmt.Errorf("component \"%s\" requires capabilities %v which are not satisfied by runner \"%s\" capabilities %v",
				c.Name, adapter.RequiredCapabilities(), t.Name(), t.Capabilities())
		}
		if err := validateLifecycleHooksForRunner(c.Name, c.Hooks, t); err != nil {
			return err
		}

		jobs = append(jobs, job{
			c: c,
			r: &t,
			a: &adapter,
		})
	}

	// Execute all jobs
	for _, j := range jobs {
		component := j.c
		adapter := *j.a
		runner := *j.r

		if componentState, ok := currentState.FindComponent(component.Name); ok {
			switch componentState.Status {
			case state.StatusApplied:
				componentResolver.RegisterPersistedComponentOutput(component.Name, componentState.Outputs)
				componentResolver.RegisterUnavailableSensitiveOutputs(component.Name, component.Outputs, componentState.Outputs)
				emitter.Emit(events.Event{
					Type:      events.EventInfo,
					Message:   "component already applied; skipping",
					Adapter:   component.Type,
					Runner:    component.Runner,
					Component: component.Name,
				})
				continue
			case state.StatusApplying:
				emitter.Emit(events.Event{
					Type:      events.EventWarning,
					Message:   "component was applying in a previous run; retrying",
					Adapter:   component.Type,
					Runner:    component.Runner,
					Component: component.Name,
				})
			case state.StatusFailed:
				emitter.Emit(events.Event{
					Type:      events.EventInfo,
					Message:   "component failed in a previous run; retrying",
					Adapter:   component.Type,
					Runner:    component.Runner,
					Component: component.Name,
				})
			case state.StatusDestroying:
				return fmt.Errorf("component %q was destroying in a previous run; run down again to finish cleanup before applying", component.Name)
			}
		}

		emitter.Emit(events.Event{
			Type:      events.EventStart,
			Message:   fmt.Sprintf("starting apply for component"),
			Adapter:   component.Type,
			Runner:    component.Runner,
			Component: component.Name,
		})

		runnerWorkDir := adapterCtx.BuildRunnerWorkDir(component.WorkDir, component.Name)
		currentState.BeginComponentApply(component, runner.Type(), runnerWorkDir)
		if err = stateManager.Save(currentState); err != nil {
			return fmt.Errorf("component initial status registration failed: %w", err)
		}

		resolvedConfig, err := varresolvers.DeepInterpolate(ctx, component.Config, resolvers)
		if err != nil {
			currentState.MarkComponentFailed(component.Name)
			_ = stateManager.Save(currentState)
			return fmt.Errorf("component %q config interpolation failed: %w", component.Name, err)
		}
		component.Config = resolvedConfig

		resolvedEnv, err := interpolateEnv(ctx, component.Env, resolvers)
		if err != nil {
			currentState.MarkComponentFailed(component.Name)
			_ = stateManager.Save(currentState)
			return fmt.Errorf("component %q env interpolation failed: %w", component.Name, err)
		}
		component.Env = componentExecutionEnv(envID, component, runner.Name(), resolvedEnv)

		cfg, warnings, err := adapter.ValidateAndLoadConfig(ctx, component)
		if err != nil {
			currentState.MarkComponentFailed(component.Name)
			_ = stateManager.Save(currentState)
			return fmt.Errorf("component \"%s\" config validation failed: %w", component.Name, err)
		}

		for _, warning := range warnings {
			emitter.Emit(warning)
		}

		component.LoadedConfig = cfg
		currentState.BeginComponentApply(component, runner.Type(), runnerWorkDir)
		if err := stateManager.Save(currentState); err != nil {
			return fmt.Errorf("failed to save applying state for component %q: %w", component.Name, err)
		}

		if err := runLifecycleHooks(ctx, runner, component.Hooks.PreApply, lifecyclePreApply, hookExecutionContext{
			envID:        envID,
			componentRef: component,
			component:    component.Name,
			runner:       runner.Name(),
			workDir:      runnerWorkDir,
			baseEnv:      component.Env,
			resolver:     resolvers,
		}); err != nil {
			currentState.MarkComponentFailed(component.Name)
			if saveErr := stateManager.Save(currentState); saveErr != nil {
				return fmt.Errorf("component %q pre_apply hook failed: %w (also failed to save failed state: %v)", component.Name, err, saveErr)
			}
			return fmt.Errorf("component %q pre_apply hook failed: %w", component.Name, err)
		}

		applyResult, err := adapter.Apply(ctx, component, runner)
		if err != nil {
			currentState.MarkComponentFailed(component.Name)
			if saveErr := stateManager.Save(currentState); saveErr != nil {
				return fmt.Errorf("component %q failed to apply: %w (also failed to save failed state: %v)", component.Name, err, saveErr)
			}
			emitter.Emit(events.Event{
				Type:      events.EventFailure,
				Message:   fmt.Sprintf("failed to apply component"),
				Adapter:   component.Type,
				Runner:    component.Runner,
				Component: component.Name,
				Err:       err,
			})

			return fmt.Errorf("component \"%s\" failed to apply", component.Name)
		}
		if err := validateApplyOutputs(component, applyResult.Outputs, emitter); err != nil {
			failedState := state.NewComponentState(component, runner.Type(), map[string]string{}, applyResult.State)
			failedState.Status = state.StatusFailed
			currentState.UpsertComponent(failedState)
			if artifactErr := stateManager.CaptureArtifacts(ctx, failedState, runner); artifactErr != nil {
				if saveErr := stateManager.Save(currentState); saveErr != nil {
					return fmt.Errorf("component %q output validation failed: %w (also failed to capture artifacts: %v and failed to save failed state: %v)", component.Name, err, artifactErr, saveErr)
				}
				return fmt.Errorf("component %q output validation failed: %w (also failed to capture artifacts: %v)", component.Name, err, artifactErr)
			}
			if saveErr := stateManager.Save(currentState); saveErr != nil {
				return fmt.Errorf("component %q output validation failed: %w (also failed to save failed state: %v)", component.Name, err, saveErr)
			}
			return err
		}
		declaredOutputs := filterDeclaredOutputs(component, applyResult.Outputs)
		stateOutputs := filterStateOutputs(component, declaredOutputs)
		componentResolver.RegisterComponentOutput(component.Name, component.Outputs, declaredOutputs)

		componentState := state.NewComponentState(component, runner.Type(), stateOutputs, applyResult.State)
		currentState.UpsertComponent(componentState)
		if err := stateManager.CaptureArtifacts(ctx, componentState, runner); err != nil {
			currentState.MarkComponentFailed(component.Name)
			if saveErr := stateManager.Save(currentState); saveErr != nil {
				return fmt.Errorf("failed to capture state artifacts for component %q: %w (also failed to save failed state: %v)", component.Name, err, saveErr)
			}
			return fmt.Errorf("failed to capture state artifacts for component %q: %w", component.Name, err)
		}
		if err := stateManager.Save(currentState); err != nil {
			return fmt.Errorf("failed to save state for component %q: %w", component.Name, err)
		}

		if err := runLifecycleHooks(ctx, runner, component.Hooks.PostApply, lifecyclePostApply, hookExecutionContext{
			envID:        envID,
			componentRef: component,
			component:    component.Name,
			runner:       runner.Name(),
			workDir:      applyResult.State.WorkDir,
			baseEnv:      component.Env,
			resolver:     resolvers,
		}); err != nil {
			currentState.MarkComponentFailed(component.Name)
			if saveErr := stateManager.Save(currentState); saveErr != nil {
				return fmt.Errorf("component %q post_apply hook failed: %w (also failed to save failed state: %v)", component.Name, err, saveErr)
			}
			return fmt.Errorf("component %q post_apply hook failed: %w", component.Name, err)
		}
	}

	fmt.Printf("Sandbox created successfully\n")
	return nil
}
