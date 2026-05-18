package orchestration

import (
	"fmt"

	"orch.io/pkg/state"
)

func upActionForExistingComponent(componentState state.ComponentState, options UpOptions) (existingComponentAction, error) {
	switch componentState.Status {
	case state.StatusApplied:
		if options.Reapply {
			return existingComponentApply, nil
		}
		return existingComponentSkip, nil
	case state.StatusDestroying:
		return "", fmt.Errorf("component %q was destroying in a previous run; run down again to finish cleanup before applying", componentState.Name)
	case state.StatusFailed:
		if componentState.Stage.IsDestroyStage() {
			return "", fmt.Errorf("component %q failed during %s in a previous run; run down again to finish cleanup before applying", componentState.Name, componentState.Stage)
		}
		return existingComponentApply, nil
	default:
		return existingComponentApply, nil
	}
}
