package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	manifestcore "orch.io/pkg/manifest/core"
)

type Status string

const (
	StatusApplied   Status = "applied"
	StatusDestroyed Status = "destroyed"
)

// RunnerRef identifies the execution context without persisting credentials.
type RunnerRef struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type ComponentStateData struct {
	WorkDir string                 `json:"workdir"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// ComponentState represents the state of a single provisioned component
type ComponentState struct {
	Name               string                       `json:"name"`
	Type               string                       `json:"type"`
	Runner             RunnerRef                    `json:"runner"`
	Source             manifestcore.ComponentSource `json:"source,omitempty"`
	WorkDir            string                       `json:"workdir,omitempty"`
	NonSensitiveConfig map[string]interface{}       `json:"non_sensitive_config,omitempty"`
	Outputs            map[string]string            `json:"outputs,omitempty"`
	Payload            map[string]interface{}       `json:"payload,omitempty"`
	Status             Status                       `json:"status"`
	ProvisionedAt      string                       `json:"provisioned_at"`
	UpdatedAt          string                       `json:"updated_at"`
}

// OrchState represents the state of an entire orch environment
type OrchState struct {
	EnvID      string           `json:"env_id"`
	ManifestID string           `json:"manifest_id"`
	Components []ComponentState `json:"components"`
	CreatedAt  string           `json:"created_at"`
	UpdatedAt  string           `json:"updated_at"`
}

// Manager handles persistence of orch state
type Manager struct {
	envID     string
	stateFile string
}

// NewStateManager creates a new state manager for the given environment ID
func NewStateManager(envID string) *Manager {
	stateDir := path.Join(".orch", envID)
	stateFile := path.Join(stateDir, "state.json")
	return &Manager{
		envID:     envID,
		stateFile: stateFile,
	}
}

func New(envID, manifestID string) *OrchState {
	now := time.Now().UTC().Format(time.RFC3339)
	return &OrchState{
		EnvID:      envID,
		ManifestID: manifestID,
		Components: make([]ComponentState, 0),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func (sm *Manager) LoadOrNew(manifestID string) (*OrchState, error) {
	if !sm.Exists() {
		return New(sm.envID, manifestID), nil
	}

	current, err := sm.Load()
	if err == nil {
		return current, nil
	}

	return nil, err
}

// Load reads the state file and returns the orch state
func (sm *Manager) Load() (*OrchState, error) {
	data, err := os.ReadFile(sm.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("state file not found: %s (environment may not exist or was never created)", sm.stateFile)
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state OrchState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}

func (s *OrchState) UpsertComponent(component ComponentState) {
	now := time.Now().UTC().Format(time.RFC3339)
	component.UpdatedAt = now

	for i, existing := range s.Components {
		if existing.Name == component.Name {
			if component.ProvisionedAt == "" {
				component.ProvisionedAt = existing.ProvisionedAt
			}
			s.Components[i] = component
			s.UpdatedAt = now
			return
		}
	}

	if component.ProvisionedAt == "" {
		component.ProvisionedAt = now
	}
	s.Components = append(s.Components, component)
	s.UpdatedAt = now
}

func (s *OrchState) MarkComponentDestroyed(name string) {
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range s.Components {
		if s.Components[i].Name == name {
			s.Components[i].Status = StatusDestroyed
			s.Components[i].UpdatedAt = now
			s.UpdatedAt = now
			return
		}
	}
}

// Save writes the orch state to the state file
func (sm *Manager) Save(state *OrchState) error {
	stateDir := path.Dir(sm.stateFile)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(sm.stateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// Exists checks if a state file exists for this environment
func (sm *Manager) Exists() bool {
	_, err := os.Stat(sm.stateFile)
	return err == nil
}

// Delete removes the state file
func (sm *Manager) Delete() error {
	if err := os.Remove(sm.stateFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete state file: %w", err)
	}
	return nil
}

func NewComponentState(
	component *manifestcore.Component,
	runnerType string,
	outputs map[string]string,
	data ComponentStateData,
) ComponentState {
	return ComponentState{
		Name: component.Name,
		Type: component.Type,
		Runner: RunnerRef{
			Name: component.Runner,
			Type: runnerType,
		},
		Source:             component.Source,
		WorkDir:            data.WorkDir,
		NonSensitiveConfig: SanitizeMap(component.Config),
		Outputs:            outputs,
		Payload:            data.Payload,
		Status:             StatusApplied,
	}
}

func NewComponentStateData(workDir string, payload interface{}) (ComponentStateData, error) {
	mapped, err := ToMap(payload)
	if err != nil {
		return ComponentStateData{}, err
	}

	return ComponentStateData{
		WorkDir: workDir,
		Payload: mapped,
	}, nil
}

func EmptyComponentStateData(workDir string) ComponentStateData {
	return ComponentStateData{
		WorkDir: workDir,
		Payload: make(map[string]interface{}),
	}
}

func ToMap(in interface{}) (map[string]interface{}, error) {
	if in == nil {
		return make(map[string]interface{}), nil
	}

	if mapped, ok := in.(map[string]interface{}); ok {
		return mapped, nil
	}

	data, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal state data: %w", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state data: %w", err)
	}

	if out == nil {
		out = make(map[string]interface{})
	}
	return out, nil
}

func SanitizeMap(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}

	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		if IsSensitiveKey(key) {
			out[key] = "<redacted>"
			continue
		}

		switch typed := value.(type) {
		case map[string]interface{}:
			out[key] = SanitizeMap(typed)
		case map[interface{}]interface{}:
			nested := make(map[string]interface{}, len(typed))
			for nestedKey, nestedValue := range typed {
				nested[fmt.Sprint(nestedKey)] = nestedValue
			}
			out[key] = SanitizeMap(nested)
		default:
			out[key] = typed
		}
	}
	return out
}

func IsSensitiveKey(key string) bool {
	normalized := strings.ToLower(key)
	sensitiveParts := []string{
		"password",
		"passwd",
		"secret",
		"token",
		"credential",
		"private_key",
		"access_key",
	}

	for _, part := range sensitiveParts {
		if strings.Contains(normalized, part) {
			return true
		}
	}

	return false
}
