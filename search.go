package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// SearchResult represents a web search result
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Source  string `json:"source"`
}

// SearchEngine interface for different search implementations
type SearchEngine interface {
	Search(query string, limit int) ([]SearchResult, error)
	Name() string
}

// DuckDuckGoEngine implements search using DDG HTML interface for reliability
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

// Search scrapes the DuckDuckGo HTML results page.
func (d *DuckDuckGoEngine) Search(query string, limit int) ([]SearchResult, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("DuckDuckGo search failed with status %d: %s", resp.StatusCode, string(body))
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	doc.Find(".result").Each(func(i int, s *goquery.Selection) {
		if len(results) >= limit {
			return
		}

		title := strings.TrimSpace(s.Find(".result__title").Text())
		snippet := strings.TrimSpace(s.Find(".result__snippet").Text())
		link, _ := s.Find(".result__url").Attr("href")

		if title != "" && snippet != "" && link != "" {
			// Clean up DDG's redirected URLs
			if unescapedLink, err := url.QueryUnescape(link); err == nil {
				if strings.Contains(unescapedLink, "uddg=") {
					if parsedURL, err := url.Parse(unescapedLink); err == nil {
						link = parsedURL.Query().Get("uddg")
					}
				}
			}

			results = append(results, SearchResult{
				Title:   title,
				URL:     link,
				Snippet: snippet,
				Source:  d.Name(),
			})
		}
	})

	if len(results) == 0 {
		return nil, fmt.Errorf("no results found on DuckDuckGo for '%s'", query)
	}

	return results, nil
}

// SearchManager manages the search engine with caching and optimization
type SearchManager struct {
	engine SearchEngine
	config *Config
	cache  *CacheManager
}

func NewSearchManager(config *Config) *SearchManager {
	sm := &SearchManager{
		config: config,
		cache:  NewCacheManager(config),
		engine: NewDuckDuckGoEngine(),
	}

	DebugLog(config, "Search manager initialized with %s engine", sm.engine.Name())
	return sm
}

// Search performs a cached search.
func (sm *SearchManager) Search(query string, limit int, sessionID string) ([]SearchResult, error) {
	cacheKey := sm.cache.GetCacheKey(fmt.Sprintf("search:%s:%d", query, limit))
	var cachedResults []SearchResult
	if sm.cache.Get(cacheKey, &cachedResults) {
		DebugLog(sm.config, "Cache hit for search: %s", query)
		return cachedResults, nil
	}

	DebugLog(sm.config, "Cache miss, performing search: %s", query)

	results, err := sm.engine.Search(query, limit)
	if err != nil {
		DebugLog(sm.config, "%s search failed: %v", sm.engine.Name(), err)
		return nil, err
	}

	if len(results) > 0 {
		sm.cache.Set(cacheKey, results, sessionID)
		DebugLog(sm.config, "%s search successful: %d results", sm.engine.Name(), len(results))
	}

	return results, nil
}

// PerformParallelSearches performs multiple searches with improved efficiency
func (sm *SearchManager) PerformParallelSearches(queries []string, limitPerQuery int, sessionID string) []SearchResult {
	DebugLog(sm.config, "Starting parallel searches for %d queries", len(queries))

	var wg sync.WaitGroup
	var mu sync.Mutex
	var allResults []SearchResult

	semaphore := make(chan struct{}, 4) // Limit concurrency

	for _, query := range queries {
		wg.Add(1)
		go func(q string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			results, err := sm.Search(q, limitPerQuery, sessionID)
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

	uniqueResults := sm.deduplicateResults(allResults)
	if len(uniqueResults) > sm.config.MaxSearchResults {
		uniqueResults = uniqueResults[:sm.config.MaxSearchResults]
	}

	DebugLog(sm.config, "Parallel searches completed: %d unique results", len(uniqueResults))
	return uniqueResults
}

// deduplicateResults removes duplicate search results based on URL and content similarity
func (sm *SearchManager) deduplicateResults(results []SearchResult) []SearchResult {
	seen := make(map[string]bool)
	var unique []SearchResult

	for _, result := range results {
		key := result.URL // Simple dedupe by URL is sufficient for now
		if !seen[key] {
			seen[key] = true
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
	builder.WriteString("\n\n--- ADDITIONAL CONTEXT FROM WEB SEARCH ---\n")

	for i, result := range results {
		builder.WriteString(fmt.Sprintf("\n[%d] %s\nSnippet: %s\nSource: %s <%s>\n",
			i+1, result.Title, result.Snippet, result.Source, result.URL))
	}

	return builder.String()
}
