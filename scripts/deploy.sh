#!/usr/bin/env bash
# Create and push a semantic version tag for release.
#
# Usage: scripts/deploy.sh
#   or:  make deploy
#
# Reads the latest v* tag, proposes the next patch version, prompts for
# confirmation (or a custom version), then creates an annotated tag and
# pushes it to origin (which triggers the release workflow).

set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "error: not inside a git repository" >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "error: working tree is not clean; commit or stash changes first" >&2
  git status --short
  exit 1
fi

# Latest semver tag (vMAJOR.MINOR.PATCH). Empty if none.
latest="$(git tag -l 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n1 || true)"

if [[ -z "$latest" ]]; then
  proposed="v0.1.0"
  echo "No existing version tags found."
else
  ver="${latest#v}"
  IFS=. read -r major minor patch <<<"$ver"
  if [[ -z "${major:-}" || -z "${minor:-}" || -z "${patch:-}" ]]; then
    echo "error: cannot parse latest tag $latest as vMAJOR.MINOR.PATCH" >&2
    exit 1
  fi
  proposed="v${major}.${minor}.$((patch + 1))"
  echo "Latest tag: $latest"
fi

echo "Proposed next tag: $proposed"
echo
printf "Version to release [%s]: " "$proposed"
read -r input

version="${input:-$proposed}"

# Normalize: accept 0.2.1 or v0.2.1
if [[ "$version" != v* ]]; then
  version="v${version}"
fi

if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "error: version must look like v1.2.3 (got $version)" >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/${version}" >/dev/null; then
  echo "error: tag $version already exists locally" >&2
  exit 1
fi

# Refuse if remote already has the tag (best-effort; needs fetch).
if git ls-remote --exit-code --tags origin "refs/tags/${version}" >/dev/null 2>&1; then
  echo "error: tag $version already exists on origin" >&2
  exit 1
fi

branch="$(git rev-parse --abbrev-ref HEAD)"
commit="$(git rev-parse --short HEAD)"

echo
echo "Will create annotated tag:"
echo "  tag:    $version"
echo "  branch: $branch"
echo "  commit: $commit"
echo "  remote: origin"
echo
printf "Proceed? [y/N] "
read -r confirm
case "$confirm" in
  y|Y|yes|YES) ;;
  *)
    echo "Aborted."
    exit 1
    ;;
esac

git tag -a "$version" -m "Release $version"
git push origin "$version"

echo
echo "✓ Tagged and pushed $version"
echo "  Release workflow should start on GitHub shortly."
