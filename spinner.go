package main

import (
	"fmt"
	"os"
	"time"
)

// StartSpinner starts a more robust CLI spinner that properly clears the line.
func StartSpinner(message string) chan struct{} {
	stop := make(chan struct{})
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
