package azure

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BetterCallB4RB4/heimdall/pkg/ui"
	"github.com/BetterCallB4RB4/heimdall/pkg/utils"
)

type AzureSubscription struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	State  string `json:"state"`
	Tenant string `json:"tenantId"`
}

func ListAzureSubscriptions() ([]AzureSubscription, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home err: %v", err)
	}

	profilePath := filepath.Join(home, ".azure", "azureProfile.json")

	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("read err: %v", err)
	}

	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("json root err: %v", err)
	}

	var subs []AzureSubscription
	if err := json.Unmarshal(root["subscriptions"], &subs); err != nil {
		return nil, fmt.Errorf("json subs err: %v", err)
	}

	return subs, nil
}

// func SelectAzureSubscription(accounts []AzureSubscription) (AzureSubscription, error) {
// 	// 1. Costruiamo comando fzf
// 	cmd := exec.Command("fzf", "--height", "40%", "--reverse")
// 	cmd.Stderr = os.Stderr // fzf UI usa stderr
//
// 	stdin, err := cmd.StdinPipe()
// 	if err != nil {
// 		return AzureSubscription{}, err
// 	}
//
// 	var out bytes.Buffer
// 	cmd.Stdout = &out
//
// 	// Avvia fzf
// 	if err := cmd.Start(); err != nil {
// 		return AzureSubscription{}, fmt.Errorf("fzf non trovato: %w", err)
// 	}
//
// 	// Mappa: nome → struct (per lookup al ritorno)
// 	nameToSub := make(map[string]AzureSubscription)
//
// 	// 2. Inviamo a fzf solo il Name
// 	go func() {
// 		defer stdin.Close()
// 		for _, acc := range accounts {
// 			nameToSub[acc.Name] = acc
// 			io.WriteString(stdin, acc.Name+"\n")
// 		}
// 	}()
//
// 	// 3. Attendiamo la selezione
// 	err = cmd.Wait()
// 	if err != nil {
// 		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
// 			return AzureSubscription{}, fmt.Errorf("selezione annullata")
// 		}
// 		return AzureSubscription{}, err
// 	}
//
// 	// Rimuoviamo newline
// 	selectedName := strings.TrimSpace(out.String())
//
// 	// Lookup nel map
// 	sub, ok := nameToSub[selectedName]
// 	if !ok {
// 		return AzureSubscription{}, fmt.Errorf("subscription non trovata: %s", selectedName)
// 	}
//
// 	return sub, nil
// }

// func triggerAzureLogin()  {
//
// 	cmd := exec.Command("az", "login", "--allow-no-subscriptions")
//
// 	// TODO:check if there is a valid token, in that case do not perform the login
//
// 	// Connect streams to allow interactive authentication
// 	cmd.Stdin = os.Stdin
// 	cmd.Stdout = os.Stdout
// 	cmd.Stderr = os.Stderr
//
//
// }

// ---------------------------------

type azTenant struct {
	TenantID      string `json:"tenantId"`
	DisplayName   string `json:"displayName"`
	DefaultDomain string `json:"defaultDomain"`
}

// tenantConfigDir returns the per-tenant Azure config directory path derived
// from the tenant display name: ~/.azure-<sanitized-name>
// e.g. "Contoso Corporation" → ~/.azure-contoso-corporation
// This is the key to per-shell isolation: setting AZURE_CONFIG_DIR to this path
// makes every az command in the process use a private, tenant-scoped session.
func tenantConfigDir(displayName string) string {
	home, _ := os.UserHomeDir()
	safe := strings.ToLower(displayName)
	safe = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, safe)
	for strings.Contains(safe, "--") {
		safe = strings.ReplaceAll(safe, "--", "-")
	}
	safe = strings.Trim(safe, "-")
	if safe == "" {
		safe = "default"
	}
	return filepath.Join(home, ".azure-"+safe)
}

