package cache

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
	dir     string
	ttl     time.Duration
	encKey  []byte // Encryption key derived from user's home directory
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

	// Derive encryption key from user's home directory
	// This ensures cache is encrypted per-user
	keyHash := sha256.Sum256([]byte(home + ".grammr.cache.key"))
	encKey := keyHash[:] // Use first 32 bytes for AES-256

	return &Cache{
		dir:    cacheDir,
		ttl:    time.Duration(ttlDays) * 24 * time.Hour,
		encKey: encKey,
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

	// Try to decrypt the data (backward compatible with unencrypted entries)
	decryptedData, err := c.decrypt(data)
	if err != nil {
		// If decryption fails, try parsing as plain JSON (backward compatibility)
		decryptedData = data
	}

	var entry CacheEntry
	if err := json.Unmarshal(decryptedData, &entry); err != nil {
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

	// Encrypt the data before writing
	encryptedData, err := c.encrypt(data)
	if err != nil {
		return fmt.Errorf("failed to encrypt cache entry: %w", err)
	}

	path := filepath.Join(c.dir, hash+".json")
	// Additional safety check: ensure the resolved path is within cache directory
	// Use filepath.Clean to resolve any path traversal attempts
	cleanPath := filepath.Clean(path)
	if !strings.HasPrefix(cleanPath, c.dir+string(filepath.Separator)) && cleanPath != c.dir {
		return fmt.Errorf("invalid cache path")
	}

	if err := os.WriteFile(path, encryptedData, CacheFilePerm); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// encrypt encrypts data using AES-GCM
func (c *Cache) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	
	// Encode as base64 for safe storage
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(ciphertext)))
	base64.StdEncoding.Encode(encoded, ciphertext)
	
	return encoded, nil
}

// decrypt decrypts data using AES-GCM
func (c *Cache) decrypt(encryptedData []byte) ([]byte, error) {
	// Try to decode from base64 first
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(encryptedData)))
	n, err := base64.StdEncoding.Decode(decoded, encryptedData)
	if err != nil {
		// If base64 decode fails, assume it's not encrypted (backward compatibility)
		return encryptedData, fmt.Errorf("not base64 encoded (likely unencrypted)")
	}
	encryptedData = decoded[:n]

	block, err := aes.NewCipher(c.encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := encryptedData[:nonceSize], encryptedData[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}
