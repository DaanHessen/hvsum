package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ollama/ollama/api"
	"github.com/spf13/pflag"
)

const appName = "hvsum"
const version = "0.2.0-beta"

func main() {
	// Define flags
	var (
		showVersion     bool
		enableSearch    bool
		showHelp        bool
		useMarkdown     bool
		disablePager    bool
		disableQnA      bool
		generateOutline bool
		copyToClipboard bool
		cleanCache      bool
		listSessions    bool
		cleanSessions   bool
		debugMode       bool
		disableCache    bool
		length          string
		sessionName     string
		saveToFile      string
		configPath      string
	)

	pflag.BoolVarP(&showVersion, "version", "v", false, "Show application version")
	pflag.BoolVarP(&enableSearch, "search", "s", false, "Enhance summary with web search")
	pflag.BoolVarP(&showHelp, "help", "h", false, "Show help message")
	pflag.BoolVarP(&useMarkdown, "markdown", "m", false, "Render output as markdown")
	pflag.BoolVar(&disablePager, "no-pager", false, "Disable pager for output")
	pflag.BoolVar(&disableQnA, "no-qna", false, "Disable interactive Q&A session")
	pflag.BoolVarP(&generateOutline, "outline", "o", false, "Generate a structured outline from the summary")
	pflag.BoolVarP(&copyToClipboard, "copy", "c", false, "Copy the summary to the clipboard")
	pflag.BoolVar(&cleanCache, "clean-cache", false, "Clean all cached data")
	pflag.BoolVar(&listSessions, "list-sessions", false, "List recent interactive sessions")
	pflag.BoolVar(&cleanSessions, "clean-sessions", false, "Clean all saved sessions")
	pflag.BoolVar(&debugMode, "debug", false, "Enable debug logging")
	pflag.BoolVar(&disableCache, "no-cache", false, "Disable caching for this session")
	pflag.StringVarP(&length, "length", "l", "detailed", "Set summary length (short, medium, long, detailed)")
	pflag.StringVar(&sessionName, "session", "", "Resume a saved session by name")
	pflag.StringVarP(&saveToFile, "write", "w", "", "Save the summary to a file (.md or .txt)")
	pflag.StringVar(&configPath, "config", "", "Path to a custom config file")

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <URL or search query>\n\n", appName)
		fmt.Fprintf(os.Stderr, "A powerful CLI tool to summarize web pages and search queries.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  %s https://example.com                     # Summarize URL only\n", appName)
		fmt.Fprintf(os.Stderr, "  %s -s https://example.com                 # Summarize URL + web search\n", appName)
		fmt.Fprintf(os.Stderr, "  %s -m -l short https://wikipedia.org/...  # Markdown, short summary\n", appName)
		fmt.Fprintf(os.Stderr, "  %s -s 'latest AI research'                # Search query with enhancement\n", appName)
		fmt.Fprintf(os.Stderr, "  %s --session mysession                    # Resume saved session\n", appName)
		fmt.Fprintf(os.Stderr, "  %s --list-sessions                        # List saved sessions\n", appName)
		fmt.Fprintf(os.Stderr, "  %s --no-cache https://example.com         # Disable caching\n\n", appName)
		fmt.Fprintf(os.Stderr, "Flags:\n")
		pflag.PrintDefaults()
	}

	pflag.Parse()

	if showVersion {
		fmt.Printf("%s version %s\n", appName, version)
		return
	}

	if showHelp {
		pflag.Usage()
		return
	}

	config, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Enable debug mode if requested
	if debugMode {
		config.DebugMode = true
	}

	// Disable cache if requested
	if disableCache {
		config.CacheEnabled = false
	}

	sessionManager := NewSessionManager(config)

	// Handle standalone flags that don't require an input arg
	if handled := handleStandaloneFlags(config, sessionManager, cleanCache, listSessions, cleanSessions); handled {
		return
	}

	args := pflag.Args()

	// Handle session resumption
	if sessionName != "" {
		resumeSession(sessionName, sessionManager, config, useMarkdown, enableSearch)
		return
	}

	// No arguments provided - show usage
	if len(args) == 0 {
		pflag.Usage()
		return
	}

	input := strings.Join(args, " ")

	// Process the input (URL or search query)
	summary, content, title, err := processInput(input, config, length, useMarkdown, enableSearch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Generate outline if requested
	if generateOutline {
		outline, outlineErr := GenerateOutline(summary, config, useMarkdown, "")
		if outlineErr != nil {
			fmt.Fprintf(os.Stderr, "Error generating outline: %v\n", outlineErr)
		} else {
			summary = outline
		}
	}

	// Display results
	RenderOutput(summary, useMarkdown, config.DisablePager || disablePager)

	// Handle file saving
	if saveToFile != "" {
		if err := SaveToFile(saveToFile, summary); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving to file: %v\n", err)
		} else {
			fmt.Printf("Summary saved to %s\n", saveToFile)
		}
	}

	// Handle clipboard copying
	if copyToClipboard {
		if err := CopyToClipboard(summary); err != nil {
			fmt.Fprintf(os.Stderr, "Error copying to clipboard: %v\n", err)
		} else {
			fmt.Println("Summary copied to clipboard.")
		}
	}

	// Start interactive session if enabled
	if !disableQnA {
		session := &SessionData{
			ID:             fmt.Sprintf("session_%d", time.Now().Unix()),
			Title:          title,
			URL:            extractURLFromInput(input),
			Query:          extractQueryFromInput(input),
			InitialSummary: summary,
			ContextContent: content,
			SearchEnabled:  enableSearch,
			CreatedAt:      time.Now(),
			LastAccessedAt: time.Now(),
			Messages: []api.Message{
				{Role: "system", Content: config.SystemPrompts.QnA},
				{Role: "assistant", Content: "I'm ready to answer questions about: " + title},
			},
		}
		StartInteractiveSession(session, config, useMarkdown, enableSearch)
	}
}

