package webplayer

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// manifestSchema is the only manifest schema this package understands.
const manifestSchema = 1

// manifestJSON is the pinned runtime manifest embedded in the binary. It is
// the only place that should ever name a runtime version or checksum.
//
//go:embed manifest.json
var manifestJSON []byte

// Manifest describes the pinned Chromium runtime for every supported
// platform.
type Manifest struct {
	Schema    int                 `json:"schema"`
	Version   string              `json:"version"`
	Platforms map[string]Platform `json:"platforms"`
}

// Platform is the runtime archive for one GOOS/GOARCH pair. Version comes
// from the enclosing manifest and is not serialized per platform.
type Platform struct {
	Version string `json:"-"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
	Entry   string `json:"entry"`
}

// Resolve returns the platform runtime for goos/goarch, or
// ErrUnsupportedPlatform.
func (m *Manifest) Resolve(goos, goarch string) (Platform, error) {
	if m == nil {
		return Platform{}, ErrInvalidManifest
	}
	platform, ok := m.Platforms[goos+"/"+goarch]
	if !ok {
		return Platform{}, ErrUnsupportedPlatform
	}
	platform.Version = m.Version
	return platform, nil
}

// Validate checks every rule the downloader and installer rely on. A manifest
// that fails validation is never used.
func (m *Manifest) Validate() error {
	if m == nil {
		return ErrInvalidManifest
	}
	if m.Schema != manifestSchema {
		return codedError(CodeInvalidManifest, fmt.Sprintf("unsupported web player manifest schema %d", m.Schema), nil)
	}
	if err := validateVersion(m.Version); err != nil {
		return err
	}
	if len(m.Platforms) == 0 {
		return codedError(CodeInvalidManifest, "the web player manifest has no platforms", nil)
	}
	for key, platform := range m.Platforms {
		if err := validatePlatformKey(key); err != nil {
			return err
		}
		if err := validatePlatform(platform); err != nil {
			return err
		}
	}
	return nil
}

func loadEmbeddedManifest() (*Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		return nil, codedError(CodeInvalidManifest, "the embedded web player manifest is not valid JSON", err)
	}
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func validateVersion(version string) error {
	if version == "" {
		return codedError(CodeInvalidManifest, "the web player manifest version is empty", nil)
	}
	if version == "." || version == ".." || strings.ContainsAny(version, `/\`) || strings.ContainsRune(version, 0) {
		return codedError(CodeInvalidManifest, "the web player manifest version is not a safe directory name", nil)
	}
	return nil
}

func validatePlatformKey(key string) error {
	goos, goarch, ok := strings.Cut(key, "/")
	if !ok || goos == "" || goarch == "" || strings.Contains(goarch, "/") {
		return codedError(CodeInvalidManifest, fmt.Sprintf("the web player manifest platform %q is not a GOOS/GOARCH pair", key), nil)
	}
	if strings.ContainsAny(goos+goarch, `\`) {
		return codedError(CodeInvalidManifest, fmt.Sprintf("the web player manifest platform %q is not safe", key), nil)
	}
	return nil
}

func validatePlatform(platform Platform) error {
	target, err := url.Parse(platform.URL)
	if err != nil {
		return codedError(CodeInvalidManifest, "a web player runtime URL is not a valid URL", err)
	}
	if !strings.EqualFold(target.Scheme, "https") {
		return codedError(CodeInvalidManifest, "a web player runtime URL does not use HTTPS", nil)
	}
	if target.Hostname() == "" {
		return codedError(CodeInvalidManifest, "a web player runtime URL has no host", nil)
	}
	if target.User != nil {
		return codedError(CodeInvalidManifest, "a web player runtime URL embeds credentials", nil)
	}
	if !validSHA256(platform.SHA256) {
		return codedError(CodeInvalidManifest, "a web player runtime SHA-256 is not 64 lowercase hexadecimal characters", nil)
	}
	if platform.Size <= 0 {
		return codedError(CodeInvalidManifest, "a web player runtime size is not positive", nil)
	}
	if platform.Entry == "" || platform.Entry == "." || platform.Entry == ".." ||
		strings.ContainsAny(platform.Entry, `/\`) || strings.ContainsRune(platform.Entry, 0) {
		return codedError(CodeInvalidManifest, "a web player runtime entry is not a plain file name", nil)
	}
	return nil
}

// validSHA256 reports whether value is exactly 64 lowercase hexadecimal
// characters.
func validSHA256(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
