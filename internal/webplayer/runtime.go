package webplayer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"

	"cadence/internal/webplayer/helper"
)

const (
	// runtimeMarkerName marks a complete, verified runtime install. Its
	// content is the SHA-256 of the archive it came from.
	runtimeMarkerName = ".cadence-runtime-ok"

	defaultMaxDownloadBytes = 512 << 20
	defaultMaxExtractBytes  = 1 << 30
	defaultMaxFiles         = 20000

	installHTTPTimeout = 10 * time.Minute
	downloadRedirects  = 5
	// downloadSizeTolerance allows a small difference between the manifest
	// size and the served archive, but not a substantially different file.
	downloadSizeTolerance = 1 << 20
)

// downloadHosts are the only suffix domains a runtime download may use.
var downloadHosts = []string{"github.com", "githubusercontent.com"}

// RuntimeOptions configures a Runtime. Zero values select production
// defaults: the XDG cache directory, the embedded manifest, the current
// GOOS/GOARCH and generous size caps.
type RuntimeOptions struct {
	Root             string
	HTTPClient       *http.Client
	AllowAnyHost     bool
	GOOS             string
	GOARCH           string
	Manifest         *Manifest
	MaxDownloadBytes int64
	MaxExtractBytes  int64
	MaxFiles         int
	Now              func() time.Time
}

// Install is one usable runtime installation.
type Install struct {
	Version    string
	Dir        string
	Executable string
	HelperDir  string
}

// Progress reports one install stage. Downloaded and Total are meaningful
// during "downloading"; Total is the manifest size.
type Progress struct {
	Stage      string
	Downloaded int64
	Total      int64
}

// Runtime downloads, verifies and owns the Chromium runtime. It is safe for
// concurrent use; installs are serialized.
type Runtime struct {
	mu sync.Mutex

	root             string
	goos             string
	goarch           string
	allowAnyHost     bool
	client           *http.Client
	manifest         *Manifest
	manifestErr      error
	maxDownloadBytes int64
	maxExtractBytes  int64
	maxFiles         int
	// now is reserved for deterministic timestamps in later phases (token
	// expiry, OAuth). Installs rely on filesystem mtimes.
	now func() time.Time
}

// NewRuntime returns a runtime with defaults applied. A nil HTTPClient gets a
// client that enforces the GitHub HTTPS allowlist on every redirect, unless
// AllowAnyHost is set (tests only).
func NewRuntime(opts RuntimeOptions) *Runtime {
	r := &Runtime{
		root:             opts.Root,
		goos:             opts.GOOS,
		goarch:           opts.GOARCH,
		allowAnyHost:     opts.AllowAnyHost,
		maxDownloadBytes: opts.MaxDownloadBytes,
		maxExtractBytes:  opts.MaxExtractBytes,
		maxFiles:         opts.MaxFiles,
		now:              opts.Now,
	}
	if r.root == "" {
		r.root = defaultRoot()
	}
	if r.goos == "" {
		r.goos = runtime.GOOS
	}
	if r.goarch == "" {
		r.goarch = runtime.GOARCH
	}
	if r.now == nil {
		r.now = time.Now
	}
	if r.maxDownloadBytes <= 0 {
		r.maxDownloadBytes = defaultMaxDownloadBytes
	}
	if r.maxExtractBytes <= 0 {
		r.maxExtractBytes = defaultMaxExtractBytes
	}
	if r.maxFiles <= 0 {
		r.maxFiles = defaultMaxFiles
	}

	switch {
	case opts.Manifest != nil:
		r.manifest = opts.Manifest
		r.manifestErr = opts.Manifest.Validate()
	default:
		r.manifest, r.manifestErr = loadEmbeddedManifest()
	}

	switch {
	case opts.HTTPClient == nil && r.allowAnyHost:
		r.client = &http.Client{Timeout: installHTTPTimeout}
	case opts.HTTPClient == nil:
		r.client = newDownloadClient()
	case r.allowAnyHost:
		r.client = opts.HTTPClient
	default:
		r.client = allowlistedClient(opts.HTTPClient)
	}
	return r
}

// Ensure returns a usable runtime, installing it first when necessary. An
// already installed runtime is only materialized into the helper directory;
// no network request is made.
func (r *Runtime) Ensure(ctx context.Context) (Install, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if install, ok := r.Installed(); ok {
		return r.materialize(install)
	}
	return r.Install(ctx, nil)
}

// Installed reports whether a verified runtime is present without touching
// the network or materializing the helper.
func (r *Runtime) Installed() (Install, bool) {
	platform, err := r.platform()
	if err != nil {
		return Install{}, false
	}
	dir := r.installDir(platform)
	if !r.trusted(dir, platform) {
		return Install{}, false
	}
	return r.installAt(dir, platform), true
}

