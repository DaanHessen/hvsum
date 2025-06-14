package main

import (
	"fmt"
	"os"
	"time"
)

// isOutputRedirected checks if stderr is being redirected (like in tests)
func isOutputRedirected() bool {
	stat, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	// If stderr is not a character device (terminal), it's likely redirected
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// StartSpinner starts a more robust CLI spinner that properly clears the line.
// Automatically disables when output is redirected to prevent log pollution.
func StartSpinner(message string) chan struct{} {
	stop := make(chan struct{})

	// If output is redirected (like in tests), don't show spinner
	if isOutputRedirected() {
		go func() {
			<-stop // Just wait for stop signal
		}()
		return stop
	}

	go func() {
		spinner := `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`
		i := 0
		for {
			select {
			case <-stop:
				// Clear the line and print the final message
				fmt.Fprintf(os.Stderr, "\r\033[K%s ✓\n", message)
				return
			default:
				// Use \r (carriage return) to go to the start of the line and \033[K to clear it
				fmt.Fprintf(os.Stderr, "\r\033[K%s %c ", message, spinner[i])
				time.Sleep(80 * time.Millisecond)
				i = (i + 1) % len(spinner)
			}
		}
	}()
	return stop
}

// stderrWriter returns os.Stderr but is wrapped for testing/mocking
func stderrWriter() *os.File {
	return os.Stderr
}
