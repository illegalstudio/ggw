// Package cow creates copy-on-write workspaces: whole-directory snapshots of a
// git repository whose data blocks are shared with the original until one side
// writes to them.
//
// Unlike a git worktree, the result is an independent repository that also
// carries every untracked file — node_modules, vendor, .env, build output — at
// no disk cost. The price is that git knows nothing about it: a snapshot is
// discovered on disk, by the marker ggw leaves in its .git directory.
package cow

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Backend names the mechanism used to take a snapshot. Only reflink exists
// today; the name is recorded in each workspace's marker so a future backend
// (btrfs subvolumes, zfs clones) can be told apart from it after the fact.
const Backend = "reflink"

// Clone snapshots src into dst, which must not exist.
//
// The copy is reflink-only on purpose: if the filesystem cannot share blocks,
// the call fails instead of silently duplicating what may be many gigabytes of
// untracked files. Timestamps are preserved so the copied git index stays
// valid and the first `git status` does not re-hash the whole tree.
func Clone(src, dst string) error {
	if cloneName == "" {
		return fmt.Errorf("copy-on-write workspaces are not supported on %s", runtime.GOOS)
	}
	// `cp` given an existing destination copies *into* it rather than failing,
	// which would quietly produce dst/<basename> instead of dst.
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("path already exists: %s", dst)
	}
	dstParent := filepath.Dir(dst)

	// Ask about one small file first. `cp` reports every file it cannot clone,
	// so on an unsupported filesystem the real copy would answer a simple
	// question with thousands of identical lines.
	if err := probeClone(src, dstParent); err != nil {
		return err
	}

	cmd := exec.Command(cloneName, cloneArgs(src, dst)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// A failed clone can leave a partial tree behind; the caller asked for
		// a workspace, not for debris.
		_ = os.RemoveAll(dst)
		// The probe just shared blocks between these two filesystems, so this
		// is something else entirely — permissions, disk space, a file that
		// cannot be read. Say what happened instead of inventing a cause.
		return fmt.Errorf("cannot snapshot %s into %s: %w%s", src, dst, err, detail(stderr.String()))
	}
	return nil
}

// probeClone reflinks a single small file from src's filesystem into dst's, to
// learn whether the real copy can work at all.
//
// It clones a file the source already has rather than writing one, so nothing
// is ever created inside the user's repository. When there is nothing to probe
// with, it stays silent and lets the real copy give the verdict.
func probeClone(src, dstParent string) error {
	sample := filepath.Join(src, ".git", "HEAD")
	if _, err := os.Stat(sample); err != nil {
		return nil
	}

	tmp, err := os.CreateTemp(dstParent, ".ggw-cow-probe-*")
	if err != nil {
		// Not being able to put a file in the destination is itself the answer,
		// and a far better one than whatever the full copy would report.
		return fmt.Errorf("cannot write into %s: %w", dstParent, err)
	}
	probe := tmp.Name()
	_ = tmp.Close()
	// cp refuses to clone onto an existing file, so only the name is wanted.
	_ = os.Remove(probe)
	defer func() { _ = os.Remove(probe) }()

	cmd := exec.Command(cloneName, cloneArgs(sample, probe)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return explainCloneFailure(src, dstParent, stderr.String())
	}
	return nil
}

// cloneName and cloneArgs produce the argv that reflink-copies src to dst on
// this platform: GNU coreutils on Linux, clonefile(2) via BSD cp on macOS.
// Both refuse to fall back to a full copy, which is exactly what we want.
var cloneName = map[string]string{"linux": "cp", "darwin": "cp"}[runtime.GOOS]

func cloneArgs(src, dst string) []string {
	if runtime.GOOS == "darwin" {
		return []string{"-a", "-c", "--", src, dst}
	}
	return []string{"-a", "--reflink=always", "--", src, dst}
}

// explainCloneFailure turns a failed *probe* into the two answers a user
// actually needs: whether the two paths are on the same filesystem, and
// whether that filesystem can share blocks at all.
//
// It is only ever reached from probeClone. Once the probe has succeeded,
// reflinks demonstrably work here and no later failure may be blamed on them.
func explainCloneFailure(src, dstParent, stderr string) error {
	srcDev, srcOK := deviceID(src)
	dstDev, dstOK := deviceID(dstParent)
	if srcOK && dstOK && srcDev != dstDev {
		return fmt.Errorf(
			"cannot snapshot %s into %s: copy-on-write clones cannot cross filesystems (%s is on %s, %s is on %s)%s",
			src, dstParent,
			src, filesystemLabel(src),
			dstParent, filesystemLabel(dstParent),
			detail(stderr),
		)
	}

	return fmt.Errorf(
		"cannot snapshot %s: the filesystem (%s) does not support copy-on-write clones — use btrfs, XFS with reflink=1, bcachefs or APFS, or create worktrees instead (--wt)%s",
		src, filesystemLabel(src), detail(stderr),
	)
}

// detail appends a command's stderr, capped: `cp` emits one line per file it
// could not handle, and a repository can have a great many files.
func detail(stderr string) string {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return ""
	}

	const max = 5
	if len(lines) > max {
		omitted := len(lines) - max
		lines = append(lines[:max:max], fmt.Sprintf("(and %d more)", omitted))
	}
	return ": " + strings.Join(lines, "; ")
}

func filesystemLabel(path string) string {
	if name := filesystemName(path); name != "" {
		return name
	}
	return "unknown filesystem"
}

// ValidateSource fails unless src can be snapshotted into a working repository.
//
// Only a main worktree qualifies: a linked worktree's .git is a *file* pointing
// back into the original repository, so a copy of it would resolve its objects
// through a path outside itself and break the moment the original moves.
func ValidateSource(src string) error {
	gitPath := filepath.Join(src, ".git")
	info, err := os.Lstat(gitPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s is not a git repository (no .git)", src)
		}
		return fmt.Errorf("cannot inspect %s: %w", gitPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is a linked worktree, not a repository: snapshot its main worktree instead", src)
	}
	return nil
}

// Sanitize strips the state a snapshot must not inherit from its source.
//
// The copied .git/worktrees registers the *source's* worktrees, which makes git
// in the snapshot refuse to check out any branch already used by one of them
// ("fatal: 'x' is already used by worktree at ..."). Those entries also point
// at directories that belong to another repository, so `git worktree prune`
// deliberately keeps them — they have to be removed outright.
func Sanitize(workspacePath string) error {
	gitDir := filepath.Join(workspacePath, ".git")

	if err := os.RemoveAll(filepath.Join(gitDir, "worktrees")); err != nil {
		return fmt.Errorf("cannot clear inherited worktree registrations: %w", err)
	}
	// A snapshot taken while the source was mid-commit carries its lock file.
	if err := os.Remove(filepath.Join(gitDir, "index.lock")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("cannot remove inherited index lock: %w", err)
	}
	return nil
}
