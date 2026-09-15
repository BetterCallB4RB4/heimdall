package aws

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/BetterCallB4RB4/heimdall/pkg/ui"
	"github.com/BetterCallB4RB4/heimdall/pkg/utils"
	"gopkg.in/ini.v1"
)

var awsRegions = []string{
	"us-east-1",
	"af-south-1",
	"ap-east-1",
	"ap-south-1",
	"ap-southeast-3",
	"ap-southeast-5",
	"ap-southeast-2",
	"ap-southeast-6",
	"ap-northeast-1",
	"ap-northeast-2",
	"ap-southeast-1",
	"ap-east-2",
	"ap-southeast-7",
	"ca-central-1",
	"eu-central-1",
	"eu-west-1",
	"eu-west-2",
	"eu-south-1",
	"eu-west-3",
	"eu-south-2",
	"eu-north-1",
	"eu-central-2",
	"il-central-1",
	"mx-central-1",
	"me-south-1",
	"me-central-1",
	"sa-east-1",
}

// AwsRegions returns the full list of supported AWS regions.
func AwsRegions() []string {
	return awsRegions
}

type SSOData struct {
	AccessToken string `json:"accessToken"`
	ExpiresAt   string `json:"expiresAt"`
}

// parseAwsExpiry parses the expiry timestamps written by the AWS CLI.
// AWS uses a non-standard "UTC" suffix (e.g. "2024-01-15T10:15:00UTC")
// instead of the RFC3339-required "Z" designator. This helper normalises
// both forms so that token validation never silently fails due to a parse error.
func parseAwsExpiry(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, strings.ReplaceAll(s, "UTC", "Z"))
}

// AWSAccount represents a single account object from the list-accounts command
type AWSAccount struct {
	AccountID     string `json:"accountId"`
	AccountName   string `json:"accountName"`
	EmailAddress  string `json:"emailAddress"`
	AccountStatus string `json:"accountStatus"`
}

// triggerSsoLoginCmd runs an `aws sso login` command built from the provided
// extra args (e.g. "--sso-session", name or "--profile", name). AWS CLI opens
// the browser when available; Heimdall also displays its verification URL and
// user code as a fallback for headless environments.
func triggerSsoLoginCmd(extraArgs ...string) {
	args := append([]string{"sso", "login"}, extraArgs...)
	cmd := exec.Command("aws", args...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		ui.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		ui.Fatal(err)
	}

	urlRe := regexp.MustCompile(`https://\S+`)
	codeRe := regexp.MustCompile(`\b[A-Z0-9]{4}-[A-Z0-9]{4}\b`)

	var u, c string
	var boxPrinted bool

	scanner := bufio.NewScanner(stderrPipe)
	for scanner.Scan() {
		line := scanner.Text()

		if u == "" {
			if match := urlRe.FindString(line); match != "" {
				u = match
			}
		}
		if c == "" {
			if match := codeRe.FindString(line); match != "" {
				c = match
			}
		}
		// Print the box as soon as both pieces are available so the user
		// knows what to open in their browser without waiting for EOF.
		if !boxPrinted && u != "" && c != "" {
			ui.Box("AWS SSO Login", [][2]string{
				{"Verification URL", u},
				{"User Code", c},
			})
			boxPrinted = true
		}
	}

	// Edge-case fallback: print whatever we collected if the box was never shown.
	if !boxPrinted && (u != "" || c != "") {
		fields := [][2]string{}
		if u != "" {
			fields = append(fields, [2]string{"Verification URL", u})
		}
		if c != "" {
			fields = append(fields, [2]string{"User Code", c})
		}
		ui.Box("AWS SSO Login", fields)
	}

	if err := cmd.Wait(); err != nil {
		ui.Fatal(err)
	}
}

// TriggerAwsSsoLogin starts an SSO login flow for the given sso-session name.
func TriggerAwsSsoLogin(sessionName string) {
	triggerSsoLoginCmd("--sso-session", sessionName)
}

// TriggerAwsProfileLogin starts an SSO login flow for the given profile name.
func TriggerAwsProfileLogin(profileName string) {
	triggerSsoLoginCmd("--profile", profileName)
}

