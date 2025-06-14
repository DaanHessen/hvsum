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

// ProcessURL handles URL-based summarization with the new two-stage approach
func ProcessURL(urlStr string, config *Config, length string, useMarkdown, enableSearch bool, sessionID string) (string, string, string, error) {
	fmt.Fprintf(os.Stderr, "🌐 Fetching content from: %s\n", urlStr)

	// Initialize cache manager
	cacheManager := NewCacheManager(config)

	// Check cache first for final result
	cacheKey := cacheManager.GetCacheKey(fmt.Sprintf("url:%s:%s:%t:%t", urlStr, length, useMarkdown, enableSearch))
	var cachedSummary string
	if cacheManager.Get(cacheKey, &cachedSummary) {
		DebugLog(config, "Cache hit for URL summary")
		return cachedSummary, cachedSummary, "Cached Summary", nil
	}

	// Extract content from URL
	content, title, err := ExtractWebContent(urlStr)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to extract content: %v", err)
	}

	DebugLog(config, "Extracted %d characters from URL", len(content))
	DebugLog(config, "Page title: %s", title)

	// Two-stage summarization process
	finalSummary, err := generateTwoStageSummary(config, length, useMarkdown, enableSearch, content, title, urlStr, sessionID)
	if err != nil {
		return "", "", "", err
	}

	// Cache the final result
	cacheManager.Set(cacheKey, finalSummary, sessionID)
	return finalSummary, content, title, nil
}

// ProcessSearchQuery handles search-only summarization with two-stage approach
func ProcessSearchQuery(query string, config *Config, length string, useMarkdown bool, sessionID string) (string, string, string, error) {
	fmt.Fprintf(os.Stderr, "🔍 Performing web search for: %s\n", query)

	// Initialize cache manager
	cacheManager := NewCacheManager(config)

	// Check cache first
	cacheKey := cacheManager.GetCacheKey(fmt.Sprintf("search:%s:%s:%t", query, length, useMarkdown))
	var cachedSummary string
	if cacheManager.Get(cacheKey, &cachedSummary) {
		DebugLog(config, "Cache hit for search summary")
		return cachedSummary, cachedSummary, query, nil
	}

	// Create search manager and perform searches
	searchManager := NewSearchManager(config)

	// Generate fewer related search queries for better performance
	relatedQueries, err := generateSearchQueries(config, query, "provide comprehensive information about this topic", sessionID)
	if err != nil {
		DebugLog(config, "Failed to generate related queries: %v", err)
		relatedQueries = []string{}
	}

	// Always include the original query and limit total queries to 3 for performance
	allQueries := []string{query}
	for _, rq := range relatedQueries {
		if len(allQueries) >= 3 {
			break
		}
		allQueries = append(allQueries, rq)
	}
	DebugLog(config, "Using %d total queries for search", len(allQueries))

	// Perform parallel searches with fewer results per query
	fmt.Fprintf(os.Stderr, "🚀 Performing parallel web searches...\n")
	searchResults := searchManager.PerformParallelSearches(allQueries, 2, sessionID)

	if len(searchResults) == 0 {
		return "", "", "", fmt.Errorf("no search results found for query: %s", query)
	}

	DebugLog(config, "Found %d total search results", len(searchResults))

	// Generate summary from search results using two-stage approach
	finalSummary, err := generateSearchOnlySummaryTwoStage(config, length, useMarkdown, query, searchResults, sessionID)
	if err != nil {
		return "", "", "", err
	}

	// Cache the result
	cacheManager.Set(cacheKey, finalSummary, sessionID)
	return finalSummary, finalSummary, query, nil
}

// generateTwoStageSummary implements the two-stage summarization process
func generateTwoStageSummary(config *Config, length string, useMarkdown, enableSearch bool, content, title, sourceURL string, sessionID string) (string, error) {
	DebugLog(config, "Starting two-stage summarization process")

	// Stage 1: Generate detailed summary with all content
	detailedSummary, err := generateDetailedSummary(config, useMarkdown, enableSearch, content, title, sourceURL, sessionID)
	if err != nil {
		return "", fmt.Errorf("stage 1 failed: %v", err)
	}

	// Stage 2: Apply length constraint if not already detailed
	if length == "detailed" {
		return detailedSummary, nil
	}

	finalSummary, err := applyLengthConstraint(config, useMarkdown, detailedSummary, length, sessionID)
	if err != nil {
		return "", fmt.Errorf("stage 2 failed: %v", err)
	}

	return finalSummary, nil
}

