package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// bashZshFunction is the shell wrapper for bash/zsh. It intercepts `ggw cd`
// and turns it into a real `cd`; everything else falls through to the binary.
const bashZshFunction = `# ggw shell integration: turn 'ggw cd' into a real chdir.
ggw() {
  if [ "$1" = "cd" ]; then
    shift
    local _ggw_dir
    if ! _ggw_dir=$(command ggw cd "$@"); then
      return 1
    fi
    builtin cd "$_ggw_dir" || return $?
  else
    command ggw "$@"
  fi
}
`

// fishFunction is the equivalent for fish.
const fishFunction = `# ggw shell integration: turn 'ggw cd' into a real chdir.
function ggw
  if test "$argv[1]" = "cd"
    set -e argv[1]
    set -l _ggw_dir (command ggw cd $argv)
    if test $status -ne 0
      return 1
    end
    builtin cd $_ggw_dir
  else
    command ggw $argv
  end
end
`

func generateCompletionScript(shell string) (string, error) {
	var buf strings.Builder
	switch shell {
	case "bash":
		if err := rootCmd.GenBashCompletion(&buf); err != nil {
			return "", err
		}
	case "zsh":
		if err := rootCmd.GenZshCompletion(&buf); err != nil {
			return "", err
		}
	case "fish":
		if err := rootCmd.GenFishCompletion(&buf, true); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported shell: %s", shell)
	}
	return buf.String(), nil
}

var initCmd = &cobra.Command{
	Use:     "init <bash|zsh|fish>",
	Short:   "Print shell integration script (eval to enable `ggw cd` and completions)",
	GroupID: GroupShell,
	Long: `Print a shell function that makes "ggw cd" actually change directory,
plus tab-completion for ggw commands and worktree names.

Add to your shell config:
  bash:  eval "$(ggw init bash)"   (in ~/.bashrc)
  zsh:   eval "$(ggw init zsh)"    (in ~/.zshrc)
  fish:  ggw init fish | source    (in ~/.config/fish/config.fish)

Then:
  ggw cd <name>      # cd into a worktree (interactive selector if name omitted)
  ggw de<TAB>        # complete commands
  ggw cd fea<TAB>    # complete worktree names`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		shell := "zsh"
		if len(args) > 0 {
			shell = args[0]
		}

		var script strings.Builder
		switch shell {
		case "bash", "zsh":
			script.WriteString(bashZshFunction)
		case "fish":
			script.WriteString(fishFunction)
		default:
			return fmt.Errorf("unsupported shell: %s (use bash, zsh, or fish)", shell)
		}

		comp, err := generateCompletionScript(shell)
		if err != nil {
			return err
		}
		script.WriteString("\n")
		script.WriteString(comp)

		if done, err := maybeJSON(map[string]any{"shell": shell, "script": script.String()}); done {
			return err
		}

		fmt.Print(script.String())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
