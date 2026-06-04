# External/Branchless Worktrees UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make branchless/external worktrees (e.g. a Codex `--detach` worktree) visible in `ggw list` and addressable in `ggw delete`/`ggw cd` via a unique, typeable handle.

**Architecture:** A new pure function `worktree.Handles(list)` assigns each worktree a stable handle — the branch name when present, otherwise the minimal number of trailing path segments that is unique across the list. The handle becomes the single source of truth for completion, exact-match resolution, and the `list` label. A small CLI helper flags worktrees that live outside the ggw-managed base as `[external]`.

**Tech Stack:** Go, cobra (commands + shell completion), huh (interactive selector), lipgloss (`internal/ui` styles).

---

## File Structure

- Create: `internal/worktree/handle.go` — pure `Handles` computation (no FS access).
- Create: `internal/worktree/handle_test.go` — unit tests for `Handles`.
- Modify: `internal/cli/cd.go` — `resolveOneWorktree` adds handle exact-match; `selectWorktree` labels with the handle.
- Modify: `internal/cli/completion.go` — completion suggests handles for branchless worktrees.
- Modify: `internal/cli/completion_test.go` — add tests for handle suggestion + resolution.
- Modify: `internal/cli/list.go` — handle as label for branchless, `(detached)`/`(bare)`/`[external]`/`[locked]` tags, `external` JSON field.
- Modify: `internal/cli/helpers.go` — `isExternalPath` helper.
- Modify: `tests/e2e_test.go` — end-to-end test for an external detached worktree.

---

## Task 1: `worktree.Handles` — unique handle computation

**Files:**
- Create: `internal/worktree/handle.go`
- Test: `internal/worktree/handle_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/worktree/handle_test.go`:

```go
package worktree

import (
	"reflect"
	"testing"
)

func TestHandles(t *testing.T) {
	tests := []struct {
		name string
		list []Worktree
		want []string
	}{
		{
			name: "branch worktrees use the branch name",
			list: []Worktree{
				{Path: "/Volumes/x/elephc", Branch: "main"},
				{Path: "/data/worktrees/org/elephc/feature-x", Branch: "feature/x"},
			},
			want: []string{"main", "feature/x"},
		},
		{
			name: "detached with unique basename uses the basename",
			list: []Worktree{
				{Path: "/Volumes/x/elephc", Branch: "main"},
				{Path: "/home/u/scratch", Detached: true},
			},
			want: []string{"main", "scratch"},
		},
		{
			name: "detached basename colliding with main grows to be unique",
			list: []Worktree{
				{Path: "/Volumes/x/elephc", Branch: "main"},
				{Path: "/home/u/.codex/worktrees/0e21/elephc", Detached: true},
			},
			want: []string{"main", "0e21/elephc"},
		},
		{
			name: "two colliding detached worktrees both grow",
			list: []Worktree{
				{Path: "/a/0e21/elephc", Detached: true},
				{Path: "/b/9f33/elephc", Detached: true},
			},
			want: []string{"0e21/elephc", "9f33/elephc"},
		},
		{
			name: "detached basename equal to a branch name is avoided",
			list: []Worktree{
				{Path: "/repo", Branch: "elephc"},
				{Path: "/home/u/0e21/elephc", Detached: true},
			},
			want: []string{"elephc", "0e21/elephc"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Handles(tt.list)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Handles() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/worktree/ -run TestHandles -v`
Expected: FAIL — `undefined: Handles`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/worktree/handle.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/worktree/ -run TestHandles -v`
Expected: PASS (all subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/worktree/handle.go internal/worktree/handle_test.go
git commit -m "feat(worktree): compute unique handles for branchless worktrees"
```

---

## Task 2: handle-aware resolution in `cd`/`delete`

**Files:**
- Modify: `internal/cli/cd.go` (`resolveOneWorktree`, `selectWorktree`)
- Test: `internal/cli/completion_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cli/completion_test.go`:

```go
func TestResolveOneWorktreeMatchesHandleNotMain(t *testing.T) {
	list := []worktree.Worktree{
		{Path: "/Volumes/x/elephc", Branch: "main"},
		{Path: "/home/u/.codex/worktrees/0e21/elephc", Detached: true},
	}

	got, err := resolveOneWorktree(list, "0e21/elephc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Path != "/home/u/.codex/worktrees/0e21/elephc" {
		t.Fatalf("resolved to %q, want the detached worktree", got.Path)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestResolveOneWorktreeMatchesHandleNotMain -v`
Expected: FAIL — the bare basename `elephc` resolves to the main worktree (no handle match yet), so either it returns the wrong worktree or no match.

