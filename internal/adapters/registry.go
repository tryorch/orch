package adapters

import (
	"context"
	"fmt"
	"strings"

	"orch.io/pkg/events"
	manifestcore "orch.io/pkg/manifest/core"
	"orch.io/pkg/runners"
	"orch.io/pkg/state"
)

type ComponentSourceSupport struct {
	Path     bool
	Files    bool
	Embedded bool
}

func (s ComponentSourceSupport) String() string {
	var supported []string
	if s.Path {
		supported = append(supported, "Path")
	}
	if s.Files {
		supported = append(supported, "Files")
	}
	if s.Embedded {
		supported = append(supported, "Embedded")
	}

	return "[" + strings.Join(supported, ", ") + "]"
}

func (s ComponentSourceSupport) SatisfiedBy(c manifestcore.ComponentSource) bool {
	if c.Path != "" && !s.Path {
		return false
	}
	if len(c.Files) > 0 && !s.Files {
		return false
	}
	if c.Embedded != "" && !s.Embedded {
		return false
	}

	if c.Type() == "none" && (s.Path || s.Files || s.Embedded) {
		return false
	}

	return true
}

type ComponentConfig interface{}
type ComponentApplyOutput map[string]string

type Adapter interface {
	Apply(ctx context.Context, c *manifestcore.Component, r runners.Runner) (ComponentApplyOutput, error)
	Destroy(ctx context.Context, c *manifestcore.Component, r runners.Runner) error
	BuildState(ctx context.Context, c *manifestcore.Component, r runners.Runner, outputs ComponentApplyOutput) (state.ComponentStateData, error)
	DestroyFromState(ctx context.Context, c state.ComponentState, r runners.Runner) error
	RequiredCapabilities() runners.Capabilities
	SupportedSources() ComponentSourceSupport

	// ValidateAndLoadConfig validates the component config and loads it into a structured format.
	// It returns the loaded config, any warning events generated during validation, and an error if validation fails.
	ValidateAndLoadConfig(ctx context.Context, c *manifestcore.Component) (ComponentConfig, []events.Event, error)
}

var registry = map[string]Adapter{}

func Register(name string, a Adapter) {
	registry[name] = a
}

func Get(name string) (Adapter, error) {
	a, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("no adapter registered for type %s", name)
	}
	return a, nil
}
