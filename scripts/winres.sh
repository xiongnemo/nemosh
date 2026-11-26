#!/usr/bin/env bash
# The Windows version resource -- what Explorer's Properties, Details shows of nemosh.exe.
#
#   ./scripts/winres.sh amd64 [arm64 386 ...]   write cmd/nemosh/rsrc_windows_<arch>.syso
#   ./scripts/winres.sh --check BINARY          check what BINARY's resource says
#
# `go build` links cmd/nemosh/rsrc_windows_<arch>.syso into nemosh.exe for that architecture
# by its name alone. The version is scripts/version.sh's, the one derivation there is: the
# full string, v and all, is FileVersion and ProductVersion, and its X.Y.Z with a fourth part
# of 0 the numeric versions Windows compares. A version with a dash is a build of no release
# tag, which the release workflow publishes as a prerelease, so its resource is flagged
# Prerelease. RT_VERSION only: nemosh has no manifest and no icon.
#
# The .syso files are build output, which git ignores. A tree that has one links it into every
# build after, whatever version that build is, so scripts/build.sh removes them once it has
# built; a CI job is a checkout of its own.

set -euo pipefail

here=$(dirname "$0")
version=$(bash "$here/version.sh" --version)
numeric="${version#v}"
numeric="${numeric%%-*}"
case "$numeric" in
  *[!0-9.]* | "") echo "winres.sh: $version has no X.Y.Z" >&2; exit 1 ;;
esac
flags="" prerelease=False
case "$version" in
  *-*) flags="Prerelease" prerelease=True ;;
esac

if [ "${1:-}" = "--check" ]; then
  [ $# -eq 2 ] || { echo "winres.sh: --check takes the binary" >&2; exit 2; }
  # Read as Explorer reads it, through FileVersionInfo, from where the caller is, in the
  # spelling PowerShell reads: /c/... is Git's bash's.
  binary="$2"
  if command -v cygpath >/dev/null; then
    binary=$(cygpath -w "$binary")
  fi
  shown=$(powershell.exe -NoProfile -NonInteractive -Command "\$v = (Get-Item -LiteralPath '$binary').VersionInfo;
    \$v.ProductVersion; \$v.FileVersion; \$v.ProductName; \$v.OriginalFilename;
    '{0}.{1}.{2}.{3}' -f \$v.FileMajorPart, \$v.FileMinorPart, \$v.FileBuildPart, \$v.FilePrivatePart;
    \$v.IsPreRelease" | tr -d '\r')
  want=$(printf '%s\n' "$version" "$version" nemosh nemosh.exe "${numeric}.0" "$prerelease")
  if [ "$shown" != "$want" ]; then
    echo "winres.sh: $2's version resource says"
    printf '%s\n' "$shown" | sed 's/^/  /'
    echo "where the build's version gives"
    printf '%s\n' "$want" | sed 's/^/  /'
    exit 1
  fi
  echo "$2: $version, $numeric.0, prerelease $prerelease"
  exit 0
fi

[ $# -gt 0 ] || { echo "winres.sh: name the architectures, as amd64 arm64 386" >&2; exit 2; }
cd "$here/.."

json=$(mktemp)
trap 'rm -f "$json"' EXIT
cat >"$json" <<EOF
{
  "RT_VERSION": {
    "#1": {
      "0000": {
        "fixed": {
          "file_version": "${numeric}.0",
          "product_version": "${numeric}.0",
          "flags": "${flags}"
        },
        "info": {
          "0409": {
            "CompanyName": "Nemo Xiong",
            "FileDescription": "BusyBox-style shell and applets in one Windows executable.",
            "FileVersion": "${version}",
            "InternalName": "nemosh",
            "LegalCopyright": "Copyright © 2026 Nemo Xiong",
            "OriginalFilename": "nemosh.exe",
            "ProductName": "nemosh",
            "ProductVersion": "${version}"
          }
        }
      }
    }
  }
}
EOF

arches=$(IFS=,; printf '%s' "$*")
# Built for the machine it runs on, whatever GOOS and GOARCH the build it serves is for; and
# told the architectures outright, since it reads GOARCH for them too.
env -u GOOS -u GOARCH go run github.com/tc-hib/go-winres@v0.3.3 make \
  --in "$json" --out cmd/nemosh/rsrc --arch "$arches"
for arch in "$@"; do
  printf 'cmd/nemosh/rsrc_windows_%s.syso\n' "$arch"
done
