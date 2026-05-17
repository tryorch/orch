package manifestcore

import "fmt"

type Manifest struct {
	Version    string                    `yaml:"version"`
	Inputs     map[string]Input          `yaml:"inputs,omitempty"`
	Metadata   Metadata                  `yaml:"metadata"`
	State      *StateConfig              `yaml:"state,omitempty"`
	Runners    map[string]RunnerManifest `yaml:"runners,omitempty"`
	Components []Component               `yaml:"components"`
}

type StateConfig struct {
	Backend string                 `yaml:"backend,omitempty"`
	Config  map[string]interface{} `yaml:"config,omitempty"`
}

type Input struct {
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
	Default     string `yaml:"default,omitempty"`
	Sensitive   bool   `yaml:"sensitive,omitempty"`
	Required    bool   `yaml:"required,omitempty"`
}

type Metadata struct {
	ID          string            `yaml:"id"`
	Description string            `yaml:"description"`
	Owner       Owner             `yaml:"owner"`
	Labels      map[string]string `yaml:"labels,omitempty"`
}

type Owner struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
}

type RunnerManifest struct {
	Type   string
	Config map[string]interface{} `yaml:"config"`
	// Providers holds any provider specific bootstrap
	// configuration needed for the execution context.
	Providers map[string]interface{} `yaml:"providers,omitempty"`
}

type ComponentType string

type Hook struct {
	Command string            `yaml:"command"`
	Shell   []string          `yaml:"shell,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
}

type Hooks struct {
	PreApply    []Hook `yaml:"pre_apply,omitempty"`
	PostApply   []Hook `yaml:"post_apply,omitempty"`
	PreDestroy  []Hook `yaml:"pre_destroy,omitempty"`
	PostDestroy []Hook `yaml:"post_destroy,omitempty"`
}

func (h Hooks) HasApplyHooks() bool {
	return len(h.PreApply) > 0 || len(h.PostApply) > 0
}

func (h Hooks) HasDestroyHooks() bool {
	return len(h.PreDestroy) > 0 || len(h.PostDestroy) > 0
}

func (h Hooks) HasAny() bool {
	return h.HasApplyHooks() || h.HasDestroyHooks()
}

type ComponentSource struct {
	// Embedded allows embedding raw string content directly in the manifest.
	Embedded string `yaml:"embedded,omitempty" json:"embedded,omitempty"`
	// Path specifies a directory path to load the component from.
	Path string `yaml:"path,omitempty" json:"path,omitempty"`
	// Files maps runner-side names to local files to load for the component.
	Files map[string]string `yaml:"files,omitempty" json:"files,omitempty"`
}

func (c ComponentSource) Validate() (bool, error) {
	count := 0
	if c.Embedded != "" {
		count++
	}
	if c.Path != "" {
		count++
	}
	if len(c.Files) > 0 {
		count++
	}

	if count > 1 {
		return false, fmt.Errorf("multiple source types specified; only one of 'embedded', 'path', or 'files' is allowed")
	}
	return true, nil
}

type ComponentSourceType string

const (
	ComponentSourceTypeEmbedded ComponentSourceType = "embedded"
	ComponentSourceTypePath     ComponentSourceType = "path"
	ComponentSourceTypeFiles    ComponentSourceType = "files"
	ComponentSourceTypeNone     ComponentSourceType = "none"
)

func (c ComponentSource) Type() ComponentSourceType {
	if c.Embedded != "" {
		return ComponentSourceTypeEmbedded
	}
	if c.Path != "" {
		return ComponentSourceTypePath
	}
	if len(c.Files) > 0 {
		return ComponentSourceTypeFiles
	}
	return ComponentSourceTypeNone
}

type Output struct {
	Name      string `yaml:"name"`
	Required  *bool  `yaml:"required,omitempty"`
	Sensitive bool   `yaml:"sensitive,omitempty"`
	Type      string `yaml:"type,omitempty"`
}

func (o Output) IsRequired() bool {
	return o.Required == nil || *o.Required
}

type Component struct {
	Name      string                 `yaml:"name"`
	Type      string                 `yaml:"type"`
	DependsOn []string               `yaml:"depends_on,omitempty"`
	Config    map[string]interface{} `yaml:"config,omitempty"`
	Hooks     Hooks                  `yaml:"hooks,omitempty"`
	Source    ComponentSource        `yaml:"source,omitempty"`
	WithFiles map[string]string      `yaml:"with,omitempty"`
	Env       map[string]string      `yaml:"env,omitempty"`
	Outputs   []Output               `yaml:"outputs,omitempty"`
	Runner    string                 `yaml:"runner,omitempty"`
	WorkDir   string                 `yaml:"workdir,omitempty"`

	// LoadedConfig holds the validated and loaded configuration specific to the component's adapter.
	LoadedConfig interface{} `yaml:"-"`
}
