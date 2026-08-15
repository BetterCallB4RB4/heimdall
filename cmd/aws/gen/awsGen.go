package gen

import (
	"github.com/spf13/cobra"
)

var AwsGenCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate AWS config artifacts",
	Long:  "Generate SSO profiles in ~/.aws/config or kubeconfig files for EKS clusters.",
	Run:   func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
}

func init() {
	AwsGenCmd.AddCommand(GenerateAwsSsoProfileCmd)
	AwsGenCmd.AddCommand(GenEksKubeConfCmd)
}