// generateDetailedSummary creates a comprehensive summary with all available information
func generateDetailedSummary(config *Config, useMarkdown, enableSearch bool, content, title, sourceURL string, sessionID string) (string, error) {
	systemPrompt := config.SystemPrompts.Summary
	if useMarkdown {
		systemPrompt += "\n\n" + config.SystemPrompts.Markdown
	}

	var searchResults []SearchResult
	if enableSearch {
		fmt.Fprintf(os.Stderr, "🔍 Enhancing summary with web search...\n")
		searchManager := NewSearchManager(config)
		queries, err := generateSearchQueries(config, content[:Min(1000, len(content))], "enhance this content summary", sessionID)
		if err != nil {
			DebugLog(config, "Search query generation failed: %v", err)
		} else {
			fmt.Fprintf(os.Stderr, "🚀 Performing parallel searches...\n")
			searchResults = searchManager.PerformParallelSearches(queries, 2, sessionID)
			DebugLog(config, "Enhanced with %d search results", len(searchResults))
		}
	}

	// Build detailed summary prompt
	userPrompt := buildDetailedPrompt(content, title, sourceURL, searchResults)

	fmt.Fprintf(os.Stderr, "🤖 Generating comprehensive summary with %s...\n", config.DefaultModel)

	var spinnerStop chan struct{}
	if useMarkdown {
		spinnerStop = StartSpinner("Generating detailed summary")
	}

	summary, err := callOllama(config, systemPrompt, userPrompt)
	if spinnerStop != nil {
		close(spinnerStop)
	}

	return summary, err
}

