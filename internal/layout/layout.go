// Package layout resolves where ggw stores workspaces on disk and how a
// repository maps onto that layout.
package layout

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/illegalstudio/ggw/internal/config"
)

// OriginOrgRepo returns the (org, repo) pair derived from the `origin` remote
// of the git repository at repoPath.
//
// If the repo has no `origin` remote configured, an explicit error is returned
// — the caller is expected to surface it. (We intentionally do not fall back
// to using the directory name: the storage layout assumes a stable
// org/repo pair, and silently inventing one is more surprising than failing.)
func OriginOrgRepo(repoPath string) (string, string, error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("no `origin` remote configured for %s (configure one with `git remote add origin …`)", repoPath)
	}
	return ParseOrgRepo(strings.TrimSpace(string(out)))
}

// ParseOrgRepo extracts the (org, repo) pair from a git remote URL.
//
// Supports SSH form (git@github.com:org/repo.git) and HTTPS form
// (https://github.com/org/repo.git). The trailing `.git` is optional.
//
// For URLs with deeper paths (e.g. GitLab subgroups: group/sub/repo), the
// last segment is taken as the repo and the second-to-last as the org. This
// loses the subgroup but keeps things flat under the worktrees dir.
func ParseOrgRepo(rawURL string) (string, string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", "", fmt.Errorf("empty remote URL")
	}

	var path string
	if strings.Contains(rawURL, "@") && strings.Contains(rawURL, ":") && !strings.Contains(rawURL, "://") {
		// SSH form: git@host:org/repo.git
		colonIdx := strings.Index(rawURL, ":")
		path = rawURL[colonIdx+1:]
	} else {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return "", "", fmt.Errorf("invalid remote URL %q: %w", rawURL, err)
		}
		path = parsed.Path
	}

	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")

	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" || parts[len(parts)-1] == "" {
		return "", "", fmt.Errorf("cannot extract org/repo from %q", rawURL)
	}

	org := parts[len(parts)-2]
	repo := parts[len(parts)-1]
	return org, repo, nil
}

// SlugifyBranch turns a git branch name into a directory-safe slug:
// lowercased, alphanumerics and underscores preserved, every other rune
// folded to a single `-`, with no leading/trailing dashes.
//
// Examples:
//
//	"feature/login"  -> "feature-login"
//	"BugFix/User Auth" -> "bugfix-user-auth"
//	"hotfix-123"     -> "hotfix-123"
func SlugifyBranch(branch string) string {
	var b strings.Builder
	b.Grow(len(branch))

	lastDash := false
	for _, r := range strings.ToLower(branch) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	return strings.TrimRight(b.String(), "-")
}

// Base returns the directory under which all ggw-managed workspaces live. A
// base_dir set in ~/.config/ggw/config.yaml wins; otherwise
// $XDG_DATA_HOME/worktrees, or ~/.local/share/worktrees as a fallback.
func Base() (string, error) {
	if dir, ok, err := config.BaseDir(); err != nil {
		return "", err
	} else if ok {
		// Config wins: the configured directory is the worktrees base, used
		// directly (no /worktrees suffix — that belongs to the default only).
		return dir, nil
	}

	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "worktrees"), nil
}

// RepoDir returns the directory holding every workspace of (org, repo):
// <base>/<org>/<repo>. It is the directory ggw scans to discover copy-on-write
// workspaces, which git itself knows nothing about.
func RepoDir(org, repo string) (string, error) {
	base, err := Base()
	if err != nil {
		return "", err
	}
	if org == "" || repo == "" {
		return "", fmt.Errorf("repo dir components must be non-empty (org=%q repo=%q)", org, repo)
	}
	return filepath.Join(base, org, repo), nil
}

// WorkspacePath returns the absolute path where a workspace for
// (org, repo, slug) should live. Both worktrees and copy-on-write workspaces
// share this namespace, so a slug is taken by whichever kind claimed it first.
func WorkspacePath(org, repo, slug string) (string, error) {
	dir, err := RepoDir(org, repo)
	if err != nil {
		return "", err
	}
	if slug == "" {
		return "", fmt.Errorf("workspace path components must be non-empty (org=%q repo=%q slug=%q)", org, repo, slug)
	}
	return filepath.Join(dir, slug), nil
}

// EnsureFreeDestination fails if destPath already exists and otherwise creates
// its parent directory, so a caller can write the workspace into it.
func EnsureFreeDestination(destPath string) error {
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("path already exists: %s", destPath)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("cannot create parent directory: %w", err)
	}
	return nil
}

// RepoRoot returns the absolute path of the git repo containing cwd.
// Returns an error if cwd is not inside a git work tree.
func RepoRoot(cwd string) (string, error) {
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository (run from within a repo)")
	}
	return strings.TrimSpace(string(out)), nil
}