func GetAwsLatestAccessToken() (SSOData, error) {
	cacheDir := filepath.Join(os.Getenv("HOME"), ".aws", "sso", "cache")
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		return SSOData{}, err
	}

	var newest string
	var newestModTime int64
	for _, f := range files {
		info, err := f.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Unix() > newestModTime {
			newestModTime = info.ModTime().Unix()
			newest = filepath.Join(cacheDir, f.Name())
		}
	}

	data, err := os.ReadFile(newest)
	if err != nil {
		return SSOData{}, err
	}

	var sso SSOData
	// ALWAYS check the error of Unmarshal
	if err := json.Unmarshal(data, &sso); err != nil {
		return SSOData{}, err
	}

	// Ensure we actually have a token, otherwise it's a "junk" file
	if sso.AccessToken == "" {
		return SSOData{}, fmt.Errorf("file found but contains no access token")
	}

	return sso, nil
}

func IsAwsSsoTokenValid() bool {
	sso, err := GetAwsLatestAccessToken()
	if err != nil {
		return false
	}
	expiry, err := parseAwsExpiry(sso.ExpiresAt)
	if err != nil {
		return false
	}
	return time.Now().Before(expiry)
}

// CLICache represents the JSON structure of the files found in ~/.aws/cli/cache/
type CLICache struct {
	Credentials struct {
		AccessKeyID     string `json:"AccessKeyId"`
		SecretAccessKey string `json:"SecretAccessKey"`
		SessionToken    string `json:"SessionToken"`
		Expiration      string `json:"Expiration"`
	} `json:"Credentials"`
	// AssumedRoleUser is present in SSO-backed credential files and carries the
	// ARN (which embeds the account ID), used for profile-specific cache matching.
	AssumedRoleUser struct {
		Arn string `json:"Arn"`
	} `json:"AssumedRoleUser"`
}

// GetAwsLatestCliCacheToken reads the most recently modified file from ~/.aws/cli/cache/
// and returns the credentials found within it.
func GetAwsLatestCliCacheToken() (CLICache, error) {
	cacheDir := filepath.Join(os.Getenv("HOME"), ".aws", "cli", "cache")
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		return CLICache{}, err
	}

	var newest string
	var newestModTime int64
	for _, f := range files {
		info, err := f.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Unix() > newestModTime {
			newestModTime = info.ModTime().Unix()
			newest = filepath.Join(cacheDir, f.Name())
		}
	}

	if newest == "" {
		return CLICache{}, fmt.Errorf("no files found in cli cache directory")
	}

	data, err := os.ReadFile(newest)
	if err != nil {
		return CLICache{}, err
	}

	var cli CLICache
	if err := json.Unmarshal(data, &cli); err != nil {
		return CLICache{}, err
	}

	if cli.Credentials.AccessKeyID == "" {
		return CLICache{}, fmt.Errorf("file found but contains no access key")
	}

	return cli, nil
}

// IsAwsCliCacheTokenValid returns true if the credentials stored in ~/.aws/cli/cache/
// exist and have not yet expired.
func IsAwsCliCacheTokenValid() bool {
	cli, err := GetAwsLatestCliCacheToken()
	if err != nil {
		return false
	}
	expiry, err := parseAwsExpiry(cli.Credentials.Expiration)
	if err != nil {
		return false
	}
	return time.Now().Before(expiry)
}

// IsAwsSsoSessionValid returns true when the named sso-session has a non-expired
// access token in ~/.aws/sso/cache/. Unlike IsAwsSsoTokenValid it finds the cache
// file by matching the sso_start_url of the session, not by modification time, so
// it remains correct when multiple sso-sessions coexist.
func IsAwsSsoSessionValid(sessionName string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	// Look up the start URL for this session so we can find the right cache file.
	startURL, _, err := GetAwsSsoSessionDetails(sessionName)
	if err != nil || startURL == "" {
		return false
	}

	cacheDir := filepath.Join(home, ".aws", "sso", "cache")
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		return false
	}

	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(cacheDir, f.Name()))
		if err != nil {
			continue
		}

		// Only consider files that belong to this sso-session.
		if !strings.Contains(string(data), startURL) {
			continue
		}

		var token struct {
			ExpiresAt string `json:"expiresAt"`
		}
		if err := json.Unmarshal(data, &token); err != nil || token.ExpiresAt == "" {
			continue
		}

		expiry, err := parseAwsExpiry(token.ExpiresAt)
		if err != nil {
			continue
		}

		if time.Now().Before(expiry) {
			return true
		}
	}

	return false
}

