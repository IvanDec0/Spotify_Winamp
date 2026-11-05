package platform

import "testing"

// These checks exercise the no-application path. Live window creation needs a
// running GTK/Wails application and is verified manually, not in unit tests.
func TestWindowServiceRequiresApplication(t *testing.T) {
	service := NewWindowService()

	if err := service.OpenBrowse(); err == nil {
		t.Fatal("OpenBrowse() error = nil without an application")
	}
	if service.BrowseOpen() {
		t.Fatal("BrowseOpen() = true without an application")
	}
	if err := service.CloseBrowse(); err == nil {
		t.Fatal("CloseBrowse() error = nil without an application")
	}
}
