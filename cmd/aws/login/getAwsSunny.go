package login

import (
	"github.com/BetterCallB4RB4/heimdall/pkg/providers/aws"
	"github.com/BetterCallB4RB4/heimdall/pkg/ui"
	"github.com/spf13/cobra"
)

var GetSunnyCmd = &cobra.Command{
	Use:   "getSunny",
	Short: "Clear AWS context from the current shell",
	Long: `Unsets AWS_PROFILE and KUBECONFIG in the calling shell.

The underlying SSO session in ~/.aws/sso/cache/ is left intact so
future logins can reuse it without a new browser prompt.`,
	Run: func(cmd *cobra.Command, args []string) {
		aws.AwsGetSunny()

		ui.Box("AWS context cleared", [][2]string{
			{"AWS_PROFILE", "unset"},
			{"KUBECONFIG", "unset"},
		})
	},
}

func init() {}
