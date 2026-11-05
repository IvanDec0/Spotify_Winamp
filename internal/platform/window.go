package platform

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// WindowService exposes the small set of native window operations that need
// platform-specific handling. Wails v3 beta.34 maps SetSize to GTK's
// gtk_window_set_default_size on GTK4, which does not resize a mapped window;
// Cadence needs a real resize for the queue expansion.
type WindowService struct{}

const (
	// browseWindowName is the stable Wails name used to focus an existing
	// Browse window instead of opening a duplicate.
	browseWindowName = "browse"

	// BrowseNavigateEvent asks an already-open Browse window to switch sections.
	BrowseNavigateEvent = "browse:navigate"
)

// browseSection normalizes the section requested by the compact window. The
// value becomes part of a URL, so only known identifiers are accepted.
func browseSection(section string) (string, error) {
	switch section {
	case "", "library":
		return "library", nil
	case "search", "podcasts", "settings":
		return section, nil
	default:
		return "", fmt.Errorf("unsupported browse section %q", section)
	}
}

func NewWindowService() *WindowService {
	return &WindowService{}
}

// OpenBrowse opens the dedicated Browse window, or focuses it when it already
// exists. An existing window receives a navigation event instead of a reload.
// Window creation runs on the GTK main thread through Wails; the window is
// opaque and keeps a normal frame so it behaves like a regular second window.
func (s *WindowService) OpenBrowse(section string) error {
	normalized, err := browseSection(section)
	if err != nil {
		return err
	}
	app := application.Get()
	if app == nil {
		return errors.New("application is not ready")
	}

	if window, ok := app.Window.GetByName(browseWindowName); ok {
		if err := application.InvokeSyncWithError(func() error {
			window.Show()
			window.Focus()
			return nil
		}); err != nil {
			return err
		}
		app.Event.Emit(BrowseNavigateEvent, normalized)
		return nil
	}

	return application.InvokeSyncWithError(func() error {
		app.Window.NewWithOptions(application.WebviewWindowOptions{
			Name:             browseWindowName,
			Title:            "Cadence Browse",
			Width:            960,
			Height:           640,
			MinWidth:         720,
			MinHeight:        480,
			URL:              "/?surface=browse&section=" + normalized,
			Zoom:             1,
			Frameless:        false,
			BackgroundType:   application.BackgroundTypeSolid,
			BackgroundColour: application.NewRGBA(9, 11, 15, 255),
			Linux: application.LinuxWindow{
				WebviewGpuPolicy: application.WebviewGpuPolicyOnDemand,
			},
		})
		return nil
	})
}

// CloseBrowse closes the Browse window. It is a no-op when the window is not
// open.
func (s *WindowService) CloseBrowse() error {
	app := application.Get()
	if app == nil {
		return errors.New("application is not ready")
	}
	window, ok := app.Window.GetByName(browseWindowName)
	if !ok {
		return nil
	}
	return application.InvokeSyncWithError(func() error {
		window.Close()
		return nil
	})
}

// BrowseOpen reports whether the Browse window currently exists.
func (s *WindowService) BrowseOpen() bool {
	app := application.Get()
	if app == nil {
		return false
	}
	_, ok := app.Window.GetByName(browseWindowName)
	return ok
}

// ServiceStartup makes the GTK4 window itself transparent. Wails v3 beta.34
// has a no-op setTransparent on GTK4, so the webview background alone still
// leaves the window's theme background (a gray square) visible around shaped
// skins. The CSS provider is installed before the first user interaction.
func (s *WindowService) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	app := application.Get()
	if app == nil {
		return nil
	}
	window, ok := app.Window.GetByName("main")
	if !ok {
		return nil
	}

	// GTK4 does not expose the native handle until after the window is shown.
	// Retry briefly so the CSS provider is installed as soon as it exists.
	go func() {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			applied := false
			_ = application.InvokeSyncWithError(func() error {
				handle := window.NativeWindow()
				if handle == nil {
					return errors.New("window is not ready")
				}
				makeWindowTransparent(handle)
				applied = true
				return nil
			})
			if applied {
				log.Println("window transparency enabled")
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		log.Println("window transparency could not be enabled")
	}()
	return nil
}

// SetSize resizes the main window in physical pixels. It must run on the GTK
// main thread, hence the InvokeSyncWithError wrapper.
func (s *WindowService) SetSize(width, height int) error {
	app := application.Get()
	if app == nil {
		return errors.New("application is not ready")
	}

	window, ok := app.Window.GetByName("main")
	if !ok {
		return errors.New("main window not found")
	}

	return application.InvokeSyncWithError(func() error {
		handle := window.NativeWindow()
		if handle == nil {
			return errors.New("main window is not available")
		}
		resizeWindow(handle, width, height)
		return nil
	})
}
