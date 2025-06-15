package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/muesli/reflow/wordwrap"
)

func createCustomRenderer() (*glamour.TermRenderer, error) {
	// Use glamour's auto style for better terminal compatibility with glow-like appearance
	// This provides the rich formatting with highlighted headers, proper colors, etc.
	return glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(120),
	)
}

// RenderContent handles all rendering, including pager and markdown.
func RenderContent(content string, useMarkdown bool, forceNoPager bool) {
	finalContent := content
	if useMarkdown {
		r, err := createCustomRenderer()
		if err == nil {
			rendered, renderErr := r.Render(content)
			if renderErr == nil {
				finalContent = rendered
			} else {
				// Don't print an error, just fall back to raw.
				// The error is often about not being in a TTY, which is not critical.
			}
		}
	} else if forceNoPager { // Only wrap non-markdown when not using a pager
		finalContent = wordwrap.String(content, 120)
	}

	if !forceNoPager {
		// Simple, clean less configuration for markdown rendering
		lessArgs := []string{
			"-R",             // Enable raw control characters (for colors)
			"-S",             // Chop long lines instead of wrapping
			"-F",             // Quit if entire file fits on screen
			"-X",             // Don't send termcap initialization/deinitialization
			"-E",             // Quit at end of file
			"--quit-on-intr", // Quit on interrupt
			"--mouse",        // Enable mouse
			"-M",             // Long prompt with percentage
		}

		cmd := exec.Command("less", lessArgs...)
		cmd.Stdin = strings.NewReader(finalContent)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = append(os.Environ(),
			"LESSCHARSET=utf-8",
			"LESS_TERMCAP_md=\033[1;36m",    // Bold cyan for headers
			"LESS_TERMCAP_me=\033[0m",       // End bold
			"LESS_TERMCAP_so=\033[1;44;37m", // Standout (search highlights)
			"LESS_TERMCAP_se=\033[0m",       // End standout
			"LESS_TERMCAP_us=\033[1;32m",    // Underline (green)
			"LESS_TERMCAP_ue=\033[0m",       // End underline
		)

		if err := cmd.Run(); err != nil {
			// Fallback to direct console output if less fails
			fmt.Print(finalContent)
			fmt.Println()
		}
	} else {
		fmt.Print(finalContent)
		fmt.Println() // Add a newline for better spacing
	}
}

// RenderToConsole is a convenience wrapper for interactive sessions that don't use the pager.
func RenderToConsole(content string, useMarkdown bool) {
	RenderContent(content, useMarkdown, true)
}
