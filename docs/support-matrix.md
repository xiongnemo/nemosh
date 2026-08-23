# Support Matrix

Everything here was **measured against a built binary on 2026-08-07**, not read
off the source or carried from an earlier document. The probe ran each applet
with an option it could not possibly implement (`-%`), then with each plausible
option letter, and recorded what came back. Where this table says an option is
unsupported, that is an observation.

Re-measure this file rather than editing it by hand when applet coverage
changes; see AGENTS.md, Documentation Hygiene.

## Platforms

A binary being published is not the same as the platform being supported, and
the second column is the one that decides. Four of these ship archives; one is
supported.

| Platform | Status | What that means |
| --- | --- | --- |
| `windows/amd64` | **Supported** | The target. Bugs here are bugs. Behavior corpus, differential suite against busybox-w32/ash, and native path, launch, device, and interrupt tests all run here. |
| `linux/amd64` | **Build and test only**, binary published | CI compiles and runs the full suite, which is what keeps the platform splits from rotting. Not a support commitment: three interactive interrupt tests are skipped here, and the Windows-only surfaces (clipboard device, `ComSpec` batch launch, 8.3 fallback, case-preserving `cd`) have no counterpart. |
| `linux/arm64` | **Compile only**, binary published | Cross-compiled and never executed. |
| `darwin/arm64` | **Build and test only**, binary published | `macos-latest` runs the full suite, which is what makes the `_other.go` half of every platform split — process listing, identity, executability, device input — *executed* rather than merely compiled. It found three failures the hour it was added, one of them a real defect in `command -v`. The strict differential does not run there: the only reference shell macOS ships is bash 3.2, so a divergence says more about 2007 than about Nemosh. |
| `darwin/amd64` | **Compile only**, binary published | Cross-compiled. GitHub's macOS runners are arm64, so the Intel build is never executed. |
| `windows/arm64` | **Untested** | Not built, not run, not claimed. |

Go 1.26, `CGO_ENABLED=0`, single binary, no runtime sidecars.

## Shell

Implemented and covered by the behavior corpus (145 cases) and the differential
suite: sequential lists, pipelines with `!` negation, `&&`/`||`, brace groups,
subshells, functions, `if`/`elif`, `for`, `while`/`until`, `case` including the
one-line forms, heredocs, redirections including `>|` and `<>`, background jobs
with `jobs` and `wait`, traps, parameter expansion with the selected operators,
field splitting, pathname expansion, arithmetic expansion, command substitution
in both spellings, aliases, and `local`.

### Refused on purpose, with a reason and a non-zero status

A capability that is absent fails loudly rather than approximating. Each of
these names why, and names what busybox-w32 does with the same name.

| Name | Status | Why |
| --- | --- | --- |
| `hash` | 126 | Command lookup is not cached, so there is nothing to remember or forget. busybox-w32 does implement it, over a hash table this shell does not have. |
| `ulimit` | 126 | Windows has no `getrlimit`. busybox-w32 does not implement it either — it keeps the name and returns 1 with no message. |
| `fg`, `bg` | 126 | They resume a *suspended* job and nothing here can suspend one — see **Process control** below, which is the long answer. busybox-w32 compiles both out under `#if JOBS`. These two say **"not implemented, and will not be"** where the rows above say only "not implemented", because they are settled rather than pending. |
| `set -b` | 2 | Asynchronous completion is reported when `wait` or `jobs` asks; there is no notification channel to switch on. |
| `set -n`, `set -v` | 2 | A script is parsed in full before any of it runs, so by the time the option is set there is no unread input left to withhold or echo. |

Beyond POSIX, `history`, `which` and `set -o nocaseglob` are implemented, both
following busybox.

History survives the session. `HISTFILE` names the file and defaults to
`~/.nemosh_history`; `HISTFILESIZE` caps it at 500 lines by default; setting
either to nothing turns saving off, which is bash's rule and busybox's. Lines
are appended one at a time in a single write, so a session that is killed still
leaves what it ran and two windows interleave whole lines rather than
overwriting each other, and the file is rewritten only once it has grown to four
times the cap (`libbb/lineedit.c:1826`, `:1841`).

Tab completion and the inline suggestion offer host names for `ssh`, read from
`~/.ssh/config` -- and `/etc/hosts` off Windows. See
`docs/design/completion.md`, Host names, for what is read and what deliberately
is not.

### Process control — what is implemented, and what will not be

One table, because the line between the two halves is a single distinction and
it is easier to see them together.

| | Implemented | Why it can be |
| --- | --- | --- |
| `jobs`, `wait`, `wait %N` | yes | bookkeeping over the shell's own job table |
| `kill %N` | yes | ending a job maps onto cancelling its context |
| `kill PID`, `kill -l` | yes | `TerminateProcess` on Windows, a real signal elsewhere |
| `pgrep`, `pkill` | yes | `CreateToolhelp32Snapshot` lists, the above terminates |
| **`fg`, `bg`** | **no, and not planned** | they resume a *suspended* job, and nothing here can suspend one |
| **Ctrl-Z as suspend** | **no** | same primitive; Ctrl-Z exits the shell on an empty line instead |

**The distinction is direction, not difficulty.** Ending something and cancelling
a context are both one-way doors, so `kill %N` is an honest implementation of the
real thing. Suspension needs a door that opens both ways — stop now, continue
later from exactly here — and there is no such door at any layer beneath this
shell:

1. **Go cannot suspend a goroutine from outside.** The runtime parks one only when
   the goroutine itself blocks — a channel, a mutex, a syscall, a sleep. There is
   no `runtime.Suspend`. And cancellation is not a substitute: it sets a flag, the
   goroutine notices at a checkpoint it chose, and unwinds. The stack is gone;
   there is no un-cancelling.
2. **A cooperative pause would be a lie.** A pause channel checked at loop
   boundaries would stop only where an applet chose to look, so a tight loop would
   ignore Ctrl-Z silently, every applet would have to implement it, and an
   external process in a background job could not be paused this way at all.
   Silent partial obedience is worse than a refusal.
3. **Windows has no `SIGSTOP` even for real processes.** `NtSuspendProcess` is
   undocumented; `SuspendThread` is documented and warned against, because
   suspending a thread that holds the heap lock deadlocks the process; debuggers
   use `DebugActiveProcess`, which is different semantics. There is no supported
   stop-and-continue primitive to build on.

busybox-w32 reaches the same conclusion, and its own comment draws the same line:
`JOBS` is `0` under `ENABLE_PLATFORM_MINGW32` (`shell/ash.c:247-253`), where the
comment reads *"JOBS_WIN32 doesn't enable job control, just some job-related
features"*. Those job-related features are precisely the top half of the table
above. Measured: `SIGSTOP`, `SIGTSTP` and `SIGCONT` appear nowhere in its `win32/`
layer.

Making `fg` work would mean moving background jobs from goroutines to real child
processes and then gambling on `SuspendThread` against the heap lock — trading a
property that holds for a feature that might.

### `kill`

A builtin, as busybox's is (`shell/ash.c:12096`), and for the same reason: `%N`
names a job and only the shell has the job table. busybox's `killcmd` does
nothing but translate `%N` into that job's pids and hand them to the ordinary
`kill` (`:4787-4830`).

Here there is nothing to translate into, because a background job is a goroutine
and has no pid. What it has is its own context, so the signal arrives as a
cancellation — and for the case that matters most that is not a weaker
substitute: an external command in a background job is launched with
`exec.CommandContext` under that context, so cancelling it terminates the real
process.

| Form | Behaviour |
| --- | --- |
| `kill %N` | cancels that job. Every signal cancels; a goroutine has no handler, so telling TERM from KILL would be a promise this cannot keep |
| `kill PID` | `TerminateProcess` on Windows, as busybox does (`win32/process.c:909`), a real signal elsewhere |
| `kill -9`, `kill -TERM`, `kill -SIGTERM` | all accepted; a script writes the number and a person writes the name |
| `kill -l` | lists the signals this shell can act on, not the whole POSIX set |
| a pid that has already exited | refused, not reported as killed — the check busybox makes with `GetExitCodeProcess` first |
| pid `0` or negative | refused on Windows: those mean process groups, which Windows has not got in the POSIX sense. Passed through elsewhere |

`kill` does not claim the job, so a later `wait %N` still finds it.

### Elevation

A program whose manifest demands administrator — `WinSAT`, `bcdedit`, `sfc` —
cannot be started from here. `CreateProcess` refuses it with
`ERROR_ELEVATION_REQUIRED` (740), and there is no flag that changes that:
Windows elevates through `ShellExecuteEx` with the `runas` verb, or through a
COM elevation moniker, and neither is a variant of `CreateProcess`.

```console
$ WinSAT
WinSAT: requires administrator, and this shell does not elevate on its own
hint: start an elevated shell and run it there, or launch it through a tool that elevates (`gsudo WinSAT ...`). See docs/support-matrix.md, Elevation
$ echo $?
126
```

**busybox-w32 does the opposite**, and this is a deliberate divergence rather
than a gap. Its `mingw_execve` retries through `ShellExecuteEx` with `runas`
when the launch comes back with 740 (`win32/process.c:560-566`, `shell_execute`
at `:514`). Two reasons not to follow it:

1. **`ShellExecuteEx` cannot pass handles to the child.** The elevated process
   gets a new console, so every redirection and pipe in the command is silently
   ignored. `WinSAT formal > report.txt` would leave an empty file and put the
   real output in a window that closes when it finishes. That is the same silent
   partial obedience refused under **Process control**, and it is no better for
   being convenient.
2. **A consent dialog that appears because a name was typed is a dialog people
   learn to dismiss.** Elevation is worth asking for on purpose.

What works today: run an already-elevated shell, or put an elevating tool in
front of the command. `gsudo` is the usual one on Windows and does the part that
is genuinely hard — an elevated helper relaying stdio back over a named pipe, so
redirection keeps working.

**The deliberate route is `su`, and it is implemented.** busybox-w32 answered this
question first and answered it with `su` (`loginutils/suw32.c`, applet-odd-named
from `suw32`), so that is the name here too. It is **not POSIX** — checked
against POSIX.1-2024, which has a `newgrp` page and no `su` page at all — so the
reference is the whole of the specification.

What it does *not* do is the important part: it does not elevate a command
inside the current pipeline. It launches **a new elevated shell in its own
console** through `ShellExecuteEx`/`runas`, and `su -c CMD` runs `CMD` in that
shell rather than in this one.

