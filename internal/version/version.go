package version

import "fmt"

// Set via ldflags at build time (see .goreleaser.yaml).
var (
	Version = "dev"
	Commit  = "none"
)

// String returns a human-readable version string.
func String() string {
	return fmt.Sprintf("ggw v%s (%s)", Version, Commit)
}
