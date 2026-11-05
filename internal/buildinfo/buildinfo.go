// Package buildinfo exposes the release version and commit injected at build
// time through -ldflags -X. Local builds keep the development defaults.
package buildinfo

// Version is the release version stamped by the build.
var Version = "dev"

// Commit is the short Git commit stamped by the build.
var Commit = "unknown"

// String returns the version alone when no commit is available, otherwise the
// version followed by the commit in parentheses.
func String() string {
	if Commit == "" || Commit == "unknown" {
		return Version
	}
	return Version + " (" + Commit + ")"
}
