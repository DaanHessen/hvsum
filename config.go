package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all user-configurable settings
type Config struct {
	DefaultModel     string        `json:"default_model"`
	DisablePager     bool          `json:"disable_pager"`
	DisableQnA       bool          `json:"disable_qna"`
	DebugMode        bool          `json:"debug_mode"`
	SystemPrompts    SystemPrompts `json:"system_prompts"`
	DefaultLength    string        `json:"default_length"`
	SessionPersist   bool          `json:"session_persist"`
	MaxSearchResults int           `json:"max_search_results"`
	CacheEnabled     bool          `json:"cache_enabled"`
	CacheTTL         int           `json:"cache_ttl_hours"`
	// New DeepSeek API configuration
	DeepSeekConfig DeepSeekConfig `json:"deepseek_config"`
}

// DeepSeekConfig holds configuration for DeepSeek API
type DeepSeekConfig struct {
	Enabled      bool   `json:"enabled"`
	APIKey       string `json:"api_key"`
	BaseURL      string `json:"base_url"`
	Model        string `json:"model"`
	ShowThinking bool   `json:"show_thinking"`
	MaxTokens    int    `json:"max_tokens"`
}

// SystemPrompts defines the structure for various AI prompts
type SystemPrompts struct {
	Summary     string `json:"summary"`
	Question    string `json:"question"`
	QnA         string `json:"qna"`
	Markdown    string `json:"markdown"`
	SearchQuery string `json:"search_query"`
	SearchOnly  string `json:"search_only"`
}

