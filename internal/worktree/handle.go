package worktree

import (
	"path/filepath"
	"strings"
)

// Handles returns a stable, typeable handle for each worktree in list,
// aligned by index.
//
// A worktree with a branch uses the branch name. A branchless worktree
// (detached / bare) uses the minimal number of trailing path segments that is
// unique among all worktrees and distinct from every branch name — e.g. a
// detached "~/.codex/worktrees/0e21/elephc" whose basename "elephc" collides
// with the main worktree becomes "0e21/elephc".
func Handles(list []Worktree) []string {
	handles := make([]string, len(list))
	reserved := make(map[string]bool)
	for _, w := range list {
		if w.Branch != "" {
			reserved[w.Branch] = true
		}
	}
	for i, w := range list {
		if w.Branch != "" {
			handles[i] = w.Branch
			continue
		}
		handles[i] = uniquePathSuffix(list, i, reserved)
	}
	return handles
}

// uniquePathSuffix grows the trailing path segments of list[i] until the
// resulting "/"-joined string is unique among every other worktree's path
// (compared at the same depth) and is not a reserved branch name.
func uniquePathSuffix(list []Worktree, i int, reserved map[string]bool) string {
	segs := pathSegments(list[i].Path)
	for k := 1; k <= len(segs); k++ {
		cand := lastSegments(segs, k)
		if reserved[cand] {
			continue
		}
		collision := false
		for j := range list {
			if j == i {
				continue
			}
			if lastSegments(pathSegments(list[j].Path), k) == cand {
				collision = true
				break
			}
		}
		if !collision {
			return cand
		}
	}
	// Fallback: the full path is always unique.
	return filepath.ToSlash(list[i].Path)
}

func pathSegments(p string) []string {
	raw := strings.Split(filepath.ToSlash(p), "/")
	segs := raw[:0]
	for _, s := range raw {
		if s != "" {
			segs = append(segs, s)
		}
	}
	return segs
}

func lastSegments(segs []string, k int) string {
	if k > len(segs) {
		k = len(segs)
	}
	return strings.Join(segs[len(segs)-k:], "/")
}
