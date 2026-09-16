package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
)

// GetSelection presents an interactive, filterable picker for the provided options and
// returns the selected string trimmed of whitespace.
//
// If no options are provided it falls back to a plain stdin prompt.
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

	selection, err := pterm.DefaultInteractiveSelect.
		WithOptions(options).
		WithFilter().
		WithMaxHeight(18).
		Show()
	if err != nil {
		Fatal(err)
		return ""
	}

	return strings.TrimSpace(selection)
}
