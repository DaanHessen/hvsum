package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
	"github.com/microcosm-cc/bluemonday"
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
	// Fallback to stripping HTML from raw content if readability fails to extract clean text
	if strings.TrimSpace(textContent) == "" && article.Content != "" {
		DebugLog(nil, "Readability text content is empty, falling back to stripping HTML from raw content.")
		// Use bluemonday to strip all HTML tags for a simple text-only version
		p := bluemonday.StripTagsPolicy()
		textContent = p.Sanitize(article.Content)
	}

	if strings.TrimSpace(textContent) == "" {
		return "", pageTitle, fmt.Errorf("failed to extract any meaningful content from the URL")
	}

	return textContent, pageTitle, nil
}