// Install downloads, verifies and atomically installs the runtime. It is
// serialized: concurrent callers share one install.
func (r *Runtime) Install(ctx context.Context, progress func(Progress)) (Install, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}
	platform, err := r.platform()
	if err != nil {
		return Install{}, err
	}
	if r.root == "" {
		return Install{}, codedError(CodeInstallFailed, "the web player runtime directory is not configured", nil)
	}

	targetDir := r.installDir(platform)
	if r.trusted(targetDir, platform) {
		return r.finishInstall(targetDir, platform)
	}

	if err := os.MkdirAll(r.runtimeDir(), 0o700); err != nil {
		return Install{}, codedError(CodeInstallFailed, "cannot create the web player runtime directory", err)
	}
	stagingDir := filepath.Join(r.root, "tmp")
	if err := os.MkdirAll(stagingDir, 0o700); err != nil {
		return Install{}, codedError(CodeInstallFailed, "cannot create the web player staging directory", err)
	}
	workDir, err := os.MkdirTemp(stagingDir, "install-")
	if err != nil {
		return Install{}, codedError(CodeInstallFailed, "cannot create a web player staging directory", err)
	}
	defer os.RemoveAll(workDir)

	zipPath := filepath.Join(workDir, "runtime.zip")
	if err := r.download(ctx, platform, zipPath, progress); err != nil {
		return Install{}, err
	}

	reportProgress(progress, Progress{Stage: "extracting"})
	extractDir := filepath.Join(workDir, "runtime")
	limits := extractLimits{
		MaxFileBytes:  r.maxExtractBytes,
		MaxTotalBytes: r.maxExtractBytes,
		MaxFiles:      r.maxFiles,
	}
	if err := extractRuntimeZip(zipPath, extractDir, platform.Entry, limits); err != nil {
		return Install{}, codedError(CodeInstallFailed, "the downloaded web player runtime is not usable", err)
	}

	if err := os.WriteFile(filepath.Join(extractDir, runtimeMarkerName), []byte(platform.SHA256), 0o600); err != nil {
		return Install{}, codedError(CodeInstallFailed, "cannot mark the web player runtime as verified", err)
	}

	reportProgress(progress, Progress{Stage: "installing"})
	// Another process may have completed the same version while this one was
	// downloading. Prefer that verified install over replacing it.
	if r.trusted(targetDir, platform) {
		return r.finishInstall(targetDir, platform)
	}
	if _, err := os.Stat(targetDir); err == nil {
		// Only a verified install is trusted, so this tree is incomplete or
		// corrupted and must be replaced.
		if err := os.RemoveAll(targetDir); err != nil {
			return Install{}, codedError(CodeInstallFailed, "cannot replace the installed web player runtime", err)
		}
	}
	if err := os.Rename(extractDir, targetDir); err != nil {
		return Install{}, codedError(CodeInstallFailed, "cannot install the web player runtime", err)
	}
	if !r.trusted(targetDir, platform) {
		return Install{}, codedError(CodeInstallFailed, "the installed web player runtime could not be verified", nil)
	}
	return r.finishInstall(targetDir, platform)
}

// Prune keeps the named runtime directory and the newest other one, removing
// anything else. It is best-effort and never removes files.
func (r *Runtime) Prune(keep string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.prune(keep)
}

// download writes the runtime archive to destination while hashing it and
// enforcing the manifest checksum and size.
func (r *Runtime) download(ctx context.Context, platform Platform, destination string, progress func(Progress)) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, platform.URL, nil)
	if err != nil {
		return codedError(CodeDownloadFailed, "cannot prepare the web player runtime download", err)
	}
	// Identity keeps the byte caps honest: no transparent decompression.
	request.Header.Set("Accept-Encoding", "identity")
	if !r.allowAnyHost {
		if err := checkDownloadURL(request.URL); err != nil {
			return codedError(CodeDownloadFailed, "the web player runtime may only be downloaded from GitHub", err)
		}
	}

	response, err := r.client.Do(request)
	if err != nil {
		return codedError(CodeDownloadFailed, "cannot download the web player runtime", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return codedError(CodeDownloadFailed, fmt.Sprintf("the web player runtime download answered with HTTP %d", response.StatusCode), nil)
	}
	if response.ContentLength > platform.Size+downloadSizeTolerance {
		return codedError(CodeDownloadFailed, "the web player runtime download is larger than expected", nil)
	}

	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return codedError(CodeDownloadFailed, "cannot save the web player runtime download", err)
	}
	hasher := sha256.New()
	writer := &progressWriter{
		writer:   io.MultiWriter(file, hasher),
		progress: progress,
		total:    platform.Size,
	}
	reportProgress(progress, Progress{Stage: "downloading", Total: platform.Size})
	copied, copyErr := io.Copy(writer, io.LimitReader(response.Body, r.maxDownloadBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		return codedError(CodeDownloadFailed, "the web player runtime download was interrupted", copyErr)
	}
	if closeErr != nil {
		return codedError(CodeDownloadFailed, "cannot save the web player runtime download", closeErr)
	}
	if copied > r.maxDownloadBytes {
		return codedError(CodeDownloadFailed, "the web player runtime download is too large", nil)
	}
	if copied > platform.Size+downloadSizeTolerance || copied < platform.Size-downloadSizeTolerance {
		return codedError(CodeDownloadFailed, "the web player runtime download has an unexpected size", nil)
	}
	reportProgress(progress, Progress{Stage: "downloading", Downloaded: copied, Total: platform.Size})

	if !validSHA256(platform.SHA256) {
		return ErrInvalidManifest
	}
	sum := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(sum, platform.SHA256) {
		return ErrChecksumMismatch
	}
	return nil
}

