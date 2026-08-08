#!/usr/bin/env bash
# Build nemosh with the version stamped in.
#
# This exists because `go build ./cmd/nemosh` cannot know the version. Go's own
# build info records the commit, the time, and whether the tree was modified --
# but not the tag and not the branch, so a plain build reports
# `v0.0.1-unknown-<commit>`. The tag and branch come from git, which is a
# build-time question, so a script asks it and injects the answer. That is the
# same split xiongnemo/Saki uses, where scripts/build-windows.ps1 plays this
# part.
#
#   ./scripts/build.sh                 build to dist/nemosh[.exe]
#   ./scripts/build.sh -o path         build to a given path
#   ./scripts/build.sh --dev           skip -s -w, keeping symbols for a debugger
#
# Any remaining arguments are passed through to `go build`, so
# `./scripts/build.sh --dev -race` works.

set -euo pipefail

cd "$(dirname "$0")/.."

output=""
strip=true
passthrough=()

while [ $# -gt 0 ]; do
  case "$1" in
    -o)
      [ $# -ge 2 ] || { echo "build.sh: -o requires a path" >&2; exit 2; }
      output="$2"
      shift 2
      ;;
    --dev)
      strip=false
      shift
      ;;
    -h|--help)
      sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      passthrough+=("$1")
      shift
      ;;
  esac
done

if [ -z "$output" ]; then
  output="dist/nemosh"
  # GOOS is what decides the suffix, not the host: a cross-build for Windows
  # from Linux still needs one.
  target="${GOOS:-$(go env GOOS)}"
  [ "$target" = "windows" ] && output="dist/nemosh.exe"
fi

mkdir -p "$(dirname "$output")"

ldflags="$(bash scripts/version.sh --ldflags)"
if [ "$strip" = true ]; then
  ldflags="-s -w $ldflags"
fi

# CGO_ENABLED=0 keeps the result a single file with no runtime sidecars, which
# is what the release target is (AGENTS.md, Builds And Packaging).
CGO_ENABLED="${CGO_ENABLED:-0}" go build -trimpath -ldflags "$ldflags" \
  ${passthrough+"${passthrough[@]}"} -o "$output" ./cmd/nemosh

printf '%s\n' "$output"
"$output" --version
