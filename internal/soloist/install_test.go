package soloist

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type archiveMember struct {
	Name     string
	Body     string
	Mode     int64
	Typeflag byte
	Linkname string
	Size     int64
}

func tarGzArchive(t *testing.T, members []archiveMember) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, member := range members {
		mode := member.Mode
		if mode == 0 {
			mode = 0o755
		}
		typeflag := member.Typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		size := int64(len(member.Body))
		if member.Size > 0 {
			size = member.Size
		}
		header := &tar.Header{
			Name:     member.Name,
			Mode:     mode,
			Typeflag: typeflag,
			Size:     size,
			Linkname: member.Linkname,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("tar header: %v", err)
		}
		if typeflag == tar.TypeReg || typeflag == tar.TypeRegA {
			if member.Size > 0 {
				if _, err := tarWriter.Write(bytes.Repeat([]byte("x"), int(member.Size))); err != nil {
					t.Fatalf("tar body: %v", err)
				}
			} else if _, err := tarWriter.Write([]byte(member.Body)); err != nil {
				t.Fatalf("tar body: %v", err)
			}
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buffer.Bytes()
}

func zipArchive(t *testing.T, members []archiveMember) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, member := range members {
		header := &zip.FileHeader{Name: member.Name, Method: zip.Deflate}
		if member.Typeflag == tar.TypeSymlink {
			header.SetMode(os.ModeSymlink | 0o777)
		} else {
			header.SetMode(0o755)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("zip header: %v", err)
		}
		if _, err := entry.Write([]byte(member.Body)); err != nil {
			t.Fatalf("zip body: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buffer.Bytes()
}

func TestArchiveName(t *testing.T) {
	tests := []struct {
		goos, goarch string
		want         string
		wantErr      bool
	}{
		{goos: "linux", goarch: "amd64", want: "soloist_release_x86_64.tar.gz"},
		{goos: "linux", goarch: "arm64", want: "soloist_release_arm64.tar.gz"},
		{goos: "linux", goarch: "arm", want: "soloist_release_arm32.tar.gz"},
		{goos: "darwin", goarch: "arm64", want: "soloist_release_osx_arm64.tar.gz"},
		{goos: "windows", goarch: "amd64", want: "soloist_release_windows_x86_64.zip"},
		{goos: "darwin", goarch: "amd64", wantErr: true},
		{goos: "windows", goarch: "arm64", wantErr: true},
		{goos: "linux", goarch: "386", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.goos+"_"+test.goarch, func(t *testing.T) {
			name, err := ArchiveName(test.goos, test.goarch)
			if test.wantErr {
				if !errors.Is(err, ErrUnsupportedPlatform) {
					t.Fatalf("error = %v, want ErrUnsupportedPlatform", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("archive: %v", err)
			}
			if name != test.want {
				t.Fatalf("name = %q, want %q", name, test.want)
			}
		})
	}
}

func TestExtractionRejectsUnsafeArchives(t *testing.T) {
	script := "#!/bin/sh\necho soloist 1.0.0 build 100\n"
	tests := []struct {
		name    string
		members []archiveMember
		max     int64
	}{
		{name: "parent traversal", members: []archiveMember{{Name: "../soloist", Body: script}}, max: 4096},
		{name: "absolute path", members: []archiveMember{{Name: "/tmp/soloist", Body: script}}, max: 4096},
		{name: "symlink", members: []archiveMember{{Name: "soloist", Typeflag: tar.TypeSymlink, Linkname: "/bin/sh"}}, max: 4096},
		{name: "hard link", members: []archiveMember{{Name: "soloist", Typeflag: tar.TypeLink, Linkname: "/etc/passwd"}}, max: 4096},
		{name: "oversized", members: []archiveMember{{Name: "soloist", Size: 8192}}, max: 4096},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			archivePath := filepath.Join(dir, "archive.tar.gz")
			if err := os.WriteFile(archivePath, tarGzArchive(t, test.members), 0o600); err != nil {
				t.Fatalf("write archive: %v", err)
			}
			if _, err := extractArchive(archivePath, dir, "linux", test.max); err == nil {
				t.Fatal("expected the archive to be rejected")
			}
		})
	}
}

func TestExtractionFindsOnlyTheBinary(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "archive.tar.gz")
	body := "#!/bin/sh\necho soloist 1.0.0 build 100\n"
	members := []archiveMember{
		{Name: "docs/README", Body: "readme"},
		{Name: "soloist", Body: body, Mode: 0o755},
		{Name: "other", Body: "other"},
	}
	if err := os.WriteFile(archivePath, tarGzArchive(t, members), 0o600); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	binaryPath, err := extractArchive(archivePath, dir, "linux", 4096)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if filepath.Base(binaryPath) != "soloist" {
		t.Fatalf("path = %q", binaryPath)
	}
	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("binary is not executable: %o", info.Mode().Perm())
	}
}

func TestZipExtractionRejectsSymlinks(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "archive.zip")
	if err := os.WriteFile(archivePath, zipArchive(t, []archiveMember{{Name: "soloist", Typeflag: tar.TypeSymlink, Linkname: "/bin/sh"}}), 0o600); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if _, err := extractArchive(archivePath, dir, "windows", 4096); err == nil {
		t.Fatal("expected the symlink to be rejected")
	}
}

