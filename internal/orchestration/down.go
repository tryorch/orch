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

func RunDown(envID string, m *manifestcore.Manifest, logger logging.Logger, inputs map[string]string) error {
	fmt.Printf("Tearing down sandbox: %s\n", envID)

	inputsResolver, err := varresolvers.NewInputsResolver(inputs, m.Inputs)
	if err != nil {
		return fmt.Errorf("failed to resolve inputs: %w", err)
	}

	stateBackend, err := statebackends.FromManifestContext(context.Background(), m.State, logger.AsDebugLogger())
	if err != nil {
		return fmt.Errorf("failed to configure state backend: %w", err)
	}
	stateManager := state.NewManager(envID, stateBackend)
	currentState, err := stateManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	componentsByName := make(map[string]*manifestcore.Component, len(m.Components))
	for i := range m.Components {
		componentsByName[m.Components[i].Name] = &m.Components[i]
	}

	componentResolver := varresolvers.NewComponentResolver()
	for _, componentState := range currentState.Components {
		componentResolver.RegisterPersistedComponentOutput(componentState.Name, componentState.Outputs)
		if component, ok := componentsByName[componentState.Name]; ok {
			componentResolver.RegisterUnavailableSensitiveOutputs(componentState.Name, component.Outputs, componentState.Outputs)
		}
	}
	resolvers := &varresolvers.ChainResolver{
		Resolvers: []varresolvers.Resolver{
			inputsResolver,
			varresolvers.NewEnvResolver(),
			componentResolver,
		},
	}

	commandResolvers := shellCommandResolver(inputsResolver, componentResolver)

	emitter := events.NewRendererEmitter()
	debugLogger := logger.AsDebugLogger()
	adapterCtx := adapters.NewAdapterContext(envID, debugLogger, emitter)
	ctx := adapters.WithAdapterContext(context.Background(), adapterCtx)

	runnerManifests := make(map[string]manifestcore.RunnerManifest)
	for _, componentState := range currentState.Components {
		if componentState.Status == state.StatusDestroyed {
			continue
		}

		if _, ok := runnerManifests[componentState.Runner.Name]; ok {
			continue
		}

		runnerManifest, ok := m.Runners[componentState.Runner.Name]
		if !ok {
			return fmt.Errorf("component %q references unknown runner %q",
				componentState.Name, componentState.Runner.Name)
		}

		cfg, err := varresolvers.DeepInterpolate(context.Background(), runnerManifest.Config, resolvers)
		if err != nil {
			return fmt.Errorf("failed to interpolate runner %q config: %w", componentState.Runner.Name, err)
		}
		runnerManifest.Config = cfg
		runnerManifests[componentState.Runner.Name] = runnerManifest
	}

	allRunners, err := runners.FromManifestRunnersMap(runnerManifests)
	if err != nil {
		return fmt.Errorf("failed to create runners from manifest: %w", err)
	}
	defer disconnectAllRunners(allRunners, emitter)

	for _, componentState := range currentState.Components {
		if componentState.Status == state.StatusDestroyed {
			continue
		}

		t, ok := allRunners[componentState.Runner.Name]
		if !ok {
			return fmt.Errorf("component %q references unknown runner %q",
				componentState.Name, componentState.Runner.Name)
		}
		if t.Type() != componentState.Runner.Type {
			return fmt.Errorf("component %q was applied with runner %q of type %q, but current manifest defines it as %q",
				componentState.Name, componentState.Runner.Name, componentState.Runner.Type, t.Type())
		}

		if yes, list := t.UsesNonAmbientCredentials(); yes {
			emitter.Emit(events.Event{
				Type: events.EventWarning,
				Message: fmt.Sprintf(
					"Runner uses non-ambient credentials (%v). Down will use the current manifest runner config for teardown.",
					strings.Join(list, ", "),
				),
				Hint:      "Prefer ambient authentication for future state-only teardown.",
				Runner:    t.Name(),
				Component: componentState.Name,
				Adapter:   componentState.Type,
			})
		}

		if err := validateDestroyHooksForRunner(componentState.Name, componentState.DestroyHooks, t); err != nil {
			return err
		}
	}

	for i := len(currentState.Components) - 1; i >= 0; i-- {
		componentState := currentState.Components[i]
		if componentState.Status == state.StatusDestroyed {
			continue
		}

		t := allRunners[componentState.Runner.Name]

		fmt.Printf("→ Destroying component: %s\n", componentState.Name)
		adapter, err := adapters.Get(componentState.Type)
		if err != nil {
			return err
		}

		if err := stateManager.RestoreArtifacts(ctx, componentState, t); err != nil {
			return fmt.Errorf("component %s artifact restore failed: %w", componentState.Name, err)
		}

		runtimeComponent, err := destroyRuntimeComponent(context.Background(), envID, componentState, componentsByName[componentState.Name], t.Name(), resolvers)
		if err != nil {
			return err
		}
		componentState.Env = runtimeComponent.Env

		if len(componentState.DestroyHooks.PreDestroy) > 0 {
			currentState.MarkComponentDestroying(componentState.Name, state.StagePreDestroy)
			if err := stateManager.Save(currentState); err != nil {
				return fmt.Errorf("failed to save pre_destroy state for component %q: %w", componentState.Name, err)
			}
			if err := runLifecycleHooks(ctx, t, componentState.DestroyHooks.PreDestroy, lifecyclePreDestroy, hookExecutionContext{
				envID:           envID,
				componentRef:    runtimeComponent,
				component:       componentState.Name,
				runner:          t.Name(),
				workDir:         componentState.WorkDir,
				baseEnv:         runtimeComponent.Env,
				resolver:        resolvers,
				commandResolver: commandResolvers,
				emitter:         emitter,
			}); err != nil {
				currentState.MarkComponentFailed(componentState.Name, state.StagePreDestroy)
				if saveErr := stateManager.Save(currentState); saveErr != nil {
					return fmt.Errorf("component %q pre_destroy hook failed: %w (also failed to save failed state: %v)", componentState.Name, err, saveErr)
				}
				return fmt.Errorf("component %q pre_destroy hook failed: %w", componentState.Name, err)
			}
		}

		currentState.MarkComponentDestroying(componentState.Name, state.StageDestroy)
		if err := stateManager.Save(currentState); err != nil {
			return fmt.Errorf("failed to save destroying state for component %q: %w", componentState.Name, err)
		}

		if err := adapter.Destroy(ctx, componentState, t); err != nil {
			currentState.MarkComponentFailed(componentState.Name, state.StageDestroy)
			emitter.Emit(events.Event{
				Type:      events.EventFailure,
				Message:   "destroy failed",
				Adapter:   componentState.Type,
				Runner:    componentState.Runner.Name,
				Component: componentState.Name,
				Stage:     string(state.StageDestroy),
				Err:       err,
			})
			if saveErr := stateManager.Save(currentState); saveErr != nil {
				return fmt.Errorf("component %s destroy failed: %w (also failed to save failed state: %v)", componentState.Name, err, saveErr)
			}
			return fmt.Errorf("component %s destroy failed: %w", componentState.Name, err)
		}

		currentState.MarkComponentDestroyed(componentState.Name, state.StageDestroy)
		if err := stateManager.Save(currentState); err != nil {
			return fmt.Errorf("failed to save state after destroying component %q: %w", componentState.Name, err)
		}

		if len(componentState.DestroyHooks.PostDestroy) > 0 {
			currentState.MarkComponentDestroying(componentState.Name, state.StagePostDestroy)
			if err := stateManager.Save(currentState); err != nil {
				return fmt.Errorf("failed to save post_destroy state for component %q: %w", componentState.Name, err)
			}
			if err := runLifecycleHooks(ctx, t, componentState.DestroyHooks.PostDestroy, lifecyclePostDestroy, hookExecutionContext{
				envID:           envID,
				componentRef:    runtimeComponent,
				component:       componentState.Name,
				runner:          t.Name(),
				workDir:         componentState.WorkDir,
				baseEnv:         runtimeComponent.Env,
				resolver:        resolvers,
				commandResolver: commandResolvers,
				emitter:         emitter,
			}); err != nil {
				currentState.MarkComponentFailed(componentState.Name, state.StagePostDestroy)
				if saveErr := stateManager.Save(currentState); saveErr != nil {
					return fmt.Errorf("component %q post_destroy hook failed: %w (also failed to save failed state: %v)", componentState.Name, err, saveErr)
				}
				return fmt.Errorf("component %q post_destroy hook failed: %w", componentState.Name, err)
			}
			currentState.MarkComponentDestroyed(componentState.Name, state.StagePostDestroy)
			if err := stateManager.Save(currentState); err != nil {
				return fmt.Errorf("failed to save post_destroy completion state for component %q: %w", componentState.Name, err)
			}
		}
	}

	if err := stateManager.Delete(); err != nil {
		return fmt.Errorf("failed to delete state after teardown: %w", err)
	}

	fmt.Printf("Sandbox torn down\n")
	return nil
}