| Form | Behaviour |
| --- | --- |
| `su`, `su root` | an elevated `nemosh -i` **in the console you are already in**, starting in the current directory |
| `su -c CMD` | that shell runs `CMD`, in the same console |
| `su -s SHELL` | launches `SHELL` instead. `cmd.exe` is given `/c`, everything else `-c`, matching busybox (`suw32.c:118-120`) |
| `su -W` | waits and reports the shell's exit status; without it `su` returns as soon as the shell is launched, having nothing to report |
| `su -t` | test mode: the `open` verb instead of `runas`, so the whole path runs with no elevation and no consent dialog. This is what makes any of it testable |
| `su USER` for any other user | refused. There is no user database here; `root` is the name this shell gives an elevated token, not an account |
| `su -N` | holds the console open at exit, so the output of `-c` survives the shell it ran in. Only meaningful where a window is going to close, so it is dropped on the in-place path; the shell grew a leading `-N` for it, as busybox's ash did (`shell/ash.c:13442`, `:16371`) |
| the consent dialog answered "no" | status 1, `elevation was refused` — a decision, not a fault |

The working directory is passed explicitly and canonicalised first, because a
directory reached through a mapped network drive may not exist under the
elevated token — drive mappings belong to a logon session (`suw32.c:96-113`).
Measured: without it, ShellExecuteEx decides for itself.

#### Running in the current console

busybox always gives its elevated shell a window of its own. This one does not,
and the reason is what the symptom was: a new console is a *plain* console.
Nothing has turned on `ENABLE_VIRTUAL_TERMINAL_PROCESSING` in it, so a shell
that draws in colour draws escape codes as text instead, and the size, font and
scrollback are not the ones being used.

**The launcher still cannot hand its console over.** Only the AppInfo service can
create an elevated process, and it is reached through `ShellExecuteEx` or a COM
elevation moniker, neither of which takes a `STARTUPINFO`. That much of the
original reasoning stands.

**The child can take it.** `AttachConsole` attaches the caller to another
process's console, and an elevated process attaching to a medium-integrity one is
allowed — privilege runs the other way. So the handover happens on the far side:
the shell is launched with `SEE_MASK_NO_CONSOLE`, given `--attach-console PID`,
and joins before it reads a stream. `CONIN$` and `CONOUT$` are opened *after* the
attach, because a process launched without a console has no valid standard
handles to inherit. This is what `gsudo` does, and it is why `gsudo` can run a
command in place where `ShellExecuteEx` alone cannot.

Two consequences, enforced rather than merely documented:

- **`-W` is implied.** Two shells attached to one console would both read the
  keyboard. The one that launched waits, and does not read, until the elevated
  one exits.
- **`-s SHELL` keeps its own window.** "Join this console" is an option of *this*
  shell; a foreign program has no equivalent to be told.

Where there is no console to join — a service, a CI runner — the plan falls back
to a window, which is the old behaviour and the only one available.

Separately and underneath all of this: the shell now turns
`ENABLE_VIRTUAL_TERMINAL_PROCESSING` on for its own output at startup and puts
the mode back on the way out. It never did, and only ever worked because ConPTY
turns it on for everything Windows Terminal runs. Measured on this machine: a
handle opened on `CONOUT$` reports mode `0x0003`, with `0x0004` clear. Anyone
running `nemosh.exe` from a classic console window was seeing literal escapes.

Ctrl-C while `-W` is waiting stops the wait and says so; it cannot stop the
shell. Terminating a high-integrity process from a medium-integrity one is
refused by the same mechanism that made elevation necessary in the first place.

`su` is registered **on Windows only**. Unix already has a real `su` in
util-linux, and an applet of that name would shadow it while doing none of what
it does — no setuid, no user database, no password. busybox-w32 makes the same
split, building `suw32` under `PLATFORM_MINGW32` alone.

Not `sudo`: Windows 11 24H2 ships a real `sudo.exe`, and the name promises one
command in the current console with its streams intact — which is precisely what
this cannot deliver. busybox also ships the complement, `drop`/`cdrop`/`pdrop`,
for running with the Administrators group disabled; those are not implemented
here.

### Beyond POSIX, on purpose

**Brace expansion.** `{a,b}`, `pre{a,b}post`, `{1..5}`, `{5..1}`, `{a..e}`,
`{01..03}`, `{1..10..3}`, and any nesting or product of those. dash has none of
it; this follows bash, measured case by case, because it is what fingers do.

It runs **before every other expansion**, which is the fact that decides the
implementation rather than a detail of it: with `x=1`, `echo {$x,2}` prints
`1 2`, so the split cannot be done on expanded text. It therefore works on the
word's *parts* -- each unquoted literal contributes characters, and a parameter,
a substitution, an escape or anything quoted becomes one opaque atom no brace can
be found inside. `"{a,b}"` and `\{a,b\}` come out literal from that alone,
without a special case.

Where it deliberately does nothing, all measured against bash: a group with
neither a comma nor a range (`{a}`, `{}`), an unmatched brace (`echo {a,b`), a
range whose endpoints are not both numeric or both alphabetic (`{1..x}`,
`{a..3}`), and a case pattern -- there the pattern is the point, the same reason
pathname expansion is kept away from it.

**`[[ ]]`, the conditional expression.** Not POSIX -- dash has only `[` -- and
this follows bash, measured case by case.

The reason it exists is the reason it cannot be an applet: inside `[[ ]]` a word
is neither split nor globbed, so `[[ $x == "a b" ]]` works where
`[ $x = "a b" ]` becomes `[ a b = a b ]` and is a usage error. An applet receives
words that have already been split; by then the information is gone. So `[[` is
intercepted before expansion, with the word AST still in hand -- which also
supplies the other thing an applet could not know: whether the right-hand side
was quoted, and therefore whether `==` compares a pattern or a literal.

| | |
| --- | --- |
| `==`, `=`, `!=` | the right side is a **pattern** unless quoted. `[[ abc == a* ]]` is true, `[[ abc == "a*" ]]` is false |
| `=~` | an extended regular expression, anchored nowhere |
| `<`, `>` | lexical comparison, **not redirection** -- which is a lexer question, and the reason `[[` had to become known to the lexer |
| `-eq -ne -lt -le -gt -ge` | numeric |
| unary tests, `-nt -ot -ef` | `test`'s own, through one exported entry point, because two copies of `-f` would drift |
| `&&`, `||`, `!`, `( )` | the conditional's own grammar, not the shell's |
| a malformed expression | **status 2**, so "that was not an expression" stays distinguishable from "the answer is no" |

Two limitations, stated rather than hidden. The expression must be on **one
line**: bash can span lines because `[[` is a reserved word its parser knows,
while here it is recognised at execution time, after the line has been divided
into commands. And `[[` is only a conditional at the **start of a command** --
`echo [[` prints two ordinary words.

**Indexed arrays.** `a=(one two three)`, `${a[0]}`, `${a[@]}`, `${a[*]}`,
`${#a[@]}`, `${#a[0]}`, `${!a[@]}`, `a[1]=x`, `a+=(four)`, `a=()`. Neither dash
nor ash has them; this follows bash, measured case by case.

The distinction that carries the feature is `"${a[@]}"` against `"${a[*]}"`: the
first is one word per element, so an element containing a blank survives, and the
second is a single word joined by IFS. Without the first there would be no reason
to have arrays at all -- a string would do.

Storage is separate from the scalar variables rather than packed into one string
with a separator, because that representation cannot hold an element containing
the separator, which is exactly the case arrays exist for. A bare `$a` is element
zero, as in bash.

Assignment is settled **before expansion**, like `[[ ]]` and for the same reason:
`a=(one "two words" three)` is three elements, and by the time a word has been
expanded the quotes are gone.

`a=(...)`'s parenthesis is part of a word rather than a subshell, and **four
layers had to learn that** -- the logical-line scanner, the group parser, the
deferred scan, and the lexer. Each refused it with a different message on the way
(`syntax error: unexpected )`, then `unsupported syntax: grouping`), and there is
a test for each ordinary use of parentheses -- subshell, command substitution,
arithmetic, function definition -- so that none of them moved.

Not implemented: associative arrays (`declare -A`), slices (`${a[@]:1}`),
`unset a[i]` (which leaves a sparse array in bash, and compacting instead would
silently shift every later index), and negative indices.

### Known divergences from bash/dash/ash

- **Parse before effects.** A syntax error anywhere in a script means none of it
  runs. bash and dash execute up to the error. `{echo bad;}` produces no output
  here; both references print nothing either but reach the command first.
- `~user` is left as written. `~` and `~/path` work.
- An alias whose value is not a list of words is refused at definition time,
  because substitution happens after parsing.
- `${#@}` is not pinned; POSIX leaves it unspecified and the references disagree.

## Applets

All 63 registered applets ship, plus `su` on Windows. **Name presence is not option parity**, and the
column that matters is the third one.

### Devices

**Windows only, with one exception.** The device model exists because Windows has
no `/dev`; on Linux and macOS the system has a real one with the machine's devices
in it, and that is the right answer, so those builds reach it through the ordinary
filesystem and this shell provides nothing under it. `/dev/clipboard` is therefore
a Windows facility and simply a path that does not exist elsewhere.

The exception is the **descriptor aliases** -- `/dev/stdin`, `/dev/stdout`,
`/dev/stderr`, `/dev/fd/N` -- which the shell answers for on every platform,
because they are not hardware. They name *this shell's* descriptors, which after a
redirect are not the process's, and which its fd table may hold as something that
is not an operating-system file at all: a pipe it made, a buffer, the clipboard.
bash documents both routes for itself, using the platform's special files where
they exist and emulating them where they do not; emulating is what keeps the fd
table authoritative.

That constraint is held by a pair of tests that fail on opposite platforms --
`device_platform_windows_test.go` and `device_platform_other_test.go` -- rather
than by a comment. CI made the case for it: with the interception left in place,
the completion tests listed the real `/dev` on ubuntu and macos, three hundred
ttys and every loop device, through a code path written to serve eight synthetic
names.

`/dev` is a set of names rather than a directory. Each is openable, and since
Stage 1 of `docs/design/device-filesystem.md` each is also *observable*: `test -e`
and `ls -l` answer for it.

| path | read | write | `ls -l` |
| --- | --- | --- | --- |
| `/dev/null` | end of file | discards | `crw-rw-rw- 0,   0` |
| `/dev/zero` | endless zero bytes | refused | as above |
| `/dev/random`, `/dev/urandom` | random bytes | refused | as above |
| `/dev/clipboard` | the Windows clipboard as text | sets it; `>>` appends | as above |
| `/dev/stdin`, `/dev/stdout`, `/dev/stderr`, `/dev/fd/N` | this process's own descriptor | same | reported as a device |

