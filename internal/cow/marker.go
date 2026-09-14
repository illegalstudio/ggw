package cow

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// MarkerName is the file that identifies a directory as a ggw copy-on-write
// workspace. It lives inside .git/ rather than the work tree, so it never
// shows up in `git status` and never risks being committed.
const MarkerName = "ggw.json"

// MarkerVersion is bumped when the on-disk shape changes incompatibly.
const MarkerVersion = 1

// Marker is the metadata ggw writes into a snapshot so it can later be
// discovered, attributed to a repository, and traced back to its source.
type Marker struct {
	Kind    string `json:"kind"`    // always "cow"
	Version int    `json:"version"` // MarkerVersion
	// SourceRepo is the main worktree this snapshot was taken from. It is how
	// ggw finds the rest of the repository's workspaces when run from inside a
	// snapshot, which git alone cannot answer.
	SourceRepo string `json:"source_repo"`
	Org        string `json:"org"`
	Repo       string `json:"repo"`
	// Branch is the branch checked out at creation time, kept for diagnostics
	// only — the live answer always comes from git.
	Branch    string `json:"branch,omitempty"`
	Backend   string `json:"backend"`
	CreatedAt string `json:"created_at"`
}

// MarkerPath returns where the marker lives for the workspace at path.
func MarkerPath(workspacePath string) string {
	return filepath.Join(workspacePath, ".git", MarkerName)
}

// Is reports whether path is a ggw copy-on-write workspace. It is the cheap
// check used when scanning a directory full of candidates.
func Is(workspacePath string) bool {
	info, err := os.Stat(MarkerPath(workspacePath))
	return err == nil && info.Mode().IsRegular()
}

// Read loads the marker of the workspace at path. A path that is not a
// copy-on-write workspace yields (nil, false, nil); a marker that exists but
// cannot be parsed is an error, because silently treating it as a plain
// worktree would hide a real problem.
func Read(workspacePath string) (*Marker, bool, error) {
	// A linked worktree's .git is a file, so looking inside it fails with
	// ENOTDIR rather than ENOENT. Check the shape first and keep the error path
	// below for problems that are actually worth reporting.
	if info, err := os.Stat(filepath.Join(workspacePath, ".git")); err != nil || !info.IsDir() {
		return nil, false, nil
	}

	path := MarkerPath(workspacePath)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("cannot read %s: %w", path, err)
	}

	var m Marker
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, true, fmt.Errorf("cannot parse %s: %w", path, err)
	}
	return &m, true, nil
}

// Write stores the marker for the workspace at path, stamping the fields that
// are not the caller's to choose.
func Write(workspacePath string, m Marker) error {
	m.Kind = "cow"
	m.Version = MarkerVersion
	m.Backend = Backend
	m.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot encode workspace marker: %w", err)
	}
	path := MarkerPath(workspacePath)
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	return nil
}
