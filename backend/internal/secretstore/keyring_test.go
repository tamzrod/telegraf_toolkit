package secretstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadKeyringRejectsMissingFile(t *testing.T) {
	if _, err := LoadKeyring(filepath.Join(t.TempDir(), "absent.key")); err == nil {
		t.Fatal("a missing key file must fail closed")
	}
	if _, err := LoadKeyring(""); err == nil {
		t.Fatal("an empty key path must fail closed")
	}
}

func TestLoadKeyringRejectsWorldReadableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loose.key")
	if err := WriteKeyFile(path, "k1", map[string][]byte{"k1": testKey(1)}); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if _, err := LoadKeyring(path); err == nil {
		t.Fatal("a group/other-readable key file must be refused")
	}
}

func TestLoadKeyringRejectsWrongKeySize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "short.key")
	if err := os.WriteFile(path, []byte(`{"active_key_id":"k1","keys":{"k1":"AAAA"}}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadKeyring(path); err == nil {
		t.Fatal("a non-32-byte key must be refused")
	}
}

func TestLoadKeyringRejectsMissingActiveKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.key")
	if err := os.WriteFile(path, []byte(`{"active_key_id":"k9","keys":{"k1":"`+b64(testKey(1))+`"}}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadKeyring(path); err == nil {
		t.Fatal("an active key ID absent from keys must be refused")
	}
}

func TestWriteKeyFileRefusesToOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "k.key")
	if err := WriteKeyFile(path, "k1", map[string][]byte{"k1": testKey(1)}); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteKeyFile(path, "k1", map[string][]byte{"k1": testKey(2)}); err == nil {
		t.Fatal("WriteKeyFile must not overwrite an existing key file")
	}
}

func TestLoadKeyringHoldsMultipleKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.key")
	if err := WriteKeyFile(path, "k2", map[string][]byte{"k1": testKey(1), "k2": testKey(2)}); err != nil {
		t.Fatalf("write: %v", err)
	}
	kr, err := LoadKeyring(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if kr.ActiveKeyID() != "k2" {
		t.Fatalf("active = %q, want k2", kr.ActiveKeyID())
	}
	if !kr.has("k1") || !kr.has("k2") {
		t.Fatal("both keys should be held")
	}
	if len(kr.ids()) != 2 {
		t.Fatalf("ids = %v, want 2", kr.ids())
	}
}

func TestNewRequiresAllDependencies(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("New must require a keyring")
	}
	kr := mustKeyring(t, map[string][]byte{"k1": testKey(1)}, "k1")
	if _, err := New(Options{Keyring: kr}); err == nil {
		t.Fatal("New must require an envelope store")
	}
}
