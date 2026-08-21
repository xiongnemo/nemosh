#!/usr/bin/env nemosh
# V1-RC behaviour acceptance, written in the dialect it is testing.
#
# Run by the binary under test -- `nemosh scripts/v1-rc.sh` -- and that is the point rather than a
# convenience. A clean Windows machine has no bash and no Go toolchain, so the differential corpus
# cannot run there at all; what a clean machine can check is the packaged binary, and the only shell
# guaranteed to be present is the one being accepted. So the script exercises the shell by being
# written in it: `if`, `for`, functions, command substitution, pipes, redirection and `$?` all have to
# work before a single check reports anything.
#
# Every check asserts a claim some document makes. That ordering is deliberate: `top -H` was listed in
# --help and in the support matrix, and did nothing, for as long as nobody executed the documentation.
#
# Exits non-zero if any check fails, so it can gate a release rather than be read.

# The binary under test, which must be named rather than found. The first version of this called
# bare `nemosh` and PATH answered with whatever was installed -- so it tested the Scoop nightly and
# reported that `top -H` was broken, in a build where it had just been fixed. Set NEMOSH_BIN, or
# accept the PATH answer knowingly.
NEMOSH_BIN=${NEMOSH_BIN:-nemosh}
if ! "$NEMOSH_BIN" --version > /dev/null 2>&1; then
	echo "cannot run '$NEMOSH_BIN'; set NEMOSH_BIN to the binary under test" >&2
	exit 2
fi
echo "testing: $("$NEMOSH_BIN" --version | head -n 1)"
echo

passed=0
failed=0

check() {
	# check NAME EXPECTED ACTUAL
	if [ "$2" = "$3" ]; then
		passed=$((passed + 1))
		echo "  ok    $1"
	else
		failed=$((failed + 1))
		echo "  FAIL  $1"
		echo "          expected: $2"
		echo "          actual:   $3"
	fi
}

# refuses NAME COMMAND... -- a capability that is absent must fail loudly, which means a non-zero
# status *and* something on stderr. Silence with a non-zero status is half a refusal.
refuses() {
	name=$1
	shift
	message=$("$@" 2>&1 >/dev/null)
	status=$?
	if [ "$status" = "0" ]; then
		failed=$((failed + 1))
		echo "  FAIL  $name: succeeded, and should have refused"
	elif [ -z "$message" ]; then
		failed=$((failed + 1))
		echo "  FAIL  $name: refused with status $status and said nothing"
	else
		passed=$((passed + 1))
		echo "  ok    $name"
	fi
}

echo "== identity"
version=$("$NEMOSH_BIN" --version)
check "--version answers" "yes" "$(if [ -n "$version" ]; then echo yes; else echo no; fi)"
echo "  ($version)"
applets=$("$NEMOSH_BIN" --list | wc -l)
check "--list names applets" "yes" "$(if [ "$applets" -gt 30 ]; then echo yes; else echo no; fi)"

echo "== the shell itself"
check "exit status of true" "0" "$(true; echo $?)"
check "exit status of false" "1" "$(false; echo $?)"
check "command not found is 127" "127" "$(nosuchcommand 2>/dev/null; echo $?)"
check "pipes" "3" "$(printf 'a\nb\nc\n' | wc -l | tr -d ' ')"
check "command substitution" "hello" "$(echo hello)"
check "variables survive a subshell boundary" "outer" "$(x=outer; (echo $x))"
check "and-or short circuits" "second" "$(false && echo first || echo second)"

echo "== redirection"
echo stdout-only > rc-out.txt
check "> writes" "stdout-only" "$(cat rc-out.txt)"
echo appended >> rc-out.txt
check ">> appends" "2" "$(wc -l < rc-out.txt | tr -d ' ')"
# &> is the one that silently half-worked in other shells: stderr must follow stdout.
ls no-such-file-here &> rc-both.txt
check "&> captures stderr" "yes" "$(if [ -s rc-both.txt ]; then echo yes; else echo no; fi)"
check "2> separates stderr" "" "$(ls no-such-file-here 2>/dev/null)"
rm -f rc-out.txt rc-both.txt

