package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreMissingFileReturnsDefaults(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.json"))
	config, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.Skins.ActiveID != "clarity" || config.Skins.ActiveTheme != "graphite" {
		t.Fatalf("default config = %#v", config)
	}
}

func TestStoreRoundTripAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	store := NewStore(path)
	config := Default()
	config.Skins.ActiveID = "afterglow"
	config.Skins.ActiveTheme = "ember"
	config.Skins.ThirdPartyDisabled = true

	if err := store.Save(config); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Skins.ActiveID != "afterglow" || loaded.Skins.ActiveTheme != "ember" || !loaded.Skins.ThirdPartyDisabled {
		t.Fatalf("loaded = %#v", loaded)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions = %o", info.Mode().Perm())
	}
	if tempFiles, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".config-*.tmp")); len(tempFiles) != 0 {
		t.Fatalf("temporary files left behind: %v", tempFiles)
	}
}

func TestStoreRejectsCorruptConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	config, err := store.Load()
	if err == nil {
		t.Fatal("Load() error = nil")
	}
	if config.Skins.ActiveID != "clarity" {
		t.Fatalf("fallback config = %#v", config)
	}
}

func TestSectionUpdatesPreserveOtherSections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := NewStore(path)

	if err := store.UpdateSpotify(func(state *SpotifyState) error {
		state.ClientID = "client-id"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateSkins(func(state *SkinState) error {
		state.ActiveID = "afterglow"
		state.ActiveTheme = "ember"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateSpotify(func(state *SpotifyState) error {
		state.ClientID = "new-client-id"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	config, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.Skins.ActiveID != "afterglow" || config.Skins.ActiveTheme != "ember" {
		t.Fatalf("skin state = %#v", config.Skins)
	}
	if config.Spotify.ClientID != "new-client-id" {
		t.Fatalf("spotify state = %#v", config.Spotify)
	}
}
