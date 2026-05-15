package main

import (
	"fmt"
	"os"

	"orch.io/internal/orchestration"
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

			return orchestration.RunUp(envID, m, logger.With(
				logging.Field{Key: "command", Value: "up"},
				logging.Field{Key: "manifest", Value: manifestPath},
			), params.Merge())
		},
	}

	upCmd.PersistentFlags().StringVarP(&manifestPath, "file", "f", "orch.yaml", "Path to manifest")
	upCmd.PersistentFlags().StringArrayVar(&cliParams, "param", []string{}, "Secret in key=value format (repeatable)")
	upCmd.PersistentFlags().StringVar(&paramsFile, "params-file", "", "Path to YAML or env params file")
	upCmd.PersistentFlags().StringVarP(&envID, "env-id", "e", "", "Environment ID")
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
			return orchestration.RunDown(envID, m, logger.With(
				logging.Field{Key: "command", Value: "down"},
				logging.Field{Key: "manifest", Value: manifestPath},
			))
		},
	}
	downCmd.PersistentFlags().StringVarP(&manifestPath, "file", "f", "orch.yaml", "Path to manifest")
	downCmd.PersistentFlags().StringVarP(&envID, "env-id", "e", "", "Environment ID")
	_ = downCmd.MarkPersistentFlagRequired("env-id")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version.String())
		},
	}

	rootCmd.AddCommand(upCmd, downCmd, versionCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
