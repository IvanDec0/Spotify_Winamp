package webplayer

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// extractLimits bounds one archive extraction. A non-positive value disables
// the corresponding limit, but Runtime always sets all of them.
type extractLimits struct {
	MaxFileBytes  int64
	MaxTotalBytes int64
	MaxFiles      int
}

// extractRuntimeZip extracts a runtime archive into destination after
// rejecting anything that could escape it or exhaust the disk. It is zip-only:
// the castlabs runtime ships as a plain zip with entries at the root.
//
// The archive may instead wrap everything in a single top-level directory;
// that directory is stripped so the entry executable always ends up directly
// under destination.
func extractRuntimeZip(zipPath, destination, entry string, limits extractLimits) error {
	if err := extractZipMembers(zipPath, destination, entry, limits); err != nil {
		// Never leave a partial runtime behind; the caller's staging
		// directory is removed anyway, but a direct call gets the same
		// guarantee.
		_ = os.RemoveAll(destination)
		return err
	}
	return nil
}

func extractZipMembers(zipPath, destination, entry string, limits extractLimits) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("the web player runtime archive is not a readable zip file: %w", err)
	}
	defer reader.Close()

	if limits.MaxFiles > 0 && len(reader.File) > limits.MaxFiles {
		return fmt.Errorf("the web player runtime archive has %d members, more than the %d limit", len(reader.File), limits.MaxFiles)
	}

	prefix, err := runtimeRootPrefix(reader.File)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}

	seen := make(map[string]struct{}, len(reader.File))
	var (
		files int
		total int64
	)
	for _, member := range reader.File {
		name, err := memberName(member.Name)
		if err != nil {
			return err
		}
		if prefix != "" {
			switch {
			case name == prefix:
				name = ""
			case strings.HasPrefix(name, prefix+"/"):
				name = strings.TrimPrefix(name, prefix+"/")
			default:
				return fmt.Errorf("archive member %q is outside the runtime directory %q", member.Name, prefix)
			}
		}
		if name == "" {
			if !member.FileInfo().IsDir() {
				return fmt.Errorf("archive member %q is not a directory", member.Name)
			}
			continue
		}
		if !filepath.IsLocal(filepath.FromSlash(name)) {
			return fmt.Errorf("archive member %q is not a local path", member.Name)
		}
		if _, duplicate := seen[name]; duplicate {
			return fmt.Errorf("archive member %q appears more than once", member.Name)
		}
		seen[name] = struct{}{}

		target := filepath.Join(destination, filepath.FromSlash(name))
		mode := member.Mode()
		switch {
		case mode&os.ModeSymlink != 0:
			return fmt.Errorf("archive member %q is a symbolic link", member.Name)
		case member.FileInfo().IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case mode.IsRegular():
			files++
			if limits.MaxFiles > 0 && files > limits.MaxFiles {
				return fmt.Errorf("the web player runtime archive has more than %d files", limits.MaxFiles)
			}
			if limits.MaxFileBytes > 0 && member.UncompressedSize64 > uint64(limits.MaxFileBytes) {
				return fmt.Errorf("archive member %q is larger than the per-file limit", member.Name)
			}
			written, err := extractMember(member, target, limits.MaxFileBytes)
			if err != nil {
				return err
			}
			total += written
			if limits.MaxTotalBytes > 0 && total > limits.MaxTotalBytes {
				return fmt.Errorf("the web player runtime archive expands beyond the total size limit")
			}
		default:
			return fmt.Errorf("archive member %q is not a regular file or directory", member.Name)
		}
	}

	if files == 0 {
		return errors.New("the web player runtime archive contains no files")
	}

	entryPath := filepath.Join(destination, filepath.FromSlash(entry))
	info, err := os.Stat(entryPath)
	if err != nil {
		return fmt.Errorf("the web player runtime archive does not contain the entry executable %q: %w", entry, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("the web player runtime entry %q is not a regular file", entry)
	}
	if err := os.Chmod(entryPath, 0o755); err != nil {
		return fmt.Errorf("cannot make the web player runtime entry executable: %w", err)
	}
	return nil
}

// extractMember writes one regular file and returns its size. It verifies the
// zip CRC by closing the member reader, so a corrupted archive cannot produce
// a silently truncated runtime.
func extractMember(member *zip.File, target string, maxFile int64) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return 0, err
	}
	source, err := member.Open()
	if err != nil {
		return 0, err
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		_ = source.Close()
		return 0, err
	}

	var reader io.Reader = source
	if maxFile > 0 {
		reader = io.LimitReader(source, maxFile+1)
	}
	written, copyErr := io.Copy(file, reader)
	// Closing the zip member is where archive/zip checks the CRC.
	sourceErr := source.Close()
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(target)
		return 0, copyErr
	}
	if maxFile > 0 && written > maxFile {
		_ = os.Remove(target)
		return 0, fmt.Errorf("archive member %q expands beyond the per-file limit", member.Name)
	}
	if sourceErr != nil {
		_ = os.Remove(target)
		return 0, sourceErr
	}
	if closeErr != nil {
		_ = os.Remove(target)
		return 0, closeErr
	}

	// Preserve the executable bits the runtime needs (chrome-sandbox and the
	// Electron binary itself) without ever granting group or other write.
	perm := member.Mode().Perm()
	if perm == 0 {
		perm = 0o644
	}
	perm &= 0o755
	if perm == 0 {
		perm = 0o644
	}
	if err := os.Chmod(target, perm); err != nil {
		_ = os.Remove(target)
		return 0, err
	}
	return written, nil
}

// runtimeRootPrefix returns the single top-level directory wrapping every
// file in the archive, or "" when the entries already live at the root. The
// castlabs zip uses the latter form, but normalizing both keeps the installer
// robust.
func runtimeRootPrefix(members []*zip.File) (string, error) {
	prefix := ""
	first := true
	for _, member := range members {
		if member.FileInfo().IsDir() {
			continue
		}
		name, err := memberName(member.Name)
		if err != nil {
			return "", err
		}
		if name == "" {
			return "", fmt.Errorf("archive member %q has an empty name", member.Name)
		}
		top, _, _ := strings.Cut(name, "/")
		if first {
			prefix = top
			first = false
			continue
		}
		if top != prefix {
			return "", nil
		}
	}
	if first {
		return "", nil
	}
	// Only strip the prefix when every file is nested below it. A file whose
	// own name equals the prefix is already at the root.
	for _, member := range members {
		if member.FileInfo().IsDir() {
			continue
		}
		name, err := memberName(member.Name)
		if err != nil {
			return "", err
		}
		if name == prefix {
			return "", nil
		}
	}
	return prefix, nil
}

// memberName normalizes an archive member name and rejects absolute paths,
// drive letters and parent traversal.
func memberName(name string) (string, error) {
	if name == "" {
		return "", errors.New("archive member has an empty name")
	}
	normalized := strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(normalized, "/") {
		return "", fmt.Errorf("archive member %q has an absolute path", name)
	}
	if len(normalized) >= 2 && normalized[1] == ':' {
		return "", fmt.Errorf("archive member %q has an absolute path", name)
	}
	if strings.ContainsRune(normalized, 0) {
		return "", fmt.Errorf("archive member %q contains a NUL byte", name)
	}
	parts := strings.Split(normalized, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", fmt.Errorf("archive member %q escapes the extraction directory", name)
		default:
			cleaned = append(cleaned, part)
		}
	}
	if len(cleaned) == 0 {
		return "", nil
	}
	return strings.Join(cleaned, "/"), nil
}
