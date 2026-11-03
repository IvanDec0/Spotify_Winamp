package skins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validManifestTemplate = `{
  "schemaVersion": 1,
  "id": "%s",
  "name": "Afterglow",
  "version": "1.0.0",
  "description": "A warm, wide development skin.",
  "author": "Cadence tests",
  "license": "MIT",
  "entry": {"layout": "layout.json", "styles": "styles.css"},
  "capabilities": ["player.read", "player.control", "queue.read", "devices.read", "devices.control"],
  "themes": ["ember", "tide"],
  "defaultTheme": "ember",
  "visualizer": {"modes": ["spectrum", "radial"], "default": "spectrum", "intensity": 0.6},
  "window": {"width": 420, "height": 220, "minWidth": 380, "minHeight": 200, "queueHeight": 460, "resizable": true}
}`

const validLayoutJSON = `{
  "schemaVersion": 1,
  "root": {
    "type": "column",
    "props": {"gap": 8},
    "children": [
      {"type": "component", "component": "titlebar"},
      {"type": "component", "component": "track-info"},
      {"type": "component", "component": "transport-controls"},
      {"type": "component", "component": "queue"}
    ]
  }
}`

const validCSS = `:host {
  --accent: #ffb347;
  background: url(assets/bg.png);
}
.cad-titlebar { color: var(--accent); }
@font-face {
  font-family: "Afterglow";
  src: url(assets/font.woff2) format("woff2");
}
`

func writeSkinPackage(t *testing.T, directory, id string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(directory, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(directory, "manifest.json"), strings.Replace(validManifestTemplate, "%s", id, 1))
	writeFile(t, filepath.Join(directory, "layout.json"), validLayoutJSON)
	writeFile(t, filepath.Join(directory, "styles.css"), validCSS)
	writeFile(t, filepath.Join(directory, "assets", "bg.png"), "\x89PNG\r\n\x1a\n")
	writeFile(t, filepath.Join(directory, "assets", "font.woff2"), "wOF2")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func validManifestStruct() Manifest {
	return Manifest{
		SchemaVersion: 1,
		ID:            "afterglow",
		Name:          "Afterglow",
		Version:       "1.0.0",
		Description:   "Test",
		Author:        "Test",
		License:       "MIT",
		Entry:         Entry{Layout: "layout.json", Styles: "styles.css"},
		Capabilities:  []string{"player.read", "player.control", "queue.read", "devices.read", "devices.control"},
		Themes:        []string{"ember", "tide"},
		DefaultTheme:  "ember",
		Visualizer: VisualizerManifest{
			Modes:     []string{"spectrum", "radial"},
			Default:   "spectrum",
			Intensity: 0.6,
		},
		Window: WindowSpec{Width: 420, Height: 220, MinWidth: 380, MinHeight: 200, QueueHeight: 460, Resizable: true},
	}
}

func TestValidateDirAcceptsValidSkin(t *testing.T) {
	directory := t.TempDir()
	writeSkinPackage(t, directory, "afterglow")

	manifest, layout, err := ValidateDir(directory)
	if err != nil {
		t.Fatalf("ValidateDir() error = %v", err)
	}
	if manifest.ID != "afterglow" {
		t.Fatalf("manifest id = %q", manifest.ID)
	}
	if layout.Root.Type != "column" {
		t.Fatalf("layout root = %q", layout.Root.Type)
	}
}

func TestValidateDirAcceptsThemeStylesheet(t *testing.T) {
	directory := t.TempDir()
	writeSkinPackage(t, directory, "afterglow")
	if err := os.MkdirAll(filepath.Join(directory, "themes"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(directory, "themes", "ember.css"), ":host { background: url(../assets/bg.png); }\n")

	if _, _, err := ValidateDir(directory); err != nil {
		t.Fatalf("ValidateDir() with theme stylesheet error = %v", err)
	}
}

func TestValidateDirRejectsAssetUrlOutsideRoot(t *testing.T) {
	for _, source := range []string{
		`.a { background: url(../secret.png); }`,
		`.a { background: url(assets/%2e%2e/secret.png); }`,
	} {
		directory := t.TempDir()
		writeSkinPackage(t, directory, "afterglow")
		writeFile(t, filepath.Join(directory, "styles.css"), source)
		if _, _, err := ValidateDir(directory); err == nil {
			t.Fatalf("ValidateDir() accepted %q", source)
		}
	}
}

func TestValidateDirRejectsUnknownFilesAndPaths(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, directory string)
		want   string
	}{
		{
			name: "unsupported asset",
			mutate: func(t *testing.T, directory string) {
				writeFile(t, filepath.Join(directory, "assets", "evil.svg"), "<svg/>")
			},
			want: "unsupported file type",
		},
		{
			name: "unexpected stylesheet",
			mutate: func(t *testing.T, directory string) {
				writeFile(t, filepath.Join(directory, "extra.css"), "body { color: red; }")
			},
			want: "unexpected stylesheet",
		},
		{
			name: "symlink",
			mutate: func(t *testing.T, directory string) {
				target := filepath.Join(directory, "assets", "bg.png")
				if err := os.Symlink(target, filepath.Join(directory, "assets", "link.png")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			},
			want: "symbolic links are not allowed",
		},
		{
			name: "bad asset header",
			mutate: func(t *testing.T, directory string) {
				writeFile(t, filepath.Join(directory, "assets", "bg.png"), "not a png")
			},
			want: "does not match its extension",
		},
		{
			name: "manifest wrong name",
			mutate: func(t *testing.T, directory string) {
				writeFile(t, filepath.Join(directory, "manifest.json"), strings.Replace(strings.Replace(validManifestTemplate, "%s", "afterglow", 1), "\"name\": \"Afterglow\"", "\"name\": 7", 1))
			},
			want: "manifest.json does not match schema",
		},
		{
			name: "unknown manifest field",
			mutate: func(t *testing.T, directory string) {
				writeFile(t, filepath.Join(directory, "manifest.json"), strings.Replace(strings.Replace(validManifestTemplate, "%s", "afterglow", 1), "\"license\": \"MIT\",", "\"license\": \"MIT\",\n  \"exec\": \"script.js\",", 1))
			},
			want: "manifest.json does not match schema",
		},
		{
			name: "unknown layout field",
			mutate: func(t *testing.T, directory string) {
				writeFile(t, filepath.Join(directory, "layout.json"), strings.Replace(validLayoutJSON, "\"type\": \"column\",", "\"type\": \"column\",\n    \"style\": \"position: fixed\",", 1))
			},
			want: "layout.json does not match schema",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			writeSkinPackage(t, directory, "afterglow")
			test.mutate(t, directory)
			_, _, err := ValidateDir(directory)
			if err == nil {
				t.Fatal("ValidateDir() error = nil")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %q, want containing %q", err, test.want)
			}
		})
	}
}

