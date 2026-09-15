package gen

import (
	"fmt"

	"github.com/BetterCallB4RB4/heimdall/pkg/providers/aws"
	"github.com/BetterCallB4RB4/heimdall/pkg/ui"
	"github.com/spf13/cobra"
)

var GenerateAwsSsoProfileCmd = &cobra.Command{
	Use:   "ssoProfile [sso-session]",
	Short: "Generate ~/.aws/config profiles from all SSO accounts",
	Long:  "Authenticate an SSO session, list all accessible accounts, and write a named profile block for each one into ~/.aws/config.",
	Run: func(cmd *cobra.Command, args []string) {
		// 1. Pick the sso-session once; reused for both login and profile generation.
		var selectedSsoSession string
		if len(args) > 0 {
			selectedSsoSession = args[0]
		} else {
			ssoSessionNames := aws.ListAwsSsoSessionsName()
			selectedSsoSession = ui.GetSelection(ssoSessionNames...)
		}

		// 2. Ensure the sso-session token is valid; re-authenticate if not.
		// IsAwsSsoSessionValid matches the cache file by sso_start_url so it is
		// accurate even when multiple sso-sessions exist.
		if !aws.IsAwsSsoSessionValid(selectedSsoSession) {
			ui.Warning("SSO session expired or not found. Triggering login...")
			aws.TriggerAwsSsoLogin(selectedSsoSession)

			// After login, verify the token is now valid before proceeding.
			if !aws.IsAwsSsoSessionValid(selectedSsoSession) {
				ui.Error("Login did not produce a valid session token. Aborting.")
				return
			}
		} else {
			ui.Success("SSO session is valid.")
		}

		// 3. Retrieve the latest token to use for listing accounts.
		ssoData, err := aws.GetAwsLatestAccessToken()
		if err != nil {
			ui.Fatal(fmt.Errorf("failed to retrieve access token: %w", err))
		}

		// 4. List the accounts using the token.
		s := ui.StartSpinner("Fetching AWS accounts...")
		accounts, err := aws.ListAwsAccounts(ssoData.AccessToken)
		if err != nil {
			s.Fail("Failed to fetch accounts")
			ui.Fatal(fmt.Errorf("failed to list AWS accounts: %w", err))
		}

		if len(accounts) == 0 {
			s.StopWithMessage("No accounts found for this SSO session.")
			return
		}
		s.StopWithMessage(fmt.Sprintf("Found %d account(s)", len(accounts)))

		// 5. Generate profiles in ~/.aws/config using the already selected sso-session.
		pg := ui.StartSpinner(fmt.Sprintf("Writing %d profile(s) to ~/.aws/config...", len(accounts)))
		aws.GenerateAwsSsoProfiles(accounts, selectedSsoSession)
		pg.StopWithMessage("~/.aws/config updated successfully.")
	},
}

func init() {}
