package adapters

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/go-viper/mapstructure/v2"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"orch.io/pkg/logging"

	"orch.io/pkg/events"
	manifestcore "orch.io/pkg/manifest/core"
	"orch.io/pkg/runners"
	"orch.io/pkg/state"
	"orch.io/pkg/utils"
)

type TerraformAdapter struct{}

type TerraformConfig struct {
	Vars map[string]string `mapstructure:"vars"`

	// ModulePath is the path to the Terraform module to be applied.
	ModulePath     string
	FoundVariables map[string]*tfconfig.Variable
}

type TerraformState struct {
	Vars    map[string]string `mapstructure:"vars" json:"vars"`
	WorkDir string            `mapstructure:"workdir" json:"workdir"`
}

func (d *TerraformAdapter) RequiredCapabilities() runners.Capabilities {
	return runners.Capabilities{FileCopy: true, Exec: true}
}

func (d *TerraformAdapter) SupportedSources() ComponentSourceSupport {
	return ComponentSourceSupport{Embedded: true, Path: true}
}

func (d *TerraformAdapter) ValidateAndLoadConfig(ctx context.Context, c *manifestcore.Component) (ComponentConfig, []events.Event, error) {
	var cfg TerraformConfig
	var warnings []events.Event

	if err := mapstructure.Decode(c.Config, &cfg); err != nil {
		return nil, warnings, err
	}

	adapterCtx, ok := AdapterContextFromContext(ctx)
	if !ok {
		return nil, nil, fmt.Errorf("failed to get adapter context")
	}

	compWorkDir := adapterCtx.GetComponentWorkDirInOrchLocalWorkDir(c.Name)
	modulePath, err := loadAndGetTerraformModulePath(c.Source, compWorkDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve terraform module path: %w", err)
	}

	module, _ := tfconfig.LoadModule(modulePath)
	if cfg.Vars == nil {
		cfg.Vars = make(map[string]string)
	}
	cfg.ModulePath = modulePath

	// Introspect variables defined in the module.
	// These can be used for validation or prompting for missing variables in the future.
	cfg.FoundVariables = module.Variables
	for varName, varDef := range module.Variables {
		if varDef.Default == nil && cfg.Vars[varName] == "" && os.Getenv(varName) == "" {
			warnings = append(warnings, events.Event{
				Type:      events.EventWarning,
				Message:   fmt.Sprintf("Terraform variable %q is required but has no default value. Make sure to provide a value via the component config vars or environment variables.", varName),
				Adapter:   c.Type,
				Runner:    c.Runner,
				Component: c.Name,
				Hint: "Provide a value for this variable in the component config " +
					"under the 'vars' section, or set it as an environment variable.",
			})
		}

		// If the variable has no default value, check if it's provided via environment variables and use that as a fallback.
		if varDef.Default == nil && os.Getenv(varName) != "" && cfg.Vars[varName] == "" {
			cfg.Vars[varName] = os.Getenv(varName)
		}
	}

	return &cfg, warnings, nil
}

