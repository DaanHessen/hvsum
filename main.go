package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/go-shiori/go-readability"
	"github.com/ollama/ollama/api"
	"github.com/spf13/pflag"
)

const appName = "hvsum"

// Config holds all user-configurable settings
type Config struct {
	DefaultModel  string `json:"default_model"`
	SystemPrompts struct {
		Summary  string `json:"summary"`
		Question string `json:"question"`
		Markdown string `json:"markdown"`
	} `json:"system_prompts"`
	DefaultLength string `json:"default_length"` // short, medium, long, detailed
}

// Length definitions using research-backed techniques for precise length control
var lengthMap = map[string]string{
	"short":    "CRITICAL: Write EXACTLY 2 sentences. No more, no less. Count each sentence as you write: 1, 2, STOP. Focus only on the most essential information. After writing exactly 2 sentences, you MUST terminate your response immediately.",
	"medium":   "CRITICAL: Write EXACTLY 4-6 sentences total. Count each sentence: 1, 2, 3, 4, 5, 6, STOP. Provide key information in a balanced way. After reaching exactly 6 sentences maximum, you MUST terminate your response immediately.",
	"long":     "CRITICAL: Write EXACTLY 8-10 sentences total. Count carefully: 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, STOP. Provide comprehensive coverage with supporting details. After reaching exactly 10 sentences maximum, you MUST terminate your response immediately.",
	"detailed": "CRITICAL: Write EXACTLY 12-15 sentences total. Count each sentence meticulously: 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, STOP. Provide thorough analysis with examples and context. After reaching exactly 15 sentences maximum, you MUST terminate your response immediately.",
}

var (
	length     = pflag.StringP("length", "l", "", "Summary length: short, medium, long, detailed")
	markdown   = pflag.BoolP("markdown", "m", false, "Format output as structured markdown")
	showHelp   = pflag.BoolP("help", "h", false, "Show help message")
	showConfig = pflag.BoolP("config", "c", false, "Show current configuration")
)

func main() {
	pflag.Parse()

	if *showHelp {
		printUsage()
		return
	}

	config, err := loadOrInitConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error with configuration: %v\n", err)
		os.Exit(1)
	}

	if *showConfig {
		printConfig(config)
		return
	}

	// Handle cleanup
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "\nUnexpected error: %v\n", r)
		}
		fmt.Fprintf(os.Stderr, "\nStopping model '%s'...\n", config.DefaultModel)
		cmd := exec.Command("ollama", "stop", config.DefaultModel)
		cmd.Run()
	}()

	args := pflag.Args()

	// Determine the effective length setting
	effectiveLength := *length
	if effectiveLength == "" {
		effectiveLength = config.DefaultLength
	}
	if effectiveLength == "" {
		effectiveLength = "detailed" // fallback default
	}

	err = handleDirectMode(args, config, effectiveLength)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func handleDirectMode(args []string, config *Config, length string) error {
	var userMessage, link string

	switch len(args) {
	case 1:
		link = args[0]
	case 2:
		userMessage = args[0]
		link = args[1]
	default:
		return fmt.Errorf("incorrect number of arguments. Use: hvsum [question] <URL>")
	}

	fmt.Fprintf(os.Stderr, "Fetching content from: %s\n", link)
	textContent, pageTitle, err := fetchAndParseURL(link)
	if err != nil {
		return fmt.Errorf("error fetching page: %v", err)
	}

	// Determine which system prompt to use
	var systemPrompt string
	if userMessage != "" {
		systemPrompt = config.SystemPrompts.Question
	} else {
		systemPrompt = config.SystemPrompts.Summary
	}

	// Add markdown formatting instructions if needed
	if *markdown {
		systemPrompt += "\n\n" + config.SystemPrompts.Markdown
	}

	// Build the user prompt
	userPrompt := buildUserPrompt(userMessage, length, textContent, pageTitle)

	fmt.Fprintf(os.Stderr, "Generating response with %s...\n\n", config.DefaultModel)
	return generateOllamaResponse(config.DefaultModel, systemPrompt, userPrompt, *markdown)
}

