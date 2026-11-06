package soloist

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DownloadsURL is Spotify's official Soloist download and update page.
	DownloadsURL = "https://developer.spotify.com/documentation/soloist/reference/downloads-and-updates"
	// DashboardURL is where the user creates a Soloist API key.
	DashboardURL = "https://developer.spotify.com/dashboard"

	// archivesURL is Spotify's server for the archives DownloadsURL links to.
	archivesURL = "https://soloist-builds.spotifycdn.com/"
	// defaultMaxArchiveBytes and defaultMaxBinaryBytes keep a hostile or
	// broken answer from filling the disk.
	defaultMaxArchiveBytes = 64 << 20
	defaultMaxBinaryBytes  = 128 << 20
	installHTTPTimeout     = 5 * time.Minute
	maxRedirects           = 5
	// etagFileName stores Spotify's archive ETag beside an installed build,
	// so a later install can skip the download when nothing changed.
	etagFileName = "etag"
)

// archiveHosts are the only hosts a Soloist download may use.
var archiveHosts = []string{"spotifycdn.com", "spotify.com"}

// Progress reports one install stage. Downloaded and Total are meaningful
// during "downloading"; Total is 0 when the server did not send a length.
type Progress struct {
	Stage      string
	Downloaded int64
	Total      int64
}

// Installer downloads, verifies and installs Soloist builds.
type Installer struct {
	// mu serializes whole install sequences so two callers cannot stage the
	// same build timestamp at once.
	mu sync.Mutex

	programsDir     string
	client          *http.Client
	allowAnyHost    bool
	archiveBaseURL  string
	goos            string
	goarch          string
	now             func() time.Time
	maxArchiveBytes int64
	maxBinaryBytes  int64
}

// InstallerOptions configures an Installer.
type InstallerOptions struct {
	ProgramsDir string
	// HTTPClient overrides the client used for downloads. Unless AllowAnyHost
	// is set, the installer enforces the Spotify host and HTTPS allowlist on
	// the initial URL and on every redirect.
	HTTPClient *http.Client
	// AllowAnyHost disables the Spotify host allowlist. It exists for tests
	// that serve archives from a local httptest server and must not be set in
	// production.
	AllowAnyHost    bool
	GOOS            string
	GOARCH          string
	Now             func() time.Time
	MaxArchiveBytes int64
	MaxBinaryBytes  int64
}

// NewInstaller returns an installer with defaults applied. When HTTPClient is
// nil, the installer builds a client that only follows redirects to Spotify.
func NewInstaller(opts InstallerOptions) *Installer {
	installer := &Installer{
		programsDir:     opts.ProgramsDir,
		allowAnyHost:    opts.AllowAnyHost,
		archiveBaseURL:  archivesURL,
		goos:            opts.GOOS,
		goarch:          opts.GOARCH,
		now:             opts.Now,
		maxArchiveBytes: opts.MaxArchiveBytes,
		maxBinaryBytes:  opts.MaxBinaryBytes,
	}
	if installer.goos == "" {
		installer.goos = runtime.GOOS
	}
	if installer.goarch == "" {
		installer.goarch = runtime.GOARCH
	}
	if installer.now == nil {
		installer.now = time.Now
	}
	if installer.maxArchiveBytes <= 0 {
		installer.maxArchiveBytes = defaultMaxArchiveBytes
	}
	if installer.maxBinaryBytes <= 0 {
		installer.maxBinaryBytes = defaultMaxBinaryBytes
	}
	switch {
	case opts.HTTPClient == nil && opts.AllowAnyHost:
		installer.client = &http.Client{Timeout: installHTTPTimeout}
	case opts.HTTPClient == nil:
		installer.client = newArchiveHTTPClient()
	case opts.AllowAnyHost:
		installer.client = opts.HTTPClient
	default:
		installer.client = allowlistedClient(opts.HTTPClient)
	}
	return installer
}

// ArchiveName returns Spotify's archive for goos/goarch, or
// ErrUnsupportedPlatform.
func ArchiveName(goos, goarch string) (string, error) {
	switch goos + "/" + goarch {
	case "linux/amd64":
		return "soloist_release_x86_64.tar.gz", nil
	case "linux/arm64":
		return "soloist_release_arm64.tar.gz", nil
	case "linux/arm":
		return "soloist_release_arm32.tar.gz", nil
	case "darwin/arm64":
		return "soloist_release_osx_arm64.tar.gz", nil
	case "windows/amd64":
		return "soloist_release_windows_x86_64.zip", nil
	default:
		return "", ErrUnsupportedPlatform
	}
}

