package cache

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
	}()

	// Set HOME to temp directory
	os.Setenv("HOME", tmpDir)

	tests := []struct {
		name    string
		ttlDays int
		wantErr bool
	}{
		{
			name:    "valid cache with 7 days TTL",
			ttlDays: 7,
			wantErr: false,
		},
		{
			name:    "valid cache with 1 day TTL",
			ttlDays: 1,
			wantErr: false,
		},
		{
			name:    "valid cache with 0 days TTL",
			ttlDays: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := New(tt.ttlDays)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if cache == nil && !tt.wantErr {
				t.Error("New() returned nil cache without error")
				return
			}
			if cache != nil {
				expectedTTL := time.Duration(tt.ttlDays) * 24 * time.Hour
				if cache.ttl != expectedTTL {
					t.Errorf("New() TTL = %v, want %v", cache.ttl, expectedTTL)
				}
				// Verify cache directory was created
				expectedDir := filepath.Join(tmpDir, ".grammr", "cache")
				if cache.dir != expectedDir {
					t.Errorf("New() dir = %v, want %v", cache.dir, expectedDir)
				}
				if _, err := os.Stat(cache.dir); os.IsNotExist(err) {
					t.Errorf("New() cache directory was not created: %v", cache.dir)
				}
			}
		})
	}
}

func TestHash(t *testing.T) {
	keyHash := sha256.Sum256([]byte("/tmp.grammr.cache.key"))
	cache := &Cache{dir: "/tmp", ttl: 24 * time.Hour, encKey: keyHash[:]}

	tests := []struct {
		name string
		text string
	}{
		{
			name: "empty string",
			text: "",
		},
		{
			name: "simple text",
			text: "Hello world",
		},
		{
			name: "text with newlines",
			text: "Hello\nworld\n",
		},
		{
			name: "unicode text",
			text: "Hello 世界",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cache.Hash(tt.text)
			// Verify hash is a valid hex string (64 characters for SHA256)
			if len(got) != 64 {
				t.Errorf("Hash() length = %v, want 64 (SHA256 hex)", len(got))
			}
			// Verify hash is consistent
			got2 := cache.Hash(tt.text)
			if got != got2 {
				t.Errorf("Hash() is not consistent: first = %v, second = %v", got, got2)
			}
			// Verify different inputs produce different hashes (except empty string)
			if tt.text != "" {
				otherHash := cache.Hash("different text")
				if got == otherHash {
					t.Errorf("Hash() produces same hash for different inputs")
				}
			}
		})
	}
}

func TestSetAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	// Derive encryption key from tmpDir (same way New() does)
	keyHash := sha256.Sum256([]byte(tmpDir + ".grammr.cache.key"))
	cache := &Cache{
		dir:    tmpDir,
		ttl:    24 * time.Hour,
		encKey: keyHash[:],
	}

	tests := []struct {
		name      string
		hash      string
		original  string
		corrected string
		wantErr   bool
	}{
		{
			name:      "simple entry",
			hash:      "test-hash-1",
			original:  "Hello world",
			corrected: "Hello, world",
			wantErr:   false,
		},
		{
			name:      "entry with newlines",
			hash:      "test-hash-2",
			original:  "Line 1\nLine 2",
			corrected: "Line 1\nLine 2.",
			wantErr:   false,
		},
		{
			name:      "empty strings",
			hash:      "test-hash-3",
			original:  "",
			corrected: "",
			wantErr:   false,
		},
		{
			name:      "unicode text",
			hash:      "test-hash-4",
			original:  "Hello 世界",
			corrected: "Hello, 世界!",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate proper hash from original text
			hash := cache.Hash(tt.original)
			if hash == "" {
				t.Fatal("Hash() returned empty string")
			}

			// Test Set
			err := cache.Set(hash, tt.original, tt.corrected)
			if (err != nil) != tt.wantErr {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify file was created
				path := filepath.Join(tmpDir, hash+".json")
				if _, err := os.Stat(path); os.IsNotExist(err) {
					t.Errorf("Set() cache file was not created: %v", path)
					return
				}

				// Verify file content (should be encrypted)
				data, err := os.ReadFile(path)
				if err != nil {
					t.Errorf("Set() failed to read cache file: %v", err)
					return
				}

				// Decrypt the data
				decryptedData, err := cache.decrypt(data)
				if err != nil {
					t.Errorf("Set() failed to decrypt cache entry: %v", err)
					return
				}

				var entry CacheEntry
				if err := json.Unmarshal(decryptedData, &entry); err != nil {
					t.Errorf("Set() failed to unmarshal cache entry: %v", err)
					return
				}

				if entry.Hash != hash {
					t.Errorf("Set() entry.Hash = %v, want %v", entry.Hash, hash)
				}
				if entry.Original != tt.original {
					t.Errorf("Set() entry.Original = %v, want %v", entry.Original, tt.original)
				}
				if entry.Corrected != tt.corrected {
					t.Errorf("Set() entry.Corrected = %v, want %v", entry.Corrected, tt.corrected)
				}
				if entry.Timestamp == 0 {
					t.Error("Set() entry.Timestamp should be set")
				}

				// Test Get
				got := cache.Get(hash)
				if got != tt.corrected {
					t.Errorf("Get() = %v, want %v", got, tt.corrected)
				}
			}
		})
	}
}

func TestGetNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	keyHash := sha256.Sum256([]byte(tmpDir + ".grammr.cache.key"))
	cache := &Cache{
		dir:    tmpDir,
		ttl:    24 * time.Hour,
		encKey: keyHash[:],
	}

	// Test getting non-existent entry
	got := cache.Get("non-existent-hash")
	if got != "" {
		t.Errorf("Get() non-existent = %v, want empty string", got)
	}
}

func TestGetExpired(t *testing.T) {
	tmpDir := t.TempDir()
	keyHash := sha256.Sum256([]byte(tmpDir + ".grammr.cache.key"))
	cache := &Cache{
		dir:    tmpDir,
		ttl:    1 * time.Hour, // 1 hour TTL
		encKey: keyHash[:],
	}

	original := "Hello world"
	corrected := "Hello, world"
	hash := cache.Hash(original)

	// Create an expired entry manually (encrypted)
	entry := CacheEntry{
		Hash:      hash,
		Original:  original,
		Corrected: corrected,
		Timestamp: time.Now().Add(-2 * time.Hour).Unix(), // 2 hours ago
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal entry: %v", err)
	}

	// Encrypt the data before writing
	encryptedData, err := cache.encrypt(data)
	if err != nil {
		t.Fatalf("Failed to encrypt entry: %v", err)
	}

	path := filepath.Join(tmpDir, hash+".json")
	if err := os.WriteFile(path, encryptedData, 0600); err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	// Get should return empty string and remove the file
	got := cache.Get(hash)
	if got != "" {
		t.Errorf("Get() expired entry = %v, want empty string", got)
	}

	// Verify file was removed
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Get() should have removed expired cache file")
	}
}

func TestGetNotExpired(t *testing.T) {
	tmpDir := t.TempDir()
	keyHash := sha256.Sum256([]byte(tmpDir + ".grammr.cache.key"))
	cache := &Cache{
		dir:    tmpDir,
		ttl:    24 * time.Hour,
		encKey: keyHash[:],
	}

	original := "Hello world"
	corrected := "Hello, world"
	hash := cache.Hash(original)

	// Create a valid (not expired) entry manually (encrypted)
	entry := CacheEntry{
		Hash:      hash,
		Original:  original,
		Corrected: corrected,
		Timestamp: time.Now().Unix(), // Current time
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal entry: %v", err)
	}

	// Encrypt the data before writing
	encryptedData, err := cache.encrypt(data)
	if err != nil {
		t.Fatalf("Failed to encrypt entry: %v", err)
	}

	path := filepath.Join(tmpDir, hash+".json")
	if err := os.WriteFile(path, encryptedData, 0600); err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	// Get should return the corrected text
	got := cache.Get(hash)
	if got != corrected {
		t.Errorf("Get() valid entry = %v, want %v", got, corrected)
	}

	// Verify file still exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Get() should not remove valid cache file")
	}
}

func TestGetInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	keyHash := sha256.Sum256([]byte(tmpDir + ".grammr.cache.key"))
	cache := &Cache{
		dir:    tmpDir,
		ttl:    24 * time.Hour,
		encKey: keyHash[:],
	}

	// Use a valid hash format (64 hex chars) but with invalid JSON content
	hash := cache.Hash("test text")
	path := filepath.Join(tmpDir, hash+".json")

	// Write invalid JSON
	if err := os.WriteFile(path, []byte("invalid json"), 0600); err != nil {
		t.Fatalf("Failed to write invalid JSON file: %v", err)
	}

	// Get should return empty string for invalid JSON
	got := cache.Get(hash)
	if got != "" {
		t.Errorf("Get() invalid JSON = %v, want empty string", got)
	}
}

func TestNewRejectsNegativeTTL(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
	}()
	os.Setenv("HOME", tmpDir)

	_, err := New(-1)
	if err == nil {
		t.Fatal("New(-1) should return an error")
	}
}

func TestSetRejectsInvalidHash(t *testing.T) {
	tmpDir := t.TempDir()
	keyHash := sha256.Sum256([]byte(tmpDir + ".grammr.cache.key"))
	cache := &Cache{
		dir:    tmpDir,
		ttl:    24 * time.Hour,
		encKey: keyHash[:],
	}

	err := cache.Set("../invalid", "orig", "corr")
	if err == nil {
		t.Fatal("Set() with invalid hash should return error")
	}
}

func TestGetRejectsInvalidHash(t *testing.T) {
	tmpDir := t.TempDir()
	keyHash := sha256.Sum256([]byte(tmpDir + ".grammr.cache.key"))
	cache := &Cache{
		dir:    tmpDir,
		ttl:    24 * time.Hour,
		encKey: keyHash[:],
	}

	if got := cache.Get("../not-a-hash"); got != "" {
		t.Fatalf("Get() with invalid hash = %q, want empty", got)
	}
}

func TestCachePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
	}()
	os.Setenv("HOME", tmpDir)

	cache, err := New(7)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	original := "Hello world"
	corrected := "Hello, world!"
	hash := cache.Hash(original)

	if err := cache.Set(hash, original, corrected); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	dirInfo, err := os.Stat(cache.dir)
	if err != nil {
		t.Fatalf("failed to stat cache dir: %v", err)
	}
	if dirInfo.Mode().Perm()&0077 != 0 {
		t.Fatalf("cache directory should not be accessible by group/others, got mode %o", dirInfo.Mode().Perm())
	}

	filePath := filepath.Join(cache.dir, hash+".json")
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat cache file: %v", err)
	}
	if fileInfo.Mode().Perm()&0077 != 0 {
		t.Fatalf("cache file should not be accessible by group/others, got mode %o", fileInfo.Mode().Perm())
	}
}
