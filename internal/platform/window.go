package platform

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// WindowService exposes the small set of native window operations that need
// platform-specific handling. Wails v3 beta.34 maps SetSize to GTK's
// gtk_window_set_default_size on GTK4, which does not resize a mapped window;
// Cadence needs a real resize for the queue expansion.
type WindowService struct{}

const (
	// mainWindowName is the compact player window. It is hidden while Browse is
	// visible and restored when Browse closes; it is never destroyed here.
	mainWindowName = "main"

	// browseWindowName is the stable Wails name used to focus an existing
	// Browse window instead of opening a duplicate.
	browseWindowName = "browse"

	// BrowseNavigateEvent asks an already-open Browse window to switch sections.
	BrowseNavigateEvent = "browse:navigate"

	// Browse keeps the custom chrome usable at the minimum supported size.
	browseMinWidth  = 720
	browseMinHeight = 480
)

// hideMainWindow hides the compact player only after the Browse window has
// been shown successfully. Playback and frontend state stay alive.
func hideMainWindow(app *application.App) {
	window, ok := app.Window.GetByName(mainWindowName)
	if !ok || !window.IsVisible() {
		return
	}
	window.Hide()
}

// showMainWindow restores the compact player. The check avoids forcing a
// window back up while the application is shutting down.
func showMainWindow(app *application.App) {
	if app == nil {
		return
	}
	if ctx := app.Context(); ctx != nil && ctx.Err() != nil {
		return
	}
	window, ok := app.Window.GetByName(mainWindowName)
	if !ok {
		return
	}
	window.Show()
	window.Focus()
}

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
// opaque and frameless so Browse owns its draggable topbar and resize grip.
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
		return application.InvokeSyncWithError(func() error {
			window.Show()
			window.Focus()
			if !window.IsVisible() {
				return errors.New("browse window could not be shown")
			}
			app.Event.Emit(BrowseNavigateEvent, normalized)
			hideMainWindow(app)
			return nil
		})
	}

	return application.InvokeSyncWithError(func() error {
		window := app.Window.NewWithOptions(application.WebviewWindowOptions{
			Name:             browseWindowName,
			Title:            "Cadence Browse",
			Width:            960,
			Height:           640,
			MinWidth:         browseMinWidth,
			MinHeight:        browseMinHeight,
			URL:              "/?surface=browse&section=" + normalized,
			Zoom:             1,
			Frameless:        true,
			BackgroundType:   application.BackgroundTypeSolid,
			BackgroundColour: application.NewRGBA(9, 11, 15, 255),
			Linux: application.LinuxWindow{
				WebviewGpuPolicy: application.WebviewGpuPolicyOnDemand,
			},
		})
		if window == nil {
			return errors.New("browse window could not be created")
		}

		// Native X, Alt+F4 and WM close paths all end here. Do not cancel the
		// close; just bring the compact player back.
		window.OnWindowEvent(events.Common.WindowClosing, func(_ *application.WindowEvent) {
			showMainWindow(app)
		})

		window.Show()
		window.Focus()
		if !window.IsVisible() {
			return errors.New("browse window could not be shown")
		}
		hideMainWindow(app)
		return nil
	})
}

// CloseBrowse closes the Browse window and restores the compact player. The
// explicit restore covers close paths where the WindowClosing listener does
// not fire.
func (s *WindowService) CloseBrowse() error {
	app := application.Get()
	if app == nil {
		return errors.New("application is not ready")
	}
	if window, ok := app.Window.GetByName(browseWindowName); ok {
		if err := application.InvokeSyncWithError(func() error {
			window.Close()
			return nil
		}); err != nil {
			return err
		}
	}
	return application.InvokeSyncWithError(func() error {
		showMainWindow(app)
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

// clampBrowseSize enforces the minimum Browse window size used by both the
// native window options and the custom resize grip.
func clampBrowseSize(width, height int) (int, int) {
	if width < browseMinWidth {
		width = browseMinWidth
	}
	if height < browseMinHeight {
		height = browseMinHeight
	}
	return width, height
}

// SetSize resizes the main window in physical pixels.
func (s *WindowService) SetSize(width, height int) error {
	return s.resizeNamedWindow(mainWindowName, width, height)
}

// SetBrowseSize resizes the frameless Browse window in physical pixels. The
// custom chrome calls it from the resize grip.
func (s *WindowService) SetBrowseSize(width, height int) error {
	width, height = clampBrowseSize(width, height)
	return s.resizeNamedWindow(browseWindowName, width, height)
}

// resizeNamedWindow must run on the GTK main thread, hence the
// InvokeSyncWithError wrapper.
func (s *WindowService) resizeNamedWindow(name string, width, height int) error {
	app := application.Get()
	if app == nil {
		return errors.New("application is not ready")
	}

	window, ok := app.Window.GetByName(name)
	if !ok {
		return fmt.Errorf("%s window not found", name)
	}

	return application.InvokeSyncWithError(func() error {
		handle := window.NativeWindow()
		if handle == nil {
			return fmt.Errorf("%s window is not available", name)
		}
		resizeWindow(handle, width, height)
		return nil
	})
}
