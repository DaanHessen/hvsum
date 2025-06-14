package main

import (
	"fmt"
	"os"
	"time"
)

// StartSpinner prints a simple rotating spinner until the returned
// stop channel is closed. It writes to stderr to avoid mixing with streamed content.
func StartSpinner(message string) chan struct{} {
	stop := make(chan struct{})
	go func() {
		frames := []rune{'|', '/', '-', '\\'}
		idx := 0
		fmt.Fprintf(stderrWriter(), "%s ", message)
		for {
			select {
			case <-stop:
				fmt.Fprintf(stderrWriter(), "\r%s ✓\n", message)
				return
			default:
				fmt.Fprintf(stderrWriter(), "\r%s %c", message, frames[idx%len(frames)])
				idx++
				time.Sleep(120 * time.Millisecond)
			}
		}
	}()
	return stop
}

// stderrWriter returns os.Stderr but is wrapped for testing/mocking
func stderrWriter() *os.File {
	return os.Stderr
}
