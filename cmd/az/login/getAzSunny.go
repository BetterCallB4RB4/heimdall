package login

import (
	"github.com/BetterCallB4RB4/heimdall/pkg/providers/azure"
	"github.com/BetterCallB4RB4/heimdall/pkg/ui"
	"github.com/spf13/cobra"
)

var GetAzSunnyCmd = &cobra.Command{
	Use:   "getSunny",
	Short: "Clear Azure context from the current shell",
	Long: `Unsets AZURE_CONFIG_DIR and KUBECONFIG in the calling shell.

The underlying az session in the per-tenant directory is left intact so
future logins can reuse it without a new browser prompt.`,
	Run: func(cmd *cobra.Command, args []string) {
		azure.AzGetSunny()

		ui.Box("Azure context cleared", [][2]string{
			{"AZURE_CONFIG_DIR", "unset"},
			{"KUBECONFIG", "unset"},
		})
	},
}

func init() {}