`ls -l /dev/null` matches busybox-w32 to the column, including the major and minor
numbers where a size would be. Both are zero and honestly so: these are provided
by the shell rather than by a driver.

`/dev/random` and `/dev/urandom` are the same source. They differ on Linux because
one can block waiting for entropy; Windows has one source that does not block, so
the distinction has nothing to represent.

**`ls /dev` lists, and busybox answers `No such file or directory`.** This is the
one deliberate divergence in the device model, and the reason is discoverability:
without a listing the only way to learn which devices exist is to read this table,
and a shell whose own features are documented rather than visible has hidden them.
`echo /dev/*` expands for the same reason, and `/dev/<TAB>` completes.

`/dev` is read-only -- mode `dr-xr-xr-x` -- because nothing can be created in it.
`/dev/fd` is listed as a name and not enumerated: its contents change with every
redirect, and a listing that depends on how it was invoked is one nobody can rely
on. A name under `/dev` that is not a device -- `/dev/nosuchthing` -- does not
exist, so the namespace has not been made to swallow everything, and nothing lives
under a device: `/dev/null/x` is not a path.

`find /dev`, `du -s /dev` and `grep -r /dev` all work, and `find -type c` selects
the devices. Two notes on the walkers:

- **`grep -r` never reads a device.** `/dev/zero` returns bytes for ever, so a
  recursive grep that read it would not return. GNU grep skips devices when
  recursing for the same reason, and only when recursing: `grep x /dev/clipboard`
  still reads the clipboard.
- **`find /` does not reach `/dev`**, because `/` here is the current drive's root
  -- it resolves to `/c` -- so `/dev` is a sibling top-level name rather than a
  directory inside `/`. On Linux `/dev` is under `/` and a root walk descends into
  it; this is a platform difference rather than a decision, and it is why walking
  the device tree needed no traversal of the real filesystem.

`realpath` answers for a device -- `realpath /dev/../dev/zero` is `/dev/zero` --
because a device has a canonical spelling and canonicalising one is what realpath
is for. A name under `/dev` that is not a device reports `No such file or
directory`.

**`cd /dev` is refused**, and it is the one place this model says no to something a
Linux user can do. A working directory needs a native form, because launching a
child process sets one, and `/dev` has none; a `cd` that succeeded would leave
every external command running in the previous directory while `pwd` said `/dev`.
`/tmp` is the contrast that makes this a rule rather than an inconsistency --
`cd /tmp` works, because `/tmp` has a native mapping behind it. The message gives
that reason rather than saying "not a directory", which would contradict
`test -d /dev`.

A device is not a program: `/dev/null` as a command is refused as not executable,
and a device entry in `PATH` is skipped. A device path passed as an *argument* to
an external program goes through unconverted, which is the argv rule
`docs/design/v0-scope.md` states -- what a Windows program makes of
`/dev/clipboard` is its own business, and converting it would be the MSYS2
behaviour this shell deliberately does not have.

| Applet | Options implemented | Unknown option is |
| --- | --- | --- |
| `base64` | `-d -i -w`; wraps at 76 like GNU, `-w0` not at all | refused by name |
| `ar` | verbs `x p t r`, plus `-o -v`; the long-name table is read, never written | refused by name |
| `ascii` | none; the character table, read down in eight columns | refused by name |
| `base32` | `-d -i -w`; wraps at 76 like `base64` | refused by name |
| `cksum` | none; `<crc> <size> <name>`, the POSIX CRC | refused by name |
| `crc32` | none; eight hex digits, the IEEE CRC | refused by name |
| `basename` | `-a`, and the `basename PATH [SUFFIX]` form | refused by name |
| `bunzip2`, `bzcat` | `-c -d -f -k -t`; **decompress only** | refused by name |
| `cat` | `-n` | refused by name |
| `chmod` | numeric mode | refused by name |
| `clear` | none | refused by name |
| `cmp` | `-s -l`; the message goes to stdout, as GNU's does | refused by name |
| `comm` | `-1 -2 -3` | refused by name |
| `expand` | `-t -i` | refused by name |
| `ftpget` | `-u -p -P -v`; `-c` accepted, resuming is not implemented | refused by name |
| `ftpput` | `-u -p -P -v -c` | refused by name |
| `factor` | none; numbers from operands or stdin | refused by name |
| `fold` | `-w -s -b` | refused by name |
| `free` | `-b -k -m -g`; `-h` accepted | refused by name |
| `cpio` | `-t -i -o -d -m -v -u -0 -F -H`; only `newc` is read or written | refused by name |
| `cp` | `-r`, `-R` | refused by name |
| `cut` | `-b -c -d -f -n -s` | refused by name |
| `date` | `-d -u` | refused by name |
| `diff` | `-u -U -q -s -i -w -B -N -L`; unified always | refused by name |
| `dirname` | none needed | refused by name |
| `dos2unix` | `-u -d`; converts **in place** with a file operand | refused by name |
| `du` | `-s -h`; **apparent** sizes in 1024-byte blocks, not allocation | refused by name |
| `echo` | `-n -e` | treated as text, which is what `echo` does |
| `env` | `-i`, and `NAME=VALUE command` | refused by name |
| `expr` | none; every argument is a term | read as a term, so a bad one is a syntax error |
| `find` | `-name -iname -path -ipath -type f\|d\|l\|c -size -mtime -newer -empty -print -print0 -maxdepth -mindepth`, and the operators `-a -o ! -not -and -or ( )` | refused **before the walk** |
| `grep` | `-i -n -v -r -R -l -L -c -q -w -x -F -o -s -h -H -E -m -A -B -C -e -f`, `--color[=WHEN]` accepted and ignored | refused by name |
| `gzip`, `gunzip`, `zcat` | `-c -d -f -k -t -1`..`-9` | refused by name |
| `hd`, `hexdump` | `-b -c -C -d -o -x -v -A -t` | refused by name |
| `httpd` | `-p -h -a -v`; `-f` accepted, this always runs in the foreground | refused by name |
| `head` | `-n -c -q -v`, the `-N` form, and an attached value (`-n2`) | refused by name |
| `id` | `-u -g -G -n`, and their clusters | refused by name |
| `ln` | `-s` | refused by name |
| `iconv` | `-f -t -l -c -o` | refused by name |
| `join` | `-1 -2 -j -t` | refused by name |
| `ls` | `-a -A -h -l -1 -C -w N -t -S -r -R -d -F`, `--color[=always\|never\|auto]` | refused by name |
| `micro` | `-H -R`; one file at a time | refused by name |
| `mkdir` | `-m -p -v` | refused by name |
| `mktemp` | `-d -q -u`, and an `XXXXXX` template | refused by name |
| `mv` | `-f`, accepted and already in force | refused by name |
| `nano` | `-H -R`; one file at a time | refused by name |
| `nc` | `-l -p -w`; `-e` **refused by name** | refused by name |
| `nl` | `-b t\|a\|n` | refused by name |
| `od` | `-b -c -C -d -o -x -v -A -t` | refused by name |
| `paste` | `-s -d`; the delimiter list cycles | refused by name |
| `pgrep` | `-l -x`, a regular expression on the process name | refused by name |
| `pkill` | `-x` and a leading `-SIG`, a regular expression on the process name | refused by name |
| `patch` | `-i -p -R`; no fuzz | refused by name |
| `posixpath` | none | treated as a path operand |
| `printenv` | none | treated as a variable name |
| `ps` | none; `PID PPID THR RSS TIME COMMAND` | refused by name |
| `top` | `-b -n N -d SEC -s COL -f TEXT -o COLS -H -t` | refused by name |
| `printf` | format string | treated as the format, which is correct |
| `pwd` | `-L -P` both accepted | accepted |
| `readlink` | `-n` | refused by name |
| `rev` | none; reverses runes, not bytes | refused by name |
| `realpath` | none | treated as a path operand |
| `rm` | `-f -r` | refused by name |
| `rmdir` | `-p -v` | refused by name |
| `sha1sum`, `sha256sum`, `sha384sum`, `sha512sum` | `-b -c -t -w` | refused by name |
| `sha3sum` | `-a 224\|256\|384\|512` (default 224), `-b -c -t -w` | refused by name |
| `sum` | `-r` (BSD, the default), `-s` (System V) | refused by name |
| `shuf` | `-n -e -i -z` | refused by name |
| `strings` | `-n -t -o -a -f` | refused by name |
| `sed` | `s/// p d q y = a i c h H g G x n N P D b t T : {}`, addresses (`N`, `$`, `/re/`, ranges, `!`), `-n -e -E -r -f -i[SUFFIX]` | refused by name |
| `seq` | `LAST`, `FIRST LAST`, `FIRST INCREMENT LAST` | read as a number, so a bad one is refused |
| `sleep` | duration operand | reported as an invalid duration |
| `ssl_client` | `-s -h -n`; `-e` accepted; the certificate is always verified | refused by name |
| `sha256sum`, `md5sum` | `-b -c -t -w`; `-c` accepts both the two-space and `*` spellings | refused by name |
| `sort` | `-n -r -u -f -b -k -t` | refused by name |
| `stat` | `-c FORMAT` with `%n %s %F %f %y %Y`; the default output is refused | refused by name |
| `split` | `-l`; two-letter suffixes, `aa` upwards | refused by name |
| `su` | `-c -s -t -W -N`; Windows only, see **Elevation** | refused by name |
| `tac` | none | refused by name |
| `tsort` | none; a cycle is reported rather than truncated | refused by name |
| `tar` | `-c -t -x -v -z -j -a -O -f -C` | refused by name |
| `tail` | `-n -c -q -v`, the `-N` form, and an attached value (`-n2`, `-n+2`) | refused by name |
| `test`, `[` | POSIX expressions | an operand, per the POSIX one-argument rule |
| `tee` | `-a` | refused by name |
| `touch` | `-c` | refused by name |
| `tr` | `-d -s -c`, ranges and backslash escapes; not classes | refused by name |
| `true`, `false` | none, by definition | ignored, which POSIX requires |
| `uname` | `-a -i -m -n -o -p -r -s -v` | refused by name |
| `uniq` | `-c -d -u -i` | refused by name |
| `unexpand` | `-t -a` | refused by name |
| `unix2dos` | `-d -u`; converts **in place** with a file operand | refused by name |
| `unzip` | `-l -t -p -j -n -o -q -K -d -x` | refused by name |
| `uudecode` | `-o`; `-o -` writes to stdout | refused by name |
| `uuencode` | `[FILE] NAME`; `-m` refused | refused by name |
| `wget` | `-O -P -U -T -q -S --header --spider`; `-c` and `-o` accepted | refused by name |
| `wc` | `-c -l -w -m -L` | refused by name |
| `whois` | `-h -p`; `-i` accepted | refused by name |
| `whoami` | none | refused by name |
| `winpath` | none | treated as a path operand |
| `xargs` | `-0 -n -I -r -t` | refused by name |
| `xxd` | `-p` | refused by name |
| `yes` | none | treated as the string to repeat |

