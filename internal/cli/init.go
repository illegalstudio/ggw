package cli

import (
	"fmt"
	"os"

	"github.com/illegalstudio/ggw/internal/config"
	"github.com/illegalstudio/ggw/internal/ui"
	"github.com/illegalstudio/ggw/internal/worktree"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:     "init",
	Short:   "Create the global config file seeded with this system's default worktrees directory",
	GroupID: GroupConfig,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.ConfigPath()
		if err != nil {
			return err
		}
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config file already exists at %s", path)
		}

		// No config exists yet, so WorktreesBase returns this system's default.
		base, err := worktree.WorktreesBase()
		if err != nil {
			return err
		}
		seed := displayPath(base) // collapse $HOME -> ~ for a portable, readable value

		if err := config.WriteDefault(path, seed); err != nil {
			return err
		}

		if done, err := maybeJSON(map[string]any{"created": true, "path": path}); done {
			return err
		}

		fmt.Println(ui.Success.Render("✓") + " Config file created at " + ui.Path.Render(path))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
