package platform

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// WindowService exposes the small set of native window operations that need
// platform-specific handling. Wails v3 beta.34 maps SetSize to GTK's
// gtk_window_set_default_size on GTK4, which does not resize a mapped window;
// Cadence needs a real resize for the queue expansion.
type WindowService struct{}

func NewWindowService() *WindowService {
	return &WindowService{}
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
