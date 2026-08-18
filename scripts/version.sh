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

# Who built it, where, and when -- the Linux kernel's banner, which answers what
# a version number cannot: two binaries claiming one commit can still differ, and
# the first thing worth knowing about a misbehaving one is whether it came off CI
# or somebody's laptop.
#
# UTC and to the second. A local time would make the same build look different
# from two desks, and anything finer is noise.
build_time=$(date -u +%Y-%m-%dT%H:%M:%SZ)
build_user=${USER:-${USERNAME:-unknown}}
build_host=$(hostname 2>/dev/null || echo unknown)

ldflags=$(printf -- '-X %s.tag=%s -X %s.commitsSinceTag=%s -X %s.branch=%s -X %s.commit=%s -X %s.dirty=%s -X %s.buildTime=%s -X %s.buildUser=%s -X %s.buildHost=%s' \
  "$package" "$tag" \
  "$package" "$commits" \
  "$package" "$branch" \
  "$package" "$commit" \
  "$package" "$dirty" \
  "$package" "$build_time" \
  "$package" "$build_user" \
  "$package" "$build_host")

if [ "${1:-}" = "--ldflags" ]; then
  printf '%s\n' "$ldflags"
  exit 0
fi

# Rendering is the Go package's job, so there is one formatter rather than a
# shell copy of it that can drift. The flags must be injected here too: without
# them `go run` falls back to debug.ReadBuildInfo, which knows the commit but
# not the tag, and would report a different version than a release build of the
# same tree.
# The first line only. `--version` prints a second one naming who built it, and
# this script's job is the version -- feeding both to awk printed `by` as a
# second answer, which is how this comment came to exist.
#
# Taken with a parameter expansion rather than `| head -n 1`, because head exits
# as soon as it has its line and `go run` then writes into a closed pipe. Under
# `set -o pipefail`, which is what GitHub's bash steps run with, that SIGPIPE
# fails the step: the darwin/arm64 release build died with `signal: broken pipe`
# on two commits out of three, passing on the third. A race that fails a release
# one time in three is worse than a slow pipe.
output=$(go run -ldflags "$ldflags" ./cmd/nemosh --version)
line=${output%%
*}

if [ "${1:-}" = "--version" ]; then
  printf '%s\n' "$line" | awk '{print $2}'
  exit 0
fi

printf '%s\n' "$line"
