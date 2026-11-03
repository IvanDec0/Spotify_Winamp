package skins

// BuiltinID is the reserved identifier of the native Clarity skin. It is always
// present and cannot be installed over, renamed or removed.
const BuiltinID = "clarity"

// ChangedEvent is emitted with a full catalog whenever skins are installed,
// activated, removed or reset.
const ChangedEvent = "skins:changed"

const (
	sourceBuiltin     = "builtin"
	sourceInstalled   = "installed"
	sourceDevelopment = "development"
)

// Manifest is the validated manifest.json of a skin package.
type Manifest struct {
	SchemaVersion int                `json:"schemaVersion"`
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Version       string             `json:"version"`
	Description   string             `json:"description"`
	Author        string             `json:"author"`
	License       string             `json:"license"`
	Entry         Entry              `json:"entry"`
	Capabilities  []string           `json:"capabilities"`
	Themes        []string           `json:"themes"`
	DefaultTheme  string             `json:"defaultTheme"`
	Visualizer    VisualizerManifest `json:"visualizer"`
	Window        WindowSpec         `json:"window"`
}

// Entry points at the layout and stylesheet inside the skin package.
type Entry struct {
	Layout string `json:"layout"`
	Styles string `json:"styles"`
}

// VisualizerManifest declares the modes and intensity a skin supports.
type VisualizerManifest struct {
	Modes     []string `json:"modes"`
	Default   string   `json:"default"`
	Intensity float64  `json:"intensity"`
}

// WindowSpec keeps the v1 single-window dimensions explicit. The values are
// fixed by the schema so skins cannot grow the controller unexpectedly.
type WindowSpec struct {
	Width       int  `json:"width"`
	Height      int  `json:"height"`
	MinWidth    int  `json:"minWidth"`
	MinHeight   int  `json:"minHeight"`
	QueueHeight int  `json:"queueHeight"`
	Resizable   bool `json:"resizable"`
}

// Layout is the validated layout.json of a skin package.
type Layout struct {
	SchemaVersion int        `json:"schemaVersion"`
	Root          LayoutNode `json:"root"`
}

// LayoutNode is one node of the declarative layout tree. Props are validated
// per component before a layout is accepted; keeping them as a map lets the
// TypeScript runtime read only the documented keys.
type LayoutNode struct {
	Type      string         `json:"type"`
	Name      string         `json:"name,omitempty"`
	Component string         `json:"component,omitempty"`
	Props     map[string]any `json:"props,omitempty"`
	Children  []LayoutNode   `json:"children,omitempty"`
}

// SkinSummary is the catalog entry shown in the settings surface.
type SkinSummary struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	Description     string   `json:"description"`
	Author          string   `json:"author"`
	License         string   `json:"license"`
	Source          string   `json:"source"`
	Builtin         bool     `json:"builtin"`
	Removable       bool     `json:"removable"`
	Active          bool     `json:"active"`
	Themes          []string `json:"themes"`
	DefaultTheme    string   `json:"defaultTheme"`
	VisualizerModes []string `json:"visualizerModes"`
}

// SkinCatalog is the complete state exposed to the frontend.
type SkinCatalog struct {
	Skins               []SkinSummary `json:"skins"`
	ActiveID            string        `json:"activeId"`
	ActiveTheme         string        `json:"activeTheme"`
	SafeMode            bool          `json:"safeMode"`
	ThirdPartyDisabled  bool          `json:"thirdPartyDisabled"`
	Warnings            []string      `json:"warnings"`
	LastError           string        `json:"lastError"`
	CanInstall          bool          `json:"canInstall"`
	CanChangeActiveSkin bool          `json:"canChangeActiveSkin"`
}

// SkinRuntime carries the declarative content the Shadow DOM renderer needs.
// The stylesheet is served by the application asset handler so relative asset
// URLs resolve inside the installed skin directory.
type SkinRuntime struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Version           string            `json:"version"`
	Author            string            `json:"author"`
	License           string            `json:"license"`
	Description       string            `json:"description"`
	Builtin           bool              `json:"builtin"`
	Themes            []string          `json:"themes"`
	DefaultTheme      string            `json:"defaultTheme"`
	VisualizerModes   []string          `json:"visualizerModes"`
	DefaultVisualizer string            `json:"defaultVisualizer"`
	Intensity         float64           `json:"intensity"`
	Layout            Layout            `json:"layout"`
	StylesheetURL     string            `json:"stylesheetUrl"`
	ThemeStylesheets  map[string]string `json:"themeStylesheets"`
	BaseHeight        int               `json:"baseHeight"`
	QueueHeight       int               `json:"queueHeight"`
}

// InstallResult reports a successful package installation. Installation does
// not activate the skin automatically; the caller can confirm first.
type InstallResult struct {
	Skin     SkinSummary `json:"skin"`
	Catalog  SkinCatalog `json:"catalog"`
	Canceled bool        `json:"canceled"`
}

// ValidationError groups user-facing validation failures for a skin package.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func validationError(message string) error {
	return &ValidationError{Message: message}
}
