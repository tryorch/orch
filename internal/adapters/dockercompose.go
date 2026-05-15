package adapters

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"gopkg.in/yaml.v3"
	"orch.io/pkg/events"
	manifestcore "orch.io/pkg/manifest/core"
	"orch.io/pkg/runners"
	"orch.io/pkg/state"
	"orch.io/pkg/utils"
)

type DockerComposeAdapter struct{}
type DockerComposeConfig struct {
	Flags []string `mapstructure:"flags"`
	// Optional custom runner command, e.g., "docker compose" or "docker-compose"
	Command string `mapstructure:"command"`

	Services map[string][]ComposeServiceMetaData
	Files    []string
}

type DockerComposeState struct {
	Command      []string          `mapstructure:"command" json:"command"`
	ComposeFiles []string          `mapstructure:"compose_files" json:"compose_files"`
	Env          map[string]string `mapstructure:"env" json:"env"`
	ProjectName  string            `mapstructure:"project_name" json:"project_name"`
	WorkDir      string            `mapstructure:"workdir" json:"workdir"`
}

func (d *DockerComposeAdapter) RequiredCapabilities() runners.Capabilities {
	return runners.Capabilities{Exec: true, FileCopy: true}
}

func (d *DockerComposeAdapter) SupportedSources() ComponentSourceSupport {
	return ComponentSourceSupport{Files: true}
}

func (d *DockerComposeAdapter) ValidateAndLoadConfig(ctx context.Context, c *manifestcore.Component) (ComponentConfig, []events.Event, error) {
	var cfg DockerComposeConfig
	var warnings []events.Event

	if err := mapstructure.Decode(c.Config, &cfg); err != nil {
		return nil, warnings, err
	}

	cfg.Services = make(map[string][]ComposeServiceMetaData)
	for _, file := range c.Source.Files {
		services, err := loadComposeFileAndExtractServices(file)
		if err != nil {
			return nil, warnings, fmt.Errorf("failed to load compose file: %w", err)
		}

		for _, service := range services {
			if service.HasFixedPorts {
				warnings = append(warnings, events.Event{
					Type: events.EventWarning,
					Message: fmt.Sprintf("Compose service %q has fixed port mappings."+
						"This may lead to port conflicts when multiple instances are deployed.", service.Name),
					Hint: "Consider using dynamic port mappings or environment variables to avoid conflicts.\n" +
						"Dynamic port mappings can be specified by omitting the host port (e.g., '8080' instead of '80:8080').\n" +
						"See more info at https://orch.io/docs/guides/docker-compose#handling-port-conflicts",
					Adapter:   c.Type,
					Runner:    c.Runner,
					Component: c.Name,
				})
			}
		}

		cfg.Services[file] = services
	}
	return &cfg, warnings, nil
}

