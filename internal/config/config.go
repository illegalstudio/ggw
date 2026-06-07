// Package config reads ggw's user configuration from ~/.config/ggw/config.yaml.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config is the ggw configuration.
type Config struct {
	// BaseDir is the directory under which all worktrees live, nested as
	// <BaseDir>/<org>/<repo>/<branch-slug>. Empty means "use the default".
	BaseDir string `mapstructure:"base_dir"`
}

// ConfigPath returns the path to the ggw config file: ~/.config/ggw/config.yaml.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "ggw", "config.yaml"), nil
}

// Load reads and parses the config file via viper, expanding a leading ~ in
// BaseDir. A missing file yields an error for which errors.Is(err, fs.ErrNotExist)
// is true, so callers can treat "no config" as "use defaults".
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("config file %s: %w", path, err)
		}
		return nil, fmt.Errorf("cannot stat config file: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config file: %w", err)
	}

	expanded, err := expandTilde(cfg.BaseDir)
	if err != nil {
		return nil, err
	}
	cfg.BaseDir = expanded
	return &cfg, nil
}

// BaseDir returns the configured worktrees base directory (~ expanded) and
// whether it is set. A missing config file or an empty base_dir yields
// ("", false, nil); a malformed config yields ("", false, err).
func BaseDir() (string, bool, error) {
	cfg, err := Load()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	if cfg.BaseDir == "" {
		return "", false, nil
	}
	return cfg.BaseDir, true, nil
}

// WriteDefault writes a commented config template to path, seeding base_dir with
// seedBaseDir. It creates the parent directory (~/.config/ggw) as needed. It does
// not check for an existing file — callers must guard against overwriting.
func WriteDefault(path, seedBaseDir string) error {
	content := fmt.Sprintf(`# GGW configuration
#
# base_dir: directory under which all worktrees live, nested as
# <base_dir>/<org>/<repo>/<branch-slug>. A leading ~ is expanded to $HOME.
base_dir: %s
`, seedBaseDir)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// expandTilde expands a leading ~ or ~/ in p to the user's home directory.
// All other paths (absolute or relative) are returned unchanged.
func expandTilde(p string) (string, error) {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot expand ~ in path %q: %w", p, err)
	}
	if p == "~" {
		return home, nil
	}
	return filepath.Join(home, p[2:]), nil
}
