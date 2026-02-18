package cli

import (
	"walkscape-helper/internal/output"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := contextFromCommand(cmd)
			if ctx.JSON {
				return output.WriteJSONSuccess(cmd.OutOrStdout(), map[string]any{
					"version": appVersion(),
					"commit":  commit,
					"date":    date,
				}, commandMeta("version"))
			}
			return output.WriteHuman(cmd.OutOrStdout(), versionLabel())
		},
	}
}