// GetProfileSsoSession reads ~/.aws/config and returns the sso-session name
// linked to the given profile via its sso_session key.
func GetProfileSsoSession(profileName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	cfg, err := ini.LoadSources(ini.LoadOptions{SkipUnrecognizableLines: true},
		filepath.Join(home, ".aws", "config"))
	if err != nil {
		return "", fmt.Errorf("failed to load ~/.aws/config: %w", err)
	}

	sectionName := fmt.Sprintf("profile %s", profileName)
	section, err := cfg.GetSection(sectionName)
	if err != nil {
		return "", fmt.Errorf("profile '%s' not found in ~/.aws/config", profileName)
	}

	ssoSession := section.Key("sso_session").String()
	if ssoSession == "" {
		return "", fmt.Errorf("profile '%s' has no sso_session configured", profileName)
	}

	return ssoSession, nil
}

// IsAwsProfileSessionValid returns true when the given profile has non-expired
// role credentials in ~/.aws/cli/cache/. The correct cache file is identified by
// matching the profile's sso_account_id against the AssumedRoleUser.Arn field
// (or a raw string search for the account ID as a fallback).
func IsAwsProfileSessionValid(profileName string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	// Read the account ID for this profile.
	cfg, err := ini.LoadSources(ini.LoadOptions{SkipUnrecognizableLines: true},
		filepath.Join(home, ".aws", "config"))
	if err != nil {
		return false
	}

	section, err := cfg.GetSection(fmt.Sprintf("profile %s", profileName))
	if err != nil {
		return false
	}

	accountID := section.Key("sso_account_id").String()
	if accountID == "" {
		return false
	}

	cacheDir := filepath.Join(home, ".aws", "cli", "cache")
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		return false
	}

	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(cacheDir, f.Name()))
		if err != nil {
			continue
		}

		// Match by account ID — it appears in AssumedRoleUser.Arn and/or the
		// raw token payload, so a string search is sufficient and avoids having
		// to reconstruct the AWS CLI cache filename hash.
		if !strings.Contains(string(data), accountID) {
			continue
		}

		var cache CLICache
		if err := json.Unmarshal(data, &cache); err != nil || cache.Credentials.Expiration == "" {
			continue
		}

		expiry, err := parseAwsExpiry(cache.Credentials.Expiration)
		if err != nil {
			continue
		}

		if time.Now().Before(expiry) {
			return true
		}
	}

	return false
}

func ListAwsAccounts(token string) ([]AWSAccount, error) {
	cmd := exec.Command("aws", "sso", "list-accounts",
		"--access-token", token,
		"--region", "us-east-1", // Ensure this matches your SSO portal region
		"--output", "json",
	)

	// Capture stderr to see the actual AWS error message
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		// Return the actual CLI error message (e.g., "Token expired")
		return nil, fmt.Errorf("aws cli error: %w, details: %s", err, stderr.String())
	}

	var wrapper struct {
		Accounts []AWSAccount `json:"accountList"`
	}

	if err := json.Unmarshal(output, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return wrapper.Accounts, nil
}

// profile and sso session in the program are used only by name, so a struct is not necessary
func ListAwsProfilesName() []string {
	home, err := os.UserHomeDir()
	ui.Fatal(err)
	configPath := filepath.Join(home, ".aws", "config")

	cfg, err := ini.Load(configPath)
	ui.Fatal(err)

	var profileNames []string
	for _, section := range cfg.Sections() {
		name := section.Name()
		if strings.HasPrefix(name, "profile ") {
			cleanName := strings.TrimPrefix(name, "profile ")
			profileNames = append(profileNames, cleanName)
		}
	}

	return profileNames
}

func ListAwsSsoSessionsName() []string {
	home, err := os.UserHomeDir()
	ui.Fatal(err)
	configPath := filepath.Join(home, ".aws", "config")

	cfg, err := ini.Load(configPath)
	ui.Fatal(err)

	var ssoSessions []string
	for _, section := range cfg.Sections() {
		name := section.Name()
		if strings.HasPrefix(name, "sso-session ") {
			cleanName := strings.TrimPrefix(name, "sso-session ")
			ssoSessions = append(ssoSessions, cleanName)
		}
	}

	return ssoSessions
}