func TestSpotifyHostAllowlist(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{host: "spotifycdn.com", want: true},
		{host: "soloist-builds.spotifycdn.com", want: true},
		{host: "a.b.spotifycdn.com", want: true},
		{host: "spotify.com", want: true},
		{host: "developer.spotify.com", want: true},
		{host: "SPOTIFYCDN.COM", want: true},
		{host: "evilspotify.com", want: false},
		{host: "spotifycdn.com.evil.com", want: false},
		{host: "example.com", want: false},
		{host: "", want: false},
	}
	for _, test := range tests {
		t.Run(test.host, func(t *testing.T) {
			if got := spotifyHostAllowed(test.host); got != test.want {
				t.Fatalf("spotifyHostAllowed(%q) = %v, want %v", test.host, got, test.want)
			}
		})
	}
}

func TestArchiveClientRedirectAllowlist(t *testing.T) {
	client := newArchiveHTTPClient()
	if client.CheckRedirect == nil {
		t.Fatal("the default client has no redirect check")
	}
	if client.Timeout != installHTTPTimeout {
		t.Fatalf("timeout = %v, want %v", client.Timeout, installHTTPTimeout)
	}
	request := func(host string) *http.Request {
		return &http.Request{URL: &url.URL{Scheme: "https", Host: host}}
	}
	via := []*http.Request{request("soloist-builds.spotifycdn.com")}
	tests := []struct {
		host    string
		wantErr bool
	}{
		{host: "soloist-builds.spotifycdn.com"},
		{host: "download.spotify.com"},
		{host: "evil.example.com", wantErr: true},
		{host: "spotifycdn.com.evil.com", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.host, func(t *testing.T) {
			err := client.CheckRedirect(request(test.host), via)
			if test.wantErr != (err != nil) {
				t.Fatalf("redirect to %q error = %v, wantErr %v", test.host, err, test.wantErr)
			}
		})
	}
	tooMany := make([]*http.Request, maxRedirects+1)
	for index := range tooMany {
		tooMany[index] = request("soloist-builds.spotifycdn.com")
	}
	if err := client.CheckRedirect(request("soloist-builds.spotifycdn.com"), tooMany); err == nil {
		t.Fatal("too many redirects were accepted")
	}
}

