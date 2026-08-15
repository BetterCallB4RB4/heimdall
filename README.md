# heimdall

`heimdall` manages cloud sessions and Kubernetes configs for AWS and Azure. It
handles SSO authentication, subscription/profile selection, and writes
kubeconfig files for EKS and AKS clusters — all scoped to the current shell
via a generated shell-integration script.

- `heimdall aws` — AWS profile selection, SSO login, EKS kubeconfig generation
- `heimdall az` — Azure tenant/subscription selection, AKS kubeconfig generation
- `heimdall getSunny` — clear all cloud context from the current shell

## Installation

This repository is **private**. You need to be added as a collaborator before
you can download a release.

### Release binary (recommended)

Download the archive for your Linux architecture from the repository's
[Releases](https://github.com/BetterCallB4RB4/heimdall/releases) page. Extract
it and put `heimdall` somewhere on your `PATH`:

```sh
tar -xzf heimdall_<version>_linux_<architecture>.tar.gz
install -m 0755 heimdall ~/.local/bin/heimdall
```

Releases support `amd64` (Intel/AMD 64-bit) and `arm64` (64-bit ARM) Linux.
Verify your download against the `checksums.txt` file included in every release.

### Install a release

Choose the release archive that matches your machine:

```sh
uname -m
```

Use the `linux_amd64` archive for `x86_64` systems and the `linux_arm64` archive
for `aarch64` systems. Download the archive from the
[Releases](https://github.com/BetterCallB4RB4/heimdall/releases) page, then
install it for your user without `sudo`:

```sh
mkdir -p ~/.local/bin
tar -xzf heimdall_<version>_linux_amd64.tar.gz
install -m 0755 heimdall ~/.local/bin/heimdall
rm heimdall
```

Ensure `~/.local/bin` is on your `PATH`:

```sh
echo "$PATH"
```

If it is absent, add the following line to `~/.bashrc` or `~/.zshrc`, then open
a new terminal:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

Verify the installation:

```sh
heimdall --version
```

To upgrade, repeat the installation steps with a newer release archive.

### Go installation

Requires Go 1.24+ and access to this private repository:

```sh
go install github.com/BetterCallB4RB4/heimdall@latest
```

This installs the binary to `$(go env GOPATH)/bin`; make sure that directory is
on your `PATH`.

### Dependencies

Install the tools needed for the commands you use:

- `aws` for AWS commands
- `az` for Azure commands
- `fzf` for interactive selection
- `kubelogin` for AKS kubeconfig generation
- [Starship](https://starship.rs/) (optional) to display the active AWS, Azure,
  and Kubernetes context in your shell prompt

### Optional Starship prompt

Add the following to `~/.config/starship.toml` to show the cloud account and
Kubernetes context selected by Heimdall. Starship reads the environment values
set by the required shell integration below; it is not required to run
Heimdall.

```toml
format = """
$directory\
$git_branch\
$git_status\
$aws\
$azure\
$kubernetes\
$line_break\
$character"""

[git_branch]
format = '[$symbol$branch(:$remote_branch)]($style) '

[aws]
symbol = "󰸏"
format = ' [$symbol ($profile)(\[$duration\])]($style)  '

[azure]
disabled = false
symbol = "󰠅"
format = '[$symbol ($subscription)]($style)  '

[kubernetes]
disabled = false
symbol = "󱃾"
format = '[$symbol $context]($style)  '
```

### Optional AWS region configuration

Heimdall has built-in mappings for common account-name tokens, including `DE`,
`IT`, and `US`. To add or override mappings, create
`~/.config/heimdall.yaml`. This file is user-local and is not included in the
repository. If it is absent, Heimdall uses its built-in defaults.

```yaml
aws:
  extra_regions:
    PROD: eu-central-1
    STG: eu-west-1
    DE: eu-west-1 # Overrides the built-in DE mapping.

  region_rules:
    - pattern: "(?i).*milan.*"
      region: eu-south-1
```

`extra_regions` matches dash-separated account-name tokens, while
`region_rules` are checked in order when no token matches.

## Shell integration (required)

`heimdall` needs to export environment variables (e.g. `AWS_PROFILE`) and run
`az account set` into your **current** shell. Since a subprocess can't modify
its parent shell's environment, `heimdall` writes commands to a temp script
that the shell function below sources back into your session.

Add this to your `~/.bashrc` / `~/.zshrc`:

```sh
heimdall() {
    local TMP_SCRIPT
    TMP_SCRIPT="$(mktemp /tmp/heimdall_script.XXXXXX)"

    HEIMDALL_SCRIPT="$TMP_SCRIPT" command heimdall "$@"

    if [[ -s "$TMP_SCRIPT" ]]; then
        source "$TMP_SCRIPT"
    fi
    rm -f "$TMP_SCRIPT"
}
```

## Releases

Build locally to verify the CLI. This creates a local binary only; do not add
it to Git because release binaries are built by GitHub Actions.

```sh
go build -o heimdall .
./heimdall --version
rm heimdall
```

The local binary reports `heimdall version dev`. Commit and push your source
and release configuration changes before creating a release tag:

```sh
git add .
git commit -m "add Linux release automation"
git push origin main
```

Create and push a version tag to publish a private GitHub Release with Linux
binaries:

```sh
git tag v0.1.0
git push origin v0.1.0
```

`git tag v0.1.0` creates a local label for the current commit. `git push origin
v0.1.0` uploads that label to GitHub, triggering the release workflow. Use a
new version for each release, such as `v0.1.1` for a bug fix.

Release downloads report their tag version, for example `heimdall version
v0.1.0`. Check an installed version with `heimdall --version`.

## AWS SSO prerequisite

Add your SSO session to `~/.aws/config` (replace with your own start URL):

```ini
[sso-session default-sso-login]
sso_start_url = https://d-xxxxxxxxxx.awsapps.com/start
sso_region = us-east-1
sso_registration_scopes = sso:account:access
```

---

<!--
## Old notes / scratchpad (kept for reference, superseded by sections above)

go install github.com/spf13/cobra-cli@latest

cobra-cli add -command name-

go run .


dirty git commands
git add . && read "msg?Commit message: " && git commit -m "$msg" && git push

-->
