package azure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyncAuthenticationCacheCopiesOnlyAuthenticationFiles(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()

	for _, name := range authenticationCacheFiles {
		if err := os.WriteFile(filepath.Join(source, name), []byte(name+" contents"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "azureProfile.json"), []byte("shell subscription"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "azureProfile.json"), []byte("persistent subscription"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := syncAuthenticationCache(source, destination); err != nil {
		t.Fatal(err)
	}

	for _, name := range authenticationCacheFiles {
		data, err := os.ReadFile(filepath.Join(destination, name))
		if err != nil {
			t.Fatal(err)
		}
		if got, want := string(data), name+" contents"; got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	data, err := os.ReadFile(filepath.Join(destination, "azureProfile.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "persistent subscription"; got != want {
		t.Errorf("azureProfile.json = %q, want %q", got, want)
	}
}

func TestSyncCurrentAuthenticationCacheUsesTenantMarker(t *testing.T) {
	shellDir := t.TempDir()
	tenantDir := t.TempDir()
	t.Setenv("AZURE_CONFIG_DIR", shellDir)

	if err := os.WriteFile(filepath.Join(shellDir, tenantConfigMarker), []byte(tenantDir), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellDir, "msal_token_cache.json"), []byte("token"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := syncCurrentAuthenticationCache(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(tenantDir, "msal_token_cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "token"; got != want {
		t.Errorf("token cache = %q, want %q", got, want)
	}
}