func TestInstallFromServer(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	built := int64(1792756912)
	body := versionScript("1.3.10.12", built)
	archive := tarGzArchive(t, []archiveMember{{Name: "soloist", Body: body, Mode: 0o755}})
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	programs := t.TempDir()
	installer := NewInstaller(InstallerOptions{
		ProgramsDir:  programs,
		HTTPClient:   server.Client(),
		AllowAnyHost: true,
		GOOS:         "linux",
		GOARCH:       "amd64",
		Now:          func() time.Time { return time.Unix(built+86400, 0) },
	})
	installer.archiveBaseURL = server.URL + "/"

	var stages []string
	var lastDownload Progress
	binary, err := installer.Install(t.Context(), func(progress Progress) {
		stages = append(stages, progress.Stage)
		if progress.Stage == "downloading" {
			lastDownload = progress
		}
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if binary.Version != "1.3.10.12" || binary.BuiltAt.Unix() != built {
		t.Fatalf("binary = %+v", binary)
	}
	wantPath := filepath.Join(programs, "1792756912", "soloist")
	if binary.Path != wantPath {
		t.Fatalf("path = %q, want %q", binary.Path, wantPath)
	}
	info, err := os.Stat(wantPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("installed binary is not executable: %o", info.Mode().Perm())
	}
	if hits.Load() != 1 {
		t.Fatalf("downloads = %d, want 1", hits.Load())
	}
	if lastDownload.Total != int64(len(archive)) || lastDownload.Downloaded != int64(len(archive)) {
		t.Fatalf("download progress = %+v, archive = %d", lastDownload, len(archive))
	}
	for _, stage := range []string{"downloading", "extracting", "verifying", "installing"} {
		if !containsStage(stages, stage) {
			t.Fatalf("stage %q missing from %v", stage, stages)
		}
	}
}

func TestInstallReusesExistingBuildWithoutOverwriting(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	built := int64(1792756912)
	body := versionScript("1.3.10.12", built)
	archive := tarGzArchive(t, []archiveMember{{Name: "soloist", Body: body, Mode: 0o755}})
	const etag = `"build-1792756912"`
	var downloads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", etag)
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		downloads.Add(1)
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	programs := t.TempDir()
	installer := NewInstaller(InstallerOptions{
		ProgramsDir:  programs,
		HTTPClient:   server.Client(),
		AllowAnyHost: true,
		GOOS:         "linux",
		GOARCH:       "amd64",
		Now:          func() time.Time { return time.Unix(built+86400, 0) },
	})
	installer.archiveBaseURL = server.URL + "/"
	if _, err := installer.Install(t.Context(), nil); err != nil {
		t.Fatalf("first install: %v", err)
	}
	storedETag, err := os.ReadFile(filepath.Join(programs, "1792756912", "etag"))
	if err != nil || strings.TrimSpace(string(storedETag)) != etag {
		t.Fatalf("stored etag = %q, err = %v", storedETag, err)
	}

	installed := filepath.Join(programs, "1792756912", "soloist")
	marker := body + "# preserved\n"
	if err := os.WriteFile(installed, []byte(marker), 0o755); err != nil {
		t.Fatalf("mark: %v", err)
	}
	binary, err := installer.Install(t.Context(), nil)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if binary.Path != installed {
		t.Fatalf("path = %q", binary.Path)
	}
	if got := downloads.Load(); got != 1 {
		t.Fatalf("downloads = %d, want 1: a matching ETag must skip the download", got)
	}
	content, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(content) != marker {
		t.Fatal("the existing build was overwritten")
	}
}

func TestInstallRefusesExpiredBuild(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	built := int64(1792756912)
	body := versionScript("1.0.0", built)
	archive := tarGzArchive(t, []archiveMember{{Name: "soloist", Body: body, Mode: 0o755}})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	programs := t.TempDir()
	installer := NewInstaller(InstallerOptions{
		ProgramsDir:  programs,
		HTTPClient:   server.Client(),
		AllowAnyHost: true,
		GOOS:         "linux",
		GOARCH:       "amd64",
		Now:          func() time.Time { return time.Unix(built+91*24*60*60, 0) },
	})
	installer.archiveBaseURL = server.URL + "/"
	if _, err := installer.Install(t.Context(), nil); !errors.Is(err, ErrExpired) {
		t.Fatalf("error = %v, want ErrExpired", err)
	}
	if _, err := os.Stat(filepath.Join(programs, "1792756912")); !os.IsNotExist(err) {
		t.Fatalf("expired build was installed: %v", err)
	}
}

func TestInstallRejectsOversizedArchive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), 100))
	}))
	defer server.Close()

	installer := NewInstaller(InstallerOptions{
		ProgramsDir:     t.TempDir(),
		HTTPClient:      server.Client(),
		AllowAnyHost:    true,
		GOOS:            "linux",
		GOARCH:          "amd64",
		MaxArchiveBytes: 10,
	})
	installer.archiveBaseURL = server.URL + "/"
	_, err := installer.Install(t.Context(), nil)
	if err == nil || CodeOf(err) != CodeDownloadFailed {
		t.Fatalf("error = %v, want CodeDownloadFailed", err)
	}
}

