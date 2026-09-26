#!/usr/bin/env bash
# Refresh tests/oils, the vendored copy of the Oils spec suite.
#
# tests/oils is a measuring instrument, not part of the corpus: its cases run as
# upstream wrote them and pin nothing, and a behaviour worth keeping is promoted
# into the corpus by hand (docs/design/reference-methodology.md). So the copy is
# never edited. This script replaces it whole from one commit of an Oils clone,
# and writes tests/oils/upstream.json, which records that commit and each
# file's mode, sha256 and case count, and which a test holds the copy to.
#
#   bash scripts/oils-vendor.sh                copy from references/shells/oils
#   bash scripts/oils-vendor.sh path/to/oils   copy from another clone
#
# What is copied, all of it from the clone's HEAD rather than its working tree:
#   - every spec/*.test.sh whose compare_shells line names bash, less ysh-* and
#     hay*, which test Oils' own languages;
#   - all of spec/testdata, which cases source and run by path;
#   - spec/bin/builtins-exec-here-doc-helper.sh, the one helper a case runs by
#     path. The ones called by name, argv.py and the rest, are Python and are
#     not copied: whatever runs the cases provides its own;
#   - LICENSE.txt.
#
# The result is staged, because git on Windows cannot see an executable bit on
# disk: upstream's modes are carried over with `git update-index --chmod`.
# When the commit changes, change it and its date in THIRD-PARTY-NOTICES.md and
# tests/oils/README.md too; a test checks both.

set -euo pipefail

cd "$(dirname "$0")/.."

source_dir="${1:-references/shells/oils}"
dest="tests/oils"
helper="spec/bin/builtins-exec-here-doc-helper.sh"

if ! git -C "$source_dir" cat-file -e "HEAD:$helper" 2>/dev/null; then
  echo "oils-vendor.sh: $source_dir is not a clone of oils-for-unix/oils" >&2
  exit 2
fi
commit=$(git -C "$source_dir" rev-parse HEAD)
committed=$(git -C "$source_dir" log -1 --format=%cs HEAD)

# The spec files, chosen by what HEAD holds, since the working tree may differ.
selected=()
while IFS= read -r path; do
  path=${path#HEAD:}
  case ${path#spec/} in ysh-* | hay*) continue ;; esac
  selected+=("$path")
done < <(git -C "$source_dir" grep -l -e '^## compare_shells:.*bash' HEAD -- ':(glob)spec/*.test.sh')

# One "mode sha path" line per file to copy, sorted as Go sorts map keys.
listing=$(git -C "$source_dir" ls-tree -r HEAD -- LICENSE.txt spec/testdata "$helper" "${selected[@]}" |
  awk '{ print $1, $3, $4 }' | LC_ALL=C sort -k3)

git rm -r -q -f --ignore-unmatch -- "$dest/spec" "$dest/LICENSE.txt"

# A process per file costs a minute on Windows, so the loop only copies, and the
# hashes and case counts are taken afterwards, one process for all the files.
paths=()
specs=()
executable=()
made=""
while read -r mode sha path; do
  case $path in *[!A-Za-z0-9._/-]*)
    echo "oils-vendor.sh: $path needs JSON escaping, which this script does not do" >&2
    exit 1 ;;
  esac
  case $path in */*)
    if [ "${path%/*}" != "$made" ]; then
      made=${path%/*}
      mkdir -p "$dest/$made"
    fi ;;
  esac
  git -C "$source_dir" cat-file blob "$sha" > "$dest/$path"
  paths+=("$path")
  case $path in *.test.sh) specs+=("$path") ;; esac
  if [ "$mode" = 100755 ]; then
    chmod +x "$dest/$path"
    executable+=("$dest/$path")
  fi
done <<< "$listing"

hashes=$(cd "$dest" && if command -v sha256sum >/dev/null; then sha256sum -b "${paths[@]}"; else shasum -a 256 -b "${paths[@]}"; fi)
# A case begins at any line starting ####, whatever follows, as test/sh_spec.py reads it.
counts=$(cd "$dest" && grep -c '^####' "${specs[@]}")

awk -v commit="$commit" -v committed="$committed" '
  FNR == 1 { input++ }
  input == 1 { split($0, part, ":"); cases[part[1]] = part[2]; next }
  input == 2 { hash[substr($2, 2)] = $1; next }
  !($3 in hash) { print "oils-vendor.sh: no sha256 for " $3 > "/dev/stderr"; failed = 1; exit }
  { entry[++n] = sprintf("\"%s\": {\"mode\": \"%s\", \"sha256\": \"%s\"%s}", $3, $1, hash[$3], ($3 in cases) ? ", \"cases\": " cases[$3] : "") }
  END {
    if (failed) exit 1
    print "{"
    print "  \"repository\": \"https://github.com/oils-for-unix/oils\","
    printf "  \"commit\": \"%s\",\n", commit
    printf "  \"committed\": \"%s\",\n", committed
    print "  \"files\": {"
    for (i = 1; i <= n; i++) printf "    %s%s\n", entry[i], (i < n ? "," : "")
    print "  }"
    print "}"
  }' <(printf '%s\n' "$counts") <(printf '%s\n' "$hashes") <(printf '%s\n' "$listing") > "$dest/upstream.json"

git add -- "$dest/spec" "$dest/LICENSE.txt" "$dest/upstream.json"
git update-index --chmod=+x -- "${executable[@]}"

total=0
while IFS=: read -r _ count; do total=$((total + count)); done <<< "$counts"
echo "oils-vendor.sh: ${#specs[@]} spec files with $total cases, ${#paths[@]} files in all, from oils ${commit:0:12} ($committed)"
