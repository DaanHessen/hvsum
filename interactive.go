package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/chzyer/readline"
	"github.com/ollama/ollama/api"
)

// StartInteractiveSession begins an enhanced interactive Q&A session
func StartInteractiveSession(initialSummary, contextContent string, config *Config, renderMarkdown, enableSearch bool) {
	DebugLog(config, "Starting enhanced interactive session with search: %v", enableSearch)

	client, err := api.ClientFromEnvironment()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Could not connect to Ollama: %v\n", err)
		return
	}

	// Initialize managers
	sessionManager := NewSessionManager(config)
	searchManager := NewSearchManager(config)
	cacheManager := NewCacheManager(config)

	// Clean expired cache and old sessions on startup
	go func() {
		cacheManager.CleanExpired()
		sessionManager.CleanOldSessions(30) // Keep sessions for 30 days
	}()

	// Create session if persistence is enabled
	var currentSession *SessionData
	if config.SessionPersist {
		title := extractTitleFromSummary(initialSummary)
		currentSession, _ = sessionManager.CreateSession(initialSummary, contextContent, title, enableSearch)
		if currentSession != nil {
			fmt.Fprintf(os.Stderr, "💾 Session saved: %s\n", currentSession.ID)
		}
	}

	// Set up readline with better configuration
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "❓ ",
		HistoryFile:     getHistoryFile(),
		AutoComplete:    createAutoCompleter(),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Could not initialize input: %v\n", err)
		return
	}
	defer rl.Close()

	// Handle Ctrl+C gracefully
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Fprintf(os.Stderr, "\n👋 Session saved. Goodbye!\n")
		if currentSession != nil {
			sessionManager.SaveSession(currentSession)
		}
		rl.Close()
		os.Exit(0)
	}()

	// Display welcome message
	displayWelcomeMessage(currentSession, enableSearch)

	// Main interaction loop
	for {
		question, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt || err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "❌ Input error: %v\n", err)
			continue
		}

		question = strings.TrimSpace(question)
		if question == "" {
			continue
		}

		// Handle special commands
		if handled := handleSpecialCommands(question, sessionManager, currentSession, rl); handled {
			if question == "/exit" || question == "/bye" || question == "/quit" {
				break
			}
			continue
		}

		DebugLog(config, "Processing question: %s", question)

		// Show thinking indicator
		fmt.Fprintf(os.Stderr, "🤔 Processing your question")
		dots := StartThinkingDots()

		// Generate response with caching
		response, err := generateEnhancedResponse(question, currentSession, config, client, searchManager, cacheManager, enableSearch)

		close(dots)
		fmt.Fprintf(os.Stderr, "\r                              \r") // Clear thinking indicator

		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
			continue
		}

		// Add to session if available
		if currentSession != nil {
			sessionManager.AddMessage(currentSession, "user", question)
			sessionManager.AddMessage(currentSession, "assistant", response)
			sessionManager.SaveSession(currentSession)
		}

		// Display response
		fmt.Fprintf(os.Stderr, "\n")
		RenderToConsole(response, renderMarkdown)
		fmt.Fprintf(os.Stderr, "\n")
	}

	// Save session before exit
	if currentSession != nil {
		sessionManager.SaveSession(currentSession)
		fmt.Fprintf(os.Stderr, "💾 Session saved\n")
	}

	fmt.Fprintf(os.Stderr, "👋 Goodbye!\n")
}

