package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cadence/internal/artwork"
	"cadence/internal/buildinfo"
	"cadence/internal/catalog"
	"cadence/internal/developer"
	"cadence/internal/diagnostics"
	"cadence/internal/engine"
	"cadence/internal/lyrics"
	"cadence/internal/platform"
	"cadence/internal/playback"
	"cadence/internal/settings"
	"cadence/internal/skins"
	"cadence/internal/soloist"
	"cadence/internal/visualizer"
	audioapi "cadence/internal/wailsapi/audio"
	lyricsapi "cadence/internal/wailsapi/lyrics"
	playerapi "cadence/internal/wailsapi/player"
	skinsapi "cadence/internal/wailsapi/skins"
	visualizerapi "cadence/internal/wailsapi/visualizer"
	"cadence/internal/webplayer"

	"github.com/adrg/xdg"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Frontend assets are embedded into the binary by Wails. The directory is
// produced by `npm run build` (or by `wails3 build`).
//
//go:embed all:frontend/dist
var assets embed.FS

const (
	appName = "Cadence"
	appID   = "io.github.ivandec0.cadence"

	// The webview keeps a 420x220 logical layout. The native window is scaled
	// around that design size; the frontend adjusts the webview zoom when the
	// user resizes the window.
	uiScale               = 2.0
	minUiScale            = 1.25
	maxUiScale            = 3.0
	logicalWidth          = 420
	logicalHeight         = 220
	expandedLogicalHeight = 460

	contentSecurityPolicy = "default-src 'self'; " +
		"script-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob:; " +
		"font-src 'self'; " +
		"connect-src 'self'; " +
		"media-src 'none'; " +
		"object-src 'none'; " +
		"base-uri 'none'; " +
		"form-action 'none'; " +
		"frame-ancestors 'none'"
)

func init() {
	application.RegisterEvent[playback.PlayerState](playerapi.StateEvent)
	application.RegisterEvent[skins.SkinCatalog](skins.ChangedEvent)
	application.RegisterEvent[engine.AudioState](audioapi.AudioEvent)
	application.RegisterEvent[string](platform.BrowseNavigateEvent)
	application.RegisterEvent[visualizer.Frame](visualizerapi.FrameEvent)
}

