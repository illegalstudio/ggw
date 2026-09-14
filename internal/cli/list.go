package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/illegalstudio/ggw/internal/ui"
	"github.com/illegalstudio/ggw/internal/workspace"

	"github.com/spf13/cobra"
)

type listEntry struct {
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	Head        string `json:"head,omitempty"`
	Branch      string `json:"branch,omitempty"`
	Main        bool   `json:"main,omitempty"`
	Detached    bool   `json:"detached,omitempty"`
	Locked      bool   `json:"locked,omitempty"`
	Bare        bool   `json:"bare,omitempty"`
	Dirty       bool   `json:"dirty"`
	Ahead       int    `json:"ahead"`
	Behind      int    `json:"behind"`
	HasUpstream bool   `json:"has_upstream"`
	StatusError string `json:"status_error,omitempty"`
	External    bool   `json:"external"`
}

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List workspaces of the current repository, with git status",
	GroupID: GroupWorktree,
	Args:    cobra.NoArgs,
	Long: `List every workspace of the current repository: its git worktrees and its
copy-on-write snapshots, which are tagged [cow].`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fullPath, _ := cmd.Flags().GetBool("full-path")

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		ctx, err := workspace.Resolve(cwd)
		if err != nil {
			return err
		}

		raw, err := workspace.List(ctx)
		if err != nil {
			return err
		}

		handles := workspace.Handles(raw)
		mainPath := ctx.MainPath

		entries := make([]listEntry, len(raw))
		for i, w := range raw {
			entries[i] = listEntry{
				Kind:     string(w.Kind),
				Path:     w.Path,
				Head:     w.Head,
				Branch:   w.Branch,
				Main:     w.Main,
				Detached: w.Detached,
				Locked:   w.Locked,
				Bare:     w.Bare,
				External: isExternalPath(w.Path, mainPath),
			}
			if w.Bare {
				continue
			}
			st, err := workspace.Status(w)
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
			fmt.Println(ui.Info.Render("No workspaces registered."))
			return nil
		}

		// The label is the handle, never the raw branch: a branch shared by two
		// workspaces names neither, and printing it would offer a name that
		// `ggw cd` and `ggw delete` then refuse to resolve.
		maxLabel := 0
		for _, h := range handles {
			if l := len(h); l > maxLabel {
				maxLabel = l
			}
		}

		fmt.Println(ui.Title.Render("Workspaces"))
		fmt.Println()
		for i, e := range entries {
			pad := strings.Repeat(" ", maxLabel-len(handles[i]))
			suffix := statusSuffix(e)
			tags := ""
			if e.Branch == "" {
				kind := "(detached)"
				if e.Bare {
					kind = "(bare)"
				}
				tags += " " + ui.Muted.Render(kind)
			}
			if e.Kind == string(workspace.KindCoW) {
				tags += " " + ui.Muted.Render("[cow]")
			}
			if e.External {
				tags += " " + ui.Muted.Render("[external]")
			}
			if e.Locked {
				tags += " " + ui.Muted.Render("[locked]")
			}
			fmt.Printf("  %s %s%s → %s%s%s\n",
				ui.Success.Render("●"),
				ui.Branch.Render(handles[i]),
				pad,
				ui.Path.Render(renderPath(e.Path, fullPath)),
				suffix,
				tags,
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
