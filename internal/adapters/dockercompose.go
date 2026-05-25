package adapters

import (
	"context"
	"fmt"
	"os"
	"path"
	"sort"
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
	for name, file := range c.Source.Files {
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

		cfg.Services[name] = services
	}
	return &cfg, warnings, nil
}

func (d *DockerComposeAdapter) Apply(ctx context.Context, c *manifestcore.Component, t runners.Runner) (ComponentApplyResult, error) {
	cfg, ok := c.LoadedConfig.(*DockerComposeConfig)
	if !ok {
		return ComponentApplyResult{}, fmt.Errorf("invalid config type for DockerComposeAdapter")
	}

	aCtx, ok := AdapterContextFromContext(ctx)
	if !ok {
		return ComponentApplyResult{}, fmt.Errorf("failed to get env ID from context")
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
			return ComponentApplyResult{}, fmt.Errorf("failed to copy with-file %q to runner: %w", name, err)
		}
		if copyRes.Error != nil {
			return ComponentApplyResult{}, fmt.Errorf("error during with-file %q copy: %w", name, copyRes.Error)
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
	for name, file := range c.Source.Files {
		composeFiles = append(composeFiles, name)
		copyRes, err := t.CopyFile(ctx, runners.FileCopyRequest{
			Source:      file,
			Destination: path.Join(workDir, name),
			ToRunner:    true,
			Overwrite:   true,
			Recursive:   false,
		})

		if err != nil {
			return ComponentApplyResult{}, fmt.Errorf("failed to copy compose file %q to runner: %w", file, err)
		}
		if copyRes.Error != nil {
			return ComponentApplyResult{}, fmt.Errorf("error during compose file %q copy: %w", file, copyRes.Error)
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

	execCommand := d.composeCommand(cfg)
	for _, cfp := range composeFiles {
		execCommand = append(execCommand, "-f", cfp)
	}

	projectName := composeProjectName(aCtx.envID, c.Name)
	composeEnv := buildOrchManagedComposeEnv(c.Env, aCtx.envID, path.Dir(workDir), c.Name)
	cmd := append(execCommand, "-p", projectName, "up", "-d")

	execRes, err := t.Exec(ctx, runners.ExecCommand{
		WorkingDir: workDir,
		Command:    cmd,
		Env:        composeEnv,
		Timeout:    0,
		Stderr:     utils.NewPrefixWriter(os.Stderr, utils.RunnerComponentPrefix(c.Runner, c.Name)),
		Stdout:     utils.NewPrefixWriter(os.Stdout, utils.RunnerComponentPrefix(c.Runner, c.Name)),
	})

	if err != nil {
		return ComponentApplyResult{}, fmt.Errorf("an error occurred %w", err)
	}

	if execRes.Error != nil {
		return ComponentApplyResult{}, fmt.Errorf("failed to run docker-compose up: %w", execRes.Error)
	}

	outputs, err := d.capturePortOutputs(ctx, t, workDir, execCommand, projectName, composeEnv, cfg.Services)
	if err != nil {
		return ComponentApplyResult{}, err
	}

	composeState := DockerComposeState{
		Command:      d.composeCommand(cfg),
		ComposeFiles: composeFiles,
		Env:          buildOrchManagedComposeEnv(nil, aCtx.envID, path.Dir(workDir), c.Name),
		ProjectName:  composeProjectName(aCtx.envID, c.Name),
		WorkDir:      workDir,
	}
	stateData, err := state.NewComponentStateData(workDir, composeState)
	if err != nil {
		return ComponentApplyResult{}, err
	}

	return ComponentApplyResult{
		Outputs: outputs,
		State:   stateData,
	}, nil
}

func (d *DockerComposeAdapter) Destroy(ctx context.Context, componentState state.ComponentState, t runners.Runner) error {
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

	env := make(map[string]string, len(componentState.Env)+len(s.Env))
	for key, value := range componentState.Env {
		env[key] = value
	}
	for key, value := range s.Env {
		env[key] = value
	}

	cmd := append(execCommand, "-p", s.ProjectName, "down", "-v")
	execRes, err := t.Exec(ctx, runners.ExecCommand{
		WorkingDir: s.WorkDir,
		Command:    cmd,
		Env:        env,
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

func (d *DockerComposeAdapter) capturePortOutputs(
	ctx context.Context,
	t runners.Runner,
	workDir string,
	composeCommand []string,
	projectName string,
	env map[string]string,
	servicesByFile map[string][]ComposeServiceMetaData,
) (ComponentApplyOutput, error) {
	outputs := make(ComponentApplyOutput)
	portsByService := make(map[string]map[string]composePortBinding)
	// Docker Compose merges every -f file into one effective project before
	// running. These outputs intentionally describe that merged service graph,
	// not the individual compose files where a service or port was declared.
	services := flattenComposeServices(servicesByFile)
	for _, service := range services {
		for _, port := range service.Ports {
			cmd := append(append([]string{}, composeCommand...), "-p", projectName, "port", service.Name, port)
			res, err := t.Exec(ctx, runners.ExecCommand{
				WorkingDir: workDir,
				Command:    cmd,
				Env:        env,
				Timeout:    0,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to inspect docker compose port for service %q port %q: %w", service.Name, port, err)
			}
			if res.Error != nil || res.ExitCode != 0 {
				return nil, fmt.Errorf("docker compose port failed for service %q port %q with exit code %d: %v", service.Name, port, res.ExitCode, res.Error)
			}

			binding := strings.TrimSpace(string(res.Stdout))
			if binding == "" {
				continue
			}
			hostPort, err := hostPortFromComposeBinding(binding)
			if err != nil {
				return nil, fmt.Errorf("failed to parse docker compose binding %q for service %q port %q: %w", binding, service.Name, port, err)
			}

			if _, ok := portsByService[service.Name]; !ok {
				portsByService[service.Name] = make(map[string]composePortBinding)
			}
			portsByService[service.Name][port] = composePortBinding{
				Binding:  binding,
				HostPort: hostPort,
			}
		}
	}
	for serviceName, ports := range portsByService {
		for port, binding := range ports {
			outputs[fmt.Sprintf("_meta.ports.services.%s.%s", serviceName, port)] = binding.HostPort
			outputs[fmt.Sprintf("_meta.bindings.services.%s.%s", serviceName, port)] = binding.Binding
		}
	}
	return outputs, nil
}

type composePortBinding struct {
	Binding  string
	HostPort string
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
	env["ORCH_ENV_ID"] = envID
	return env
}

func init() {
	Register("docker-compose", &DockerComposeAdapter{})
}

// ComposeFile Ports Utilities
type ComposeFile struct {
	Services map[string]ComposeService `yaml:"services"`
}

type ComposeService struct {
	Ports []yaml.Node `yaml:"ports"`
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

		for _, port := range service.Ports {
			containerPort, fixed, ok := composeContainerPort(port)
			if !ok {
				continue
			}
			if fixed {
				s.HasFixedPorts = true
			}
			ports = append(ports, containerPort)
		}

		s.Ports = uniqueSortedStrings(ports)
		services = append(services, s)
	}

	return services, nil
}

func composeContainerPort(port yaml.Node) (string, bool, bool) {
	switch port.Kind {
	case yaml.ScalarNode:
		return composeContainerPortFromShortSyntax(port.Value)
	case yaml.MappingNode:
		return composeContainerPortFromLongSyntax(port)
	default:
		return "", false, false
	}
}

func composeContainerPortFromShortSyntax(value string) (string, bool, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false, false
	}

	withoutProtocol := strings.SplitN(value, "/", 2)[0]
	parts := strings.Split(withoutProtocol, ":")
	containerPort := parts[len(parts)-1]
	if strings.Contains(containerPort, "-") {
		return "", len(parts) > 1, false
	}
	return containerPort, len(parts) > 1, containerPort != ""
}

func composeContainerPortFromLongSyntax(port yaml.Node) (string, bool, bool) {
	var target string
	var fixed bool
	for i := 0; i+1 < len(port.Content); i += 2 {
		key := port.Content[i].Value
		value := strings.TrimSpace(port.Content[i+1].Value)
		switch key {
		case "target":
			target = strings.SplitN(value, "/", 2)[0]
		case "published":
			fixed = value != ""
		}
	}
	if target == "" || strings.Contains(target, "-") {
		return "", fixed, false
	}
	return target, fixed, true
}

func flattenComposeServices(servicesByFile map[string][]ComposeServiceMetaData) []ComposeServiceMetaData {
	merged := make(map[string]map[string]struct{})
	for _, services := range servicesByFile {
		for _, service := range services {
			if _, ok := merged[service.Name]; !ok {
				merged[service.Name] = make(map[string]struct{})
			}
			for _, port := range service.Ports {
				merged[service.Name][port] = struct{}{}
			}
		}
	}

	serviceNames := make([]string, 0, len(merged))
	for serviceName := range merged {
		serviceNames = append(serviceNames, serviceName)
	}
	sort.Strings(serviceNames)

	services := make([]ComposeServiceMetaData, 0, len(serviceNames))
	for _, serviceName := range serviceNames {
		services = append(services, ComposeServiceMetaData{
			Name:  serviceName,
			Ports: sortedMapKeys(merged[serviceName]),
		})
	}
	return services
}

func hostPortFromComposeBinding(binding string) (string, error) {
	hostPort := binding
	if idx := strings.LastIndex(hostPort, ":"); idx >= 0 {
		hostPort = hostPort[idx+1:]
	}
	hostPort = strings.TrimSpace(hostPort)
	if hostPort == "" {
		return "", fmt.Errorf("missing host port")
	}
	return hostPort, nil
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	return sortedMapKeys(seen)
}

func sortedMapKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func composeProjectName(envID, componentName string) string {
	return fmt.Sprintf("orch_%s_%s", envID, componentName)
}
