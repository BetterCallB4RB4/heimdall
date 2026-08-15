package aws

import (
	"github.com/BetterCallB4RB4/heimdall/cmd/aws/gen"
	"github.com/BetterCallB4RB4/heimdall/cmd/aws/login"
	"github.com/BetterCallB4RB4/heimdall/cmd/aws/sel"
	"github.com/spf13/cobra"
)

var AwsCmd = &cobra.Command{
	Use:   "aws [profile]",
	Short: "AWS helpers (select profile or run subcommands)",
	Long:  "Run `aws <profile>` to select a profile, or use subcommands like `aws gen`, `aws login`.",
	Args:  cobra.MaximumNArgs(1),
	Run:   func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
}

func init() {
	AwsCmd.AddCommand(sel.AwsSelCmd)
	AwsCmd.AddCommand(login.AwsLoginCmd)
	AwsCmd.AddCommand(gen.AwsGenCmd)
	AwsCmd.AddCommand(login.GetSunnyCmd)
}