- [ ] **Step 3: Write minimal implementation**

In `internal/cli/cd.go`, replace the body of `resolveOneWorktree` (currently lines 78-114) with this version that inserts an exact-handle match between the exact branch/path match and the basename match:

```go
func resolveOneWorktree(list []worktree.Worktree, query string) (*worktree.Worktree, error) {
	if query == "" {
		return selectWorktree(list, "Select a worktree")
	}

	handles := worktree.Handles(list)

	for i, w := range list {
		if w.Branch == query || w.Path == query {
			return &list[i], nil
		}
	}
	for i := range list {
		if handles[i] == query {
			return &list[i], nil
		}
	}
	for i, w := range list {
		if filepath.Base(w.Path) == query {
			return &list[i], nil
		}
	}

	qLower := strings.ToLower(query)
	var matches []int
	for i, w := range list {
		if strings.Contains(strings.ToLower(w.Branch), qLower) ||
			strings.Contains(strings.ToLower(w.Path), qLower) {
			matches = append(matches, i)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no worktree matches %q", query)
	}
	if len(matches) == 1 {
		return &list[matches[0]], nil
	}

	subset := make([]worktree.Worktree, len(matches))
	for i, idx := range matches {
		subset[i] = list[idx]
	}
	return selectWorktree(subset, fmt.Sprintf("Multiple worktrees match %q", query))
}
```

Then replace the option-building loop in `selectWorktree` (currently lines 121-134) so labels use the handle:

```go
	handles := worktree.Handles(list)
	options := make([]huh.Option[int], len(list))
	for i, w := range list {
		options[i] = huh.NewOption(fmt.Sprintf("%s → %s", handles[i], displayPath(w.Path)), i)
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestResolveOneWorktreeMatchesHandleNotMain -v`
Expected: PASS.

- [ ] **Step 5: Verify the whole cli package still builds and tests pass**

Run: `go test ./internal/cli/ -v`
Expected: PASS (existing completion tests unaffected).

- [ ] **Step 6: Commit**

```bash
git add internal/cli/cd.go internal/cli/completion_test.go
git commit -m "feat(cli): resolve and label worktrees by handle in cd/delete"
```

---

## Task 3: completion suggests handles

**Files:**
- Modify: `internal/cli/completion.go`
- Test: `internal/cli/completion_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cli/completion_test.go`:

```go
func TestWorktreeCompletionItemsSuggestsHandleForDetached(t *testing.T) {
	root := "/Volumes/x/elephc"
	list := []worktree.Worktree{
		{Path: root, Branch: "main"},
		{Path: "/home/u/.codex/worktrees/0e21/elephc", Detached: true},
	}

	got := worktreeCompletionItems(list, root, "")
	want := []string{"main", "0e21/elephc"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestWorktreeCompletionItemsSuggestsHandleForDetached -v`
Expected: FAIL — current code suggests the bare basename `elephc`, not `0e21/elephc`.

- [ ] **Step 3: Write minimal implementation**

In `internal/cli/completion.go`, thread the precomputed handle through the item builders.

Replace `worktreeCompletionItems` (currently lines 64-70):

```go
func worktreeCompletionItems(list []worktree.Worktree, root, toComplete string) []string {
	handles := worktree.Handles(list)
	var comps []string
	for i, w := range list {
		comps = appendWorktreeCompletionItems(comps, w, handles[i], root, toComplete)
	}
	return comps
}
```

Replace `appendWorktreeCompletionItems` (currently lines 72-90) so branchless worktrees offer the handle and branch worktrees keep the existing branch+custom-basename behavior:

```go
func appendWorktreeCompletionItems(comps []string, w worktree.Worktree, handle, root, toComplete string) []string {
	if w.Branch == "" {
		// Branchless (detached/bare): the only stable, unambiguous token is
		// the computed handle (e.g. "0e21/elephc"). The bare basename may
		// collide with the main worktree, so we never suggest it.
		if w.Path != root && completionMatches(handle, toComplete) {
			comps = append(comps, handle)
		}
		return comps
	}

	if completionMatches(w.Branch, toComplete) {
		comps = append(comps, w.Branch)
	}

	base := filepath.Base(w.Path)
	// Do not suggest the main worktree basename: it is just the project
	// directory name and the user can already refer to it by branch name.
	if base == "" || base == w.Branch || w.Path == root {
		return comps
	}
	if base == worktree.SlugifyBranch(w.Branch) {
		return comps
	}
	if completionMatches(base, toComplete) {
		comps = append(comps, base)
	}
	return comps
}
```

Then update `deleteCompletion` to pass handles. Replace its loop (currently lines 54-61):

