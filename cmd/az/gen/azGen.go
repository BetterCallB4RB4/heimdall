package gen

import (
	"github.com/spf13/cobra"
)

var AzGenCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate Azure config artifacts",
	Long:  "Generate kubeconfig files for AKS clusters.",
	Run:   func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
}

func init() {
	AzGenCmd.AddCommand(GenAksKubeConfCmd)
}
