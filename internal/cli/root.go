package cli

import (
	"fmt"
	"os"

	"walkscape-helper/internal/output"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version = "0.1.0"
	commit  = "none"
	date    = "unknown"
)

func appVersion() string {
	return version
}

func versionLabel() string {
	if commit == "none" && date == "unknown" {
		return appVersion()
	}
	return fmt.Sprintf("%s (commit: %s, built: %s)", appVersion(), commit, date)
}

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "wsh",
		Short: "Walkscape Helper CLI",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			viper.SetEnvPrefix("WSH")
			viper.AutomaticEnv()

			if !cmd.Flags().Changed("db-path") {
				dbPath := viper.GetString("db_path")
				if dbPath != "" {
					if err := cmd.Flags().Set("db-path", dbPath); err != nil {
						return err
					}
				}
			}

			if !cmd.Flags().Changed("json") {
				if viper.GetBool("json") {
					if err := cmd.Flags().Set("json", "true"); err != nil {
						return err
					}
				}
			}

			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.SetErrPrefix("")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return output.NewError("validation_error", err.Error(), nil)
	})

	root.PersistentFlags().Bool("json", false, "output as json")
	root.PersistentFlags().Bool("verbose", false, "enable verbose logging")
	root.PersistentFlags().String("db-path", defaultDBPath(), "path to sqlite database")

	root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		_ = args
		ctx := contextFromCommand(cmd)
		if ctx.JSON {
			cmd.SetOut(os.Stdout)
			cmd.SetErr(os.Stderr)
		}
	}

	root.AddCommand(newGuideCmd())
	root.AddCommand(newCharacterCmd())
	root.AddCommand(newVersionCmd())

	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)

	root.SetHelpCommand(&cobra.Command{Hidden: true})

	root.SetVersionTemplate(fmt.Sprintf("%s\n", versionLabel()))
	root.Version = appVersion()

	return root
}

func commandMeta(cmd string) map[string]any {
	return map[string]any{
		"command": cmd,
		"version": appVersion(),
		"commit":  commit,
		"date":    date,
	}
}
