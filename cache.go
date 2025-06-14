package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CacheEntry represents a cached result
type CacheEntry struct {
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
	TTL       int         `json:"ttl_hours"`
}

// CacheManager handles caching operations
type CacheManager struct {
	cacheDir string
	config   *Config
}

// NewCacheManager creates a new cache manager
func NewCacheManager(config *Config) *CacheManager {
	configDir, _ := os.UserConfigDir()
	cacheDir := filepath.Join(configDir, appName, "cache")
	os.MkdirAll(cacheDir, 0755)

	return &CacheManager{
		cacheDir: cacheDir,
		config:   config,
	}
}

// GetCacheKey generates a cache key from input data
func (cm *CacheManager) GetCacheKey(data string) string {
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// Get retrieves cached data if valid
func (cm *CacheManager) Get(key string, target interface{}) bool {
	if !cm.config.CacheEnabled {
		return false
	}

	filePath := filepath.Join(cm.cacheDir, key+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return false
	}

	// Check if cache is expired
	if time.Since(entry.Timestamp).Hours() > float64(entry.TTL) {
		os.Remove(filePath) // Clean up expired cache
		return false
	}

	// Convert entry data back to target type
	entryBytes, err := json.Marshal(entry.Data)
	if err != nil {
		return false
	}

	return json.Unmarshal(entryBytes, target) == nil
}

// Set stores data in cache
func (cm *CacheManager) Set(key string, data interface{}) error {
	if !cm.config.CacheEnabled {
		return nil
	}

	entry := CacheEntry{
		Data:      data,
		Timestamp: time.Now(),
		TTL:       cm.config.CacheTTL,
	}

	entryBytes, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	filePath := filepath.Join(cm.cacheDir, key+".json")
	return os.WriteFile(filePath, entryBytes, 0644)
}

// CleanExpired removes expired cache entries
func (cm *CacheManager) CleanExpired() error {
	if !cm.config.CacheEnabled {
		return nil
	}

	entries, err := os.ReadDir(cm.cacheDir)
	if err != nil {
		return err
	}

	cleaned := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			filePath := filepath.Join(cm.cacheDir, entry.Name())

			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			var cacheEntry CacheEntry
			if err := json.Unmarshal(data, &cacheEntry); err != nil {
				continue
			}

			if time.Since(cacheEntry.Timestamp).Hours() > float64(cacheEntry.TTL) {
				os.Remove(filePath)
				cleaned++
			}
		}
	}

	DebugLog(cm.config, "Cleaned %d expired cache entries", cleaned)
	return nil
}

// Clear removes all cache entries
func (cm *CacheManager) Clear() error {
	entries, err := os.ReadDir(cm.cacheDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			os.Remove(filepath.Join(cm.cacheDir, entry.Name()))
		}
	}

	return nil
}