The six most recently added -- `tac`, `rev`, `nl`, `base64`, `sha256sum`,
`md5sum` -- were measured against **GNU coreutils**, not busybox: busybox's are
the small versions, and the behaviour people rely on, including the checksum
format printed in every release note, is GNU's. Each carries the observed output
in its test table.

Three of these diverge from GNU on purpose, and say so where it matters:

- **`du` counts apparent sizes**, rounded up to a 1024-byte block, where GNU
  counts what the filesystem allocated. The two differ in both directions: a
  3000-byte file occupies 4096 on NTFS, and a 3-byte one may occupy nothing
  because it fits in the MFT record. Measured on one tree, GNU said 5 and this
  says 6. Go cannot read allocation size portably, and a `du` that silently means
  something slightly different from the one in a script is worse than one that is
  documented to mean apparent size. GNU spells this `--apparent-size`.
- **`stat` implements only `-c FORMAT`.** The default output is inode numbers,
  device ids, permission bits in two notations and three timestamps -- mostly
  fields Windows has not got or reports through a different API, and a block of
  zeroes would be indistinguishable from a real answer.
- **`ps` prints `PID` and `COMMAND` and nothing else.** No TTY, no STAT, no TIME,
  not the command line: Windows has no controlling terminal in the POSIX sense,
  and reading another process's command line means walking its PEB, which an
  ordinary session may not do for anything it does not own. A column of `?` per
  row would be worse than no column.

### Line endings, and which applets keep them

Settled on 2026-08-23, after seven applets were found wrong at once. Every
expectation below was measured against busybox **and** GNU, which agree with each
other.

**The rule is per applet and it has two halves.** An applet whose output is a *copy*
of its input keeps the endings it was given; one whose output is a *new document*
normalises to LF and terminates the last line.

| Keeps the input's endings | Normalises to LF |
| --- | --- |
| `cat` `rev` `head` `tail` `fold` `expand` `unexpand` | `nl` `sort` `uniq` `cut` `grep` |

`tac` is in neither column because it reorders: an unterminated final line becomes
the *first* output line and the terminator lands at the end, so `a\nb` answers
`ba\n`. The endings stay where the endings were rather than travelling with their
lines. Odd, and what busybox does.

**What was wrong.** `sed`, `rev`, `head`, `tail`, `fold`, `expand` and `unexpand`
added a newline to a file whose last line had none, and six of them turned every
CRLF into LF. The root cause was one shared helper: `eachLine` used
`bufio.ScanLines`, which throws the terminator away, so it reported `"\n"` for every
line -- including a final line that had none and a CRLF line whose `\r` had already
been eaten. Its own comment claimed "the final line's ending is not knowable from
Scanner", and that was the mistake: it is knowable, by keeping the terminator in the
token.

**The second half mattered more on this platform than the first.** A Windows-first
shell that rewrites every CRLF file it filters is corrupting the common case --
`head build.log > first.txt` should not change the line endings of the copy.

**Nothing caught it because no fixture in the suite lacked a trailing newline.**
There is now one property test over every line-oriented applet and all three ending
shapes. Its fixtures are written as bytes from Go rather than by a shell, and that
is not fussiness: Git Bash turns `printf 'a\nb' > f` into `a\r\nb`, and measuring
against that fixture produced two wrong conclusions before it was noticed.

**`sed`'s rule is not per line**, which is why it has its own writer. The newline is
omitted only on the very last thing written:

```console
$ printf 'a\nb' | sed p
a
a
b
b      <- no newline here, but the duplicate b above has one
```

The same line is written twice with two different endings, so what distinguishes
them is only that one is last. The terminator therefore belongs to the *write* and
carries which input line produced it -- `sed 2d` on `a\nb` deletes the second line,
so the last output came from the first, which *was* terminated, and both references
answer `a\n`.

One case where the references disagree: on `a\nb`, `sed 2q` gives `a\nb` from busybox
and `a\nb\n` from GNU. **busybox is followed** -- it is the primary reference, and
adding a byte to a file that did not have one is the behaviour this change removes.

And one measured behaviour that is *not* a defect, recorded because it surprises:
**`sed -i` on a CRLF file rewrites it as LF**, six bytes to four. GNU and busybox do
exactly the same. So `sed -i 's/x/y/' notes.txt` changes every line ending in the
file even when nothing matched.

### Text Encodings

Measured against busybox-w32 on the same files, because "supports Unicode" is not
a claim anyone can check.

| encoding | `grep` matches | `wc -m` | copied byte-exact |
| --- | --- | --- | --- |
| UTF-8 | yes | characters | yes |
| UTF-8 with BOM | yes, and the mark is consumed | characters, mark counted | yes |
| UTF-16 LE or BE **with a BOM** | **yes** | bytes | yes |
| UTF-16 LE or BE **without a BOM** | no | bytes | yes |
| GBK, Big5, Shift-JIS, EUC-KR | bytes | bytes | yes |

`wc -m` counts characters where the reference counts bytes -- busybox answers 22
for a file this answers 18 for -- which is a deliberate divergence and the reason
the column is there at all.

Three decisions are worth stating, because each is a refusal as much as a feature.

**A byte-order mark, and no heuristics.** Windows writes UTF-16LE constantly:
Notepad's "Unicode", PowerShell 5.1's `>` redirection, registry exports. All of
them write a BOM, which is the writer declaring what it wrote, and that can be
trusted. Deciding an encoding for a file that declared nothing is how a binary
eventually gets rewritten, so UTF-16 without a BOM stays unread. ripgrep draws
this line in the same place.

**Only the applets that interpret text.** `grep` decodes, because a regular
expression cannot match across UTF-16 code units and finding nothing in a file
full of the word you searched for is the least useful answer available. `cat`,
`head`, `tail`, `base64` and the rest stay byte-exact, because `cat a > b` has to
copy a file rather than reinterpret one.

**What `grep` prints is UTF-8.** A decoded line comes back decoded, so
`grep x u16.txt > out.txt` writes UTF-8 rather than UTF-16. That is the only
answer that does not require every applet to remember what it read, and it is
better said here than discovered.

Still outstanding, and both for the same reason -- they would have to choose an
output encoding for a file they rewrite, which needs deciding rather than
defaulting:

- `sed` over a UTF-16 file matches nothing, so it copies the file through. **That
  sentence was false until 2026-08-23**: it copied the file through and appended a
  byte, because a UTF-16 file's last byte is a NUL rather than a newline and every
  line-oriented filter here terminated its output unconditionally. `sed -i` wrote
  that byte to the file. A 24-byte file came back as 25. It now round-trips
  byte-identical, and a test asserts it.
- `wc -m` counts bytes for UTF-16, because `wc -c` in the same run must count the
  file's real size and one pass cannot honestly do both.

busybox-w32 reads none of these, so this is a feature beyond the reference rather
than a divergence from it.

### Options a script is most likely to reach for and not find

The list used to be `xargs -0`, `xargs -n`, `sort -k`, `grep -r` and `tail -c`.
All five are implemented, measured against GNU. What is still absent:

- **`ls -i -n -u -c`.** `-i` wants an inode number Windows does not keep, `-n` a
  numeric owner this build does not resolve, and `-u`/`-c` the access and change
  times, which NTFS records but which no sort here reads yet. `-t -S -r -R -d -F
  -A` landed on 2026-08-22, so `ls -ltr` works.
- **`tail -f`.** Following a file needs a polling loop and a decision about what
  to do when it is truncated or replaced under you, and an implementation that
  silently stops following is worse than one that says it cannot.
- **`xargs -P`.** Running batches in parallel needs a scheduler this does not
  have, and pretending to accept it would serialise silently.
- **Nothing of `sed`.** This bullet listed `-i`, `-f`, `a i c y` and the hold space
  as absent and was simply stale: all of them landed on 2026-08-22 along with `{}`
  blocks, the multiline commands and branching. Measured against the built binary
  rather than trusted.
- **`find -exec` and `-delete`.** The operators, `-size`, `-mtime`, `-newer`,
  `-empty`, `-maxdepth` and `-print0` landed on 2026-08-22; these two did not,
  because one needs the execution model and quoting rules and the other needs a
  deliberate decision about a destructive default. `| xargs -0` covers most of
  what `-exec` is reached for, and now has `-print0` to pair with.
- **`grep --include`, `--exclude` and `-z`.** The first two need a name filter
  threaded through the `-r` walk; `-z` is a NUL-terminated-line mode. `-A -B -C
  -e -f -L` landed on 2026-08-22.

Every one of them is refused by name, so a script asking for it fails rather than
quietly getting something else.

Two deliberate near-misses worth naming:

- **`grep -E` is accepted and does nothing**, because Go's regexp is RE2 and has
  no basic mode -- what grep here always did was extended. `-G` is *not* accepted,
  because claiming to switch to basic syntax and not doing it would be the lie.
- **`wc -m` counts runes.** GNU said 19 where this says 18 for the same input,
  which is a locale artifact rather than a disagreement: with no locale set a
  character is a byte, and under `LC_ALL=C.UTF-8` GNU says 18 too. Runes are what
  everything else here measures in.

v1.1 shipped on 2026-08-22 and the applet work after it closed the archive,
compression, text and networking groups; see `docs/design/v1-scope.md` and the
per-applet tables in `docs/testing/applet-test-inventory.md` for what is left.

### `find`

**Operators.** `-a`, `-o`, `!`, their long spellings `-and`, `-or`, `-not`, and
parentheses, with POSIX precedence: `-a` binds tighter than `-o`, `!` binds
tighter than both, and adjacency is an implicit `-a`.

Until 2026-08-22 there were none, which made `find` a single-predicate filter
rather than find. `!` was worse than absent: it does not begin with a dash, so
path collection took it as a *path operand* and `find . ! -name x` answered
`find: !: No such file or directory` — blaming a file for an operator, the same
failure shape `stream_options.go` exists to prevent for `cat -n f.txt`. Path
collection now stops at `!`, `(` and `)`.

**Tests.** `-name`, `-iname`, `-path`, `-ipath`, `-type`, `-size`, `-mtime`,
`-newer`, `-empty`. `-name` matches the basename, not the path, because busybox
uses `fnmatch` without `FNM_PATHNAME` and a basename carries no separator for
`*` to cross; `-path` matches the whole path *with* the separator crossable, for
the same reason in reverse — which is why `-name` uses Go's `path.Match` and
`-path` cannot, since `path.Match` hard-codes a non-crossing `*`.
`-type` classifies `f`, `d`, `l`, and `c`; busybox also accepts `b`, `s`, and
`p`, refused by name here rather than answered as though a block device could
never match.

