package skins

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	maxZipArchiveSize      = 20 << 20
	maxZipUncompressedSize = 100 << 20
	maxZipCompressionRatio = 100
)

type zipEntry struct {
	file     *zip.File
	relative string
	dir      bool
	limit    int64
}

// extractZip validates an archive completely before writing any file into
// destination. It returns the first validation error so the caller can remove
// the staging directory.
func extractZip(archivePath, destination string) error {
	info, err := os.Lstat(archivePath)
	if err != nil {
		return fmt.Errorf("open skin archive: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return validationError("skin archive must be a regular file")
	}
	if info.Size() > maxZipArchiveSize {
		return validationError("skin archive exceeds the 20 MB limit")
	}

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return validationError("skin archive is not a valid ZIP file")
	}
	defer reader.Close()

	if len(reader.File) > maxSkinFiles*2 {
		return validationError("skin archive contains too many entries")
	}

	entries := make([]zipEntry, 0, len(reader.File))
	seen := make(map[string]string)
	var totalUncompressed, totalCompressed uint64
	files := 0

	for _, file := range reader.File {
		if file.Flags&0x1 != 0 {
			return validationError("encrypted ZIP entries are not allowed")
		}
		if file.Method != zip.Store && file.Method != zip.Deflate {
			return validationError(fmt.Sprintf("unsupported ZIP compression method %d", file.Method))
		}

		relative, isDir, err := archiveEntryPath(file.Name)
		if err != nil {
			return err
		}
		folded := strings.ToLower(relative)
		if previous, exists := seen[folded]; exists {
			return validationError(fmt.Sprintf("duplicate archive entry: %s and %s", previous, relative))
		}
		seen[folded] = relative

		mode := file.Mode()
		if mode&os.ModeSymlink != 0 {
			return validationError(fmt.Sprintf("symbolic links are not allowed: %s", relative))
		}
		if mode&os.ModeType != 0 && !mode.IsDir() {
			return validationError(fmt.Sprintf("unsupported file type in archive: %s", relative))
		}
		if mode&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
			return validationError(fmt.Sprintf("special permissions are not allowed: %s", relative))
		}
		if isDir {
			if file.UncompressedSize64 != 0 {
				return validationError(fmt.Sprintf("directory entry contains data: %s", relative))
			}
			entries = append(entries, zipEntry{file: file, relative: relative, dir: true})
			continue
		}

		extension := strings.ToLower(path.Ext(relative))
		if !allowedExtension(relative, extension) {
			return validationError(fmt.Sprintf("unsupported file type in archive: %s", relative))
		}
		limit := fileSizeLimit(relative, extension)
		if file.UncompressedSize64 > uint64(limit) {
			return validationError(fmt.Sprintf("%s exceeds its size limit", relative))
		}
		if files >= maxSkinFiles {
			return validationError("skin archive exceeds the 500 file limit")
		}
		files++
		totalUncompressed += file.UncompressedSize64
		totalCompressed += file.CompressedSize64
		if totalUncompressed > maxZipUncompressedSize {
			return validationError("skin archive exceeds the 100 MB uncompressed limit")
		}
		entries = append(entries, zipEntry{file: file, relative: relative, limit: limit})
	}

	if totalUncompressed > 0 {
		if totalCompressed == 0 || totalUncompressed > uint64(maxZipCompressionRatio)*totalCompressed {
			return validationError("skin archive exceeds the 100:1 compression ratio limit")
		}
	}

	remaining := int64(maxZipUncompressedSize)
	for _, entry := range entries {
		target := filepath.Join(destination, filepath.FromSlash(entry.relative))
		if entry.dir {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return fmt.Errorf("create skin directory: %w", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return fmt.Errorf("create skin directory: %w", err)
		}
		written, err := extractZipFile(entry, target, remaining)
		if err != nil {
			return err
		}
		remaining -= written
		if remaining < 0 {
			return validationError("skin archive exceeds the 100 MB uncompressed limit")
		}
	}
	return nil
}

func extractZipFile(entry zipEntry, target string, budget int64) (int64, error) {
	source, err := entry.file.Open()
	if err != nil {
		return 0, validationError(fmt.Sprintf("cannot read archive entry: %s", entry.relative))
	}
	defer source.Close()

	destination, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, fmt.Errorf("create skin file: %w", err)
	}
	defer destination.Close()

	limit := entry.limit
	if budget < limit {
		limit = budget
	}
	written, err := io.Copy(destination, io.LimitReader(source, limit+1))
	if err != nil {
		return written, fmt.Errorf("extract %s: %w", entry.relative, err)
	}
	if written > entry.limit {
		return written, validationError(fmt.Sprintf("%s exceeds its size limit", entry.relative))
	}
	if written > budget {
		return written, validationError("skin archive exceeds the 100 MB uncompressed limit")
	}
	if err := destination.Sync(); err != nil {
		return written, err
	}
	return written, nil
}

func archiveEntryPath(name string) (string, bool, error) {
	if name == "" || strings.ContainsRune(name, 0) || strings.Contains(name, "\\") {
		return "", false, validationError("archive contains an unsafe path")
	}
	if strings.Contains(name, ":") {
		return "", false, validationError(fmt.Sprintf("archive entry contains a drive or scheme prefix: %s", name))
	}

	isDir := strings.HasSuffix(name, "/")
	trimmed := strings.TrimSuffix(name, "/")
	if trimmed == "" || strings.HasPrefix(trimmed, "/") {
		return "", false, validationError(fmt.Sprintf("archive entry is absolute: %s", name))
	}
	cleaned := path.Clean(trimmed)
	if cleaned != trimmed || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false, validationError(fmt.Sprintf("archive entry escapes the skin directory: %s", name))
	}
	return cleaned, isDir, nil
}

// copySkinDirectory copies a validated directory into a fresh staging
// directory. It re-checks symlinks and file types during the copy so a
// directory changing between validation and installation cannot escape the
// package root.
func copySkinDirectory(source, destination string) error {
	var totalSize int64
	var fileCount int
	return filepath.WalkDir(source, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == source {
			return nil
		}
		relative, err := filepath.Rel(source, current)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.Type()&os.ModeSymlink != 0 {
			return validationError(fmt.Sprintf("symbolic links are not allowed: %s", relative))
		}
		target := filepath.Join(destination, filepath.FromSlash(relative))
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !entry.Type().IsRegular() {
			return validationError(fmt.Sprintf("unsupported file type: %s", relative))
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		extension := strings.ToLower(path.Ext(relative))
		if !allowedExtension(relative, extension) {
			return validationError(fmt.Sprintf("unsupported file type: %s", relative))
		}
		if info.Size() > fileSizeLimit(relative, extension) {
			return validationError(fmt.Sprintf("%s exceeds its size limit", relative))
		}
		fileCount++
		if fileCount > maxSkinFiles {
			return validationError("skin package exceeds the 500 file limit")
		}
		totalSize += info.Size()
		if totalSize > maxSkinSize {
			return validationError("skin package exceeds the 50 MB uncompressed limit")
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		input, err := os.Open(current)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			_ = input.Close()
			return err
		}
		written, copyErr := io.Copy(output, io.LimitReader(input, info.Size()+1))
		closeInputErr := input.Close()
		closeOutputErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeInputErr != nil {
			return closeInputErr
		}
		if closeOutputErr != nil {
			return closeOutputErr
		}
		if written > info.Size() {
			return validationError(fmt.Sprintf("%s changed while it was being installed", relative))
		}
		return nil
	})
}

func isZipFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".zip")
}

func removeIfExists(path string) error {
	err := os.RemoveAll(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