// SanitizeAwsConfig rewrites ~/.aws/config fixing:
//   - spaces inside section headers (e.g. "[profile CR IF-foo]" -> "[profile CRIF-foo]")
//   - spaces inside key names (e.g. "sso_ role_name" -> "sso_role_name")
//   - sso_start_url values that are a session name instead of an https:// URL,
//     replaced with the real URL read from the matching [sso-session] block.
//
// SanitizeAwsConfig rewrites ~/.aws/config to fix common formatting issues.
// Returns (true, nil) when the file was modified, (false, nil) when it was
// already clean, and (false, err) on any I/O or parse error.
func SanitizeAwsConfig() (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}
	configPath := filepath.Join(home, ".aws", "config")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return false, fmt.Errorf("cannot read ~/.aws/config: %w", err)
	}

	// --- Pass 1: fix spaces in section headers and key names ---
	sectionRe := regexp.MustCompile(`^\[\s*(.+?)\s*\]`)
	keyRe := regexp.MustCompile(`^([^=]+?)\s*=\s*(.*)$`)

	var pass1 strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if sectionRe.MatchString(trimmed) {
			// Remove all internal spaces from inside the brackets,
			// but preserve the single space between the type and the name
			// e.g. "[ profile CR IF-foo ]" -> "[profile CRIF-foo]"
			inner := sectionRe.FindStringSubmatch(trimmed)[1]
			// remove spaces but keep the first space that separates type from name
			parts := strings.SplitN(inner, " ", 2)
			if len(parts) == 2 {
				sectionType := strings.ReplaceAll(parts[0], " ", "")
				sectionName := strings.ReplaceAll(parts[1], " ", "")
				pass1.WriteString(fmt.Sprintf("[%s %s]\n", sectionType, sectionName))
			} else {
				pass1.WriteString(fmt.Sprintf("[%s]\n", strings.ReplaceAll(inner, " ", "")))
			}
			continue
		}

		if keyRe.MatchString(trimmed) && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, ";") {
			m := keyRe.FindStringSubmatch(trimmed)
			key := strings.ReplaceAll(m[1], " ", "")
			val := strings.TrimSpace(m[2])
			pass1.WriteString(fmt.Sprintf("%s = %s\n", key, val))
			continue
		}

		// blank lines and comments pass through
		pass1.WriteString(line + "\n")
	}

	// --- Pass 2: collect sso-session start URLs from the cleaned config ---
	sessionURLs := map[string]string{} // sessionName -> sso_start_url
	sessionRegions := map[string]string{}
	var currentSection string
	pass1Lines := strings.Split(strings.TrimSuffix(pass1.String(), "\n"), "\n")
	for _, line := range pass1Lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[sso-session ") && strings.HasSuffix(trimmed, "]") {
			currentSection = strings.TrimSuffix(strings.TrimPrefix(trimmed, "[sso-session "), "]")
		} else if strings.HasPrefix(trimmed, "[") {
			currentSection = ""
		}
		if currentSection != "" {
			if strings.HasPrefix(trimmed, "sso_start_url") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 {
					sessionURLs[currentSection] = strings.TrimSpace(parts[1])
				}
			}
			if strings.HasPrefix(trimmed, "sso_region") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 {
					sessionRegions[currentSection] = strings.TrimSpace(parts[1])
				}
			}
		}
	}

	// --- Pass 3: fix sso_start_url in profile sections ---
	var pass3 strings.Builder
	currentSection = ""
	currentSessionRef := ""
	for _, line := range pass1Lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "[profile ") && strings.HasSuffix(trimmed, "]") {
			currentSection = "profile"
			currentSessionRef = ""
			pass3.WriteString(line + "\n")
			continue
		} else if strings.HasPrefix(trimmed, "[") {
			currentSection = ""
			currentSessionRef = ""
			pass3.WriteString(line + "\n")
			continue
		}

		if currentSection == "profile" {
			// capture sso_session reference
			if strings.HasPrefix(trimmed, "sso_session") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 {
					currentSessionRef = strings.TrimSpace(parts[1])
				}
			}
			// fix sso_start_url if it's not an https URL
			if strings.HasPrefix(trimmed, "sso_start_url") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 {
					val := strings.TrimSpace(parts[1])
					if !strings.HasPrefix(val, "https://") && currentSessionRef != "" {
						if realURL, ok := sessionURLs[currentSessionRef]; ok && strings.HasPrefix(realURL, "https://") {
							pass3.WriteString(fmt.Sprintf("sso_start_url = %s\n", realURL))
							continue
						}
					}
				}
			}
		}

		pass3.WriteString(line + "\n")
	}

	_ = sessionRegions // available if needed later

	// Only write the file back when content actually changed.
	sanitized := pass3.String()
	if sanitized == string(data) {
		return false, nil
	}

	if err := os.WriteFile(configPath, []byte(sanitized), 0o600); err != nil {
		return false, fmt.Errorf("failed to write sanitized ~/.aws/config: %w", err)
	}

	ui.Success("~/.aws/config sanitized successfully.")
	return true, nil
}

