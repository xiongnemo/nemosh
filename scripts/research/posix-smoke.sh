#!/bin/sh

# Lightweight behavior probe for research CI.
# Usage: scripts/research/posix-smoke.sh <shell> [shell-arg...]

set -eu

if [ "$#" -eq 0 ]; then
  echo "usage: $0 <shell> [shell-arg...]" >&2
  exit 64
fi

tmp=${TMPDIR:-/tmp}/nemosh-posix-smoke-$$
mkdir -p "$tmp"
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

case_file=$tmp/case.sh
cat >"$case_file" <<'CASE'
set -u

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

[ "$(printf '%s\n' 'a b')" = 'a b' ] || fail 'command substitution or quoting'

v=alpha
[ "${v:-fallback}" = alpha ] || fail 'parameter expansion set value'
unset v
[ "${v:-fallback}" = fallback ] || fail 'parameter expansion default value'

set -- one 'two words' three
[ "$#" -eq 3 ] || fail 'positional count'
[ "$2" = 'two words' ] || fail 'positional quoting'

out=$(printf '%s\n' one two three | while IFS= read -r line; do printf '<%s>' "$line"; done)
[ "$out" = '<one><two><three>' ] || fail 'pipeline read loop'

redir_file=${TMPDIR:-/tmp}/nemosh-redir-$$
printf 'hello\n' >"$redir_file"
[ "$(cat <"$redir_file")" = hello ] || fail 'redirection'
rm -f "$redir_file"

here=$(cat <<'EOF'
literal $HOME
EOF
)
[ "$here" = 'literal $HOME' ] || fail 'quoted here document'

x=outer
( x=inner )
[ "$x" = outer ] || fail 'subshell environment isolation'

false && fail 'and-list false branch'
true || fail 'or-list true branch'

printf 'OK\n'
CASE

printf '== shell command ==\n'
printf '%s\n' "$*"
printf '== version probe ==\n'
("$@" --version 2>/dev/null || "$@" -c 'echo version-unavailable') | sed -n '1,3p'
printf '== behavior probe ==\n'
"$@" "$case_file"
