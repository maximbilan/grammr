package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
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
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".grammr", "cache")

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
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
	path := filepath.Join(c.dir, hash+".json")

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
		os.Remove(path)
		return ""
	}

	return entry.Corrected
}

func (c *Cache) Set(hash, original, corrected string) error {
	entry := CacheEntry{
		Hash:      hash,
		Original:  original,
		Corrected: corrected,
		Timestamp: time.Now().Unix(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	path := filepath.Join(c.dir, hash+".json")
	return os.WriteFile(path, data, 0644)
}
