package adapters

import (
	"context"
	"fmt"
	"os/exec"
	"path"

	"github.com/go-viper/mapstructure/v2"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/hashicorp/terraform-exec/tfexec"
	"orch.io/pkg/logging"

	"orch.io/pkg/events"
	manifestcore "orch.io/pkg/manifest/core"
	"orch.io/pkg/runners"
	"orch.io/pkg/utils"
)

type TerraformAdapter struct{}

type TerraformConfig struct {
	WorkDir string            `mapstructure:"workdir"`
	Vars    map[string]string `mapstructure:"vars"`

	// ModulePath is the path to the Terraform module to be applied.
	ModulePath     string
	FoundVariables map[string]*tfconfig.Variable
}

func (d *TerraformAdapter) RequiredCapabilities() runners.Capabilities {
	return runners.Capabilities{FileCopy: true, Exec: true, API: true}
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

	compWorkDir := adapterCtx.GetComponentWorkDir(c.Name)
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

	return &cfg, warnings, nil
}

func (d *TerraformAdapter) Apply(ctx context.Context, c *manifestcore.Component, t runners.Runner) error {
	cfg, ok := c.LoadedConfig.(*TerraformConfig)
	if !ok {
		return fmt.Errorf("invalid config type for TerraformAdapter")
	}

	aCtx, ok := AdapterContextFromContext(ctx)
	if !ok {
		return fmt.Errorf("failed to get env ID from context")
	}

	workDir := path.Join(cfg.WorkDir, "orch", aCtx.envID, c.Name, "module")
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
			return fmt.Errorf("failed to copy terraform with-file %q to runner: %w", name, err)
		}
		if copyRes.Error != nil {
			return fmt.Errorf("error during with-file %q copy: %w", name, copyRes.Error)
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

	// Copy module files to workDir
	copyRes, err := t.CopyFile(ctx, runners.FileCopyRequest{
		Source:      cfg.ModulePath,
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

	aCtx.emitter.Emit(events.Event{
		Type:      events.EventInfo,
		Message:   fmt.Sprintf("Copied terraform module to %q", workDir),
		Adapter:   c.Type,
		Runner:    c.Runner,
		Component: c.Name,
		Duration:  copyRes.Duration,
	})

	execPath, err := getTerraformExecPath()
	aCtx.logger.Debug("found terraform executable", logging.Field{Key: "path", Value: execPath})

	if err != nil {
		return err
	}

	tf, err := tfexec.NewTerraform(cfg.ModulePath, execPath)
	if err != nil {
		return err
	}

	if err := tf.Init(ctx, tfexec.Upgrade(true)); err != nil {
		return err
	}

	aCtx.logger.Debug("terraform initialized", logging.Field{Key: "module", Value: cfg.ModulePath})
	var tfPlanVars []tfexec.PlanOption
	for k, v := range cfg.Vars {
		tfPlanVars = append(tfPlanVars, tfexec.Var(k+"="+v))
	}

	aCtx.logger.Debug("running Terraform plan")
	if _, err := tf.Plan(ctx, tfPlanVars...); err != nil {
		return err
	}

	var tfApplyVars []tfexec.ApplyOption
	for k, v := range cfg.Vars {
		tfApplyVars = append(tfApplyVars, tfexec.Var(k+"="+v))
	}

	aCtx.logger.Debug("applying Terraform changes")
	if err := tf.Apply(ctx, tfApplyVars...); err != nil {
		return err
	}

	_, _ = tf.Output(ctx)

	return nil
}

func (d *TerraformAdapter) Destroy(ctx context.Context, c *manifestcore.Component, t runners.Runner) error {
	cmd := exec.Command("docker-compose", "-f", c.Source.Path, "down")
	fmt.Printf("Running docker-compose down for %s...\n", c.Source.Path)
	return cmd.Run()
}

func init() {
	Register("terraform", &TerraformAdapter{})
}

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

func getTerraformExecPath() (string, error) {
	execPath, err := exec.LookPath("terraform")
	if err != nil {
		return "", fmt.Errorf("terraform executable not found in PATH: %w", err)
	}
	return execPath, nil
}
