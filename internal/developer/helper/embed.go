// Package helper embeds the Spotify Developer dashboard helper application.
//
// The helper is a minimal Electron main process. It renders the Spotify
// Developer dashboard in a hidden window, automates the actions requested by
// the developer service over the loopback control socket, and reports only the
// resulting public Client ID or Soloist API key. It never logs credentials.
package helper

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed assets
var assets embed.FS

const (
	appAsset   = "assets/app"
	fileMode   = 0o600
	folderMode = 0o700
)

// Materialize writes the helper application under dir. Identical files are
// left untouched; new content is written through a temporary file and renamed
// into place. The directory and files are owner-only.
func Materialize(dir string) error {
	if dir == "" {
		return errors.New("the developer helper directory is empty")
	}
	if err := os.MkdirAll(dir, folderMode); err != nil {
		return err
	}
	if err := os.Chmod(dir, folderMode); err != nil {
		return err
	}

	app, err := fs.Sub(assets, appAsset)
	if err != nil {
		return fmt.Errorf("the embedded developer helper assets are missing: %w", err)
	}
	return fs.WalkDir(app, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		target := filepath.Join(dir, filepath.FromSlash(name))
		if entry.IsDir() {
			if err := os.MkdirAll(target, folderMode); err != nil {
				return err
			}
			return os.Chmod(target, folderMode)
		}
		data, err := fs.ReadFile(app, name)
		if err != nil {
			return err
		}
		return writeIfChanged(target, data)
	})
}

func writeIfChanged(path string, data []byte) error {
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return os.Chmod(path, fileMode)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Chmod(fileMode); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}
