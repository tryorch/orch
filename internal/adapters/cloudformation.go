package adapters

import (
	"context"
	"fmt"
	"os"
	"path"
	"sort"

	"github.com/go-viper/mapstructure/v2"
	"orch.io/pkg/events"
	manifestcore "orch.io/pkg/manifest/core"
	"orch.io/pkg/runners"
	"orch.io/pkg/state"
	"orch.io/pkg/utils"
)

type CloudFormationAdapter struct{}

type CloudFormationConfig struct {
	StackName    string            `mapstructure:"stack_name"`
	Region       string            `mapstructure:"region"`
	Parameters   map[string]string `mapstructure:"parameters"`
	Capabilities []string          `mapstructure:"capabilities"`
	Tags         map[string]string `mapstructure:"tags"`
	RoleARN      string            `mapstructure:"role_arn"`

	TemplatePath string
}

type CloudFormationState struct {
	Region       string `mapstructure:"region" json:"region,omitempty"`
	StackName    string `mapstructure:"stack_name" json:"stack_name"`
	TemplateFile string `mapstructure:"template_file" json:"template_file"`
	WorkDir      string `mapstructure:"workdir" json:"workdir"`
}

func (d *CloudFormationAdapter) RequiredCapabilities() runners.Capabilities {
	return runners.Capabilities{Exec: true, FileCopy: true}
}

func (d *CloudFormationAdapter) SupportedSources() ComponentSourceSupport {
	return ComponentSourceSupport{
		Embedded: true,
		Files:    true,
	}
}

func (d *CloudFormationAdapter) ValidateAndLoadConfig(ctx context.Context, c *manifestcore.Component) (ComponentConfig, []events.Event, error) {
	var cfg CloudFormationConfig
	if err := mapstructure.Decode(c.Config, &cfg); err != nil {
		return nil, nil, err
	}

	aCtx, ok := AdapterContextFromContext(ctx)
	if !ok {
		return nil, nil, fmt.Errorf("failed to get adapter context")
	}

	if cfg.StackName == "" {
		cfg.StackName = fmt.Sprintf("orch-%s-%s", aCtx.EnvID(), c.Name)
	}

	templatePath, err := d.localTemplatePath(aCtx, c)
	if err != nil {
		return nil, nil, err
	}
	cfg.TemplatePath = templatePath

	return &cfg, nil, nil
}

func (d *CloudFormationAdapter) Apply(ctx context.Context, c *manifestcore.Component, t runners.Runner) (ComponentApplyOutput, error) {
	cfg, ok := c.LoadedConfig.(*CloudFormationConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type for CloudFormationAdapter")
	}

	aCtx, ok := AdapterContextFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("failed to get adapter context")
	}

	workDir := aCtx.BuildRunnerWorkDir(c.WorkDir, c.Name)
	templateFile := path.Base(cfg.TemplatePath)
	if err := d.copyTemplate(ctx, c, t, cfg.TemplatePath, path.Join(workDir, templateFile)); err != nil {
		return nil, err
	}

	cmd := d.deployCommand(cfg, templateFile)
	if err := d.execAWS(ctx, t, workDir, cmd, c.Runner, c.Name, "deploy"); err != nil {
		return nil, err
	}

	return make(ComponentApplyOutput), nil
}

func (d *CloudFormationAdapter) Destroy(ctx context.Context, c *manifestcore.Component, t runners.Runner) error {
	cfg, ok := c.LoadedConfig.(*CloudFormationConfig)
	if !ok {
		return fmt.Errorf("invalid config type for CloudFormationAdapter")
	}

	aCtx, ok := AdapterContextFromContext(ctx)
	if !ok {
		return fmt.Errorf("failed to get adapter context")
	}

	return d.deleteStack(ctx, CloudFormationState{
		Region:    cfg.Region,
		StackName: cfg.StackName,
		WorkDir:   aCtx.BuildRunnerWorkDir(c.WorkDir, c.Name),
	}, c.Runner, c.Name, t)
}

func (d *CloudFormationAdapter) BuildState(ctx context.Context, c *manifestcore.Component, t runners.Runner, outputs ComponentApplyOutput) (state.ComponentStateData, error) {
	cfg, ok := c.LoadedConfig.(*CloudFormationConfig)
	if !ok {
		return state.ComponentStateData{}, fmt.Errorf("invalid config type for CloudFormationAdapter")
	}

	aCtx, ok := AdapterContextFromContext(ctx)
	if !ok {
		return state.ComponentStateData{}, fmt.Errorf("failed to get adapter context")
	}

	workDir := aCtx.BuildRunnerWorkDir(c.WorkDir, c.Name)
	cfState := CloudFormationState{
		Region:       cfg.Region,
		StackName:    cfg.StackName,
		TemplateFile: path.Base(cfg.TemplatePath),
		WorkDir:      workDir,
	}

	return state.NewComponentStateData(workDir, cfState)
}

func (d *CloudFormationAdapter) DestroyFromState(ctx context.Context, componentState state.ComponentState, t runners.Runner) error {
	var s CloudFormationState
	if err := mapstructure.Decode(componentState.Payload, &s); err != nil {
		return fmt.Errorf("failed to decode CloudFormation state: %w", err)
	}
	if s.StackName == "" {
		return fmt.Errorf("CloudFormation state for component %q has no stack_name", componentState.Name)
	}
	if s.WorkDir == "" {
		return fmt.Errorf("CloudFormation state for component %q has no workdir", componentState.Name)
	}

	return d.deleteStack(ctx, s, componentState.Runner.Name, componentState.Name, t)
}