// applyLengthConstraint reduces a detailed summary to the requested length
func applyLengthConstraint(config *Config, useMarkdown bool, detailedSummary, targetLength string, sessionID string) (string, error) {
	lengthInstruction, exists := lengthMap[targetLength]
	if !exists {
		lengthInstruction = lengthMap["medium"]
	}

	systemPrompt := fmt.Sprintf(`You are an expert content editor. Your task is to reduce a detailed summary to a specific length while preserving the most important information.

CRITICAL RULES:
1. Length requirement: %s
2. Preserve the most essential information
3. Maintain clarity and coherence
4. Remove redundant or less important details
5. Keep the same format and structure style

OUTPUT: Only the reduced summary, no meta-commentary.`, lengthInstruction)

	if useMarkdown {
		systemPrompt += "\n\nMaintain markdown formatting in your reduced summary."
	}

	userPrompt := fmt.Sprintf("Reduce this detailed summary to the specified length:\n\n%s", detailedSummary)

	// Use cache for length reductions
	cacheManager := NewCacheManager(config)
	cacheKey := cacheManager.GetCacheKey(fmt.Sprintf("reduce:%s:%s", detailedSummary[:Min(200, len(detailedSummary))], targetLength))
	var cachedReduction string
	if cacheManager.Get(cacheKey, &cachedReduction) {
		DebugLog(config, "Cache hit for length reduction")
		return cachedReduction, nil
	}

	fmt.Fprintf(os.Stderr, "📝 Applying length constraint (%s)...\n", targetLength)

	summary, err := callOllama(config, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	// Cache the reduction
	cacheManager.Set(cacheKey, summary, sessionID)
	return summary, nil
}

// generateSearchOnlySummaryTwoStage applies two-stage approach to search-only results
func generateSearchOnlySummaryTwoStage(config *Config, length string, useMarkdown bool, query string, searchResults []SearchResult, sessionID string) (string, error) {
	// Stage 1: Generate detailed summary from all search results
	detailedSummary, err := generateDetailedSearchSummary(config, useMarkdown, query, searchResults, sessionID)
	if err != nil {
		return "", fmt.Errorf("stage 1 failed: %v", err)
	}

	// Stage 2: Apply length constraint if needed
	if length == "detailed" {
		return detailedSummary, nil
	}

	finalSummary, err := applyLengthConstraint(config, useMarkdown, detailedSummary, length, sessionID)
	if err != nil {
		return "", fmt.Errorf("stage 2 failed: %v", err)
	}

	return finalSummary, nil
}

// generateDetailedSearchSummary creates comprehensive summary from search results
func generateDetailedSearchSummary(config *Config, useMarkdown bool, query string, searchResults []SearchResult, sessionID string) (string, error) {
	systemPrompt := config.SystemPrompts.SearchOnly
	if useMarkdown {
		systemPrompt += "\n\n" + config.SystemPrompts.Markdown
	}

	userPrompt := fmt.Sprintf(`Create a comprehensive summary about: %s

Based on the following search results, provide a detailed summary covering all relevant aspects found. Synthesize information from multiple sources and organize it logically.

%s

Create a thorough, well-structured summary that covers all important information from these search results.`, query, FormatSearchResults(searchResults))

	fmt.Fprintf(os.Stderr, "🤖 Generating comprehensive summary with %s...\n", config.DefaultModel)

	var spinnerStop chan struct{}
	if useMarkdown {
		spinnerStop = StartSpinner("Generating detailed summary")
	}

	summary, err := callOllama(config, systemPrompt, userPrompt)
	if spinnerStop != nil {
		close(spinnerStop)
	}

	return summary, err
}

// buildDetailedPrompt creates a comprehensive prompt for detailed summarization
func buildDetailedPrompt(content, title, sourceURL string, searchResults []SearchResult) string {
	prompt := fmt.Sprintf(`Create a comprehensive summary of the following content. Be thorough and cover all important aspects, key points, and relevant details.

Title: %s
Content:
%s`, title, content)

	if len(searchResults) > 0 {
		prompt += FormatSearchResults(searchResults)
		prompt += "\n\nUse both the webpage content and the search results to create a comprehensive summary."
	}

	if sourceURL != "" {
		prompt += fmt.Sprintf("\n\nSource URL: %s", sourceURL)
	}

	return prompt
}

// generateSearchQueries uses AI to generate relevant search queries with caching
func generateSearchQueries(config *Config, contextText, purpose string, sessionID string) ([]string, error) {
	DebugLog(config, "Generating search queries for: %.100s...", contextText)

	// Check cache first
	cacheManager := NewCacheManager(config)
	cacheKey := cacheManager.GetCacheKey(fmt.Sprintf("queries:%s:%s", contextText[:Min(200, len(contextText))], purpose))
	var cachedQueries []string
	if cacheManager.Get(cacheKey, &cachedQueries) {
		DebugLog(config, "Cache hit for search queries")
		return cachedQueries, nil
	}

	// Simplified prompt for faster processing
	prompt := fmt.Sprintf(`Generate 2 specific search queries based on this context:

%s

Purpose: %s

Return only 2 queries, one per line:`, contextText[:Min(500, len(contextText))], purpose)

	queries, err := callOllama(config, config.SystemPrompts.SearchQuery, prompt)
	if err != nil {
		return nil, err
	}

	// Parse queries from response
	lines := strings.Split(strings.TrimSpace(queries), "\n")
	var parsedQueries []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			// Remove common prefixes
			line = strings.TrimPrefix(line, "- ")
			line = strings.TrimPrefix(line, "* ")
			line = strings.TrimPrefix(line, "1. ")
			line = strings.TrimPrefix(line, "2. ")
			if len(line) > 3 && len(line) < 200 {
				parsedQueries = append(parsedQueries, line)
			}
		}
		// Limit to maximum 2 queries for performance
		if len(parsedQueries) >= 2 {
			break
		}
	}

	if len(parsedQueries) == 0 {
		return nil, fmt.Errorf("no valid queries generated")
	}

	// Cache the queries
	cacheManager.Set(cacheKey, parsedQueries, sessionID)
	DebugLog(config, "Generated %d search queries", len(parsedQueries))
	return parsedQueries, nil
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
func GenerateOutline(summary string, config *Config, useMarkdown bool, sessionID string) (string, error) {
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
	cacheManager.Set(cacheKey, outline, sessionID)
	return outline, nil
}
