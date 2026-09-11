// Package version holds build information, set with -ldflags -X.
package version

var (
	// Version is the release version, e.g. v1.1.0.
	Version = "dev"
	// Commit is the short git commit the binary was built from.
	Commit = ""
)
