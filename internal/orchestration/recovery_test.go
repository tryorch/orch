package orchestration

import (
	"testing"

	"orch.io/pkg/state"
)

func TestUpActionForExistingComponent(t *testing.T) {
	tests := []struct {
		name    string
		state   state.ComponentState
		options UpOptions
		want    existingComponentAction
		wantErr bool
	}{
		{
			name:  "applied skips by default",
			state: state.ComponentState{Name: "db", Status: state.StatusApplied, Stage: state.StagePostApply},
			want:  existingComponentSkip,
		},
		{
			name:    "applied reapplies with option",
			state:   state.ComponentState{Name: "db", Status: state.StatusApplied, Stage: state.StagePostApply},
			options: UpOptions{Reapply: true},
			want:    existingComponentApply,
		},
		{
			name:  "applying retries",
			state: state.ComponentState{Name: "db", Status: state.StatusApplying, Stage: state.StageApply},
			want:  existingComponentApply,
		},
		{
			name:  "failed apply stage retries",
			state: state.ComponentState{Name: "db", Status: state.StatusFailed, Stage: state.StageApply},
			want:  existingComponentApply,
		},
		{
			name:    "failed destroy stage blocks",
			state:   state.ComponentState{Name: "db", Status: state.StatusFailed, Stage: state.StageDestroy},
			wantErr: true,
		},
		{
			name:    "destroying blocks",
			state:   state.ComponentState{Name: "db", Status: state.StatusDestroying, Stage: state.StageDestroy},
			wantErr: true,
		},
		{
			name:  "destroyed applies",
			state: state.ComponentState{Name: "db", Status: state.StatusDestroyed, Stage: state.StageDestroy},
			want:  existingComponentApply,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := upActionForExistingComponent(tt.state, tt.options)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("action = %q, want %q", got, tt.want)
			}
		})
	}
}
