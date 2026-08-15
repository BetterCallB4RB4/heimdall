package cmd

import (
	"github.com/BetterCallB4RB4/heimdall/pkg/providers/aws"
	"github.com/BetterCallB4RB4/heimdall/pkg/providers/azure"
	"github.com/BetterCallB4RB4/heimdall/pkg/ui"
	"github.com/spf13/cobra"
)

var getSunnyCmd = &cobra.Command{
	Use:   "getSunny",
	Short: "Clear all cloud context from the current shell",
	Long: `Unsets AWS_PROFILE, AZURE_CONFIG_DIR, and KUBECONFIG in the calling shell.

Cached SSO sessions and per-tenant Azure directories are left intact so
future logins can reuse them without new browser prompts.

To clear a single provider only, use the provider-scoped variant:
  heimdall aws getSunny
  heimdall az  getSunny`,
	Run: func(cmd *cobra.Command, args []string) {
		azure.AzGetSunny()
		aws.AwsGetSunny()

		ui.Box("All cloud context cleared", [][2]string{
			{"AWS_PROFILE", "unset"},
			{"AZURE_CONFIG_DIR", "unset"},
			{"KUBECONFIG", "unset"},
		})
	},
}

func init() {
	rootCmd.AddCommand(getSunnyCmd)
}
