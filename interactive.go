package main

import (
	"context"
	"fmt"
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
func StartInteractiveSession(session *SessionData, config *Config, renderMarkdown, enableSearch bool) {
	if session == nil {
		fmt.Fprintln(os.Stderr, "Cannot start interactive session without session data.")
		return
	}
	DebugLog(config, "Starting enhanced interactive session for: %s", session.ID)

	// Clear screen for a "new window" feel
	fmt.Print("\033[2J\033[H")

	client, err := api.ClientFromEnvironment()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Could not connect to Ollama: %v\n", err)
		return
	}

	// Initialize managers
	sessionManager := NewSessionManager(config)
	searchManager := NewSearchManager(config)
	cacheManager := NewCacheManager(config)

	// Clean expired cache on startup
	go cacheManager.CleanExpired()

	currentSession := session

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
		handleSessionExit(currentSession, sessionManager, cacheManager, rl)
		rl.Close()
		os.Exit(0)
	}()

	// Display welcome message and session context
	displaySessionWelcome(currentSession, enableSearch, renderMarkdown)

	// Main interaction loop
	for {
		question, err := rl.Readline()
		if err != nil {
			handleSessionExit(currentSession, sessionManager, cacheManager, rl)
			break
		}

		question = strings.TrimSpace(question)
		if question == "" {
			continue
		}

		// Handle special commands
		if handled := handleSpecialCommands(question, sessionManager, currentSession, rl, renderMarkdown); handled {
			if question == "/exit" || question == "/bye" || question == "/quit" {
				handleSessionExit(currentSession, sessionManager, cacheManager, rl)
				break
			}
			continue
		}

		DebugLog(config, "Processing question: %s", question)

		// Show thinking indicator
		thinkingMsg := "🤔 Processing"
		stopDots := StartThinkingDots(thinkingMsg)

		// Generate response with caching
		response, err := generateEnhancedResponse(question, currentSession, config, client, searchManager, cacheManager, enableSearch)

		close(stopDots)
		// Ensure the line is fully cleared before printing the response
		fmt.Fprintf(os.Stderr, "\r\033[K")

		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
			continue
		}

		// Add to session
		sessionManager.AddMessage(currentSession, "user", question)
		sessionManager.AddMessage(currentSession, "assistant", response)

		// Display response
		fmt.Fprintf(os.Stderr, "\n")
		RenderToConsole(response, renderMarkdown)
		fmt.Fprintf(os.Stderr, "\n")
	}

	fmt.Fprintf(os.Stderr, "👋 Goodbye!\n")
}

// displaySessionWelcome shows welcome message with session context
func displaySessionWelcome(session *SessionData, searchEnabled bool, renderMarkdown bool) {
	fmt.Fprintf(os.Stderr, "🤖 Interactive Session: %s", session.GetTitle())
	if searchEnabled {
		fmt.Fprintf(os.Stderr, " (🔍 web search enabled)")
	}
	fmt.Fprintf(os.Stderr, "\n")

	// Show initial summary if this is a resumed session with content
	if session.InitialSummary != "" && hasUserMessages(session) {
		fmt.Fprintf(os.Stderr, "\n📋 Session Summary:\n")
		fmt.Fprintf(os.Stderr, "─────────────────\n")
		RenderToConsole(session.InitialSummary, renderMarkdown)
		fmt.Fprintf(os.Stderr, "\n")

		// Show recent conversation history
		userMessages := getUserMessages(session)
		if len(userMessages) > 0 {
			fmt.Fprintf(os.Stderr, "💬 Recent Questions:\n")
			for i, msg := range userMessages {
				if i >= 3 { // Show only last 3 questions
					break
				}
				fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, TruncateString(msg.Content, 80))
			}
			fmt.Fprintf(os.Stderr, "\n")
		}
	}

	fmt.Fprintf(os.Stderr, "💡 Type /help for commands, /history to see full conversation, or /exit to quit\n\n")
}

// hasUserMessages checks if session has actual user interactions
func hasUserMessages(session *SessionData) bool {
	for _, msg := range session.Messages {
		if msg.Role == "user" {
			return true
		}
	}
	return false
}

// getUserMessages returns user messages in reverse chronological order (newest first)
func getUserMessages(session *SessionData) []api.Message {
	var userMessages []api.Message
	for i := len(session.Messages) - 1; i >= 0; i-- {
		if session.Messages[i].Role == "user" {
			userMessages = append(userMessages, session.Messages[i])
		}
	}
	return userMessages
}

