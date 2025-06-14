package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all user-configurable settings
type Config struct {
	DefaultModel  string `json:"default_model"`
	DisablePager  bool   `json:"disable_pager"`
	DisableQnA    bool   `json:"disable_qna"`
	DebugMode     bool   `json:"debug_mode"`
	SystemPrompts struct {
		Summary     string `json:"summary"`
		Question    string `json:"question"`
		QnA         string `json:"qna"`
		Markdown    string `json:"markdown"`
		SearchQuery string `json:"search_query"`
		SearchOnly  string `json:"search_only"`
	} `json:"system_prompts"`
	DefaultLength    string `json:"default_length"`
	SessionPersist   bool   `json:"session_persist"`
	MaxSearchResults int    `json:"max_search_results"`
	CacheEnabled     bool   `json:"cache_enabled"`
	CacheTTL         int    `json:"cache_ttl_hours"`
}

// LoadConfig loads or creates the configuration file
func LoadConfig() (*Config, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	configPath := filepath.Join(configDir, appName, "config.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Creating default configuration at: %s\n", configPath)
		defaultConfig := createDefaultConfig()
		if err := saveConfig(configPath, defaultConfig); err != nil {
			return nil, fmt.Errorf("could not create default config: %w", err)
		}
		return defaultConfig, nil
	}

	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("config file is corrupted: %w", err)
	}

	return &config, nil
}

// Print displays the current configuration
func (c *Config) Print() {
	fmt.Printf("Current Configuration:\n")
	fmt.Printf("Model: %s\n", c.DefaultModel)
	fmt.Printf("Default Length: %s\n", c.DefaultLength)
	fmt.Printf("Disable Pager: %t\n", c.DisablePager)
	fmt.Printf("Disable Q&A: %t\n", c.DisableQnA)
	fmt.Printf("Debug Mode: %t\n", c.DebugMode)
	fmt.Printf("Session Persist: %t\n", c.SessionPersist)
	fmt.Printf("Max Search Results: %d\n", c.MaxSearchResults)
	fmt.Printf("Cache Enabled: %t\n", c.CacheEnabled)
	fmt.Printf("Cache TTL: %d hours\n", c.CacheTTL)
	fmt.Printf("Config Location: %s\n", getConfigPath())
	fmt.Printf("\nAvailable lengths: short, medium, long, detailed\n")
}

func createDefaultConfig() *Config {
	return &Config{
		DefaultModel:     "gemma3",
		DefaultLength:    "medium",
		DisablePager:     false,
		DisableQnA:       false,
		DebugMode:        false,
		SessionPersist:   true,
		MaxSearchResults: 8,
		CacheEnabled:     true,
		CacheTTL:         24,
		SystemPrompts: struct {
			Summary     string `json:"summary"`
			Question    string `json:"question"`
			QnA         string `json:"qna"`
			Markdown    string `json:"markdown"`
			SearchQuery string `json:"search_query"`
			SearchOnly  string `json:"search_only"`
		}{
			Summary: `You are an expert content summarizer. Create clear, concise summaries that capture essential information.

CORE RULES:
1. Follow length limits exactly: short (3-5 sentences), medium (6-10 sentences), long (15-20 sentences), detailed (as needed)
2. Output ONLY the summary - no meta text like "Here's a summary"
3. Focus on key facts, insights, and actionable information
4. Ignore ads, navigation, and boilerplate content
5. Use clear, engaging language that's easy to scan

FORMAT: Structure as coherent paragraphs. For markdown mode, use proper headings and formatting.`,

			QnA: `You are a helpful Q&A assistant discussing a document summary. Answer questions directly and concisely.

RULES:
1. Keep answers to 2-3 sentences maximum unless complex
2. Use document context first, then general knowledge if needed
3. If using external knowledge, note: "From general knowledge:"
4. Stay relevant to the topic
5. For web search results, integrate them naturally with document content`,

			Markdown: `FORMAT YOUR RESPONSE AS CLEAN MARKDOWN:

STRUCTURE:
# [Main Title/Topic]

## Key Points  
- Point 1 with **important** details
- Point 2 with context
- Point 3 with implications

## [Relevant Section]
Content organized logically

Use **bold** for emphasis, *italics* for subtle emphasis, and > for important quotes.`,

			SearchQuery: `Generate 2-3 focused search queries based on the context. Each query should explore different aspects.

Return ONLY the queries, one per line:`,

			SearchOnly: `Create a comprehensive summary based on web search results. Synthesize information from multiple sources into a coherent response.

RULES:
1. Base content ONLY on provided search results
2. Combine information intelligently across sources
3. Follow specified length requirements
4. Be factual and accurate
5. Do not speculate beyond the search results`,
		},
	}
}

func saveConfig(path string, config *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(config)
}

func getConfigPath() string {
	configDir, _ := os.UserConfigDir()
	return filepath.Join(configDir, appName, "config.json")
}
