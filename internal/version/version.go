// Package version holds build metadata for CodeSteward. The values are
// overridden at build time via -ldflags -X on these variables.
package version

var (
	// Version is the semantic version of the build.
	Version = "0.1.0-dev"
	// Commit is the git commit the binary was built from.
	Commit = "none"
	// Date is the build timestamp.
	Date = "unknown"
)
