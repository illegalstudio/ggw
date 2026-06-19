// Package project reads and applies a repository's .ggw.yaml provisioning file.
package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// ConfigFileName is the per-project provisioning file, read from the main
// worktree root.
const ConfigFileName = ".ggw.yaml"

// Config describes how to provision a freshly created worktree.
type Config struct {
	Copy       []string `mapstructure:"copy"`
	Symlink    []string `mapstructure:"symlink"`
	PostCreate []string `mapstructure:"post_create"`
}

// Load reads .ggw.yaml from mainWorktreePath. The bool reports whether the file
// exists; a missing file yields (nil, false, nil) so callers treat it as "no
// provisioning configured".
func Load(mainWorktreePath string) (*Config, bool, error) {
	path := filepath.Join(mainWorktreePath, ConfigFileName)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("cannot stat %s: %w", path, err)
	}

	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, true, fmt.Errorf("cannot read %s: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, true, fmt.Errorf("cannot parse %s: %w", path, err)
	}
	return &cfg, true, nil
}

// WriteTemplate writes a commented .ggw.yaml template to path. It errors if the
// file already exists unless force is true.
func WriteTemplate(path string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", path)
		}
	}
	return os.WriteFile(path, []byte(template), 0o644)
}

const template = `# .ggw.yaml — per-project worktree provisioning.
#
# Paths are relative to the repository root. ` + "`copy`" + ` and ` + "`symlink`" + ` sources are
# taken from the main worktree; the destination is the same relative path in
# the newly created worktree.

# Files/directories copied (recursively) into each new worktree.
copy:
  # - .env

# Files/directories symlinked into each new worktree. The symlink points at the
# absolute path in the main worktree.
symlink:
  # - node_modules
  # - vendor

# Shell commands run (via ` + "`sh -c`" + `) inside each new worktree, in order, after
# copy and symlink. Execution stops at the first non-zero exit code.
post_create:
  # - composer install
  # - npm ci
`
