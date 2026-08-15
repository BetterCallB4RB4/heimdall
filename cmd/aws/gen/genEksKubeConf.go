package gen

import (
	"sort"
	"strings"

	"github.com/BetterCallB4RB4/heimdall/pkg/providers/aws"
	"github.com/BetterCallB4RB4/heimdall/pkg/ui"
	"github.com/spf13/cobra"
)

var GenEksKubeConfCmd = &cobra.Command{
	Use:   "kubeConf",
	Short: "Write a kubeconfig for an EKS cluster (all regions scan)",
	Long:  "Scans all AWS regions in parallel for EKS clusters, presents an fzf picker, then runs aws eks update-kubeconfig for the selected cluster.",
	Run: func(cmd *cobra.Command, args []string) {
		clusterToRegion := aws.ListEKSClustersByRegion("")

		// Build sorted list of "cluster: region" entries.
		entries := make([]string, 0, len(clusterToRegion))
		for cluster, region := range clusterToRegion {
			entries = append(entries, cluster+": "+region)
		}
		sort.Strings(entries)

		// Use fzf to select an entry and extract the cluster name.
		selected := ui.GetSelection(entries...)
		if selected == "" {
			return
		}

		clusterName, region, _ := strings.Cut(selected, ": ")
		aws.WriteEksKubeConfig("", region, clusterName)
	},
}

func init() {}
