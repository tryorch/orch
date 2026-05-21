package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"orch.io/internal/adapters/adaptersupport"
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
	HasBackend     bool
	BackendType    string
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
	hasBackend, backendType, err := detectTerraformBackend(modulePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to inspect terraform backend configuration: %w", err)
	}
	cfg.HasBackend = hasBackend
	cfg.BackendType = backendType
	if cfg.HasBackend {
		adapterCtx.logger.Debug(
			"detected terraform backend; skipping local terraform state artifact capture",
			logging.Field{Key: "component", Value: c.Name},
			logging.Field{Key: "backend", Value: cfg.BackendType},
		)
	} else {
		adapterCtx.logger.Debug(
			"no terraform backend detected; local terraform state artifacts will be captured",
			logging.Field{Key: "component", Value: c.Name},
		)
	}

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

func (d *TerraformAdapter) Apply(ctx context.Context, c *manifestcore.Component, t runners.Runner) (ComponentApplyResult, error) {
	cfg, ok := c.LoadedConfig.(*TerraformConfig)
	if !ok {
		return ComponentApplyResult{}, fmt.Errorf("invalid config type for TerraformAdapter")
	}

	aCtx, ok := AdapterContextFromContext(ctx)
	if !ok {
		return ComponentApplyResult{}, fmt.Errorf("failed to get adapter context")
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
			return ComponentApplyResult{}, fmt.Errorf("failed to copy terraform with-file %q to runner: %w", name, err)
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

	if err := d.copyModuleToRunner(ctx, t, cfg.ModulePath, workDir, c.Type, c.Runner, c.Name); err != nil {
		return ComponentApplyResult{}, err
	}

	// Execute terraform init on runner
	aCtx.logger.Debug("executing terraform init", logging.Field{Key: "workdir", Value: workDir})
	if err := d.init(ctx, t, workDir, c.Env, c.Runner, c.Name); err != nil {
		return ComponentApplyResult{}, err
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
		return ComponentApplyResult{}, fmt.Errorf("failed to execute terraform apply: %w", err)
	}
	if applyRes.Error != nil || applyRes.ExitCode != 0 {
		return ComponentApplyResult{}, fmt.Errorf("terraform apply failed with exit code %d: %v", applyRes.ExitCode, applyRes.Error)
	}

	outputs, err := d.outputs(ctx, t, workDir, c.Env, c.Runner, c.Name)
	if err != nil {
		return ComponentApplyResult{}, err
	}
	stateData, err := d.buildState(c, workDir)
	if err != nil {
		return ComponentApplyResult{}, err
	}

	return ComponentApplyResult{
		Outputs: outputs,
		State:   stateData,
	}, nil
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

func (d *TerraformAdapter) buildState(c *manifestcore.Component, workDir string) (state.ComponentStateData, error) {
	cfg, ok := c.LoadedConfig.(*TerraformConfig)
	if !ok {
		return state.ComponentStateData{}, fmt.Errorf("invalid config type for TerraformAdapter")
	}

	vars := make(map[string]string, len(cfg.Vars))
	for key, value := range cfg.Vars {
		vars[key] = value
	}

	terraformState := TerraformState{
		Vars:    vars,
		WorkDir: workDir,
	}

	if cfg.HasBackend {
		return state.NewComponentStateData(workDir, terraformState)
	}

	return state.NewComponentStateData(
		workDir,
		terraformState,
		state.Artifact{
			Name:      "terraform-state",
			Path:      "terraform.tfstate",
			Required:  true,
			Sensitive: true,
		},
		state.Artifact{
			Name:      "terraform-state-backup",
			Path:      "terraform.tfstate.backup",
			Sensitive: true,
		},
		state.Artifact{
			Name: ".terraform.lock.hcl",
			Path: ".terraform.lock.hcl",
		},
	)
}

func (d *TerraformAdapter) DestroyFromState(ctx context.Context, componentState state.ComponentState, t runners.Runner) error {
	var s TerraformState
	if err := mapstructure.Decode(componentState.Payload, &s); err != nil {
		return fmt.Errorf("failed to decode terraform state: %w", err)
	}
	if s.WorkDir == "" {
		return fmt.Errorf("terraform state for component %q has no workdir", componentState.Name)
	}

	aCtx, ok := AdapterContextFromContext(ctx)
	if !ok {
		return fmt.Errorf("failed to get adapter context")
	}

	modulePath, err := loadAndGetTerraformModulePath(
		componentState.Source,
		aCtx.GetComponentWorkDirInOrchLocalWorkDir(componentState.Name),
	)
	if err != nil {
		return fmt.Errorf("failed to resolve terraform module path for destroy: %w", err)
	}

	if err := d.copyModuleToRunner(ctx, t, modulePath, s.WorkDir, componentState.Type, componentState.Runner.Name, componentState.Name); err != nil {
		return err
	}

	aCtx.logger.Debug("executing terraform init", logging.Field{Key: "workdir", Value: s.WorkDir})
	if err := d.init(ctx, t, s.WorkDir, nil, componentState.Runner.Name, componentState.Name); err != nil {
		return err
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

type terraformOutputValue struct {
	Sensitive bool        `json:"sensitive"`
	Value     interface{} `json:"value"`
}

func (d *TerraformAdapter) outputs(ctx context.Context, t runners.Runner, workDir string, env map[string]string, runnerName string, componentName string) (ComponentApplyOutput, error) {
	outputRes, err := t.Exec(ctx, runners.ExecCommand{
		WorkingDir: workDir,
		Command:    []string{"terraform", "output", "-json"},
		Env:        env,
		Timeout:    0,
		Stderr:     utils.NewPrefixWriter(os.Stderr, utils.RunnerComponentPrefix(runnerName, componentName)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute terraform output: %w", err)
	}
	if outputRes.Error != nil || outputRes.ExitCode != 0 {
		return nil, fmt.Errorf("terraform output failed with exit code %d: %v", outputRes.ExitCode, outputRes.Error)
	}

	outputs, err := parseTerraformOutputs(outputRes.Stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to parse terraform output JSON: %w", err)
	}
	return outputs, nil
}

func parseTerraformOutputs(data []byte) (ComponentApplyOutput, error) {
	var raw map[string]terraformOutputValue
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	outputs := make(ComponentApplyOutput, len(raw))
	for key, output := range raw {
		if output.Sensitive {
			continue
		}
		value, err := adaptersupport.StringifyOutputValue(output.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert terraform output %q: %w", key, err)
		}
		outputs[key] = value
	}
	return outputs, nil
}

func (d *TerraformAdapter) copyModuleToRunner(ctx context.Context, t runners.Runner, modulePath string, workDir string, adapterName string, runnerName string, componentName string) error {
	stagedModulePath, cleanup, err := stageTerraformSource(modulePath)
	if err != nil {
		return err
	}
	defer cleanup()

	copyRes, err := t.CopyFile(ctx, runners.FileCopyRequest{
		Source:      stagedModulePath,
		Destination: workDir,
		ToRunner:    true,
		Overwrite:   true,
		Recursive:   true,
	})
	if err != nil {
		return fmt.Errorf("failed to copy terraform module to runner: %w", err)
	}

	if copyRes.Error != nil {
		return fmt.Errorf("error during terraform module copy: %w", copyRes.Error)
	}

	if aCtx, ok := AdapterContextFromContext(ctx); ok {
		aCtx.emitter.Emit(events.Event{
			Type:      events.EventInfo,
			Message:   fmt.Sprintf("Copied terraform module to %q", workDir),
			Adapter:   adapterName,
			Runner:    runnerName,
			Component: componentName,
			Duration:  copyRes.Duration,
		})
	}

	return nil
}

func stageTerraformSource(modulePath string) (string, func(), error) {
	// Destroy restores tool artifacts before copying source back to the runner.
	// Stage a source-only module copy so stale local tfstate or .terraform data
	// from the input module cannot overwrite the restored artifacts.
	tmpDir, err := os.MkdirTemp("", "orch-terraform-module-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to create temporary terraform module directory: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	if err := filepath.WalkDir(modulePath, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(modulePath, currentPath)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		if shouldSkipTerraformSourcePath(entry, relPath) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		destination := filepath.Join(tmpDir, relPath)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0755)
		}

		return copyLocalFile(currentPath, destination)
	}); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("failed to stage terraform module: %w", err)
	}

	return tmpDir, cleanup, nil
}

func shouldSkipTerraformSourcePath(entry os.DirEntry, relPath string) bool {
	if entry.IsDir() && entry.Name() == ".terraform" {
		return true
	}

	switch filepath.ToSlash(relPath) {
	case "terraform.tfstate", "terraform.tfstate.backup":
		return true
	default:
		return false
	}
}

func copyLocalFile(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}

	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() {
		_ = src.Close()
	}()

	dst, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer func() {
		_ = dst.Close()
	}()

	_, err = io.Copy(dst, src)
	return err
}

func detectTerraformBackend(modulePath string) (bool, string, error) {
	entries, err := os.ReadDir(modulePath)
	if err != nil {
		return false, "", err
	}

	parser := hclparse.NewParser()
	for _, entry := range entries {
		if entry.IsDir() || !isTerraformConfigFile(entry.Name()) {
			continue
		}

		filePath := filepath.Join(modulePath, entry.Name())
		var parseDiags hcl.Diagnostics
		var file *hcl.File
		if strings.HasSuffix(entry.Name(), ".tf.json") {
			file, parseDiags = parser.ParseJSONFile(filePath)
		} else {
			file, parseDiags = parser.ParseHCLFile(filePath)
		}
		if parseDiags.HasErrors() {
			return false, "", fmt.Errorf("failed to parse %s: %s", filePath, parseDiags.Error())
		}

		content, _, contentDiags := file.Body.PartialContent(&hcl.BodySchema{
			Blocks: []hcl.BlockHeaderSchema{{Type: "terraform"}},
		})
		if contentDiags.HasErrors() {
			return false, "", fmt.Errorf("failed to inspect %s: %s", filePath, contentDiags.Error())
		}

		for _, terraformBlock := range content.Blocks {
			backendContent, _, backendDiags := terraformBlock.Body.PartialContent(&hcl.BodySchema{
				Blocks: []hcl.BlockHeaderSchema{
					{Type: "backend", LabelNames: []string{"type"}},
				},
			})
			if backendDiags.HasErrors() {
				return false, "", fmt.Errorf("failed to inspect backend block in %s: %s", filePath, backendDiags.Error())
			}
			for _, backendBlock := range backendContent.Blocks {
				if len(backendBlock.Labels) > 0 {
					return true, backendBlock.Labels[0], nil
				}
				return true, "unknown", nil
			}
		}
	}

	return false, "", nil
}

func isTerraformConfigFile(name string) bool {
	return strings.HasSuffix(name, ".tf") || strings.HasSuffix(name, ".tf.json")
}

func (d *TerraformAdapter) init(ctx context.Context, t runners.Runner, workDir string, env map[string]string, runnerName string, componentName string) error {
	initRes, err := t.Exec(ctx, runners.ExecCommand{
		WorkingDir: workDir,
		Command:    []string{"terraform", "init", "-upgrade"},
		Env:        env,
		Timeout:    0,
		Stderr:     utils.NewPrefixWriter(os.Stderr, utils.RunnerComponentPrefix(runnerName, componentName)),
		Stdout:     utils.NewPrefixWriter(os.Stdout, utils.RunnerComponentPrefix(runnerName, componentName)),
	})
	if err != nil {
		return fmt.Errorf("failed to execute terraform init: %w", err)
	}
	if initRes.Error != nil || initRes.ExitCode != 0 {
		return fmt.Errorf("terraform init failed with exit code %d: %v", initRes.ExitCode, initRes.Error)
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
