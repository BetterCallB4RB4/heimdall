package login

import (
	"github.com/BetterCallB4RB4/heimdall/pkg/providers/azure"
	"github.com/BetterCallB4RB4/heimdall/pkg/ui"
	"github.com/BetterCallB4RB4/heimdall/pkg/utils"
	"github.com/spf13/cobra"
)

var AzLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Azure and select a subscription",
	Long:  "Run browser SSO login, pick a tenant via fzf, then pick a subscription. Exports AZURE_CONFIG_DIR scoped to this shell.",
	Run: func(cmd *cobra.Command, args []string) {
		// Step 1: browser auth (if needed) → fzf tenant picker → per-tenant login.
		if err := azure.TriggerSSOLoginAzure(); err != nil {
			ui.Error("Login failed: %v", err)
			return
		}

		// Step 2: fzf subscription picker within the active tenant.
		tenantID := azure.GetCurrentTenantID()
		if tenantID == "" {
			ui.Error("No tenant active after login.")
			return
		}

		sub, err := azure.SelectAndSetSubscription(tenantID)
		if err != nil {
			ui.Error("Subscription selection failed: %v", err)
			return
		}

		utils.AddScriptEntry("unset KUBECONFIG")
		ui.Success("Subscription set to: %s", sub.Name)
	},
}

func init() {}
