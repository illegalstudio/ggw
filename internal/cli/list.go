package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/illegalstudio/ggw/internal/ui"
	"github.com/illegalstudio/ggw/internal/worktree"

	"github.com/spf13/cobra"
)

type listEntry struct {
	Path        string `json:"path"`
	Head        string `json:"head,omitempty"`
	Branch      string `json:"branch,omitempty"`
	Detached    bool   `json:"detached,omitempty"`
	Locked      bool   `json:"locked,omitempty"`
	Bare        bool   `json:"bare,omitempty"`
	Dirty       bool   `json:"dirty"`
	Ahead       int    `json:"ahead"`
	Behind      int    `json:"behind"`
	HasUpstream bool   `json:"has_upstream"`
	StatusError string `json:"status_error,omitempty"`
}

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List worktrees of the current repository, with git status",
	GroupID: GroupWorktree,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fullPath, _ := cmd.Flags().GetBool("full-path")

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		root, err := worktree.RepoRoot(cwd)
		if err != nil {
			return err
		}

		raw, err := worktree.List(root)
		if err != nil {
			return err
		}

		entries := make([]listEntry, len(raw))
		for i, w := range raw {
			entries[i] = listEntry{
				Path:     w.Path,
				Head:     w.Head,
				Branch:   w.Branch,
				Detached: w.Detached,
				Locked:   w.Locked,
				Bare:     w.Bare,
			}
			if w.Bare {
				continue
			}
			st, err := worktree.GetStatus(w.Path)
			if err != nil {
				entries[i].StatusError = err.Error()
				continue
			}
			entries[i].Dirty = st.Dirty
			entries[i].Ahead = st.Ahead
			entries[i].Behind = st.Behind
			entries[i].HasUpstream = st.HasUpstream
		}

		if done, err := maybeJSON(map[string]any{"worktrees": entries}); done {
			return err
		}

		if len(entries) == 0 {
			fmt.Println(ui.Info.Render("No worktrees registered."))
			return nil
		}

		labels := make([]string, len(entries))
		maxLabel := 0
		for i, e := range entries {
			labels[i] = labelFor(e)
			if l := len(labels[i]); l > maxLabel {
				maxLabel = l
			}
		}

		fmt.Println(ui.Title.Render("Worktrees"))
		fmt.Println()
		for i, e := range entries {
			pad := strings.Repeat(" ", maxLabel-len(labels[i]))
			suffix := statusSuffix(e)
			lockTag := ""
			if e.Locked {
				lockTag = " " + ui.Muted.Render("[locked]")
			}
			fmt.Printf("  %s %s%s → %s%s%s\n",
				ui.Success.Render("●"),
				ui.Branch.Render(labels[i]),
				pad,
				ui.Path.Render(renderPath(e.Path, fullPath)),
				suffix,
				lockTag,
			)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	listCmd.Flags().Bool("full-path", false, "Show the full (tildified) path instead of the [...] short form")
	rootCmd.AddCommand(listCmd)
}

func renderPath(p string, full bool) string {
	if full {
		return displayPath(p)
	}
	return compactPath(p)
}

func labelFor(e listEntry) string {
	if e.Branch != "" {
		return e.Branch
	}
	if e.Detached {
		return "(detached)"
	}
	if e.Bare {
		return "(bare)"
	}
	return "(unknown)"
}

func statusSuffix(e listEntry) string {
	if e.Bare {
		return ""
	}
	if e.StatusError != "" {
		return "   " + ui.Error.Render("status error: "+e.StatusError)
	}
	var parts []string
	if e.Dirty {
		parts = append(parts, ui.Error.Render("dirty"))
	}
	if e.HasUpstream {
		if e.Ahead > 0 {
			parts = append(parts, ui.Info.Render(fmt.Sprintf("↑%d", e.Ahead)))
		}
		if e.Behind > 0 {
			parts = append(parts, ui.Info.Render(fmt.Sprintf("↓%d", e.Behind)))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "   " + strings.Join(parts, " ")
}
