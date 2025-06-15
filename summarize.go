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

	// Enhanced workflow: DeepSeek for detailed summary, Ollama for length reduction
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

// generateTwoStageSummary implements the enhanced workflow with proper API usage
func generateTwoStageSummary(config *Config, length string, useMarkdown, enableSearch bool, content, title, sourceURL string, sessionID string) (string, error) {
	DebugLog(config, "Starting enhanced summarization workflow")

	// Stage 1: Generate detailed summary using DeepSeek (no length constraints)
	detailedSummary, err := generateDetailedSummary(config, useMarkdown, enableSearch, content, title, sourceURL, sessionID)
	if err != nil {
		return "", fmt.Errorf("DeepSeek detailed summary failed: %v", err)
	}

	// Stage 2: Apply length constraint using Ollama if needed (preserve detailed for markdown)
	if length == "detailed" {
		return detailedSummary, nil
	}

	finalSummary, err := applyLengthConstraint(config, useMarkdown, detailedSummary, length, sessionID)
	if err != nil {
		return "", fmt.Errorf("Ollama length reduction failed: %v", err)
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

	// Build detailed summary prompt with explicit source verification instructions
	userPrompt := buildDetailedPromptWithVerification(content, title, sourceURL, searchResults)

	// Always use DeepSeek for detailed summaries with no length constraints
	summary, err := CallDeepSeekOrFallback(config, systemPrompt, userPrompt, useMarkdown)

	if err != nil {
		return "", err
	}

	// DeepSeek already performs fact verification in its thinking process
	// No additional verification needed to avoid double API calls
	return summary, nil
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

	fmt.Fprintf(os.Stderr, "📝 Applying length constraint (%s) with Ollama...\n", targetLength)

	// Always use Ollama for length reduction to save DeepSeek API costs
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

// generateDetailedSearchSummary creates a comprehensive summary from search results with verification
func generateDetailedSearchSummary(config *Config, useMarkdown bool, query string, searchResults []SearchResult, sessionID string) (string, error) {
	systemPrompt := config.SystemPrompts.SearchOnly
	if useMarkdown {
		systemPrompt += "\n\n" + config.SystemPrompts.Markdown
	}

	userPrompt := fmt.Sprintf(`Based on the search results below, create a comprehensive summary for the query: "%s"

SEARCH RESULTS:
%s

Create a detailed summary that synthesizes information from these search results. Remember to:
1. Only use information explicitly found in the search results
2. Attribute information to sources when possible
3. Note any conflicting information between sources
4. State clearly if information is insufficient for any aspect of the query`, query, FormatSearchResults(searchResults))

	// Use DeepSeek for detailed search summaries only
	summary, err := CallDeepSeekOrFallback(config, systemPrompt, userPrompt, useMarkdown)

	if err != nil {
		return "", err
	}

	// DeepSeek already performs fact verification in its thinking process
	// No additional verification needed to avoid double API calls
	return summary, nil
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

// buildDetailedPromptWithVerification creates a comprehensive prompt for detailed summarization with verification instructions
func buildDetailedPromptWithVerification(content, title, sourceURL string, searchResults []SearchResult) string {
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

	prompt += "\n\nPlease verify the accuracy of this summary against the source content before accepting it."

	return prompt
}

// generateSearchQueries uses AI to generate relevant search queries with caching
func generateSearchQueries(config *Config, contextText, purpose string, sessionID string) ([]string, error) {
	cacheManager := NewCacheManager(config)
	cacheKey := cacheManager.GetCacheKey(fmt.Sprintf("queries:%s:%s", purpose, contextText[:Min(200, len(contextText))]))

	var cachedQueries []string
	if cacheManager.Get(cacheKey, &cachedQueries) {
		DebugLog(config, "Cache hit for search queries")
		return cachedQueries, nil
	}

	systemPrompt := `You are an expert at generating search queries. Your task is to analyze a user's question and the surrounding context to create highly specific, targeted search queries that will find the precise missing piece of information.

**RULES:**
1.  **Analyze the User's Goal**: Look at the most recent question and the conversation history. What is the user *really* asking for? Are they confused? Do they need a specific detail, a definition, or a comparison?
2.  **Isolate the Ambiguity**: Identify the core uncertainty in the user's question. For example, if the user asks "Was it voluntary or influenced?", the key concepts are "voluntary" and "influence" in the context of the subject.
3.  **Create Specific Queries**: Generate 2-3 queries that are laser-focused on resolving that ambiguity. Avoid generic queries.
    *   **Bad Generic Query**: ` + "`Eva Braun death`" + `
    *   **Good Specific Query**: ` + "`Eva Braun suicide voluntary or coerced by Hitler`" + `
    *   **Good Specific Query**: ` + "`evidence of Hitler influencing Eva Braun's suicide`" + `
4.  **Format**: Return ONLY the queries, one per line. Do not add any other text, numbers, or bullet points.
`

	// The 'purpose' field now contains the user's most recent question.
	// The 'contextText' contains the conversation history and document summary.
	userPrompt := fmt.Sprintf(`
**Conversation Context & Document Summary:**
%s

**User's Most Recent Question:**
%s

Based on the rules, generate specific search queries to answer the user's most recent question.`, contextText, purpose)

	DebugLog(config, "Generating search queries for: %s", purpose)

	client, err := api.ClientFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("could not connect to Ollama: %v", err)
	}

	var responseBuilder strings.Builder
	isStreaming := false // Not streaming for query generation
	req := &api.ChatRequest{
		Model: config.DefaultModel,
		Messages: []api.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream:  &isStreaming,
		Options: map[string]interface{}{"temperature": 0.2},
	}

	err = client.Chat(context.Background(), req, func(res api.ChatResponse) error {
		responseBuilder.WriteString(res.Message.Content)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("query generation failed: %v", err)
	}

	queries := strings.Split(strings.TrimSpace(responseBuilder.String()), "\n")
	var cleanedQueries []string
	for _, q := range queries {
		// Remove any markdown list characters or extra whitespace
		q = strings.TrimLeft(q, "-* ")
		if q != "" {
			cleanedQueries = append(cleanedQueries, q)
		}
	}

	if len(cleanedQueries) > 0 {
		cacheManager.Set(cacheKey, cleanedQueries, sessionID)
		DebugLog(config, "Generated %d search queries", len(cleanedQueries))
	}

	return cleanedQueries, nil
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

	// Try DeepSeek first for outline generation if available
	if ShouldUseDeepSeek(config) {
		client := NewDeepSeekClient(config)
		if client != nil {
			outline, err := client.GenerateOutlineWithDeepSeek(summary, config, useMarkdown)
			if err == nil {
				if spinnerStop != nil {
					close(spinnerStop)
				}
				cacheManager.Set(cacheKey, outline, sessionID)
				return outline, nil
			}
			fmt.Fprintf(os.Stderr, "⚠️ DeepSeek outline generation failed, falling back to local model: %v\n", err)
		}
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

// performFactVerification performs fact verification on the summary
func performFactVerification(config *Config, summary, content string, searchResults []SearchResult, sessionID string) (string, error) {
	// Create fact verification prompt
	verificationPrompt := `You are a strict fact-checker. Your task is to verify if the summary contains ONLY information that can be found in the provided source content.

VERIFICATION PROTOCOL:
1. Check every factual claim in the summary against the source content
2. Flag any information that cannot be directly found in the source
3. Look for invented details, assumptions, or extrapolations not in the source
4. Check for dramatic language or storytelling elements not present in source
5. Verify all dates, numbers, names, and specific details

If you find any hallucinations or invented content, provide a corrected version that removes only verified information from the source.

RESPONSE FORMAT:
If the summary is accurate: "VERIFIED: [original summary]"
If corrections needed: "CORRECTED: [corrected summary with only verified information]"

SOURCE CONTENT:
` + content

	if len(searchResults) > 0 {
		verificationPrompt += "\n\nADDITIONAL SEARCH RESULTS:\n" + FormatSearchResults(searchResults)
	}

	verificationPrompt += "\n\nSUMMARY TO VERIFY:\n" + summary

	// Call AI for verification
	verificationSystem := `You are an expert fact-checker with strict protocols. Verify the summary contains ONLY information explicitly present in the source material. Do not allow any invented details, assumptions, or extrapolations.`

	result, err := CallDeepSeekOrFallback(config, verificationSystem, verificationPrompt, false)
	if err != nil {
		return "", err
	}

	// Parse verification result
	if strings.HasPrefix(result, "VERIFIED:") {
		return strings.TrimSpace(strings.TrimPrefix(result, "VERIFIED:")), nil
	} else if strings.HasPrefix(result, "CORRECTED:") {
		correctedSummary := strings.TrimSpace(strings.TrimPrefix(result, "CORRECTED:"))
		DebugLog(config, "Summary corrected for accuracy by fact verification")
		return correctedSummary, nil
	}

	// If verification format is unexpected, return original with warning
	DebugLog(config, "Unexpected verification response format")
	return summary, nil
}

// performSearchFactVerification performs fact verification on search-based summaries
func performSearchFactVerification(config *Config, summary string, searchResults []SearchResult, query string, sessionID string) (string, error) {
	// Create search fact verification prompt
	verificationPrompt := `You are a strict fact-checker. Your task is to verify if the search-based summary contains ONLY information that can be found in the provided search results.

VERIFICATION PROTOCOL:
1. Check every factual claim in the summary against the search results
2. Flag any information that cannot be directly found in the search results
3. Look for invented details, assumptions, or extrapolations not in the search results
4. Check for dramatic language or storytelling elements not present in search results
5. Verify all dates, numbers, names, and specific details

If you find any hallucinations or invented content, provide a corrected version that removes only verified information from the search results.

RESPONSE FORMAT:
If the summary is accurate: "VERIFIED: [original summary]"
If corrections needed: "CORRECTED: [corrected summary with only verified information]"

SEARCH RESULTS:
` + FormatSearchResults(searchResults)

	verificationPrompt += "\n\nSUMMARY TO VERIFY:\n" + summary

	// Call AI for verification
	verificationSystem := `You are an expert fact-checker with strict protocols. Verify the summary contains ONLY information explicitly present in the search results. Do not allow any invented details, assumptions, or extrapolations.`

	result, err := CallDeepSeekOrFallback(config, verificationSystem, verificationPrompt, false)
	if err != nil {
		return "", err
	}

	// Parse verification result
	if strings.HasPrefix(result, "VERIFIED:") {
		return strings.TrimSpace(strings.TrimPrefix(result, "VERIFIED:")), nil
	} else if strings.HasPrefix(result, "CORRECTED:") {
		correctedSummary := strings.TrimSpace(strings.TrimPrefix(result, "CORRECTED:"))
		DebugLog(config, "Summary corrected for accuracy by search fact verification")
		return correctedSummary, nil
	}

	// If verification format is unexpected, return original with warning
	DebugLog(config, "Unexpected verification response format")
	return summary, nil
}