// generateEnhancedResponse creates a response with caching and search enhancement
func generateEnhancedResponse(question string, session *SessionData, config *Config, client *api.Client, searchManager *SearchManager, cacheManager *CacheManager, enableSearch bool) (string, error) {
	// Check cache first
	cacheKey := cacheManager.GetCacheKey(fmt.Sprintf("qa:%s:%s", question, session.InitialSummary[:Min(100, len(session.InitialSummary))]))
	var cachedResponse string
	if cacheManager.Get(cacheKey, &cachedResponse) {
		DebugLog(config, "Cache hit for Q&A")
		return cachedResponse, nil
	}

	// Prepare messages
	messages := session.Messages

	// Enhance with search if enabled and question seems to benefit from it
	var enhancedContent string
	if enableSearch && shouldEnhanceWithSearch(question) {
		fmt.Fprintf(os.Stderr, "\n🔍 Searching for additional context...")

		searchQueries, err := generateSearchQueries(config, session.ContextContent, question)
		if err == nil && len(searchQueries) > 0 {
			searchResults := searchManager.PerformParallelSearches(searchQueries, 3)
			if len(searchResults) > 0 {
				enhancedContent = FormatSearchResults(searchResults)
				DebugLog(config, "Enhanced question with %d search results", len(searchResults))
			}
		}
	}

	// Build final question
	finalQuestion := question
	if enhancedContent != "" {
		finalQuestion += enhancedContent + "\n\nPlease provide a concise answer based on the document summary and additional search context."
	}

	// Add instruction for concise responses
	finalQuestion += "\n\nIMPORTANT: Keep your response concise and directly answer the question. Maximum 3-4 sentences unless more detail is specifically requested."

	messages = append(messages, api.Message{Role: "user", Content: finalQuestion})

	// Generate response
	isStreaming := !config.DisablePager // Stream if pager is disabled
	req := &api.ChatRequest{
		Model:    config.DefaultModel,
		Messages: messages,
		Stream:   &isStreaming,
	}

	var responseBuilder strings.Builder
	err := client.Chat(context.Background(), req, func(resp api.ChatResponse) error {
		content := resp.Message.Content
		responseBuilder.WriteString(content)
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to generate response: %v", err)
	}

	response := responseBuilder.String()

	// Cache the response
	cacheManager.Set(cacheKey, response)

	return response, nil
}

// shouldEnhanceWithSearch determines if a question would benefit from web search
func shouldEnhanceWithSearch(question string) bool {
	lowerQ := strings.ToLower(question)

	// Keywords that suggest current/external information would be helpful
	searchTriggers := []string{
		"current", "latest", "recent", "today", "now", "update",
		"what happened", "news", "compare", "vs", "versus",
		"price", "cost", "how much", "where", "when", "who",
		"statistics", "data", "numbers", "research", "study",
	}

	for _, trigger := range searchTriggers {
		if strings.Contains(lowerQ, trigger) {
			return true
		}
	}

	return false
}

// handleSpecialCommands processes special interactive commands
func handleSpecialCommands(command string, sessionManager *SessionManager, currentSession *SessionData, rl *readline.Instance) bool {
	switch command {
	case "/help", "/h":
		displayHelp()
		return true

	case "/history", "/s":
		if currentSession != nil {
			fmt.Fprintln(os.Stderr, "📜 Conversation History:")
			for _, msg := range currentSession.Messages {
				if msg.Role == "user" || msg.Role == "assistant" {
					fmt.Fprintf(os.Stderr, "  [%s] %s\n", strings.Title(msg.Role), TruncateString(msg.Content, 100))
				}
			}
		} else {
			fmt.Fprintln(os.Stderr, "Session persistence is disabled. No history available.")
		}
		return true

	case "/clear", "/c":
		fmt.Print("\033[2J\033[H") // Clear screen
		displayWelcomeMessage(currentSession, true)
		return true

	case "/info", "/i":
		if currentSession != nil {
			currentSession.PrintSessionInfo()
		} else {
			fmt.Fprintf(os.Stderr, "📄 No active session (persistence disabled)\n")
		}
		return true

	case "/exit", "/bye", "/quit":
		return true

	default:
		return false
	}
}

// displayWelcomeMessage shows the initial welcome message
func displayWelcomeMessage(session *SessionData, searchEnabled bool) {
	fmt.Fprintf(os.Stderr, "🤖 Ready to answer questions")
	if session != nil {
		fmt.Fprintf(os.Stderr, " about: %s", session.GetTitle())
	}
	if searchEnabled {
		fmt.Fprintf(os.Stderr, " (🔍 web search enabled)")
	}
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "💡 Type /help for commands or /exit to quit\n\n")
}

// displayHelp shows available commands
func displayHelp() {
	fmt.Fprintf(os.Stderr, `
🆘 Available Commands:
  /help, /h         - Show this help
  /history, /s      - Show conversation history for this session
  /clear, /c        - Clear screen
  /info, /i         - Show current session info
  /exit, /bye, /quit - Exit interactive mode

💡 Session Management:
  - Sessions are automatically saved if enabled.
  - To resume a previous session, exit and run:
    hvsum --list-sessions
    hvsum -i <session_id>

`)
}

// createAutoCompleter creates an auto-completer for readline
func createAutoCompleter() readline.AutoCompleter {
	return readline.NewPrefixCompleter(
		readline.PcItem("/help"),
		readline.PcItem("/history"),
		readline.PcItem("/clear"),
		readline.PcItem("/info"),
		readline.PcItem("/exit"),
		readline.PcItem("/bye"),
		readline.PcItem("/quit"),
	)
}

// getHistoryFile returns the path to the readline history file
func getHistoryFile() string {
	configDir, _ := os.UserConfigDir()
	return filepath.Join(configDir, appName, "history")
}

// extractTitleFromSummary extracts a title from the summary for session naming
func extractTitleFromSummary(summary string) string {
	lines := strings.Split(summary, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && len(line) < 100 {
			// Clean up markdown and return first meaningful line
			title := strings.TrimPrefix(line, "# ")
			title = strings.TrimPrefix(title, "## ")
			if len(title) > 60 {
				title = title[:57] + "..."
			}
			return title
		}
	}
	return "Interactive Session"
}

// StartThinkingDots shows animated thinking indicator
func StartThinkingDots() chan struct{} {
	stop := make(chan struct{})

	go func() {
		dots := 0
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				fmt.Fprintf(os.Stderr, "\r🤔 Processing your question%s   ", strings.Repeat(".", dots%4))
				dots++
			}
		}
	}()

	return stop
}