func (d *DockerComposeAdapter) Apply(ctx context.Context, c *manifestcore.Component, t runners.Runner) (ComponentApplyOutput, error) {
	cfg, ok := c.LoadedConfig.(*DockerComposeConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type for DockerComposeAdapter")
	}

	aCtx, ok := AdapterContextFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("failed to get env ID from context")
	}

	workDir := aCtx.BuildRunnerWorkDir(c.WorkDir, c.Name)

	// Copy WithFiles to workDir
	for name, file := range c.WithFiles {
		copyRes, err := t.CopyFile(ctx, runners.FileCopyRequest{
			Source:      file,
			Destination: path.Join(workDir, name),
			ToRunner:    true,
			Overwrite:   true,
			Recursive:   false,
		})

		if err != nil {
			return nil, fmt.Errorf("failed to copy with-file %q to runner: %w", name, err)
		}
		if copyRes.Error != nil {
			return nil, fmt.Errorf("error during with-file %q copy: %w", name, copyRes.Error)
		}

		aCtx.emitter.Emit(events.Event{
			Type:      events.EventInfo,
			Message:   fmt.Sprintf("Copied with-file %q to %q", name, workDir),
			Adapter:   c.Type,
			Runner:    c.Runner,
			Component: c.Name,
			Duration:  copyRes.Duration,
		})
	}

	// Copy compose files to workDir
	composeFiles := make([]string, 0, len(c.Source.Files))
	for _, file := range c.Source.Files {
		composeFiles = append(composeFiles, path.Base(file))
		copyRes, err := t.CopyFile(ctx, runners.FileCopyRequest{
			Source:      file,
			Destination: path.Join(workDir, path.Base(file)),
			ToRunner:    true,
			Overwrite:   true,
			Recursive:   false,
		})

		if err != nil {
			return nil, fmt.Errorf("failed to copy compose file %q to runner: %w", file, err)
		}
		if copyRes.Error != nil {
			return nil, fmt.Errorf("error during compose file %q copy: %w", file, copyRes.Error)
		}

		aCtx.emitter.Emit(events.Event{
			Type:      events.EventInfo,
			Message:   fmt.Sprintf("Copied compose file %q to %q", file, workDir),
			Adapter:   c.Type,
			Runner:    c.Runner,
			Component: c.Name,
			Duration:  copyRes.Duration,
		})
	}

	execCommand := []string{"docker", "compose"}
	if cfg.Command != "" {
		execCommand = strings.Split(cfg.Command, " ")
	}

	if len(cfg.Flags) > 0 {
		execCommand = append(execCommand, cfg.Flags...)
	}

	for _, cfp := range composeFiles {
		execCommand = append(execCommand, "-f", cfp)
	}

	cmd := append(execCommand, "-p", composeProjectName(aCtx.envID, c.Name), "up", "-d")

	execRes, err := t.Exec(ctx, runners.ExecCommand{
		WorkingDir: workDir,
		Command:    cmd,
		Env:        buildOrchManagedComposeEnv(c.Env, aCtx.envID, path.Dir(workDir), c.Name),
		Timeout:    0,
		Stderr:     utils.NewPrefixWriter(os.Stderr, utils.RunnerComponentPrefix(c.Runner, c.Name)),
		Stdout:     utils.NewPrefixWriter(os.Stdout, utils.RunnerComponentPrefix(c.Runner, c.Name)),
	})

	if err != nil {
		return nil, fmt.Errorf("an error occurred %w", err)
	}

	if execRes.Error != nil {
		return nil, fmt.Errorf("failed to run docker-compose up: %w", execRes.Error)
	}

	return make(ComponentApplyOutput), nil
}

func (d *DockerComposeAdapter) Destroy(ctx context.Context, c *manifestcore.Component, t runners.Runner) error {
	cfg, ok := c.LoadedConfig.(*DockerComposeConfig)
	if !ok {
		return fmt.Errorf("invalid config type for DockerComposeAdapter")
	}

	aCtx, ok := AdapterContextFromContext(ctx)
	if !ok {
		return fmt.Errorf("failed to get adapter context")
	}

	workDir := aCtx.BuildRunnerWorkDir(c.WorkDir, c.Name)

	// Build compose file paths on runner
	composeFiles := make([]string, 0, len(c.Source.Files))
	for _, file := range c.Source.Files {
		composeFiles = append(composeFiles, path.Base(file))
	}

	// Build docker compose down command
	execCommand := []string{"docker", "compose"}
	if cfg.Command != "" {
		execCommand = strings.Split(cfg.Command, " ")
	}

	if len(cfg.Flags) > 0 {
		execCommand = append(execCommand, cfg.Flags...)
	}

	for _, cfp := range composeFiles {
		execCommand = append(execCommand, "-f", cfp)
	}

	cmd := append(execCommand, "-p", composeProjectName(aCtx.envID, c.Name), "down", "-v")

	// Execute docker compose down on runner
	execRes, err := t.Exec(ctx, runners.ExecCommand{
		WorkingDir: workDir,
		Command:    cmd,
		Env:        buildOrchManagedComposeEnv(c.Env, aCtx.envID, path.Dir(workDir), c.Name),
		Timeout:    0,
		Stderr:     utils.NewPrefixWriter(os.Stderr, utils.RunnerComponentPrefix(c.Runner, c.Name)),
		Stdout:     utils.NewPrefixWriter(os.Stdout, utils.RunnerComponentPrefix(c.Runner, c.Name)),
	})

	if err != nil {
		return fmt.Errorf("failed to execute docker compose down: %w", err)
	}

	if execRes.Error != nil || execRes.ExitCode != 0 {
		return fmt.Errorf("docker compose down failed with exit code %d: %v", execRes.ExitCode, execRes.Error)
	}

	return nil
}

