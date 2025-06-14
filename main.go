package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/glamour"
	"github.com/chzyer/readline"
	"github.com/go-shiori/go-readability"
	"github.com/ollama/ollama/api"
	"github.com/spf13/pflag"
)

const appName = "hvsum"

// Config holds all user-configurable settings
type Config struct {
	DefaultModel  string `json:"default_model"`
	DisablePager  bool   `json:"disable_pager"`
	DisableQnA    bool   `json:"disable_qna"`
	SystemPrompts struct {
		Summary  string `json:"summary"`
		Question string `json:"question"`
		QnA      string `json:"qna"`
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
	copyToClip = pflag.BoolP("copy", "c", false, "Copy summary to clipboard")
	saveToFile = pflag.StringP("save", "s", "", "Save summary to file")
	showHelp   = pflag.BoolP("help", "h", false, "Show help message")
	showConfig = pflag.Bool("config", false, "Show current configuration")
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

	// Determine the effective length setting
	effectiveLength := *length
	if effectiveLength == "" {
		effectiveLength = config.DefaultLength
	}
	if effectiveLength == "" {
		effectiveLength = "detailed" // fallback default
	}

	fmt.Fprintf(os.Stderr, "Fetching content from: %s\n", link)
	textContent, pageTitle, err := fetchAndParseURL(link)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Generate initial summary
	initialSummary, err := generateInitialSummary(config, effectiveLength, *markdown, textContent, pageTitle)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating summary: %v\n", err)
		os.Exit(1)
	}

	// Handle clipboard copy
	if *copyToClip {
		err := clipboard.WriteAll(initialSummary)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error copying to clipboard: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Summary copied to clipboard!\n")
		}
	}

	// Handle file save
	if *saveToFile != "" {
		err := os.WriteFile(*saveToFile, []byte(initialSummary), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error saving to file: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Summary saved to %s\n", *saveToFile)
		}
	}

	// Display summary - use pager if enabled, otherwise display directly
	if config.DisablePager {
		fmt.Fprintln(os.Stderr, "")
		renderToConsole(initialSummary, *markdown)
		if !config.DisableQnA {
			fmt.Fprintln(os.Stderr, "\n"+strings.Repeat("─", 60))
			fmt.Fprintln(os.Stderr, "Ask questions about the content above (Ctrl+C or Ctrl+D to exit):")
		}
	} else {
		renderWithPager(initialSummary, *markdown)
		if !config.DisableQnA {
			fmt.Fprintln(os.Stderr, "Ask questions about the content above (Ctrl+C or Ctrl+D to exit):")
		}
	}

	// Start interactive Q&A session if not disabled
	if !config.DisableQnA {
		startInteractiveSession(initialSummary, config, *markdown)
	}
}

func renderWithPager(content string, useMarkdown bool) {
	finalContent := content
	if useMarkdown {
		rendered, err := glamour.Render(content, "auto")
		if err != nil {
			fmt.Printf("Markdown rendering failed. Using raw output:\n\n")
		} else {
			finalContent = rendered
		}
	}

	// Use less with specific options for better display
	cmd := exec.Command("less", "-R", "-S", "-F", "-X")
	cmd.Stdin = strings.NewReader(finalContent)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set the terminal for less
	cmd.Env = append(os.Environ(), "LESS=-R -S -F -X")

	if err := cmd.Run(); err != nil {
		// Fallback to direct output if less fails
		fmt.Print(finalContent)
	}
}

func renderToConsole(content string, useMarkdown bool) {
	finalContent := content
	if useMarkdown {
		rendered, err := glamour.Render(content, "auto")
		if err != nil {
			fmt.Printf("Markdown rendering failed. Using raw output:\n\n%s\n", content)
		} else {
			finalContent = rendered
		}
	}
	fmt.Print(finalContent)
	fmt.Println() // Add a newline for better spacing in chat
}

func renderAndDisplay(content string, useMarkdown bool) {
	finalContent := content
	if useMarkdown {
		rendered, err := glamour.Render(content, "auto")
		if err != nil {
			fmt.Printf("Markdown rendering failed. Using raw output:\n\n")
		} else {
			finalContent = rendered
		}
	}

	// Use a pager to display the content
	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less"
	}
	path, err := exec.LookPath(pager)
	if err != nil {
		// Pager not found, fallback to printing
		fmt.Print(finalContent)
		return
	}

	cmd := exec.Command(path, "-R")
	cmd.Stdin = strings.NewReader(finalContent)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Pager returned an error (e.g., user quit), just print as a fallback
		fmt.Print(finalContent)
	}
}

