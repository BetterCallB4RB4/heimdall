package cmd

import (
	"os"

	"github.com/BetterCallB4RB4/heimdall/cmd/aws"
	"github.com/BetterCallB4RB4/heimdall/cmd/az"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "heimdall",
	Version: version,
	Short:   "Cloud session and kubeconfig manager for AWS and Azure",
	Long: `heimdall manages cloud sessions and Kubernetes configs for AWS and Azure.

It handles SSO authentication, subscription and profile selection, and writes
kubeconfig files for EKS and AKS clusters — all scoped to the current shell
via a generated shell-integration script.

  heimdall aws   — AWS profile selection, SSO login, EKS kubeconfig generation
  heimdall az    — Azure tenant/subscription selection, AKS kubeconfig generation
  heimdall getSunny — clear all cloud context from the current shell`,
}

// Execute is the entry point called by main. It runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(az.AzCmd)
	rootCmd.AddCommand(aws.AwsCmd)
}