**Actions.** `-print` and `-print0`. An action anywhere suppresses the implicit
`-print`, which is what stops `find . -name x -print` printing twice.

**Global options.** `-maxdepth` and `-mindepth`, which bound the traversal rather
than filter it: `-maxdepth 1` stops the walk from *reading* a subdirectory
instead of reading it and discarding the entries.

Still **refused before the first directory is read**: `-exec`, `-delete`,
`-perm`, `-prune`, `-regex`, `-depth`, `-user`, `-group`, and the rest.

```console
$ find . -perm 644
find: unsupported expression: -perm
$ echo $?
1
```

That ordering is the fix, not a detail. Until 2026-08-07 `find` honoured no
expression at all: it walked the whole tree, printed every path, and only then
reported the predicate as a missing file. `find . -name '*.tmp' | xargs rm`
therefore received every file.

Output follows POSIX rather than being cleaned: the path operand is written
exactly as given, then a slash, then the rest. `find .` yields `./a.txt`, not
`a.txt`.

#### Two deliberate divergences from busybox-w32

**`-size` divides and rounds up; busybox compares raw bytes.** POSIX states it
outright — the size "divided by 512 and rounded up to the next integer" — and GNU
applies the same rounding to every unit suffix. busybox-w32 compares
`st_size` against `N * unit` instead. Measured 2026-08-22 in a tree holding files
of 0, 1, 100 and 3000 bytes:

| | busybox-w32 | nemosh (POSIX/GNU) |
| --- | --- | --- |
| `-size 1c` | the 1-byte file | the 1-byte file |
| `-size 1k` | **nothing** | the 1-byte and 100-byte files |
| `-size +1` | the 3000-byte file | the 3000-byte file |

busybox's reading makes an exact-match `-size` with a unit suffix nearly
unusable, since it demands a file of exactly 1024 bytes. `+` and `-`
comparisons agree either way, which is most real use.

**`-newer` keeps NTFS's full timestamp precision; busybox truncates to whole
seconds.** Measured 2026-08-22: files created within one second of each other
are all "not newer" under busybox, while nemosh orders them. Given a file
clearly older, the two agree exactly. GNU find also compares at full precision.

### `diff` and `patch`

Added 2026-08-23, and kept as a pair: shipping `patch` against a `diff` whose
output shape later changed would break it silently, so the tests round-trip the
two against each other rather than testing each alone. They also interoperate in
both directions with busybox's own `diff` and `patch`.

`diff` completes a family that was already half here -- `cmp` compares bytes,
`comm` compares sorted lines -- and is absent from stock Windows entirely.

- **The output is unified always**, which is busybox's default and *not* GNU's:
  GNU prints the older "normal" format unless asked. 8 of 8 measured forms agree
  with busybox byte for byte.
- **No timestamps in the header**, again following busybox, so two runs over
  unchanged files produce identical output.
- Common prefix and suffix are stripped before the
  longest-common-subsequence table is built. That is not tidiness: the table is
  O(n*m), and two versions of a real file usually differ in a few lines out of
  thousands, so without it a large pair costs gigabytes.
- `-i`, `-w` and `-B` change what "the same line" *means* rather than how the diff
  is computed.

`patch` **refuses a hunk that does not match, naming the hunk, the line, and the
text it found instead.** There is deliberately no fuzz: shifting a hunk up and
down until the context happens to line up is how a patch lands somewhere it was
never meant to, and the wrong place usually still compiles. A failed patch leaves
the file untouched.

The names in a diff come from whoever wrote it, so `patch` checks them with the
same containment helper the archivers use -- `--- ../../etc/passwd` is the same
attack a tar entry would be.

One consequence worth naming: **adding `diff` shadows the system's.** A test of
process substitution had been asserting GNU's normal-format output, because until
now `diff <(…) <(…)` ran whatever was on PATH. Shadowing is what a busybox-style
bundle is for, but it does change what an existing script sees.

### The dump formats, and the uu pair

`od`, `hexdump`, `hd`, `uuencode` and `uudecode`, added 2026-08-23. `xxd` already
existed, so these shapes were pinned by differential rather than invented: 21 of
21 measured forms agree with busybox byte for byte, over "hello", an
84-character line and 300 random bytes.

Three defaults distinguish the three dumpers, which is the whole reason all three
exist: `od` uses octal offsets and octal words, `hexdump` hex offsets and hex
words, and `hd` is `hexdump -C` -- hex bytes with an ASCII gutter.

Two details are worth stating:

- **The word forms read each byte pair little-endian.** `he` is `0x68 0x65` and
  prints as `6568`, not `6865`. It is the most surprising thing about either tool.
- **`hexdump` pads a short line to eight slots and `od` does not.** Trimming
  trailing whitespace is the obvious tidy-up and it silently broke `hexdump` while
  leaving `od` correct.

`uuencode` and `uudecode` carry the pre-base64 wire format. The lone backtick that
ends the body is a zero-length line spelled with a backtick rather than a space,
because trailing spaces do not survive mail. **The name in the header came from
the sender, so `uudecode` checks it with the same containment helper the archivers
use** -- `begin 644 ../../evil` is the same attack a tar entry would be, and
`-o -` is how the bytes go to stdout instead of to whatever file the sender chose.

### The editor: `nano` and `micro`

One editor under two names, added 2026-08-23. The key map is chosen by the name
it was invoked as -- how busybox varies behaviour by `argv[0]` -- because calling
something `nano` and then binding `^S` to save would be a name that lies.

| | save | quit | search | cut | paste | go to line |
| --- | --- | --- | --- | --- | --- | --- |
| `nano` | `^O` | `^X` | `^W` | `^K` | `^U` | `^_` |
| `micro` | `^S` | `^Q` | `^F` | `^K` | `^V` | `^L` |

**`-H` lists exactly what is implemented, and what is not**, in the manner of
`busybox vi -H`. The list is generated from the binding table, so it cannot claim
a key the editor does not bind -- a test walks both to check. It also names the
absences (no syntax highlighting, no multiple buffers, no replace, no mouse, no
configuration file), because an editor that silently lacks replace is worse than
one that says so.

**Writing it here was not a compromise.** micro's editing core lives entirely
under `internal/`, which Go forbids importing across modules -- only
`pkg/highlight` is reachable -- and it depends on a *fork* of tcell where this
build uses upstream, and two tcells driving one Windows console is the conflict
`top_view.go:46` already documents. busybox does the same thing with `vi`: its
header reads *"tiny vi.c: A small 'vi' clone"*, about 3000 lines from scratch,
keeping the name and the key language. `nano` itself is a clone of `pico` for the
same kind of reason.

The buffer is tview's `TextArea`, which already handles a cursor, selection,
double-width characters and undo. Two things came out of using it:

- **Its offsets are byte positions, not runes.** `GetTextLength` answers 10 for
  two CJK characters plus `ab` and two newlines. Counting runes put the cursor in
  the wrong place on any line holding a multibyte character.
- `Replace` is used rather than `SetText` for the line commands, because
  `SetText` discards the undo history.

Other behaviours worth knowing:

- **Bytes are saved as they arrived.** The editor does not decode, so it cannot
  re-encode, and a UTF-16 or Latin-1 file keeps its encoding -- the same rule
  `sed -i` follows.
- **`^X`/`^Q` with unsaved changes warns once and leaves on the second press.** A
  yes/no prompt needs a reader this does not have, and losing a buffer to one
  keystroke is the outcome worth preventing.
- **A terminal is required, and merely having a file on stdin is not enough.**
  `nano file < /dev/null` leased successfully and then hung waiting for keys that
  would never arrive; the check is now whether stdin is a terminal.
- A file that does not exist opens as a new buffer rather than failing. More than
  one file is refused, since there are no buffers to put the second in.

The interactive path is tested headlessly over tcell's simulation screen, typing
keys and asserting the file on disk. That needs polling rather than assuming
synchrony: an injected key and a `QueueUpdate` callback share one select loop with
no ordering between them, so a read taken straight after a key press can see the
frame *before* it was handled.

**And polling for the right thing.** A second version of that trap cost a real
finding: a test of `^G` waited for a key label, matched it in frame zero because the
*footer* already shows every label, and then asserted against a frame that predated
the help message. Waiting for a string that was already true is not waiting. It now
waits on `Keys:`, which only the help line produces.

**The prompt is a second input mode, and that is where the tests concentrate.**
While one is open every key means something different, so search, go-to-line and
help are asserted for the thing that would be silently wrong: that the typed term
goes into the prompt and *not into the file*, that Escape restores editing and the
abandoned term never reaches disk, that Backspace edits the prompt rather than the
document behind it, and that an action key -- `^X`, which quits -- is swallowed
rather than fired.

**One defect came out of it, in line counting.**
`strings.Split("a
b
", "
")` answers three elements, because a newline is a
terminator and Split treats it as a separator. So every file ending in a newline --
which is every well-formed text file -- had one line more than it has, and
`^_ 9999` on a sixty-line file answered "Line 61". One helper now does the splitting
for both the line count and the cursor's byte arithmetic; an empty buffer stays one
empty line rather than zero, because zero would divide by the line count in the
search wrap.

### The archivers, and where an entry may land

`tar` and `unzip`, added 2026-08-22. Interoperability verified in both directions
against busybox, Windows' own `tar.exe`, and PowerShell's `Compress-Archive`.

**An archive is untrusted input that names its own destinations**, so extraction
checks every entry before creating anything, through one helper the archivers
share. `applet-test-inventory.md` names "path traversal safety" as this group's
test focus; the Windows hazard list is longer than the Unix one. An entry is
refused when it is:

| hazard | why it matters here |
| --- | --- |
| `../escape`, `a/b/../../../escape` | checked *after* cleaning, so a prefix test cannot be fooled |
| `/absolute`, `C:\x`, `C:/x` | absolute or drive-qualified |
| `C:relative` | drive-*relative*: resolved against that drive's own current directory |
| `NUL`, `nul.txt`, `sub/CON`, `COM1.tar.gz` | Windows resolves these in **every** directory, so extracting one writes to the device and silently loses the data |
| `evil.`, `evil ` | Windows strips a trailing dot or space, so these collide with `evil` |
| `FOO` beside `foo` | NTFS is case-insensitive, so the second silently overwrites the first |
| a link whose target leaves the root | a later entry writing *through* the link escapes |

