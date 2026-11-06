package webplayer

import (
	"errors"
	"strings"
	"testing"
)

func TestEmbeddedManifest(t *testing.T) {
	manifest, err := loadEmbeddedManifest()
	if err != nil {
		t.Fatalf("load embedded manifest: %v", err)
	}
	if manifest.Schema != manifestSchema {
		t.Fatalf("schema = %d", manifest.Schema)
	}
	platform, err := manifest.Resolve("linux", "amd64")
	if err != nil {
		t.Fatalf("resolve linux/amd64: %v", err)
	}
	if platform.Version != "44.9.3+wvcus" {
		t.Fatalf("version = %q", platform.Version)
	}
	if platform.SHA256 != "80cb5218ab814597282ebe41d16fb5a98abb0c75370e9195914e40e097359fad" {
		t.Fatalf("sha256 = %q", platform.SHA256)
	}
	if platform.Size != 123397658 {
		t.Fatalf("size = %d", platform.Size)
	}
	if platform.Entry != "electron" {
		t.Fatalf("entry = %q", platform.Entry)
	}
	if !strings.HasPrefix(platform.URL, "https://github.com/") {
		t.Fatalf("url = %q", platform.URL)
	}
	if _, err := manifest.Resolve("windows", "amd64"); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("unknown platform error = %v", err)
	}
	if _, err := manifest.Resolve("linux", "arm64"); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("unknown architecture error = %v", err)
	}
}

func TestManifestValidateRejects(t *testing.T) {
	validSHA := strings.Repeat("a", 64)
	validPlatform := Platform{
		URL:    "https://github.com/castlabs/electron-releases/runtime.zip",
		SHA256: validSHA,
		Size:   10,
		Entry:  "electron",
	}
	base := func() *Manifest {
		return &Manifest{
			Schema:    manifestSchema,
			Version:   "1.0.0",
			Platforms: map[string]Platform{"linux/amd64": validPlatform},
		}
	}

	tests := []struct {
		name     string
		mutate   func(*Manifest)
		contains string
	}{
		{"schema", func(m *Manifest) { m.Schema = 2 }, "schema"},
		{"empty version", func(m *Manifest) { m.Version = "" }, "version"},
		{"traversal version", func(m *Manifest) { m.Version = "../escape" }, "version"},
		{"no platforms", func(m *Manifest) { m.Platforms = nil }, "no platforms"},
		{"bad platform key", func(m *Manifest) { m.Platforms = map[string]Platform{"linux": validPlatform} }, "GOOS/GOARCH"},
		{"http url", func(m *Manifest) {
			platform := m.Platforms["linux/amd64"]
			platform.URL = "http://example.com/runtime.zip"
			m.Platforms["linux/amd64"] = platform
		}, "HTTPS"},
		{"url credentials", func(m *Manifest) {
			platform := m.Platforms["linux/amd64"]
			platform.URL = "https://user:secret@example.com/runtime.zip"
			m.Platforms["linux/amd64"] = platform
		}, "credentials"},
		{"uppercase sha", func(m *Manifest) {
			platform := m.Platforms["linux/amd64"]
			platform.SHA256 = strings.ToUpper(platform.SHA256)
			m.Platforms["linux/amd64"] = platform
		}, "SHA-256"},
		{"short sha", func(m *Manifest) {
			platform := m.Platforms["linux/amd64"]
			platform.SHA256 = "abcd"
			m.Platforms["linux/amd64"] = platform
		}, "SHA-256"},
		{"zero size", func(m *Manifest) {
			platform := m.Platforms["linux/amd64"]
			platform.Size = 0
			m.Platforms["linux/amd64"] = platform
		}, "size"},
		{"empty entry", func(m *Manifest) {
			platform := m.Platforms["linux/amd64"]
			platform.Entry = ""
			m.Platforms["linux/amd64"] = platform
		}, "entry"},
		{"entry with path", func(m *Manifest) {
			platform := m.Platforms["linux/amd64"]
			platform.Entry = "bin/electron"
			m.Platforms["linux/amd64"] = platform
		}, "entry"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := base()
			test.mutate(manifest)
			err := manifest.Validate()
			if err == nil {
				t.Fatal("expected a validation error")
			}
			if !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("error %q does not mention %q", err, test.contains)
			}
			if !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("error %v is not ErrInvalidManifest", err)
			}
		})
	}
}

func TestManifestValidateAccepts(t *testing.T) {
	manifest := &Manifest{
		Schema:  manifestSchema,
		Version: "1.0.0",
		Platforms: map[string]Platform{
			"linux/amd64": {
				URL:    "https://github.com/castlabs/electron-releases/runtime.zip",
				SHA256: strings.Repeat("a", 64),
				Size:   1,
				Entry:  "electron",
			},
		},
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}