func fetchAndParseURL(urlString string) (string, string, error) {
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse URL: %w", err)
	}

	resp, err := http.Get(urlString)
	if err != nil {
		return "", "", err
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

func generateOllamaResponse(model, system, prompt string, renderMarkdown bool) error {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama: %v", err)
	}

	stream := !renderMarkdown // Only stream when NOT using markdown
	req := &api.GenerateRequest{
		Model:  model,
		System: system,
		Prompt: prompt,
		Stream: &stream,
	}

	var responseBuilder strings.Builder

	err = client.Generate(context.Background(), req, func(resp api.GenerateResponse) error {
		responseBuilder.WriteString(resp.Response)

		// Only stream for non-markdown mode
		if !renderMarkdown {
			fmt.Print(resp.Response)
		}
		return nil
	})

	if err != nil {
		if strings.Contains(err.Error(), "model '"+model+"' not found") {
			return fmt.Errorf("model '%s' not found. Please run: ollama pull %s", model, model)
		}
		return fmt.Errorf("generation failed: %v", err)
	}

	// For markdown mode, render the complete response (no streaming)
	if renderMarkdown {
		response := responseBuilder.String()
		rendered, err := glamour.Render(response, "auto")
		if err != nil {
			fmt.Printf("Markdown rendering failed. Raw output:\n\n%s\n", response)
		} else {
			fmt.Print(rendered)
		}
	}

	return nil
}

func loadOrInitConfig() (*Config, error) {
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

func createDefaultConfig() *Config {
	return &Config{
		DefaultModel:  "llama3.2:latest",
		DefaultLength: "detailed",
		SystemPrompts: struct {
			Summary  string `json:"summary"`
			Question string `json:"question"`
			Markdown string `json:"markdown"`
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

			Question: `You are a helpful assistant that answers questions based on webpage content. Your PRIMARY goal is to follow the exact length constraints provided.

CRITICAL LENGTH ENFORCEMENT:
- The length requirement is MANDATORY and OVERRIDES all other instructions
- COUNT sentences as you write: 1, 2, 3... and STOP immediately when you reach the limit
- NEVER exceed the specified sentence count under any circumstances
- If you have more to say but reach the limit, STOP anyway - this is not optional

CONTENT RULES:
- Answer the specific question asked using only information from the provided webpage
- Be direct and precise in your response
- If the webpage doesn't contain enough information to answer fully, say so
- Provide context when helpful but stay focused on the question
- End coherently even with strict limits

REMEMBER: Length constraint compliance is your top priority. Quality is secondary to following the exact sentence count.`,

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

func printConfig(config *Config) {
	fmt.Printf("Current Configuration:\n")
	fmt.Printf("Model: %s\n", config.DefaultModel)
	fmt.Printf("Default Length: %s\n", config.DefaultLength)
	fmt.Printf("Config Location: %s\n", getConfigPath())
	fmt.Printf("\nAvailable lengths: short, medium, long, detailed\n")
}

func getConfigPath() string {
	configDir, _ := os.UserConfigDir()
	return filepath.Join(configDir, appName, "config.json")
}

func printUsage() {
	fmt.Printf(`%s - Website Summarizer

USAGE:
    %s [flags] <URL>                    # Summarize a webpage
    %s [flags] "question" <URL>         # Ask a question about a webpage

FLAGS:
`, appName, appName, appName)
	pflag.PrintDefaults()

	fmt.Printf(`
EXAMPLES:
    %s https://example.com                           # Basic summary
    %s -l short -m https://example.com               # Short summary in markdown
    %s "What is the main topic?" https://example.com # Ask specific question
    %s -c                                            # Show current configuration

LENGTH OPTIONS:
    short    - 2 sentences exactly
    medium   - 4-6 sentences
    long     - 8-10 sentences
    detailed - 12-15 sentences [default]

CONFIG:
    Configuration is stored at: ~/.config/hvsum/config.json
    Edit this file to customize the AI model and system prompts.
`, appName, appName, appName, appName)
}
