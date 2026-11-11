package webplayer

import (
	"archive/zip"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testZipEntry struct {
	Name string
	Body string
	Mode fs.FileMode
}

// writeTestZip builds a zip archive for the extraction and runtime tests.
func writeTestZip(t *testing.T, entries []testZipEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runtime.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	writer := zip.NewWriter(file)
	for _, entry := range entries {
		mode := entry.Mode
		if mode == 0 {
			if strings.HasSuffix(entry.Name, "/") {
				mode = 0o755
			} else {
				mode = 0o644
			}
		}
		header := &zip.FileHeader{Name: entry.Name, Method: zip.Deflate}
		header.SetMode(mode)
		member, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("create zip header %q: %v", entry.Name, err)
		}
		if _, err := member.Write([]byte(entry.Body)); err != nil {
			t.Fatalf("write zip member %q: %v", entry.Name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return path
}

func generousLimits() extractLimits {
	return extractLimits{MaxFileBytes: 1 << 20, MaxTotalBytes: 4 << 20, MaxFiles: 100}
}

func TestExtractRuntimeZip(t *testing.T) {
	archive := writeTestZip(t, []testZipEntry{
		{Name: "electron", Body: "runtime-binary", Mode: 0o755},
		{Name: "resources/app.asar", Body: "asar"},
		{Name: "locales/", Mode: 0o755},
		{Name: "locales/en-US.pak", Body: "pak"},
	})
	destination := filepath.Join(t.TempDir(), "runtime")

	if err := extractRuntimeZip(archive, destination, "electron", generousLimits()); err != nil {
		t.Fatalf("extract: %v", err)
	}
	assertFileContent(t, filepath.Join(destination, "electron"), "runtime-binary")
	assertFileContent(t, filepath.Join(destination, "resources", "app.asar"), "asar")
	assertFileContent(t, filepath.Join(destination, "locales", "en-US.pak"), "pak")

	assertMode(t, filepath.Join(destination, "electron"), 0o755)
	assertMode(t, filepath.Join(destination, "resources", "app.asar"), 0o644)
	assertMode(t, filepath.Join(destination, "locales"), 0o755)
}

func TestExtractRuntimeZipSingleTopLevelDirectory(t *testing.T) {
	archive := writeTestZip(t, []testZipEntry{
		{Name: "electron-v44.9.3/electron", Body: "runtime-binary", Mode: 0o755},
		{Name: "electron-v44.9.3/resources/app.asar", Body: "asar"},
		{Name: "electron-v44.9.3/locales/", Mode: 0o755},
		{Name: "electron-v44.9.3/locales/en-US.pak", Body: "pak"},
	})
	destination := filepath.Join(t.TempDir(), "runtime")

	if err := extractRuntimeZip(archive, destination, "electron", generousLimits()); err != nil {
		t.Fatalf("extract: %v", err)
	}
	assertFileContent(t, filepath.Join(destination, "electron"), "runtime-binary")
	assertFileContent(t, filepath.Join(destination, "resources", "app.asar"), "asar")
	if _, err := os.Stat(filepath.Join(destination, "electron-v44.9.3")); !os.IsNotExist(err) {
		t.Fatalf("the wrapping directory was not stripped: %v", err)
	}
}

func TestExtractRuntimeZipRejectsUnsafePaths(t *testing.T) {
	tests := []struct {
		name     string
		entries  []testZipEntry
		escaped  string
		contains string
	}{
		{
			name:     "parent traversal",
			entries:  []testZipEntry{{Name: "../escape.txt", Body: "escape"}},
			escaped:  "escape.txt",
			contains: "escapes",
		},
		{
			name:     "nested parent traversal",
			entries:  []testZipEntry{{Name: "a/../../escape.txt", Body: "escape"}},
			escaped:  "escape.txt",
			contains: "escapes",
		},
		{
			name:     "absolute path",
			entries:  []testZipEntry{{Name: "/tmp/escape.txt", Body: "escape"}},
			contains: "absolute",
		},
		{
			name:     "symbolic link",
			entries:  []testZipEntry{{Name: "link", Body: "/etc/passwd", Mode: fs.ModeSymlink | 0o777}},
			contains: "symbolic link",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			base := t.TempDir()
			destination := filepath.Join(base, "runtime")
			archive := writeTestZip(t, test.entries)
			err := extractRuntimeZip(archive, destination, "electron", generousLimits())
			if err == nil {
				t.Fatal("expected an extraction error")
			}
			if !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("error %q does not mention %q", err, test.contains)
			}
			if test.escaped != "" {
				if _, statErr := os.Stat(filepath.Join(base, test.escaped)); !os.IsNotExist(statErr) {
					t.Fatalf("archive wrote outside the destination: %v", statErr)
				}
			}
			if _, statErr := os.Stat(destination); !os.IsNotExist(statErr) {
				t.Fatalf("a failed extraction left the destination behind: %v", statErr)
			}
		})
	}
}

func TestExtractRuntimeZipRejectsDuplicateMembers(t *testing.T) {
	archive := writeTestZip(t, []testZipEntry{
		{Name: "electron", Body: "first", Mode: 0o755},
		{Name: "electron", Body: "second", Mode: 0o755},
	})
	err := extractRuntimeZip(archive, filepath.Join(t.TempDir(), "runtime"), "electron", generousLimits())
	if err == nil || !strings.Contains(err.Error(), "more than once") {
		t.Fatalf("expected a duplicate member error, got %v", err)
	}
}

func TestExtractRuntimeZipLimits(t *testing.T) {
	tests := []struct {
		name    string
		entries []testZipEntry
		limits  extractLimits
		want    string
	}{
		{
			name:    "per-file",
			entries: []testZipEntry{{Name: "electron", Body: strings.Repeat("x", 32), Mode: 0o755}},
			limits:  extractLimits{MaxFileBytes: 16, MaxTotalBytes: 1 << 20, MaxFiles: 10},
			want:    "per-file",
		},
		{
			name: "total",
			entries: []testZipEntry{
				{Name: "electron", Body: strings.Repeat("x", 16), Mode: 0o755},
				{Name: "resources/app.asar", Body: strings.Repeat("x", 16)},
			},
			limits: extractLimits{MaxFileBytes: 1 << 20, MaxTotalBytes: 24, MaxFiles: 10},
			want:   "total size",
		},
		{
			name: "file count",
			entries: []testZipEntry{
				{Name: "electron", Body: "x", Mode: 0o755},
				{Name: "a", Body: "x"},
				{Name: "b", Body: "x"},
			},
			limits: extractLimits{MaxFileBytes: 1 << 20, MaxTotalBytes: 1 << 20, MaxFiles: 2},
			want:   "more than",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			archive := writeTestZip(t, test.entries)
			err := extractRuntimeZip(archive, filepath.Join(t.TempDir(), "runtime"), "electron", test.limits)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected an error mentioning %q, got %v", test.want, err)
			}
		})
	}
}

func TestExtractRuntimeZipRequiresEntry(t *testing.T) {
	archive := writeTestZip(t, []testZipEntry{{Name: "other", Body: "x"}})
	err := extractRuntimeZip(archive, filepath.Join(t.TempDir(), "runtime"), "electron", generousLimits())
	if err == nil || !strings.Contains(err.Error(), "entry executable") {
		t.Fatalf("expected a missing entry error, got %v", err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("%s content = %q, want %q", path, data, want)
	}
}

func assertMode(t *testing.T, path string, want fs.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %o, want %o", path, got, want)
	}
}
