package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/illegalstudio/ggw/internal/workspace"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var cdCmd = &cobra.Command{
	Use:               "cd [name]",
	Short:             "Print the workspace path (chdir requires shell integration — see `ggw shell-init`)",
	GroupID:           GroupWorktree,
	ValidArgsFunction: workspaceCompletion,
	Long: `Print the absolute path of a workspace on stdout.

Without shell integration, this just prints — the binary cannot change the
parent shell's directory. To get an actual chdir, run "ggw shell-init <shell>" once
(see its --help) and reload your shell. Then:

  ggw cd feature/login   # cd into the matching workspace
  ggw cd                 # interactive selector

Matching: exact branch → exact path → handle → basename → substring on branch or path.
Multiple matches drop into an interactive selector.

Detached or external worktrees can be addressed by their handle, e.g. "ggw cd 0e21/elephc".
So can two copy-on-write workspaces sharing a branch: each gets a handle from its
directory name.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		ctx, err := workspace.Resolve(cwd)
		if err != nil {
			return err
		}

		list, err := workspace.List(ctx)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			return fmt.Errorf("no workspaces registered for this repository")
		}

		query := ""
		if len(args) > 0 {
			query = args[0]
		}

		ws, err := resolveOneWorkspace(list, query)
		if err != nil {
			return err
		}

		if done, err := maybeJSON(map[string]any{
			"path":   ws.Path,
			"branch": ws.Branch,
			"kind":   string(ws.Kind),
		}); done {
			return err
		}

		// No newline: shell wrapper captures with $(...) which trims trailing newlines anyway,
		// but explicit `Print` (no newline) keeps copy-paste of stdout clean too.
		fmt.Print(ws.Path)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cdCmd)
}

// resolveOneWorkspace picks a workspace from list. Empty query opens an
// interactive selector. Otherwise: exact path/branch → handle → basename →
// substring; anything that matches more than one opens a selector.
func resolveOneWorkspace(list []workspace.Workspace, query string) (*workspace.Workspace, error) {
	if query == "" {
		return selectWorkspace(list, "Select a workspace")
	}

	for i, w := range list {
		if w.Path == query {
			return &list[i], nil
		}
	}

	// A branch no longer identifies one workspace on its own: copy-on-write
	// workspaces created with --as can share it, so an ambiguous branch name
	// goes to the selector rather than silently picking the first match.
	if matches := indexesMatching(list, func(w workspace.Workspace) bool { return w.Branch == query }); len(matches) > 0 {
		return pickOne(list, matches, fmt.Sprintf("Multiple workspaces are on %q", query))
	}

	handles := workspace.Handles(list)
	for i := range list {
		if handles[i] == query {
			return &list[i], nil
		}
	}

	// Basenames deliberately resolve first-match-wins rather than prompting:
	// git worktree list reports the main worktree first, so a basename it
	// shares with a detached workspace picks the main one, and the detached one
	// stays reachable through its handle.
	for i, w := range list {
		if filepath.Base(w.Path) == query {
			return &list[i], nil
		}
	}

	qLower := strings.ToLower(query)
	matches := indexesMatching(list, func(w workspace.Workspace) bool {
		return strings.Contains(strings.ToLower(w.Branch), qLower) ||
			strings.Contains(strings.ToLower(w.Path), qLower)
	})
	if len(matches) == 0 {
		return nil, fmt.Errorf("no workspace matches %q", query)
	}
	return pickOne(list, matches, fmt.Sprintf("Multiple workspaces match %q", query))
}

func indexesMatching(list []workspace.Workspace, pred func(workspace.Workspace) bool) []int {
	var out []int
	for i, w := range list {
		if pred(w) {
			out = append(out, i)
		}
	}
	return out
}

// pickOne returns the single match, or opens a selector over the matches.
func pickOne(list []workspace.Workspace, matches []int, title string) (*workspace.Workspace, error) {
	if len(matches) == 1 {
		return &list[matches[0]], nil
	}
	// selectWorkspace recomputes handles within this subset; they stay
	// unambiguous among the shown choices.
	subset := make([]workspace.Workspace, len(matches))
	for i, idx := range matches {
		subset[i] = list[idx]
	}
	return selectWorkspace(subset, title)
}

func selectWorkspace(list []workspace.Workspace, title string) (*workspace.Workspace, error) {
	if jsonOutput {
		return nil, fmt.Errorf("%s: refusing interactive prompt in --json mode (be more specific)", title)
	}

	handles := workspace.Handles(list)
	options := make([]huh.Option[int], len(list))
	for i, w := range list {
		label := fmt.Sprintf("%s → %s", handles[i], displayPath(w.Path))
		if w.Kind == workspace.KindCoW {
			label += " [cow]"
		}
		options[i] = huh.NewOption(label, i)
	}

	var idx int
	err := huh.NewSelect[int]().
		Title(title).
		Description("Start typing to filter").
		Options(options...).
		Filtering(true).
		Height(20).
		Value(&idx).
		Run()
	if err != nil {
		return nil, err
	}
	return &list[idx], nil
}