func GetAwsSsoSessionDetails(ssoSessionName string) (startURL string, ssoRegion string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	configPath := filepath.Join(home, ".aws", "config")

	cfg, err := ini.LoadSources(ini.LoadOptions{
		SkipUnrecognizableLines: true,
	}, configPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to load ~/.aws/config: %w", err)
	}

	sectionName := fmt.Sprintf("sso-session %s", ssoSessionName)
	section, err := cfg.GetSection(sectionName)
	if err != nil {
		return "", "", fmt.Errorf("sso-session '%s' not found in ~/.aws/config", ssoSessionName)
	}

	startURL = section.Key("sso_start_url").String()
	ssoRegion = section.Key("sso_region").String()

	if startURL == "" {
		return "", "", fmt.Errorf("sso_start_url is missing in [sso-session %s]", ssoSessionName)
	}
	if ssoRegion == "" {
		return "", "", fmt.Errorf("sso_region is missing in [sso-session %s]", ssoSessionName)
	}

	return startURL, ssoRegion, nil
}

func EnsureAwsSsoSession(cfg *ini.File, ssoSessionName string, startURL string, ssoRegion string) error {
	sectionName := fmt.Sprintf("sso-session %s", ssoSessionName)
	section, err := cfg.GetSection(sectionName)
	if err != nil {
		section, err = cfg.NewSection(sectionName)
		if err != nil {
			return fmt.Errorf("failed to create sso-session section: %w", err)
		}
	}
	section.Key("sso_start_url").SetValue(startURL)
	section.Key("sso_region").SetValue(ssoRegion)
	section.Key("sso_registration_scopes").SetValue("sso:account:access")
	return nil
}

func GenerateAwsSsoProfiles(accounts []AWSAccount, ssoSessionName string) {
	homeDir, err := os.UserHomeDir()
	ui.Fatal(err)
	configPath := filepath.Join(homeDir, ".aws", "config")

	// Load existing config
	cfg, err := ini.LoadSources(ini.LoadOptions{
		SkipUnrecognizableLines: true,
	}, configPath)
	if err != nil {
		// If file doesn't exist, create an empty one instead of crashing
		cfg = ini.Empty()
	}

	// Read sso_start_url and sso_region from the existing sso-session block
	startURL, ssoRegion, err := GetAwsSsoSessionDetails(ssoSessionName)
	if err != nil {
		ui.Fatal(fmt.Errorf("cannot generate profiles: %w — ensure [sso-session %s] exists in ~/.aws/config with sso_start_url and sso_region set", err, ssoSessionName))
	}

	// Ensure the sso-session block is present and complete in the config
	if err := EnsureAwsSsoSession(cfg, ssoSessionName, startURL, ssoRegion); err != nil {
		ui.Fatal(err)
	}

	// Iterate through accounts and create/update sections
	for _, acc := range accounts {
		sectionName := fmt.Sprintf("profile %s-sso", acc.AccountName)

		// This will fetch the section if it exists, or create it if it doesn't
		section, err := cfg.GetSection(sectionName)
		if err != nil {
			// Section doesn't exist, so we create it
			section, err = cfg.NewSection(sectionName)
			if err != nil {
				ui.Fatal(err)
			}
		}

		// Set the keys (this will overwrite existing keys or append new ones)
		section.Key("sso_session").SetValue(ssoSessionName)
		section.Key("sso_start_url").SetValue(startURL)
		section.Key("sso_region").SetValue(ssoRegion)
		section.Key("sso_account_id").SetValue(acc.AccountID)
		section.Key("sso_role_name").SetValue("AdministratorAccess")
		section.Key("output").SetValue("json")
	}
	// Save the file
	err = cfg.SaveTo(configPath)
	ui.Fatal(err)
}

