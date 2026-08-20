package skills

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	// Name is the directory name the skill is installed under.
	Name           = "ggw"
	markerFileName = ".ggw-managed.json"
)

// InstallStatus describes what Install did to a destination.
type InstallStatus string

const (
	StatusInstalled InstallStatus = "installed"
	StatusUnchanged InstallStatus = "up-to-date"
	StatusUpdated   InstallStatus = "updated"
	StatusReplaced  InstallStatus = "replaced"
)

// installMarker records which installer wrote a skill directory and the digest
// of the contents it wrote, so a later run can tell "ours, untouched" apart
// from "edited by the user".
type installMarker struct {
	Installer string `json:"installer"`
	Digest    string `json:"digest"`
}

type installedState struct {
	exists            bool
	current           bool
	markerCurrent     bool
	managedUnmodified bool
}

// AgentSkillsInstallPath returns the Agent Skills destination for a home directory.
func AgentSkillsInstallPath(home string) string {
	return filepath.Join(home, ".agents", "skills", Name)
}

// ClaudeSkillsInstallPath returns the Claude Code destination for a home directory.
func ClaudeSkillsInstallPath(home string) string {
	return filepath.Join(home, ".claude", "skills", Name)
}

// Install writes the bundled skill to destination. It is a no-op when the
// destination already matches the bundle, updates a copy this installer wrote
// and the user has not edited, and refuses to touch anything else unless
// replace is true.
func Install(destination string, replace bool) (InstallStatus, error) {
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
	if state.current {
		if !state.markerCurrent {
			if err := writeMarker(destination, bundledDigest); err != nil {
				return "", err
			}
		}
		return StatusUnchanged, nil
	}
	if state.exists && !state.managedUnmodified && !replace {
		return "", fmt.Errorf("skill already exists at %s and contains different files; rerun with --force to replace it", destination)
	}

	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("create skills directory: %w", err)
	}

	staging, err := stageBundledSkill(parent, bundledDigest)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)

	if !state.exists {
		if err := os.Rename(staging, destination); err != nil {
			return "", fmt.Errorf("install skill: %w", err)
		}
		return StatusInstalled, nil
	}

	if err := replaceInstalledSkill(destination, staging); err != nil {
		return "", err
	}
	if state.managedUnmodified {
		return StatusUpdated, nil
	}
	return StatusReplaced, nil
}

func inspectInstalledSkill(destination, bundledDigest string) (installedState, error) {
	info, err := os.Lstat(destination)
	if errors.Is(err, os.ErrNotExist) {
		return installedState{}, nil
	}
	if err != nil {
		return installedState{}, fmt.Errorf("inspect skill destination: %w", err)
	}

	state := installedState{exists: true}
	if !info.IsDir() {
		return state, nil
	}

	installedDigest, err := digestInstalledSkill(destination)
	if err != nil {
		return installedState{}, fmt.Errorf("inspect installed skill: %w", err)
	}
	state.current = installedDigest == bundledDigest

	marker, err := readMarker(destination)
	if err != nil {
		return installedState{}, err
	}
	state.markerCurrent = marker != nil && marker.Digest == bundledDigest
	state.managedUnmodified = marker != nil && marker.Digest == installedDigest
	return state, nil
}

// stageBundledSkill writes the bundle to a temporary directory inside parent,
// so that moving it into place is an atomic rename on the same filesystem.
func stageBundledSkill(parent, digest string) (string, error) {
	staging, err := os.MkdirTemp(parent, ".ggw-skill-")
	if err != nil {
		return "", fmt.Errorf("create skill staging directory: %w", err)
	}

	err = fs.WalkDir(bundled, Name, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relative, err := filepath.Rel(Name, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}

		target := filepath.Join(staging, filepath.FromSlash(relative))
		if entry.IsDir() {
			return os.Mkdir(target, 0o755)
		}

		contents, err := bundled.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, contents, 0o644)
	})
	if err != nil {
		os.RemoveAll(staging)
		return "", fmt.Errorf("stage bundled skill: %w", err)
	}

	if err := writeMarker(staging, digest); err != nil {
		os.RemoveAll(staging)
		return "", err
	}
	return staging, nil
}

