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
	// Use the glamour's WithStandardStyle option.
	// We can't customize it as deeply as I thought without defining a full JSON stylesheet.
	// This approach uses the built-in "dark" theme which is a good starting point.
	return glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(100),
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
		finalContent = wordwrap.String(content, 100)
	}

	if !forceNoPager {
		// Invoke less with options
		lessArgs := []string{
			"-R", "-S", "-F", "-X", "-E", "--quit-on-intr", "--mouse",
			"-Ps ", "-Pm ", "-PM ",
		}
		cmd := exec.Command("less", lessArgs...)
		cmd.Stdin = strings.NewReader(finalContent)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = append(os.Environ(), "LESSCHARSET=utf-8")

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
