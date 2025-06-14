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
	"short":    "3-5 concise sentences maximum. Focus on the most essential information only.",
	"medium":   "6-10 sentences in 2 clear paragraphs. Cover key points without redundancy.",
	"long":     "15-20 sentences in 3-4 paragraphs. Comprehensive but focused coverage.",
	"detailed": "Thorough summary covering all essential aspects. Be comprehensive but avoid fluff.",
}

// ProcessURL handles URL-based summarization with caching and search enhancement
func ProcessURL(urlStr string, config *Config, length string, useMarkdown, enableSearch bool) (string, error) {
	fmt.Fprintf(os.Stderr, "🌐 Fetching content from: %s\n", urlStr)

	// Initialize cache manager
	cacheManager := NewCacheManager(config)

	// Check cache first
	cacheKey := cacheManager.GetCacheKey(fmt.Sprintf("url:%s:%s:%t:%t", urlStr, length, useMarkdown, enableSearch))
	var cachedSummary string
	if cacheManager.Get(cacheKey, &cachedSummary) {
		DebugLog(config, "Cache hit for URL summary")
		return cachedSummary, nil
	}

	// Extract content from URL
	content, title, err := ExtractWebContent(urlStr)
	if err != nil {
		return "", fmt.Errorf("failed to extract content: %v", err)
	}

	DebugLog(config, "Extracted %d characters from URL", len(content))
	DebugLog(config, "Page title: %s", title)

	// Generate summary with optional search enhancement
	summary, err := generateSummary(config, length, useMarkdown, enableSearch, content, title, urlStr, false)
	if err != nil {
		return "", err
	}

	// Cache the result
	cacheManager.Set(cacheKey, summary)
	return summary, nil
}

// ProcessSearchQuery handles search-only summarization with caching
func ProcessSearchQuery(query string, config *Config, length string, useMarkdown bool) (string, error) {
	fmt.Fprintf(os.Stderr, "🔍 Performing web search for: %s\n", query)

	// Initialize cache manager
	cacheManager := NewCacheManager(config)

	// Check cache first
	cacheKey := cacheManager.GetCacheKey(fmt.Sprintf("search:%s:%s:%t", query, length, useMarkdown))
	var cachedSummary string
	if cacheManager.Get(cacheKey, &cachedSummary) {
		DebugLog(config, "Cache hit for search summary")
		return cachedSummary, nil
	}

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
	summary, err := generateSearchOnlySummary(config, length, useMarkdown, query, searchResults)
	if err != nil {
		return "", err
	}

	// Cache the result
	cacheManager.Set(cacheKey, summary)
	return summary, nil
}

// generateSummary creates a summary with optional search enhancement
func generateSummary(config *Config, length string, useMarkdown, enableSearch bool, content, title, sourceURL string, isSearchOnly bool) (string, error) {
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

	if sourceURL != "" {
		userPrompt += fmt.Sprintf("\n\nSource URL: %s", sourceURL)
	}

	fmt.Fprintf(os.Stderr, "🤖 Generating summary with %s...\n", config.DefaultModel)

	var spinnerStop chan struct{}
	if useMarkdown {
		spinnerStop = StartSpinner("Generating summary")
	}

	summary, err := callOllama(config, systemPrompt, userPrompt)
	if spinnerStop != nil {
		close(spinnerStop)
	}
	return summary, err
}

// generateSearchOnlySummary creates a summary based purely on search results
func generateSearchOnlySummary(config *Config, length string, useMarkdown bool, query string, searchResults []SearchResult) (string, error) {
	DebugLog(config, "Generating search-only summary for: %s", query)

	systemPrompt := config.SystemPrompts.SearchOnly
	if useMarkdown {
		systemPrompt += "\n\n" + config.SystemPrompts.Markdown
	}

	userPrompt := buildSearchOnlyPrompt(query, length, searchResults)

	fmt.Fprintf(os.Stderr, "🤖 Generating comprehensive summary with %s...\n", config.DefaultModel)

	var spinnerStop chan struct{}
	if useMarkdown {
		spinnerStop = StartSpinner("Generating summary")
	}

	summary, err := callOllama(config, systemPrompt, userPrompt)
	if spinnerStop != nil {
		close(spinnerStop)
	}
	return summary, err
}

