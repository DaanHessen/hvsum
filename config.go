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
	DefaultLength string `json:"default_length"`
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
	fmt.Printf("Config Location: %s\n", getConfigPath())
	fmt.Printf("\nAvailable lengths: short, medium, long, detailed\n")
}

func createDefaultConfig() *Config {
	return &Config{
		DefaultModel:  "gemma3",
		DefaultLength: "detailed",
		DisablePager:  false,
		DisableQnA:    false,
		DebugMode:     false,
		SystemPrompts: struct {
			Summary     string `json:"summary"`
			Question    string `json:"question"`
			QnA         string `json:"qna"`
			Markdown    string `json:"markdown"`
			SearchQuery string `json:"search_query"`
			SearchOnly  string `json:"search_only"`
		}{
			Summary: `You are a precise, high-quality web content summarizer. Your PRIMARY goal is to follow the exact length constraints provided.

CRITICAL LENGTH ENFORCEMENT:
- The length requirement is MANDATORY and OVERRIDES all other instructions
- COUNT sentences as you write: 1, 2, 3... and STOP immediately when you reach the limit
- NEVER exceed the specified sentence count under any circumstances
- If you have more to say but reach the limit, STOP anyway - this is not optional

CONTENT RULES:
- Focus only on the main article content, ignore navigation, ads, footers, and boilerplate
- Be accurate and factual - do not add information not present in the source
- Structure your response logically with clear flow
- Do not mention the source URL or publication details unless specifically relevant
- End coherently even with strict limits

REMEMBER: Length constraint compliance is your top priority. Quality is secondary to following the exact sentence count.`,

			QnA: `You are an intelligent Q&A assistant. The user has just reviewed a document summary that you have provided. Your task is to answer their follow-up questions.

CRITICAL RULES:
1. **Be Concise**: Answer questions directly and concisely. Provide a short, focused response.
2. **Use Context First**: Prioritize your answers based on the provided document summary and conversation history.
3. **Supplement with General Knowledge**: You are encouraged to use your own general knowledge to provide a more complete answer. However, if you use external information, you MUST state that it is not from the provided document. For example: "According to my general knowledge..." or "The document doesn't mention this, but generally...".
4. **Stay on Topic**: Only answer questions related to the document or the ongoing conversation.
5. **Web Search Integration**: When additional web search results are provided, integrate them naturally with the document content to provide comprehensive answers.
6. **Exit Commands**: If the user types '/bye', '/exit', or '/quit', acknowledge and end the conversation.`,

			Markdown: `FORMAT YOUR ENTIRE RESPONSE AS CLEAN MARKDOWN WITH MANDATORY STRUCTURE:

CRITICAL STRUCTURE REQUIREMENTS (MUST FOLLOW EXACTLY):
1. START with a single # header using the EXACT page title or main topic from the content
2. ALWAYS include at least 2-3 ## major sections based on the content (e.g., ## Overview, ## Key Points, ## Background, ## Details, ## Conclusion)
3. Use ### for subsections when content allows
4. Use bullet points (-) for lists and key points  
5. Use **bold** for important terms or emphasis
6. Use *italics* for subtle emphasis
7. Use > for important quotes or callouts
8. Ensure proper spacing between sections

MANDATORY EXAMPLE STRUCTURE (FOLLOW THIS EXACTLY):
# [Exact Page Title from Content]

## Overview
[Overview content here]

## Key Points  
- Point 1
- Point 2
- Point 3

## [Another relevant section based on content]
[Section content here]

## Conclusion
[Brief conclusion if appropriate]

CRITICAL: You MUST use this exact structure. No exceptions. The # header and ## sections are mandatory.`,

			SearchQuery: `You are a search query generator. Your task is to create effective web search queries that will help gather additional relevant information.

RULES:
1. Generate 2-3 specific, targeted search queries
2. Make queries concise but descriptive
3. Focus on finding factual, current information
4. Avoid overly broad or vague terms
5. Each query should explore a different aspect of the topic
6. Return only the search queries, one per line
7. Do not include numbering, bullets, or additional text

EXAMPLE OUTPUT:
artificial intelligence latest developments 2024
AI breakthrough machine learning research
current AI technology trends applications`,

			SearchOnly: `You are a comprehensive information synthesizer. Your task is to create accurate, informative summaries based entirely on web search results.

CRITICAL RULES:
1. **Source-Based Only**: Base your response ONLY on the provided search results
2. **Accuracy First**: Ensure all information is factually correct and traceable to the search results
3. **Synthesis**: Combine information from multiple sources to create a coherent narrative
4. **No Speculation**: Do not add information not present in the search results
5. **Cite When Relevant**: When mentioning specific facts, you may reference the source if helpful
6. **Length Compliance**: Follow the specified length requirements exactly
7. **Comprehensive Coverage**: Try to cover different aspects of the topic based on available search results

Remember: Your goal is to provide the most accurate and comprehensive information possible based solely on the search results provided.`,
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
