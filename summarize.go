package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ollama/ollama/api"
)

// Length definitions for precise length control
var lengthMap = map[string]string{
	"short":    "Provide a response that is **exactly 2 sentences long**. Your entire output must be contained within two sentences. This is a strict requirement.",
	"medium":   "Provide a response that is **between 4 and 6 sentences long**. Aim for clarity and conciseness within this range. This is a strict requirement.",
	"long":     "Provide a comprehensive response that is **between 8 and 10 sentences long**. Cover the topic in detail within this range. This is a strict requirement.",
	"detailed": "Provide a highly detailed response that is **between 12 and 15 sentences long**. Explore the topic thoroughly with examples and context. This is a strict requirement.",
}

// ProcessURL handles URL-based summarization with optional search enhancement
func ProcessURL(urlStr string, config *Config, length string, useMarkdown, enableSearch bool) (string, error) {
	fmt.Fprintf(os.Stderr, "🌐 Fetching content from: %s\n", urlStr)

	// Extract content from URL
	content, title, err := ExtractWebContent(urlStr)
	if err != nil {
		return "", fmt.Errorf("failed to extract content: %v", err)
	}

	DebugLog(config, "Extracted %d characters from URL", len(content))
	DebugLog(config, "Page title: %s", title)

	// Generate summary with optional search enhancement
	return generateSummary(config, length, useMarkdown, enableSearch, content, title, false)
}

// ProcessSearchQuery handles search-only summarization
func ProcessSearchQuery(query string, config *Config, length string, useMarkdown bool) (string, error) {
	fmt.Fprintf(os.Stderr, "🔍 Performing web search for: %s\n", query)

	// Create search manager and perform searches
	searchManager := NewSearchManager(config)

	// Generate related search queries for comprehensive coverage
	relatedQueries, err := generateSearchQueries(config, query, "provide comprehensive information about this topic")
	if err != nil {
		DebugLog(config, "Failed to generate related queries: %v", err)
		relatedQueries = []string{query}
	} else {
		relatedQueries = append([]string{query}, relatedQueries...)
		DebugLog(config, "Generated %d total queries for search", len(relatedQueries))
	}

	// Perform parallel searches
	fmt.Fprintf(os.Stderr, "🚀 Performing parallel web searches...\n")
	searchResults := searchManager.PerformParallelSearches(relatedQueries, 3)

	if len(searchResults) == 0 {
		return "", fmt.Errorf("no search results found for query: %s", query)
	}

	DebugLog(config, "Found %d total search results", len(searchResults))

	// Generate summary from search results
	return generateSearchOnlySummary(config, length, useMarkdown, query, searchResults)
}

// generateSummary creates a summary with optional search enhancement
func generateSummary(config *Config, length string, useMarkdown, enableSearch bool, content, title string, isSearchOnly bool) (string, error) {
	DebugLog(config, "Generating summary with search enabled: %v", enableSearch)

	systemPrompt := config.SystemPrompts.Summary
	if useMarkdown {
		systemPrompt += "\n\n" + config.SystemPrompts.Markdown
	}

	var userPrompt string
	var searchResults []SearchResult

	if enableSearch && !isSearchOnly {
		fmt.Fprintf(os.Stderr, "🔍 Enhancing summary with web search...\n")

		// Generate search queries based on content
		searchManager := NewSearchManager(config)
		queries, err := generateSearchQueries(config, content[:Min(1000, len(content))], "enhance this content summary")
		if err != nil {
			DebugLog(config, "Search query generation failed: %v", err)
		} else {
			fmt.Fprintf(os.Stderr, "🚀 Performing parallel searches...\n")
			searchResults = searchManager.PerformParallelSearches(queries, 2)
			DebugLog(config, "Enhanced with %d search results", len(searchResults))
		}
	}

	userPrompt = buildUserPrompt("", length, content, title)
	if len(searchResults) > 0 {
		userPrompt += FormatSearchResults(searchResults)
		userPrompt += "\n\nUse both the webpage content and the search results to create a comprehensive summary."
	}

	fmt.Fprintf(os.Stderr, "🤖 Generating summary with %s...\n\n", config.DefaultModel)

	return callOllama(config, systemPrompt, userPrompt)
}