// handleSessionExit manages session saving with save/discard/delete options
func handleSessionExit(session *SessionData, sm *SessionManager, cm *CacheManager, rl *readline.Instance) {
	if session == nil {
		return
	}

	if !sm.config.SessionPersist {
		cm.ClearSessionCache(session.ID)
		fmt.Fprintln(os.Stderr, "🗑️ Session not saved (persistence disabled). Cache cleared.")
		return
	}

	// Always prompt to save, regardless of content
	rl.SetPrompt("💾 Save session? [S]ave / [D]iscard / Delete [R]ecord: ")
	defer rl.SetPrompt("❓ ")

	for {
		answer, err := rl.Readline()
		if err != nil {
			// On error or interrupt, default to discarding
			cm.ClearSessionCache(session.ID)
			fmt.Fprintln(os.Stderr, "\n🗑️ Session discarded. Cache cleared.")
			return
		}

		answer = strings.ToLower(strings.TrimSpace(answer))

		switch answer {
		case "s", "save", "":
			// Prompt for session name
			rl.SetPrompt("📝 Session name (Enter for auto-generated): ")
			sessionName, err := rl.Readline()
			if err != nil {
				cm.ClearSessionCache(session.ID)
				fmt.Fprintln(os.Stderr, "\n🗑️ Session discarded. Cache cleared.")
				return
			}

			sessionName = strings.TrimSpace(sessionName)
			if sessionName == "" {
				sessionName = generateSessionName(session.Title)
			} else {
				sessionName = cleanSessionName(sessionName)
			}

			// Update session and save
			session.ID = sessionName
			session.Title = sessionName
			err = sm.SaveSession(session)
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ Error saving session: %v\n", err)
				cm.ClearSessionCache(session.ID)
				return
			}

			cm.CommitSessionCache(session.ID)
			fmt.Fprintf(os.Stderr, "💾 Session saved as: %s\n", sessionName)
			return

		case "d", "discard":
			cm.ClearSessionCache(session.ID)
			fmt.Fprintln(os.Stderr, "🗑️ Session discarded. Cache cleared.")
			return

		case "r", "delete":
			// Delete existing session if it exists
			if sm.SessionExists(session.ID) {
				err := sm.DeleteSession(session.ID)
				if err != nil {
					fmt.Fprintf(os.Stderr, "❌ Error deleting session: %v\n", err)
				} else {
					fmt.Fprintf(os.Stderr, "🗑️ Session '%s' deleted permanently.\n", session.ID)
				}
			}
			cm.ClearSessionCache(session.ID)
			return

		default:
			fmt.Fprintln(os.Stderr, "Please choose [S]ave, [D]iscard, or Delete [R]ecord:")
		}
	}
}

// generateSessionName creates a session name from the title
func generateSessionName(title string) string {
	if title == "" {
		return fmt.Sprintf("session_%d", time.Now().Unix())
	}

	// Take first few meaningful words
	words := strings.Fields(title)
	var nameWords []string
	for i, word := range words {
		if i >= 3 {
			break
		}
		cleaned := cleanSessionName(word)
		if cleaned != "" {
			nameWords = append(nameWords, cleaned)
		}
	}

	if len(nameWords) == 0 {
		return fmt.Sprintf("session_%d", time.Now().Unix())
	}

	return strings.Join(nameWords, "_")
}

// cleanSessionName removes invalid characters from session names
func cleanSessionName(name string) string {
	// Replace spaces and special chars with underscores
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")

	// Keep only alphanumeric and underscores
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		}
	}

	return strings.ToLower(result.String())
}

