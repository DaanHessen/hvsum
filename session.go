package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ollama/ollama/api"
)

// SessionData represents a saved interactive session
type SessionData struct {
	ID             string        `json:"id"`
	Title          string        `json:"title"`
	URL            string        `json:"url,omitempty"`
	Query          string        `json:"query,omitempty"`
	InitialSummary string        `json:"initial_summary"`
	ContextContent string        `json:"context_content"`
	Messages       []api.Message `json:"messages"`
	CreatedAt      time.Time     `json:"created_at"`
	LastAccessedAt time.Time     `json:"last_accessed_at"`
	SearchEnabled  bool          `json:"search_enabled"`
}

// SessionManager handles session persistence and management
type SessionManager struct {
	sessionsDir string
	config      *Config
}

// NewSessionManager creates a new session manager
func NewSessionManager(config *Config) *SessionManager {
	configDir, _ := os.UserConfigDir()
	sessionsDir := filepath.Join(configDir, appName, "sessions")
	os.MkdirAll(sessionsDir, 0755)

	return &SessionManager{
		sessionsDir: sessionsDir,
		config:      config,
	}
}

// CreateSession creates a new session
func (sm *SessionManager) CreateSession(summary, contextContent, title string, enableSearch bool) (*SessionData, error) {
	if !sm.config.SessionPersist {
		return nil, nil // Sessions disabled
	}

	sessionID := fmt.Sprintf("session_%d", time.Now().Unix())

	session := &SessionData{
		ID:             sessionID,
		Title:          title,
		InitialSummary: summary,
		ContextContent: contextContent,
		Messages: []api.Message{
			{
				Role:    "system",
				Content: sm.config.SystemPrompts.QnA,
			},
			{
				Role:    "assistant",
				Content: "I'm ready to answer questions about: " + title,
			},
		},
		CreatedAt:      time.Now(),
		LastAccessedAt: time.Now(),
		SearchEnabled:  enableSearch,
	}

	if err := sm.SaveSession(session); err != nil {
		return nil, err
	}

	DebugLog(sm.config, "Created new session: %s", sessionID)
	return session, nil
}

// SaveSession saves a session to disk
func (sm *SessionManager) SaveSession(session *SessionData) error {
	if !sm.config.SessionPersist || session == nil {
		return nil
	}

	session.LastAccessedAt = time.Now()

	sessionPath := filepath.Join(sm.sessionsDir, session.ID+".json")
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(sessionPath, data, 0644)
}

// LoadSession loads a session from disk
func (sm *SessionManager) LoadSession(sessionID string) (*SessionData, error) {
	if !sm.config.SessionPersist {
		return nil, fmt.Errorf("sessions are disabled")
	}

	sessionPath := filepath.Join(sm.sessionsDir, sessionID+".json")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return nil, err
	}

	var session SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	session.LastAccessedAt = time.Now()
	sm.SaveSession(&session) // Update access time

	return &session, nil
}

// ListSessions returns all available sessions
func (sm *SessionManager) ListSessions() ([]*SessionData, error) {
	if !sm.config.SessionPersist {
		return nil, nil
	}

	entries, err := os.ReadDir(sm.sessionsDir)
	if err != nil {
		return nil, err
	}

	var sessions []*SessionData
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			sessionID := entry.Name()[:len(entry.Name())-5] // Remove .json
			session, err := sm.LoadSession(sessionID)
			if err != nil {
				continue // Skip corrupted sessions
			}
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// DeleteSession removes a session
func (sm *SessionManager) DeleteSession(sessionID string) error {
	sessionPath := filepath.Join(sm.sessionsDir, sessionID+".json")
	return os.Remove(sessionPath)
}

// ClearAll removes all saved sessions.
func (sm *SessionManager) ClearAll() error {
	dir, err := os.ReadDir(sm.sessionsDir)
	if err != nil {
		return fmt.Errorf("failed to read sessions directory: %w", err)
	}
	for _, d := range dir {
		os.RemoveAll(filepath.Join(sm.sessionsDir, d.Name()))
	}
	DebugLog(sm.config, "Cleared all sessions.")
	return nil
}

// CleanOldSessions removes sessions older than specified days
func (sm *SessionManager) CleanOldSessions(maxAgeDays int) error {
	sessions, err := sm.ListSessions()
	if err != nil {
		return err
	}

	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
	cleaned := 0

	for _, session := range sessions {
		if session.LastAccessedAt.Before(cutoff) {
			if err := sm.DeleteSession(session.ID); err == nil {
				cleaned++
			}
		}
	}

	DebugLog(sm.config, "Cleaned %d old sessions", cleaned)
	return nil
}

// FindRecentSessions returns recently accessed sessions
func (sm *SessionManager) FindRecentSessions(limit int) ([]*SessionData, error) {
	sessions, err := sm.ListSessions()
	if err != nil {
		return nil, err
	}

	// Sort by last accessed time (newest first)
	for i := 0; i < len(sessions)-1; i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[i].LastAccessedAt.Before(sessions[j].LastAccessedAt) {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}

	if len(sessions) > limit {
		sessions = sessions[:limit]
	}

	return sessions, nil
}

// AddMessage adds a message to the session
func (sm *SessionManager) AddMessage(session *SessionData, role, content string) {
	if session == nil {
		return
	}

	session.Messages = append(session.Messages, api.Message{
		Role:    role,
		Content: content,
	})

	// Keep only last 20 messages to prevent sessions from growing too large
	if len(session.Messages) > 22 { // 2 system + 20 conversation
		// Keep system messages and last 18 conversation messages
		systemMsgs := session.Messages[:2]
		recentMsgs := session.Messages[len(session.Messages)-18:]
		session.Messages = append(systemMsgs, recentMsgs...)
	}
}

// GetTitle generates or returns session title
func (session *SessionData) GetTitle() string {
	if session.Title != "" {
		return session.Title
	}

	if session.URL != "" {
		return fmt.Sprintf("Web: %s", session.URL)
	}

	if session.Query != "" {
		return fmt.Sprintf("Search: %s", session.Query)
	}

	return fmt.Sprintf("Session %s", session.ID)
}

// GetAge returns the age of the session
func (session *SessionData) GetAge() string {
	age := time.Since(session.CreatedAt)

	if age < time.Hour {
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	} else if age < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(age.Hours()))
	} else {
		return fmt.Sprintf("%dd ago", int(age.Hours()/24))
	}
}

// PrintSessionInfo prints formatted session information
func (session *SessionData) PrintSessionInfo() {
	fmt.Printf("📄 %s\n", session.GetTitle())
	fmt.Printf("   ID: %s | Created: %s | Messages: %d\n",
		session.ID, session.GetAge(), len(session.Messages)-2) // -2 for system messages
	if session.SearchEnabled {
		fmt.Printf("   🔍 Web search enabled\n")
	}
}
