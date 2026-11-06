package webplayer

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// clientIDFileLimit bounds the stored Client ID file. The value is a public
// 32-character identifier; anything larger is corrupt.
const clientIDFileLimit = 1 << 10

var clientIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// ClientIDPath returns the path of the stored Spotify Client ID inside the
// web player config directory.
func ClientIDPath(configDir string) string {
	return filepath.Join(configDir, "client-id")
}

// LoadClientID reads the stored Spotify Client ID. A missing, unreadable or
// malformed file returns "" so the caller can fall back to the built-in app.
func LoadClientID(configDir string) string {
	configDir = strings.TrimSpace(configDir)
	if configDir == "" {
		return ""
	}
	path := ClientIDPath(configDir)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > clientIDFileLimit {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	value := strings.TrimSpace(string(data))
	if !validClientID(value) {
		return ""
	}
	return value
}

// validClientID reports whether value is a Spotify Client ID: 32 lowercase
// hexadecimal characters.
func validClientID(value string) bool {
	return clientIDPattern.MatchString(value)
}
