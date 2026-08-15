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
| `su`, `su root` | an elevated `nemosh -i` in a new console, starting in the current directory |
| `su -c CMD` | that shell runs `CMD` |
| `su -s SHELL` | launches `SHELL` instead. `cmd.exe` is given `/c`, everything else `-c`, matching busybox (`suw32.c:118-120`) |
| `su -W` | waits and reports the shell's exit status; without it `su` returns as soon as the shell is launched, having nothing to report |
| `su -t` | test mode: the `open` verb instead of `runas`, so the whole path runs with no elevation and no consent dialog. This is what makes any of it testable |
| `su USER` for any other user | refused. There is no user database here; `root` is the name this shell gives an elevated token, not an account |
| `su -N` | **refused.** It should hold the console open at exit, and that needs an option in the shell itself which this build's argument parsing has not got |
| the consent dialog answered "no" | status 1, `elevation was refused` — a decision, not a fault |

The working directory is passed explicitly and canonicalised first, because a
directory reached through a mapped network drive may not exist under the
elevated token — drive mappings belong to a logon session (`suw32.c:96-113`).
Measured: without it, ShellExecuteEx decides for itself.

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

### Known divergences from bash/dash/ash

- **Parse before effects.** A syntax error anywhere in a script means none of it
  runs. bash and dash execute up to the error. `{echo bad;}` produces no output
  here; both references print nothing either but reach the command first.
- `~user` is left as written. `~` and `~/path` work.
- An alias whose value is not a list of words is refused at definition time,
  because substitution happens after parsing.
- `${#@}` is not pinned; POSIX leaves it unspecified and the references disagree.

## Applets

All 48 registered applets ship, plus `su` on Windows. **Name presence is not option parity**, and the
column that matters is the third one.

| Applet | Options implemented | Unknown option is |
| --- | --- | --- |
| `basename` | `-a`, and the `basename PATH [SUFFIX]` form | refused by name |
| `cat` | `-n` | refused by name |
| `chmod` | numeric mode | refused by name |
| `clear` | none | refused by name |
| `cp` | `-r`, `-R` | refused by name |
| `cut` | `-b -c -d -f -n -s` | refused by name |
| `date` | `-d -u` | refused by name |
| `dirname` | none needed | refused by name |
| `echo` | `-n -e` | treated as text, which is what `echo` does |
| `env` | `-i`, and `NAME=VALUE command` | refused by name |
| `find` | `-name`, `-type f\|d\|l`, `-print`, implicit AND | refused **before the walk** |
| `grep` | `-i -n -v`, `--color[=WHEN]` accepted and ignored | refused by name |
| `head` | `-n -c`, and the `-N` form | refused by name |
| `id` | `-u -g -G -n`, and their clusters | refused by name |
| `ln` | `-s` | refused by name |
| `ls` | `-a -h -l -1`, `--color[=always\|never\|auto]` | refused by name |
| `mkdir` | `-m -p -v` | refused by name |
| `mktemp` | `-d -q -u`, and an `XXXXXX` template | refused by name |
| `mv` | `-f`, accepted and already in force | refused by name |
| `pgrep` | `-l -x`, a regular expression on the process name | refused by name |
| `pkill` | `-x` and a leading `-SIG`, a regular expression on the process name | refused by name |
| `posixpath` | none | treated as a path operand |
| `printenv` | none | treated as a variable name |
| `printf` | format string | treated as the format, which is correct |
| `pwd` | `-L -P` both accepted | accepted |
| `readlink` | `-n` | refused by name |
| `realpath` | none | treated as a path operand |
| `rm` | `-f -r` | refused by name |
| `rmdir` | `-p -v` | refused by name |
| `sed` | `s///` substitution | refused by name |
| `seq` | `LAST`, `FIRST LAST`, `FIRST INCREMENT LAST` | read as a number, so a bad one is refused |
| `sleep` | duration operand | reported as an invalid duration |
| `sort` | `-n -r` | refused by name |
| `su` | `-c -s -t -W`; Windows only, see **Elevation** | refused by name |
| `tail` | `-n`, and the `-N` form | refused by name |
| `test`, `[` | POSIX expressions | an operand, per the POSIX one-argument rule |
| `tee` | `-a` | refused by name |
| `touch` | `-c` | refused by name |
| `tr` | `-d -s -c`, ranges and backslash escapes; not classes | refused by name |
| `true`, `false` | none, by definition | ignored, which POSIX requires |
| `uname` | `-a -i -m -n -o -p -r -s -v` | refused by name |
| `uniq` | `-c` | refused by name |
| `wc` | `-c -l -w` | refused by name |
| `whoami` | none | refused by name |
| `winpath` | none | treated as a path operand |
| `xargs` | none | refused by name |
| `yes` | none | treated as the string to repeat |

### Options a script is most likely to reach for and not find

`xargs -0`, `xargs -n`, `sort -k`, `grep -r`, `tail -c`, and `ls -l` beyond the
basic long form. Every one of them is refused by name, so a script asking for it
fails rather than quietly getting something else.

`tail -c` is worth calling out because `head -c` now exists: head counts bytes
and tail does not, and the asymmetry is deliberate rather than overlooked --
claiming both would be the kind of thing a script discovers the hard way.

Filling in the rest is v1.1; see
`docs/design/v1-scope.md` and the per-applet tables in
`docs/testing/applet-test-inventory.md`.

### `find`

`-name`, `-type`, and `-print` are implemented, combining with the implicit AND
POSIX specifies. `-name` matches the basename, not the path, because busybox
uses `fnmatch` without `FNM_PATHNAME` and a basename carries no separator for
`*` to cross. `-type` classifies `f`, `d`, and `l`; busybox also accepts `b`,
`c`, `s`, and `p`, which are refused by name here rather than answered as though
a block device could never match.

Every other predicate — `-mtime`, `-size`, `-perm`, `-exec`, `-prune`, `-regex`,
`-maxdepth`, and the rest — is **refused before the first directory is read**:

```console
$ find . -mtime 1
find: unrecognized: -mtime
$ echo $?
1
```

That ordering is the fix, not a detail. Until 2026-08-07 `find` honoured no
expression at all: it walked the whole tree, printed every path, and only then
reported the predicate as a missing file. `find . -name '*.tmp' | xargs rm`
therefore received every file. Both halves were measured, and twelve forms —
`.`, `./`, `sub`, `sub/`, and each predicate combination — now match busybox-w32
byte for byte.

Output follows POSIX rather than being cleaned: the path operand is written
exactly as given, then a slash, then the rest. `find .` yields `./a.txt`, not
`a.txt`.
