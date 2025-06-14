package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

const appName = "hvsum"
const version = "0.2.0-beta" // Setting a version

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
		length          string
		interactiveMode string
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
	pflag.StringVarP(&length, "length", "l", "medium", "Set summary length (short, medium, long, detailed)")
	pflag.StringVarP(&interactiveMode, "interactive", "i", "", "Start an interactive session with a file or session ID")
	pflag.StringVarP(&saveToFile, "save", "S", "", "Save the summary to a file")
	pflag.StringVar(&configPath, "config", "", "Path to a custom config file")

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <URL or search query>\n\n", appName)
		fmt.Fprintf(os.Stderr, "A powerful CLI tool to summarize web pages and search queries.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  %s https://example.com\n", appName)
		fmt.Fprintf(os.Stderr, "  %s -s 'latest AI research'\n", appName)
		fmt.Fprintf(os.Stderr, "  %s -m -l long https://en.wikipedia.org/wiki/Go_(programming_language)\n\n", appName)
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

	sessionManager := NewSessionManager(config)

	// Handle session-related flags first
	if interactiveMode != "" {
		session, err := sessionManager.LoadSession(interactiveMode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading session %s: %v\n", interactiveMode, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Resuming session: %s\n", session.GetTitle())
		StartInteractiveSession(session.InitialSummary, session.ContextContent, config, useMarkdown, session.SearchEnabled)
		return
	}

	if cleanCache {
		cacheManager := NewCacheManager(config)
		if err := cacheManager.Clear(); err != nil {
			fmt.Fprintf(os.Stderr, "Error cleaning cache: %v\n", err)
		} else {
			fmt.Println("Cache cleaned successfully.")
		}
		return
	}

	if listSessions {
		sessions, err := sessionManager.FindRecentSessions(10)
		if err != nil || len(sessions) == 0 {
			fmt.Println("No recent sessions found.")
			return
		}

		fmt.Println("Recent Sessions:")
		for _, session := range sessions {
			session.PrintSessionInfo()
		}
		fmt.Println("\nUse `hvsum -i <session_id>` to resume.")
		return
	}

	if cleanSessions {
		if err := sessionManager.ClearAll(); err != nil {
			fmt.Fprintf(os.Stderr, "Error cleaning sessions: %v\n", err)
		} else {
			fmt.Println("All sessions cleaned successfully.")
		}
		return
	}

	args := pflag.Args()
	if len(args) == 0 {
		pflag.Usage()
		return
	}
	input := strings.Join(args, " ")

	var summary, content string

	if IsValidURL(input) {
		summary, err = ProcessURL(input, config, length, useMarkdown, enableSearch)
		content = summary // For Q&A context
	} else {
		summary, err = ProcessSearchQuery(input, config, length, useMarkdown)
		content = summary // For Q&A context
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if generateOutline {
		outline, err := GenerateOutline(summary, config, useMarkdown)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating outline: %v\n", err)
		} else {
			summary = outline
		}
	}

	if useMarkdown || !disablePager {
		RenderWithPager(summary, useMarkdown)
	} else {
		RenderToConsole(summary, useMarkdown)
	}

	if copyToClipboard {
		if err := CopyToClipboard(summary); err != nil {
			fmt.Fprintf(os.Stderr, "Error copying to clipboard: %v\n", err)
		} else {
			fmt.Println("Summary copied to clipboard.")
		}
	}

	if saveToFile != "" {
		if err := SaveToFile(saveToFile, summary); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving to file: %v\n", err)
		} else {
			fmt.Printf("Summary saved to %s\n", saveToFile)
		}
	}

	if !disableQnA {
		StartInteractiveSession(summary, content, config, useMarkdown, enableSearch)
	}
}