// fetchTenants returns all tenants accessible to the logged-in user.
// Always uses the global ~/.azure/ directory so that it works reliably
// even when the process has a per-tenant AZURE_CONFIG_DIR set.
func fetchTenants() ([]azTenant, error) {
	cmd := exec.Command("az", "rest",
		"--method", "GET",
		"--url", "https://management.azure.com/tenants?api-version=2022-12-01",
		"--output", "json",
		"--only-show-errors",
	)
	cmd.Env = overrideAzDir(os.Environ(), globalAzDir())
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("az rest tenants: %v (%s)", err, strings.TrimSpace(errBuf.String()))
	}

	var resp struct {
		Value []azTenant `json:"value"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("json parse: %w", err)
	}

	for i := range resp.Value {
		if strings.TrimSpace(resp.Value[i].DisplayName) == "" {
			if resp.Value[i].DefaultDomain != "" {
				resp.Value[i].DisplayName = resp.Value[i].DefaultDomain
			} else {
				resp.Value[i].DisplayName = resp.Value[i].TenantID
			}
		}
	}
	return resp.Value, nil
}

// ListAzureSubscriptionsLive fetches subscriptions live from the current az session.
func ListAzureSubscriptionsLive() ([]AzureSubscription, error) {
	cmd := exec.Command("az", "account", "list", "--output", "json", "--only-show-errors", "--all")
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("az account list: %v (%s)", err, strings.TrimSpace(errBuf.String()))
	}
	var subs []AzureSubscription
	if err := json.Unmarshal(out.Bytes(), &subs); err != nil {
		return nil, fmt.Errorf("json parse: %w", err)
	}
	return subs, nil
}

// azLoginWithUI runs a pre-configured az login exec.Cmd, intercepts its stderr,
// and renders the same pretty-box + spinner UX used by the AWS SSO login:
//
//   - Extracts the "A web browser has been opened at <URL>" line and renders a
//     ui.Box with any caller-supplied extraFields prepended (e.g. tenant name)
//     followed by a "Browser" field containing the URL.
//   - Starts a spinner "Waiting for authentication..." once the URL is found.
//   - Silently drops well-known Azure CLI noise lines (MFA-failure messages,
//     tenant-listing blurb, bare UUID tenant-ID lines) that would otherwise
//     clutter the terminal with no actionable content for the user.
//   - Passes any unrecognised stderr lines through to os.Stderr so that real
//     errors are never silently swallowed.
//   - Stops the spinner with success or failure once cmd.Wait() returns.
func azLoginWithUI(cmd *exec.Cmd, boxTitle string, extraFields [][2]string) error {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("az login start: %w", err)
	}

	urlRe := regexp.MustCompile(`A web browser has been opened at (https://\S+)`)
	// UUID prefix pattern — bare "xxxxxxxx-xxxx-… 'Tenant Name'" lines that az
	// prints after the "please use az login --tenant" informational blurb.
	uuidLineRe := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-`)

	noisePrefixes := []string{
		"Authentication failed against tenant",
		"The following tenants don't contain accessible subscriptions",
		"If you need to access subscriptions in the following tenants",
		"Please continue the login in the web browser",
	}

	var sp ui.Spinner
	var boxPrinted bool

	scanner := bufio.NewScanner(stderrPipe)
	for scanner.Scan() {
		line := scanner.Text()

		// Extract the browser URL and render the pretty box + start spinner.
		if !boxPrinted {
			if matches := urlRe.FindStringSubmatch(line); len(matches) > 1 {
				fields := make([][2]string, 0, len(extraFields)+1)
				fields = append(fields, extraFields...)
				fields = append(fields, [2]string{"Browser", matches[1]})
				ui.Box(boxTitle, fields)
				sp = ui.StartSpinner("Waiting for authentication...")
				boxPrinted = true
				continue
			}
		}

		// Silently drop known-noisy informational lines.
		noisy := false
		for _, prefix := range noisePrefixes {
			if strings.HasPrefix(line, prefix) {
				noisy = true
				break
			}
		}
		if noisy || uuidLineRe.MatchString(line) {
			continue
		}

		// Pass through anything else — real errors must not be swallowed.
		fmt.Fprintln(os.Stderr, line)
	}

	waitErr := cmd.Wait()
	if boxPrinted {
		if waitErr != nil {
			sp.Fail("Authentication failed")
		} else {
			sp.Stop()
		}
	}
	return waitErr
}

// triggerInitialLogin runs az login (browser SSO) using the global ~/.azure/
// directory, regardless of any per-tenant AZURE_CONFIG_DIR in the process env.
// This bootstrap is tenant-agnostic: we just need a valid token so that
// fetchTenants() can list all tenants for the interactive picker.
// core.login_experience_v2=off is written to the global config so the az
// login command never shows the built-in interactive subscription/tenant table.
func triggerInitialLogin() error {
	dir := globalAzDir()
	ensureAzConfig(context.Background(), dir)
	// Do NOT pass --only-show-errors: az login prints the browser URL and
	// device code at info/warning level. azLoginWithUI intercepts stderr and
	// renders a clean ui.Box with the URL while silently dropping the MFA-
	// failure noise that az login emits for tenants requiring per-tenant auth.
	cmd := exec.Command("az", "login", "--allow-no-subscriptions", "--output", "none")
	cmd.Env = overrideAzDir(os.Environ(), dir)
	return azLoginWithUI(cmd, "Azure Login", nil)
}

// SelectTenantAndLogin presents a tenant picker, computes the per-tenant
// config directory, sets AZURE_CONFIG_DIR in the current process (so every
// subsequent az exec inherits it), and runs az login --tenant scoped to that dir.
// Returns the config dir path so the caller can export it to the parent shell.
func SelectTenantAndLogin() (string, error) {
	tenants, err := fetchTenants()
	if err != nil {
		return "", fmt.Errorf("could not fetch tenants: %w", err)
	}
	if len(tenants) == 0 {
		return "", fmt.Errorf("no tenants found")
	}

	choices := make([]string, 0, len(tenants))
	choiceToTenant := make(map[string]azTenant, len(tenants))
	for _, t := range tenants {
		label := fmt.Sprintf("%s (%s)", t.DisplayName, t.DefaultDomain)
		choices = append(choices, label)
		choiceToTenant[label] = t
	}

	selected := ui.GetSelection(choices...)
	tenant, ok := choiceToTenant[selected]
	if !ok || tenant.TenantID == "" {
		return "", fmt.Errorf("no tenant selected")
	}

	configDir := tenantConfigDir(tenant.DisplayName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", fmt.Errorf("could not create tenant config dir %s: %w", configDir, err)
	}

	// Write core.login_experience_v2=off into the per-tenant config dir before
	// az login --tenant runs. Without this the dir is empty and az falls back
	// to the interactive subscription/tenant table.
	ensureAzConfig(context.Background(), configDir)

	// Apply to the current Go process: every az exec.Command from here on
	// inherits this env var and uses the per-tenant config directory.
	os.Setenv("AZURE_CONFIG_DIR", configDir)

	// azLoginWithUI intercepts stderr: renders a ui.Box with the tenant name
	// and browser URL, spins while waiting, and filters MFA-failure noise.
	cmd := exec.Command("az", "login",
		"--tenant", tenant.TenantID,
		"--allow-no-subscriptions",
		"--output", "none",
	)
	if err := azLoginWithUI(cmd, "Azure Login", [][2]string{{"Tenant", tenant.DisplayName}}); err != nil {
		return "", fmt.Errorf("az login --tenant failed: %w", err)
	}

	return configDir, nil
}

// TriggerSSOLoginAzure is the full login flow: browser auth (if needed) →
// tenant picker → scoped re-login into a per-tenant config directory.
// It checks the GLOBAL ~/.azure/ session (not a per-tenant dir) so the
// bootstrap az login is only triggered when truly needed, and always runs
// against the global dir — no interactive table will appear.
// The resulting AZURE_CONFIG_DIR is written to generated_script.sh so the
// parent shell gets it after the heimdall wrapper sources the script.
func TriggerSSOLoginAzure() error {
	if !isSessionValidInDir(globalAzDir()) {
		if err := triggerInitialLogin(); err != nil {
			return fmt.Errorf("az login failed: %w", err)
		}
	}

	tenantDir, err := SelectTenantAndLogin()
	if err != nil {
		return err
	}

	// Create (or reuse) the per-shell private config directory and update the
	// process env so the subsequent SelectAndSetSubscription call writes to it.
	shellDir, err := CreateShellConfigDir(tenantDir)
	if err != nil {
		return fmt.Errorf("could not create shell config dir: %w", err)
	}
	os.Setenv("AZURE_CONFIG_DIR", shellDir)

	// Export the shell-private dir to the parent shell.
	utils.AddScriptEntry("unset AZURE_CONFIG_DIR")
	utils.AddScriptEntry(fmt.Sprintf("export AZURE_CONFIG_DIR=%s", shellDir))
	return nil
}

// GetCurrentTenantID returns the tenantId of the currently active az session,
// or an empty string if no session exists or the tenant is not set.
func GetCurrentTenantID() string {
	cmd := exec.Command("az", "account", "show", "--output", "json", "--only-show-errors")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return ""
	}
	var account struct {
		TenantID string `json:"tenantId"`
	}
	if err := json.Unmarshal(out.Bytes(), &account); err != nil {
		return ""
	}
	return account.TenantID
}

// GetActiveSubscription returns the Azure subscription active in the current
// AZURE_CONFIG_DIR without changing authentication or subscription state.
func GetActiveSubscription() (AzureSubscription, error) {
	cmd := exec.Command("az", "account", "show", "--output", "json", "--only-show-errors")
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return AzureSubscription{}, fmt.Errorf("az account show: %v (%s)", err, strings.TrimSpace(errBuf.String()))
	}

	var subscription AzureSubscription
	if err := json.Unmarshal(out.Bytes(), &subscription); err != nil {
		return AzureSubscription{}, fmt.Errorf("parse active Azure subscription: %w", err)
	}
	return subscription, nil
}

// ListAzureSubscriptionsByTenant fetches all subscriptions for the given
// tenantID directly from the ARM API, scoped to that tenant.
func ListAzureSubscriptionsByTenant(tenantID string) ([]AzureSubscription, error) {
	cmd := exec.Command("az", "account", "list",
		"--output", "json",
		"--only-show-errors",
		"--all",
		"--query", fmt.Sprintf("[?tenantId=='%s']", tenantID),
	)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("az account list: %v (%s)", err, strings.TrimSpace(errBuf.String()))
	}
	var subs []AzureSubscription
	if err := json.Unmarshal(out.Bytes(), &subs); err != nil {
		return nil, fmt.Errorf("json parse: %w", err)
	}
	return subs, nil
}

// SetAzureSubscription sets the active subscription within the current
// AZURE_CONFIG_DIR (per-tenant dir). Because AZURE_CONFIG_DIR is already set
// in the process env by SelectTenantAndLogin, az account set writes only to
// the per-tenant azureProfile.json — no global state is modified.
func SetAzureSubscription(selectedAccount AzureSubscription) {
	cmd := exec.Command("az", "account", "set", "--subscription", selectedAccount.ID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		ui.Error("az account set failed: %v\nOutput: %s", err, out)
	}
}

// SelectAndSetSubscription presents a subscription picker for the given
// tenantID, sets it as the active subscription in the current AZURE_CONFIG_DIR,
// and returns the chosen subscription. It is shared between az login and az select.
func SelectAndSetSubscription(tenantID string) (AzureSubscription, error) {
	subs, err := ListAzureSubscriptionsByTenant(tenantID)
	if err != nil {
		return AzureSubscription{}, fmt.Errorf("could not list subscriptions: %w", err)
	}
	if len(subs) == 0 {
		return AzureSubscription{}, fmt.Errorf("no subscriptions found for this tenant")
	}

	names := make([]string, 0, len(subs))
	for _, s := range subs {
		if s.Name != "" {
			names = append(names, s.Name)
		}
	}

	selection := ui.GetSelection(names...)
	if selection == "" {
		return AzureSubscription{}, fmt.Errorf("no subscription selected")
	}

	for _, s := range subs {
		if s.Name == selection {
			SetAzureSubscription(s)
			return s, nil
		}
	}
	return AzureSubscription{}, fmt.Errorf("selected subscription %q not found", selection)
}

// hasValidAzSession checks if Azure CLI is already authenticated.
// We keep it simple: az account show returns 0 when logged in.
func IsAzSessionValid() bool {
	cmd := exec.Command("az", "account", "show", "--output", "none", "--only-show-errors")
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

// runInteractive runs a command attaching stdio to allow interactive browser flows.
func runInteractive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// overrideAzDir returns a copy of env with AZURE_CONFIG_DIR replaced by dir.
// Use this to scope individual az subprocess calls to a specific config dir
// without mutating the current process environment.
func overrideAzDir(env []string, dir string) []string {
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if !strings.HasPrefix(e, "AZURE_CONFIG_DIR=") {
			out = append(out, e)
		}
	}
	return append(out, "AZURE_CONFIG_DIR="+dir)
}

// isSessionValidInDir returns true when az account show succeeds using the
// given config directory, regardless of the current process AZURE_CONFIG_DIR.
func isSessionValidInDir(dir string) bool {
	cmd := exec.Command("az", "account", "show", "--output", "none", "--only-show-errors")
	cmd.Env = overrideAzDir(os.Environ(), dir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

// globalAzDir returns the path of the global (~/.azure/) Azure config directory.
func globalAzDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".azure")
}

func ensureAzConfig(ctx context.Context, dir string) {
	// best-effort: if it fails, continue anyway
	cmd := exec.CommandContext(ctx, "az", "config", "set", "core.login_experience_v2=off")
	cmd.Env = overrideAzDir(os.Environ(), dir)
	_ = cmd.Run()
}

// aksServerID is the well-known Azure Kubernetes Service AAD Server application
// ID.  It is the same across every Azure tenant and every AKS cluster.
const aksServerID = "6dae42f8-4368-4678-94ff-3960e28e3630"

// managementResourceID is the Azure Resource Manager API resource identifier.
// Used to probe and refresh management-plane tokens.
const managementResourceID = "https://management.core.windows.net/"

const tenantConfigMarker = "heimdall-tenant-config-dir"

var authenticationCacheFiles = []string{
	"msal_token_cache.json",
	"msal_http_cache.bin",
}

// EnsureManagementToken probes whether the current AZURE_CONFIG_DIR session
// can obtain an access token for the ARM management plane.
//
// The probe runs az account get-access-token, which exercises the full token
// refresh path (unlike az account show, which only reads cached metadata).
// If the probe fails — most commonly AADSTS70043 (Conditional Access
// sign-in frequency exceeded, refresh token expired) — an interactive
// az login is triggered scoped to the management resource, satisfying the
// CA policy and caching new tokens in the active AZURE_CONFIG_DIR.
//
// azureProfile.json (and therefore the active subscription selection) is
// not modified by this call.
func EnsureManagementToken(tenantID string) error {
	probe := exec.Command("az", "account", "get-access-token",
		"--resource", managementResourceID,
		"--output", "none",
		"--only-show-errors",
	)
	probe.Stdout = io.Discard
	probe.Stderr = io.Discard
	if probe.Run() == nil {
		return syncCurrentAuthenticationCache()
	}

	// Token refresh failed — most likely AADSTS70043 (Conditional Access
	// sign-in frequency policy expired the refresh token).
	// Trigger interactive re-auth scoped to the management plane so the
	// CA challenge is satisfied and new tokens land in AZURE_CONFIG_DIR.
	ui.Warning("Management token expired (Conditional Access). Re-authenticating...")
	configDir := os.Getenv("AZURE_CONFIG_DIR")
	if configDir == "" {
		configDir = globalAzDir()
	}
	ensureAzConfig(context.Background(), configDir)

	// --scope is not a valid az login parameter for ARM re-authentication and
	// causes AADSTS900144 (missing client_id in OAuth request). Plain --tenant
	// re-auth satisfies the Conditional Access policy; az account get-access-token
	// will succeed afterwards. Subscription picker is off via core.login_experience_v2=off.
	reauth := exec.Command("az", "login",
		"--tenant", tenantID,
		"--allow-no-subscriptions",
		"--output", "none",
	)
	reauth.Stdin = os.Stdin
	reauth.Stdout = os.Stdout
	reauth.Stderr = os.Stderr
	if err := reauth.Run(); err != nil {
		return err
	}
	return syncCurrentAuthenticationCache()
}

// EnsureAksToken probes whether the current AZURE_CONFIG_DIR session can obtain
// an access token for the AKS API server resource.
//
// The probe is a fast, non-interactive az account get-access-token call.  If it
// succeeds the function returns nil immediately — no browser is opened.
//
// If it fails (most commonly AADSTS50078: CAP MFA re-challenge required) an
// interactive az login is triggered, scoped to the AKS resource so that the
// MFA challenge is satisfied and the resulting token is cached in
// AZURE_CONFIG_DIR for subsequent kubelogin calls.
//
// tenantID must be the tenant of the active subscription (returned by
// GetCurrentTenantID after SelectAndSetSubscription).
func EnsureAksToken(tenantID string) error {
	probe := exec.Command("az", "account", "get-access-token",
		"--resource", aksServerID,
		"--output", "none",
		"--only-show-errors",
	)
	probe.Stdout = io.Discard
	probe.Stderr = io.Discard
	if probe.Run() == nil {
		return syncCurrentAuthenticationCache()
	}

	// Token invalid or CAP MFA expired — trigger interactive re-auth.
	// NOTE: do NOT pass --only-show-errors here; az login prints the browser
	// URL to stderr as an informational message and suppressing it leaves the
	// user with a hanging prompt and no indication of what to do.
	ui.Warning("AKS token expired (CAP MFA). Re-authenticating...")
	// --scope is not valid for az login and causes AADSTS900144. Plain --tenant
	// re-auth satisfies the CAP MFA challenge; the AKS token will be obtainable
	// via az account get-access-token afterwards.
	reauth := exec.Command("az", "login",
		"--tenant", tenantID,
		"--allow-no-subscriptions",
		"--output", "none",
	)
	reauth.Stdin = os.Stdin
	reauth.Stdout = os.Stdout
	reauth.Stderr = os.Stderr
	if err := reauth.Run(); err != nil {
		return err
	}
	return syncCurrentAuthenticationCache()
}

func GenAksKubeConfig() {
	var accountInfo struct {
		EnvironmentName string `json:"environmentName"`
		ID              string `json:"id"`
		Name            string `json:"name"`
		TenantID        string `json:"tenantId"`
		User            struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"user"`
	}

	sa := ui.StartSpinner("Fetching account info...")
	cmdAccount := exec.Command("az", "account", "show", "--output", "json")
	outAccount, err := cmdAccount.Output()
	if err != nil {
		sa.Fail("Failed to fetch account info")
		ui.Fatal(fmt.Errorf("az account show failed: %w", err))
		return
	}
	if err := json.Unmarshal(outAccount, &accountInfo); err != nil {
		sa.Fail("Failed to parse account info")
		ui.Fatal(fmt.Errorf("failed to parse account info: %w", err))
		return
	}
	sa.StopWithMessage(fmt.Sprintf("Logged in as %s (subscription: %s)", accountInfo.User.Name, accountInfo.Name))

	// Proactively verify the AKS CAP token before attempting any AKS operation.
	// AADSTS50078 (MFA re-challenge required) is only surfaced by kubelogin at
	// kubectl time otherwise — here we catch and resolve it early.
	if err := EnsureAksToken(accountInfo.TenantID); err != nil {
		ui.Fatal(fmt.Errorf("AKS token refresh failed: %w", err))
		return
	}

	var clusters []struct {
		Name          string `json:"name"`
		ResourceGroup string `json:"resourceGroup"`
		Location      string `json:"location"`
	}

	sc := ui.StartSpinner("Fetching AKS clusters...")
	cmdList := exec.Command("az", "aks", "list", "--output", "json")
	outList, err := cmdList.Output()
	if err != nil {
		sc.Fail("Failed to fetch AKS clusters")
		ui.Fatal(fmt.Errorf("az aks list failed: %w", err))
		return
	}
	if err := json.Unmarshal(outList, &clusters); err != nil {
		sc.Fail("Failed to parse AKS cluster list")
		ui.Fatal(fmt.Errorf("failed to parse AKS cluster list: %w", err))
		return
	}
	sc.StopWithMessage(fmt.Sprintf("Found %d AKS cluster(s)", len(clusters)))

	// Display clusters as a table.
	if len(clusters) > 0 {
		rows := make([][]string, 0, len(clusters))
		for _, c := range clusters {
			rows = append(rows, []string{c.Name, c.ResourceGroup, c.Location})
		}
		ui.Table("AKS Clusters", []string{"Name", "Resource Group", "Location"}, rows)
	}

	var selectedCluster string
	var selectedRG string
	if len(clusters) > 1 {
		clusterNames := make([]string, 0, len(clusters))
		for _, c := range clusters {
			clusterNames = append(clusterNames, c.Name)
		}

		selectedCluster = ui.GetSelection(clusterNames...)

		if selectedCluster == "" {
			ui.Info("No cluster selected.")
			return
		}
		for _, c := range clusters {
			if c.Name == selectedCluster {
				selectedRG = c.ResourceGroup
				break
			}
		}

	} else {
		selectedCluster = clusters[0].Name
		selectedRG = clusters[0].ResourceGroup
	}

	home, err := os.UserHomeDir()
	if err != nil {
		ui.Error("Failed to resolve home directory: %v", err)
		return
	}
	kubeconfig := filepath.Join(home, ".kube", "clusters", "az", selectedCluster)

	sk := ui.StartSpinner(fmt.Sprintf("Writing kubeconfig for %s...", selectedCluster))
	if _, err := os.Stat(kubeconfig); err == nil {
		// Kubeconfig exists — re-run kubelogin conversion to ensure the login
		// method is azurecli (inherits the active `az` session, no device code).
		kubelogin := exec.Command(
			"kubelogin", "convert-kubeconfig",
			"-l", "azurecli",
			"--kubeconfig", kubeconfig,
		)
		if err := kubelogin.Run(); err != nil {
			sk.Fail("kubelogin conversion failed")
			ui.Error("kubelogin convert-kubeconfig failed: %v", err)
			return
		}
		parentCommand := fmt.Sprintf("export KUBECONFIG=%s", kubeconfig)
		utils.AddScriptEntry(parentCommand)
		sk.StopWithMessage(fmt.Sprintf("KUBECONFIG → %s (converted)", kubeconfig))
	} else if os.IsNotExist(err) {
		ui.Info("Kubeconfig not found, creating new...")
		updateCmd := exec.Command(
			"az", "aks", "get-credentials",
			"--name", selectedCluster,
			"--resource-group", selectedRG,
			"--file", kubeconfig,
		)

		if err := updateCmd.Run(); err != nil {
			sk.Fail("Failed to fetch AKS credentials")
			ui.Error("az aks get-credentials failed: %v", err)
			return
		}

		kubelogin := exec.Command(
			"kubelogin", "convert-kubeconfig",
			"-l", "azurecli",
			"--kubeconfig", kubeconfig,
		)

		if err := kubelogin.Run(); err != nil {
			sk.Fail("kubelogin conversion failed")
			ui.Error("kubelogin convert-kubeconfig failed: %v", err)
			return
		}

		parentCommand := fmt.Sprintf("export KUBECONFIG=%s", kubeconfig)
		utils.AddScriptEntry(parentCommand)
		sk.StopWithMessage(fmt.Sprintf("KUBECONFIG → %s (created)", kubeconfig))
	}
}

