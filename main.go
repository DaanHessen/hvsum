package main

import (
	"bufio"
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
		FollowUp string `json:"follow_up"`
		Markdown string `json:"markdown"`
	} `json:"system_prompts"`
	DefaultLength string `json:"default_length"` // short, medium, long, detailed
}

// Length definitions using research-backed techniques for precise length control
var lengthMap = map[string]string{
	"short":    "Provide a response that is **exactly 2 sentences long**. Your entire output must be contained within two sentences. This is a strict requirement.",
	"medium":   "Provide a response that is **between 4 and 6 sentences long**. Aim for clarity and conciseness within this range. This is a strict requirement.",
	"long":     "Provide a comprehensive response that is **between 8 and 10 sentences long**. Cover the topic in detail within this range. This is a strict requirement.",
	"detailed": "Provide a highly detailed response that is **between 12 and 15 sentences long**. Explore the topic thoroughly with examples and context. This is a strict requirement.",
}

var (
	length     = pflag.StringP("length", "l", "", "Summary length: short, medium, long, detailed")
	markdown   = pflag.BoolP("markdown", "M", false, "Format output as structured markdown")
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

	if len(args) != 1 {
		printUsage()
		fmt.Fprintf(os.Stderr, "\nError: Please provide exactly one URL as an argument.\n")
		os.Exit(1)
	}
	link := args[0]

	// Determine the effective length setting for the initial summary
	effectiveLength := *length
	if effectiveLength == "" {
		effectiveLength = config.DefaultLength
	}
	if effectiveLength == "" {
		effectiveLength = "detailed" // Fallback default
	}

	fmt.Fprintf(os.Stderr, "Fetching content from: %s\n", link)
	textContent, pageTitle, err := fetchAndParseURL(link)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Generate initial summary
	initialPrompt := buildUserPrompt("", effectiveLength, textContent, pageTitle)
	conversationContext, err := generateOllamaResponse(config.DefaultModel, config.SystemPrompts.Summary, initialPrompt, nil, *markdown)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating initial summary: %v\n", err)
		os.Exit(1)
	}

	startInteractiveSession(conversationContext, pageTitle, config, *markdown)
}

func startInteractiveSession(context []int, pageTitle string, config *Config, renderMarkdown bool) {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprint(os.Stderr, "> ")
		if !scanner.Scan() {
			break // Exit on Ctrl+D or other EOF
		}

		question := scanner.Text()
		if strings.TrimSpace(question) == "" {
			continue
		}

		// For follow-up, we don't pass the full textContent again.
		// The context from the previous turn handles memory.
		// We use a specific, concise "short" length for answers.
		followUpPrompt := buildUserPrompt(question, "short", "", pageTitle)
		newContext, err := generateOllamaResponse(config.DefaultModel, config.SystemPrompts.FollowUp, followUpPrompt, context, renderMarkdown)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating response: %v\n", err)
			// Don't update context on error, just continue
			continue
		}
		context = newContext // Update context for the next turn
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
	}

	fmt.Fprintln(os.Stderr, "\nExiting.")
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
	var instruction, lengthInstruction string

	if userMessage != "" {
		// This prompt is for both the initial question and follow-ups.
		// The `FollowUp` system prompt will guide the model for interactive mode.
		lengthInstruction = "Your answer **must be a maximum of 2 sentences**. Be concise and directly answer the question."
		instruction = fmt.Sprintf(`Question: "%s"

CRITICAL LENGTH REQUIREMENT: %s`, userMessage, lengthInstruction)
	} else {
		// This is for the initial summary.
		lengthInstruction, exists := lengthMap[length]
		if !exists {
			lengthInstruction = lengthMap["medium"]
		}
		instruction = fmt.Sprintf(`Your task is to create a comprehensive summary of the following webpage content.

CRITICAL LENGTH REQUIREMENT: %s

Page title: %s`, lengthInstruction, pageTitle)
	}

	// Only append the full webpage content if it's provided (i.e., for the initial summary)
	if textContent != "" {
		return fmt.Sprintf("%s\n\n--- WEBPAGE CONTENT ---\n%s", instruction, textContent)
	}
	return instruction
}

