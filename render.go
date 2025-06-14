package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/glamour"
)

// RenderWithPager displays content using the system pager
func RenderWithPager(content string, useMarkdown bool) {
	finalContent := content
	if useMarkdown {
		rendered, err := glamour.Render(content, "auto")
		if err != nil {
			fmt.Printf("Markdown rendering failed. Using raw output:\n\n")
		} else {
			finalContent = rendered
		}
	}

	// Invoke less with options that:
	//   • Keep ANSI colours (-R)
	//   • Chop long lines rather than wrap (-S)
	//   • Quit automatically if the output fits one screen (-F)
	//   • Keep the buffer on screen after exit (-X)
	//   • Quit automatically as soon as end-of-file is reached (-E) so users don't need to press "q"
	//   • Override the prompt strings (-Ps, -Pm, -PM) with a single whitespace to hide the default ':' prompt
	lessArgs := []string{
		"-R",                   // render colour sequences raw
		"-S",                   // chop long lines
		"-F",                   // quit if one screen
		"-X",                   // do not clear the screen on exit
		"-E",                   // quit at end-of-file automatically
		"-Ps ", "-Pm ", "-PM ", // blank prompts to suppress ':' window
	}

	cmd := exec.Command("less", lessArgs...)
	cmd.Stdin = strings.NewReader(finalContent)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set environment variables for less behavior
	env := os.Environ()
	env = append(env, "LESS=-R -S -F -X -E -Ps  -Pm  -PM ")
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		// Fallback to direct output if less fails
		fmt.Print(finalContent)
	}
}

// RenderToConsole displays content directly to the console
func RenderToConsole(content string, useMarkdown bool) {
	finalContent := content
	if useMarkdown {
		rendered, err := glamour.Render(content, "auto")
		if err != nil {
			fmt.Printf("Markdown rendering failed. Using raw output:\n\n%s\n", content)
		} else {
			finalContent = rendered
		}
	}
	fmt.Print(finalContent)
	fmt.Println() // Add a newline for better spacing
}