func generateInitialSummary(config *Config, length string, renderMarkdown bool, textContent, pageTitle string) (string, error) {
	systemPrompt := config.SystemPrompts.Summary
	if renderMarkdown {
		systemPrompt += "\n\n" + config.SystemPrompts.Markdown
	}
	userPrompt := buildUserPrompt("", length, textContent, pageTitle)

	fmt.Fprintf(os.Stderr, "Generating summary with %s...\n\n", config.DefaultModel)

	client, err := api.ClientFromEnvironment()
	if err != nil {
		return "", fmt.Errorf("failed to connect to Ollama: %v", err)
	}

	stream := false // Never stream summary, so we can capture it
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

func startInteractiveSession(initialSummary string, config *Config, renderMarkdown bool) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not start interactive session: %v\n", err)
		return
	}

	messages := []api.Message{
		{
			Role:    "system",
			Content: config.SystemPrompts.QnA,
		},
		{
			Role:    "assistant",
			Content: "Here is the summary of the document we are discussing:\n\n" + initialSummary,
		},
	}

	// Set up readline for proper terminal input handling
	rl, err := readline.New("> ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not initialize readline: %v\n", err)
		return
	}
	defer rl.Close()

	for {
		question, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt || err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
			continue
		}

		if strings.TrimSpace(question) == "" {
			continue
		}

		messages = append(messages, api.Message{Role: "user", Content: question})

		isStreaming := !renderMarkdown // Stream if not using markdown
		req := &api.ChatRequest{
			Model:    config.DefaultModel,
			Messages: messages,
			Stream:   &isStreaming,
		}

		var responseBuilder strings.Builder
		fmt.Fprintf(os.Stderr, "\n")
		err = client.Chat(context.Background(), req, func(resp api.ChatResponse) error {
			content := resp.Message.Content
			responseBuilder.WriteString(content)
			if isStreaming {
				fmt.Print(content)
			}
			return nil
		})
		if !isStreaming { // Add a newline for markdown mode for better spacing
			fmt.Fprintf(os.Stderr, "\n")
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating response: %v\n", err)
			continue
		}

		fullResponse := responseBuilder.String()
		messages = append(messages, api.Message{Role: "assistant", Content: fullResponse})

		if !isStreaming {
			renderToConsole(fullResponse, renderMarkdown)
		} else {
			fmt.Println() // Ensure there's a newline after streaming
		}
	}

	fmt.Fprintln(os.Stderr, "\nExiting interactive mode.")
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
		DisablePager:  false, // Pager enabled by default
		DisableQnA:    false, // Q&A enabled by default
		SystemPrompts: struct {
			Summary  string `json:"summary"`
			Question string `json:"question"`
			QnA      string `json:"qna"`
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

			QnA: `You are an intelligent Q&A assistant. The user has just reviewed a document summary that you have provided. Your task is to answer their follow-up questions.

CRITICAL RULES:
1.  **Be Concise**: Answer questions directly and concisely. Provide a short, focused response.
2.  **Use Context First**: Prioritize your answers based on the provided document summary and conversation history.
3.  **Supplement with General Knowledge**: You are encouraged to use your own general knowledge to provide a more complete answer. However, if you use external information, you MUST state that it is not from the provided document. For example: "According to my general knowledge..." or "The document doesn't mention this, but generally...".
4.  **Stay on Topic**: Only answer questions related to the document or the ongoing conversation.`,

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
	fmt.Printf("Disable Pager: %t\n", config.DisablePager)
	fmt.Printf("Disable Q&A: %t\n", config.DisableQnA)
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
    %s https://example.com                           # Summary then interactive Q&A
    %s -l short -M https://example.com               # Short, markdown summary then Q&A
    %s -c https://example.com                        # Copy summary to clipboard
    %s -s summary.txt https://example.com            # Save summary to file
    %s --config                                      # Show current configuration

LENGTH OPTIONS (for initial summary):
    short    - 2 sentences exactly
    medium   - 4-6 sentences
    long     - 8-10 sentences
    detailed - 12-15 sentences [default]

CONFIG:
    Configuration is stored at: ~/.config/hvsum/config.json
    Edit this file to customize the AI model and system prompts.
    Set "disable_pager": true to show summary directly in terminal.
    Set "disable_qna": true to skip the interactive Q&A session.
`, appName, appName, appName, appName, appName)
}
