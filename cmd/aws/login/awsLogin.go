package login

import (
	"github.com/BetterCallB4RB4/heimdall/pkg/providers/aws"
	"github.com/spf13/cobra"
)

var AwsLoginCmd = &cobra.Command{
	Use:   "login [sso-session]",
	Short: "Authenticate an AWS SSO session",
	Long:  "Trigger an AWS SSO login flow. Pass a session name to skip the picker.",
	Run: func(cmd *cobra.Command, args []string) {
		var profile string
		if len(args) > 0 {
			profile = args[0]
		}
		aws.LoginAwsCalled(profile)
	},
}

func init() {}
