package project

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProvisionOptions configures a single Provision run.
type ProvisionOptions struct {
	MainPath string    // absolute path of the main worktree (source root)
	DestPath string    // absolute path of the new worktree (destination root)
	Config   *Config   // parsed .ggw.yaml
	Out      io.Writer // where post_create command output is streamed
}

// Provision applies the config to the new worktree: copy, then symlink, then
// post_create commands. It returns the first error encountered; the caller is
// responsible for rolling back the worktree.
func Provision(opts ProvisionOptions) error {
	for _, rel := range opts.Config.Copy {
		if err := copyEntry(opts.MainPath, opts.DestPath, rel); err != nil {
			return err
		}
	}
	for _, rel := range opts.Config.Symlink {
		if err := symlinkEntry(opts.MainPath, opts.DestPath, rel); err != nil {
			return err
		}
	}
	for _, command := range opts.Config.PostCreate {
		if err := runCommand(opts.DestPath, command, opts.Out); err != nil {
			return err
		}
	}
	return nil
}

func copyEntry(mainPath, destPath, rel string) error {
	src, dst, err := resolveEntry(mainPath, destPath, rel)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("copy %q: %w", rel, err)
	}
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("copy %q: destination already exists: %s", rel, dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("copy %q: %w", rel, err)
	}
	if info.IsDir() {
		return copyTree(src, dst)
	}
	return copyFile(src, dst, info.Mode())
}

func symlinkEntry(mainPath, destPath, rel string) error {
	src, dst, err := resolveEntry(mainPath, destPath, rel)
	if err != nil {
		return err
	}
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("symlink %q: %w", rel, err)
	}
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("symlink %q: destination already exists: %s", rel, dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("symlink %q: %w", rel, err)
	}
	if err := os.Symlink(src, dst); err != nil {
		return fmt.Errorf("symlink %q: %w", rel, err)
	}
	return nil
}

func runCommand(destPath, command string, out io.Writer) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = destPath
	cmd.Env = os.Environ()
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %q failed: %w", command, err)
	}
	return nil
}

// resolveEntry validates a relative entry and returns its absolute source
// (under mainPath) and destination (under destPath).
func resolveEntry(mainPath, destPath, rel string) (string, string, error) {
	if rel == "" {
		return "", "", fmt.Errorf("empty path in .ggw.yaml")
	}
	if filepath.IsAbs(rel) {
		return "", "", fmt.Errorf("path %q must be relative", rel)
	}
	clean := filepath.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("path %q escapes the repository", rel)
	}
	return filepath.Join(mainPath, clean), filepath.Join(destPath, clean), nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode())
	})
}
