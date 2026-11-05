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

func TestDefaultEngineState(t *testing.T) {
	config := Default()
	if config.Engine == nil {
		t.Fatal("Default().Engine = nil")
	}
	want := EngineState{Bitrate: 320, CacheSizeLimit: "1GB", ImageSize: "default"}
	if *config.Engine != want {
		t.Fatalf("default engine = %#v, want %#v", *config.Engine, want)
	}
}

func TestStoreUpdateEnginePreservesSkins(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.json"))
	config := Default()
	config.Skins.ActiveID = "afterglow"
	config.Skins.ActiveTheme = "ember"
	config.Skins.ThirdPartyDisabled = true
	if err := store.Save(config); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := store.UpdateEngine(func(engine *EngineState) error {
		engine.Bitrate = 96
		engine.CacheEnabled = true
		engine.CacheSizeLimit = "2GB"
		engine.ImageSize = "large"
		return nil
	}); err != nil {
		t.Fatalf("UpdateEngine() error = %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Skins.ActiveID != "afterglow" || loaded.Skins.ActiveTheme != "ember" || !loaded.Skins.ThirdPartyDisabled {
		t.Fatalf("skins changed by engine update: %#v", loaded.Skins)
	}
	if loaded.Engine == nil || loaded.Engine.Bitrate != 96 || !loaded.Engine.CacheEnabled ||
		loaded.Engine.CacheSizeLimit != "2GB" || loaded.Engine.ImageSize != "large" {
		t.Fatalf("engine = %#v", loaded.Engine)
	}
}

func TestStoreLegacyFileWithoutEngineGetsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	legacy := `{
  "schemaVersion": 1,
  "skins": {
    "activeId": "legacy",
    "activeTheme": "graphite",
    "lastKnownGoodId": "legacy",
    "lastKnownGoodTheme": "graphite",
    "thirdPartyDisabled": false
  }
}
`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := NewStore(path).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.Skins.ActiveID != "legacy" {
		t.Fatalf("legacy skins = %#v", config.Skins)
	}
	if config.Engine == nil || config.Engine.Bitrate != 320 || config.Engine.CacheSizeLimit != "1GB" ||
		config.Engine.ImageSize != "default" {
		t.Fatalf("legacy engine = %#v", config.Engine)
	}
}

func TestStoreEngineRoundTrip(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.json"))
	want := EngineState{
		Bitrate:                   160,
		CrossfadeDurationMS:       4500,
		NormalisationDisabled:     true,
		NormalisationUseAlbumGain: true,
		NormalisationPregain:      -3.5,
		CacheEnabled:              true,
		CacheDir:                  "/tmp/cadence-audio-cache",
		CacheSizeLimit:            "4GB",
		FlacEnabled:               true,
		MPRISEnabled:              true,
		DeviceIsGroup:             true,
		ImageSize:                 "xlarge",
	}
	config := Default()
	*config.Engine = want
	if err := store.Save(config); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Engine == nil || *loaded.Engine != want {
		t.Fatalf("engine = %#v, want %#v", loaded.Engine, want)
	}
}
