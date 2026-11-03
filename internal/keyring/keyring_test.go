package keyring

import (
	"errors"
	"testing"
)

func TestMemoryStoreLifecycle(t *testing.T) {
	store := NewMemoryStore()
	if _, err := store.GetRefreshToken(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetRefreshToken() error = %v", err)
	}
	if err := store.SetRefreshToken("token"); err != nil {
		t.Fatal(err)
	}
	token, err := store.GetRefreshToken()
	if err != nil || token != "token" {
		t.Fatalf("GetRefreshToken() = %q, %v", token, err)
	}
	if err := store.DeleteRefreshToken(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetRefreshToken(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetRefreshToken() after delete error = %v", err)
	}
}

func TestMemoryStoreReportsUnavailable(t *testing.T) {
	store := NewMemoryStore()
	store.availableErr = errors.New("no secret service")
	if err := store.Available(); err == nil {
		t.Fatal("Available() error = nil")
	}
	if err := store.SetRefreshToken("token"); err == nil {
		t.Fatal("SetRefreshToken() error = nil")
	}
}
