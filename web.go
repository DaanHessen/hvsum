package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

// ExtractWebContent fetches and extracts clean content from a URL
func ExtractWebContent(urlStr string) (string, string, error) {
	// Add https:// if no protocol is specified
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "https://" + urlStr
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse URL: %w", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(urlStr)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	article, err := readability.FromReader(resp.Body, parsedURL)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse content: %v", err)
	}

	pageTitle := article.Title
	if pageTitle == "" {
		pageTitle = "Web Page Summary"
	}

	textContent := article.TextContent
	// Fallback to raw text if readability fails to extract meaningful content
	if strings.TrimSpace(textContent) == "" {
		textContent = article.Content
	}

	return textContent, pageTitle, nil
}
