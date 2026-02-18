package cli

import "github.com/spf13/cobra"

func newCompletionCmd(root *cobra.Command) *cobra.Command {
	var noDescriptions bool

	cmd := &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "Generate shell completion scripts",
		Long:      "Generate shell completion scripts for wsh.",
		Example:   "  wsh completion bash > ~/.local/share/bash-completion/completions/wsh\n  wsh completion zsh > ~/.zsh/completions/_wsh\n  wsh completion fish > ~/.config/fish/completions/wsh.fish\n  wsh completion powershell > wsh.ps1",
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.ExactValidArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(cmd.OutOrStdout(), !noDescriptions)
			case "zsh":
				if noDescriptions {
					return root.GenZshCompletionNoDesc(cmd.OutOrStdout())
				}
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), !noDescriptions)
			case "powershell":
				if noDescriptions {
					return root.GenPowerShellCompletion(cmd.OutOrStdout())
				}
				return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "disable completion descriptions")

	return cmd
}
