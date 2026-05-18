package orchestration

import (
	"context"
	"fmt"

	"orch.io/internal/adapters"
	"orch.io/pkg/events"
	"orch.io/pkg/logging"
	manifestcore "orch.io/pkg/manifest/core"
	"orch.io/pkg/runners"
	"orch.io/pkg/state"
	statebackends "orch.io/pkg/state/backends"
	"orch.io/pkg/varresolvers"
)

func RunDown(envID string, m *manifestcore.Manifest, logger logging.Logger) error {
	fmt.Printf("Tearing down sandbox: %s\n", envID)

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
			varresolvers.NewEnvResolver(),
			componentResolver,
		},
	}

	emitter := events.NewRendererEmitter()
	debugLogger := logger.AsDebugLogger()
	adapterCtx := adapters.NewAdapterContext(envID, debugLogger, emitter)
	ctx := adapters.WithAdapterContext(context.Background(), adapterCtx)
	allRunners, err := runners.FromManifestRunnersMap(m.Runners)
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

		if yes, list := t.UsesNonAmbientCredentials(); yes {
			return fmt.Errorf("runner %q uses non-ambient credentials (%v); destroy only supports ambient auth",
				t.Name(), list)
		}

		if component, ok := componentsByName[componentState.Name]; ok {
			if err := validateLifecycleHooksForRunner(component.Name, component.Hooks, t); err != nil {
				return err
			}
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

		if component, ok := componentsByName[componentState.Name]; ok {
			currentState.MarkComponentDestroying(componentState.Name, state.StagePreDestroy)
			if err := stateManager.Save(currentState); err != nil {
				return fmt.Errorf("failed to save pre_destroy state for component %q: %w", componentState.Name, err)
			}
			if err := runLifecycleHooks(ctx, t, component.Hooks.PreDestroy, lifecyclePreDestroy, hookExecutionContext{
				envID:        envID,
				componentRef: component,
				component:    componentState.Name,
				runner:       t.Name(),
				workDir:      componentState.WorkDir,
				baseEnv:      component.Env,
				resolver:     resolvers,
				emitter:      emitter,
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

		if err := adapter.DestroyFromState(ctx, componentState, t); err != nil {
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

		if component, ok := componentsByName[componentState.Name]; ok {
			currentState.MarkComponentDestroying(componentState.Name, state.StagePostDestroy)
			if err := stateManager.Save(currentState); err != nil {
				return fmt.Errorf("failed to save post_destroy state for component %q: %w", componentState.Name, err)
			}
			if err := runLifecycleHooks(ctx, t, component.Hooks.PostDestroy, lifecyclePostDestroy, hookExecutionContext{
				envID:        envID,
				componentRef: component,
				component:    componentState.Name,
				runner:       t.Name(),
				workDir:      componentState.WorkDir,
				baseEnv:      component.Env,
				resolver:     resolvers,
				emitter:      emitter,
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
