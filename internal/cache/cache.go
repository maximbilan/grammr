package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// CacheDirPerm is the permission for the cache directory (0700 = rwx------)
	// Restrictive permissions protect the directory from being accessed by other users
	CacheDirPerm os.FileMode = 0700
	// CacheFilePerm is the permission for cache files (0600 = rw-------)
	// Restrictive permissions protect cached content from being read by other users
	CacheFilePerm os.FileMode = 0600
)

type Cache struct {
	dir string
	ttl time.Duration
}

type CacheEntry struct {
	Hash      string `json:"hash"`
	Original  string `json:"original"`
	Corrected string `json:"corrected"`
	Timestamp int64  `json:"timestamp"`
}

func New(ttlDays int) (*Cache, error) {
	if ttlDays < 0 {
		return nil, fmt.Errorf("cache TTL days must be non-negative, got %d", ttlDays)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	cacheDir := filepath.Join(home, ".grammr", "cache")
	if err := os.MkdirAll(cacheDir, CacheDirPerm); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Cache{
		dir: cacheDir,
		ttl: time.Duration(ttlDays) * 24 * time.Hour,
	}, nil
}

func (c *Cache) Hash(text string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(text)))
}

func (c *Cache) Get(hash string) string {
	if hash == "" {
		return ""
	}

	// Validate hash to prevent path traversal attacks
	if !isValidHash(hash) {
		return ""
	}

	path := filepath.Join(c.dir, hash+".json")
	// Additional safety check: ensure the resolved path is within cache directory
	// Use filepath.Clean to resolve any path traversal attempts
	cleanPath := filepath.Clean(path)
	if !strings.HasPrefix(cleanPath, c.dir+string(filepath.Separator)) && cleanPath != c.dir {
		return ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return ""
	}

	// Check if expired
	if time.Since(time.Unix(entry.Timestamp, 0)) > c.ttl {
		_ = os.Remove(path) // Ignore error on removal
		return ""
	}

	return entry.Corrected
}

// isValidHash validates that the hash is a valid SHA256 hex string (64 characters)
func isValidHash(hash string) bool {
	if len(hash) != 64 {
		return false
	}
	for _, r := range hash {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func (c *Cache) Set(hash, original, corrected string) error {
	if hash == "" {
		return fmt.Errorf("hash cannot be empty")
	}

	// Validate hash to prevent path traversal attacks
	if !isValidHash(hash) {
		return fmt.Errorf("invalid hash format")
	}

	entry := CacheEntry{
		Hash:      hash,
		Original:  original,
		Corrected: corrected,
		Timestamp: time.Now().Unix(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	path := filepath.Join(c.dir, hash+".json")
	// Additional safety check: ensure the resolved path is within cache directory
	// Use filepath.Clean to resolve any path traversal attempts
	cleanPath := filepath.Clean(path)
	if !strings.HasPrefix(cleanPath, c.dir+string(filepath.Separator)) && cleanPath != c.dir {
		return fmt.Errorf("invalid cache path")
	}

	if err := os.WriteFile(path, data, CacheFilePerm); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}
