package platform

import (
	"errors"

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

// SetSize resizes the main window in logical pixels. It must run on the GTK
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