func disconnectAllRunners(allRunners map[string]runners.Runner, emitter events.Emitter) {
	for _, t := range allRunners {
		if err := t.Disconnect(); err != nil {
			emitter.Emit(events.Event{
				Type:    events.EventWarning,
				Message: fmt.Sprintf("failed to disconnect from runner %q: %v", t.Name(), err),
				Runner:  t.Name(),
			})
		}
	}
}

func destroyRuntimeComponent(
	ctx context.Context,
	envID string,
	componentState state.ComponentState,
	manifestComponent *manifestcore.Component,
	runnerName string,
	resolver varresolvers.Resolver,
) (*manifestcore.Component, error) {
	component := &manifestcore.Component{
		Name:   componentState.Name,
		Type:   componentState.Type,
		Runner: componentState.Runner.Name,
		Source: componentState.Source,
		Env:    map[string]string{},
	}
	if manifestComponent != nil {
		component.Env = manifestComponent.Env
	}

	resolvedEnv, err := interpolateEnv(ctx, component.Env, resolver)
	if err != nil {
		return nil, fmt.Errorf("component %q env interpolation failed during destroy: %w", componentState.Name, err)
	}
	component.Env = componentExecutionEnv(envID, component, runnerName, componentState.WorkDir, resolvedEnv)
	return component, nil
}
