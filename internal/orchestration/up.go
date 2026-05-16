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
	adapterCtx := adapters.NewAdapterContext(envID, logger.AsDebugLogger(), emitter)
	ctx := adapters.WithAdapterContext(context.Background(), adapterCtx)
	stateBackend, err := statebackends.FromManifestContext(context.Background(), m.State, logger.AsDebugLogger())
	if err != nil {
		return fmt.Errorf("failed to configure state backend: %w", err)
	}
	stateManager := state.NewManager(envID, stateBackend)
	currentState, err := stateManager.LoadOrNew(m.Metadata.ID)
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

		if _, err := c.Source.Validate(); err != nil {
			return fmt.Errorf("component \"%s\" has an invalid source configuration", c.Name)
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

		if !adapter.SupportedSources().SatisfiedBy(c.Source) {
			return fmt.Errorf("component \"%s\" source type \"%s\" is not supported by adapter \"%s\". Supported source types are: %s",
				c.Name, c.Source.Type(), c.Type, adapter.SupportedSources().String())
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

		emitter.Emit(events.Event{
			Type:      events.EventStart,
			Message:   fmt.Sprintf("starting apply for component"),
			Adapter:   component.Type,
			Runner:    component.Runner,
			Component: component.Name,
		})

		preApplyWorkDir := adapterCtx.BuildRunnerWorkDir(component.WorkDir, component.Name)

		resolvedConfig, err := varresolvers.DeepInterpolate(ctx, component.Config, resolvers)
		if err != nil {
			return fmt.Errorf("component %q config interpolation failed: %w", component.Name, err)
		}
		component.Config = resolvedConfig

		resolvedEnv, err := interpolateEnv(ctx, component.Env, resolvers)
		if err != nil {
			return fmt.Errorf("component %q env interpolation failed: %w", component.Name, err)
		}
		component.Env = componentExecutionEnv(envID, component, runner.Name(), resolvedEnv)

		cfg, warnings, err := adapter.ValidateAndLoadConfig(ctx, component)
		if err != nil {
			return fmt.Errorf("component \"%s\" config validation failed: %w", component.Name, err)
		}

		for _, warning := range warnings {
			emitter.Emit(warning)
		}

		component.LoadedConfig = cfg

		if err := runLifecycleHooks(ctx, runner, component.Hooks.PreApply, lifecyclePreApply, hookExecutionContext{
			envID:        envID,
			componentRef: component,
			component:    component.Name,
			runner:       runner.Name(),
			workDir:      preApplyWorkDir,
			baseEnv:      component.Env,
			resolver:     resolvers,
		}); err != nil {
			return fmt.Errorf("component %q pre_apply hook failed: %w", component.Name, err)
		}

		applyResult, err := adapter.Apply(ctx, component, runner)
		if err != nil {
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
		componentResolver.RegisterComponentOutput(component.Name, component.Outputs, applyResult.Outputs)

		componentState := state.NewComponentState(component, string(runner.Type()), applyResult.Outputs, applyResult.State)
		currentState.UpsertComponent(componentState)
		if err := stateManager.CaptureArtifacts(ctx, componentState, runner); err != nil {
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
			return fmt.Errorf("component %q post_apply hook failed: %w", component.Name, err)
		}
	}

	// Disconnect all runners
	for _, t := range allRunners {
		if err := t.Disconnect(); err != nil {
			emitter.Emit(events.Event{
				Type:    events.EventWarning,
				Message: fmt.Sprintf("failed to disconnect from runner \"%s\": %v", t.Name(), err),
				Runner:  t.Name(),
			})
		}
	}

	fmt.Printf("Sandbox created successfully\n")
	return nil
}