// AzGetSunny clears the Azure context from the current shell by unsetting
// AZURE_CONFIG_DIR and KUBECONFIG in the generated script.
// The underlying az session in the per-tenant directory is left intact so
// other shells and future logins can reuse it without a new browser prompt.
func AzGetSunny() {
	utils.AddScriptEntry("unset AZURE_CONFIG_DIR")
	utils.AddScriptEntry("unset KUBECONFIG")
}

// getDefaultSubscriptionFromDir reads the azureProfile.json inside dir and
// returns the tenant display name and the name of the default subscription
// (isDefault: true). Returns empty strings if the file is missing or malformed.
func getDefaultSubscriptionFromDir(dir string) (tenantName, subName string) {
	data, err := os.ReadFile(filepath.Join(dir, "azureProfile.json"))
	if err != nil {
		return "", ""
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	var profile struct {
		Subscriptions []struct {
			Name              string `json:"name"`
			IsDefault         bool   `json:"isDefault"`
			TenantDisplayName string `json:"tenantDisplayName"`
		} `json:"subscriptions"`
	}
	if err := json.Unmarshal(data, &profile); err != nil {
		return "", ""
	}

	for _, s := range profile.Subscriptions {
		if s.IsDefault {
			return s.TenantDisplayName, s.Name
		}
	}
	return "", ""
}

// PickExistingTenantSession scans ~/.azure-*/ directories for ones that
// already hold a valid az session (checked with isSessionValidInDir).
// If exactly one is found it is returned directly.
// If multiple are found the user picks interactively; labels show the tenant
// display name and the currently active subscription so the choice is clear.
// Returns an error when no valid session exists in any per-tenant dir.
func PickExistingTenantSession() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	matches, err := filepath.Glob(filepath.Join(home, ".azure-*"))
	if err != nil {
		return "", err
	}

	type entry struct {
		dir   string
		label string
	}

	var valid []entry
	for _, dir := range matches {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		if !isSessionValidInDir(dir) {
			continue
		}

		tenantName, subName := getDefaultSubscriptionFromDir(dir)
		// Build a readable label: prefer display name, fall back to dir suffix.
		name := tenantName
		if name == "" {
			name = strings.TrimPrefix(filepath.Base(dir), ".azure-")
		}
		label := name
		if subName != "" {
			label = fmt.Sprintf("%s  →  %s", name, subName)
		}

		valid = append(valid, entry{dir: dir, label: label})
	}

	switch len(valid) {
	case 0:
		return "", fmt.Errorf("no valid tenant session found")
	case 1:
		return valid[0].dir, nil
	default:
		labels := make([]string, len(valid))
		for i, e := range valid {
			labels[i] = e.label
		}
		selected := ui.GetSelection(labels...)
		for _, e := range valid {
			if e.label == selected {
				return e.dir, nil
			}
		}
		return "", fmt.Errorf("no tenant selected")
	}
}

