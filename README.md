# hvsum - Website Summarizer

A fast, intelligent terminal tool for summarizing web content using local AI models via Ollama.

## Features

- **Precise Length Control**: Exact sentence counts enforced (2, 4-6, 8-10, or 12-15 sentences)
- **Beautiful Markdown Output**: Clean, structured markdown with mandatory headers and sections
- **Question Mode**: Ask specific questions about webpage content
- **Smart Streaming**: Real-time output for text mode, clean rendering for markdown
- **Configurable**: Customize AI models and system prompts via config file
- **Fast & Local**: Uses Ollama for local AI processing - no API keys needed

## Installation

1. **Install Ollama** (if not already installed):
   ```bash
   curl -fsSL https://ollama.ai/install.sh | sh
   ```

2. **Pull an AI model**:
   ```bash
   ollama pull llama3.2:latest
   ```

3. **Install hvsum**:
   ```bash
   go install github.com/your-username/hvsum@latest
   ```

## Usage

### Basic Summarization
```bash
# Default medium-length summary
hvsum https://example.com

# Short, concise summary
hvsum -l short https://news-article.com

# Detailed analysis
hvsum -l detailed https://research-paper.com
```

### Markdown Output
```bash
# Beautiful formatted output in terminal with proper structure
hvsum -m https://example.com

# Short markdown summary with headers and sections
hvsum -l short -m https://blog-post.com
```

**Streaming**: Content streams in real-time for text mode. For markdown mode, output is rendered cleanly without streaming for better readability.

### Question Mode
```bash
# Ask specific questions about content
hvsum "What are the main benefits?" https://product-page.com

# Detailed question with markdown formatting
hvsum -l long -m "How does this technology work?" https://tech-article.com
```

### Configuration
```bash
# View current settings
hvsum -c

# Show help
hvsum -h
```

## Length Options

| Length | Output | Best For |
|--------|--------|----------|
| `short` | Exactly 2 sentences | Quick overview, key points only |
| `medium` | 4-6 sentences | Balanced summary with main details |
| `long` | 8-10 sentences | Comprehensive coverage |
| `detailed` | 12-15 sentences | In-depth analysis with examples [**default**] |

**Note**: Length constraints are strictly enforced. The model counts sentences and stops exactly at the specified limit.

## Configuration

Configuration is stored at `~/.config/hvsum/config.json`. Edit this file to:

- Change the default AI model
- Customize system prompts
- Set default summary length

### Example Config
```json
{
  "default_model": "llama3.2:latest",
  "default_length": "medium",
  "system_prompts": {
    "summary": "Custom summarization instructions...",
    "question": "Custom question-answering instructions...",
    "markdown": "Custom markdown formatting rules...",
    "search": "Custom search synthesis instructions..."
  }
}
```

## Examples

### News Article Summary
```bash
hvsum -m https://news.example.com/article
```
Output:
```markdown
# Breaking: New AI Breakthrough Announced

## Key Developments
- **Performance**: 40% improvement over previous models
- **Efficiency**: Reduced computational requirements
- **Applications**: Healthcare, education, and research

## Impact
The breakthrough promises to make AI more accessible...
```

### Technical Documentation
```bash
hvsum -l detailed "How do I implement authentication?" https://docs.api.com
```

### Quick Research
```bash
hvsum -l short https://wikipedia.org/wiki/Machine_Learning
```

## Tips

1. **Use markdown mode** (`-m`) for better readability in terminal
2. **Set default length** in config to match your typical needs
3. **Question mode** works great for extracting specific information
4. **Longer content** may require the `detailed` length option for full coverage

## Requirements

- **Ollama**: Local AI runtime
- **Internet connection**: For fetching web content
- **Go 1.19+**: For building from source

## Troubleshooting

### Model Not Found
```bash
# Pull the model first
ollama pull llama3.2:latest
```

### Config Issues
```bash
# View current config location
hvsum -c

# Delete config to regenerate defaults
rm ~/.config/hvsum/config.json
```

### Network Issues
- Ensure the URL is accessible
- Check for redirects or paywalls
- Some sites may block automated access

## Future Features

- **Search Mode**: Multi-source web search and synthesis (coming soon)
- **PDF Support**: Direct PDF summarization
- **Batch Processing**: Multiple URLs at once
- **Export Options**: Save summaries to files

---

Built with ❤️ for developers who need quick, intelligent web content analysis. 