func SelectAwsProfile(profileName string) {
	env_setting1 := fmt.Sprintf("unset AWS_PROFILE")
	env_setting2 := fmt.Sprintf("unset KUBECONFIG")
	command := fmt.Sprintf("export AWS_PROFILE=%s", profileName)
	os.Setenv("AWS_PROFILE", profileName)
	utils.AddScriptEntry(env_setting1)
	utils.AddScriptEntry(env_setting2)
	utils.AddScriptEntry(command)
}

// ClusterEntry pairs a cluster name with its region for display and selection.
type ClusterEntry struct {
	Region  string
	Cluster string
}

// GenEksKubeConfigAllRegions lists EKS clusters across all AWS regions in parallel,
// presents them in a single interactive picker as "region/cluster", then writes the
// kubeconfig for the selected cluster.
func GenEksKubeConfigAllRegions(profile string) {
	if profile == "" {
		profile = os.Getenv("AWS_PROFILE")
	}
	if profile == "" {
		ui.Fatal(fmt.Errorf("AWS_PROFILE is not set and no profile was provided"))
		return
	}

	spinner := ui.StartSpinner("Scanning all AWS regions for EKS clusters...")
	regionMap := listEKSAllRegions(profile)
	spinner.Stop()

	// Flatten into a sorted list of "region/cluster" labels.
	var entries []ClusterEntry
	for region, clusters := range regionMap {
		for _, c := range clusters {
			entries = append(entries, ClusterEntry{Region: region, Cluster: c})
		}
	}

	if len(entries) == 0 {
		ui.Info("No EKS clusters found across all regions.")
		return
	}

	// Build the picker labels and a lookup map.
	labels := make([]string, 0, len(entries))
	labelToEntry := make(map[string]ClusterEntry, len(entries))
	for _, e := range entries {
		label := fmt.Sprintf("%s / %s", e.Region, e.Cluster)
		labels = append(labels, label)
		labelToEntry[label] = e
	}

	selected := ui.GetSelection(labels...)
	if selected == "" {
		return
	}

	entry := labelToEntry[selected]
	writeEksKubeConfig(profile, entry.Region, entry.Cluster)
}

// ListEKSClustersByRegion lists EKS clusters across all AWS regions in parallel
// and returns a map of cluster name → region. It also renders the result as a
// formatted table.
func ListEKSClustersByRegion(profile string) map[string]string {
	spinner := ui.StartSpinner("Scanning all AWS regions for EKS clusters...")
	regionMap := listEKSAllRegions(profile)
	spinner.Stop()

	clusterToRegion := make(map[string]string)
	for region, clusters := range regionMap {
		for _, cluster := range clusters {
			clusterToRegion[cluster] = region
		}
	}

	if len(clusterToRegion) == 0 {
		ui.Info("No EKS clusters found.")
	} else {
		rows := make([][]string, 0, len(clusterToRegion))
		for cluster, region := range clusterToRegion {
			rows = append(rows, []string{cluster, region})
		}
		ui.Table("EKS Clusters Found", []string{"Cluster", "Region"}, rows)
	}

	return clusterToRegion
}