// generateEnhancedResponse creates a response with intelligent search fallback
func generateEnhancedResponse(question string, session *SessionData, config *Config, client *api.Client, searchManager *SearchManager, cacheManager *CacheManager, enableSearch bool) (string, error) {
	// Check cache first
	cacheKey := cacheManager.GetCacheKey(fmt.Sprintf("qa:%s:%s", question, session.InitialSummary[:Min(100, len(session.InitialSummary))]))
	var cachedResponse string
	if cacheManager.Get(cacheKey, &cachedResponse) {
		DebugLog(config, "Cache hit for Q&A")
		return cachedResponse, nil
	}

	// Build context-rich system prompt
	systemPrompt := config.SystemPrompts.QnA

	// Prepare the document context
	documentContext := fmt.Sprintf(`DOCUMENT SUMMARY:
%s

FULL DOCUMENT CONTEXT:
%s

---

Based ONLY on the above document content, answer the following question. If the answer is not in the document, respond with exactly: "SEARCH_NEEDED: [brief description of what information is missing]"`, session.InitialSummary, session.ContextContent[:Min(2000, len(session.ContextContent))])

	// Build conversation context for pronoun resolution
	conversationContext := ""
	if len(session.Messages) > 0 {
		// Get last few messages for context
		recentMessages := session.Messages
		if len(recentMessages) > 6 {
			recentMessages = recentMessages[len(recentMessages)-6:]
		}

		var contextParts []string
		for _, msg := range recentMessages {
			if msg.Role == "user" || msg.Role == "assistant" {
				contextParts = append(contextParts, fmt.Sprintf("%s: %s", strings.Title(msg.Role), msg.Content))
			}
		}
		if len(contextParts) > 0 {
			conversationContext = fmt.Sprintf(`

RECENT CONVERSATION CONTEXT:
%s

`, strings.Join(contextParts, "\n"))
		}
	}

	// First attempt: try with existing context
	userPrompt := fmt.Sprintf(`%s%s

QUESTION: %s

INSTRUCTIONS: Your task is to answer the user's QUESTION using the provided context.

1. First, evaluate if the "DOCUMENT SUMMARY", "FULL DOCUMENT CONTEXT", and "RECENT CONVERSATION CONTEXT" contain enough information to fully and comprehensively answer the question.
2. Pay close attention to requests for more detail, elaboration, or specific information. If the user asks for more detail (e.g., "in a couple of paragraphs") and the context only provides a brief summary, you must treat the context as insufficient.
3. If the context is insufficient to provide a detailed, comprehensive answer that meets the user's request, you MUST respond with ONLY the string "SEARCH_NEEDED: [a concise search query to find the missing information]". Do not provide a partial or summary answer from the existing context in this case.
4. If the context IS sufficient, provide a complete and comprehensive answer based on the provided information.`, documentContext, conversationContext, question)

	messages := []api.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	// Generate initial response
	isStreaming := !config.DisablePager
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

	initialResponse := strings.TrimSpace(responseBuilder.String())

	// Determine if a search is needed based on the initial response.
	var needsSearch bool
	var modelSearchQuery string

	if strings.HasPrefix(initialResponse, "SEARCH_NEEDED:") {
		needsSearch = true
		modelSearchQuery = strings.TrimSpace(strings.TrimPrefix(initialResponse, "SEARCH_NEEDED:"))
		// Clean up the query - remove brackets if present
		modelSearchQuery = strings.Trim(modelSearchQuery, "[]")
		DebugLog(config, "Model explicitly requested search with query: '%s'", modelSearchQuery)
	} else if enableSearch && (len(initialResponse) < 35 || containsSearchTriggers(initialResponse)) {
		// The model didn't ask for a search, but the response is too short or contains trigger
		// phrases indicating it doesn't know the answer.
		needsSearch = true
		DebugLog(config, "Initial response is unhelpful (short or has triggers), forcing search.")
	}

	if enableSearch && needsSearch {
		DebugLog(config, "Search needed - performing automatic search")

		// Build context for search query generation
		recentContext := ""
		if len(session.Messages) >= 2 {
			lastUserMsg := ""
			lastAssistantMsg := ""
			for i := len(session.Messages) - 1; i >= 0; i-- {
				if session.Messages[i].Role == "user" && lastUserMsg == "" {
					lastUserMsg = session.Messages[i].Content
				} else if session.Messages[i].Role == "assistant" && lastAssistantMsg == "" {
					lastAssistantMsg = session.Messages[i].Content
				}
				if lastUserMsg != "" && lastAssistantMsg != "" {
					break
				}
			}
			if lastUserMsg != "" && lastAssistantMsg != "" {
				recentContext = fmt.Sprintf(" Previous Q&A: Q: %s A: %s", lastUserMsg, lastAssistantMsg[:Min(200, len(lastAssistantMsg))])
			}
		}

		// Fix the parameters for generateSearchQueries - contextText first, then purpose (user's question)
		searchContext := fmt.Sprintf("%s.%s", session.ContextContent[:Min(600, len(session.ContextContent))], recentContext)
		searchQueries, err := generateSearchQueries(config, searchContext, question, session.ID)

		// Prepend the model's suggested query to the list if it exists
		if modelSearchQuery != "" {
			searchQueries = append([]string{modelSearchQuery}, searchQueries...)
		}

		DebugLog(config, "Generated %d search queries, performing searches...", len(searchQueries))

		if err == nil && len(searchQueries) > 0 {
			fmt.Fprintf(os.Stderr, "\n🔍 Searching for additional information...")
			searchResults := searchManager.PerformParallelSearches(searchQueries, 3, session.ID)

			DebugLog(config, "Search completed, found %d results", len(searchResults))

			if len(searchResults) > 0 {
				DebugLog(config, "Found additional information via search, regenerating response")

				// Regenerate response with search results - use clearer instructions
				enhancedContext := fmt.Sprintf(`%s%s

ADDITIONAL SEARCH RESULTS:
%s

Now you have both the document content and search results. Answer the question completely using all available information.`, documentContext, conversationContext, FormatSearchResults(searchResults))

				enhancedPrompt := fmt.Sprintf(`%s

QUESTION: %s

INSTRUCTIONS: Provide a complete and comprehensive answer using the document content and search results. Pay attention to pronouns and references from previous questions. Do NOT respond with "SEARCH_NEEDED" - provide the actual answer.`, enhancedContext, question)

				enhancedMessages := []api.Message{
					{Role: "system", Content: systemPrompt},
					{Role: "user", Content: enhancedPrompt},
				}

				req = &api.ChatRequest{
					Model:    config.DefaultModel,
					Messages: enhancedMessages,
					Stream:   &isStreaming,
				}

				var enhancedBuilder strings.Builder
				err = client.Chat(context.Background(), req, func(resp api.ChatResponse) error {
					content := resp.Message.Content
					enhancedBuilder.WriteString(content)
					return nil
				})

				if err == nil {
					finalResponse := enhancedBuilder.String()
					// Cache the enhanced response
					cacheManager.Set(cacheKey, finalResponse, session.ID)
					return finalResponse, nil
				} else {
					DebugLog(config, "Error generating enhanced response: %v", err)
				}
			} else {
				DebugLog(config, "No search results found")
			}
		} else {
			DebugLog(config, "Search query generation failed or no queries: %v", err)
		}

		// If search failed or no results, return a helpful message
		fallbackResponse := "I don't have enough information in the document to answer this question completely, and my search for additional information didn't yield relevant results."
		cacheManager.Set(cacheKey, fallbackResponse, session.ID)
		return fallbackResponse, nil
	}

	// Only cache responses that are NOT search requests
	if !strings.HasPrefix(initialResponse, "SEARCH_NEEDED:") {
		cacheManager.Set(cacheKey, initialResponse, session.ID)
	}
	return initialResponse, nil
}