// Install downloads the newest build for this platform, verifies it, and
// installs it under its build timestamp. An already installed valid build for
// the same timestamp is reused instead of overwritten.
func (i *Installer) Install(ctx context.Context, progress func(Progress)) (*Binary, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	if i.programsDir == "" {
		return nil, codedError(CodeDownloadFailed, "the Soloist programs directory is not configured", nil)
	}
	archive, err := ArchiveName(i.goos, i.goarch)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(i.programsDir, 0o700); err != nil {
		return nil, codedError(CodeDownloadFailed, "cannot create the Soloist programs directory", err)
	}
	// Spotify ignores If-None-Match, so the ETag is compared from a HEAD
	// response before downloading the archive again.
	if known, etag, ok := i.knownBuild(ctx); ok && etag != "" {
		if unchanged, err := i.archiveUnchanged(ctx, archive, etag); err == nil && unchanged {
			return known, nil
		}
	}
	workDir, err := os.MkdirTemp(i.programsDir, ".soloist-download-")
	if err != nil {
		return nil, codedError(CodeDownloadFailed, "cannot create a Soloist download directory", err)
	}
	defer os.RemoveAll(workDir)

	archivePath := filepath.Join(workDir, archive)
	etag, err := i.download(ctx, archive, archivePath, progress)
	if err != nil {
		return nil, err
	}

	reportProgress(progress, Progress{Stage: "extracting"})
	extractDir := filepath.Join(workDir, "build")
	if err := os.MkdirAll(extractDir, 0o700); err != nil {
		return nil, codedError(CodeDownloadFailed, "cannot create a Soloist extraction directory", err)
	}
	extractedPath, err := extractArchive(archivePath, extractDir, i.goos, i.maxBinaryBytes)
	if err != nil {
		return nil, codedError(CodeDownloadFailed, "Spotify's Soloist archive is not usable", err)
	}
	if i.goos != "windows" {
		if err := os.Chmod(extractedPath, 0o755); err != nil {
			return nil, codedError(CodeDownloadFailed, "cannot make the downloaded Soloist runnable", err)
		}
	}

	reportProgress(progress, Progress{Stage: "verifying"})
	binary, err := InspectBinary(ctx, extractedPath)
	if err != nil {
		return nil, codedError(CodeDownloadFailed, "the downloaded program is not a valid Spotify Soloist build", err)
	}
	if binary.Expired(i.now()) {
		return nil, ErrExpired
	}

	built := strconv.FormatInt(binary.BuiltAt.Unix(), 10)
	targetDir := filepath.Join(i.programsDir, built)
	targetPath := filepath.Join(targetDir, executableName(i.goos))
	if existing, ok := i.reuseExisting(ctx, targetPath); ok {
		i.storeETag(targetDir, etag)
		return existing, nil
	}

	reportProgress(progress, Progress{Stage: "installing"})
	stageDir, err := os.MkdirTemp(i.programsDir, built+".tmp-")
	if err != nil {
		return nil, codedError(CodeDownloadFailed, "cannot create a Soloist install directory", err)
	}
	defer os.RemoveAll(stageDir)
	stagePath := filepath.Join(stageDir, executableName(i.goos))
	if err := os.Rename(extractedPath, stagePath); err != nil {
		return nil, codedError(CodeDownloadFailed, "cannot stage the downloaded Soloist", err)
	}
	if i.goos != "windows" {
		if err := os.Chmod(stagePath, 0o755); err != nil {
			return nil, codedError(CodeDownloadFailed, "cannot make the installed Soloist runnable", err)
		}
	}
	// reuseExisting rejected this path, so an existing target is invalid or
	// expired and may be replaced. A valid target was returned above.
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.RemoveAll(targetDir); err != nil {
			return nil, codedError(CodeDownloadFailed, "cannot replace the installed Soloist build", err)
		}
	}
	if err := os.Rename(stageDir, targetDir); err != nil {
		return nil, codedError(CodeDownloadFailed, "cannot install the downloaded Soloist", err)
	}
	i.storeETag(targetDir, etag)
	binary.Path = targetPath
	return binary, nil
}