Each hazard has a test with a hand-built archive, and each asserts two things: the
entry was refused **and nothing was written outside the root**. The second is the
one that matters -- a refusal reported after the file was created is no refusal. A
hostile entry is skipped rather than aborting, so one bad name does not cost the
honest ones.

**Backslashes are normalised, not refused**, and that is an interoperability
decision with a measurement behind it: PowerShell's `Compress-Archive` writes
backslash-separated names -- `src\a.txt` -- against the zip specification, which
mandates `/`. Refusing them made every PowerShell-made zip completely
unextractable, and PowerShell is the most likely producer of a zip on this
platform. Normalising is not a weakening, because `a\..\..\evil` becomes
`a/../../evil` and the escape check still catches it; what it costs is that a Unix
file *literally* named `a\b` becomes `a/b`, a rare misreading with no security
consequence and the same thing every Windows unzip does.

**Listing does not check**, deliberately: `tar -t` is how somebody inspects an
archive they do not trust, so hiding the hostile entry would defeat the purpose.

`tar` reuses this build's own gzip, so `tar -czf` needs no second program.
`unzip` requires a file operand and says why: zip keeps its central directory at
the *end* of the file, so it cannot be read from a pipe. With neither `-o` nor
`-n`, an existing file is left alone and said so -- busybox prompts, and there is
no prompt here.

### `cpio` and `ar`, the two with no library behind them

Added 2026-08-23. Go has no package for either format, so both headers are read
and written byte by byte here -- which is why the tests are unusually specific
about columns and padding.

**Both go through the same containment helper `tar` and `unzip` use**, and both are
run against the whole hostile-name table for exactly that reason: a shared helper
is only shared if every caller reaches it.

- **`cpio` archives a *list*, not a tree.** That is the whole reason it still
  exists: `find . -name '*.go' | cpio -o -H newc > src.cpio` takes exactly the
  names on stdin, where `tar` would descend them. It is the pair to `find`, and
  `-0` reads the NUL-separated form `find -print0` writes -- the same splitter
  `xargs -0` uses, because it means the same thing.
- **Only `newc` and its CRC variant are read.** The magic is checked on its own
  *before* the rest of the header, so an `odc` (76-byte) or old-binary (26-byte)
  archive is named rather than reported as truncated -- the first version asked for
  110 bytes first and answered `unexpected EOF` for every small archive in either.
- **A missing trailer is reported.** cpio ends with an explicit `TRAILER!!!` entry,
  so truncation is knowable rather than a guess.
- **`ar`'s operation is a verb**, not an option: `ar t lib.a`. Only the *first word*
  is examined, and that is a fix rather than a preference -- scanning every argument
  meant a Windows temporary path, which contains a `p` in `AppData`, was read as the
  verb, had a letter removed from its middle, and came back to the option parser as
  `-C:\Users\...`. Every letter of that first word must also be one `ar` knows, which
  is what makes `ar libtest.a` refuse on its `l` the way GNU does instead of finding
  the `t` in the file name.
- **`ar r` creates and refuses an archive that already exists.** Adding to one means
  reading every existing header, deciding which member each operand replaces, and
  rewriting the long-name table; a half-done version of that silently corrupts the
  archive it was handed. The long-name table *is* read, because a `.deb` or a
  GNU-made library has one.
- **A name too long for the sixteen-column header is refused, not truncated**, and
  the half-written archive is deleted: a truncated name is a different file written
  silently.
- **A member whose name cannot be resolved is skipped, not fatal.** A stored
  `/absolute.txt` arrives here as an unreadable long-name *offset* rather than a
  path, and aborting on it would cost every honest member after it. The header gave
  the size, so the stream stays aligned across the skip.
- Two fields are synthesised rather than invented: **uid and gid are written as
  zero**, because Windows has no numeric owner and busybox-w32's own answer of 4095
  is not a user either; and the **mode is 0644, 0755 or 0444**, because Windows has
  no execute bit and writing Go's `os.FileMode` bits into a Unix mode field would be
  a misreport rather than a translation.

Interoperability verified in both directions against busybox-w32 v1.38.0: `cpio -t`
and `-tv` match byte for byte including the `N blocks` line, each reads the other's
archives, and a nemosh-made archive is the same 388 bytes as busybox's for the same
input -- differing only in the inode serial, the owner, and the mode, all three
documented above. `ar t`, `ar tv` and `ar p` are byte-identical.

### The compression filters

`gzip`, `gunzip`, `zcat`, `bunzip2` and `bzcat`, added 2026-08-22. Round trips
verified in both directions against busybox-made archives.

**These are stream filters, and that is why `tar.exe` does not cover them.**
Stock Windows ships `tar.exe` and `curl.exe` and neither `gzip` nor `unzip`
(measured). `... | gzip > x.gz` and `zcat log.gz | grep` are pipelines; bsdtar
cannot stand in for either.

- **A file operand is replaced and the original removed**, unless `-k` or `-c`.
  That is both references' default and the behaviour that surprises people.
- **Removing the original is where Windows differs from Unix**: a file cannot be
  deleted while a handle to it is open. The first draft here held the source open
  across the remove and failed with *"The process cannot access the file because
  it is being used by another process"* — and would have passed silently on Linux,
  where the unlink succeeds regardless. The source is now closed first.
- An existing companion is not overwritten without `-f`, so a second run cannot
  silently destroy an archive; and a half-written companion is deleted on failure,
  so a truncated archive never looks like a real one.
- `.tgz` and `.tbz` stand for `.tar.gz` and `.tar.bz2`, so decompressing one
  restores the `.tar` rather than losing the extension.

**`bzip2` compression is absent and the name is not registered.** The standard
library decompresses bzip2 but cannot compress it. Leaving the name unregistered
rather than refusing it means PATH lookup still finds a real `bzip2.exe` if the
machine has one — more useful than a refusal, and the same reasoning applies to
`xz`, `lzma`, `lzop` and the Linux package formats, none of which are provided.

Two divergences from busybox that writing these tests found, both in `tar` and both
cases of doing something quietly instead of refusing:

- **`tar -c -x` chose an operation instead of refusing.** A switch on the three
  letters in order meant `tar -c -x -f a.tar src` created the archive and ignored
  the `-x`, and somebody who typed both meant one of them and got the other half the
  time. busybox refuses the same invocation by printing its usage. Exactly one of
  `-c -t -x` is now required, which is also what this applet's own sibling `cpio`
  already did for `-t -i -o`.
- **`-C DIR` created the directory instead of requiring it.** The directory appeared
  as a side effect of writing the first entry into it, so `tar -xf a.tar -C /tpm`
  made a new directory rather than reporting the misspelling. The option is spelled
  "change to this directory", and changing to one that is not there is an error;
  busybox says `can't change directory to 'nope'` and GNU agrees.

**`bunzip2` and `bzcat` had no test at all until 2026-08-23** -- registered,
documented here, and run by nothing, with `tar -j` untested alongside them. They
worked when finally tried by hand, which is the bad kind of luck: a silent
regression had nowhere to be caught. Their fixtures have to be *literals*, because
Go has no bzip2 writer -- the same fact that keeps the `bzip2` name unregistered --
so the input cannot come from the code under test and comes from busybox instead.

One divergence where the reference is broken: **busybox's `zcat` cannot read a
pipe.** `cat x.gz | busybox zcat` answers `lseek(...): Invalid seek` while
`busybox zcat < x.gz` works — it seeks on its input, which a redirect allows and a
pipe does not. This reads sequentially, so both work.

### The network clients

`wget`, `nc`, `whois`, `ssl_client`, `httpd`, `ftpget` and `ftpput`, added
2026-08-23. Every test runs against a server started by the test: a test that
needs a name resolved is a test that fails on a train.

**busybox-w32 keeps only these seven**, out of the dozens busybox has, because
Windows lacks the APIs for the rest — so this is the whole networking group rather
than a selection from it.

- **TLS is native, so there is no `ssl_client` helper in the pipeline.** busybox's
  `wget` cannot speak TLS and shells out to `ssl_client` for it; Go's `net/http`
  can. `ssl_client` is still here, for what the name actually means — `openssl
  s_client` on a machine with no openssl — and `wget` does not use it.
- **A name that came from the server is checked like an archive entry.** `wget`
  takes its output name from the URL's last path element, and a redirect chooses
  that; so it goes through `safeArchivePath`, the same helper `tar` and `unzip`
  use. `httpd` puts every request path through it too. A URL is untrusted input
  naming a destination, which is exactly what a tar entry is.
- **A 4xx or 5xx is a failed download**, not a file whose contents are the error
  page. Writing the body would leave something that looks like a success.
- **`--spider` is a HEAD**, not a GET with the body discarded — the caller said not
  to download it.
- **`nc -e` is refused by name.** Running a program on a connection is a remote
  shell, and this build does not offer one.
- **`ssl_client` always verifies the certificate**, with no option to skip: a TLS
  pipe that does not check the certificate is a plaintext pipe wearing a costume.
- **FTP is passive-only.** Active mode asks the server to connect *back*, which no
  machine behind a router or a Windows firewall can accept, so offering it would be
  offering something that mostly fails.
- **The two FTP argument orders are mirrors, and getting that wrong was invisible.**
  `ftpget HOST [LOCAL] REMOTE` against `ftpput HOST [REMOTE] LOCAL` -- in both, the
  file being *written* is named first. `ftpput` had them reversed, so
  `ftpput host remote.txt local.txt` read `remote.txt` from disk and uploaded it as
  `local.txt`. With one file operand the two names collapse to the same string, and
  every test that existed passed one operand or none, so nothing caught it until the
  applets were run against an FTP server the tests start.
- **`httpd` binds 127.0.0.1 unless `-a` says otherwise**, where busybox binds every
  interface, and it runs **no CGI**. Reaching the network is a decision; CGI turns a
  file server into an execution service. Both defaults are asserted by tests.
- **`nc` waits for the reply, and getting that wrong was invisible.** Copying both
  ways and returning on whichever direction finished first looks symmetric and is
  not: for a request and a response, which is most of what `nc` is for, the *write*
  side always finishes first, so `nc` exited before reading a byte. Go's `select`
  chooses uniformly among ready cases, so the passing test was a coin flip per run
  -- it took a manual `nc 127.0.0.1 8231` against this build's own `httpd` to see
  the empty output. The read side is now what ends the session, and the test's
  server reads to EOF before replying, so the old behaviour cannot pass by luck.
