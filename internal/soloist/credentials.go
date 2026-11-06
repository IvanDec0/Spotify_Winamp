package soloist

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// ReadAPIKey reads the API key file, trimming surrounding whitespace. A
// missing or empty file reports ErrAPIKeyMissing.
func ReadAPIKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrAPIKeyMissing
		}
		return "", codedError(CodeAPIKeyMissing, "cannot read the Spotify Soloist API key", err)
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", ErrAPIKeyMissing
	}
	return key, nil
}

// SaveAPIKey writes the key atomically with owner-only permissions. The key
// is never included in an error.
func SaveAPIKey(path, key string) error {
	key = strings.TrimSpace(key)
	if err := ValidateAPIKey(key); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return codedError(CodeAPIKeyInvalid, "cannot create the Spotify Soloist key directory", err)
	}
	// Tighten a directory that already existed with broader permissions.
	if err := os.Chmod(dir, 0o700); err != nil {
		return codedError(CodeAPIKeyInvalid, "cannot secure the Spotify Soloist key directory", err)
	}
	file, err := os.CreateTemp(dir, ".soloist-key-*")
	if err != nil {
		return codedError(CodeAPIKeyInvalid, "cannot create the Spotify Soloist key file", err)
	}
	tempPath := file.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return codedError(CodeAPIKeyInvalid, "cannot secure the Spotify Soloist key file", err)
	}
	if _, err := file.WriteString(key); err != nil {
		_ = file.Close()
		return codedError(CodeAPIKeyInvalid, "cannot write the Spotify Soloist key file", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return codedError(CodeAPIKeyInvalid, "cannot flush the Spotify Soloist key file", err)
	}
	if err := file.Close(); err != nil {
		return codedError(CodeAPIKeyInvalid, "cannot close the Spotify Soloist key file", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return codedError(CodeAPIKeyInvalid, "cannot install the Spotify Soloist key file", err)
	}
	committed = true
	return nil
}

// ClearAPIKey removes the key file. A missing file is not an error.
func ClearAPIKey(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return codedError(CodeAPIKeyInvalid, "cannot remove the Spotify Soloist key file", err)
	}
	return nil
}

// ValidateAPIKey rejects empty keys and keys with inner whitespace or control
// characters.
func ValidateAPIKey(key string) error {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return codedError(CodeAPIKeyInvalid, "the Spotify Soloist API key is empty", nil)
	}
	for _, char := range trimmed {
		if unicode.IsSpace(char) || unicode.IsControl(char) {
			return codedError(CodeAPIKeyInvalid, "the Spotify Soloist API key contains whitespace or control characters", nil)
		}
	}
	return nil
}
