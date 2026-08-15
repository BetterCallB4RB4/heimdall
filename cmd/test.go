package cmd

import (
	"sort"
	"strings"

	"github.com/BetterCallB4RB4/heimdall/pkg/providers/aws"
	"github.com/BetterCallB4RB4/heimdall/pkg/ui"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Dev sandbox: EKS cluster picker and kubeconfig writer",
	Run: func(cmd *cobra.Command, args []string) {
		// List all EKS clusters across every region in parallel, present them
		// in fzf, then print the selected cluster name.
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

func init() {
	rootCmd.AddCommand(testCmd)
}
