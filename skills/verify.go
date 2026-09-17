package skills

import "fmt"

// VerifyStatus describes how an installed skill compares to the one bundled
// with this binary.
type VerifyStatus string

const (
	// VerifyNotInstalled means no skill exists at the destination.
	VerifyNotInstalled VerifyStatus = "not-installed"
	// VerifyUpToDate means the destination matches the bundled skill.
	VerifyUpToDate VerifyStatus = "up-to-date"
	// VerifyOutdated means this installer wrote the destination, it is
	// untouched, and it differs from the bundled skill — a relic of another
	// ggw version.
	VerifyOutdated VerifyStatus = "outdated"
	// VerifyModified means the destination differs from the bundled skill and
	// was either edited by the user or not written by this installer.
	VerifyModified VerifyStatus = "modified"
)

// Stale reports whether an installed skill disagrees with the bundled one.
// Absent and up-to-date destinations are not stale.
func (s VerifyStatus) Stale() bool {
	return s == VerifyOutdated || s == VerifyModified
}

// Verify compares the skill installed at destination with the bundled one,
// using the same digest machinery as Install.
func Verify(destination string) (VerifyStatus, error) {
	if destination == "" {
		return "", fmt.Errorf("skill destination is required")
	}

	bundledDigest, err := digestBundledSkill()
	if err != nil {
		return "", err
	}

	state, err := inspectInstalledSkill(destination, bundledDigest)
	if err != nil {
		return "", err
	}

	switch {
	case !state.exists:
		return VerifyNotInstalled, nil
	case !state.isDir:
		return "", fmt.Errorf("skill destination exists but is not a directory (a symlink?)")
	case state.current:
		return VerifyUpToDate, nil
	case state.managedUnmodified:
		return VerifyOutdated, nil
	default:
		return VerifyModified, nil
	}
}
