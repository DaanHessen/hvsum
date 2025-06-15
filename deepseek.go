package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DeepSeekMessage represents a message in the DeepSeek API format
type DeepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// DeepSeekRequest represents the request structure for DeepSeek API
type DeepSeekRequest struct {
	Model     string            `json:"model"`
	Messages  []DeepSeekMessage `json:"messages"`
	Stream    bool              `json:"stream"`
	MaxTokens int               `json:"max_tokens,omitempty"`
}

// DeepSeekResponse represents the response structure from DeepSeek API
type DeepSeekResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role             string `json:"role"`
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content,omitempty"`
		} `json:"message"`
		Delta struct {
			Role             string `json:"role,omitempty"`
			Content          string `json:"content,omitempty"`
			ReasoningContent string `json:"reasoning_content,omitempty"`
		} `json:"delta,omitempty"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// DeepSeekClient handles communication with DeepSeek API
type DeepSeekClient struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// NewDeepSeekClient creates a new DeepSeek API client
func NewDeepSeekClient(config *Config) *DeepSeekClient {
	if !config.DeepSeekConfig.Enabled {
		return nil
	}

	apiKey := config.DeepSeekConfig.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("DEEPSEEK_API_KEY")
	}

	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "Warning: DeepSeek API key not found. Set DEEPSEEK_API_KEY environment variable or configure in config file.\n")
		return nil
	}

	return &DeepSeekClient{
		APIKey:  apiKey,
		BaseURL: config.DeepSeekConfig.BaseURL,
		HTTPClient: &http.Client{
			Timeout: 300 * time.Second, // 5 minutes timeout for reasoning
		},
	}
}

// GenerateWithReasoning calls DeepSeek API with streaming support for thinking process
func (client *DeepSeekClient) GenerateWithReasoning(config *Config, systemPrompt, userPrompt string, useMarkdown bool) (string, error) {
	if client == nil {
		return "", fmt.Errorf("DeepSeek client not initialized")
	}

	messages := []DeepSeekMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	// Enable streaming only for thinking process when markdown is active
	// Stream thinking but not the final summary for clean markdown rendering
	enableStreaming := useMarkdown && config.DeepSeekConfig.ShowThinking

	request := DeepSeekRequest{
		Model:     config.DeepSeekConfig.Model,
		Messages:  messages,
		Stream:    enableStreaming,
		MaxTokens: config.DeepSeekConfig.MaxTokens,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", client.BaseURL+"/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+client.APIKey)

	// Show progress indicator
	fmt.Fprintf(os.Stderr, "🧠 Generating summary with %s...\n", config.DeepSeekConfig.Model)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	if enableStreaming {
		return client.handleStreamingResponse(resp.Body, config.DeepSeekConfig.ShowThinking, useMarkdown)
	} else {
		return client.handleNonStreamingResponse(resp.Body, config.DeepSeekConfig.ShowThinking)
	}
}

// handleNonStreamingResponse processes non-streaming response from DeepSeek API
func (client *DeepSeekClient) handleNonStreamingResponse(body io.Reader, showThinking bool) (string, error) {
	responseBody, err := io.ReadAll(body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	var response DeepSeekResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	choice := response.Choices[0]

	// Show thinking process if enabled
	if showThinking && choice.Message.ReasoningContent != "" {
		fmt.Fprintf(os.Stderr, "\n<thinking>\n%s\n</thinking>\n\n", choice.Message.ReasoningContent)
	}

	fmt.Fprintf(os.Stderr, "✅ Summary generated successfully\n")
	return choice.Message.Content, nil
}

// handleStreamingResponse processes the streaming response from DeepSeek API
func (client *DeepSeekClient) handleStreamingResponse(body io.Reader, showThinking bool, useMarkdown bool) (string, error) {
	scanner := bufio.NewScanner(body)
	var reasoningContent strings.Builder
	var finalContent strings.Builder
	thinkingPhase := true
	thinkingStarted := false

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var response DeepSeekResponse
		if err := json.Unmarshal([]byte(data), &response); err != nil {
			DebugLog(&Config{DebugMode: true}, "Failed to parse streaming response: %v", err)
			continue
		}

		if len(response.Choices) == 0 {
			continue
		}

		choice := response.Choices[0]

		// Handle reasoning content (thinking phase)
		if choice.Delta.ReasoningContent != "" {
			reasoningContent.WriteString(choice.Delta.ReasoningContent)
			if showThinking && !thinkingStarted {
				fmt.Fprintf(os.Stderr, "\n<thinking>\n")
				thinkingStarted = true
			}
			if showThinking {
				fmt.Print(choice.Delta.ReasoningContent)
			}
		}

		// Handle final answer content - buffer everything for clean markdown rendering
		if choice.Delta.Content != "" {
			// If we transition from thinking to answering, close thinking block
			if thinkingPhase {
				thinkingPhase = false
				if showThinking && thinkingStarted {
					fmt.Print("\n</thinking>\n\n")
					fmt.Fprintf(os.Stderr, "✅ Thinking complete, buffering final summary...\n")
				}
			}

			// Always buffer the final content - don't stream it for clean markdown
			finalContent.WriteString(choice.Delta.Content)
		}

		// Check if we've finished
		if choice.FinishReason != "" {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading stream: %v", err)
	}

	if showThinking && thinkingStarted && thinkingPhase {
		fmt.Print("\n</thinking>\n\n")
	}

	fmt.Fprintf(os.Stderr, "✅ Summary generated successfully\n")
	return finalContent.String(), nil
}

// GenerateOutlineWithDeepSeek creates an outline using DeepSeek API
func (client *DeepSeekClient) GenerateOutlineWithDeepSeek(summary string, config *Config, useMarkdown bool) (string, error) {
	if client == nil {
		return "", fmt.Errorf("DeepSeek client not initialized")
	}

	systemPrompt := `You are an expert content organizer. Create a structured outline from the provided summary.

OUTLINE REQUIREMENTS:
1. Use clear hierarchical structure (I, A, 1, a format or markdown headers)
2. Capture all main topics and subtopics from the source
3. Maintain logical flow and organization
4. Use parallel structure for similar items
5. Include sufficient detail to be useful as a reference

CRITICAL: Base the outline ONLY on information present in the provided summary. Do not add external information.`

	userPrompt := fmt.Sprintf("Create a structured outline from this summary:\n\n%s", summary)

	return client.GenerateWithReasoning(config, systemPrompt, userPrompt, useMarkdown)
}

// ShouldUseDeepSeek determines if DeepSeek should be used based on configuration
func ShouldUseDeepSeek(config *Config) bool {
	return config.DeepSeekConfig.Enabled && config.DeepSeekConfig.APIKey != "" || os.Getenv("DEEPSEEK_API_KEY") != ""
}

// CallDeepSeekOrFallback attempts to use DeepSeek API, falls back to Ollama if unavailable
func CallDeepSeekOrFallback(config *Config, systemPrompt, userPrompt string, useMarkdown bool) (string, error) {
	if ShouldUseDeepSeek(config) {
		client := NewDeepSeekClient(config)
		if client != nil {
			result, err := client.GenerateWithReasoning(config, systemPrompt, userPrompt, useMarkdown)
			if err != nil {
				fmt.Fprintf(os.Stderr, "⚠️ DeepSeek API failed, falling back to local model: %v\n", err)
				return callOllama(config, systemPrompt, userPrompt)
			}
			return result, nil
		}
	}

	// Fallback to Ollama
	return callOllama(config, systemPrompt, userPrompt)
}