func main() {
	safeMode := flag.Bool("safe-mode", false, "start with the built-in skin and disable third-party skins")
	demoMode := flag.Bool("demo", false, "use the simulated player instead of Spotify")
	devSkins := flag.String("skins-dev", "", "directory containing development skins")
	diagnosticsEnabled := flag.Bool("diagnostics", false, "expose local diagnostics and pprof on a loopback port")
	printVersion := flag.Bool("version", false, "print the version and exit")
	webLogin := flag.Bool("web-login", false, "connect the Spotify web player account and exit")
	webLogout := flag.Bool("web-logout", false, "disconnect the Spotify web player account and exit")
	webInstall := flag.Bool("web-install", false, "install the web player runtime and exit")
	flag.Parse()

	if *printVersion {
		fmt.Printf("Cadence %s\n", buildinfo.String())
		return
	}
	if *webLogin || *webLogout || *webInstall {
		if err := runWebCommand(*webLogin, *webLogout, *webInstall); err != nil {
			log.Fatalf("web player: %v", err)
		}
		return
	}

	if *demoMode {
		log.Println("demo mode: using the simulated player")
	}
	if *safeMode {
		log.Println("safe mode: third-party skins are disabled for this session")
	}

	devDir := *devSkins
	if devDir == "" {
		if info, err := os.Stat("skins-dev"); err == nil && info.IsDir() {
			devDir = "skins-dev"
		}
	}
	if devDir == "" {
		if executable, err := os.Executable(); err == nil {
			candidates := []string{
				filepath.Join(filepath.Dir(executable), "skins"),
				filepath.Join(filepath.Dir(executable), "..", "share", "cadence", "skins"),
			}
			for _, candidate := range candidates {
				if info, err := os.Stat(candidate); err == nil && info.IsDir() {
					devDir = candidate
					break
				}
			}
		}
	}
	if devDir != "" {
		absolute, err := filepath.Abs(devDir)
		if err != nil {
			log.Fatalf("resolve development skins directory: %v", err)
		}
		devDir = absolute
	}

	settingsStore := settings.NewStore(filepath.Join(xdg.ConfigHome, "cadence", "config.json"))
	savedSettings, err := settingsStore.Load()
	if err != nil {
		log.Printf("load settings: %v; using defaults", err)
	}
	engineSettings := savedSettings.Engine
	if engineSettings == nil {
		defaults := settings.Default()
		engineSettings = defaults.Engine
	}
	playbackBackend := settings.BackendWeb
	keepWebAvailable := false
	if savedSettings.Playback != nil {
		playbackBackend = settings.NormalizePlaybackBackend(savedSettings.Playback.Backend)
		keepWebAvailable = savedSettings.Playback.KeepWebAvailable
	}
	soloistMode := playbackBackend == settings.BackendSoloist
	webMode := playbackBackend == settings.BackendWeb
	libraryOnly := soloistMode || webMode
	audioCacheDir := engineSettings.CacheDir
	if audioCacheDir == "" {
		audioCacheDir = filepath.Join(xdg.CacheHome, "cadence", "audio-cache")
	}
	skinManager, err := skins.NewManager(skins.Options{
		SkinsDir: filepath.Join(xdg.DataHome, "cadence", "skins"),
		DevDir:   devDir,
		Store:    settingsStore,
		SafeMode: *safeMode,
	})
	if err != nil {
		log.Fatalf("initialise skins: %v", err)
	}

	demoProvider := playback.NewDemoProvider()
	artCache := artwork.NewWithOptions(artwork.Options{
		DiskDir:   filepath.Join(xdg.CacheHome, "cadence", "art"),
		DiskLimit: 128 << 20,
	})

	// The developer automation starts a hidden helper per action against the
	// dashboard profile; the service itself holds no process between actions.
	// It is only used for one-time setup (Client ID / Soloist key). OAuth login
	// uses the system browser through xdg-open.
	var developerService *developer.Service
	if !*demoMode {
		service, err := developer.NewService(developer.Options{
			ConfigDir: filepath.Join(xdg.ConfigHome, "cadence"),
			Root:      filepath.Join(xdg.CacheHome, "cadence", "developer"),
			LogPath:   filepath.Join(xdg.CacheHome, "cadence", "logs", "developer.log"),
		})
		if err != nil {
			log.Fatalf("initialise developer automation: %v", err)
		}
		developerService = service
	}

	// The web service exists in every non-demo mode so Settings can connect,
	// install and inspect the runtime before the backend is selected. It binds
	// no socket and starts no process until asked.
	var webService *webplayer.Service
	if !*demoMode {
		options := webplayer.ServiceOptions{
			ClientID:      webClientID(),
			Art:           artCache,
			KeepAvailable: keepWebAvailable,
		}
		service, err := webplayer.NewService(options)
		if err != nil {
			log.Fatalf("initialise web player: %v", err)
		}
		webService = service
	}

	var recorder *diagnostics.Recorder
	var engineHTTPClient *http.Client
	if *diagnosticsEnabled {
		recorder = diagnostics.NewRecorder()
		engineHTTPClient = &http.Client{
			Timeout:   12 * time.Second,
			Transport: diagnostics.WrapTransport(http.DefaultTransport, recorder),
		}
		log.Println("diagnostics enabled: counters and pprof are exposed on a loopback port")
	}

	libraryDeviceName := "Cadence"
	if libraryOnly {
		// The go-librespot sidecar only keeps the catalog session in Soloist
		// and web modes; its device name makes that role explicit in Spotify
		// Connect.
		libraryDeviceName = "Cadence Library"
	}
	engineOptions := engine.Options{
		ConfigDir:                 filepath.Join(xdg.ConfigHome, "cadence", "audio"),
		LogPath:                   filepath.Join(xdg.CacheHome, "cadence", "logs", "go-librespot.log"),
		DeviceName:                libraryDeviceName,
		AudioBackend:              "pulseaudio",
		AudioDevice:               "default",
		Bitrate:                   engineSettings.Bitrate,
		CrossfadeDurationMS:       engineSettings.CrossfadeDurationMS,
		NormalisationDisabled:     engineSettings.NormalisationDisabled,
		NormalisationUseAlbumGain: engineSettings.NormalisationUseAlbumGain,
		NormalisationPregain:      engineSettings.NormalisationPregain,
		CacheEnabled:              engineSettings.CacheEnabled,
		CacheDir:                  audioCacheDir,
		CacheSizeLimit:            engineSettings.CacheSizeLimit,
		FlacEnabled:               engineSettings.FlacEnabled,
		MPRISEnabled:              engineSettings.MPRISEnabled,
		// DeviceIsGroup only changes the Spotify group icon, not Jam
		// participation, which uses the engine's Connect mode.
		DeviceIsGroup: engineSettings.DeviceIsGroup,
		ImageSize:     engineSettings.ImageSize,
		HTTPClient:    engineHTTPClient,
	}
	if libraryOnly {
		// Soloist and the web player own audio in these modes: go-librespot
		// must not expose MPRIS or keep a second audio cache.
		engineOptions.CacheEnabled = false
		engineOptions.MPRISEnabled = false
	}
	audioRuntime := engine.NewSupervisor(engineOptions)

	var soloistService *soloist.Service
	// The service exists in both modes so Settings can install Soloist and
	// store its API key before the user switches backends. Only the Soloist
	// backend starts the process.
	soloistService = soloist.NewService(soloist.Options{
		ProgramsDir: filepath.Join(xdg.DataHome, "cadence", "soloist", "programs"),
		DataDir:     filepath.Join(xdg.DataHome, "cadence", "soloist", "data"),
		CacheDir:    filepath.Join(xdg.CacheHome, "cadence", "soloist", "cache"),
		LogPath:     filepath.Join(xdg.CacheHome, "cadence", "logs", "soloist.log"),
		APIKeyPath:  filepath.Join(xdg.ConfigHome, "cadence", "soloist", "api-key"),
		DeviceName:  "Cadence",
	})

	var provider playback.Provider = demoProvider
	var catalogPlayback catalog.Playback
	if !*demoMode {
		switch playbackBackend {
		case settings.BackendSoloist:
			soloistProvider := playback.NewSoloistProvider(soloistService, artCache)
			provider = soloistProvider
			catalogPlayback = soloistProvider
		case settings.BackendWeb:
			webProvider := webService.Provider()
			provider = webProvider
			catalogPlayback = webProvider
		default:
			provider = playback.NewEngineProvider(audioRuntime, artCache)
		}
	}
	playerService := playerapi.NewService(provider)
	artCache.SetRecorder(recorder)
	playerService.SetRecorder(recorder)
	windowService := platform.NewWindowService()
	skinService := skinsapi.NewService(skinManager)
	catalogService := catalog.NewService(catalog.Options{
		Supervisor: audioRuntime,
		Art:        artCache,
		Enabled:    !*demoMode,
		Playback:   catalogPlayback,
	})
	var soloistBackend audioapi.SoloistBackend
	if soloistService != nil {
		soloistBackend = soloistService
	}
	var webBackend audioapi.WebBackend
	if webService != nil {
		webBackend = webService
	}
	var developerBackend audioapi.DeveloperBackend
	if developerService != nil {
		developerBackend = developerService
	}
	audioService := audioapi.NewService(audioapi.Options{
		Catalog:              catalogService,
		Supervisor:           audioRuntime,
		Settings:             settingsStore,
		Enabled:              !*demoMode,
		Soloist:              soloistBackend,
		Web:                  webBackend,
		Developer:            developerBackend,
		PlaybackBackend:      playbackBackend,
		DisableAutoProvision: strings.TrimSpace(os.Getenv("CADENCE_SPOTIFY_CLIENT_ID")) != "",
	})
	visualizerService := visualizerapi.NewService()
	lyricsClient := lyrics.NewClient(lyrics.Options{
		CacheDir: filepath.Join(xdg.CacheHome, "cadence", "lyrics"),
	})
	lyricsService := lyricsapi.NewService(lyricsClient)

	if recorder != nil {
		server, err := startDiagnosticsServer(recorder, audioRuntime.Version)
		if err != nil {
			log.Fatalf("start diagnostics server: %v", err)
		}
		defer func() {
			_ = server.Close()
		}()
	}

	frontend := application.AssetFileServerFS(assets)
	assetHandler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.HasPrefix(request.URL.Path, "/skins/"):
			skinManager.ServeAsset(writer, request)
		case strings.HasPrefix(request.URL.Path, "/art/"):
			artCache.ServeHTTP(writer, request)
		default:
			frontend.ServeHTTP(writer, request)
		}
	})

	cadence := application.New(application.Options{
		Name:        appName,
		Description: "A compact local Spotify player with declarative skins",
		Assets: application.AssetOptions{
			Handler: withSecurityHeaders(assetHandler),
		},
		Services: []application.Service{
			application.NewService(playerService),
			application.NewService(windowService),
			application.NewService(skinService),
			application.NewService(audioService),
			application.NewService(visualizerService),
			application.NewService(lyricsService),
		},
		Linux: application.LinuxOptions{
			ProgramName:   "cadence",
			ApplicationID: appID,
		},
	})

	// Declarative skins may use a taller logical canvas (for example the
	// square Nightstar chassis). The frontend keeps the window in sync when the
	// active skin changes, but starting at the right height avoids a visible
	// squash during the first paint.
	initialHeight := logicalHeight
	if runtime, err := skinManager.ActiveRuntime(); err == nil && !runtime.Builtin && runtime.BaseHeight > 0 {
		initialHeight = runtime.BaseHeight
	}

	// One frameless window. The queue and Library expand it vertically while
	// the webview zoom keeps text readable.
	cadence.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            appName,
		Width:            int(float64(logicalWidth) * uiScale),
		Height:           int(float64(initialHeight) * uiScale),
		MinWidth:         int(float64(logicalWidth) * minUiScale),
		MinHeight:        int(float64(initialHeight) * minUiScale),
		MaxWidth:         int(float64(logicalWidth) * maxUiScale),
		MaxHeight:        int(float64(max(expandedLogicalHeight, initialHeight)) * maxUiScale),
		Zoom:             uiScale,
		Frameless:        true,
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		URL:              "/",
		Linux: application.LinuxWindow{
			WebviewGpuPolicy: application.WebviewGpuPolicyOnDemand,
		},
	})

	runErr := cadence.Run()
	if developerService != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := developerService.Stop(stopCtx); err != nil {
			log.Printf("stop developer automation: %v", err)
		}
		cancel()
	}
	if webService != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := webService.Stop(stopCtx); err != nil {
			log.Printf("stop web player: %v", err)
		}
		cancel()
	}
	if runErr != nil {
		log.Fatal(runErr)
	}
}

