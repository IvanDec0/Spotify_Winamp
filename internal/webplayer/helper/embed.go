// Package helper embeds the Cadence web player helper: a minimal Electron
// main process, a sandboxed preload and the Spotify Web Playback SDK page.
//
// The assets are embedded in the Cadence binary and materialized next to the
// downloaded Chromium runtime. The renderer never gets Node integration, and
// the preload exposes only the two control calls the player needs.
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
	pageAsset   = "assets/page.html"
	scriptAsset = "assets/page.js"
	appAsset    = "assets/app"
	fileMode    = 0o600
	folderMode  = 0o700
)

// Page returns a copy of the embedded player page. The local HTTP server
// serves it; it is not written to the helper directory.
func Page() []byte {
	data, err := assets.ReadFile(pageAsset)
	if err != nil {
		return nil
	}
	return bytes.Clone(data)
}

// Script returns a copy of the embedded player glue. The local HTTP server
// serves it as a same-origin script so the page needs neither inline script
// nor a relaxed Content Security Policy.
func Script() []byte {
	data, err := assets.ReadFile(scriptAsset)
	if err != nil {
		return nil
	}
	return bytes.Clone(data)
}

// Materialize writes the helper application files under dir. It is
// idempotent: identical files are left untouched, and new content is written
// through a temporary file and renamed into place.
func Materialize(dir string) error {
	if dir == "" {
		return errors.New("the web player helper directory is empty")
	}
	if err := os.MkdirAll(dir, folderMode); err != nil {
		return err
	}
	if err := os.Chmod(dir, folderMode); err != nil {
		return err
	}

	app, err := fs.Sub(assets, appAsset)
	if err != nil {
		return fmt.Errorf("the embedded web player helper assets are missing: %w", err)
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

// writeIfChanged avoids rewriting an identical file so a helper restart does
// not invalidate the materialized assets for no reason.
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
	if err := os.Rename(tempName, path); err != nil {
		return err
	}
	return nil
}