// containsSearchTriggers checks if response indicates missing information
func containsSearchTriggers(response string) bool {
	lowerResponse := strings.ToLower(response)
	triggers := []string{
		"not provided", "not mentioned", "not included", "not contain",
		"no information", "doesn't mention", "doesn't include",
		"not found in", "not available", "not specified",
	}

	for _, trigger := range triggers {
		if strings.Contains(lowerResponse, trigger) {
			return true
		}
	}
	return false
}

// handleSpecialCommands processes special interactive commands
func handleSpecialCommands(command string, sessionManager *SessionManager, currentSession *SessionData, rl *readline.Instance, renderMarkdown bool) bool {
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
		displaySessionWelcome(currentSession, currentSession.SearchEnabled, renderMarkdown)
		return true

	case "/info", "/i":
		if currentSession != nil {
			currentSession.PrintSessionInfo()
		} else {
			fmt.Fprintf(os.Stderr, "📄 No active session\n")
		}
		return true

	case "/exit", "/bye", "/quit":
		return true

	default:
		return false
	}
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
  - Sessions can be saved with custom names when exiting
  - To resume a session, use: hvsum --session <name>
  - List all sessions with: hvsum --list-sessions

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

// StartThinkingDots shows animated thinking indicator
func StartThinkingDots(message string) chan struct{} {
	stop := make(chan struct{})

	// If output is redirected (like in tests), don't show animation
	if isOutputRedirected() {
		go func() {
			<-stop // Just wait for stop signal
		}()
		return stop
	}

	go func() {
		dots := ""
		ticker := time.NewTicker(400 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				// Final clear of the line
				fmt.Fprintf(os.Stderr, "\r\033[K")
				return
			case <-ticker.C:
				if len(dots) >= 3 {
					dots = ""
				}
				dots += "."
				fmt.Fprintf(os.Stderr, "\r\033[K%s%s", message, dots)
			}
		}
	}()

	return stop
}