- Two more found in the writing, both Go-shaped rather than protocol-shaped:
  `<-ctx.Done()` in a goroutine **leaks that goroutine forever** when the context is
  never cancelled, because an uncancellable context's `Done()` is a nil channel and
  a receive from one never returns (`context.AfterFunc` now); and `httpd` served a
  directory's `index.html` **twice**, once through `http.ServeFile` with a
  fabricated request and once by writing the bytes.

**Three of these had no end-to-end test until 2026-08-23**, and saying so is more
useful than the fix: `ftpget` and `ftpput` had only a unit test on the passive-mode
reply parser, `whois` only on its server table, and `ssl_client` none at all. All
three now run against a server the test starts -- including a written-out FTP server
that answers USER, PASS, TYPE, PASV, RETR and STOR, and whose greeting is
deliberately multi-line so a client that reads only the first line of a reply goes
out of step and fails. `ssl_client` is asserted in the only direction its own design
allows: a Go test server's certificate is signed by a CA no trust store knows, so a
*successful* handshake would need the verification-skipping option this build
refuses to have, and the refusal is what gets tested.

**The bundled `completions/wget.toml` was removed in the same change.** It
described GNU wget, and its own comment said the point of the file was that one
name can be two programs. A nemosh `wget` makes it three, and `internal/capability`
— which a test binds to behaviour by running the applet — now answers for the name,
so keeping a second unverified description of it was the one thing the completions
rule forbids. `NEMOSH_OVERRIDE_APPLETS=wget` still reaches a real `wget.exe`;
`scripts/completions/generate.py wget` writes a spec for the one installed.

### The encoding tools, and the policy they settle

`dos2unix`, `unix2dos` and `iconv`, added 2026-08-22. All seven measured forms
agree with busybox byte for byte.

**`iconv` settles a question two other applets were waiting on.** `sed -i` and
`wc -m` were both recorded as outstanding on UTF-16 input, and the blocker was
the same for each: neither had a policy for which encoding to *write*. `iconv` is
the tool whose entire job is that choice, so the policy lives there and the others
can follow it:

- **An encoding is named, never guessed.** `-f` and `-t` are explicit and there is
  no detection step — the same rule `grep` follows in honouring only a byte-order
  mark and never sniffing. Guessing is how a binary gets rewritten.
- **Both default to UTF-8**, so a bare `iconv file` is a no-op rather than a
  surprise.
- **No byte-order mark is written** unless the named encoding is one of the
  explicit BOM forms. An uninvited BOM breaks `#!` lines and CSV headers.
- **A character the target cannot represent is an error**, not a silent
  substitution. `-c` is how a caller asks for the lossy behaviour on purpose.

`iconv -l` lists only encodings this build can actually construct, and a test
converts to every name it prints — a name that appeared and then failed would be
worse than one that never appeared.

`dos2unix` and `unix2dos` are one implementation, the direction chosen by the name
and overridable with `-u`/`-d`. Two behaviours are worth knowing:

- **A file operand is converted in place** and nothing goes to stdout. That is
  busybox's default and it is the trap in this applet.
- **A lone carriage return survives.** Only the CR of a CRLF is removed, because
  on an old Mac file a bare CR is the line ending itself and dropping it would
  join every line into one. Binary data with a stray CR is likewise untouched.
- **Running `unix2dos` twice is safe.** The input is normalised to LF first, so no
  CR is ever doubled — a bare LF-to-CRLF replacement produces CRCRLF on its second
  run.
- An unchanged file is not rewritten, so its modification time survives and a
  build system is not told it changed.

### The small text tools, and `free`

Eleven applets added on 2026-08-22: `free`, `factor`, `fold`, `tsort`, `strings`,
`ascii`, `expand`, `unexpand`, `join`, `base32`, `shuf`. **30 of 31 measured forms
agree with busybox-w32 byte for byte**; the exception is a stray operand, where
busybox silently ignores `ascii extra` and this refuses it — the same laxness
already diverged from for `find )`.

Four things here are not guessable:

- **`free`'s Swap row is Windows' commit charge**, not a swap file. There is no
  swap partition to measure, and commit is the number that says whether the next
  allocation will be refused. The `shared` column is always zero because Windows
  has no equivalent counter, and inventing one from working-set overlap would be a
  guess presented as a measurement. `free` reads the same sampler `top` draws its
  meters from, so the two cannot disagree about the machine's memory — and it
  refuses off Windows with `ErrListUnsupported` rather than reporting zeros.
- **`tsort`'s order among independent items is unspecified, and all three
  implementations differ.** For `a b / b c / d e`, busybox answers `a b d e c`,
  GNU answers `a d b e c`, and this answers `a b c d e`. All three satisfy the
  constraints. The input-stable order is chosen because it is the only
  reproducible one: two runs over one file agree, so a diff between them means
  something.
- **`join -1` and `-2` are per-file fields.** `join -1 2 -2 1` joins the second
  field of the first file to the first of the second. Collapsing them into one
  number silently answers nothing for every asymmetric join, which is what the
  first draft here did. `-j` sets both; it is GNU's and busybox does not have it,
  offered because refusing a standard option is the worse divergence.
- **`ascii`'s column spacing is busybox's hand-tuned layout** — the gaps are 11,
  11, 9, 9, 9, 10, 10 characters, so it cannot come from one format string. The
  table is read *down*: the first column is 0–15, not 0,1,2 across. Reading it
  across is the obvious implementation and gives a completely different table.

`expand`, `unexpand` and `fold` all count **runes**, so a tab after CJK text lands
where it looks like it should and a wrapped line is never cut through a character.
`shuf` is the one applet whose output is deliberately not reproducible, so its
tests assert the multiset and the count rather than an order.

### The checksum family

`md5sum` and `sha256sum` were the only two. busybox-w32 also has `sha1sum`,
`sha384sum`, `sha512sum`, `sha3sum`, `cksum`, `crc32` and `sum`, and a clean
Windows machine has none of them — which is most of why anyone reaches for a
checksum tool at all. All seven landed on 2026-08-22. 58 of 58 measured forms
agree with busybox byte for byte, over `hello
`, an empty file, a single byte,
1500 zero bytes and 100 KB of random data.

Three of them are easy to get plausibly wrong, so each is stated:

- **`sha3sum` defaults to 224 bits**, not 512. Measured. And SHA-3 is not SHA-2
  truncated — SHA3-224 and SHA-224 are different functions over different
  permutations — so it cannot share a constructor with the others. All four
  widths were cross-checked against Go's `crypto/sha3` as well as against
  busybox, because a wrong constant yields a digest that looks fine and is
  useless.
- **`cksum` is not `crc32`.** POSIX's CRC walks a different polynomial
  most-significant-bit first, feeds the file *length* through the register
  afterwards, and complements the result. It cannot come from Go's `hash/crc32`,
  whose tables are all reflected: handing that package cksum's polynomial gives a
  plausible number that is not cksum. The clearest single check is an empty file,
  which answers 4294967295 — the complement of an untouched register.
- **`sum` has two incompatible algorithms.** BSD (`-r`, the default) rotates the
  accumulator right before adding each byte and counts 1024-byte blocks; System V
  (`-s`) folds a plain byte total twice into sixteen bits and counts 512-byte
  blocks. The fold is done twice because the first can itself carry out of sixteen
  bits, which is a real difference above about a megabyte.

One deliberate divergence: **`sum` omits the file name for a single operand and
prints no trailing space**, which is GNU's output. busybox prints its format's
trailing space with an empty name — `36979     1 ` — a stray byte rather than a
behaviour. With more than one operand all three agree.

### A lone `-` is standard input

POSIX gives `-` that meaning for every utility taking file operands, and it is
how a script mixes a stream into a list of files:

```console
$ cat header.txt - footer.txt
```

**Eleven applets answered `No such file or directory` for it** until 2026-08-22 —
`cat`, `head`, `tail`, `wc`, `grep`, `sed`, `sort`, `nl`, `rev`, `base64` and the
checksums — while `cut`, `uniq`, `paste` and `comm` each carried their own
`operand == "-"` check. Four private answers and eleven omissions is what one
shared seam is for; it is `OpenProcessOperand`.

Closing the operand does not close the shell's stdin, so `cat - -` finds the
stream empty rather than closed. The wrapper forwards `ReadContext` rather than
using `io.NopCloser`, because a `NopCloser` would hide the cancellation the shell
puts there and `cat -` alone would stop being interruptible — the same
wrapper-hides-capability bug this package has had three times.

`head` and `tail` also **carry on past an unreadable operand** now, reporting it
and leaving status 1 behind, where they used to stop: `head -n1 a.txt nosuch b.txt`
silently dropped `b.txt`. The `==> name <==` header is written after the open
succeeds, so a file that could not be read no longer gets a header above the
error saying it is missing.

**The header rule is POSIX's**, taken from `head`, which specifies the shape
exactly:

> `"\n==> %s <==\n", <pathname>` — "except that the first header written shall
> not include the initial &lt;newline&gt;", and only "when more than one *file
> operand* is specified".

So the blank line belongs to the *following* header rather than trailing the
previous block, which is why there is none after the last file; and the rule keys
off how many operands were named, not how many opened. POSIX's `tail` takes a
single file operand and says nothing about headers, so the multi-file form is an
extension and follows `head`.

That leaves two places where busybox's own `head` and `tail` disagree with **each
other**, and this follows the consistent answer — which is GNU's, and busybox
`head`'s — in both:

| | busybox `head` | busybox `tail` | GNU, and here |
| --- | --- | --- | --- |
| header when one of two operands is unreadable | prints it | **prints none** | prints it |
| a `-` operand in a header | `standard input` | **`-`** | `standard input` |

`head` and `tail` disagreeing with each other is worse than one of them
disagreeing with a reference, and a bare `-` in a header would read as a file of
that name.

### `sed`

**Addresses**, which is what turned `sed` from an `s///` filter into sed:

```console
$ sed -n '2,4p' s.txt      # a line range
$ sed '/x/d'               # delete every matching line
$ sed -n '$p'              # the last line
$ sed '2,$d'               # line two to the end
$ sed -n '/a/,/b/p'        # from a match on a to the next on b
$ sed -n '2!p'             # every line except two
```

Commands: `s///`, `p`, `d`, `q`, `y///`, `=`, `a`, `i`, `c`, the hold-space five
`h H g G x`, the multiline `n N P D`, the branches `b t T` with `:` labels, and
`{}` blocks, separated by `;` or a newline. Options: `-n`, `-e` (repeatable), `-E`/`-r`, `-f FILE` and
`-i[SUFFIX]`. `s///` takes `g`, a repeat count, and `i`/`I` for
case-insensitive matching. An **empty script is a valid no-op** rather than an
error, so `sed "$expr" file` still copies the file when the variable is empty. Before 2026-08-22 none of that existed — `sed -n`
was refused as an unsupported *script*, since the first argument was assumed to
be one.