func containsStage(stages []string, want string) bool {
	for _, stage := range stages {
		if stage == want {
			return true
		}
	}
	return false
}

func TestInstallerInjectedClientEnforcesSpotifyAllowlist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	locked := NewInstaller(InstallerOptions{
		ProgramsDir: t.TempDir(),
		HTTPClient:  server.Client(),
		GOOS:        "linux",
		GOARCH:      "amd64",
	})
	locked.archiveBaseURL = server.URL + "/"
	if _, err := locked.archiveRequest(t.Context(), http.MethodGet, "soloist_release_x86_64.tar.gz"); err == nil || CodeOf(err) != CodeDownloadFailed {
		t.Fatalf("non-Spotify initial URL error = %v, want CodeDownloadFailed", err)
	}

	request := func(raw string) *http.Request {
		parsed, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		return &http.Request{URL: parsed}
	}
	via := []*http.Request{request("https://soloist-builds.spotifycdn.com/archive")}
	if err := locked.client.CheckRedirect(request("https://evil.example/archive"), via); err == nil {
		t.Fatal("a non-Spotify redirect was accepted")
	}
	if err := locked.client.CheckRedirect(request("http://soloist-builds.spotifycdn.com/archive"), via); err == nil {
		t.Fatal("an HTTP redirect was accepted")
	}
	if err := locked.client.CheckRedirect(request("https://soloist-builds.spotifycdn.com/archive"), via); err != nil {
		t.Fatalf("a Spotify HTTPS redirect was rejected: %v", err)
	}
	tooMany := make([]*http.Request, maxRedirects+1)
	for index := range tooMany {
		tooMany[index] = via[0]
	}
	if err := locked.client.CheckRedirect(request("https://soloist-builds.spotifycdn.com/archive"), tooMany); err == nil {
		t.Fatal("too many redirects were accepted")
	}

	sentinel := errors.New("caller redirect check")
	injected := server.Client()
	injected.CheckRedirect = func(*http.Request, []*http.Request) error { return sentinel }
	chained := NewInstaller(InstallerOptions{
		ProgramsDir: t.TempDir(),
		HTTPClient:  injected,
		GOOS:        "linux",
		GOARCH:      "amd64",
	})
	if err := chained.client.CheckRedirect(request("https://soloist-builds.spotifycdn.com/archive"), via); !errors.Is(err, sentinel) {
		t.Fatalf("caller CheckRedirect was not chained: %v", err)
	}

	open := NewInstaller(InstallerOptions{
		ProgramsDir:  t.TempDir(),
		HTTPClient:   server.Client(),
		AllowAnyHost: true,
		GOOS:         "linux",
		GOARCH:       "amd64",
	})
	open.archiveBaseURL = server.URL + "/"
	if _, err := open.archiveRequest(t.Context(), http.MethodGet, "soloist_release_x86_64.tar.gz"); err != nil {
		t.Fatalf("AllowAnyHost rejected the test server: %v", err)
	}
}

func TestArchiveURLRequiresHTTPS(t *testing.T) {
	tests := []struct {
		raw     string
		wantErr bool
	}{
		{raw: "https://soloist-builds.spotifycdn.com/archive"},
		{raw: "https://developer.spotify.com/archive"},
		{raw: "http://soloist-builds.spotifycdn.com/archive", wantErr: true},
		{raw: "ftp://soloist-builds.spotifycdn.com/archive", wantErr: true},
		{raw: "https://evil.example/archive", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.raw, func(t *testing.T) {
			parsed, err := url.Parse(test.raw)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := checkSpotifyURL(parsed) != nil; got != test.wantErr {
				t.Fatalf("checkSpotifyURL(%q) error = %v, wantErr %v", test.raw, checkSpotifyURL(parsed), test.wantErr)
			}
		})
	}
	if checkSpotifyURL(nil) == nil {
		t.Fatal("nil URL was accepted")
	}
}