// replaceInstalledSkill swaps staging in for destination, keeping the previous
// contents until the swap succeeds so it can be restored on failure.
func replaceInstalledSkill(destination, staging string) error {
	parent := filepath.Dir(destination)
	backup, err := reserveBackupPath(parent)
	if err != nil {
		return err
	}
	if err := os.Rename(destination, backup); err != nil {
		return fmt.Errorf("back up existing skill: %w", err)
	}

	if err := os.Rename(staging, destination); err != nil {
		if restoreErr := os.Rename(backup, destination); restoreErr != nil {
			return fmt.Errorf("install skill: %w; restore previous skill: %v", err, restoreErr)
		}
		return fmt.Errorf("install skill: %w", err)
	}

	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("skill installed, but old skill cleanup failed: %w", err)
	}
	return nil
}

func reserveBackupPath(parent string) (string, error) {
	backup, err := os.MkdirTemp(parent, ".ggw-skill-backup-")
	if err != nil {
		return "", fmt.Errorf("reserve skill backup: %w", err)
	}
	if err := os.Remove(backup); err != nil {
		return "", fmt.Errorf("prepare skill backup: %w", err)
	}
	return backup, nil
}

// readMarker returns nil (without an error) when the marker is absent,
// unreadable as JSON, or written by something other than this installer.
func readMarker(destination string) (*installMarker, error) {
	contents, err := os.ReadFile(filepath.Join(destination, markerFileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read skill installation marker: %w", err)
	}

	var marker installMarker
	if err := json.Unmarshal(contents, &marker); err != nil || marker.Installer != Name || marker.Digest == "" {
		return nil, nil
	}
	return &marker, nil
}

func writeMarker(destination, digest string) error {
	contents, err := json.MarshalIndent(installMarker{Installer: Name, Digest: digest}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode skill installation marker: %w", err)
	}
	contents = append(contents, '\n')

	temporary, err := os.CreateTemp(destination, ".ggw-marker-")
	if err != nil {
		return fmt.Errorf("create skill installation marker: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return fmt.Errorf("set skill installation marker permissions: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return fmt.Errorf("write skill installation marker: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close skill installation marker: %w", err)
	}
	if err := os.Rename(temporaryPath, filepath.Join(destination, markerFileName)); err != nil {
		return fmt.Errorf("install skill installation marker: %w", err)
	}
	return nil
}

func digestBundledSkill() (string, error) {
	digest := sha256.New()
	err := fs.WalkDir(bundled, Name, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relative, err := filepath.Rel(Name, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		relative = filepath.ToSlash(relative)

		if entry.IsDir() {
			return addDigestEntry(digest, 'd', relative, nil)
		}
		contents, err := bundled.ReadFile(path)
		if err != nil {
			return err
		}
		return addDigestEntry(digest, 'f', relative, contents)
	})
	if err != nil {
		return "", fmt.Errorf("digest bundled skill: %w", err)
	}
	return "sha256:" + hex.EncodeToString(digest.Sum(nil)), nil
}

// digestInstalledSkill hashes an installed tree the same way as the bundle,
// skipping the marker file so that writing the marker does not change the
// digest the marker records.
func digestInstalledSkill(destination string) (string, error) {
	digest := sha256.New()
	err := filepath.WalkDir(destination, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relative, err := filepath.Rel(destination, path)
		if err != nil {
			return err
		}
		if relative == "." || relative == markerFileName {
			return nil
		}
		relative = filepath.ToSlash(relative)

		switch {
		case entry.IsDir():
			return addDigestEntry(digest, 'd', relative, nil)
		case entry.Type()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return addDigestEntry(digest, 'l', relative, []byte(target))
		case entry.Type().IsRegular():
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return addDigestEntry(digest, 'f', relative, contents)
		default:
			return addDigestEntry(digest, 'o', relative, []byte(entry.Type().String()))
		}
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(digest.Sum(nil)), nil
}

// addDigestEntry writes a length-prefixed record so that different trees cannot
// collide by concatenating to the same byte stream.
func addDigestEntry(digest hash.Hash, kind byte, path string, contents []byte) error {
	if _, err := digest.Write([]byte{kind}); err != nil {
		return err
	}
	if err := binary.Write(digest, binary.BigEndian, uint64(len(path))); err != nil {
		return err
	}
	if _, err := digest.Write([]byte(path)); err != nil {
		return err
	}
	if err := binary.Write(digest, binary.BigEndian, uint64(len(contents))); err != nil {
		return err
	}
	_, err := digest.Write(contents)
	return err
}