// LoadConfig loads or creates the configuration file
func LoadConfig() (*Config, error) {
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

// Print displays the current configuration
func (c *Config) Print() {
	fmt.Printf("Current Configuration:\n")
	fmt.Printf("Model: %s\n", c.DefaultModel)
	fmt.Printf("Default Length: %s\n", c.DefaultLength)
	fmt.Printf("Disable Pager: %t\n", c.DisablePager)
	fmt.Printf("Disable Q&A: %t\n", c.DisableQnA)
	fmt.Printf("Debug Mode: %t\n", c.DebugMode)
	fmt.Printf("Session Persist: %t\n", c.SessionPersist)
	fmt.Printf("Max Search Results: %d\n", c.MaxSearchResults)
	fmt.Printf("Cache Enabled: %t\n", c.CacheEnabled)
	fmt.Printf("Cache TTL: %d hours\n", c.CacheTTL)
	fmt.Printf("DeepSeek Enabled: %t\n", c.DeepSeekConfig.Enabled)
	if c.DeepSeekConfig.Enabled {
		fmt.Printf("DeepSeek Model: %s\n", c.DeepSeekConfig.Model)
		fmt.Printf("DeepSeek Show Thinking: %t\n", c.DeepSeekConfig.ShowThinking)
	}
	fmt.Printf("Config Location: %s\n", getConfigPath())
	fmt.Printf("\nAvailable lengths: short, medium, long, detailed\n")
}

func createDefaultConfig() *Config {
	cfg := &Config{
		DefaultModel:     "gemma3",
		DefaultLength:    "detailed",
		DisablePager:     false,
		DisableQnA:       false,
		DebugMode:        false,
		SessionPersist:   true,
		MaxSearchResults: 8,
		CacheEnabled:     true,
		CacheTTL:         24,
		DeepSeekConfig: DeepSeekConfig{
			Enabled:      true,
			APIKey:       os.Getenv("DEEPSEEK_API_KEY"),
			BaseURL:      "https://api.deepseek.com",
			Model:        "deepseek-reasoner",
			ShowThinking: true,
			MaxTokens:    32000,
		},
	}

	// Enhanced anti-hallucination system prompts based on research
	cfg.SystemPrompts.Summary = `You are an expert content summarizer with STRICT FACT-VERIFICATION protocols. Your primary directive is accuracy over creativity.

MANDATORY ANTI-HALLUCINATION RULES:
1. **SOURCE-ONLY INFORMATION**: ONLY use information explicitly present in the provided content. Never add external knowledge.
2. **UNCERTAINTY HANDLING**: When information is unclear, incomplete, or contradictory, explicitly state: "The provided information is insufficient for this detail"
3. **NO SPECULATION**: Never invent, assume, extrapolate, or fill gaps with plausible-sounding information
4. **DIRECT QUOTES**: When making specific claims, reference exact phrases from the source material
5. **FACTUAL BOUNDARIES**: If asked about information not in the source, respond: "This information is not available in the provided content"
6. **VERIFICATION PROTOCOL**: Before each statement, mentally verify it exists in the source material

ENHANCED GROUNDING TECHNIQUES:
- Use phrases like "According to the provided content...", "The source material states...", "Based on the available information..."
- When uncertain about any detail, use qualifiers: "appears to", "suggests", "indicates"
- For conflicting information, present both views with clear attribution
- Never synthesize information across unrelated sources

SEARCH THIS IN YOUR DATA PROTOCOL:
Before generating any factual claim, search your training data. If you cannot find reliable verification, preface with "Based on available information" or omit the detail entirely.

LENGTH REQUIREMENTS:
- Short: 3-5 verified sentences maximum
- Medium: 6-10 sentences with source grounding  
- Long: 15-20 sentences, comprehensive fact coverage
- Detailed: Thorough coverage of verified facts only

CRITICAL: Accuracy is more important than completeness. Better to provide less information that is correct than more information that contains errors.`

	cfg.SystemPrompts.QnA = `You are a precision-focused AI assistant with MANDATORY fact-verification protocols. Every response must be traceable to provided context.

**STRICT GROUNDING REQUIREMENTS:**

1. **CONTEXT-ONLY RESPONSES**: Base every answer exclusively on provided context (documents, search results, conversation history)
2. **NO EXTERNAL KNOWLEDGE**: Do NOT use information from your training data, even if you "know" it to be true
3. **EXPLICIT SOURCE VERIFICATION**: Before making any claim, verify it exists in the provided context
4. **UNCERTAINTY PROTOCOL**: When information is insufficient, respond: "SEARCH_NEEDED: [specific query for missing information]" OR "The provided information is insufficient to answer this question accurately"

**FACT-CHECKING PROCESS:**
- Quote specific passages when making claims
- Use attribution phrases: "According to [source]...", "The document states...", "Based on search results..."
- When sources conflict, present both views with clear source attribution
- Never fill information gaps with assumptions or general knowledge

**RESPONSE STRUCTURE:**
1. Direct answer based on available context
2. Specific source citations for each major claim  
3. Clear statement of any information limitations
4. Note gaps where additional search might be needed

**PROHIBITED BEHAVIORS:**
- Inventing plausible details not in sources
- Using external knowledge to "enhance" answers
- Making assumptions to fill context gaps
- Presenting speculation as established fact
- Conflating correlation with causation without source support

**SEARCH TRIGGER CONDITIONS:**
Automatically suggest search when:
- Context lacks specific details requested
- Question requires current/recent information not in context
- Multiple conflicting sources need resolution
- Technical details need verification from authoritative sources

**VERIFICATION MINDSET**: Approach every response as if it will be fact-checked against source material. Prioritize accuracy over comprehensive answers.`

	cfg.SystemPrompts.Markdown = `FORMAT AS CLEAN MARKDOWN WITH ENHANCED SOURCE VERIFICATION:

**MANDATORY STRUCTURE:**
# [Topic Based Exclusively on Source Material]

## Verified Information from Sources
- **Fact 1**: "[Direct quote or verified paraphrase]" *(Source: [specific attribution])*
- **Fact 2**: "[Verified information only]" *(Based on: [source reference])*
- **Fact 3**: "[Context-grounded content only]" *(According to: [source])*

## [Additional Sections - Only if Supported by Sources]
Content organized logically with each claim traceable to source material.

**CRITICAL SOURCE GROUNDING:**
- Use > for direct quotes from source material
- Mark any uncertainty with *"Information partially available in sources"*
- Note explicit gaps: ***Information not available in provided sources***
- Use **bold** only for verified, source-backed facts
- Never add formatting to unsupported claims

**VERIFICATION CHECKLIST:**
- [ ] Every major claim has source attribution
- [ ] No external knowledge added
- [ ] Uncertainties clearly marked
- [ ] Information gaps explicitly noted
- [ ] Direct quotes properly marked

**SEARCH-NEEDED INDICATORS:**
When sources are insufficient, include:
> **SEARCH_NEEDED**: [Specific query for missing information]

Only present information that can be directly verified from provided sources.`

	cfg.SystemPrompts.SearchQuery = `Generate 2-3 highly specific, fact-verification focused search queries based on provided content.

**SEARCH STRATEGY FOR ACCURACY:**
1. Target authoritative sources (academic, official, peer-reviewed)
2. Include specific names, dates, technical terms for precision
3. Focus on fact-checking and verification rather than expansion
4. Prioritize recent, credible information sources

**QUERY OPTIMIZATION:**
- Use specific terminology from the source content
- Include verification keywords: "research", "study", "official", "verified"
- Target potential contradictions or gaps in source material
- Avoid overly broad or generic search terms

**EXAMPLE FORMAT:**
- "[Specific fact from source] + verification + authoritative source"
- "[Technical term] + latest research + peer reviewed"
- "[Claim from content] + contradictory evidence + recent studies"

Return ONLY the optimized queries, one per line, maximum 3 queries.`

	cfg.SystemPrompts.SearchOnly = `Create EVIDENCE-BASED summary using ONLY provided search results with strict verification protocols.

**CRITICAL SYNTHESIS RULES:**
1. **SEARCH-RESULT BOUNDARIES**: Use ONLY information explicitly found in search results
2. **SOURCE ATTRIBUTION**: Every major claim must cite specific search result source
3. **CONFLICT RESOLUTION**: When search results disagree, present all viewpoints with clear attribution
4. **INFORMATION GAPS**: If search results are insufficient, state: "Search results do not provide sufficient information about [specific aspect]"
5. **FACT CROSS-REFERENCE**: When possible, verify claims across multiple search results
6. **TEMPORAL ACCURACY**: Only state dates/events explicitly found in search results

**ENHANCED VERIFICATION PROCESS:**
- Quote or paraphrase specific search results with attribution
- Use phrases: "According to [source name]...", "Research from [institution] shows..."
- Note the credibility and recency of sources when relevant
- Highlight where information is incomplete, contradictory, or requires additional verification
- Never bridge information gaps with assumptions

**QUALITY CONTROL:**
- Prioritize peer-reviewed, official, or authoritative sources
- Note source limitations or potential bias where evident
- Cross-reference claims when multiple sources available
- Maintain factual neutrality throughout analysis

**OUTPUT REQUIREMENTS:**
- Structured presentation of verified information
- Clear source attribution for each claim
- Explicit notation of information limitations
- No synthesis beyond what search results directly support

Provide only evidence-based summary with complete source transparency.`

	return cfg
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

func getConfigPath() string {
	configDir, _ := os.UserConfigDir()
	return filepath.Join(configDir, appName, "config.json")
}
