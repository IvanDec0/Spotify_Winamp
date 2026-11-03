package skins

import (
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ServeAsset serves files from the active declarative skin. Only stylesheets
// and validated asset types are exposed; manifests, layouts and arbitrary
// files never reach the webview. Paths are canonicalized so encoded traversal
// cannot escape the skin root.
func (m *Manager) ServeAsset(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.Header().Set("Allow", "GET, HEAD")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	const prefix = "/skins/"
	if !strings.HasPrefix(request.URL.Path, prefix) {
		http.NotFound(writer, request)
		return
	}
	remainder := strings.TrimPrefix(request.URL.Path, prefix)
	parts := strings.SplitN(remainder, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.NotFound(writer, request)
		return
	}
	id, relative := parts[0], parts[1]
	activeID, root, ok := m.ActiveSkinDir()
	if !ok || id != activeID {
		http.NotFound(writer, request)
		return
	}

	if strings.ContainsRune(relative, 0) || strings.Contains(relative, "\\") || strings.HasPrefix(relative, "/") {
		http.NotFound(writer, request)
		return
	}
	cleaned := path.Clean(relative)
	if cleaned != relative || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		http.NotFound(writer, request)
		return
	}
	extension := strings.ToLower(path.Ext(cleaned))
	if extension != ".css" && !isAssetExtension(extension) {
		http.NotFound(writer, request)
		return
	}

	target := filepath.Join(root, filepath.FromSlash(cleaned))
	info, err := os.Lstat(target)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		http.NotFound(writer, request)
		return
	}
	file, err := os.Open(target)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	defer file.Close()

	contentType := mime.TypeByExtension(extension)
	switch extension {
	case ".css":
		contentType = "text/css; charset=utf-8"
	case ".png":
		contentType = "image/png"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".webp":
		contentType = "image/webp"
	case ".woff2":
		contentType = "font/woff2"
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	writer.Header().Set("Content-Type", contentType)
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(writer, request, filepath.Base(cleaned), info.ModTime(), file)
}

// AssetHandler routes /skins/ to the manager and everything else to the
// supplied frontend handler.
func AssetHandler(manager *Manager, frontend http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/skins/") {
			manager.ServeAsset(writer, request)
			return
		}
		frontend.ServeHTTP(writer, request)
	})
}

// Handler is a convenience wrapper used by tests.
func (m *Manager) Handler() http.Handler {
	return http.HandlerFunc(m.ServeAsset)
}
