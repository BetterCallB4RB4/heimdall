// Package ui is the single source of truth for all terminal output in heimdall.
// Providers and commands import only this package for anything the user sees;
// they never import pterm or fmt for display purposes directly.
// Swapping the rendering backend in the future only requires changes here.
package ui

import (
	"fmt"

	"github.com/pterm/pterm"
)

// ---------------------------------------------------------------------------
// Spinner
// ---------------------------------------------------------------------------

// Spinner wraps pterm's SpinnerPrinter so callers do not need to import pterm.
type Spinner struct {
	s *pterm.SpinnerPrinter
}

// StartSpinner starts an animated spinner with the given message and returns a
// handle. Call Stop, StopWithMessage, or Fail on the handle when the operation
// completes to replace the animation with a result indicator.
func StartSpinner(msg string) Spinner {
	s, _ := pterm.DefaultSpinner.Start(msg)
	return Spinner{s: s}
}

// Stop marks the spinner as succeeded (green checkmark) and stops the animation.
func (sp Spinner) Stop() {
	if sp.s != nil {
		sp.s.Success()
	}
}

// StopWithMessage marks the spinner as succeeded and replaces the original
// message with msg.
func (sp Spinner) StopWithMessage(msg string) {
	if sp.s != nil {
		sp.s.Success(msg)
	}
}

// Fail marks the spinner as failed (red ✗) and replaces the message with msg.
func (sp Spinner) Fail(msg string) {
	if sp.s != nil {
		sp.s.Fail(msg)
	}
}

// ---------------------------------------------------------------------------
// Prefixed message printers
// ---------------------------------------------------------------------------

// Info prints a blue informational message.
func Info(format string, args ...any) {
	pterm.Info.Println(fmt.Sprintf(format, args...))
}

// Success prints a green success confirmation.
func Success(format string, args ...any) {
	pterm.Success.Println(fmt.Sprintf(format, args...))
}

// Warning prints a yellow warning.
func Warning(format string, args ...any) {
	pterm.Warning.Println(fmt.Sprintf(format, args...))
}

// Error prints a red non-fatal error. Execution continues after this call.
func Error(format string, args ...any) {
	pterm.Error.Println(fmt.Sprintf(format, args...))
}

// Fatal prints a red fatal error and exits with code 1.
// It is a no-op when err is nil, matching the utils.FatalErr contract.
func Fatal(err error) {
	if err != nil {
		pterm.Fatal.Println(err)
	}
}

// ---------------------------------------------------------------------------
// Structured display helpers
// ---------------------------------------------------------------------------

// Header prints a full-width styled banner — use for top-level section titles.
func Header(title string) {
	pterm.DefaultHeader.WithFullWidth().Println(title)
}

// Section prints a styled sub-section label with a separator line.
func Section(title string) {
	pterm.DefaultSection.Println(title)
}

// Box prints an ordered set of key/value pairs inside a labeled bordered box.
//
// Example:
//
//	ui.Box("AWS SSO Login", [][2]string{
//	    {"Verification URL", url},
//	    {"User Code",        code},
//	})
func Box(title string, fields [][2]string) {
	var content string
	for _, kv := range fields {
		content += fmt.Sprintf("  %-20s %s\n", kv[0]+" :", kv[1])
	}
	pterm.DefaultBox.WithTitle(title).Println(content)
}

// Table prints a formatted table with a header row.
// An optional section title is printed above the table when non-empty.
func Table(title string, headers []string, rows [][]string) {
	if title != "" {
		pterm.DefaultSection.Println(title)
	}
	tableData := pterm.TableData{headers}
	for _, r := range rows {
		tableData = append(tableData, r)
	}
	if err := pterm.DefaultTable.WithHasHeader().WithData(tableData).Render(); err != nil {
		pterm.Error.Println(err)
	}
}

// List prints a bulleted list of items.
func List(items []string) {
	bulletItems := make([]pterm.BulletListItem, 0, len(items))
	for _, item := range items {
		bulletItems = append(bulletItems, pterm.BulletListItem{Level: 0, Text: item})
	}
	if err := pterm.DefaultBulletList.WithItems(bulletItems).Render(); err != nil {
		pterm.Error.Println(err)
	}
}
