package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"orch.io/pkg/logging"
	manifestcore "orch.io/pkg/manifest/core"
	"orch.io/pkg/state"
	statebackends "orch.io/pkg/state/backends"
)

type StateInspectOptions struct {
	Output string
	Writer io.Writer
}

func RunStateInspect(envID string, m *manifestcore.Manifest, logger logging.Logger, options StateInspectOptions) error {
	writer := options.Writer
	if writer == nil {
		writer = io.Discard
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

	switch options.Output {
	case "", "table":
		return renderStateInspectTable(writer, currentState)
	case "json":
		return renderStateInspectJSON(writer, currentState)
	default:
		return fmt.Errorf("unsupported output %q; supported outputs: table, json", options.Output)
	}
}

func renderStateInspectJSON(writer io.Writer, currentState *state.OrchState) error {
	data, err := json.MarshalIndent(currentState, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	_, err = fmt.Fprintln(writer, string(data))
	return err
}

func renderStateInspectTable(writer io.Writer, currentState *state.OrchState) error {
	fmt.Fprintf(writer, "Environment: %s\n", currentState.EnvID)
	fmt.Fprintf(writer, "Manifest:    %s\n", currentState.ManifestID)
	fmt.Fprintf(writer, "Updated:     %s\n\n", currentState.UpdatedAt)

	tw := tabwriter.NewWriter(writer, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "COMPONENT\tTYPE\tRUNNER\tSTATUS\tSTAGE\tUPDATED")
	for _, component := range currentState.Components {
		fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			component.Name,
			component.Type,
			component.Runner.Name,
			component.Status,
			component.Stage,
			component.UpdatedAt,
		)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	hints := recoveryHints(currentState.Components)
	if len(hints) == 0 {
		return nil
	}

	fmt.Fprintln(writer, "\nRecovery:")
	for _, hint := range hints {
		fmt.Fprintf(writer, "- %s\n", hint)
	}
	return nil
}

func recoveryHints(components []state.ComponentState) []string {
	hints := make([]string, 0)
	for _, component := range components {
		switch component.Status {
		case state.StatusDestroying:
			hints = append(hints, fmt.Sprintf("%s was interrupted during destroy; run `orch down -e <env-id>` to retry cleanup.", component.Name))
		case state.StatusFailed:
			if component.Stage.IsDestroyStage() {
				hints = append(hints, fmt.Sprintf("%s failed during %s; run `orch down -e <env-id>` to retry cleanup.", component.Name, component.Stage))
			} else {
				hints = append(hints, fmt.Sprintf("%s failed during %s; run `orch up -e <env-id>` to retry or `orch down -e <env-id>` to clean up.", component.Name, component.Stage))
			}
		case state.StatusApplying:
			hints = append(hints, fmt.Sprintf("%s was interrupted during %s; run `orch up -e <env-id>` to retry or `orch down -e <env-id>` to clean up.", component.Name, component.Stage))
		}
	}
	return hints
}
