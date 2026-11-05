package main

import (
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cadence/internal/artwork"
	"cadence/internal/catalog"
	"cadence/internal/diagnostics"
	"cadence/internal/engine"
	"cadence/internal/platform"
	"cadence/internal/playback"
	"cadence/internal/settings"
	"cadence/internal/skins"
	audioapi "cadence/internal/wailsapi/audio"
	playerapi "cadence/internal/wailsapi/player"
	skinsapi "cadence/internal/wailsapi/skins"

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
}

func main() {
	safeMode := flag.Bool("safe-mode", false, "start with the built-in skin and disable third-party skins")
	demoMode := flag.Bool("demo", false, "use the simulated player instead of Spotify")
	devSkins := flag.String("skins-dev", "", "directory containing development skins")
	diagnosticsEnabled := flag.Bool("diagnostics", false, "expose local diagnostics and pprof on a loopback port")
	flag.Parse()

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
		DiskLimit: 256 << 20,
	})

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

	audioRuntime := engine.NewSupervisor(engine.Options{
		ConfigDir:                 filepath.Join(xdg.ConfigHome, "cadence", "audio"),
		LogPath:                   filepath.Join(xdg.CacheHome, "cadence", "logs", "go-librespot.log"),
		DeviceName:                "Cadence",
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
	})
	var provider playback.Provider = demoProvider
	if !*demoMode {
		provider = playback.NewEngineProvider(audioRuntime, artCache)
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
	})
	audioService := audioapi.NewService(audioapi.Options{
		Catalog:    catalogService,
		Supervisor: audioRuntime,
		Settings:   settingsStore,
		Enabled:    !*demoMode,
	})

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

	if err := cadence.Run(); err != nil {
		log.Fatal(err)
	}
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
