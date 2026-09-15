package aws

import (
	"github.com/BetterCallB4RB4/heimdall/cmd/aws/gen"
	"github.com/BetterCallB4RB4/heimdall/cmd/aws/login"
	"github.com/BetterCallB4RB4/heimdall/cmd/aws/sel"
	"github.com/spf13/cobra"
)

var AwsCmd = &cobra.Command{
	Use:   "aws",
	Short: "AWS helpers (select profile or run subcommands)",
	Long:  "Manage AWS profiles, SSO sessions, and EKS kubeconfig files.",
	Run:   func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
}

func init() {
	AwsCmd.AddCommand(sel.AwsSelCmd)
	AwsCmd.AddCommand(login.AwsLoginCmd)
	AwsCmd.AddCommand(gen.AwsGenCmd)
	AwsCmd.AddCommand(login.GetSunnyCmd)
}
