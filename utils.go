package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/atotto/clipboard"
)

// DebugLog prints debug messages if debug mode is enabled
func DebugLog(config *Config, format string, args ...interface{}) {
	if config != nil && config.DebugMode {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

// IsValidURL checks if the input string is a valid URL
func IsValidURL(input string) bool {
	// Check if it starts with http:// or https://
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		_, err := url.Parse(input)
		return err == nil
	}

	// Check if it looks like a domain (contains a dot and no spaces)
	if strings.Contains(input, ".") && !strings.Contains(input, " ") {
		// Try to parse it as a URL with https:// prefix
		_, err := url.Parse("https://" + input)
		return err == nil
	}

	return false
}

// CopyToClipboard copies text to the system clipboard
func CopyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}

// SaveToFile saves text to a file
func SaveToFile(filename, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}

// Min returns the minimum of two integers
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max returns the maximum of two integers
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// TruncateString truncates a string to a maximum length
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
