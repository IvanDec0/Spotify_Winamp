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

func TestClampBrowseSize(t *testing.T) {
	tests := []struct {
		width, height int
		wantW, wantH  int
	}{
		{width: 1200, height: 800, wantW: 1200, wantH: 800},
		{width: 500, height: 300, wantW: browseMinWidth, wantH: browseMinHeight},
		{width: 720, height: 480, wantW: 720, wantH: 480},
		{width: 0, height: 0, wantW: browseMinWidth, wantH: browseMinHeight},
	}
	for _, test := range tests {
		gotW, gotH := clampBrowseSize(test.width, test.height)
		if gotW != test.wantW || gotH != test.wantH {
			t.Fatalf("clampBrowseSize(%d, %d) = %d, %d; want %d, %d",
				test.width, test.height, gotW, gotH, test.wantW, test.wantH)
		}
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
		{input: "nowplaying", want: "nowplaying", ok: true},
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
