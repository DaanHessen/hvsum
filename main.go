package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/pflag"
)

const appName = "hvsum"
const version = "2.0.0"

var (
	length       = pflag.StringP("length", "l", "", "Summary length: short, medium, long, detailed")
	markdown     = pflag.BoolP("markdown", "M", false, "Format output as structured markdown")
	copyToClip   = pflag.BoolP("copy", "c", false, "Copy summary to clipboard")
	saveToFile   = pflag.StringP("save", "s", "", "Save summary to file")
	enableSearch = pflag.Bool("search", false, "Enable AI-powered web search to enhance summaries and answers")
	debugMode    = pflag.Bool("debug", false, "Enable debug mode with verbose logging")
	showHelp     = pflag.BoolP("help", "h", false, "Show help message")
	showConfig   = pflag.Bool("config", false, "Show current configuration")
	showVersion  = pflag.BoolP("version", "v", false, "Show version information")
)

func main() {
	pflag.Parse()

	if *showHelp {
		printUsage()
		return
	}

	if *showVersion {
		fmt.Printf("%s version %s\n", appName, version)
		return
	}

	config, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	if *debugMode {
		config.DebugMode = true
	}

	if *showConfig {
		config.Print()
		return
	}

	DebugLog(config, "Starting hvsum v%s", version)

	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "\nUnexpected error: %v\n", r)
		}
		DebugLog(config, "Stopping model '%s'", config.DefaultModel)
		fmt.Fprintf(os.Stderr, "\nStopping model '%s'...\n", config.DefaultModel)
		cmd := exec.Command("ollama", "stop", config.DefaultModel)
		cmd.Run()
	}()

	args := pflag.Args()
	if len(args) != 1 {
		printUsage()
		fmt.Fprintf(os.Stderr, "\nError: Please provide exactly one URL or search query as an argument.\n")
		os.Exit(1)
	}

	input := args[0]
	effectiveLength := getEffectiveLength(config)

	DebugLog(config, "Input: %s", input)
	DebugLog(config, "Length: %s", effectiveLength)
	DebugLog(config, "Search enabled: %v", *enableSearch)

	isURL := IsValidURL(input)
	DebugLog(config, "Input is URL: %v", isURL)

	var summary string

	if isURL {
		summary, err = ProcessURL(input, config, effectiveLength, *markdown, *enableSearch)
	} else {
		summary, err = ProcessSearchQuery(input, config, effectiveLength, *markdown)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	handleOutput(summary, config)

	if !config.DisableQnA {
		contextContent := input
		if isURL {
			contextContent = summary
		}
		StartInteractiveSession(summary, contextContent, config, *markdown, *enableSearch || !isURL)
	}
}

func getEffectiveLength(config *Config) string {
	if *length != "" {
		return *length
	}
	if config.DefaultLength != "" {
		return config.DefaultLength
	}
	return "detailed"
}

func handleOutput(summary string, config *Config) {
	DebugLog(config, "Generated summary length: %d characters", len(summary))

	if *copyToClip {
		if err := CopyToClipboard(summary); err != nil {
			fmt.Fprintf(os.Stderr, "Error copying to clipboard: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Summary copied to clipboard!\n")
		}
	}

	if *saveToFile != "" {
		if err := SaveToFile(*saveToFile, summary); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving to file: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Summary saved to %s\n", *saveToFile)
		}
	}

	if config.DisablePager {
		fmt.Fprintln(os.Stderr, "")
		RenderToConsole(summary, *markdown)
		if !config.DisableQnA {
			fmt.Fprintln(os.Stderr, "\n"+strings.Repeat("─", 60))
			fmt.Fprintln(os.Stderr, "Ask questions about the content above (type '/bye' or Ctrl+C to exit):")
		}
	} else {
		RenderWithPager(summary, *markdown)
		if !config.DisableQnA {
			fmt.Fprintln(os.Stderr, "Ask questions about the content above (type '/bye' or Ctrl+C to exit):")
		}
	}
}

func printUsage() {
	fmt.Printf(`%s v%s - Advanced Website Summarizer & Interactive Q&A

USAGE:
    %s [flags] <URL or search query>

DESCRIPTION:
    %s can either summarize a webpage from a URL or perform web searches and 
    summarize the results. After the summary, it enters an interactive mode where 
    you can ask follow-up questions. Type '/bye' or press Ctrl+C to exit.

FLAGS:
`, appName, version, appName, appName)
	pflag.PrintDefaults()

	fmt.Printf(`
EXAMPLES:
    %s https://example.com                           # Summarize webpage then Q&A
    %s -l short -M https://example.com               # Short, markdown summary then Q&A
    %s --search https://example.com                  # Enhanced summary with web search
    %s --search "artificial intelligence"            # Search-only mode (no URL)
    %s --debug "machine learning"                    # Debug mode with verbose logging
    %s -c https://example.com                        # Copy summary to clipboard
    %s -s summary.txt https://example.com            # Save summary to file
    %s --config                                      # Show current configuration

LENGTH OPTIONS:
    short    - 2 sentences exactly
    medium   - 4-6 sentences
    long     - 8-10 sentences
    detailed - 12-15 sentences [default]

SEARCH FEATURE:
    --search flag enables AI-powered web search to enhance summaries and Q&A responses.
    Real web search APIs are used to provide comprehensive and up-to-date information.

SEARCH-ONLY MODE:
    If you provide a search query instead of a URL, hvsum will perform web searches
    and create a summary based on the search results.

INTERACTIVE COMMANDS:
    /bye, /exit, /quit - Exit the interactive Q&A session
    Ctrl+C or Ctrl+D   - Also exit the session

DEBUG MODE:
    --debug flag enables verbose logging showing search queries, results, and
    internal operations.

CONFIG:
    Configuration is stored at: ~/.config/hvsum/config.json
    Edit this file to customize the AI model and system prompts.
`, appName, appName, appName, appName, appName, appName, appName, appName)
}
