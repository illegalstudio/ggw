package cli

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/illegalstudio/ggw/internal/worktree"
)

var (
	homeOnce  sync.Once
	homeRaw   string
	homeReal  string // filepath.EvalSymlinks(homeRaw); only set when it differs
	homeReady bool

	wtBaseOnce  sync.Once
	wtBaseRaw   string
	wtBaseReal  string
	wtBaseReady bool
)

func resolveHome() {
	homeOnce.Do(func() {
		h, err := os.UserHomeDir()
		if err != nil || h == "" {
			return
		}
		homeRaw = h
		homeReady = true
		if real, err := filepath.EvalSymlinks(h); err == nil && real != h {
			homeReal = real
		}
	})
}

func resolveWorktreesBase() {
	wtBaseOnce.Do(func() {
		b, err := worktree.WorktreesBase()
		if err != nil || b == "" {
			return
		}
		wtBaseRaw = b
		wtBaseReady = true
		// EvalSymlinks may fail if the dir doesn't exist yet; that's fine,
		// we just won't have a canonical form to compare against.
		if real, err := filepath.EvalSymlinks(b); err == nil && real != b {
			wtBaseReal = real
		}
	})
}

// displayPath returns a human-friendly version of an absolute path with $HOME
// collapsed to "~". It also tries the symlink-resolved $HOME (covers macOS,
// where git canonicalizes paths to /private/var/... while $HOME may still be
// /var/...).
//
// Use only for output meant to be read by a human — never for stdout that
// another process (e.g. the `ggw cd` shell wrapper) consumes, because shells
// don't expand tildes inside quoted strings.
func displayPath(p string) string {
	resolveHome()
	if !homeReady {
		return p
	}
	for _, h := range [...]string{homeRaw, homeReal} {
		if h == "" {
			continue
		}
		if p == h {
			return "~"
		}
		if strings.HasPrefix(p, h+string(os.PathSeparator)) {
			return "~" + p[len(h):]
		}
	}
	return p
}

// compactPath is a stronger version of displayPath used by `ggw list` only:
// for paths under the ggw worktrees base, the entire shared prefix becomes
// "[...]"; everything else falls back to tildification.
//
//	~/.local/share/worktrees/acme/api/feature-x  →  [...]/acme/api/feature-x
//	~/code/some-repo                             →  ~/code/some-repo
func compactPath(p string) string {
	resolveWorktreesBase()
	if wtBaseReady {
		for _, b := range [...]string{wtBaseRaw, wtBaseReal} {
			if b == "" {
				continue
			}
			if strings.HasPrefix(p, b+string(os.PathSeparator)) {
				return "[...]" + p[len(b):]
			}
		}
	}
	return displayPath(p)
}
