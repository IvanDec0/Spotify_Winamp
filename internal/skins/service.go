package skins

import (
	"context"
	"errors"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Service exposes the skin manager to the frontend.
type Service struct {
	manager *Manager

	mu  sync.Mutex
	app *application.App
}

// NewService creates the Wails-facing skin service.
func NewService(manager *Manager) *Service {
	return &Service{manager: manager}
}

func (s *Service) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	s.mu.Lock()
	s.app = application.Get()
	app := s.app
	s.mu.Unlock()
	s.manager.SetEmitter(func(catalog SkinCatalog) {
		if app != nil {
			app.Event.Emit(ChangedEvent, catalog)
		}
	})
	return nil
}

// GetCatalog returns the complete skin list and current selection.
func (s *Service) GetCatalog() SkinCatalog {
	return s.manager.Catalog()
}

// GetRuntime returns the declarative layout and stylesheet URLs for the active
// skin. Built-in Clarity returns Builtin=true and an empty layout.
func (s *Service) GetRuntime() (SkinRuntime, error) {
	return s.manager.ActiveRuntime()
}

// Activate switches to a skin by id.
func (s *Service) Activate(id string) (SkinCatalog, error) {
	return s.manager.Activate(id)
}

// SetTheme selects a theme declared by the active skin.
func (s *Service) SetTheme(theme string) (SkinCatalog, error) {
	return s.manager.SetTheme(theme)
}

// SetThirdPartyDisabled disables third-party skins and returns to Clarity.
func (s *Service) SetThirdPartyDisabled(disabled bool) (SkinCatalog, error) {
	return s.manager.SetThirdPartyDisabled(disabled)
}

// Reset returns to the built-in skin and clears the skin settings.
func (s *Service) Reset() (SkinCatalog, error) {
	return s.manager.Reset()
}

// Uninstall removes a managed third-party skin.
func (s *Service) Uninstall(id string) (SkinCatalog, error) {
	return s.manager.Uninstall(id)
}

// Install installs a skin from an explicit directory or ZIP path. The UI uses
// the dialog helpers below; this method is useful for scripts and tests.
func (s *Service) Install(path string) (InstallResult, error) {
	return s.manager.Install(path)
}

// InstallZipDialog opens a native file picker and installs the selected ZIP.
func (s *Service) InstallZipDialog() (InstallResult, error) {
	app := application.Get()
	if app == nil {
		return InstallResult{}, errors.New("application is not ready")
	}
	path, err := app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:                "Install a Cadence skin",
		Message:              "Choose a skin .zip archive. It will be validated before installation.",
		ButtonText:           "Install",
		CanChooseFiles:       true,
		CanChooseDirectories: false,
		Filters: []application.FileFilter{
			{DisplayName: "Cadence skin (*.zip)", Pattern: "*.zip"},
		},
	}).PromptForSingleSelection()
	if err != nil {
		return InstallResult{}, err
	}
	if path == "" {
		return InstallResult{Canceled: true}, nil
	}
	return s.manager.Install(path)
}

// InstallDirectoryDialog opens a native folder picker and installs the
// selected skin directory.
func (s *Service) InstallDirectoryDialog() (InstallResult, error) {
	app := application.Get()
	if app == nil {
		return InstallResult{}, errors.New("application is not ready")
	}
	path, err := app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:                "Install a Cadence skin",
		Message:              "Choose a folder containing manifest.json, layout.json and styles.css.",
		ButtonText:           "Install",
		CanChooseFiles:       false,
		CanChooseDirectories: true,
	}).PromptForSingleSelection()
	if err != nil {
		return InstallResult{}, err
	}
	if path == "" {
		return InstallResult{Canceled: true}, nil
	}
	return s.manager.Install(path)
}

// ReportRuntimeReady records a successful frontend mount as the last known
// good fallback.
func (s *Service) ReportRuntimeReady(id, theme string) error {
	return s.manager.ReportRuntimeReady(id, theme)
}

// ReportRuntimeFailure asks Go to roll back to a safe skin after a runtime
// mount failure.
func (s *Service) ReportRuntimeFailure(id, reason string) (SkinCatalog, error) {
	return s.manager.ReportRuntimeFailure(id, reason)
}