Three things are worth knowing because they are not guessable:

- **Several file operands are ONE stream.** Line numbers run on across the
  boundary and `$` is the last line of the last file: `sed -n '3p' f1 f2` answers
  the third line overall. Measured. This is why the old per-file loop had to go —
  it did not matter while sed had no addresses and would have been silently wrong
  the moment it had.
- **`$` needs one line of lookahead**, not the whole input. sed is a filter, and
  reading a log into memory to find out where it ends is not what a filter does.
- **`p` without `-n` prints twice**, because the pattern space is also written at
  the end of the script. That is the reference behaviour and the reason `-n`
  exists.
- **A range whose closing address never matches runs to the end**, and one whose
  numeric end has already passed is a single line: `sed -n '$,1p'` answers the
  last line.

Diagnostics match busybox to the character: `no address after comma`,
`unmatched '/'`, `unsupported command ,`. 30 of 30 measured forms agree.

Address `0` is refused with `invalid usage of line address 0`, which is GNU's
answer: line numbering starts at 1. busybox instead lets `0` parse as *no*
address, so its `sed -n '0p'` prints every line — measured, and a quirk rather
than a rule worth copying.

**`{}` blocks** group commands under one address, which is what makes
`sed -n '/x/{p;q}'` apply both to the matching line and neither to any other. `d`
and `q` inside a block end the whole cycle rather than just the block.

**`y///`** transliterates, by rune rather than by byte — `y/áé/ae/` works, where
indexing bytes would replace half a character. Unequal lengths are **refused**:
busybox transliterates the pairs it has and silently ignores the rest, so its
`y/abc/xy/` leaves every `c` alone, which is a wrong answer with no diagnostic.
GNU refuses it and so does this.

**`-i`** edits in place, with `-i.bak` keeping the original. Each file is its own
stream: line numbers restart, `$` is that file's last line, and an address range
does not leak into the next file — GNU's behaviour, and the coherent reading of
what `-i` means. busybox restarts the numbering but leaves an open range running
across the boundary, so its `sed -i -n '2,3p' a b` keeps `b`'s first line because
`a`'s range never closed. The whole result is built before anything is written,
so a script that fails halfway leaves the file as it was.

`-i` had been deferred on the grounds that rewriting a file forces a choice of
**output** encoding. It does not: sed here is byte-exact, so the bytes written
back are the bytes read, transformed. That question only arrives if sed starts
*decoding* UTF-16 on input, and it is still deferred until then.

**`a`, `i` and `c`** append after the line, insert before it, and replace it.
Their argument is the one thing in sed that is not delimited, so it has its own
reader: the text runs to the end of the line or the end of the `-e` fragment, and
a `;` inside it is text — `sed '1a\x;p'` appends the literal `x;p`, where every
other command would have taken `p` as the next one. A leading backslash protects
the text's own leading whitespace; without one, blanks are separators. `\n` and
`\t` are interpreted, which is how a multi-line insert fits in one argument.

Three of their rules are not guessable, and each is pinned:

- **`-n` does not suppress them.** The text belongs to the script rather than to
  the line, so `sed -n '1a\text'` prints `text` and nothing else.
- **`a` survives a `d`.** The text belongs *after* the line whether or not the
  line is printed, so it is queued and flushed at the end of the cycle.
- **`c` on a range prints once**, as the range closes, not once per line:
  `sed '1,2c\once'` answers a single `once`.

**The hold space and branching** are what make sed more than a line filter, and
they are tested through the one-liners they exist for — each measured against
busybox:

```console
$ sed -n '1!G;h;$p' file          # reverse it, which is tac
$ sed ':a;N;$!ba;s/\n/ /g' file   # join every line
$ sed -n 'N;P;D' file             # a sliding two-line window
$ sed -n 'H;${x;s/\n/,/g;p}'      # collect into one comma-separated line
```

Branching forced a restructure worth naming: **the command tree is flattened into
one instruction list**, because a label can sit inside a block that a jump comes
from outside, and a recursive walk over a tree gives such a jump nowhere to land.
That is what sed itself does, and it is why `:a;N;$!ba` works. `D` uses the same
machinery — it restarts the script *without* reading a line, which is a jump to
instruction zero.

Two details that are easy to get wrong, both pinned:

- **`N` at end of input still prints the pattern space**, so
  `sed 'N;s/\n/ /'` over three lines answers `a b` and then a bare `c`. It is
  printed by `N` rather than left to the end-of-cycle print, because ending the
  run has to skip that print for `q`'s sake.
- **`N` advances the line counter**, so `$` still names the real last line. A
  consumed line that was not counted would make `$` name the wrong one.

Still refused: `l`, the `w`/`r` file commands, `e`, and GNU's `first~step`
addresses. `l` needs an unambiguous-print escaping table and the file commands
need a second decision about where output goes.

### `grep`

**Context lines** — `-A N`, `-B N`, `-C N` — with `--` between groups that are
not contiguous. `-A` is straightforward; `-B` is the reason it needs state, since
whether a line is context is not known when it is read but only once a *later*
line matches. A line already printed as trailing context never enters the
holding ring, which is what stops an overlapping group printing anything twice.

A match and a context line are told apart by their separator, for both prefixes:
`g.txt:2:M1` against `g.txt-3-l3`.

Two rules were measured rather than chosen:

- **`-A0` prints no separator at all**, even between groups several lines apart.
  GNU does print one there; busybox does not, and busybox is the reference.
- **The separator spans files.** A `--` belongs between the last group of one
  file and the first of the next, so the printer's state has to outlive a single
  file.
- **`-c`, `-l`, `-L` and `-q` ignore context entirely** — `grep -c -A1` counts
  matches. But `-o` does *not*: it prints the matched part for a match and the
  whole line for context.
- **`-m` still owes the trailing context of its last match**, so
  `grep -A1 -m1 M` prints the match and the line after it.

**`-e` and `-f`** supply patterns, so several can be given and a pattern starting
with a dash stops looking like an option. With either present the first operand
is a file rather than the pattern. Each pattern is escaped and anchored
*separately* before being joined into an alternation, which is a correctness
matter and not a style one: `-F -e a.c` escaped as one string would escape the
`|` that joins them, and `-x -e a -e b` anchored as one alternation would give
`^a|b$`, matching any line containing `b`. An empty `-f` file means no pattern,
which matches nothing and exits 1 — the measured answer, and the opposite of
treating "no pattern" as "empty pattern".

**`-L`** is `-l` inverted, and it inverts the exit status with it: it exits 0 when
it listed something, which is when some file did *not* match.

30 of 30 measured forms agree with busybox-w32 byte for byte, exit statuses
included. Diagnostics match too: `grep -A x` answers `invalid number 'x'`.

### `head` and `tail`

**An attached value works now**: `head -n2`, `head -c2`, `tail -n+2`,
`tail -c+3`. Before 2026-08-22 `head -2` worked, `head -n 2` worked, and
`head -n2` was refused — the worst shape a gap can take, because a user cannot
predict which of three spellings the shell has.

The cause is worth recording, because it is a fork this package still has. There
are two option readers: `parseAppletOptions` is a real getopt and takes an
attached value like `chmod -m755`, while `streamOptionsAndOperands` matches whole
argument strings against a whitelist and therefore cannot express "this letter
carries a value". `head` and `tail` were built on the second. Only those two
were affected — `grep -m1` and `sort -k1` already worked, being on the first —
so the fix is `head` and `tail`'s own reader rather than a merge of the two:
neither the bare `-N` form nor the signed counts `+2` and `-2` is a getopt shape.

**More than one file operand now gets a header**, which is the other half of the
same commit:

```console
$ head -n1 a.txt b.txt
==> a.txt <==
1

==> b.txt <==
x
```

`-q` suppresses it and `-v` forces it for a single file. Before this there were
no headers at all, so `head *.log` produced lines with no way to tell which file
each came from. A single file and stdin still get none; stdin has no name to
print. The header names the operand **as spelled**, so `./a.txt` comes back as
`./a.txt`.

Diagnostics match the reference to the character, single quotes included:
`head -n2c` answers `head: invalid number '2c'` where this used to say
`invalid count: 2c`, and once a sign is taken off it is the digits that are
reported, so `head -n-x` answers `'x'` and not `'-x'`. 24 of 24 measured forms
now agree with busybox-w32 byte for byte.

`tail -f` and `head -z` stay refused: following a file needs a polling loop and a
decision about truncation, and `-z` is a GNU-only NUL-terminated-line mode that
is a real choice rather than an oversight.

### `ls`

Beyond the long form and the layout options, `ls` sorts and descends: `-t` by
time, `-S` by size, `-r` reversing whichever key is in force, `-R` descending,
`-d` naming a directory instead of listing it, `-F` marking what each entry is,
and `-A` for the hidden entries without `.` and `..`. All seven were refused by
name until 2026-08-22, which meant `ls -ltr` — about as well-worn a command as
there is — failed on its options.

Three rules are worth stating because they are not guessable, and all three were
measured against busybox-w32 rather than chosen:

- **The last sort option wins.** `ls -S -t` orders by time and `ls -t -S` by
  size. GNU documents this; busybox does it.
- **`-a` beats `-A` in either order.** `ls -A -a` and `ls -a -A` both list `.`
  and `..`. GNU instead lets whichever came last win, so this follows busybox.
- **The name is the tie-break for every key**, and `-r` reverses the tie-break
  along with the key. Without that, two files of the same size in the same second
  could come out in either order and a diff between two listings of an unchanged
  directory would mean nothing.

`-R` heads each directory with its path and a colon, separates blocks with one
blank line, writes no blank line after the last block, and gives an empty
directory a header and nothing else. The header path is built from the operand
**as spelled**, so `ls -R .` says `./sub`, matching `find .` writing `./a.txt`;
`.` and `..` are listed under `-a` but never descended into. The separator is a
forward slash even on Windows, which is busybox's answer and also nemosh's
canonical path form — uutils' Windows tests assert a backslash there, and that
divergence is deliberate.

`-F` marks a directory `/`, a symlink `@`, and an executable `*`. Windows has no
execute bit, so the executable test is the same suffix list the shell uses for
command lookup. The indicator sits outside the colour escapes, where busybox puts
it, so a filter stripping the colour does not keep a stray marker.

`ls -d -l` on a directory prints exactly the line a directory listing prints for
it, including the same two pre-existing Windows differences from busybox: the
mode reads `drwxrwxrwx` where busybox says `drwxrwxr-x`, and the link count is 1.
Neither is `-d`'s doing.
