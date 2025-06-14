package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/muesli/reflow/wordwrap"
)

// RenderWithPager displays content using the system pager
func RenderWithPager(content string, useMarkdown bool) {
	finalContent := content
	if useMarkdown {
		// Use a specific style for better readability
		r, _ := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(100),
		)
		rendered, err := r.Render(content)
		if err != nil {
			fmt.Printf("Markdown rendering failed. Using raw output:\n\n")
		} else {
			finalContent = rendered
		}
	}

	// Invoke less with options
	lessArgs := []string{
		"-R",                   // render colour sequences raw
		"-S",                   // chop long lines
		"-F",                   // quit if one screen
		"-X",                   // do not clear the screen on exit
		"-E",                   // quit at end-of-file automatically
		"--quit-on-intr",       // quit on interrupt (Ctrl+C)
		"--mouse",              // enable mouse scrolling
		"-Ps ", "-Pm ", "-PM ", // blank prompts to suppress ':' window
	}

	cmd := exec.Command("less", lessArgs...)
	cmd.Stdin = strings.NewReader(finalContent)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set environment variables for less behavior
	env := os.Environ()
	env = append(env, "LESSCHARSET=utf-8")
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		// Fallback to direct console output if less fails
		RenderToConsole(content, useMarkdown)
	}
}

// RenderToConsole displays content directly to the console with word wrapping
func RenderToConsole(content string, useMarkdown bool) {
	finalContent := content
	if useMarkdown {
		r, _ := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(100),
		)
		rendered, err := r.Render(content)
		if err != nil {
			fmt.Printf("Markdown rendering failed. Using raw output:\n\n%s\n", content)
			finalContent = content
		} else {
			finalContent = rendered
		}
	} else {
		// Wrap non-markdown text for better console readability
		finalContent = wordwrap.String(content, 100)
	}

	fmt.Print(finalContent)
	fmt.Println() // Add a newline for better spacing
}
