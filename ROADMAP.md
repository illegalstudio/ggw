# ggw roadmap

## Iteration 1: Scaffold + basic commands

- [x] Set up `go.mod` and folder structure
- [x] Cobra root command + stub subcommands
- [x] Worktree path resolution logic (`org/repo/slug`)
- [x] Branch name slugification
- [x] `ggw list` (+ `--json`)
- [x] `ggw create <name>` (auto-create branch if it doesn't exist)
- [x] Minimal README
- [x] Smoke test
- [x] Unit tests for slugification and path resolution

## Iteration 2: Navigation

- [x] `ggw shell-init zsh|bash|fish`
- [x] `ggw cd [name]` (prints path; shell wrapper intercepts and chdirs)
- [x] `huh` selector when `name` is omitted or ambiguous
- [x] Shell function tested in zsh/bash

## Iteration 3: Operations

- [x] `ggw exec [name] -- <cmd>` (propagates exit code, refuses `--json`)
- [x] `ggw delete [name]` with `--without-branch` and `--force` (`huh` confirm; `--force` skips confirmation)

## Backlog / future

- [x] Config file for path override (`ggw init`, `~/.config/ggw/config.yaml`)
- [x] Post-create hooks / project provisioning (`.ggw.yaml`: copy, symlink, post_create)
- [x] Tab completion zsh/bash/fish
- [x] Goreleaser
- [x] Homebrew tap
- [ ] Configurable fallback when `origin` remote is missing (today: explicit error)