echo "== text, and the width accounting that has broken before"
printf '\xe6\x96\x87\xe5\xad\x97\n' > rc-cjk.txt
# Seven: six bytes of CJK and the newline. This said six, written while `printf '\x'` was still
# broken -- the escapes came out literal, so the number had never been checked against an answer.
check "CJK round-trips through a file" "7" "$(wc -c < rc-cjk.txt | tr -d ' ')"
# Three characters -- 文, 字 and the newline -- against seven bytes. That gap is the whole point:
# this is a deliberate divergence from the reference, and busybox answers 7 here because it counts
# bytes for -m. Asserting the character count is how that stays true.
check "wc -m counts characters" "3" "$(wc -m < rc-cjk.txt | tr -d ' ')"
check "rev does not corrupt multibyte text" "yes" "$(if [ "$(rev < rc-cjk.txt | rev)" = "$(cat rc-cjk.txt)" ]; then echo yes; else echo no; fi)"
printf 'a\r\nb\r\n' > rc-crlf.txt
check "CRLF input counts as two lines" "2" "$(wc -l < rc-crlf.txt | tr -d ' ')"
rm -f rc-cjk.txt rc-crlf.txt

echo "== a name with a space and a name in CJK"
mkdir -p "rc dir/文字"
echo inside > "rc dir/文字/file.txt"
check "quoted paths with spaces" "inside" "$(cat "rc dir/文字/file.txt")"
check "find reaches it" "1" "$(find "rc dir" -name 'file.txt' | wc -l | tr -d ' ')"
rm -rf "rc dir"

echo "== environment"
check "an exported variable reaches a child" "seen" "$(RC_PROBE=seen "$NEMOSH_BIN" -c 'echo $RC_PROBE')"
# Windows environment names are case-insensitive, and a shell that treats PATH and Path as two
# variables hands a child two conflicting values.
check "PATH and Path are one variable" "yes" "$(if [ -n "$PATH" ]; then echo yes; else echo no; fi)"

echo "== capabilities that are absent, which must say so"
refuses "fg refuses" "$NEMOSH_BIN" -c 'fg'
refuses "bg refuses" "$NEMOSH_BIN" -c 'bg'
refuses "hash refuses" "$NEMOSH_BIN" -c 'hash'
refuses "ulimit refuses" "$NEMOSH_BIN" -c 'ulimit'
refuses "set -n refuses" "$NEMOSH_BIN" -c 'set -n'
refuses "an unknown applet option refuses" "$NEMOSH_BIN" -c 'ls --no-such-option'
refuses "an unknown top column refuses" "$NEMOSH_BIN" -c 'top -b -o nosuchcolumn'

echo "== the process monitor, whose documentation was wrong once"
check "top -b prints a summary" "yes" "$(if "$NEMOSH_BIN" -c 'top -b -n 1' | grep -q 'processes'; then echo yes; else echo no; fi)"
check "top -b has a column header" "yes" "$(if "$NEMOSH_BIN" -c 'top -b -n 1' | grep -q 'COMMAND'; then echo yes; else echo no; fi)"
# The claim that was false: -H is documented as showing threads, and for a while it changed nothing.
plain=$("$NEMOSH_BIN" -c 'top -b -n 1' | wc -l)
threads=$("$NEMOSH_BIN" -c 'top -b -n 1 -H' | wc -l)
check "top -H adds thread rows" "yes" "$(if [ "$threads" -gt "$plain" ]; then echo yes; else echo no; fi)"
echo "  ($plain rows, $threads with -H)"
check "top -o selects columns" "yes" "$(if "$NEMOSH_BIN" -c 'top -b -n 1 -o pid,command' | grep -q 'PID'; then echo yes; else echo no; fi)"
check "ps reports processes" "yes" "$(if "$NEMOSH_BIN" -c 'ps' | grep -q 'COMMAND'; then echo yes; else echo no; fi)"

echo "== a reader that goes away ends the output rather than failing it"
# The bug this script found on its first run. A producer whose consumer exits early gets a write
# error on Windows, where POSIX would have delivered SIGPIPE and said nothing -- so `cmd | head -1`
# printed "write /dev/stdout: The pipe is being closed." busybox-w32 is silent; measured.
noise=$("$NEMOSH_BIN" -c "\"$NEMOSH_BIN\" -c 'seq 1 200000' | grep -q 7" 2>&1 >/dev/null)
check "no complaint when a pipe closes early" "" "$noise"

echo "== help, for every applet, because an applet with no usage is undocumented"
missing=0
for name in $("$NEMOSH_BIN" --list); do
	# `false` exits non-zero by definition, whatever it is asked, and `[` needs its closing
	# bracket before it will do anything at all. Neither is missing a usage.
	if [ "$name" = "false" ] || [ "$name" = "[" ]; then
		continue
	fi
	if ! "$NEMOSH_BIN" -c "$name --help" > /dev/null 2>&1; then
		missing=$((missing + 1))
		echo "        no usage: $name"
	fi
done
check "every applet answers --help" "0" "$missing"

echo
echo "passed $passed, failed $failed"
if [ "$failed" != "0" ]; then
	exit 1
fi