func (d *TerraformAdapter) Apply(ctx context.Context, c *manifestcore.Component, t runners.Runner) (ComponentApplyOutput, error) {
	cfg, ok := c.LoadedConfig.(*TerraformConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type for TerraformAdapter")
	}

	aCtx, ok := AdapterContextFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("failed to get adapter context")
	}

	workDir := aCtx.BuildRunnerWorkDir(c.WorkDir, c.Name)

	// Copy with-files to runner
	for name, file := range c.WithFiles {
		copyRes, err := t.CopyFile(ctx, runners.FileCopyRequest{
			Source:      file,
			Destination: path.Join(workDir, name),
			ToRunner:    true,
			Overwrite:   true,
			Recursive:   false,
		})

		if err != nil {
			return nil, fmt.Errorf("failed to copy terraform with-file %q to runner: %w", name, err)
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

	// Copy module files to runner
	copyRes, err := t.CopyFile(ctx, runners.FileCopyRequest{
		Source:      cfg.ModulePath,
		Destination: workDir,
		ToRunner:    true,
		Overwrite:   true,
		Recursive:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to copy terraform module to runner: %w", err)
	}

	if copyRes.Error != nil {
		return nil, fmt.Errorf("error during terraform module copy: %w", copyRes.Error)
	}

	aCtx.emitter.Emit(events.Event{
		Type:      events.EventInfo,
		Message:   fmt.Sprintf("Copied terraform module to %q", workDir),
		Adapter:   c.Type,
		Runner:    c.Runner,
		Component: c.Name,
		Duration:  copyRes.Duration,
	})

	// Execute terraform init on runner
	aCtx.logger.Debug("executing terraform init", logging.Field{Key: "workdir", Value: workDir})
	initRes, err := t.Exec(ctx, runners.ExecCommand{
		WorkingDir: workDir,
		Command:    []string{"terraform", "init", "-upgrade"},
		Env:        c.Env,
		Timeout:    0,
		Stderr:     utils.NewPrefixWriter(os.Stderr, utils.RunnerComponentPrefix(c.Runner, c.Name)),
		Stdout:     utils.NewPrefixWriter(os.Stdout, utils.RunnerComponentPrefix(c.Runner, c.Name)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute terraform init: %w", err)
	}
	if initRes.Error != nil || initRes.ExitCode != 0 {
		return nil, fmt.Errorf("terraform init failed with exit code %d: %v", initRes.ExitCode, initRes.Error)
	}

	// Build terraform apply command with variables
	applyCmd := []string{"terraform", "apply", "-auto-approve"}
	for k, v := range cfg.Vars {
		applyCmd = append(applyCmd, "-var", fmt.Sprintf("%s=%s", k, v))
	}

	// Execute terraform apply on runner
	aCtx.logger.Debug("executing terraform apply", logging.Field{Key: "workdir", Value: workDir})
	applyRes, err := t.Exec(ctx, runners.ExecCommand{
		WorkingDir: workDir,
		Command:    applyCmd,
		Env:        c.Env,
		Timeout:    0,
		Stderr:     utils.NewPrefixWriter(os.Stderr, utils.RunnerComponentPrefix(c.Runner, c.Name)),
		Stdout:     utils.NewPrefixWriter(os.Stdout, utils.RunnerComponentPrefix(c.Runner, c.Name)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute terraform apply: %w", err)
	}
	if applyRes.Error != nil || applyRes.ExitCode != 0 {
		return nil, fmt.Errorf("terraform apply failed with exit code %d: %v", applyRes.ExitCode, applyRes.Error)
	}

	// TODO: Extract outputs using terraform output -json
	// For now, return empty outputs
	return make(ComponentApplyOutput), nil
}

func (d *TerraformAdapter) Destroy(ctx context.Context, c *manifestcore.Component, t runners.Runner) error {
	cfg, ok := c.LoadedConfig.(*TerraformConfig)
	if !ok {
		return fmt.Errorf("invalid config type for TerraformAdapter")
	}

	aCtx, ok := AdapterContextFromContext(ctx)
	if !ok {
		return fmt.Errorf("failed to get adapter context")
	}

	workDir := aCtx.BuildRunnerWorkDir(c.WorkDir, c.Name)

	// Build terraform destroy command with variables
	destroyCmd := []string{"terraform", "destroy", "-auto-approve"}
	for k, v := range cfg.Vars {
		destroyCmd = append(destroyCmd, "-var", fmt.Sprintf("%s=%s", k, v))
	}

	// Execute terraform destroy on runner
	aCtx.logger.Debug("executing terraform destroy", logging.Field{Key: "workdir", Value: workDir})
	destroyRes, err := t.Exec(ctx, runners.ExecCommand{
		WorkingDir: workDir,
		Command:    destroyCmd,
		Env:        c.Env,
		Timeout:    0,
		Stderr:     utils.NewPrefixWriter(os.Stderr, utils.RunnerComponentPrefix(c.Runner, c.Name)),
		Stdout:     utils.NewPrefixWriter(os.Stdout, utils.RunnerComponentPrefix(c.Runner, c.Name)),
	})
	if err != nil {
		return fmt.Errorf("failed to execute terraform destroy: %w", err)
	}
	if destroyRes.Error != nil || destroyRes.ExitCode != 0 {
		return fmt.Errorf("terraform destroy failed with exit code %d: %v", destroyRes.ExitCode, destroyRes.Error)
	}

	return nil
}

func (d *TerraformAdapter) BuildState(ctx context.Context, c *manifestcore.Component, t runners.Runner, outputs ComponentApplyOutput) (state.ComponentStateData, error) {
	cfg, ok := c.LoadedConfig.(*TerraformConfig)
	if !ok {
		return state.ComponentStateData{}, fmt.Errorf("invalid config type for TerraformAdapter")
	}

	aCtx, ok := AdapterContextFromContext(ctx)
	if !ok {
		return state.ComponentStateData{}, fmt.Errorf("failed to get adapter context")
	}

	workDir := aCtx.BuildRunnerWorkDir(c.WorkDir, c.Name)
	vars := make(map[string]string, len(cfg.Vars))
	for key, value := range cfg.Vars {
		vars[key] = value
	}

	terraformState := TerraformState{
		Vars:    vars,
		WorkDir: workDir,
	}

	return state.NewComponentStateData(workDir, terraformState)
}

func (d *TerraformAdapter) DestroyFromState(ctx context.Context, componentState state.ComponentState, t runners.Runner) error {
	var s TerraformState
	if err := mapstructure.Decode(componentState.Payload, &s); err != nil {
		return fmt.Errorf("failed to decode terraform state: %w", err)
	}
	if s.WorkDir == "" {
		return fmt.Errorf("terraform state for component %q has no workdir", componentState.Name)
	}

	destroyCmd := []string{"terraform", "destroy", "-auto-approve"}
	for key, value := range s.Vars {
		destroyCmd = append(destroyCmd, "-var", fmt.Sprintf("%s=%s", key, value))
	}

	destroyRes, err := t.Exec(ctx, runners.ExecCommand{
		WorkingDir: s.WorkDir,
		Command:    destroyCmd,
		Timeout:    0,
		Stderr:     utils.NewPrefixWriter(os.Stderr, utils.RunnerComponentPrefix(componentState.Runner.Name, componentState.Name)),
		Stdout:     utils.NewPrefixWriter(os.Stdout, utils.RunnerComponentPrefix(componentState.Runner.Name, componentState.Name)),
	})
	if err != nil {
		return fmt.Errorf("failed to execute terraform destroy: %w", err)
	}
	if destroyRes.Error != nil || destroyRes.ExitCode != 0 {
		return fmt.Errorf("terraform destroy failed with exit code %d: %v", destroyRes.ExitCode, destroyRes.Error)
	}

	return nil
}

func init() {
	Register("terraform", &TerraformAdapter{})
}

// loadAndGetTerraformModulePath handles loading the Terraform module from the component
// source and returns the path to the module on disk. It supports both embedded content and local paths.
// For embedded content, it writes the content to a temporary directory and returns that path. For local paths,
// it validates that the path exists and is a directory before returning it.
func loadAndGetTerraformModulePath(source manifestcore.ComponentSource, compWorkDir string) (string, error) {
	fs := utils.LocalFS{}
	if source.Type() == manifestcore.ComponentSourceTypeEmbedded && source.Embedded != "" {
		dir := path.Join(compWorkDir, "module")

		// Create the directory if it doesn't exist
		info, err := fs.Stat(dir)
		if err != nil || !info.IsDir() {
			err := fs.MkdirAll(dir)
			if err != nil {
				return "", fmt.Errorf("failed to create temporary terraform module directory: %w", err)
			}
		}

		writer, err := fs.Create(path.Join(dir, "main.tf"))
		if err != nil {
			return "", fmt.Errorf("failed to create temporary terraform file: %w", err)
		}

		defer func(writer utils.FileWriter) {
			err := writer.Close()
			if err != nil {

			}
		}(writer)

		_, err = writer.Write([]byte(source.Embedded))
		if err != nil {
			return "", fmt.Errorf("failed to write terraform content to temporary file: %w", err)
		}

		return dir, nil
	}

	if source.Type() == manifestcore.ComponentSourceTypePath && source.Path != "" {
		isDir, err := fs.IsDir(source.Path)

		if err != nil {
			return "", fmt.Errorf("invalid terraform module path %q: %w", source.Path, err)
		}

		if !isDir {
			return "", fmt.Errorf("terraform module path %q is not a directory", source.Path)
		}

		return source.Path, nil
	}

	return "", fmt.Errorf("unsupported terraform component source type")
}