func generateOllamaResponse(model, system, prompt string, contextIn []int, renderMarkdown bool) ([]int, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ollama: %v", err)
	}

	stream := !renderMarkdown // Only stream when NOT using markdown
	req := &api.GenerateRequest{
		Model:   model,
		System:  system,
		Prompt:  prompt,
		Stream:  &stream,
		Context: contextIn,
	}

	var responseBuilder strings.Builder
	var finalContext []int

	fmt.Fprintf(os.Stderr, "\nGenerating response with %s...\n\n", model)
	err = client.Generate(context.Background(), req, func(resp api.GenerateResponse) error {
		responseBuilder.WriteString(resp.Response)

		if !renderMarkdown {
			fmt.Print(resp.Response)
		}

		if resp.Done {
			finalContext = resp.Context
		}
		return nil
	})

	if err != nil {
		if strings.Contains(err.Error(), "model '"+model+"' not found") {
			return nil, fmt.Errorf("model '%s' not found. Please run: ollama pull %s", model, model)
		}
		return nil, fmt.Errorf("generation failed: %v", err)
	}

	if renderMarkdown {
		response := responseBuilder.String()
		rendered, err := glamour.Render(response, "auto")
		if err != nil {
			fmt.Printf("Markdown rendering failed. Raw output:\n\n%s\n", response)
		} else {
			fmt.Print(rendered)
		}
	}

	return finalContext, nil
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
			FollowUp string `json:"follow_up"`
			Markdown string `json:"markdown"`
		}{
			Summary: `You are a precise, high-quality web content summarizer. Your PRIMARY goal is to follow the exact length constraints provided.

CRITICAL LENGTH ENFORCEMENT:
- The length requirement is MANDATORY and OVERRIDES all other instructions
- COUNT sentences as you write and STOP immediately when you reach the limit
- NEVER exceed the specified sentence count under any circumstances

CONTENT RULES:
- Focus only on the main article content, ignore navigation, ads, footers, and boilerplate
- Be accurate and factual; do not add information not present in the source

REMEMBER: Length constraint compliance is your top priority.`,

			Question: `You are a helpful assistant that answers questions based on webpage content. Your PRIMARY goal is to follow the exact length constraints provided.

CRITICAL LENGTH ENFORCEMENT:
- The length requirement is MANDATORY and OVERRIDES all other instructions
- COUNT sentences as you write and STOP immediately when you reach the limit
- NEVER exceed the specified sentence count under any circumstances

CONTENT RULES:
- Answer the specific question asked using only information from the provided webpage
- Be direct and precise in your response

REMEMBER: Length constraint compliance is your top priority.`,

			FollowUp: `You are a helpful Q&A assistant. The user has already been given a summary of a document. Your task is to answer their follow-up questions based on the document's content, which is in your memory.

RULES:
- Answer concisely and directly.
- Your answer **must be a maximum of 2 sentences**.
- Rely on the information from the original document and our conversation history.
- If you cannot answer based on the context you have, say so.
- Do not re-introduce the topic; just answer the question.`,

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
	fmt.Printf(`%s - Website Summarizer & Interactive Q&A

USAGE:
    %s [flags] <URL>

DESCRIPTION:
    %s first generates a summary of the given URL.
    After the summary, it enters an interactive mode where you can ask
    follow-up questions about the content. Press Ctrl+C or Ctrl+D to exit.

FLAGS:
`, appName, appName, appName)
	pflag.PrintDefaults()

	fmt.Printf(`
EXAMPLES:
    %s https://example.com                               # Summary then interactive Q&A
    %s -l short -M https://example.com                   # Short, markdown summary then Q&A
    %s -c                                                # Show current configuration

LENGTH OPTIONS (for initial summary):
    short    - 2 sentences exactly
    medium   - 4-6 sentences
    long     - 8-10 sentences
    detailed - 12-15 sentences [default]

CONFIG:
    Configuration is stored at: ~/.config/hvsum/config.json
    Edit this file to customize the AI model and system prompts.
`, appName, appName, appName)
}