func (d *DockerComposeAdapter) BuildState(ctx context.Context, c *manifestcore.Component, t runners.Runner, outputs ComponentApplyOutput) (state.ComponentStateData, error) {
	cfg, ok := c.LoadedConfig.(*DockerComposeConfig)
	if !ok {
		return state.ComponentStateData{}, fmt.Errorf("invalid config type for DockerComposeAdapter")
	}

	aCtx, ok := AdapterContextFromContext(ctx)
	if !ok {
		return state.ComponentStateData{}, fmt.Errorf("failed to get adapter context")
	}

	workDir := aCtx.BuildRunnerWorkDir(c.WorkDir, c.Name)
	composeFiles := make([]string, 0, len(c.Source.Files))
	for _, file := range c.Source.Files {
		composeFiles = append(composeFiles, path.Base(file))
	}

	composeState := DockerComposeState{
		Command:      d.composeCommand(cfg),
		ComposeFiles: composeFiles,
		Env:          buildOrchManagedComposeEnv(nil, aCtx.envID, path.Dir(workDir), c.Name),
		ProjectName:  composeProjectName(aCtx.envID, c.Name),
		WorkDir:      workDir,
	}

	return state.NewComponentStateData(workDir, composeState)
}

func (d *DockerComposeAdapter) DestroyFromState(ctx context.Context, componentState state.ComponentState, t runners.Runner) error {
	var s DockerComposeState
	if err := mapstructure.Decode(componentState.Payload, &s); err != nil {
		return fmt.Errorf("failed to decode docker-compose state: %w", err)
	}

	if len(s.ComposeFiles) == 0 {
		return fmt.Errorf("docker-compose state for component %q has no compose files", componentState.Name)
	}

	execCommand := append([]string{}, s.Command...)
	for _, cfp := range s.ComposeFiles {
		execCommand = append(execCommand, "-f", cfp)
	}

	cmd := append(execCommand, "-p", s.ProjectName, "down", "-v")
	execRes, err := t.Exec(ctx, runners.ExecCommand{
		WorkingDir: s.WorkDir,
		Command:    cmd,
		Env:        s.Env,
		Timeout:    0,
		Stderr:     utils.NewPrefixWriter(os.Stderr, utils.RunnerComponentPrefix(componentState.Runner.Name, componentState.Name)),
		Stdout:     utils.NewPrefixWriter(os.Stdout, utils.RunnerComponentPrefix(componentState.Runner.Name, componentState.Name)),
	})

	if err != nil {
		return fmt.Errorf("failed to execute docker compose down: %w", err)
	}

	if execRes.Error != nil || execRes.ExitCode != 0 {
		return fmt.Errorf("docker compose down failed with exit code %d: %v", execRes.ExitCode, execRes.Error)
	}

	return nil
}

func (d *DockerComposeAdapter) composeCommand(cfg *DockerComposeConfig) []string {
	execCommand := []string{"docker", "compose"}
	if cfg.Command != "" {
		execCommand = strings.Split(cfg.Command, " ")
	}

	if len(cfg.Flags) > 0 {
		execCommand = append(execCommand, cfg.Flags...)
	}

	return execCommand
}

func buildOrchManagedComposeEnv(
	base map[string]string,
	envID string,
	workDir string,
	componentName string,
) map[string]string {
	env := make(map[string]string)
	for key, value := range base {
		env[key] = value
	}

	env["COMPOSE_PROJECT_NAME"] = composeProjectName(envID, componentName)
	env["ORCH_COMPOSE_WORKING_DIR"] = workDir
	env["ORCH_ENV_ID"] = envID
	return env
}

func init() {
	Register("docker-compose", &DockerComposeAdapter{})
}

// ComposeFile Ports Utilities
type ComposeFile struct {
	Services map[string]struct {
		Ports []string `yaml:"ports"`
	} `yaml:"services"`
}

type ComposeServiceMetaData struct {
	Ports          []string
	Name           string
	HasFixedPorts  bool
	PublishesPorts bool
}

func loadComposeFileAndExtractServices(filePath string) ([]ComposeServiceMetaData, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose file: %w", err)
	}

	var composeFile ComposeFile
	if err := yaml.Unmarshal(data, &composeFile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal compose file: %w", err)
	}

	services := make([]ComposeServiceMetaData, 0, len(composeFile.Services))
	for name, service := range composeFile.Services {

		var ports []string
		s := ComposeServiceMetaData{
			Name:           name,
			PublishesPorts: len(service.Ports) > 0,
		}

		// Check if any port mapping is fixed (i.e., host port specified)
		for _, port := range service.Ports {
			if len(strings.Split(port, ":")) > 1 {
				s.HasFixedPorts = true
			} else {
				ports = append(ports, port)
			}
		}

		s.Ports = ports
		services = append(services, s)
	}

	return services, nil
}

func composeProjectName(envID, componentName string) string {
	return fmt.Sprintf("orch_%s_%s", envID, componentName)
}
