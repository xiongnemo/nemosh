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
| `basename` | `-a`, and the `basename PATH [SUFFIX]` form | refused by name |
| `cat` | `-n` | refused by name |
| `chmod` | numeric mode | refused by name |
| `clear` | none | refused by name |
| `cmp` | `-s -l`; the message goes to stdout, as GNU's does | refused by name |
| `comm` | `-1 -2 -3` | refused by name |
| `cp` | `-r`, `-R` | refused by name |
| `cut` | `-b -c -d -f -n -s` | refused by name |
| `date` | `-d -u` | refused by name |
| `dirname` | none needed | refused by name |
| `du` | `-s -h`; **apparent** sizes in 1024-byte blocks, not allocation | refused by name |
| `echo` | `-n -e` | treated as text, which is what `echo` does |
| `env` | `-i`, and `NAME=VALUE command` | refused by name |
| `expr` | none; every argument is a term | read as a term, so a bad one is a syntax error |
| `find` | `-name -iname -path -ipath -type f\|d\|l\|c -size -mtime -newer -empty -print -print0 -maxdepth -mindepth`, and the operators `-a -o ! -not -and -or ( )` | refused **before the walk** |
| `grep` | `-i -n -v -r -R -l -c -q -w -x -F -o -s -h -H -E -m`, `--color[=WHEN]` accepted and ignored | refused by name |
| `head` | `-n -c -q -v`, the `-N` form, and an attached value (`-n2`) | refused by name |
| `id` | `-u -g -G -n`, and their clusters | refused by name |
| `ln` | `-s` | refused by name |
| `ls` | `-a -A -h -l -1 -C -w N -t -S -r -R -d -F`, `--color[=always\|never\|auto]` | refused by name |
| `mkdir` | `-m -p -v` | refused by name |
| `mktemp` | `-d -q -u`, and an `XXXXXX` template | refused by name |
| `mv` | `-f`, accepted and already in force | refused by name |
| `nl` | `-b t\|a\|n` | refused by name |
| `paste` | `-s -d`; the delimiter list cycles | refused by name |
| `pgrep` | `-l -x`, a regular expression on the process name | refused by name |
| `pkill` | `-x` and a leading `-SIG`, a regular expression on the process name | refused by name |
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
| `sed` | `s///` substitution | refused by name |
| `seq` | `LAST`, `FIRST LAST`, `FIRST INCREMENT LAST` | read as a number, so a bad one is refused |
| `sleep` | duration operand | reported as an invalid duration |
| `sha256sum`, `md5sum` | `-b -c -t -w`; `-c` accepts both the two-space and `*` spellings | refused by name |
| `sort` | `-n -r -u -f -b -k -t` | refused by name |
| `stat` | `-c FORMAT` with `%n %s %F %f %y %Y`; the default output is refused | refused by name |
| `split` | `-l`; two-letter suffixes, `aa` upwards | refused by name |
| `su` | `-c -s -t -W -N`; Windows only, see **Elevation** | refused by name |
| `tac` | none | refused by name |
| `tail` | `-n -c -q -v`, the `-N` form, and an attached value (`-n2`, `-n+2`) | refused by name |
| `test`, `[` | POSIX expressions | an operand, per the POSIX one-argument rule |
| `tee` | `-a` | refused by name |
| `touch` | `-c` | refused by name |
| `tr` | `-d -s -c`, ranges and backslash escapes; not classes | refused by name |
| `true`, `false` | none, by definition | ignored, which POSIX requires |
| `uname` | `-a -i -m -n -o -p -r -s -v` | refused by name |
| `uniq` | `-c -d -u -i` | refused by name |
| `wc` | `-c -l -w -m -L` | refused by name |
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

- `sed` over a UTF-16 file matches nothing, so it copies the file through.
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
- **`sed` beyond `s///`.**
- **`find -exec` and `-delete`.** The operators, `-size`, `-mtime`, `-newer`,
  `-empty`, `-maxdepth` and `-print0` landed on 2026-08-22; these two did not,
  because one needs the execution model and quoting rules and the other needs a
  deliberate decision about a destructive default. `| xargs -0` covers most of
  what `-exec` is reached for, and now has `-print0` to pair with.
- **`grep -A -B -C`.** Context lines need a ring buffer of preceding lines; the
  option is refused rather than approximated.

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

Filling in the rest is v1.1; see
`docs/design/v1-scope.md` and the per-applet tables in
`docs/testing/applet-test-inventory.md`.

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
