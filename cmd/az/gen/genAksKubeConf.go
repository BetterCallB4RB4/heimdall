package gen

import (
	"github.com/BetterCallB4RB4/heimdall/pkg/providers/azure"
	"github.com/spf13/cobra"
)

var GenAksKubeConfCmd = &cobra.Command{
	Use:   "kubeConf",
	Short: "Write a kubeconfig for an AKS cluster",
	Long:  "Lists AKS clusters in the active subscription, presents an fzf picker, then runs az aks get-credentials and kubelogin conversion.",
	Run: func(cmd *cobra.Command, args []string) {
		azure.GenAksKubeConfig()
	},
}

func init() {}
