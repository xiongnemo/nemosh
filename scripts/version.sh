#!/usr/bin/env bash
# Derive the build version from git and print the -ldflags for it.
#
# The rules are AGENTS.md's, in the Versioning section. This script is the one
# implementation of them: CI and a local release build must not derive the
# version two different ways, or a package manager sees a version no commit
# produced.
#
#   ./scripts/version.sh            prints the full `nemosh <version> (...)` line
#   ./scripts/version.sh --version  prints just the version string
#   ./scripts/version.sh --ldflags  prints the -X flags for `go build`
#
# Requires a full clone. A shallow one has no tags and silently yields the
# v0.0.1 fallback, which is why CI checks out with fetch-depth: 0.

set -euo pipefail

package="github.com/xiongnemo/nemosh/internal/version"

# --exclude='*-*' drops prerelease and dev tags such as v0.1.2-dev-abcdef123456.
# They must never become the base, or every dev build would rebase onto the
# previous dev build instead of onto the last real release.
tag=$(git describe --tags --abbrev=0 --match='v[0-9]*' --exclude='*-*' HEAD 2>/dev/null || true)

if [ -n "$tag" ]; then
  commits=$(git rev-list --count "${tag}..HEAD")
else
  commits=$(git rev-list --count HEAD 2>/dev/null || echo 0)
fi

# GitHub detaches HEAD, so rev-parse would answer "HEAD". Prefer what the
# workflow knows: head_ref on a pull request, ref_name on a push.
#
# On a tag push GITHUB_REF_NAME is the tag, which would render `v0.1.0` as
# `v0.1.0-v0.1.0-<sha>`. A tag is not a branch, so that source is skipped and
# the build is labelled `release` instead.
branch="${GITHUB_HEAD_REF:-}"
if [ -z "$branch" ] && [ "${GITHUB_REF_TYPE:-}" != "tag" ]; then
  branch="${GITHUB_REF_NAME:-}"
fi
if [ -z "$branch" ]; then
  branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
fi
if [ -z "$branch" ] || [ "$branch" = "HEAD" ]; then
  if [ "${GITHUB_REF_TYPE:-}" = "tag" ]; then
    branch="release"
  else
    branch="detached"
  fi
fi

commit=$(git rev-parse HEAD 2>/dev/null || echo "")

dirty=false
if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
  dirty=true
fi

ldflags=$(printf -- '-X %s.tag=%s -X %s.commitsSinceTag=%s -X %s.branch=%s -X %s.commit=%s -X %s.dirty=%s' \
  "$package" "$tag" \
  "$package" "$commits" \
  "$package" "$branch" \
  "$package" "$commit" \
  "$package" "$dirty")

if [ "${1:-}" = "--ldflags" ]; then
  printf '%s\n' "$ldflags"
  exit 0
fi

# Rendering is the Go package's job, so there is one formatter rather than a
# shell copy of it that can drift. The flags must be injected here too: without
# them `go run` falls back to debug.ReadBuildInfo, which knows the commit but
# not the tag, and would report a different version than a release build of the
# same tree.
line=$(go run -ldflags "$ldflags" ./cmd/nemosh --version)

if [ "${1:-}" = "--version" ]; then
  printf '%s\n' "$line" | awk '{print $2}'
  exit 0
fi

printf '%s\n' "$line"
