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
)

func RunDown(envID string, m *manifestcore.Manifest, logger logging.Logger) error {
	fmt.Printf("Tearing down sandbox: %s\n", envID)

	stateManager := state.NewStateManager(envID)
	currentState, err := stateManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	emitter := events.NewRendererEmitter()
	ctx := adapters.NewAdapterContext(context.Background(), envID, logger.AsDebugLogger(), emitter)
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

		if err := adapter.DestroyFromState(ctx, componentState, t); err != nil {
			return fmt.Errorf("component %s destroy failed: %w", componentState.Name, err)
		}

		currentState.MarkComponentDestroyed(componentState.Name)
		if err := stateManager.Save(currentState); err != nil {
			return fmt.Errorf("failed to save state after destroying component %q: %w", componentState.Name, err)
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