// listEKSAllRegions queries every region in parallel and returns region→clusters.
func listEKSAllRegions(profile string) map[string][]string {
	const regionTimeout = 5 * time.Second

	type result struct {
		region   string
		clusters []string
		timedOut bool
	}

	regions := awsRegions
	ch := make(chan result, len(regions))
	var wg sync.WaitGroup

	for _, r := range regions {
		wg.Add(1)
		go func(region string) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), regionTimeout)
			defer cancel()

			args := []string{"eks", "list-clusters", "--region", region, "--output", "json"}
			if profile != "" {
				args = append(args, "--profile", profile)
			}
			out, err := exec.CommandContext(ctx, "aws", args...).Output()
			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					ch <- result{region: region, timedOut: true}
				} else {
					ch <- result{region: region}
				}
				return
			}
			var resp struct {
				Clusters []string `json:"clusters"`
			}
			if err := json.Unmarshal(out, &resp); err != nil {
				ch <- result{region: region}
				return
			}
			ch <- result{region: region, clusters: resp.Clusters}
		}(r)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	out := make(map[string][]string)
	var timedOutRegions []string
	for res := range ch {
		if res.timedOut {
			timedOutRegions = append(timedOutRegions, res.region)
			continue
		}
		if len(res.clusters) > 0 {
			out[res.region] = res.clusters
		}
	}

	if len(timedOutRegions) > 0 {
		ui.Warning("%d region(s) timed out after %s:", len(timedOutRegions), regionTimeout)
		ui.List(timedOutRegions)
	}

	return out
}

// WriteEksKubeConfig is the exported wrapper around writeEksKubeConfig.
func WriteEksKubeConfig(profile, region, clusterName string) {
	writeEksKubeConfig(profile, region, clusterName)
}

// writeEksKubeConfig runs aws eks update-kubeconfig for the given cluster/region.
func writeEksKubeConfig(profile, region, clusterName string) {
	home, err := os.UserHomeDir()
	if err != nil {
		ui.Error("Failed to resolve home directory: %v", err)
		return
	}

	kubeconfig := filepath.Join(home, ".kube", "clusters", "aws", clusterName)

	s := ui.StartSpinner(fmt.Sprintf("Writing kubeconfig for %s (%s)...", clusterName, region))

	args := []string{
		"eks", "update-kubeconfig",
		"--region", region,
		"--name", clusterName,
		"--kubeconfig", kubeconfig,
		"--alias", clusterName,
	}
	if profile != "" {
		args = append(args, "--profile", profile)
	}

	cmd := exec.Command("aws", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		s.Fail(fmt.Sprintf("Failed to update kubeconfig: %v", err))
		return
	}

	utils.AddScriptEntry(fmt.Sprintf("export KUBECONFIG=%s", kubeconfig))
	s.StopWithMessage(fmt.Sprintf("KUBECONFIG → %s", kubeconfig))
}

func IsProfileLogin() bool {
	home, _ := os.UserHomeDir()
	cacheDirs := []string{
		filepath.Join(home, ".aws", "cli", "cache"),
		// filepath.Join(home, ".aws", "sso", "cache"),
	}

	for _, cacheDir := range cacheDirs {
		files, err := os.ReadDir(cacheDir)
		if err != nil {
			continue
		}

		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".json") {
				continue
			}

			data, _ := os.ReadFile(filepath.Join(cacheDir, file.Name()))

			// AWS SSO files are often keyed by a hash of the StartURL,
			// not the Profile Name.
			// Check if 'ExpiresAt' or 'Expiration' exists and is in the future.
			var creds struct {
				ExpiresAt   string `json:"expiresAt"`
				Credentials struct {
					Expiration string `json:"Expiration"`
				} `json:"Credentials"`
			}

			json.Unmarshal(data, &creds)

			// Check both possible JSON paths for expiration
			expiryStr := creds.ExpiresAt
			if expiryStr == "" {
				expiryStr = creds.Credentials.Expiration
			}

			if expiryStr != "" {
				t, err := parseAwsExpiry(expiryStr)
				if err == nil && time.Now().Before(t) {
					// Note: This still doesn't guarantee it's the RIGHT profile,
					// just that SOME session is active.
					return IsAwsSsoTokenValid()
				}
			}
		}
	}
	return false
}

func UpdateProfileLogin() {
	profile := os.Getenv("AWS_PROFILE")
	cmd := exec.Command("aws", "sso", "login", "--profile", profile)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		ui.Fatal(err)
	}
}

func LoginAwsCalled(selection string) {
	// If not, trigger the interactive selection.
	if selection == "" {
		ssoSessions := ListAwsSsoSessionsName()
		selection = ui.GetSelection(ssoSessions...)
	}
	TriggerAwsSsoLogin(selection)
}

func AwsGetSunny() {
	utils.AddScriptEntry("unset AWS_PROFILE")
	utils.AddScriptEntry("unset KUBECONFIG")
}
