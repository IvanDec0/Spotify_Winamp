package soloist

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell helpers need a Unix-like system")
	}
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return path
}

func versionScript(version string, built int64) string {
	return "#!/bin/sh\necho \"soloist " + version + " build " + itoa(built) + " (20261023) (g19412df6ea)\"\n"
}

func itoa(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		wantVersion string
		wantBuilt   int64
		wantErr     bool
	}{
		{
			name:        "exact reference line",
			text:        "soloist 1.3.10.12 build 1792756912 (20261023) (g19412df6ea) (...)",
			wantVersion: "1.3.10.12",
			wantBuilt:   1792756912,
		},
		{
			name:        "exact line",
			text:        "soloist 1.3.10.12 build 1792756912 (20261023) (g19412df6ea) (x86_64)",
			wantVersion: "1.3.10.12",
			wantBuilt:   1792756912,
		},
		{
			name:        "banner lines",
			text:        "Soloist\nsoloist 1.3.10.12 build 1792756912 (20261023) (g19412df6ea)\ntrailing",
			wantVersion: "1.3.10.12",
			wantBuilt:   1792756912,
		},
		{
			name:        "unrelated soloist word first",
			text:        "soloist is the player\nsoloist 2.0.0 build 123 (20260101)",
			wantVersion: "2.0.0",
			wantBuilt:   123,
		},
		{name: "empty", text: "", wantErr: true},
		{name: "missing version", text: "soloist", wantErr: true},
		{name: "missing build", text: "soloist 1.2.3", wantErr: true},
		{name: "wrong literal", text: "soloist 1.2.3 builds 123", wantErr: true},
		{name: "not a number", text: "soloist 1.2.3 build nope", wantErr: true},
		{name: "no soloist token", text: "1.2.3 build 123", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binary, err := ParseVersion(test.text)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", binary)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if binary.Version != test.wantVersion {
				t.Fatalf("version = %q, want %q", binary.Version, test.wantVersion)
			}
			if binary.BuiltAt.Unix() != test.wantBuilt {
				t.Fatalf("built = %d, want %d", binary.BuiltAt.Unix(), test.wantBuilt)
			}
		})
	}
}

func TestBinaryExpiry(t *testing.T) {
	built := time.Unix(1792756912, 0).UTC()
	binary := &Binary{Version: "1.0.0", BuiltAt: built}
	expires := built.Add(90 * 24 * time.Hour)

	if !binary.ExpiresAt().Equal(expires) {
		t.Fatalf("ExpiresAt = %v, want %v", binary.ExpiresAt(), expires)
	}
	if binary.Expired(expires.Add(-time.Second)) {
		t.Fatal("expired one second before expiry")
	}
	if !binary.Expired(expires) {
		t.Fatal("not expired exactly at expiry")
	}
	if got := binary.DaysLeft(built); got != 90 {
		t.Fatalf("DaysLeft(built) = %d, want 90", got)
	}
	if got := binary.DaysLeft(expires.Add(-time.Second)); got != 0 {
		t.Fatalf("DaysLeft(before expiry) = %d, want 0", got)
	}
	if got := binary.DaysLeft(expires); got != 0 {
		t.Fatalf("DaysLeft(expiry) = %d, want 0", got)
	}
	if got := binary.DaysLeft(expires.Add(24 * time.Hour)); got != -1 {
		t.Fatalf("DaysLeft(after expiry) = %d, want -1", got)
	}
}

func TestInspectBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper")
	}
	dir := t.TempDir()
	path := writeScript(t, dir, "soloist", versionScript("1.3.10.12", 1792756912))
	binary, err := InspectBinary(t.Context(), path)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if binary.Path != path {
		t.Fatalf("path = %q, want %q", binary.Path, path)
	}
	if binary.Version != "1.3.10.12" || binary.BuiltAt.Unix() != 1792756912 {
		t.Fatalf("binary = %+v", binary)
	}
}

func TestInspectBinaryReportsFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper")
	}
	dir := t.TempDir()
	path := writeScript(t, dir, "broken", "#!/bin/sh\necho 'error while loading shared libraries: libssl.so' >&2\nexit 127\n")
	_, err := InspectBinary(t.Context(), path)
	if err == nil {
		t.Fatal("expected an error")
	}
	if CodeOf(err) != CodeNotInstalled {
		t.Fatalf("code = %q", CodeOf(err))
	}
	if got := err.Error(); got == "" || !contains(got, "libssl.so") {
		t.Fatalf("error = %q, want the stderr line", got)
	}
}

func TestFindInstalledEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := writeScript(t, dir, "custom-soloist", versionScript("9.9.9", 999))
	binary, err := FindInstalled(filepath.Join(dir, "programs"), path)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if binary.Path != path || binary.Version != "9.9.9" {
		t.Fatalf("binary = %+v", binary)
	}
}

func TestFindInstalledEnvOverrideInvalidDoesNotFallThrough(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, filepath.Join(dir, "programs", "100"), "soloist", versionScript("1.0.0", 100))
	missing := filepath.Join(dir, "does-not-exist")
	if _, err := FindInstalled(filepath.Join(dir, "programs"), missing); err == nil {
		t.Fatal("expected the env override error")
	}
}

func TestFindInstalledNewestNumericDirectory(t *testing.T) {
	dir := t.TempDir()
	programs := filepath.Join(dir, "programs")
	writeScript(t, filepath.Join(programs, "100"), "soloist", versionScript("1.0.0", 100))
	writeScript(t, filepath.Join(programs, "200"), "soloist", versionScript("2.0.0", 200))
	writeScript(t, filepath.Join(programs, "50"), "soloist", versionScript("0.5.0", 50))
	binary, err := FindInstalled(programs, "")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if binary.Version != "2.0.0" {
		t.Fatalf("version = %q, want 2.0.0", binary.Version)
	}
	want := filepath.Join(programs, "200", "soloist")
	if binary.Path != want {
		t.Fatalf("path = %q, want %q", binary.Path, want)
	}
}

func TestFindInstalledPathFallback(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	writeScript(t, binDir, "soloist", versionScript("3.0.0", 300))
	t.Setenv("PATH", binDir)
	binary, err := FindInstalled(filepath.Join(dir, "programs"), "")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if binary.Version != "3.0.0" {
		t.Fatalf("version = %q", binary.Version)
	}
}

func TestFindInstalledNotFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", filepath.Join(dir, "empty"))
	_, err := FindInstalled(filepath.Join(dir, "programs"), "")
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("error = %v, want ErrNotInstalled", err)
	}
}

func contains(text, fragment string) bool {
	return len(fragment) == 0 || (len(text) >= len(fragment) && indexOf(text, fragment) >= 0)
}

func indexOf(text, fragment string) int {
	for index := 0; index+len(fragment) <= len(text); index++ {
		if text[index:index+len(fragment)] == fragment {
			return index
		}
	}
	return -1
}