// CreateShellConfigDir returns a per-shell Azure config directory scoped to the
// parent shell's PID: /tmp/azure-heimdall-<ppid>/
//
// If the directory already exists (same shell running heimdall again, e.g. to
// switch subscription) it is returned as-is — the token cache already lives
// there and the embedded refresh token will silently renew access tokens.
//
// If the directory does not yet exist it is created and all regular files from
// tenantDir are copied into it (azureProfile.json, msal_token_cache.json,
// az.json, etc.).  Subdirectories are intentionally skipped.
//
// The caller is responsible for calling os.Setenv("AZURE_CONFIG_DIR", shellDir)
// so that subsequent az subprocess calls use the private directory.
func CreateShellConfigDir(tenantDir string) (string, error) {
	shellDir := fmt.Sprintf("/tmp/azure-heimdall-%d", os.Getppid())

	if _, err := os.Stat(shellDir); err == nil {
		// Already exists for this shell session — reuse as-is.
		if err := os.WriteFile(filepath.Join(shellDir, tenantConfigMarker), []byte(tenantDir), 0600); err != nil {
			return "", fmt.Errorf("write tenant config marker: %w", err)
		}
		return shellDir, nil
	}

	if err := os.MkdirAll(shellDir, 0700); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", shellDir, err)
	}

	entries, err := os.ReadDir(tenantDir)
	if err != nil {
		return "", fmt.Errorf("readdir %s: %w", tenantDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		src := filepath.Join(tenantDir, entry.Name())
		dst := filepath.Join(shellDir, entry.Name())
		data, err := os.ReadFile(src)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", src, err)
		}
		if err := os.WriteFile(dst, data, 0600); err != nil {
			return "", fmt.Errorf("write %s: %w", dst, err)
		}
	}
	if err := os.WriteFile(filepath.Join(shellDir, tenantConfigMarker), []byte(tenantDir), 0600); err != nil {
		return "", fmt.Errorf("write tenant config marker: %w", err)
	}

	return shellDir, nil
}

