package secretstore

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// keyFileMode is the required permission on the key file: owner read-only. A
// key file readable by group or others is refused, because the master key must
// not be exposed to other accounts on the host.
const keyFileMode os.FileMode = 0o600

// keyFile is the on-disk JSON shape. Keys are base64url-encoded 32-byte values.
//
// The file is provisioned outside the application, is not committed to Git, and
// is not stored in the metadata database. `ActiveKeyID` names the key new
// envelopes are wrapped with; `Retained` holds keys kept only to unwrap
// existing envelopes during a bounded rotation window.
type keyFile struct {
	ActiveKeyID string            `json:"active_key_id"`
	Keys        map[string]string `json:"keys"`
}

// Keyring holds the master keys this backend may use. It is safe for concurrent
// use.
type Keyring struct {
	mu     sync.RWMutex
	active string
	keys   map[string][]byte
}

// LoadKeyring reads and validates a master-key file. It fails closed: a missing,
// unreadable, world-readable, malformed or empty key file yields an error and no
// keyring, so the store cannot resolve or write secrets without a valid key.
func LoadKeyring(path string) (*Keyring, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("master key file path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat master key file: %w", err)
	}
	if info.IsDir() {
		return nil, errors.New("master key path is a directory, not a file")
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return nil, fmt.Errorf("master key file permissions %#o are too open; require %#o (owner-only)", perm, keyFileMode)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read master key file: %w", err)
	}

	var parsed keyFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse master key file: %w", err)
	}
	if parsed.ActiveKeyID == "" {
		return nil, errors.New("master key file has no active_key_id")
	}
	if len(parsed.Keys) == 0 {
		return nil, errors.New("master key file has no keys")
	}

	keys := make(map[string][]byte, len(parsed.Keys))
	for id, encoded := range parsed.Keys {
		key, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("master key %q is not valid base64url: %w", id, err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("master key %q must be 32 bytes for AES-256, got %d", id, len(key))
		}
		keys[id] = key
	}
	if _, ok := keys[parsed.ActiveKeyID]; !ok {
		return nil, fmt.Errorf("active_key_id %q is not present in keys", parsed.ActiveKeyID)
	}

	return &Keyring{active: parsed.ActiveKeyID, keys: keys}, nil
}

// ActiveKeyID returns the key ID new envelopes are wrapped with.
func (k *Keyring) ActiveKeyID() string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.active
}

// key returns the key material for an ID.
func (k *Keyring) key(id string) ([]byte, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	key, ok := k.keys[id]
	return key, ok
}

// has reports whether a key ID is held.
func (k *Keyring) has(id string) bool {
	_, ok := k.key(id)
	return ok
}

// ids returns the held key IDs (unsorted) for diagnostics that never expose key
// material.
func (k *Keyring) ids() []string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := make([]string, 0, len(k.keys))
	for id := range k.keys {
		out = append(out, id)
	}
	return out
}

// WriteKeyFile provisions a key file atomically with owner-only permissions. It
// is used by operators and by tests to create a key outside the application; the
// running backend only ever reads a key file. It refuses to overwrite an
// existing file so a live key is never silently replaced.
func WriteKeyFile(path, activeKeyID string, keys map[string][]byte) error {
	if activeKeyID == "" || len(keys) == 0 {
		return errors.New("key file requires an active key ID and at least one key")
	}
	if _, ok := keys[activeKeyID]; !ok {
		return fmt.Errorf("active key ID %q is not present in keys", activeKeyID)
	}
	encoded := make(map[string]string, len(keys))
	for id, key := range keys {
		if len(key) != 32 {
			return fmt.Errorf("key %q must be 32 bytes, got %d", id, len(key))
		}
		encoded[id] = base64.RawURLEncoding.EncodeToString(key)
	}
	payload, err := json.MarshalIndent(keyFile{ActiveKeyID: activeKeyID, Keys: encoded}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode key file: %w", err)
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create key file directory: %w", err)
		}
	}
	// O_EXCL so an existing key is never overwritten.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, keyFileMode)
	if err != nil {
		return fmt.Errorf("create key file: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(payload); err != nil {
		return fmt.Errorf("write key file: %w", err)
	}
	return f.Sync()
}
