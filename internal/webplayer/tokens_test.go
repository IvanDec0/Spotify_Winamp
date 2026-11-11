package webplayer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sampleTokenSession() Tokens {
	return Tokens{
		AccessToken:  "access-token-value",
		RefreshToken: "refresh-token-value",
		TokenType:    "Bearer",
		Scope:        strings.Join(DefaultScopes, " "),
		Expiry:       time.Now().Add(time.Hour).UTC().Truncate(time.Second),
		Account:      "Listener",
	}
}

func TestTokenStoreSaveLoadClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "tokens.json")
	store := NewTokenStore(path)
	want := sampleTokenSession()
	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat token file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("token file permissions = %o, want 600", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat token directory: %v", err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("token directory permissions = %o, want 700", dirInfo.Mode().Perm())
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != want {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrNoTokens) {
		t.Fatalf("Load() after Clear() = %v, want ErrNoTokens", err)
	}
	if err := store.Clear(); err != nil {
		t.Fatalf("second Clear() error = %v", err)
	}
}

func TestTokenStoreMissingFile(t *testing.T) {
	store := NewTokenStore(filepath.Join(t.TempDir(), "tokens.json"))
	_, err := store.Load()
	if !errors.Is(err, ErrNoTokens) {
		t.Fatalf("Load() error = %v, want ErrNoTokens", err)
	}
	if CodeOf(err) != CodeNoTokens {
		t.Fatalf("code = %q, want %q", CodeOf(err), CodeNoTokens)
	}
}

func TestTokenStoreCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	if err := os.WriteFile(path, []byte(`{"access_token":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewTokenStore(path).Load(); err == nil {
		t.Fatal("Load() of a corrupt file = nil error")
	}
}

func TestTokenStoreOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	data := make([]byte, maxTokenFileSize+1)
	for index := range data {
		data[index] = 'x'
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewTokenStore(path).Load(); err == nil {
		t.Fatal("Load() of an oversized file = nil error")
	}
}

func TestTokenStoreTightensPermissionsOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	store := NewTokenStore(path)
	if err := store.Save(sampleTokenSession()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions after Load() = %o, want 600", info.Mode().Perm())
	}
}