// knownBuild returns the newest installed build and its stored ETag. A build
// without an ETag or an expired build does not allow the short-circuit.
func (i *Installer) knownBuild(ctx context.Context) (*Binary, string, bool) {
	path := newestInstalledPath(i.programsDir)
	if path == "" {
		return nil, "", false
	}
	binary, err := InspectBinary(ctx, path)
	if err != nil || binary.Expired(i.now()) {
		return nil, "", false
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(path), etagFileName))
	if err != nil {
		return binary, "", true
	}
	return binary, strings.TrimSpace(string(data)), true
}

// archiveUnchanged reports whether Spotify still serves the archive with the
// recorded ETag.
func (i *Installer) archiveUnchanged(ctx context.Context, archive, etag string) (bool, error) {
	request, err := i.archiveRequest(ctx, http.MethodHead, archive)
	if err != nil {
		return false, err
	}
	response, err := i.client.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified {
		return true, nil
	}
	if response.StatusCode != http.StatusOK {
		return false, nil
	}
	current := response.Header.Get("ETag")
	return current != "" && current == etag, nil
}

func (i *Installer) storeETag(dir, etag string) {
	etag = strings.TrimSpace(etag)
	if etag == "" || dir == "" {
		return
	}
	// Best effort: a missing ETag only costs a download next time.
	_ = os.WriteFile(filepath.Join(dir, etagFileName), []byte(etag), 0o600)
}

func (i *Installer) reuseExisting(ctx context.Context, path string) (*Binary, bool) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, false
	}
	existing, err := InspectBinary(ctx, path)
	if err != nil || existing.Expired(i.now()) {
		return nil, false
	}
	return existing, true
}

func (i *Installer) download(ctx context.Context, archive, destination string, progress func(Progress)) (string, error) {
	request, err := i.archiveRequest(ctx, http.MethodGet, archive)
	if err != nil {
		return "", err
	}
	response, err := i.client.Do(request)
	if err != nil {
		return "", codedError(CodeDownloadFailed, "cannot download Spotify Soloist", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", codedError(CodeDownloadFailed, fmt.Sprintf("Spotify answered the Soloist download with HTTP %d", response.StatusCode), nil)
	}
	etag := response.Header.Get("ETag")
	total := response.ContentLength
	if total < 0 {
		// An unknown length is reported as 0, not -1.
		total = 0
	}
	if total > i.maxArchiveBytes {
		return "", codedError(CodeDownloadFailed, "Spotify's Soloist archive is too large", nil)
	}
	file, err := os.Create(destination)
	if err != nil {
		return "", codedError(CodeDownloadFailed, "cannot save the Spotify Soloist download", err)
	}
	reportProgress(progress, Progress{Stage: "downloading", Total: total})
	writer := &progressWriter{writer: file, progress: progress, total: total}
	copied, copyErr := io.Copy(writer, io.LimitReader(response.Body, i.maxArchiveBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		return "", codedError(CodeDownloadFailed, "the Spotify Soloist download was interrupted", copyErr)
	}
	if closeErr != nil {
		return "", codedError(CodeDownloadFailed, "cannot save the Spotify Soloist download", closeErr)
	}
	if copied > i.maxArchiveBytes {
		return "", codedError(CodeDownloadFailed, "Spotify's Soloist archive is too large", nil)
	}
	reportProgress(progress, Progress{Stage: "downloading", Downloaded: copied, Total: total})
	return etag, nil
}

// archiveRequest builds one request for the platform's archive. Unless
// AllowAnyHost is set, the initial URL must be an HTTPS Spotify URL.
func (i *Installer) archiveRequest(ctx context.Context, method, archive string) (*http.Request, error) {
	rawURL := strings.TrimRight(i.archiveBaseURL, "/") + "/" + archive
	request, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, codedError(CodeDownloadFailed, "cannot prepare the Spotify Soloist download", err)
	}
	// Identity keeps the byte caps honest: no transparent decompression.
	request.Header.Set("Accept-Encoding", "identity")
	if !i.allowAnyHost {
		if err := checkSpotifyURL(request.URL); err != nil {
			return nil, codedError(CodeDownloadFailed, "Spotify Soloist may only be downloaded from Spotify", err)
		}
	}
	return request, nil
}

type progressWriter struct {
	writer   io.Writer
	progress func(Progress)
	total    int64
	written  int64
}

func (w *progressWriter) Write(p []byte) (int, error) {
	written, err := w.writer.Write(p)
	w.written += int64(written)
	reportProgress(w.progress, Progress{Stage: "downloading", Downloaded: w.written, Total: w.total})
	return written, err
}

func reportProgress(progress func(Progress), value Progress) {
	if progress != nil {
		progress(value)
	}
}

func newArchiveHTTPClient() *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		transport = transport.Clone()
	} else {
		transport = &http.Transport{Proxy: http.ProxyFromEnvironment}
	}
	return &http.Client{
		Timeout:       installHTTPTimeout,
		Transport:     transport,
		CheckRedirect: spotifyRedirectCheck(nil),
	}
}