```go
	handles := worktree.Handles(list)
	var comps []string
	for i, w := range list {
		if w.Path == root || w.Path == mainWorktreePath {
			continue
		}
		comps = appendWorktreeCompletionItems(comps, w, handles[i], root, toComplete)
	}
	return comps, cobra.ShellCompDirectiveNoFileComp
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestWorktreeCompletionItems -v`
Expected: PASS — including the existing `TestWorktreeCompletionItemsSkipsGeneratedBranchSlug` and `TestWorktreeCompletionItemsKeepsCustomBasename`.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/completion.go internal/cli/completion_test.go
git commit -m "feat(cli): suggest unique handles for branchless worktrees in completion"
```

---

## Task 4: `list` shows handle, tags, and `external`

**Files:**
- Modify: `internal/cli/helpers.go` (add `isExternalPath`)
- Modify: `internal/cli/list.go` (`listEntry`, label, tags, JSON)

> Note: `isExternalPath` depends on `WorktreesBase()` resolved through the
> `sync.Once`-guarded `resolveWorktreesBase`, so it is verified end-to-end in
> Task 5 rather than with an isolated unit test (the cached base would make a
> standalone env-based unit test order-dependent).

- [ ] **Step 1: Add the `isExternalPath` helper**

Append to `internal/cli/helpers.go`:

```go
// isExternalPath reports whether a worktree was created outside ggw: it is
// neither the main worktree nor located under the ggw-managed worktrees base.
// Used to tag such worktrees (e.g. a Codex `--detach` worktree) as [external].
func isExternalPath(p, mainPath string) bool {
	if p == mainPath {
		return false
	}
	resolveWorktreesBase()
	if wtBaseReady {
		for _, b := range [...]string{wtBaseRaw, wtBaseReal} {
			if b == "" {
				continue
			}
			if strings.HasPrefix(p, b+string(os.PathSeparator)) {
				return false
			}
		}
	}
	return true
}
```

- [ ] **Step 2: Add the `External` field to `listEntry`**

In `internal/cli/list.go`, add the field to the `listEntry` struct (after `Bare`, around line 20):

```go
	Bare        bool   `json:"bare,omitempty"`
	External    bool   `json:"external"`
```

- [ ] **Step 3: Populate handle + external, and render tags**

In `internal/cli/list.go`, inside the `RunE` after `raw, err := worktree.List(root)` (around line 48), compute handles and the main path:

```go
		handles := worktree.Handles(raw)
		mainPath := raw[0].Path
```

In the entry-building loop, set `External` when constructing each `listEntry`:

```go
			entries[i] = listEntry{
				Path:     w.Path,
				Head:     w.Head,
				Branch:   w.Branch,
				Detached: w.Detached,
				Locked:   w.Locked,
				Bare:     w.Bare,
				External: isExternalPath(w.Path, mainPath),
			}
```

Replace the label computation (currently lines 83-90) so it uses the handle:

```go
		labels := make([]string, len(entries))
		maxLabel := 0
		for i := range entries {
			labels[i] = labelFor(entries[i], handles[i])
			if l := len(labels[i]); l > maxLabel {
				maxLabel = l
			}
		}
```

Replace the print loop (currently lines 94-109) so it builds a combined tag string:

```go
		for i, e := range entries {
			pad := strings.Repeat(" ", maxLabel-len(labels[i]))
			suffix := statusSuffix(e)
			tags := ""
			if e.Branch == "" {
				kind := "(detached)"
				if e.Bare {
					kind = "(bare)"
				}
				tags += " " + ui.Muted.Render(kind)
			}
			if e.External {
				tags += " " + ui.Muted.Render("[external]")
			}
			if e.Locked {
				tags += " " + ui.Muted.Render("[locked]")
			}
			fmt.Printf("  %s %s%s → %s%s%s\n",
				ui.Success.Render("●"),
				ui.Branch.Render(labels[i]),
				pad,
				ui.Path.Render(renderPath(e.Path, fullPath)),
				suffix,
				tags,
			)
		}
```

Replace `labelFor` (currently lines 127-138) so branchless entries use the handle:

```go
func labelFor(e listEntry, handle string) string {
	if e.Branch != "" {
		return e.Branch
	}
	return handle
}
```

- [ ] **Step 4: Build and run the cli tests**

Run: `go build ./... && go test ./internal/cli/ -v`
Expected: PASS — and the build confirms `labelFor`'s new signature is consistent and `os`/`strings` are already imported in `helpers.go`/`list.go`.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/helpers.go internal/cli/list.go
git commit -m "feat(cli): show handle, detached/external/locked tags in list"
```

---

