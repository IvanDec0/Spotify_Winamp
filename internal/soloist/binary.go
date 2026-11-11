package soloist

import (
	"bytes"
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	// buildLifetime is how long Spotify keeps a Soloist build working after
	// its build timestamp.
	buildLifetime = 90 * 24 * time.Hour
	// versionTimeout bounds one `soloist --version` run.
	versionTimeout = 15 * time.Second
	// versionOutputLimit caps each captured output stream.
	versionOutputLimit = 4 << 10
	// versionReasonLimit caps the diagnostic line embedded in an error.
	versionReasonLimit = 300
)

// Binary is an installed Spotify Soloist build, as `soloist --version`
// describes it.
type Binary struct {
	Path    string
	Version string
	BuiltAt time.Time
}

// ExpiresAt returns when Spotify stops accepting this build.
func (b *Binary) ExpiresAt() time.Time {
	return b.BuiltAt.Add(buildLifetime)
}

// Expired reports whether the build no longer works at now.
func (b *Binary) Expired(now time.Time) bool {
	return !now.Before(b.ExpiresAt())
}

// DaysLeft returns the whole days until the build expires, floored. It is
// negative once the build has expired.
func (b *Binary) DaysLeft(now time.Time) int {
	return int(math.Floor(b.ExpiresAt().Sub(now).Hours() / 24))
}

// ParseVersion finds the version and build timestamp in Soloist's version
// output: "soloist <version> build <unix-seconds> (...)". Extra leading or
// trailing lines are tolerated.
func ParseVersion(text string) (*Binary, error) {
	words := strings.Fields(text)
	for index := 0; index < len(words); index++ {
		if words[index] != "soloist" {
			continue
		}
		if index+3 >= len(words) {
			continue
		}
		version := words[index+1]
		if words[index+2] != "build" {
			continue
		}
		seconds, err := strconv.ParseInt(words[index+3], 10, 64)
		if err != nil {
			continue
		}
		return &Binary{Version: version, BuiltAt: time.Unix(seconds, 0).UTC()}, nil
	}
	return nil, codedError(CodeProtocol, "cannot parse Spotify Soloist's version output", nil)
}

// InspectBinary runs `path --version` and parses the build. It does not
// validate expiry.
func InspectBinary(ctx context.Context, path string) (*Binary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, versionTimeout)
	defer cancel()

	var stdout, stderr cappedBuffer
	stdout.limit = versionOutputLimit
	stderr.limit = versionOutputLimit
	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if runErr != nil {
		reason := lastNonEmptyLine(stderr.String())
		if reason == "" {
			reason = lastNonEmptyLine(stdout.String())
		}
		reason = trimBinaryPrefix(reason, path)
		reason = truncateRunes(reason, versionReasonLimit)
		if reason == "" {
			reason = runErr.Error()
		}
		return nil, codedError(CodeNotInstalled, "Spotify Soloist does not run: "+reason, runErr)
	}

	binary, parseErr := ParseVersion(stdout.String())
	if parseErr != nil {
		binary, parseErr = ParseVersion(stdout.String() + "\n" + stderr.String())
	}
	if parseErr != nil {
		return nil, codedError(CodeNotInstalled, "the executable is not a Spotify Soloist build", parseErr)
	}
	binary.Path = path
	return binary, nil
}

// FindInstalled locates a Soloist build. envPath (CADENCE_SOLOIST) wins and
// must be valid; otherwise the newest numeric programs directory is used,
// then PATH.
func FindInstalled(programsDir, envPath string) (*Binary, error) {
	if path := strings.TrimSpace(envPath); path != "" {
		return InspectBinary(context.Background(), path)
	}
	if path := newestInstalledPath(programsDir); path != "" {
		return InspectBinary(context.Background(), path)
	}
	path, err := exec.LookPath("soloist")
	if err != nil {
		return nil, ErrNotInstalled
	}
	return InspectBinary(context.Background(), path)
}

func newestInstalledPath(programsDir string) string {
	entries, err := os.ReadDir(programsDir)
	if err != nil {
		return ""
	}
	var newest int64
	var newestPath string
	found := false
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		built, err := strconv.ParseInt(entry.Name(), 10, 64)
		if err != nil {
			continue
		}
		candidate := filepath.Join(programsDir, entry.Name(), executableName(runtime.GOOS))
		info, err := os.Stat(candidate)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if !found || built > newest {
			found = true
			newest = built
			newestPath = candidate
		}
	}
	return newestPath
}

func executableName(goos string) string {
	if goos == "windows" {
		return "soloist.exe"
	}
	return "soloist"
}

type cappedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if remaining := b.limit - b.buf.Len(); remaining > 0 {
		if len(p) > remaining {
			_, _ = b.buf.Write(p[:remaining])
		} else {
			_, _ = b.buf.Write(p)
		}
	}
	return len(p), nil
}

func (b *cappedBuffer) String() string {
	return b.buf.String()
}

func lastNonEmptyLine(text string) string {
	lines := strings.Split(text, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		if line := strings.TrimSpace(lines[index]); line != "" {
			return line
		}
	}
	return ""
}

func trimBinaryPrefix(line, path string) string {
	if line == "" {
		return ""
	}
	if rest, ok := strings.CutPrefix(line, path); ok {
		if rest, ok := strings.CutPrefix(rest, ":"); ok {
			return strings.TrimSpace(rest)
		}
	}
	return line
}

func truncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}
