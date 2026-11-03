// Package settings persists Cadence's small, user-editable configuration.
// Skin state and Spotify state are written through section-specific updates so
// concurrent features never overwrite each other's configuration.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// SchemaVersion identifies the on-disk configuration format.
const SchemaVersion = 1

// SkinState is the persisted skin configuration.
type SkinState struct {
	ActiveID           string `json:"activeId"`
	ActiveTheme        string `json:"activeTheme"`
	LastKnownGoodID    string `json:"lastKnownGoodId"`
	LastKnownGoodTheme string `json:"lastKnownGoodTheme"`
	ThirdPartyDisabled bool   `json:"thirdPartyDisabled"`
}

// Config is the root configuration document. New top-level sections can be
// added without changing the skin or Spotify stores.
type Config struct {
	SchemaVersion int       `json:"schemaVersion"`
	Skins         SkinState `json:"skins"`
}

// Default returns a configuration that always selects the built-in skin and
// leaves Spotify unconfigured.
func Default() Config {
	return Config{
		SchemaVersion: SchemaVersion,
		Skins: SkinState{
			ActiveID:           "clarity",
			ActiveTheme:        "graphite",
			LastKnownGoodID:    "clarity",
			LastKnownGoodTheme: "graphite",
		},
	}
}

// Store reads and writes the configuration atomically. A zero-value Store is
// not usable; construct it with NewStore.
type Store struct {
	path string
	mu   sync.Mutex
}

// NewStore creates a store rooted at an explicit config file path. The parent
// directory is created on the first save.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Path returns the configuration file path.
func (s *Store) Path() string {
	return s.path
}

// Load reads the configuration. A missing file returns Default. Invalid JSON
// or a newer schema is reported to the caller, which should fall back to
// defaults rather than refusing to start.
func (s *Store) Load() (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Store) loadLocked() (Config, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Default(), fmt.Errorf("read settings: %w", err)
	}
	if len(data) > 256<<10 {
		return Default(), errors.New("settings file exceeds 256 KB")
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Default(), fmt.Errorf("parse settings: %w", err)
	}
	if config.SchemaVersion != SchemaVersion {
		return Default(), fmt.Errorf("unsupported settings schema version %d", config.SchemaVersion)
	}
	return config, nil
}

// UpdateSkins atomically updates only the skin section while preserving the
// latest Spotify settings on disk.
func (s *Store) UpdateSkins(mutate func(*SkinState) error) error {
	return s.update(func(config *Config) error {
		return mutate(&config.Skins)
	})
}

func (s *Store) update(mutate func(*Config) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, err := s.loadLocked()
	if err != nil {
		// The previous file is corrupt or from an unsupported version. Start
		// from defaults rather than making the feature permanently unusable.
		config = Default()
	}
	config.SchemaVersion = SchemaVersion
	if err := mutate(&config); err != nil {
		return err
	}
	return s.saveLocked(config)
}

// Save writes the complete configuration atomically with owner-only
// permissions. Prefer UpdateSkins or UpdateSpotify for feature code.
func (s *Store) Save(config Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(config)
}

func (s *Store) saveLocked(config Config) error {
	if config.SchemaVersion == 0 {
		config.SchemaVersion = SchemaVersion
	}
	if config.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported settings schema version %d", config.SchemaVersion)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}

	temp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary settings file: %w", err)
	}
	tempName := temp.Name()
	defer func() {
		_ = os.Remove(tempName)
	}()

	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, s.path); err != nil {
		return fmt.Errorf("replace settings file: %w", err)
	}
	return nil
}
