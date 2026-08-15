package az

import (
	"github.com/BetterCallB4RB4/heimdall/cmd/az/gen"
	"github.com/BetterCallB4RB4/heimdall/cmd/az/login"
	"github.com/BetterCallB4RB4/heimdall/cmd/az/sel"
	"github.com/spf13/cobra"
)

var AzCmd = &cobra.Command{
	Use:   "az",
	Short: "Azure helpers (login, select subscription, generate kubeconfig)",
	Long:  "Manage Azure sessions with per-tenant isolation. Use subcommands: login, select, gen.",
	Run:   func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
}

func init() {
	AzCmd.AddCommand(sel.AzSelectCmd)
	AzCmd.AddCommand(login.AzLoginCmd)
	AzCmd.AddCommand(gen.AzGenCmd)
	AzCmd.AddCommand(login.GetAzSunnyCmd)
}
