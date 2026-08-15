package ui

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GetSelection presents an interactive fzf picker for the provided options and
// returns the selected string trimmed of whitespace.
//
// If no options are provided it falls back to a plain stdin prompt, which is
// useful during testing or when fzf is unavailable.
func GetSelection(options ...string) string {
	if len(options) == 0 {
		fmt.Print("Enter selection: ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			Fatal(err)
			return ""
		}
		return strings.TrimSpace(input)
	}

	fzfPath, err := exec.LookPath("fzf")
	if err != nil {
		Fatal(fmt.Errorf("fzf not found in PATH: %w", err))
		return ""
	}

	return runFzf(fzfPath, options)
}

// runFzf launches fzf with the given options and returns the selected entry.
// stderr is forwarded to the terminal so fzf can render its interactive UI.
// stdout captures the single selected line.
func runFzf(path string, options []string) string {
	cmd := exec.Command(path, "--height", "18", "--reverse", "--border")
	cmd.Stdin = strings.NewReader(strings.Join(options, "\n"))

	var out bytes.Buffer
	cmd.Stdout = &out
	// fzf renders its TUI on stderr; forwarding it lets the user see the picker.
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		Fatal(err)
		return ""
	}

	return strings.TrimSpace(out.String())
}