// syncCurrentAuthenticationCache persists Azure CLI's refreshed MSAL cache from
// a shell-private directory. azureProfile.json is deliberately excluded so each
// shell retains its own active subscription.
func syncCurrentAuthenticationCache() error {
	shellDir := os.Getenv("AZURE_CONFIG_DIR")
	if shellDir == "" {
		return nil
	}

	tenantDirBytes, err := os.ReadFile(filepath.Join(shellDir, tenantConfigMarker))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read tenant config marker: %w", err)
	}
	tenantDir := strings.TrimSpace(string(tenantDirBytes))
	if tenantDir == "" {
		return fmt.Errorf("tenant config marker is empty")
	}

	return syncAuthenticationCache(shellDir, tenantDir)
}

func syncAuthenticationCache(sourceDir, destinationDir string) error {
	for _, name := range authenticationCacheFiles {
		source := filepath.Join(sourceDir, name)
		data, err := os.ReadFile(source)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", source, err)
		}
		if err := os.WriteFile(filepath.Join(destinationDir, name), data, 0600); err != nil {
			return fmt.Errorf("write authentication cache %s: %w", name, err)
		}
	}
	return nil
}

// THIS MAY CHANGE THE OTHER VERSION IsAzSessionValid
// func IsAzLogged() bool {
// 	// 1. Determine the .azure directory path
// 	home, err := os.UserHomeDir()
// 	if err != nil {
// 		return false
// 	}
//
// 	azureDir := filepath.Join(home, ".azure")
// 	// Respect AZURE_CONFIG_DIR if set
// 	if envDir := os.Getenv("AZURE_CONFIG_DIR"); envDir != "" {
// 		azureDir = envDir
// 	}
//
// 	// 2. Check for azureProfile.json (The "Who am I" file)
// 	profilePath := filepath.Join(azureDir, "azureProfile.json")
// 	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
// 		return false
// 	}
//
// 	// 3. Check for accessTokens.json (The "Session" file)
// 	// On Windows, recent versions of az cli might encrypt this file,
// 	// but its existence still indicates a login attempt.
// 	tokenPath := filepath.Join(azureDir, "accessTokens.json")
// 	tokenInfo, err := os.Stat(tokenPath)
// 	if os.IsNotExist(err) || tokenInfo.Size() < 10 {
// 		return false
// 	}
//
// 	// Optional: Deep parse to ensure the token list isn't empty []
// 	content, err := os.ReadFile(tokenPath)
// 	if err == nil {
// 		var tokens []interface{}
// 		if err := json.Unmarshal(content, &tokens); err == nil {
// 			return len(tokens) > 0
// 		}
// 	}
//
// 	return true
// }