// allowlistedClient wraps an injected client with the Spotify allowlist and
// the redirect cap, chaining any CheckRedirect the caller already set.
func allowlistedClient(client *http.Client) *http.Client {
	cloned := *client
	cloned.CheckRedirect = spotifyRedirectCheck(client.CheckRedirect)
	return &cloned
}

func spotifyRedirectCheck(original func(*http.Request, []*http.Request) error) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		if len(via) > maxRedirects {
			return errors.New("too many redirects")
		}
		if err := checkSpotifyURL(request.URL); err != nil {
			return err
		}
		if original != nil {
			return original(request, via)
		}
		return nil
	}
}

func checkSpotifyURL(target *url.URL) error {
	if target == nil {
		return errors.New("the URL is not a Spotify server")
	}
	if !strings.EqualFold(target.Scheme, "https") {
		return errors.New("the URL does not use HTTPS")
	}
	if !spotifyHostAllowed(target.Hostname()) {
		return errors.New("the host is not a Spotify server")
	}
	return nil
}

func spotifyHostAllowed(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" {
		return false
	}
	for _, allowed := range archiveHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

func extractArchive(archivePath, destination, goos string, maxBinary int64) (string, error) {
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return extractZip(archivePath, destination, executableName(goos), maxBinary)
	}
	return extractTarGz(archivePath, destination, executableName(goos), maxBinary)
}

func extractTarGz(archivePath, destination, target string, maxBinary int64) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if err := safeArchiveName(header.Name); err != nil {
			return "", err
		}
		if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
			return "", fmt.Errorf("archive member %q is a link", header.Name)
		}
		if filepath.Base(strings.ReplaceAll(header.Name, "\\", "/")) != target {
			continue
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return "", fmt.Errorf("archive member %q is not a regular file", header.Name)
		}
		if header.Size < 0 || header.Size > maxBinary {
			return "", fmt.Errorf("archive member %q is too large", header.Name)
		}
		outputPath := filepath.Join(destination, target)
		if err := writeExtracted(outputPath, reader, maxBinary); err != nil {
			return "", err
		}
		return outputPath, nil
	}
	return "", errors.New("the archive does not contain Soloist")
}

func extractZip(archivePath, destination, target string, maxBinary int64) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	for _, entry := range reader.File {
		if err := safeArchiveName(entry.Name); err != nil {
			return "", err
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("archive member %q is a link", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(strings.ReplaceAll(entry.Name, "\\", "/")) != target {
			continue
		}
		if !entry.Mode().IsRegular() {
			return "", fmt.Errorf("archive member %q is not a regular file", entry.Name)
		}
		if entry.UncompressedSize64 > uint64(maxBinary) {
			return "", fmt.Errorf("archive member %q is too large", entry.Name)
		}
		contents, err := entry.Open()
		if err != nil {
			return "", err
		}
		outputPath := filepath.Join(destination, target)
		writeErr := writeExtracted(outputPath, contents, maxBinary)
		closeErr := contents.Close()
		if writeErr != nil {
			return "", writeErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		return outputPath, nil
	}
	return "", errors.New("the archive does not contain Soloist")
}

func writeExtracted(path string, contents io.Reader, maxBinary int64) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(file, io.LimitReader(contents, maxBinary+1))
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return closeErr
	}
	if written > maxBinary {
		_ = os.Remove(path)
		return errors.New("the Soloist binary is too large")
	}
	return nil
}

func safeArchiveName(name string) error {
	if name == "" {
		return errors.New("archive member has an empty name")
	}
	normalized := strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(normalized, "/") {
		return fmt.Errorf("archive member %q has an absolute path", name)
	}
	if len(normalized) >= 2 && normalized[1] == ':' {
		return fmt.Errorf("archive member %q has an absolute path", name)
	}
	for _, part := range strings.Split(normalized, "/") {
		if part == ".." {
			return fmt.Errorf("archive member %q escapes the extraction directory", name)
		}
	}
	return nil
}