## Task 5: end-to-end coverage for an external detached worktree

**Files:**
- Test: `tests/e2e_test.go`

- [ ] **Step 1: Write the failing test**

Append to `tests/e2e_test.go` a test that creates a detached worktree outside the ggw base, with a leaf name (`api`) equal to the repo basename to force handle disambiguation to `0e21/api`:

```go
func TestCLIExternalDetachedWorktree(t *testing.T) {
	home := setupHome(t)
	repo := initGitRepo(t, home)

	// A detached worktree created outside ggw's base (e.g. by another tool),
	// whose leaf "api" collides with the repo basename so the handle must
	// grow to "0e21/api".
	extPath := filepath.Join(home, "external", "0e21", "api")
	if err := os.MkdirAll(filepath.Dir(extPath), 0755); err != nil {
		t.Fatalf("create external parent dir: %v", err)
	}
	runGit(t, home, repo, "worktree", "add", "--detach", extPath)
	canonicalExt := canonicalPath(t, extPath)

	// list: shows the handle and the [external] tag.
	out, err := runGGW(t, home, repo, "list")
	if err != nil {
		t.Fatalf("ggw list failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "0e21/api") || !strings.Contains(out, "[external]") {
		t.Fatalf("list output missing handle or external tag:\n%s", out)
	}

	// list --json: external flag is set for the detached worktree.
	out, err = runGGW(t, home, repo, "--json", "list")
	if err != nil {
		t.Fatalf("ggw --json list failed: %v\n%s", err, out)
	}
	var listPayload struct {
		Worktrees []struct {
			Path     string `json:"path"`
			External bool   `json:"external"`
		} `json:"worktrees"`
	}
	if err := json.Unmarshal([]byte(out), &listPayload); err != nil {
		t.Fatalf("list JSON is invalid: %v\n%s", err, out)
	}
	foundExternal := false
	for _, w := range listPayload.Worktrees {
		if w.Path == canonicalExt {
			foundExternal = w.External
		}
	}
	if !foundExternal {
		t.Fatalf("external worktree not flagged in JSON: %+v", listPayload.Worktrees)
	}

	// completion: offers the handle, never the ambiguous bare basename.
	out, err = runGGW(t, home, repo, "__complete", "delete", "")
	if err != nil {
		t.Fatalf("ggw delete completion failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "0e21/api") {
		t.Fatalf("completion missing handle:\n%s", out)
	}

	// cd: resolves the handle to the external worktree path.
	out, err = runGGW(t, home, repo, "cd", "0e21/api")
	if err != nil {
		t.Fatalf("ggw cd failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) != canonicalExt {
		t.Fatalf("cd output = %q, want %q", strings.TrimSpace(out), canonicalExt)
	}

	// delete: removes the external worktree by handle.
	out, err = runGGW(t, home, repo, "delete", "--force", "0e21/api")
	if err != nil {
		t.Fatalf("ggw delete failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(extPath); !os.IsNotExist(err) {
		t.Fatalf("external worktree still exists after delete, stat err: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails (if run before Tasks 1-4) or passes**

Run: `go test ./tests/ -run TestCLIExternalDetachedWorktree -v`
Expected: PASS once Tasks 1-4 are implemented. (If a step regressed, this is where it surfaces — e.g. handle not shown, or `cd`/`delete` not resolving the handle.)

- [ ] **Step 3: Run the full suite**

Run: `go test ./...`
Expected: PASS for all packages.

- [ ] **Step 4: Commit**

```bash
git add tests/e2e_test.go
git commit -m "test(e2e): cover external detached worktree in list/cd/delete"
```

---

## Self-Review Notes

- **Spec coverage:** handle concept (Task 1); `resolveOneWorktree` priority branch→handle→basename→substring (Task 2); completion handles for `cd` and `delete` (Task 3); `selectWorktree` handle labels (Task 2); `list` handle label + `(detached)` + `[external]` + JSON `external` (Task 4); external detection under/outside `WorktreesBase` with main excluded (Task 4 + verified Task 5); delete keeps the standard confirmation, `--force` skips it (unchanged — no edit to `delete.go`). All spec sections map to a task.
- **Type consistency:** `worktree.Handles(list []Worktree) []string` is used identically in `cd.go`, `completion.go`, and `list.go`. `labelFor` is updated to its two-argument form at its sole call site. `appendWorktreeCompletionItems` gains a `handle` parameter, updated at both call sites (`worktreeCompletionItems` and `deleteCompletion`).
- **Out of scope (unchanged):** no numeric indices, no special confirmation/warning for external worktrees, no other-tool lock handling, no unrelated refactors.