// generateSearchOnlySummary creates a summary based purely on search results
func generateSearchOnlySummary(config *Config, length string, useMarkdown bool, query string, searchResults []SearchResult) (string, error) {
	DebugLog(config, "Generating search-only summary for: %s", query)

	systemPrompt := config.SystemPrompts.SearchOnly
	if useMarkdown {
		systemPrompt += "\n\n" + config.SystemPrompts.Markdown
	}

	userPrompt := buildSearchOnlyPrompt(query, length, searchResults)

	fmt.Fprintf(os.Stderr, "🤖 Generating comprehensive summary with %s...\n\n", config.DefaultModel)

	return callOllama(config, systemPrompt, userPrompt)
}

// generateSearchQueries uses AI to generate relevant search queries
func generateSearchQueries(config *Config, contextText, purpose string) ([]string, error) {
	DebugLog(config, "Generating search queries for: %.100s...", contextText)

	prompt := fmt.Sprintf(`Based on the following context and purpose, generate 2-3 specific web search queries that would help gather additional relevant information. Return only the search queries, one per line, without numbering or additional text.

Context: %s

Purpose: %s

Generate search queries:`, contextText, purpose)

	response, err := callOllama(config, config.SystemPrompts.SearchQuery, prompt)
	if err != nil {
		return nil, err
	}

	queries := strings.Split(strings.TrimSpace(response), "\n")
	var cleanQueries []string
	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query != "" {
			cleanQueries = append(cleanQueries, query)
		}
	}

	DebugLog(config, "Generated %d search queries: %v", len(cleanQueries), cleanQueries)
	return cleanQueries, nil
}

// buildUserPrompt creates a prompt for content summarization
func buildUserPrompt(userMessage, length, textContent, pageTitle string) string {
	lengthInstruction, exists := lengthMap[length]
	if !exists {
		lengthInstruction = lengthMap["medium"]
	}

	var instruction string
	if userMessage != "" {
		instruction = fmt.Sprintf(`Your task is to answer the following question based on the webpage content.

Question: "%s"

CRITICAL LENGTH REQUIREMENT: %s

IMPORTANT: Count your sentences/paragraphs as you write. When you reach the exact limit specified above, STOP immediately. Do not exceed the limit under any circumstances.

Page title: %s

REMINDER: Follow the length requirement exactly. Count as you go and stop when you reach the limit.`, userMessage, lengthInstruction, pageTitle)
	} else {
		instruction = fmt.Sprintf(`Your task is to create a comprehensive summary of the following webpage content.

CRITICAL LENGTH REQUIREMENT: %s

IMPORTANT: Count your sentences/paragraphs as you write. When you reach the exact limit specified above, STOP immediately. Do not exceed the limit under any circumstances.

Page title: %s

Focus on the main content, ignore navigation, footers, and boilerplate text.

REMINDER: Follow the length requirement exactly. Count as you go and stop when you reach the limit.`, lengthInstruction, pageTitle)
	}

	return fmt.Sprintf("%s\n\n--- WEBPAGE CONTENT ---\n%s", instruction, textContent)
}

// buildSearchOnlyPrompt creates a prompt for search-only summarization
func buildSearchOnlyPrompt(query, length string, results []SearchResult) string {
	lengthInstruction, exists := lengthMap[length]
	if !exists {
		lengthInstruction = lengthMap["medium"]
	}

	prompt := fmt.Sprintf(`Your task is to create a comprehensive summary based on web search results for the query: "%s"

CRITICAL LENGTH REQUIREMENT: %s

IMPORTANT: Count your sentences/paragraphs as you write. When you reach the exact limit specified above, STOP immediately. Do not exceed the limit under any circumstances.

Focus on providing accurate, factual information based on the search results below. Synthesize the information to create a coherent and informative summary.

REMINDER: Follow the length requirement exactly. Count as you go and stop when you reach the limit.

%s`, query, lengthInstruction, FormatSearchResults(results))

	return prompt
}

// callOllama makes a call to the Ollama API
func callOllama(config *Config, systemPrompt, userPrompt string) (string, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return "", fmt.Errorf("failed to connect to Ollama: %v", err)
	}

	stream := false
	req := &api.GenerateRequest{
		Model:  config.DefaultModel,
		System: systemPrompt,
		Prompt: userPrompt,
		Stream: &stream,
	}

	var responseBuilder strings.Builder
	err = client.Generate(context.Background(), req, func(resp api.GenerateResponse) error {
		responseBuilder.WriteString(resp.Response)
		return nil
	})

	if err != nil {
		if strings.Contains(err.Error(), "model '"+config.DefaultModel+"' not found") {
			return "", fmt.Errorf("model '%s' not found. Please run: ollama pull %s", config.DefaultModel, config.DefaultModel)
		}
		return "", fmt.Errorf("generation failed: %v", err)
	}

	return responseBuilder.String(), nil
}
