package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/illegalstudio/ggw/internal/project"
	"github.com/illegalstudio/ggw/internal/ui"
	"github.com/illegalstudio/ggw/internal/workspace"

	"github.com/spf13/cobra"
)

var projectInitCmd = &cobra.Command{
	Use:     "project-init",
	Short:   "Create a .ggw.yaml provisioning file at the repository root",
	GroupID: GroupConfig,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		ctx, err := workspace.Resolve(cwd)
		if err != nil {
			return err
		}
		mainPath, err := ctx.SourceRepo()
		if err != nil {
			return err
		}
		path := filepath.Join(mainPath, project.ConfigFileName)

		if err := project.WriteTemplate(path, force); err != nil {
			return err
		}

		if done, err := maybeJSON(map[string]any{"created": true, "path": path}); done {
			return err
		}

		fmt.Println(ui.Success.Render("✓") + " Project config created at " + ui.Path.Render(displayPath(path)))
		return nil
	},
}

func init() {
	projectInitCmd.Flags().Bool("force", false, "Overwrite an existing .ggw.yaml")
	rootCmd.AddCommand(projectInitCmd)
}