// generateSearchQueries uses AI to generate relevant search queries with caching
func generateSearchQueries(config *Config, contextText, purpose string) ([]string, error) {
	DebugLog(config, "Generating search queries for: %.100s...", contextText)

	// Check cache first
	cacheManager := NewCacheManager(config)
	cacheKey := cacheManager.GetCacheKey(fmt.Sprintf("queries:%s:%s", contextText[:Min(200, len(contextText))], purpose))
	var cachedQueries []string
	if cacheManager.Get(cacheKey, &cachedQueries) {
		DebugLog(config, "Cache hit for search queries")
		return cachedQueries, nil
	}

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
		if query != "" && len(query) > 5 { // Filter out very short queries
			cleanQueries = append(cleanQueries, query)
		}
	}

	// Cache the results
	cacheManager.Set(cacheKey, cleanQueries)

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
		instruction = fmt.Sprintf(`Answer this question based on the webpage content: "%s"

Length requirement: %s

Page: %s

Focus on accuracy and relevance.`, userMessage, lengthInstruction, pageTitle)
	} else {
		instruction = fmt.Sprintf(`Create a focused summary of this webpage content.

Length requirement: %s
Page: %s

Focus on key information, insights, and actionable content. Ignore navigation, ads, and boilerplate.`, lengthInstruction, pageTitle)
	}

	return fmt.Sprintf("%s\n\n--- CONTENT ---\n%s", instruction, textContent)
}

// buildSearchOnlyPrompt creates a prompt for search-only summarization
func buildSearchOnlyPrompt(query, length string, results []SearchResult) string {
	lengthInstruction, exists := lengthMap[length]
	if !exists {
		lengthInstruction = lengthMap["medium"]
	}

	prompt := fmt.Sprintf(`Create a comprehensive summary for the query: "%s"

Length requirement: %s

Synthesize information from the search results below to provide accurate, factual information. Focus on key facts, current information, and relevant insights.

%s`, query, lengthInstruction, FormatSearchResults(results))

	return prompt
}

// callOllama makes a call to the Ollama API with better error handling
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
		Options: map[string]interface{}{
			"temperature": 0.1, // Lower temperature for more consistent summaries
			"top_p":       0.9,
		},
	}

	var responseBuilder strings.Builder
	err = client.Generate(context.Background(), req, func(resp api.GenerateResponse) error {
		responseBuilder.WriteString(resp.Response)
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to generate response: %v", err)
	}

	response := strings.TrimSpace(responseBuilder.String())
	if response == "" {
		return "", fmt.Errorf("received empty response from model")
	}

	return response, nil
}

// GenerateOutline creates an outline from a summary with caching
func GenerateOutline(summary string, config *Config, useMarkdown bool) (string, error) {
	if summary == "" {
		return "", fmt.Errorf("cannot generate outline from empty summary")
	}

	// Check cache first
	cacheManager := NewCacheManager(config)
	cacheKey := cacheManager.GetCacheKey(fmt.Sprintf("outline:%s:%t", summary[:Min(200, len(summary))], useMarkdown))
	var cachedOutline string
	if cacheManager.Get(cacheKey, &cachedOutline) {
		DebugLog(config, "Cache hit for outline")
		return cachedOutline, nil
	}

	systemPrompt := `You are an expert at creating clear, structured outlines. Create a hierarchical outline from the provided content.

Rules:
1. Use clear, descriptive headings
2. Create 3-5 main sections maximum  
3. Include 2-4 subsections under each main section where relevant
4. Focus on key concepts and important details
5. Be concise but comprehensive`

	if useMarkdown {
		systemPrompt += `

Format as clean markdown:
- Use ## for main sections
- Use ### for subsections  
- Use - for bullet points
- Use **bold** for emphasis`
	}

	userPrompt := fmt.Sprintf("Create a structured outline from this content:\n\n%s", summary)

	var spinnerStop chan struct{}
	if useMarkdown {
		spinnerStop = StartSpinner("Generating outline")
	}

	outline, err := callOllama(config, systemPrompt, userPrompt)
	if spinnerStop != nil {
		close(spinnerStop)
	}

	if err != nil {
		return "", err
	}

	// Cache the result
	cacheManager.Set(cacheKey, outline)
	return outline, nil
}
