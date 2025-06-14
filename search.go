package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// SearchResult represents a web search result
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// SearchEngine interface for different search implementations
type SearchEngine interface {
	Search(query string, limit int) ([]SearchResult, error)
	Name() string
}

// DuckDuckGoEngine implements search using DuckDuckGo API
type DuckDuckGoEngine struct {
	client *http.Client
}

func NewDuckDuckGoEngine() *DuckDuckGoEngine {
	return &DuckDuckGoEngine{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (d *DuckDuckGoEngine) Name() string {
	return "DuckDuckGo"
}

func (d *DuckDuckGoEngine) Search(query string, limit int) ([]SearchResult, error) {
	apiURL := "https://api.duckduckgo.com/"

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("q", query)
	q.Add("format", "json")
	q.Add("no_html", "1")
	q.Add("skip_disambig", "1")
	req.URL.RawQuery = q.Encode()

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search API returned status %d", resp.StatusCode)
	}

	var result struct {
		Abstract      string `json:"Abstract"`
		AbstractURL   string `json:"AbstractURL"`
		Heading       string `json:"Heading"`
		Answer        string `json:"Answer"`
		Definition    string `json:"Definition"`
		RelatedTopics []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var results []SearchResult

	// Add instant answer if available
	if result.Answer != "" {
		results = append(results, SearchResult{
			Title:   fmt.Sprintf("Answer: %s", query),
			URL:     "https://duckduckgo.com/?q=" + url.QueryEscape(query),
			Snippet: result.Answer,
		})
	}

	// Add abstract/definition
	if result.Abstract != "" {
		title := result.Heading
		if title == "" {
			title = fmt.Sprintf("Information about: %s", query)
		}

		resultURL := result.AbstractURL
		if resultURL == "" {
			resultURL = "https://duckduckgo.com/?q=" + url.QueryEscape(query)
		}

		results = append(results, SearchResult{
			Title:   title,
			URL:     resultURL,
			Snippet: result.Abstract,
		})
	}

	// Add definition
	if result.Definition != "" {
		results = append(results, SearchResult{
			Title:   fmt.Sprintf("Definition: %s", query),
			URL:     "https://duckduckgo.com/?q=" + url.QueryEscape(query),
			Snippet: result.Definition,
		})
	}

	// Add related topics
	for i, topic := range result.RelatedTopics {
		if i >= limit-len(results) {
			break
		}
		if topic.Text != "" && topic.FirstURL != "" {
			results = append(results, SearchResult{
				Title:   fmt.Sprintf("Related: %s", strings.Split(topic.Text, " - ")[0]),
				URL:     topic.FirstURL,
				Snippet: topic.Text,
			})
		}
	}

	return results, nil
}

// SerpAPIEngine implements search using SerpAPI (requires API key)
type SerpAPIEngine struct {
	apiKey string
	client *http.Client
}

func NewSerpAPIEngine(apiKey string) *SerpAPIEngine {
	return &SerpAPIEngine{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (s *SerpAPIEngine) Name() string {
	return "SerpAPI"
}

func (s *SerpAPIEngine) Search(query string, limit int) ([]SearchResult, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("SerpAPI key not configured")
	}

	apiURL := "https://serpapi.com/search"

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("q", query)
	q.Add("api_key", s.apiKey)
	q.Add("engine", "google")
	q.Add("num", fmt.Sprintf("%d", limit))
	req.URL.RawQuery = q.Encode()

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SerpAPI returned status %d", resp.StatusCode)
	}

	var result struct {
		OrganicResults []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic_results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, item := range result.OrganicResults {
		results = append(results, SearchResult{
			Title:   item.Title,
			URL:     item.Link,
			Snippet: item.Snippet,
		})
	}

	return results, nil
}

// SearchManager manages multiple search engines and performs optimized searches
type SearchManager struct {
	engines []SearchEngine
	config  *Config
}

func NewSearchManager(config *Config) *SearchManager {
	sm := &SearchManager{
		config: config,
	}

	// Add DuckDuckGo engine (always available)
	sm.engines = append(sm.engines, NewDuckDuckGoEngine())

	// Add SerpAPI engine if API key is available
	if serpAPIKey := os.Getenv("SERPAPI_KEY"); serpAPIKey != "" {
		sm.engines = append(sm.engines, NewSerpAPIEngine(serpAPIKey))
		DebugLog(config, "SerpAPI engine enabled")
	}

	DebugLog(config, "Search manager initialized with %d engines", len(sm.engines))
	return sm
}

// Search performs optimized search using available engines
func (sm *SearchManager) Search(query string, limit int) ([]SearchResult, error) {
	DebugLog(sm.config, "Starting search for: %s (limit: %d)", query, limit)

	var allResults []SearchResult
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Try all engines in parallel for maximum speed
	for _, engine := range sm.engines {
		wg.Add(1)
		go func(eng SearchEngine) {
			defer wg.Done()

			DebugLog(sm.config, "Searching with %s engine", eng.Name())
			results, err := eng.Search(query, limit)
			if err != nil {
				DebugLog(sm.config, "%s search failed: %v", eng.Name(), err)
				return
			}

			mu.Lock()
			allResults = append(allResults, results...)
			DebugLog(sm.config, "%s returned %d results", eng.Name(), len(results))
			mu.Unlock()
		}(engine)
	}

	wg.Wait()

	// Deduplicate and limit results
	uniqueResults := deduplicateResults(allResults)
	if len(uniqueResults) > limit {
		uniqueResults = uniqueResults[:limit]
	}

	DebugLog(sm.config, "Search completed: %d unique results", len(uniqueResults))
	return uniqueResults, nil
}

// PerformParallelSearches performs multiple searches in parallel
func (sm *SearchManager) PerformParallelSearches(queries []string, limitPerQuery int) []SearchResult {
	DebugLog(sm.config, "Starting parallel searches for %d queries", len(queries))

	var wg sync.WaitGroup
	var mu sync.Mutex
	var allResults []SearchResult

	// Limit concurrent searches to avoid overwhelming servers
	semaphore := make(chan struct{}, 3)

	for _, query := range queries {
		wg.Add(1)
		go func(q string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			results, err := sm.Search(q, limitPerQuery)
			if err != nil {
				DebugLog(sm.config, "Parallel search failed for '%s': %v", q, err)
				return
			}

			mu.Lock()
			allResults = append(allResults, results...)
			mu.Unlock()
		}(query)
	}

	wg.Wait()

	// Deduplicate final results
	uniqueResults := deduplicateResults(allResults)
	DebugLog(sm.config, "Parallel searches completed: %d total unique results", len(uniqueResults))
	return uniqueResults
}

// deduplicateResults removes duplicate search results based on URL
func deduplicateResults(results []SearchResult) []SearchResult {
	seen := make(map[string]bool)
	var unique []SearchResult

	for _, result := range results {
		if !seen[result.URL] {
			seen[result.URL] = true
			unique = append(unique, result)
		}
	}

	return unique
}

// FormatSearchResults formats search results for inclusion in prompts
func FormatSearchResults(results []SearchResult) string {
	if len(results) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("\n--- WEB SEARCH RESULTS ---\n")

	for i, result := range results {
		builder.WriteString(fmt.Sprintf("\nResult %d:\nTitle: %s\nURL: %s\nSnippet: %s\n",
			i+1, result.Title, result.URL, result.Snippet))
	}

	return builder.String()
}
