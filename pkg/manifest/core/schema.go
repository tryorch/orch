package manifestcore

import "fmt"

type Manifest struct {
	Version    string                    `yaml:"version"`
	Inputs     map[string]Input          `yaml:"inputs,omitempty"`
	Metadata   Metadata                  `yaml:"metadata"`
	Runners    map[string]RunnerManifest `yaml:"runners,omitempty"`
	Components []Component               `yaml:"components"`
}

type Input struct {
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
	Default     string `yaml:"default,omitempty"`
	Sensitive   bool   `yaml:"sensitive,omitempty"`
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

type Hooks struct {
	Create struct {
		PreRun  []string `yaml:"preRun,omitempty"`
		PostRun []string `yaml:"postRun,omitempty"`
	} `yaml:"create,omitempty"`
	Destroy struct {
		PreRun  []string `yaml:"preRun,omitempty"`
		PostRun []string `yaml:"postRun,omitempty"`
	} `yaml:"destroy,omitempty"`
}

type ComponentSource struct {
	// Embedded allows embedding raw string content directly in the manifest.
	Embedded string `yaml:"embedded,omitempty"`
	// Path specifies a directory path to load the component from.
	Path string `yaml:"path,omitempty"`
	// Files specifies a list of files to load for the component.
	Files []string `yaml:"files,omitempty"`
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

type Component struct {
	Name      string                 `yaml:"name"`
	Type      string                 `yaml:"type"`
	DependsOn []string               `yaml:"depends_on,omitempty"`
	Config    map[string]interface{} `yaml:"config,omitempty"`
	Hooks     Hooks                  `yaml:"hooks,omitempty"`
	Source    ComponentSource        `yaml:"source,omitempty"`
	WithFiles map[string]string      `yaml:"with,omitempty"`
	Env       map[string]string      `yaml:"env,omitempty"`
	Outputs   []string               `yaml:"outputs,omitempty"`
	Runner    string                 `yaml:"runner,omitempty"`

	// LoadedConfig holds the validated and loaded configuration specific to the component's adapter.
	LoadedConfig interface{} `yaml:"-"`
}