func TestValidateManifestRules(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{name: "reserved id", mutate: func(m *Manifest) { m.ID = BuiltinID }},
		{name: "unknown default theme", mutate: func(m *Manifest) { m.DefaultTheme = "missing" }},
		{name: "unknown visualizer default", mutate: func(m *Manifest) { m.Visualizer.Default = "grid" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := validManifestStruct()
			test.mutate(&manifest)
			document, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := validateManifest(document); err == nil {
				t.Fatal("validateManifest() error = nil")
			}
		})
	}
}

func TestValidateLayoutSemantics(t *testing.T) {
	manifest := validManifestStruct()

	t.Run("valid", func(t *testing.T) {
		layout := Layout{SchemaVersion: 1, Root: LayoutNode{
			Type: "column",
			Children: []LayoutNode{
				{Type: "component", Component: "titlebar"},
				{Type: "component", Component: "track-info"},
			},
		}}
		if err := validateLayoutSemantics(&layout, &manifest); err != nil {
			t.Fatalf("validateLayoutSemantics() error = %v", err)
		}
	})

	tests := []struct {
		name   string
		root   LayoutNode
		change func(*Manifest)
		want   string
	}{
		{
			name: "missing titlebar",
			root: LayoutNode{Type: "column", Children: []LayoutNode{{Type: "component", Component: "track-info"}}},
			want: "titlebar",
		},
		{
			name: "duplicate component",
			root: LayoutNode{Type: "column", Children: []LayoutNode{
				{Type: "component", Component: "titlebar"},
				{Type: "component", Component: "titlebar"},
			}},
			want: "more than once",
		},
		{
			name: "unknown prop",
			root: LayoutNode{Type: "column", Children: []LayoutNode{
				{Type: "component", Component: "titlebar", Props: map[string]any{"sparkle": true}},
			}},
			want: "unknown titlebar prop",
		},
		{
			name: "wrong prop type",
			root: LayoutNode{Type: "column", Children: []LayoutNode{
				{Type: "component", Component: "album-art", Props: map[string]any{"size": 20.0}},
			}},
			want: "between",
		},
		{
			name: "visualizer mode not declared",
			root: LayoutNode{Type: "column", Children: []LayoutNode{
				{Type: "component", Component: "titlebar"},
				{Type: "component", Component: "visualizer", Props: map[string]any{"mode": "grid"}},
			}},
			want: "not declared",
		},
		{
			name: "missing capability",
			root: LayoutNode{Type: "column", Children: []LayoutNode{
				{Type: "component", Component: "titlebar"},
				{Type: "component", Component: "queue"},
			}},
			change: func(m *Manifest) { m.Capabilities = []string{"player.read"} },
			want:   "queue.read",
		},
		{
			name: "root is component",
			root: LayoutNode{Type: "component", Component: "titlebar"},
			want: "root must be",
		},
		{
			name: "empty container",
			root: LayoutNode{Type: "column", Children: []LayoutNode{{Type: "row"}}},
			want: "at least one child",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			local := manifest
			if test.change != nil {
				test.change(&local)
			}
			layout := Layout{SchemaVersion: 1, Root: test.root}
			err := validateLayoutSemantics(&layout, &local)
			if err == nil {
				t.Fatal("validateLayoutSemantics() error = nil")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %q, want containing %q", err, test.want)
			}
		})
	}
}

func TestValidateLayoutDepthAndNodeCount(t *testing.T) {
	manifest := validManifestStruct()
	root := LayoutNode{Type: "component", Component: "titlebar"}
	for index := 0; index < maxLayoutDepth+1; index++ {
		root = LayoutNode{Type: "column", Children: []LayoutNode{root}}
	}
	layout := Layout{SchemaVersion: 1, Root: root}
	if err := validateLayoutSemantics(&layout, &manifest); err == nil || !strings.Contains(err.Error(), "depth") {
		t.Fatalf("depth error = %v", err)
	}
}

func TestDevelopmentSkinsAreValid(t *testing.T) {
	root := filepath.Join("..", "..", "skins-dev")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read development skins: %v", err)
	}
	found := false
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		found = true
		if _, _, err := ValidateDir(filepath.Join(root, entry.Name())); err != nil {
			t.Errorf("development skin %q: %v", entry.Name(), err)
		}
	}
	if !found {
		t.Fatal("no development skins found")
	}
}
