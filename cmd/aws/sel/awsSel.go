package sel

import (
	"github.com/BetterCallB4RB4/heimdall/pkg/providers/aws"
	"github.com/BetterCallB4RB4/heimdall/pkg/ui"
	"github.com/spf13/cobra"
)

var AwsSelCmd = &cobra.Command{
	Use:   "select [profile]",
	Short: "Select an AWS profile and ensure its session is valid",
	Long:  "Pick an AWS profile via fzf, validate (and refresh if needed) its SSO session, then export AWS_PROFILE to the calling shell.",
	Run: func(cmd *cobra.Command, args []string) {
		aws.AwsGetSunny()

		// Repair any formatting issues in ~/.aws/config (spaces in section
		// names, spaces in key names, etc.) before reading from it.
		if changed, err := aws.SanitizeAwsConfig(); err != nil {
			ui.Warning("Could not sanitize ~/.aws/config: %v", err)
		} else if changed {
			ui.Info("~/.aws/config had formatting issues and was automatically repaired.")
		}

		// 1. Select the profile first so all subsequent auth checks can be
		//    scoped to the exact session and account this profile belongs to.
		var selection string
		if len(args) > 0 {
			selection = args[0]
		} else {
			profileNames := aws.ListAwsProfilesName()
			selection = ui.GetSelection(profileNames...)
		}

		if selection == "" {
			ui.Info("No profile selected.")
			return
		}

		// 2. Validate the sso-session that backs this profile.
		//    GetProfileSsoSession reads the sso_session key from [profile <name>]
		//    so the check is tied to the exact identity provider.
		ssoSession, err := aws.GetProfileSsoSession(selection)
		if err != nil {
			ui.Warning("Could not determine SSO session for profile '%s': %v", selection, err)
		} else {
			if !aws.IsAwsSsoSessionValid(ssoSession) {
				ui.Warning("SSO session '%s' expired. Triggering login...", ssoSession)
				aws.TriggerAwsSsoLogin(ssoSession)
				if !aws.IsAwsSsoSessionValid(ssoSession) {
					ui.Error("SSO login did not produce a valid session. Aborting.")
					return
				}
			}
		}

		// 3. For SSO profiles the valid portal token is sufficient — the AWS CLI
		//    exchanges it for role credentials automatically on first use when
		//    AWS_PROFILE is set. TriggerAwsProfileLogin is only needed as a
		//    fallback for legacy profiles that have no sso_session link.
		if err != nil {
			// Legacy profile: no sso_session found — check and refresh credentials manually.
			if !aws.IsAwsProfileSessionValid(selection) {
				ui.Warning("Role credentials for profile '%s' expired. Refreshing...", selection)
				aws.TriggerAwsProfileLogin(selection)
			}
		}

		aws.SelectAwsProfile(selection)
	},
}

func init() {}