// handleStandaloneFlags processes flags that can be run without other arguments
func handleStandaloneFlags(config *Config, sessionManager *SessionManager, cleanCache, listSessions, cleanSessions bool) bool {
	if cleanCache {
		cacheManager := NewCacheManager(config)
		cacheManager.Clear()
		fmt.Println("✅ Cache cleared successfully")
		return true
	}

	if listSessions {
		sessions, err := sessionManager.ListSessions()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing sessions: %v\n", err)
			return true
		}

		if len(sessions) == 0 {
			fmt.Println("No saved sessions found.")
			return true
		}

		fmt.Println("📜 Saved Sessions:")
		for _, session := range sessions {
			fmt.Printf("  %s - %s (%s)\n", session.ID, session.GetTitle(), session.GetAge())
		}
		return true
	}

	if cleanSessions {
		err := sessionManager.ClearAll()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error cleaning sessions: %v\n", err)
		} else {
			fmt.Println("✅ All sessions cleared successfully")
		}
		return true
	}

	return false
}

// resumeSession resumes a saved session
func resumeSession(sessionName string, sessionManager *SessionManager, config *Config, useMarkdown, enableSearch bool) {
	session, err := sessionManager.LoadSession(sessionName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading session '%s': %v\n", sessionName, err)
		fmt.Fprintf(os.Stderr, "Use --list-sessions to see available sessions.\n")
		os.Exit(1)
	}

	fmt.Printf("📂 Resuming session: %s\n", session.GetTitle())
	StartInteractiveSession(session, config, useMarkdown, session.SearchEnabled)
}

// processInput handles both URLs and search queries with the new two-stage approach
func processInput(input string, config *Config, length string, useMarkdown, enableSearch bool) (summary, content, title string, err error) {
	var sessionID = fmt.Sprintf("temp_%d", time.Now().Unix())

	if IsValidURL(input) {
		summary, content, title, err = ProcessURL(input, config, length, useMarkdown, enableSearch, sessionID)
	} else {
		summary, content, title, err = ProcessSearchQuery(input, config, length, useMarkdown, sessionID)
	}

	return summary, content, title, err
}

// RenderOutput determines whether to use a pager or print to console.
func RenderOutput(content string, useMarkdown bool, forceNoPager bool) {
	if !forceNoPager {
		RenderWithPager(content, useMarkdown)
	} else {
		RenderToConsole(content, useMarkdown)
	}
}

// extractURLFromInput extracts URL if input is a URL
func extractURLFromInput(input string) string {
	if IsValidURL(input) {
		return input
	}
	return ""
}

// extractQueryFromInput extracts query if input is not a URL
func extractQueryFromInput(input string) string {
	if !IsValidURL(input) {
		return input
	}
	return ""
}