func (d *CloudFormationAdapter) localTemplatePath(aCtx AdapterContext, c *manifestcore.Component) (string, error) {
	if c.Source.Embedded != "" {
		dir := path.Join(aCtx.GetComponentWorkDirInOrchLocalWorkDir(c.Name), "cloudformation")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create local CloudFormation template directory: %w", err)
		}

		templatePath := path.Join(dir, "template.yml")
		if err := os.WriteFile(templatePath, []byte(c.Source.Embedded), 0644); err != nil {
			return "", fmt.Errorf("failed to write embedded CloudFormation template: %w", err)
		}
		return templatePath, nil
	}

	if len(c.Source.Files) != 1 {
		return "", fmt.Errorf("CloudFormation file source requires exactly one template file, got %d", len(c.Source.Files))
	}

	return c.Source.Files[0], nil
}

func (d *CloudFormationAdapter) copyTemplate(ctx context.Context, c *manifestcore.Component, t runners.Runner, source, destination string) error {
	copyRes, err := t.CopyFile(ctx, runners.FileCopyRequest{
		Source:      source,
		Destination: destination,
		ToRunner:    true,
		Overwrite:   true,
		Recursive:   false,
	})
	if err != nil {
		return fmt.Errorf("failed to copy CloudFormation template to runner: %w", err)
	}
	if copyRes.Error != nil {
		return fmt.Errorf("error during CloudFormation template copy: %w", copyRes.Error)
	}

	if aCtx, ok := AdapterContextFromContext(ctx); ok {
		aCtx.emitter.Emit(events.Event{
			Type:      events.EventInfo,
			Message:   fmt.Sprintf("Copied CloudFormation template to %q", destination),
			Adapter:   c.Type,
			Runner:    c.Runner,
			Component: c.Name,
			Duration:  copyRes.Duration,
		})
	}

	return nil
}

func (d *CloudFormationAdapter) deployCommand(cfg *CloudFormationConfig, templateFile string) []string {
	cmd := []string{
		"aws", "cloudformation", "deploy",
		"--stack-name", cfg.StackName,
		"--template-file", templateFile,
		"--no-fail-on-empty-changeset",
	}

	if cfg.Region != "" {
		cmd = append(cmd, "--region", cfg.Region)
	}
	if cfg.RoleARN != "" {
		cmd = append(cmd, "--role-arn", cfg.RoleARN)
	}
	if len(cfg.Capabilities) > 0 {
		cmd = append(cmd, "--capabilities")
		cmd = append(cmd, cfg.Capabilities...)
	}
	if len(cfg.Parameters) > 0 {
		cmd = append(cmd, "--parameter-overrides")
		for _, key := range sortedKeys(cfg.Parameters) {
			cmd = append(cmd, fmt.Sprintf("%s=%s", key, cfg.Parameters[key]))
		}
	}
	if len(cfg.Tags) > 0 {
		cmd = append(cmd, "--tags")
		for _, key := range sortedKeys(cfg.Tags) {
			cmd = append(cmd, fmt.Sprintf("%s=%s", key, cfg.Tags[key]))
		}
	}

	return cmd
}

func (d *CloudFormationAdapter) deleteStack(ctx context.Context, s CloudFormationState, runnerName, componentName string, t runners.Runner) error {
	deleteCmd := []string{"aws", "cloudformation", "delete-stack", "--stack-name", s.StackName}
	waitCmd := []string{"aws", "cloudformation", "wait", "stack-delete-complete", "--stack-name", s.StackName}
	if s.Region != "" {
		deleteCmd = append(deleteCmd, "--region", s.Region)
		waitCmd = append(waitCmd, "--region", s.Region)
	}

	if err := d.execAWS(ctx, t, s.WorkDir, deleteCmd, runnerName, componentName, "delete-stack"); err != nil {
		return err
	}
	if err := d.execAWS(ctx, t, s.WorkDir, waitCmd, runnerName, componentName, "wait stack-delete-complete"); err != nil {
		return err
	}

	return nil
}

func (d *CloudFormationAdapter) execAWS(ctx context.Context, t runners.Runner, workDir string, cmd []string, runnerName, componentName, action string) error {
	execRes, err := t.Exec(ctx, runners.ExecCommand{
		WorkingDir: workDir,
		Command:    cmd,
		Timeout:    0,
		Stderr:     utils.NewPrefixWriter(os.Stderr, utils.RunnerComponentPrefix(runnerName, componentName)),
		Stdout:     utils.NewPrefixWriter(os.Stdout, utils.RunnerComponentPrefix(runnerName, componentName)),
	})
	if err != nil {
		return fmt.Errorf("failed to execute CloudFormation %s: %w", action, err)
	}
	if execRes.Error != nil || execRes.ExitCode != 0 {
		return fmt.Errorf("CloudFormation %s failed with exit code %d: %v", action, execRes.ExitCode, execRes.Error)
	}
	return nil
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func init() {
	Register("cloudformation", &CloudFormationAdapter{})
}