func runWebCommand(login, logout, install bool) error {
	options := webplayer.ServiceOptions{ClientID: webClientID()}
	service, err := webplayer.NewService(options)
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := service.Stop(ctx); err != nil {
			log.Printf("stop web player: %v", err)
		}
	}()

	switch {
	case login:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if err := service.Login(ctx); err != nil {
			return err
		}
		fmt.Println("Spotify connected.")
		return nil
	case logout:
		if err := service.Logout(); err != nil {
			return err
		}
		fmt.Println("Spotify disconnected.")
		return nil
	case install:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		runtime, err := service.InstallRuntime(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("Web player runtime %s installed at %s\n", runtime.Version, runtime.Dir)
		return nil
	default:
		return nil
	}
}

func webClientID() string {
	if value := strings.TrimSpace(os.Getenv("CADENCE_SPOTIFY_CLIENT_ID")); value != "" {
		return value
	}
	// An empty value makes webplayer.NewService load the Client ID stored by
	// Settings, then fall back to the built-in public app.
	return ""
}

func startDiagnosticsServer(recorder *diagnostics.Recorder, engineVersion func() string) (*http.Server, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	server := &http.Server{
		Handler:           diagnosticsHandler(recorder, engineVersion),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("diagnostics server stopped: %v", err)
		}
	}()
	log.Printf("diagnostics available at http://%s/debug/stats (pprof at /debug/pprof/)", listener.Addr().String())
	return server, nil
}

func diagnosticsHandler(recorder *diagnostics.Recorder, engineVersion func() string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/stats", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writer.Header().Set("Allow", http.MethodGet)
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		version := ""
		if engineVersion != nil {
			version = engineVersion()
		}
		payload := struct {
			diagnostics.Snapshot
			EngineVersion string `json:"engine_version"`
		}{
			Snapshot:      recorder.Snapshot(),
			EngineVersion: version,
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(writer).Encode(payload); err != nil {
			log.Printf("encode diagnostics snapshot: %v", err)
		}
	})
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		if !strings.HasPrefix(request.URL.Path, "/skins/") {
			writer.Header().Set("X-Frame-Options", "DENY")
		}
		next.ServeHTTP(writer, request)
	})
}