func (r *Runtime) finishInstall(dir string, platform Platform) (Install, error) {
	install := r.installAt(dir, platform)
	materialized, err := r.materialize(install)
	if err != nil {
		return Install{}, err
	}
	// Pruning is best-effort: a broken old tree must never fail an install.
	_ = r.prune(filepath.Base(dir))
	return materialized, nil
}

func (r *Runtime) materialize(install Install) (Install, error) {
	if err := helper.Materialize(install.HelperDir); err != nil {
		return Install{}, codedError(CodeInstallFailed, "cannot prepare the embedded web player helper", err)
	}
	return install, nil
}

func (r *Runtime) installAt(dir string, platform Platform) Install {
	return Install{
		Version:    platform.Version,
		Dir:        dir,
		Executable: filepath.Join(dir, platform.Entry),
		HelperDir:  r.helperDir(),
	}
}

// trusted reports whether dir holds a complete install for platform: the
// marker carries the expected SHA-256 and the entry is a regular file.
func (r *Runtime) trusted(dir string, platform Platform) bool {
	marker, err := os.ReadFile(filepath.Join(dir, runtimeMarkerName))
	if err != nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(string(marker)), platform.SHA256) {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, platform.Entry))
	return err == nil && info.Mode().IsRegular()
}

// prune keeps keep's directory and the newest other runtime directory.
func (r *Runtime) prune(keep string) error {
	keep = filepath.Base(filepath.Clean(keep))
	if keep == "." || keep == string(filepath.Separator) {
		keep = ""
	}
	dir := r.runtimeDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	type candidate struct {
		name    string
		modTime time.Time
	}
	candidates := make([]candidate, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() == keep || !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{name: entry.Name(), modTime: info.ModTime()})
	}
	if len(candidates) <= 1 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].modTime.Equal(candidates[j].modTime) {
			return candidates[i].name > candidates[j].name
		}
		return candidates[i].modTime.After(candidates[j].modTime)
	})
	for _, stale := range candidates[1:] {
		_ = os.RemoveAll(filepath.Join(dir, stale.name))
	}
	return nil
}

func (r *Runtime) platform() (Platform, error) {
	if r.manifestErr != nil {
		return Platform{}, r.manifestErr
	}
	return r.manifest.Resolve(r.goos, r.goarch)
}

func (r *Runtime) runtimeDir() string {
	return filepath.Join(r.root, "runtime")
}

func (r *Runtime) helperDir() string {
	return filepath.Join(r.root, "helper")
}

func (r *Runtime) installDir(platform Platform) string {
	return filepath.Join(r.runtimeDir(), fmt.Sprintf("%s-%s-%s", platform.Version, r.goos, r.goarch))
}

// defaultRoot is the XDG cache location. It is resolved once when the runtime
// is built so later environment changes cannot move an installed runtime.
func defaultRoot() string {
	base := xdg.CacheHome
	if base == "" {
		if dir, err := os.UserCacheDir(); err == nil {
			base = dir
		}
	}
	if base == "" {
		// NewRuntime cannot report an error. This only happens when HOME and
		// the XDG variables are all unset; the caller should set Root.
		base = os.TempDir()
	}
	return filepath.Join(base, "cadence", "webplayer")
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

func newDownloadClient() *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		transport = transport.Clone()
	} else {
		transport = &http.Transport{Proxy: http.ProxyFromEnvironment}
	}
	return &http.Client{
		Timeout:       installHTTPTimeout,
		Transport:     transport,
		CheckRedirect: downloadRedirectCheck(nil),
	}
}

// allowlistedClient wraps an injected client with the download allowlist and
// the redirect cap, chaining any CheckRedirect the caller already set.
func allowlistedClient(client *http.Client) *http.Client {
	cloned := *client
	cloned.CheckRedirect = downloadRedirectCheck(client.CheckRedirect)
	return &cloned
}

func downloadRedirectCheck(original func(*http.Request, []*http.Request) error) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		if len(via) > downloadRedirects {
			return errors.New("too many redirects")
		}
		if err := checkDownloadURL(request.URL); err != nil {
			return err
		}
		if original != nil {
			return original(request, via)
		}
		return nil
	}
}

func checkDownloadURL(target *url.URL) error {
	if target == nil {
		return errors.New("the URL is not a GitHub download")
	}
	if !strings.EqualFold(target.Scheme, "https") {
		return errors.New("the download does not use HTTPS")
	}
	if target.User != nil {
		return errors.New("the download URL embeds credentials")
	}
	if !downloadHostAllowed(target.Hostname()) {
		return errors.New("the host is not a GitHub download host")
	}
	return nil
}

func downloadHostAllowed(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" {
		return false
	}
	for _, allowed := range downloadHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}
