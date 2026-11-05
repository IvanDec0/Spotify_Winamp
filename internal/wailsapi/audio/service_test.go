package audioapi

import (
	"path/filepath"
	"testing"

	"cadence/internal/engine"
	"cadence/internal/settings"
)

func TestEngineSettingsDefaultsAndPersist(t *testing.T) {
	store := settings.NewStore(filepath.Join(t.TempDir(), "config.json"))
	supervisor := engine.NewSupervisor(engine.Options{})
	service := NewService(Options{Supervisor: supervisor, Settings: store})

	defaults, err := service.GetEngineSettings()
	if err != nil {
		t.Fatalf("GetEngineSettings() error = %v", err)
	}
	if defaults.Bitrate != 320 || defaults.CacheSizeLimit != "1GB" || defaults.ImageSize != "default" {
		t.Fatalf("default engine settings = %#v", defaults)
	}

	if err := store.UpdateSkins(func(state *settings.SkinState) error {
		state.ActiveID = "afterglow"
		state.ActiveTheme = "ember"
		return nil
	}); err != nil {
		t.Fatalf("UpdateSkins() error = %v", err)
	}

	saved, err := service.SetEngineSettings(EngineSettings{
		Bitrate:                   128,
		CrossfadeDurationMS:       12001,
		NormalisationDisabled:     true,
		NormalisationUseAlbumGain: true,
		NormalisationPregain:      21,
		CacheEnabled:              true,
		CacheDir:                  "/tmp/cadence-audio-cache",
		CacheSizeLimit:            "3GB",
		FlacEnabled:               true,
		MPRISEnabled:              true,
		DeviceIsGroup:             true,
		ImageSize:                 "huge",
	})
	if err != nil {
		t.Fatalf("SetEngineSettings() error = %v", err)
	}
	want := EngineSettings{
		Bitrate:                   320,
		CrossfadeDurationMS:       0,
		NormalisationDisabled:     true,
		NormalisationUseAlbumGain: true,
		NormalisationPregain:      0,
		CacheEnabled:              true,
		CacheDir:                  "/tmp/cadence-audio-cache",
		CacheSizeLimit:            "1GB",
		FlacEnabled:               true,
		MPRISEnabled:              true,
		DeviceIsGroup:             true,
		ImageSize:                 "default",
	}
	if saved != want {
		t.Fatalf("saved = %#v, want %#v", saved, want)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Skins.ActiveID != "afterglow" || loaded.Skins.ActiveTheme != "ember" {
		t.Fatalf("engine update overwrote skins: %#v", loaded.Skins)
	}
	if loaded.Engine == nil || *loaded.Engine != want {
		t.Fatalf("persisted engine = %#v, want %#v", loaded.Engine, want)
	}

	after, err := service.GetEngineSettings()
	if err != nil {
		t.Fatalf("GetEngineSettings() after set error = %v", err)
	}
	if after != want {
		t.Fatalf("GetEngineSettings() = %#v, want %#v", after, want)
	}
	if supervisor.Client() != nil {
		t.Fatal("SetEngineSettings started or replaced the engine client")
	}
}

func TestEngineSettingsRequireStore(t *testing.T) {
	service := NewService(Options{Supervisor: engine.NewSupervisor(engine.Options{})})
	if _, err := service.GetEngineSettings(); err == nil {
		t.Fatal("GetEngineSettings() error = nil")
	}
	if _, err := service.SetEngineSettings(EngineSettings{}); err == nil {
		t.Fatal("SetEngineSettings() error = nil")
	}
}
