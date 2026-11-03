package main

import (
	"embed"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"cadence/internal/artcache"
	"cadence/internal/keyring"
	"cadence/internal/platform"
	"cadence/internal/player"
	"cadence/internal/settings"
	"cadence/internal/skins"
	"cadence/internal/spotify"

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
	application.RegisterEvent[player.PlayerState](player.StateEvent)
	application.RegisterEvent[skins.SkinCatalog](skins.ChangedEvent)
	application.RegisterEvent[spotify.AuthState](spotify.AuthEvent)
}

func main() {
	safeMode := flag.Bool("safe-mode", false, "start with the built-in skin and disable third-party skins")
	demoMode := flag.Bool("demo", false, "use the simulated player instead of Spotify")
	devSkins := flag.String("skins-dev", "", "directory containing development skins")
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
	if devDir != "" {
		absolute, err := filepath.Abs(devDir)
		if err != nil {
			log.Fatalf("resolve development skins directory: %v", err)
		}
		devDir = absolute
	}

	settingsStore := settings.NewStore(filepath.Join(xdg.ConfigHome, "cadence", "config.json"))
	skinManager, err := skins.NewManager(skins.Options{
		SkinsDir: filepath.Join(xdg.DataHome, "cadence", "skins"),
		DevDir:   devDir,
		Store:    settingsStore,
		SafeMode: *safeMode,
	})
	if err != nil {
		log.Fatalf("initialise skins: %v", err)
	}

	demoProvider := player.NewDemoProvider()
	playerService := player.NewService(demoProvider)
	windowService := platform.NewWindowService()
	skinService := skins.NewService(skinManager)
	artCache := artcache.New(nil)
	spotifyService := spotify.NewService(spotify.Options{
		Store:     settingsStore,
		Keyring:   keyring.NewOSStore(),
		Player:    playerService,
		Demo:      demoProvider,
		Art:       artCache,
		ForceDemo: *demoMode,
	})

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
		Description: "A compact Spotify Connect controller with declarative skins",
		Assets: application.AssetOptions{
			Handler: withSecurityHeaders(assetHandler),
		},
		Services: []application.Service{
			application.NewService(playerService),
			application.NewService(windowService),
			application.NewService(skinService),
			application.NewService(spotifyService),
		},
		Linux: application.LinuxOptions{
			ProgramName:   "cadence",
			ApplicationID: appID,
		},
	})

	// One frameless window. Clarity's default size is 420x220; the queue
	// expands the window vertically while keeping the player visible.
	cadence.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            appName,
		Width:            420,
		Height:           220,
		MinWidth:         380,
		MinHeight:        200,
		MaxHeight:        560,
		Frameless:        true,
		BackgroundColour: application.NewRGB(9, 11, 15),
		URL:              "/",
		Linux: application.LinuxWindow{
			WebviewGpuPolicy: application.WebviewGpuPolicyOnDemand,
		},
	})

	if err := cadence.Run(); err != nil {
		log.Fatal(err)
	}
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
