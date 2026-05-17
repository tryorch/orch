package state

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"orch.io/pkg/logging"
	manifestcore "orch.io/pkg/manifest/core"
	"orch.io/pkg/runners"
)

type Status string

const (
	StatusApplying   Status = "applying"
	StatusApplied    Status = "applied"
	StatusFailed     Status = "failed"
	StatusDestroying Status = "destroying"
	StatusDestroyed  Status = "destroyed"
)

// RunnerRef identifies the execution context without persisting credentials.
type RunnerRef struct {
	Name string             `json:"name"`
	Type runners.RunnerType `json:"type"`
}

type ComponentStateData struct {
	WorkDir   string                 `json:"workdir"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Artifacts []Artifact             `json:"artifacts,omitempty"`
}

type Artifact struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Required  bool   `json:"required,omitempty"`
	Sensitive bool   `json:"sensitive,omitempty"`
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
	Artifacts          []Artifact                   `json:"artifacts,omitempty"`
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

	logger logging.DebugLogger
}

// Manager handles persistence of orch state
type Manager struct {
	envID   string
	backend Backend
}

func NewManager(envID string, backend Backend) *Manager {
	return &Manager{envID: envID, backend: backend}
}

func New(envID, manifestID string, logger logging.DebugLogger) *OrchState {
	now := time.Now().UTC().Format(time.RFC3339)
	return &OrchState{
		EnvID:      envID,
		ManifestID: manifestID,
		Components: make([]ComponentState, 0),
		CreatedAt:  now,
		UpdatedAt:  now,
		logger:     logger,
	}
}

func (sm *Manager) LoadOrNew(manifestID string, logger logging.DebugLogger) (*OrchState, error) {
	exists, err := sm.Exists(context.Background())
	if err != nil {
		return nil, err
	}
	if !exists {
		return New(sm.envID, manifestID, logger), nil
	}

	current, err := sm.Load()
	if err == nil {
		return current, nil
	}

	return nil, err
}

// Load reads the state file and returns the orch state
func (sm *Manager) Load() (*OrchState, error) {
	return sm.backend.Load(context.Background(), sm.envID)
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

func (s *OrchState) FindComponent(name string) (ComponentState, bool) {
	for _, component := range s.Components {
		if component.Name == name {
			return component, true
		}
	}
	return ComponentState{}, false
}

func (s *OrchState) BeginComponentApply(component *manifestcore.Component, runnerType runners.RunnerType, workDir string) {
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range s.Components {
		if s.Components[i].Name == component.Name {
			s.Components[i].Type = component.Type
			s.Components[i].Runner = RunnerRef{
				Name: component.Runner,
				Type: runnerType,
			}
			s.Components[i].Source = component.Source
			s.Components[i].WorkDir = workDir
			s.Components[i].NonSensitiveConfig = SanitizeMap(component.Config)
			s.Components[i].Status = StatusApplying
			s.Components[i].UpdatedAt = now
			s.UpdatedAt = now
			return
		}
	}

	// create new if not found.
	s.Components = append(s.Components, ComponentState{
		Name: component.Name,
		Type: component.Type,
		Runner: RunnerRef{
			Name: component.Runner,
			Type: runnerType,
		},
		Source:             component.Source,
		WorkDir:            workDir,
		NonSensitiveConfig: SanitizeMap(component.Config),
		Outputs:            make(map[string]string),
		Payload:            make(map[string]interface{}),
		Status:             StatusApplying,
		ProvisionedAt:      now,
		UpdatedAt:          now,
	})
	s.UpdatedAt = now
}

func (s *OrchState) MarkComponentFailed(name string) {
	s.markComponentStatus(name, StatusFailed)
}

func (s *OrchState) MarkComponentDestroying(name string) {
	s.markComponentStatus(name, StatusDestroying)
}

func (s *OrchState) MarkComponentDestroyed(name string) {
	s.markComponentStatus(name, StatusDestroyed)
}

func (s *OrchState) markComponentStatus(name string, status Status) {
	logger := s.logger
	if logger == nil {
		logger = &logging.NoopDebugLogger{}
	}
	logger.Debug(
		"component status transitioned",
		logging.Field{Key: "name", Value: name},
		logging.Field{Key: "status", Value: status},
	)
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range s.Components {
		if s.Components[i].Name == name {
			s.Components[i].Status = status
			s.Components[i].UpdatedAt = now
			s.UpdatedAt = now
			return
		}
	}
}

// Save writes the orch state to the state file
func (sm *Manager) Save(state *OrchState) error {
	return sm.backend.Save(context.Background(), sm.envID, state)
}

// Exists checks if a state file exists for this environment
func (sm *Manager) Exists(ctx context.Context) (bool, error) {
	return sm.backend.Exists(ctx, sm.envID)
}

// Delete removes the state file
func (sm *Manager) Delete() error {
	return sm.backend.Delete(context.Background(), sm.envID)
}

func NewComponentState(
	component *manifestcore.Component,
	runnerType runners.RunnerType,
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
		Artifacts:          data.Artifacts,
		Status:             StatusApplied,
	}
}

func NewComponentStateData(workDir string, payload interface{}, artifacts ...Artifact) (ComponentStateData, error) {
	mapped, err := ToMap(payload)
	if err != nil {
		return ComponentStateData{}, err
	}

	return ComponentStateData{
		WorkDir:   workDir,
		Payload:   mapped,
		Artifacts: artifacts,
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
