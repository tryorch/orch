package main

import (
	"fmt"
	"os"

	"orch.io/internal/orchestration"
	"orch.io/internal/scaffold"
	"orch.io/pkg/logging"
	"orch.io/pkg/manifest"
	"orch.io/pkg/utils"
	"orch.io/pkg/version"

	"github.com/spf13/cobra"
)

// Logger is initialized in PersistentPreRunE
// because it depends on parsed flags and TTY detection.
var logger logging.Logger

func main() {
	var manifestPath string
	var cliParams []string
	var paramsFile string
	var isDebug bool
	var envID string
	var reapply bool
	var stateInspectOutput string
	var initForce bool
	var initID string

	rootCmd := &cobra.Command{
		Use:           "orch",
		Short:         "Orch — ephemeral sandbox orchestrator",
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	rootCmd.PersistentFlags().BoolVar(&isDebug, "debug", false, "Enable debug logging")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		isTTY := utils.IsTTY()

		zl, err := logging.NewRootZapLogger(isTTY, isDebug)
		if err != nil {
			return err
		}

		logger = logging.NewZapLogger(zl)

		return nil
	}

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Create a starter Orch manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := scaffold.RunInit(scaffold.InitOptions{
				Path:  manifestPath,
				ID:    initID,
				Force: initForce,
			}); err != nil {
				return err
			}
			fmt.Printf("Created %s\n", manifestPath)
			return nil
		},
	}
	initCmd.PersistentFlags().StringVarP(&manifestPath, "file", "f", "orch.yaml", "Path to manifest")
	initCmd.PersistentFlags().StringVar(&initID, "id", "", "Manifest metadata ID (defaults to current directory name)")
	initCmd.PersistentFlags().BoolVar(&initForce, "force", false, "Overwrite an existing manifest")

	upCmd := &cobra.Command{
		Use:   "up",
		Short: "Provision resources defined in manifest",
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := utils.ValidateEnvID(envID); err != nil {
				return fmt.Errorf("invalid env-id: %w", err)
			}

			m, err := manifest.Load(manifestPath, logger)
			if err != nil {
				return err
			}
			params, err := LoadParameters(paramsFile, cliParams)
			if err != nil {
				return err
			}

			return orchestration.RunUpWithOptions(envID, m, logger.With(
				logging.Field{Key: "command", Value: "up"},
				logging.Field{Key: "manifest", Value: manifestPath},
			), params.Merge(), orchestration.UpOptions{
				Reapply: reapply,
			})
		},
	}

	upCmd.PersistentFlags().StringVarP(&manifestPath, "file", "f", "orch.yaml", "Path to manifest")
	upCmd.PersistentFlags().StringArrayVar(&cliParams, "param", []string{}, "Secret in key=value format (repeatable)")
	upCmd.PersistentFlags().StringVar(&paramsFile, "params-file", "", "Path to YAML or env params file")
	upCmd.PersistentFlags().StringVarP(&envID, "env-id", "e", "", "Environment ID")
	upCmd.PersistentFlags().BoolVar(&reapply, "reapply", false, "Re-run apply for components already marked applied")
	_ = upCmd.MarkPersistentFlagRequired("env-id")

	downCmd := &cobra.Command{
		Use:   "down",
		Short: "Tear down resources from last run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := utils.ValidateEnvID(envID); err != nil {
				return fmt.Errorf("invalid env-id: %w", err)
			}

			m, err := manifest.Load(manifestPath, logger)
			if err != nil {
				return err
			}
			params, err := LoadParameters(paramsFile, cliParams)
			if err != nil {
				return err
			}
			return orchestration.RunDown(envID, m, logger.With(
				logging.Field{Key: "command", Value: "down"},
				logging.Field{Key: "manifest", Value: manifestPath},
			), params.Merge())
		},
	}
	downCmd.PersistentFlags().StringVarP(&manifestPath, "file", "f", "orch.yaml", "Path to manifest")
	downCmd.PersistentFlags().StringArrayVar(&cliParams, "param", []string{}, "Secret in key=value format (repeatable)")
	downCmd.PersistentFlags().StringVar(&paramsFile, "params-file", "", "Path to YAML or env params file")
	downCmd.PersistentFlags().StringVarP(&envID, "env-id", "e", "", "Environment ID")
	_ = downCmd.MarkPersistentFlagRequired("env-id")

	stateCmd := &cobra.Command{
		Use:   "state",
		Short: "Inspect and manage Orch state",
	}

	stateInspectCmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect persisted state for an environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := utils.ValidateEnvID(envID); err != nil {
				return fmt.Errorf("invalid env-id: %w", err)
			}

			m, err := manifest.Load(manifestPath, logger)
			if err != nil {
				return err
			}

			return orchestration.RunStateInspect(envID, m, logger.With(
				logging.Field{Key: "command", Value: "state inspect"},
				logging.Field{Key: "manifest", Value: manifestPath},
			), orchestration.StateInspectOptions{
				Output: stateInspectOutput,
				Writer: os.Stdout,
			})
		},
	}
	stateInspectCmd.PersistentFlags().StringVarP(&manifestPath, "file", "f", "orch.yaml", "Path to manifest")
	stateInspectCmd.PersistentFlags().StringVarP(&envID, "env-id", "e", "", "Environment ID")
	stateInspectCmd.PersistentFlags().StringVarP(&stateInspectOutput, "output", "o", "table", "Output format: table or json")
	_ = stateInspectCmd.MarkPersistentFlagRequired("env-id")
	stateCmd.AddCommand(stateInspectCmd)

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version.String())
		},
	}

	rootCmd.AddCommand(initCmd, upCmd, downCmd, stateCmd, versionCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
