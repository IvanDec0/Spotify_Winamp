package skins

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	maxManifestSize = 64 << 10
	maxLayoutSize   = 256 << 10
	maxCSSSize      = 1 << 20
	maxAssetSize    = 10 << 20
	maxSkinSize     = 50 << 20
	maxSkinFiles    = 500
)

//go:embed schema/*.schema.json
var schemaFiles embed.FS

var (
	compilerOnce   sync.Once
	manifestSchema *jsonschema.Schema
	layoutSchema   *jsonschema.Schema
	compilerErr    error

	slugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$`)
)

func schemas() (*jsonschema.Schema, *jsonschema.Schema, error) {
	compilerOnce.Do(func() {
		compiler := jsonschema.NewCompiler()
		compiler.AssertFormat()

		for resource, name := range map[string]string{
			"https://cadence.local/schemas/manifest-v1.json": "schema/manifest-v1.schema.json",
			"https://cadence.local/schemas/layout-v1.json":   "schema/layout-v1.schema.json",
		} {
			file, err := schemaFiles.Open(name)
			if err != nil {
				compilerErr = err
				return
			}
			document, err := jsonschema.UnmarshalJSON(file)
			_ = file.Close()
			if err != nil {
				compilerErr = err
				return
			}
			if err := compiler.AddResource(resource, document); err != nil {
				compilerErr = err
				return
			}
		}

		manifestSchema, compilerErr = compiler.Compile("https://cadence.local/schemas/manifest-v1.json")
		if compilerErr != nil {
			return
		}
		layoutSchema, compilerErr = compiler.Compile("https://cadence.local/schemas/layout-v1.json")
	})
	return manifestSchema, layoutSchema, compilerErr
}

// ValidateDir validates a complete skin directory and returns its manifest and
// layout. It rejects symlinks, hard links, case collisions, unknown file types,
// oversized packages and unsafe CSS before any skin can be activated.
func ValidateDir(root string) (*Manifest, *Layout, error) {
	root = filepath.Clean(root)
	info, err := os.Lstat(root)
	if err != nil {
		return nil, nil, fmt.Errorf("open skin directory: %w", err)
	}
	if !info.IsDir() {
		return nil, nil, validationError("skin root must be a directory")
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, nil, validationError("skin root cannot be a symbolic link")
	}

	files := make(map[string]os.FileInfo)
	seenCase := make(map[string]string)
	totalSize := int64(0)
	err = filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == root {
			return nil
		}

		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)

		if entry.Type()&os.ModeSymlink != 0 {
			return validationError(fmt.Sprintf("symbolic links are not allowed: %s", relative))
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return validationError(fmt.Sprintf("unsupported file type: %s", relative))
		}
		if strings.ContainsRune(relative, '\\') {
			return validationError(fmt.Sprintf("backslashes are not allowed in paths: %s", relative))
		}
		if relative == ".." || strings.HasPrefix(relative, "../") || strings.HasPrefix(relative, "/") {
			return validationError(fmt.Sprintf("unsafe path: %s", relative))
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Nlink > 1 {
			return validationError(fmt.Sprintf("hard links are not allowed: %s", relative))
		}

		extension := strings.ToLower(path.Ext(relative))
		if !allowedExtension(relative, extension) {
			return validationError(fmt.Sprintf("unsupported file type: %s", relative))
		}

		limit := fileSizeLimit(relative, extension)
		if info.Size() > limit {
			return validationError(fmt.Sprintf("%s exceeds its size limit", relative))
		}
		totalSize += info.Size()
		if totalSize > maxSkinSize {
			return validationError("skin package exceeds the 50 MB uncompressed limit")
		}
		if len(files) >= maxSkinFiles {
			return validationError("skin package exceeds the 500 file limit")
		}

		folded := strings.ToLower(relative)
		if previous, ok := seenCase[folded]; ok {
			return validationError(fmt.Sprintf("case-colliding paths: %s and %s", previous, relative))
		}
		seenCase[folded] = relative
		files[relative] = info

		if isAssetExtension(extension) {
			if err := validateAssetFile(filepath.Join(root, filepath.FromSlash(relative)), extension); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	manifestInfo, ok := files["manifest.json"]
	if !ok {
		return nil, nil, validationError("manifest.json must be at the skin root")
	}
	if manifestInfo.Size() > maxManifestSize {
		return nil, nil, validationError("manifest.json exceeds 64 KB")
	}
	layoutInfo, ok := files["layout.json"]
	if !ok {
		return nil, nil, validationError("layout.json must be at the skin root")
	}
	if layoutInfo.Size() > maxLayoutSize {
		return nil, nil, validationError("layout.json exceeds 256 KB")
	}
	stylesInfo, ok := files["styles.css"]
	if !ok {
		return nil, nil, validationError("styles.css must be at the skin root")
	}
	if stylesInfo.Size() > maxCSSSize {
		return nil, nil, validationError("styles.css exceeds 1 MB")
	}

	manifestBytes, err := readLimitedFile(filepath.Join(root, "manifest.json"), maxManifestSize)
	if err != nil {
		return nil, nil, err
	}
	manifest, err := validateManifest(manifestBytes)
	if err != nil {
		return nil, nil, err
	}

	layoutBytes, err := readLimitedFile(filepath.Join(root, "layout.json"), maxLayoutSize)
	if err != nil {
		return nil, nil, err
	}
	layout, err := validateLayout(layoutBytes)
	if err != nil {
		return nil, nil, err
	}

	stylesheetFiles, err := skinStylesheets(files, manifest)
	if err != nil {
		return nil, nil, err
	}
	for _, stylesheet := range stylesheetFiles {
		cssBytes, err := readLimitedFile(filepath.Join(root, filepath.FromSlash(stylesheet)), maxCSSSize)
		if err != nil {
			return nil, nil, err
		}
		urls, err := ValidateCSS(string(cssBytes))
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", stylesheet, err)
		}
		baseDir := path.Dir(stylesheet)
		for _, assetURL := range urls {
			relativeAsset := path.Clean(path.Join(baseDir, assetURL))
			if relativeAsset == ".." || strings.HasPrefix(relativeAsset, "../") || strings.HasPrefix(relativeAsset, "/") {
				return nil, nil, validationError(fmt.Sprintf("%s references an asset outside the skin directory: %s", stylesheet, assetURL))
			}
			assetPath := filepath.Join(root, filepath.FromSlash(relativeAsset))
			assetInfo, err := os.Lstat(assetPath)
			if err != nil {
				return nil, nil, validationError(fmt.Sprintf("%s references a missing asset: %s", stylesheet, assetURL))
			}
			if !assetInfo.Mode().IsRegular() || assetInfo.Mode()&os.ModeSymlink != 0 {
				return nil, nil, validationError(fmt.Sprintf("%s references an invalid asset: %s", stylesheet, assetURL))
			}
			assetExtension := strings.ToLower(path.Ext(relativeAsset))
			if !isAssetExtension(assetExtension) {
				return nil, nil, validationError(fmt.Sprintf("%s references an unsupported asset type: %s", stylesheet, assetURL))
			}
		}
	}

	if err := validateLayoutSemantics(layout, manifest); err != nil {
		return nil, nil, err
	}
	if err := validateLayoutAssets(layout, files); err != nil {
		return nil, nil, err
	}
	return manifest, layout, nil
}

// validateLayoutAssets makes image props point at real files in the package.
// CSS URLs are checked above; this closes the same gap for declarative images.
func validateLayoutAssets(layout *Layout, files map[string]os.FileInfo) error {
	var walk func(node LayoutNode) error
	walk = func(node LayoutNode) error {
		if node.Type == "image" {
			src := path.Clean(node.Props["src"].(string))
			info, ok := files[src]
			if !ok {
				return validationError(fmt.Sprintf("layout image references a missing asset: %s", src))
			}
			if !info.Mode().IsRegular() {
				return validationError(fmt.Sprintf("layout image references an invalid asset: %s", src))
			}
		}
		for _, child := range node.Children {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(layout.Root); err != nil {
		return err
	}
	return nil
}

// skinStylesheets returns styles.css plus any optional theme stylesheet that
// matches a manifest theme name. Extra CSS files are rejected so authors do
// not assume that an unloaded file affects the skin.
func skinStylesheets(files map[string]os.FileInfo, manifest *Manifest) ([]string, error) {
	result := []string{"styles.css"}
	for relative := range files {
		if !strings.HasSuffix(strings.ToLower(relative), ".css") || relative == "styles.css" {
			continue
		}
		if strings.HasPrefix(relative, "themes/") && !strings.Contains(strings.TrimPrefix(relative, "themes/"), "/") {
			theme := strings.TrimSuffix(strings.TrimPrefix(relative, "themes/"), ".css")
			if contains(manifest.Themes, theme) {
				result = append(result, relative)
				continue
			}
		}
		return nil, validationError(fmt.Sprintf("unexpected stylesheet: %s", relative))
	}
	sort.Strings(result)
	return result, nil
}

func validateManifest(data []byte) (*Manifest, error) {
	schema, _, err := schemas()
	if err != nil {
		return nil, fmt.Errorf("compile skin schemas: %w", err)
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, validationError("manifest.json is not valid JSON")
	}
	if err := schema.Validate(document); err != nil {
		return nil, validationError("manifest.json does not match schema v1: " + compactSchemaError(err))
	}

	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, validationError("manifest.json could not be decoded: " + err.Error())
	}
	if manifest.ID == BuiltinID {
		return nil, validationError("the id \"clarity\" is reserved for the built-in skin")
	}
	if !slugPattern.MatchString(manifest.ID) {
		return nil, validationError("manifest id must be a lowercase slug")
	}
	if !contains(manifest.Themes, manifest.DefaultTheme) {
		return nil, validationError("defaultTheme must be one of the declared themes")
	}
	if !contains(manifest.Visualizer.Modes, manifest.Visualizer.Default) {
		return nil, validationError("visualizer.default must be one of the declared modes")
	}
	if manifest.Visualizer.Intensity < 0 || manifest.Visualizer.Intensity > 1 {
		return nil, validationError("visualizer.intensity must be between 0 and 1")
	}
	if len(manifest.Capabilities) == 0 {
		return nil, validationError("at least one capability is required")
	}
	return &manifest, nil
}

func validateLayout(data []byte) (*Layout, error) {
	_, schema, err := schemas()
	if err != nil {
		return nil, fmt.Errorf("compile skin schemas: %w", err)
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, validationError("layout.json is not valid JSON")
	}
	if err := schema.Validate(document); err != nil {
		return nil, validationError("layout.json does not match schema v1: " + compactSchemaError(err))
	}

	var layout Layout
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&layout); err != nil {
		return nil, validationError("layout.json could not be decoded: " + err.Error())
	}
	return &layout, nil
}

func readLimitedFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, validationError(filepath.Base(path) + " must be a regular file")
	}
	if info.Size() > limit {
		return nil, validationError(filepath.Base(path) + " exceeds its size limit")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, validationError(filepath.Base(path) + " exceeds its size limit")
	}
	return data, nil
}

func allowedExtension(relative, extension string) bool {
	if relative == "manifest.json" || relative == "layout.json" || relative == "styles.css" {
		return true
	}
	return extension == ".css" || isAssetExtension(extension)
}

func isAssetExtension(extension string) bool {
	switch extension {
	case ".png", ".jpg", ".jpeg", ".webp", ".woff2":
		return true
	default:
		return false
	}
}

func fileSizeLimit(relative, extension string) int64 {
	switch relative {
	case "manifest.json":
		return maxManifestSize
	case "layout.json":
		return maxLayoutSize
	}
	if extension == ".css" {
		return maxCSSSize
	}
	return maxAssetSize
}

func validateAssetFile(path string, extension string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	header := make([]byte, 12)
	read, err := io.ReadFull(file, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return validationError(fmt.Sprintf("asset is unreadable: %s", filepath.Base(path)))
	}
	header = header[:read]

	valid := false
	switch extension {
	case ".png":
		valid = bytes.HasPrefix(header, []byte("\x89PNG\r\n\x1a\n"))
	case ".jpg", ".jpeg":
		valid = len(header) >= 3 && header[0] == 0xff && header[1] == 0xd8 && header[2] == 0xff
	case ".webp":
		valid = len(header) >= 12 && string(header[:4]) == "RIFF" && string(header[8:12]) == "WEBP"
	case ".woff2":
		valid = bytes.HasPrefix(header, []byte("wOF2"))
	}
	if !valid {
		return validationError(fmt.Sprintf("asset content does not match its extension: %s", filepath.Base(path)))
	}
	return nil
}

func compactSchemaError(err error) string {
	message := err.Error()
	message = strings.ReplaceAll(message, "\n", " ")
	message = strings.ReplaceAll(message, "  ", " ")
	if len(message) > 600 {
		message = message[:600] + "…"
	}
	return message
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
