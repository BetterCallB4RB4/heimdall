package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// AddScriptEntry appends command to ~/generated_script.sh so the parent shell
// can source it and receive environment variable changes after heimdall exits.
// This is the shell-integration mechanism that allows heimdall to modify the
// calling shell's environment (export AWS_PROFILE, AZURE_CONFIG_DIR, etc.)
// even though a child process cannot directly mutate its parent's environment.
func AddScriptEntry(command string) {
	scriptPath := os.Getenv("HEIMDALL_SCRIPT")
	if scriptPath == "" {
		home, _ := os.UserHomeDir()
		scriptPath = filepath.Join(home, "generated_script.sh")
	}
	f, err := os.OpenFile(scriptPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening script file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	if _, err := f.WriteString(command + "\n"); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing to script file: %v\n", err)
		os.Exit(1)
	}
}
