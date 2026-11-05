package platform

import "testing"

// These checks exercise validation and the no-application path. Live window
// creation needs a running GTK/Wails application and is verified manually, not
// in unit tests.
func TestWindowServiceRequiresApplication(t *testing.T) {
	service := NewWindowService()

	if err := service.OpenBrowse("library"); err == nil {
		t.Fatal("OpenBrowse(\"library\") error = nil without an application")
	}
	if service.BrowseOpen() {
		t.Fatal("BrowseOpen() = true without an application")
	}
	if err := service.CloseBrowse(); err == nil {
		t.Fatal("CloseBrowse() error = nil without an application")
	}
}

func TestBrowseSection(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "", want: "library", ok: true},
		{input: "library", want: "library", ok: true},
		{input: "search", want: "search", ok: true},
		{input: "podcasts", want: "podcasts", ok: true},
		{input: "settings", want: "settings", ok: true},
		{input: "../settings", ok: false},
		{input: "Library", ok: false},
	}
	for _, test := range tests {
		got, err := browseSection(test.input)
		if test.ok && (err != nil || got != test.want) {
			t.Fatalf("browseSection(%q) = %q, %v; want %q", test.input, got, err, test.want)
		}
		if !test.ok && err == nil {
			t.Fatalf("browseSection(%q) error = nil", test.input)
		}
	}
}
