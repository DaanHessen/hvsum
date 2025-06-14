# hvsum v2.0.0 - Advanced Website Summarizer & Interactive Q&A

A completely rewritten, high-performance web summarization tool with real search API integration, parallel processing, and interactive Q&A capabilities.

## 🚀 What's New in v2.0

- **Completely rewritten** with modular Go architecture
- **Real search APIs** - No more simulations! Uses DuckDuckGo + SerpAPI
- **Parallel processing** - 3-5x faster search and content extraction
- **Optimized AI prompts** - Better quality summaries with gemma2 default
- **Enhanced web scraping** - Better content extraction with go-readability
- **Improved UX** - Better pager experience, debug mode, and error handling

## 🏗️ Architecture

```
hvsum/
├── main.go         # Entry point and CLI handling
├── config.go       # Configuration management
├── search.go       # Real search API integration (DuckDuckGo + SerpAPI)
├── summarize.go    # AI summarization with Ollama
├── web.go          # Web content extraction
├── interactive.go  # Q&A session management
├── render.go       # Content display and paging
├── utils.go        # Utility functions
└── go.mod          # Dependencies
```

## 📦 Installation

### Prerequisites

1. **Go 1.19+** installed
2. **Ollama** installed and running
3. **gemma2 model** pulled: `ollama pull gemma2:latest`

### Quick Install

```bash
# Clone and build
git clone <your-repo> hvsum
cd hvsum
go mod tidy
go build -o hvsum .

# Make it globally available
sudo cp hvsum /usr/local/bin/
```

### Dependencies

The tool uses these Go modules:
- `github.com/spf13/pflag` - CLI flags
- `github.com/ollama/ollama/api` - AI integration
- `github.com/go-shiori/go-readability` - Content extraction
- `github.com/charmbracelet/glamour` - Markdown rendering
- `github.com/chzyer/readline` - Terminal input
- `github.com/atotto/clipboard` - Clipboard operations

## 🔍 Search API Setup

### Free Option: DuckDuckGo (Default)
- **Cost**: Completely FREE
- **Setup**: No configuration needed
- **Limitations**: Instant answers only, limited results
- **Speed**: Fast for basic queries

### Premium Option: SerpAPI (Recommended)
- **Cost**: $50/month for 5,000 searches (or free tier: 100 searches/month)
- **Setup**: 
  ```bash
  export SERPAPI_KEY="your_api_key_here"
  # Add to ~/.bashrc or ~/.zshrc for persistence
  ```
- **Benefits**: Full Google search results, unlimited queries, higher quality
- **Speed**: Very fast with comprehensive results

### Get SerpAPI Key:
1. Go to [serpapi.com](https://serpapi.com)
2. Sign up for free account (100 searches/month)
3. Get your API key from dashboard
4. Set environment variable: `export SERPAPI_KEY="your_key"`

## 🎯 Usage

### Basic Usage
```bash
# Summarize a webpage
hvsum https://example.com

# Search-only mode (no URL needed)
hvsum "artificial intelligence trends 2024"

# Enhanced with web search
hvsum --search https://example.com
hvsum --search "machine learning"
```

### Advanced Options
```bash
# Length control
hvsum -l short https://example.com     # 2 sentences
hvsum -l medium https://example.com    # 4-6 sentences  
hvsum -l long https://example.com      # 8-10 sentences
hvsum -l detailed https://example.com  # 12-15 sentences (default)

# Output formats
hvsum -M https://example.com           # Markdown format
hvsum -c https://example.com           # Copy to clipboard
hvsum -s summary.txt https://example.com  # Save to file

# Debug mode
hvsum --debug "python programming"     # See all operations
```

### Interactive Commands
After the summary, you enter Q&A mode:
- Ask any question about the content
- Type `/bye`, `/exit`, or `/quit` to exit
- Press `Ctrl+C` or `Ctrl+D` to exit
- Questions are enhanced with real-time web search (if `--search` enabled)

## ⚙️ Configuration

Configuration is stored at `~/.config/hvsum/config.json`:

```json
{
  "default_model": "gemma2:latest",
  "default_length": "detailed", 
  "disable_pager": false,
  "disable_qna": false,
  "debug_mode": false,
  "system_prompts": {
    "summary": "...",
    "qna": "...",
    "markdown": "...",
    "search_query": "...",
    "search_only": "..."
  }
}
```

### View/Edit Config
```bash
hvsum --config                    # View current config
# Edit: ~/.config/hvsum/config.json
```

## 🚄 Performance Optimizations

### Speed Improvements
- **Parallel search processing** - Multiple search engines simultaneously
- **Concurrent content extraction** - Fetch multiple URLs at once
- **Optimized HTTP clients** - Proper timeouts and connection pooling
- **Smart caching** - Deduplicated search results
- **Efficient AI calls** - Optimized prompts and streaming

### Quality Improvements
- **Real search APIs** - No more placeholder results
- **Better content extraction** - go-readability for clean text
- **Enhanced prompts** - Tuned for gemma2 model
- **Length enforcement** - Precise sentence counting
- **Context preservation** - Better conversation memory

## 🔧 Troubleshooting

### Common Issues

**Model not found:**
```bash
ollama pull gemma2:latest
```

**Search not working:**
```bash
# Check if SerpAPI key is set
echo $SERPAPI_KEY

# Test DuckDuckGo (should always work)
hvsum --debug "test query"
```

**Pager issues:**
```bash
# Disable pager if needed
hvsum --config
# Set "disable_pager": true
```

**Permission errors:**
```bash
# Make sure hvsum is executable
chmod +x hvsum
```

## 📊 Cost Analysis

### Free Tier (DuckDuckGo only)
- **Cost**: $0
- **Searches**: Unlimited
- **Quality**: Basic instant answers
- **Best for**: Simple queries, definitions, basic facts

### Premium Tier (SerpAPI + DuckDuckGo)
- **Cost**: $50/month (or $0 for 100 searches/month)
- **Searches**: 5,000/month (paid) or 100/month (free)
- **Quality**: Full Google search results
- **Best for**: Research, comprehensive summaries, professional use

### Recommendation
- **Start with free tier** to test functionality
- **Upgrade to SerpAPI free tier** (100 searches/month) for better results
- **Consider paid tier** if you need >100 searches/month

## 🎨 Examples

### Website Summarization
```bash
❯ hvsum https://archlinux.org
🌐 Fetching content from: https://archlinux.org
🤖 Generating summary with gemma2:latest...

# Arch Linux

## Overview
Arch Linux is a lightweight and flexible Linux distribution...
[Summary appears in pager]

Ask questions about the content above (type '/bye' or Ctrl+C to exit):
> What are the main principles of Arch Linux?
🔍 Searching for additional information...
🚀 Performing parallel searches for your question...

Arch Linux follows the KISS principle (Keep It Simple, Stupid)...
```

### Search-Only Mode
```bash
❯ hvsum --search "quantum computing 2024"
🔍 Performing web search for: quantum computing 2024
🚀 Performing parallel web searches...
📄 Extracting content from top results...
🤖 Generating comprehensive summary with gemma2:latest...

# Quantum Computing Developments in 2024

## Overview
Quantum computing has seen significant breakthroughs in 2024...
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## 📄 License

MIT License - see LICENSE file for details.

## 🙏 Acknowledgments

- **Ollama** for local AI inference
- **SerpAPI** for search functionality  
- **DuckDuckGo** for free search API
- **Go community** for excellent libraries 