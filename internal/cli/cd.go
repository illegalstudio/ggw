package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/illegalstudio/ggw/internal/worktree"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var cdCmd = &cobra.Command{
	Use:               "cd [name]",
	Short:             "Print the worktree path (chdir requires shell integration — see `ggw shell-init`)",
	GroupID:           GroupWorktree,
	ValidArgsFunction: worktreeCompletion,
	Long: `Print the absolute path of a worktree on stdout.

Without shell integration, this just prints — the binary cannot change the
parent shell's directory. To get an actual chdir, run "ggw shell-init <shell>" once
(see its --help) and reload your shell. Then:

  ggw cd feature/login   # cd into the matching worktree
  ggw cd                 # interactive selector

Matching: exact branch → exact directory slug → substring on branch or path.
Multiple matches drop into an interactive selector.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		root, err := worktree.RepoRoot(cwd)
		if err != nil {
			return err
		}

		list, err := worktree.List(root)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			return fmt.Errorf("no worktrees registered for this repository")
		}

		query := ""
		if len(args) > 0 {
			query = args[0]
		}

		wt, err := resolveOneWorktree(list, query)
		if err != nil {
			return err
		}

		if done, err := maybeJSON(map[string]any{"path": wt.Path, "branch": wt.Branch}); done {
			return err
		}

		// No newline: shell wrapper captures with $(...) which trims trailing newlines anyway,
		// but explicit `Print` (no newline) keeps copy-paste of stdout clean too.
		fmt.Print(wt.Path)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cdCmd)
}

// resolveOneWorktree picks a worktree from list. Empty query opens an
// interactive selector. Otherwise: exact branch → exact slug → substring;
// multiple substring matches open a selector.
func resolveOneWorktree(list []worktree.Worktree, query string) (*worktree.Worktree, error) {
	if query == "" {
		return selectWorktree(list, "Select a worktree")
	}

	for i, w := range list {
		if w.Branch == query || w.Path == query {
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

func selectWorktree(list []worktree.Worktree, title string) (*worktree.Worktree, error) {
	if jsonOutput {
		return nil, fmt.Errorf("%s: refusing interactive prompt in --json mode (be more specific)", title)
	}

	options := make([]huh.Option[int], len(list))
	for i, w := range list {
		label := w.Branch
		if label == "" {
			if w.Detached {
				label = "(detached)"
			} else if w.Bare {
				label = "(bare)"
			} else {
				label = filepath.Base(w.Path)
			}
		}
		options[i] = huh.NewOption(fmt.Sprintf("%s → %s", label, displayPath(w.Path)), i)
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
