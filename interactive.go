package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/ollama/ollama/api"
)

// StartInteractiveSession begins the interactive Q&A session
func StartInteractiveSession(initialSummary, contextContent string, config *Config, renderMarkdown, enableSearch bool) {
	DebugLog(config, "Starting interactive session with search enabled: %v", enableSearch)

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

	searchManager := NewSearchManager(config)

	for {
		question, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt || err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
			continue
		}

		question = strings.TrimSpace(question)
		if question == "" {
			continue
		}

		// Check for exit commands
		if question == "/bye" || question == "/exit" || question == "/quit" {
			fmt.Fprintln(os.Stderr, "Goodbye!")
			break
		}

		DebugLog(config, "User question: %s", question)

		// Enhance question with web search if enabled
		var enhancedContent string
		if enableSearch {
			fmt.Fprintf(os.Stderr, "🔍 Searching for additional information...\n")

			// Generate search queries for the question
			searchQueries, err := generateSearchQueries(config, contextContent, question)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Could not generate search queries: %v\n", err)
				DebugLog(config, "Search query generation failed: %v", err)
			} else {
				// Use parallel searches for better performance
				fmt.Fprintf(os.Stderr, "🚀 Performing parallel searches for your question...\n")
				searchResults := searchManager.PerformParallelSearches(searchQueries, 2)

				if len(searchResults) > 0 {
					enhancedContent = FormatSearchResults(searchResults)
					DebugLog(config, "Enhanced question with %d search results", len(searchResults))
				}
			}
		}

		finalQuestion := question
		if enhancedContent != "" {
			finalQuestion += enhancedContent + "\n\nPlease answer the question using both the document summary and the additional search results above."
		}

		// Enforce concise answers (2-3 sentences maximum)
		finalQuestion += "\n\nCRITICAL REQUIREMENT: Provide a direct, concise answer in no more than a couple sentences. Do NOT add extraneous information."

		messages = append(messages, api.Message{Role: "user", Content: finalQuestion})

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

		if !isStreaming {
			fmt.Fprintf(os.Stderr, "\n")
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating response: %v\n", err)
			continue
		}

		fullResponse := responseBuilder.String()
		messages = append(messages, api.Message{Role: "assistant", Content: fullResponse})

		if !isStreaming {
			RenderToConsole(fullResponse, renderMarkdown)
		} else {
			fmt.Println() // Ensure there's a newline after streaming
		}

		DebugLog(config, "Response generated, length: %d characters", len(fullResponse))
	}

	fmt.Fprintln(os.Stderr, "\nExiting interactive mode.")
}
