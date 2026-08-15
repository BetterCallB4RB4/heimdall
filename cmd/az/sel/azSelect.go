package sel

import (
	"fmt"
	"os"

	"github.com/BetterCallB4RB4/heimdall/pkg/providers/azure"
	"github.com/BetterCallB4RB4/heimdall/pkg/ui"
	"github.com/BetterCallB4RB4/heimdall/pkg/utils"
	"github.com/spf13/cobra"
)

var AzSelectCmd = &cobra.Command{
	Use:   "select [subscription]",
	Short: "Select an Azure subscription (reuse or refresh session as needed)",
	Long: `Pick an Azure subscription via fzf.

Three cases are handled automatically:
  a) No AZURE_CONFIG_DIR set  — reuses an existing per-tenant session on disk (fzf picker if multiple), or triggers a full browser login if none found.
  b) AZURE_CONFIG_DIR set but session expired — re-triggers browser login.
  c) AZURE_CONFIG_DIR set and valid — goes straight to the subscription picker.`,
	Run: func(cmd *cobra.Command, args []string) {
		// 1. Ensure a valid per-tenant session is active in this shell.
		if os.Getenv("AZURE_CONFIG_DIR") == "" {
			configDir, err := azure.PickExistingTenantSession()
			if err != nil {
				// No reusable session on disk — run the full browser login flow.
				ui.Info("No existing Azure session found. Triggering login...")
				if err := azure.TriggerSSOLoginAzure(); err != nil {
					ui.Error("Login failed: %v", err)
					return
				}
				if !azure.IsAzSessionValid() {
					ui.Error("Login did not produce a valid session. Aborting.")
					return
				}
			} else {
				// Reuse the existing per-tenant session — no browser needed.
				shellDir, err := azure.CreateShellConfigDir(configDir)
				if err != nil {
					ui.Error("Could not create shell config dir: %v", err)
					return
				}
				os.Setenv("AZURE_CONFIG_DIR", shellDir)
				utils.AddScriptEntry("unset AZURE_CONFIG_DIR")
				utils.AddScriptEntry(fmt.Sprintf("export AZURE_CONFIG_DIR=%s", shellDir))
			}
		} else if !azure.IsAzSessionValid() {
			ui.Warning("Azure session expired. Triggering login...")
			if err := azure.TriggerSSOLoginAzure(); err != nil {
				ui.Error("Login failed: %v", err)
				return
			}
			if !azure.IsAzSessionValid() {
				ui.Error("Login did not produce a valid session. Aborting.")
				return
			}
		}

		// 2. Check if a tenant is already active. If not, let the user pick one
		//    and re-authenticate scoped to it. This is a safety fallback —
		//    normally step 1 already established AZURE_CONFIG_DIR.
		tenantID := azure.GetCurrentTenantID()
		if tenantID == "" {
			ui.Info("No tenant selected. Please pick a tenant:")
			configDir, err := azure.SelectTenantAndLogin()
			if err != nil {
				ui.Error("Tenant selection failed: %v", err)
				return
			}
			shellDir, sErr := azure.CreateShellConfigDir(configDir)
			if sErr != nil {
				ui.Error("Could not create shell config dir: %v", sErr)
				return
			}
			os.Setenv("AZURE_CONFIG_DIR", shellDir)
			utils.AddScriptEntry("unset AZURE_CONFIG_DIR")
			utils.AddScriptEntry(fmt.Sprintf("export AZURE_CONFIG_DIR=%s", shellDir))
			tenantID = azure.GetCurrentTenantID()
			if tenantID == "" {
				ui.Error("Could not determine tenant after login. Aborting.")
				return
			}
		}

		// 3. Pick and set subscription via fzf (or match the CLI arg).
		if len(args) > 0 {
			subs, err := azure.ListAzureSubscriptionsByTenant(tenantID)
			if err != nil {
				ui.Error("Could not list subscriptions: %v", err)
				return
			}
			for _, s := range subs {
				if s.Name == args[0] || s.ID == args[0] {
					azure.SetAzureSubscription(s)
					utils.AddScriptEntry("unset KUBECONFIG")
					ui.Success("Subscription set to: %s", s.Name)
					if err := azure.EnsureManagementToken(tenantID); err != nil {
						ui.Error("Re-authentication failed: %v", err)
					}
					return
				}
			}
			ui.Error("Subscription %q not found in tenant.", args[0])
			return
		}

		sub, err := azure.SelectAndSetSubscription(tenantID)
		if err != nil {
			ui.Error("Subscription selection failed: %v", err)
			return
		}
		utils.AddScriptEntry("unset KUBECONFIG")
		ui.Success("Subscription set to: %s", sub.Name)
		if err := azure.EnsureManagementToken(tenantID); err != nil {
			ui.Error("Re-authentication failed: %v", err)
		}
	},
}

func init() {}