//---------------------------------------------------------

// package azure
//
// import (
// 	"bytes"
// 	"context"
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"io"
// 	"log"
// 	"os"
// 	"os/exec"
// 	"path/filepath"
// 	"strings"
//
// 	"github.com/BetterCallB4RB4/heimdall/pkg/utils"
// )
//
// type AzureSubscription struct {
// 	ID     string `json:"id"`
// 	Name   string `json:"name"`
// 	State  string `json:"state"`
// 	Tenant string `json:"tenantId"`
// }
//
// func ListAzureSubscriptions() ([]AzureSubscription, error) {
// 	home, err := os.UserHomeDir()
// 	if err != nil {
// 		return nil, fmt.Errorf("home err: %v", err)
// 	}
//
// 	profilePath := filepath.Join(home, ".azure", "azureProfile.json")
//
// 	data, err := os.ReadFile(profilePath)
// 	if err != nil {
// 		return nil, fmt.Errorf("read err: %v", err)
// 	}
//
// 	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
//
// 	var root map[string]json.RawMessage
// 	if err := json.Unmarshal(data, &root); err != nil {
// 		return nil, fmt.Errorf("json root err: %v", err)
// 	}
//
// 	var subs []AzureSubscription
// 	if err := json.Unmarshal(root["subscriptions"], &subs); err != nil {
// 		return nil, fmt.Errorf("json subs err: %v", err)
// 	}
//
// 	return subs, nil
// }
//
// // func SelectAzureSubscription(accounts []AzureSubscription) (AzureSubscription, error) {
// // 	// 1. Costruiamo comando fzf
// // 	cmd := exec.Command("fzf", "--height", "40%", "--reverse")
// // 	cmd.Stderr = os.Stderr // fzf UI usa stderr
// //
// // 	stdin, err := cmd.StdinPipe()
// // 	if err != nil {
// // 		return AzureSubscription{}, err
// // 	}
// //
// // 	var out bytes.Buffer
// // 	cmd.Stdout = &out
// //
// // 	// Avvia fzf
// // 	if err := cmd.Start(); err != nil {
// // 		return AzureSubscription{}, fmt.Errorf("fzf non trovato: %w", err)
// // 	}
// //
// // 	// Mappa: nome → struct (per lookup al ritorno)
// // 	nameToSub := make(map[string]AzureSubscription)
// //
// // 	// 2. Inviamo a fzf solo il Name
// // 	go func() {
// // 		defer stdin.Close()
// // 		for _, acc := range accounts {
// // 			nameToSub[acc.Name] = acc
// // 			io.WriteString(stdin, acc.Name+"\n")
// // 		}
// // 	}()
// //
// // 	// 3. Attendiamo la selezione
// // 	err = cmd.Wait()
// // 	if err != nil {
// // 		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
// // 			return AzureSubscription{}, fmt.Errorf("selezione annullata")
// // 		}
// // 		return AzureSubscription{}, err
// // 	}
// //
// // 	// Rimuoviamo newline
// // 	selectedName := strings.TrimSpace(out.String())
// //
// // 	// Lookup nel map
// // 	sub, ok := nameToSub[selectedName]
// // 	if !ok {
// // 		return AzureSubscription{}, fmt.Errorf("subscription non trovata: %s", selectedName)
// // 	}
// //
// // 	return sub, nil
// // }
//
// // func triggerAzureLogin()  {
// //
// // 	cmd := exec.Command("az", "login", "--allow-no-subscriptions")
// //
// // 	// TODO:check if there is a valid token, in that case do not perform the login
// //
// // 	// Connect streams to allow interactive authentication
// // 	cmd.Stdin = os.Stdin
// // 	cmd.Stdout = os.Stdout
// // 	cmd.Stderr = os.Stderr
// //
// //
// // }
//
// // ---------------------------------
//
// type tenant struct {
// 	TenantID      string `json:"tenantId"`
// 	DisplayName   string `json:"displayName"`
// 	DefaultDomain string `json:"defaultDomain"`
// }
//
// type tenantsListResponse struct {
// 	Value []tenant `json:"value"`
// }
//
// // sceglio il tenant
// func TriggerSSOLoginAzure() {
// 	if !IsAzSessionValid() {
// 		if err := runInteractive("az", "login", "--allow-no-subscriptions"); err != nil {
// 			fmt.Printf("az login failed: %v\n", err)
// 		}
// 	}
// }
//
// // scegli la subscription nel tenant
// func TriggerAzureLogin() error {
// 	TriggerSSOLoginAzure()
//
// 	// 1) Retrieve tenants with names (more reliable than az account tenant list)
// 	tenants, err := fetchTenants()
// 	if err != nil {
// 		return fmt.Errorf("failed to fetch tenants: %w", err)
// 	}
// 	if len(tenants) == 0 {
// 		return errors.New("no tenants found for this user")
// 	}
//
// 	// Build user-friendly, unique choices + lookup map
// 	choices := make([]string, 0, len(tenants))
// 	choiceToTenantID := make(map[string]string, len(tenants))
//
// 	for _, t := range tenants {
// 		label := fmt.Sprintf("%s (%s)", t.DisplayName, t.DefaultDomain)
// 		choices = append(choices, label)
// 		choiceToTenantID[label] = t.TenantID
// 	}
//
// 	selectedLabel := utils.GetSelection(choices...)
// 	tenantID := choiceToTenantID[selectedLabel]
// 	if tenantID == "" {
// 		return fmt.Errorf("could not resolve tenantID for selection %q", selectedLabel)
// 	}
//
// 	if err := runInteractive("az", "login", "--tenant", tenantID, "--allow-no-subscriptions"); err != nil {
// 		return fmt.Errorf("az login --tenant failed: %w", err)
// 	}
//
// 	return nil
// }
//
// func SetAzureSubscrioption(selctedAccount AzureSubscription) {
// 	// subscriptionList, err := ListAzureSubscriptions()
// 	// if err != nil {
// 	// 	return
// 	// }
// 	//
// 	// choices := make([]string, 0, len(subscriptionList))
// 	//
// 	// for _, sub := range subscriptionList {
// 	// 	// Make the label unique and readable
// 	// 	choices = append(choices, sub.Name)
// 	// }
//
// 	cmd := exec.Command("az", "account", "set", "--subscription", selctedAccount.ID)
// 	out, err := cmd.CombinedOutput()
// 	if err != nil {
// 		fmt.Printf("Command failed: %v\nOutput: %s\n", err, out)
// 	}
// }
//
// // hasValidAzSession checks if Azure CLI is already authenticated.
// // We keep it simple: az account show returns 0 when logged in.
// func IsAzSessionValid() bool {
// 	cmd := exec.Command("az", "account", "show", "--output", "none", "--only-show-errors")
// 	cmd.Stdin = nil
// 	cmd.Stdout = io.Discard
// 	cmd.Stderr = io.Discard
// 	return cmd.Run() == nil
// }
//
// // fetchTenants gets tenants with displayName/defaultDomain via ARM tenants endpoint.
// // The REST response contains displayName & defaultDomain. [2](https://github.com/Azure/azure-cli/issues/19276)
// func fetchTenants() ([]tenant, error) {
// 	// Using az rest is the most reliable way to get tenant displayName/defaultDomain.
// 	cmd := exec.Command("az", "rest",
// 		"--method", "GET",
// 		"--url", "https://management.azure.com/tenants?api-version=2020-01-01",
// 		"--only-show-errors",
// 		"-o", "json",
// 	)
//
// 	var out bytes.Buffer
// 	var errBuf bytes.Buffer
// 	cmd.Stdout = &out
// 	cmd.Stderr = &errBuf
//
// 	if err := cmd.Run(); err != nil {
// 		return nil, fmt.Errorf("az rest error: %v (%s)", err, strings.TrimSpace(errBuf.String()))
// 	}
//
// 	var resp tenantsListResponse
// 	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
// 		return nil, fmt.Errorf("json parse error: %w", err)
// 	}
//
// 	// Normalize missing names
// 	for i := range resp.Value {
// 		if strings.TrimSpace(resp.Value[i].DisplayName) == "" {
// 			// fallback: show domain if name missing
// 			if resp.Value[i].DefaultDomain != "" {
// 				resp.Value[i].DisplayName = resp.Value[i].DefaultDomain
// 			} else {
// 				resp.Value[i].DisplayName = resp.Value[i].TenantID
// 			}
// 		}
// 	}
//
// 	return resp.Value, nil
// }
//
// // func pickTenantWithFzf(tenants []tenant) (tenant, error) {
// // 	// Build input lines for fzf: "<name> (<domain>)\t<tenantId>"
// // 	var input strings.Builder
// // 	for _, t := range tenants {
// // 		label := t.DisplayName
// // 		if t.DefaultDomain != "" && t.DefaultDomain != t.DisplayName {
// // 			label = fmt.Sprintf("%s (%s)", t.DisplayName, t.DefaultDomain)
// // 		}
// // 		input.WriteString(label)
// // 		input.WriteString("\t")
// // 		input.WriteString(t.TenantID)
// // 		input.WriteString("\n")
// // 	}
// //
// // 	// Prepare fzf command
// // 	// --with-nth=1 -> show only the label column
// // 	// --delimiter '\t' -> split columns
// // 	// --prompt -> nice UX
// // 	fzf := exec.Command("fzf",
// // 		"--delimiter=\t",
// // 		"--with-nth=1",
// // 		"--prompt=Tenant> ",
// // 		"--height=40%",
// // 		"--border",
// // 	)
// //
// // 	fzf.Stdin = strings.NewReader(input.String())
// //
// // 	var out bytes.Buffer
// // 	var errBuf bytes.Buffer
// // 	fzf.Stdout = &out
// // 	fzf.Stderr = &errBuf
// //
// // 	if err := fzf.Run(); err != nil {
// // 		// Exit code 130 typically means ESC/abort in fzf
// // 		return tenant{}, fmt.Errorf("fzf error: %v (%s)", err, strings.TrimSpace(errBuf.String()))
// // 	}
// //
// // 	choice := strings.TrimSpace(out.String())
// // 	if choice == "" {
// // 		return tenant{}, errors.New("empty selection")
// // 	}
// //
// // 	parts := strings.Split(choice, "\t")
// // 	if len(parts) != 2 {
// // 		return tenant{}, fmt.Errorf("unexpected fzf selection format: %q", choice)
// // 	}
// // 	tenantID := strings.TrimSpace(parts[1])
// //
// // 	// Find the selected tenant struct
// // 	for _, t := range tenants {
// // 		if t.TenantID == tenantID {
// // 			return t, nil
// // 		}
// // 	}
// // 	return tenant{}, fmt.Errorf("selected tenant id not found in list: %s", tenantID)
// // }
//
// // runInteractive runs a command attaching stdio to allow interactive browser/device-code flows.
// func runInteractive(name string, args ...string) error {
// 	cmd := exec.Command(name, args...)
// 	cmd.Stdin = os.Stdin
// 	cmd.Stdout = os.Stdout
// 	cmd.Stderr = os.Stderr
// 	return cmd.Run()
// }
//
// func ensureAzConfig(ctx context.Context) {
// 	// best-effort: if it fails, continue anyway
// 	_ = exec.CommandContext(ctx, "az", "config", "set", "core.login_experience_v2=off").Run()
// }
//
// func GenAksKubeConfig() {
// 	var accountInfo struct {
// 		EnvironmentName string `json:"environmentName"`
// 		ID              string `json:"id"`
// 		Name            string `json:"name"`
// 		User            struct {
// 			Name string `json:"name"`
// 			Type string `json:"type"`
// 		} `json:"user"`
// 	}
//
// 	fmt.Println("--- Recupero informazioni account ---")
// 	cmdAccount := exec.Command("az", "account", "show", "--output", "json")
// 	outAccount, err := cmdAccount.Output()
// 	if err != nil {
// 		log.Fatalf("Errore durante az account show: %v", err)
// 	}
//
// 	if err := json.Unmarshal(outAccount, &accountInfo); err != nil {
// 		log.Fatalf("Errore parsing JSON account: %v", err)
// 	}
// 	fmt.Printf("Logged in as: %s (Subscription: %s)\n", accountInfo.User.Name, accountInfo.Name)
//
// 	// 2. az aks list -> Lista cluster e relativi Resource Group
// 	fmt.Println("\n--- Lista Cluster AKS e Resource Groups ---")
// 	var clusters []struct {
// 		Name          string `json:"name"`
// 		ResourceGroup string `json:"resourceGroup"`
// 		Location      string `json:"location"`
// 	}
//
// 	cmdList := exec.Command("az", "aks", "list", "--output", "json")
// 	outList, err := cmdList.Output()
// 	if err != nil {
// 		log.Fatalf("Errore durante az aks list: %v", err)
// 	}
//
// 	if err := json.Unmarshal(outList, &clusters); err != nil {
// 		log.Fatalf("Errore parsing JSON clusters: %v", err)
// 	}
//
// 	for _, c := range clusters {
// 		fmt.Printf("Cluster: %s | RG: %s | Location: %s\n", c.Name, c.ResourceGroup, c.Location)
// 	}
//
// 	var selectedCluster string
// 	var selectedRG string
// 	if len(clusters) > 1 {
// 		fmt.Println("Multiple clusters found. Please select one:")
// 		// GetSelection handles the fzf interaction or manual input
//
// 		clusterNames := make([]string, 0, len(clusters))
// 		for _, c := range clusters {
// 			clusterNames = append(clusterNames, c.Name)
// 		}
//
// 		selectedCluster = utils.GetSelection(clusterNames...)
//
// 		if selectedCluster == "" {
// 			fmt.Println("No cluster selected. Exiting.")
// 			return
// 		}
// 		for _, c := range clusters {
// 			if c.Name == selectedCluster {
// 				selectedRG = c.ResourceGroup
// 				break
// 			}
// 		}
//
// 	} else {
// 		selectedCluster = clusters[0].Name
// 		selectedRG = clusters[0].ResourceGroup
// 	}
//
// 	// 4. Update Kubeconfig for the selected cluster
// 	fmt.Printf("Updating kubeconfig for: %s... ", selectedCluster)
//
// 	// kubeconfig := fmt.Sprintf("~/.kube/clusters/az/%s", selectedCluster)
//
// 	home, err := os.UserHomeDir()
// 	if err != nil {
// 		fmt.Printf("Error: %v\n", err)
// 	}
// 	kubeconfig := filepath.Join(home, ".kube", "clusters", "az", selectedCluster)
//
// 	if _, err := os.Stat(kubeconfig); err == nil {
// 		// if kubeconfig exixst reuse the old one
// 		parentCommand := fmt.Sprintf("export KUBECONFIG=%s", kubeconfig)
// 		utils.AddScriptEntry(parentCommand)
// 	} else if os.IsNotExist(err) {
// 		fmt.Println("kubeconfig does NOT exist, creating new")
// 		updateCmd := exec.Command(
// 			"az", "aks", "get-credentials",
// 			"--name", selectedCluster,
// 			"--resource-group", selectedRG,
// 			"--file", kubeconfig,
// 		)
//
// 		if err := updateCmd.Run(); err != nil {
// 			fmt.Printf("Error: %v\n", err)
// 		}
//
// 		kubelogin := exec.Command(
// 			"kubelogin", "convert-kubeconfig",
// 			"-l", "azurecli",
// 			"--kubeconfig", kubeconfig,
// 		)
//
// 		if err := kubelogin.Run(); err != nil {
// 			fmt.Printf("Error: %v\n", err)
// 		}
//
// 		parentCommand := fmt.Sprintf("export KUBECONFIG=%s", kubeconfig)
// 		utils.AddScriptEntry(parentCommand)
//
// 	}
//
// 	// DEBUG
// 	// out, err := updateCmd.CombinedOutput()
// 	// if err != nil {
// 	// 	fmt.Printf("kubelogin failed: %v\n", err)
// 	// 	fmt.Printf("output:\n%s\n", string(out))
// 	// 	// If you want the numeric exit code:
// 	// 	if ee, ok := err.(*exec.ExitError); ok {
// 	// 		fmt.Printf("exit code: %d\n", ee.ExitCode())
// 	// 	}
// 	// 	return
// 	// }
// }
//
// func AzGetSunny() {
// 	utils.AddScriptEntry("unset KUBECONFIG")
// 	if IsAzSessionValid() {
//
// 		azLogout := exec.Command(
// 			"az", "logout",
// 		)
//
// 		if err := azLogout.Run(); err != nil {
// 			fmt.Printf("Error: %v\n", err)
// 		}
// 	}
//
// 	// out, err := azLogout.CombinedOutput()
// 	// if err != nil {
// 	// 	fmt.Printf("kubelogin failed: %v\n", err)
// 	// 	fmt.Printf("output:\n%s\n", string(out))
// 	// 	// If you want the numeric exit code:
// 	// 	if ee, ok := err.(*exec.ExitError); ok {
// 	// 		fmt.Printf("exit code: %d\n", ee.ExitCode())
// 	// 	}
// 	// 	return
// 	// }
// }
//
// // THIS MAY CHANGE THE OTHER VERSION IsAzSessionValid
// // func IsAzLogged() bool {
// // 	// 1. Determine the .azure directory path
// // 	home, err := os.UserHomeDir()
// // 	if err != nil {
// // 		return false
// // 	}
// //
// // 	azureDir := filepath.Join(home, ".azure")
// // 	// Respect AZURE_CONFIG_DIR if set
// // 	if envDir := os.Getenv("AZURE_CONFIG_DIR"); envDir != "" {
// // 		azureDir = envDir
// // 	}
// //
// // 	// 2. Check for azureProfile.json (The "Who am I" file)
// // 	profilePath := filepath.Join(azureDir, "azureProfile.json")
// // 	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
// // 		return false
// // 	}
// //
// // 	// 3. Check for accessTokens.json (The "Session" file)
// // 	// On Windows, recent versions of az cli might encrypt this file,
// // 	// but its existence still indicates a login attempt.
// // 	tokenPath := filepath.Join(azureDir, "accessTokens.json")
// // 	tokenInfo, err := os.Stat(tokenPath)
// // 	if os.IsNotExist(err) || tokenInfo.Size() < 10 {
// // 		return false
// // 	}
// //
// // 	// Optional: Deep parse to ensure the token list isn't empty []
// // 	content, err := os.ReadFile(tokenPath)
// // 	if err == nil {
// // 		var tokens []interface{}
// // 		if err := json.Unmarshal(content, &tokens); err == nil {
// // 			return len(tokens) > 0
// // 		}
// // 	}
// //
// // 	return true
// // }
