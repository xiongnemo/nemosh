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
| `ulimit` | 126 | Windows has no `getrlimit`. busybox-w32 does not implement it either — it keeps the name and returns 1 with no message. |
| `bind` | 126 | The line editor's keys are its own: it reads no inputrc, and nothing rebinds them. busybox-w32 has no `bind` either, and says `not found`, 127, which reads as a program to install. `command -v bind` answers no, so a bashrc that asks first skips it. |
| `fg`, `bg` | 126 | They resume a *suspended* job and nothing here can suspend one — see **Process control** below, which is the long answer. busybox-w32 compiles both out under `#if JOBS`. These two say **"not implemented, and will not be"** where the rows above say only "not implemented", because they are settled rather than pending. |
| `set -b` | 2 | Completion is already reported at the next prompt, which is the default behaviour it would be switching off. What `-b` asks for is the report *immediately*, mid-command, and there is no notification channel to switch on for that. |
| `set -v` | 2 | A script is parsed in full before any of it runs, so by the time the option is set there is no unread input left to echo. `nemosh -v` is refused for the same reason. `set -n` acts: nothing after it runs, as in both references, and `nemosh -n SCRIPT` is the syntax check. |
| `set -m`, `set -o monitor` | 2 | There is no job control to switch on: nothing here can stop a job and resume it, which is what `fg` and `bg` are refused for too. busybox-w32 accepts the option, with job control compiled out. |

The shell's own command line takes what busybox's does:
`nemosh [-ils] [-|+aCeEfnux] [-|+o NAME]... [-c COMMAND [NAME [ARG]...] | SCRIPT [ARG]...]`.
The letters mean what `set` makes them mean, so `nemosh -eu -o pipefail script` works,
and so does a script that begins `#!/bin/sh -e`, which is launched as `nemosh -e script`.
Only `-c` and `-i` were read before, and only as the first argument; every other
option was "invalid". `-l` reads `/etc/profile` and `$HOME/.profile` first. `-n`
parses the script and runs none of it. `$-` has `c` for a command string, `s` for
commands read from standard input, and `i` and `s` for a session, among the letters
that are on in busybox's order, from the end of its option table: `set -eu` under `-c`
is `uce`. A name `-o` does not have exits 2, bash's answer. busybox reports it and then
exits 0.

busybox-w32's other options are here too:
- `set -o ignoreeof` (`-I`) refuses an end of input at the prompt. It prints
  `Use "exit" to leave shell.`, and the fiftieth in a row leaves anyway, as
  busybox's does.
- `set -o nohiddenglob` keeps files with the Windows Hidden attribute out of
  pathname expansion.
- `set -o nohidsysglob` keeps out only those that are Hidden and System, the pair
  on `desktop.ini`.

Both glob options are off by default, as they are in busybox.

Beyond POSIX, `history`, `which` and `set -o nocaseglob` are implemented, both
following busybox.

History survives the session. `HISTFILE` names the file, exported or not, and a
prompt sets it to `~/.nemosh_history` once the rc file has run without setting it,
as busybox sets `~/.ash_history`; `HISTFILESIZE` caps it at 500 lines by default; setting
either to nothing turns saving off, which is bash's rule and busybox's. Lines
are appended one at a time in a single write, so a session that is killed still
leaves what it ran and two windows interleave whole lines rather than
overwriting each other, and the file is rewritten only once it has grown to four
times the cap (`libbb/lineedit.c:1826`, `:1841`).

`history`'s options are bash's, busybox's taking none: `history N` lists the newest
N; `-c` clears; `-d` takes out a position, one counted back from the end (`-1`), or a
range (`2-4`); `-s` adds its words as one entry in place of the line that ran it; `-p`
prints its words history-expanded; and `-a`, `-n`, `-r` and `-w` append to, read on
from, read and write FILE or `HISTFILE`. `-a` writes only the lines the file has not
got, which at a prompt is what `history -s` added, every typed line being written as
it runs.

`fc` is bash's, busybox having none. `-l` lists, `-n` without numbers and `-r` newest
first; FIRST and LAST are numbers, numbers counted back from the newest, or the newest
entry that starts with a word, read by bash's rules to the quirk (a FIRST naming the
newest entry lists from the oldest). `-s [pat=rep] [command]`, and `-e -`, runs an
entry again in place of the `fc` line. Without `-l` the entries go to a file and to
`-e`'s editor, or `FCEDIT`'s, or `EDITOR`'s, or `vi`, and what the file holds when
the editor succeeds is said on standard error and run. On Windows `fc` was the file
compare, `fc.exe`, which that name still runs.

Tab completion and the inline suggestion offer host names for `ssh`, read from
`~/.ssh/config` -- and `/etc/hosts` off Windows. See
`docs/design/completion.md`, Host names, for what is read and what deliberately
is not.

### Process control — what is implemented, and what will not be

One table, because the line between the two halves is a single distinction and
it is easier to see them together.

| | Implemented | Why it can be |
| --- | --- | --- |
| `jobs`, `wait`, `wait %N ...`, `wait -n` | yes | bookkeeping over the shell's own job table. Several operands answer the last one's status; `wait -n` answers whichever job ends first; one the shell does not know is 127. A pid that is no child is passed over in silence and leaves the status as it was, 127 before any other, as busybox's waitcmd leaves it; it said bash's `pid N is not a child of this shell`. `jobs` in a pipeline stage or a command substitution lists the shell's jobs, as both references do, so `jobs -p \| wc -l` and `kill $(jobs -p)` work; they saw an empty table before. In a subshell `( )` the table is its own, and empty, as in both |
| `kill %N` | yes | a job is a process: the signal goes over its control pipe, and KILL ends its Job Object. A goroutine job (`NEMOSH_JOBS=goroutine`) is ended by cancelling its context |
| `coproc cmd`, `coproc NAME { ...; }` | yes | bash's, which busybox-w32 has not got: a background job with a pipe to its stdin and one from its stdout. The shell's ends are `${NAME[1]}` and `${NAME[0]}`, on 60 and 63 as bash puts them. `$NAME_PID` is the job's `$!`. `exec {NAME[1]}>&-` ends its input, and the `wait` that reaps it closes both ends and unsets both names. NAME is COPROC unless the command is a compound one (a group, a subshell, a loop, an if or a case), as in bash. It runs under either launcher, since it is an ordinary job whose group redirects its 0 and 1. It used to be refused with 126 |
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

Background jobs are real child processes now (below), so making `fg` work would
come down to gambling on `SuspendThread` against the heap lock — trading a
property that holds for a feature that might.

**Background jobs are processes, as busybox-w32's are.** It has no fork either:
`spawn_forkshell` (`shell/ash.c:17040`) copies the shell's state into a file
mapping with an inheritable handle and launches a fresh `sh --fs <handle>`, which
maps it and carries on, and `forkparent` sets `$!` from `GetProcessId`. Here a
`cmd &` starts this binary as `nemosh --job <handle>` and sends it the job's state
and program over an inherited pipe (docs/design/background-processes.md). What that
buys is a real pid, a `wait` over process handles, a `kill` that can tell one
signal from another (below), and a KILL that takes the programs the job started with
it. What it does not buy is suspension — the three reasons above are about Windows
and Go, not about goroutines. It costs about 4-6 ms a job and 11 MB while one runs
(startup-and-footprint.md). `NEMOSH_JOBS=goroutine` is the way back to the goroutine
each job used to be, which starts in microseconds and whose `$!` is `%1`.

### `kill`

A builtin, as busybox's is (`shell/ash.c:12096`), and for the same reason: `%N`
names a job and only the shell has the job table. busybox's `killcmd` does
nothing but translate `%N` into that job's pids and hand them to the ordinary
`kill` (`:4787-4830`).

Here `%N` and a job's pid both reach the job's record, and the record knows how
to reach the job. A job that is a process is sent the signal as a message on a
control pipe, so its own trap can run, and KILL ends its Job Object, which is the
job and every program it started. A job that is a goroutine (`NEMOSH_JOBS=goroutine`)
has no pid; what it has is its own context, so a signal nothing catches arrives as a
cancellation, and an external command in the job, launched with
`exec.CommandContext` under that context, is terminated with it.

| Form | Behaviour |
| --- | --- |
| `kill %N` | **a trap the job set for the signal runs**, once the command in progress has finished, and then the job carries on; `trap '' TERM` makes it ignore the signal. That is bash's behaviour, for every signal `kill -l` lists but KILL. busybox-w32 cannot deliver a signal to a trap, so every kill there ends the job. A signal the job does not catch ends it, and its EXIT trap still runs. KILL is never caught. The status says which signal it was, as busybox's does: `wait` answers 128 plus the signal -- 137 for KILL, 143 for TERM -- and `jobs` names it `Killed` or `Terminated` rather than `Done(1)`, which is what it used to say. `wait %N` and `wait PID` say the word on stderr too, but not for INT or PIPE, and a `wait` with no operands says nothing, as busybox's says nothing. The words are libc's, `Segmentation fault`, as bash and busybox on Linux print them; busybox-w32's own table names all but KILL and TERM bare, `SEGV`. Off Windows a job process ends by the signal itself: for QUIT, ILL, FPE, SEGV, ABRT and PIPE, which Go's runtime will not hand back, it becomes `/bin/sh` and sends the signal to itself |
| `trap … TERM` in a script | runs when a TERM reaches the script from outside it, as bash's does. Off Windows that is a real `kill` (and HUP and QUIT too), and the trap runs once the command in progress has finished. On Windows it is the console closing, the user logging off or the machine shutting down, which is all Go reports as TERM there. Windows ends every process on the console within seconds of that, so the script is stopped where it is: its TERM trap runs, then its EXIT trap, and it exits by the signal. A prompt ignores TERM, as bash's does |
| `trap … PIPE` | SIGPIPE is what a write into a pipe no one reads raises: the shell, a subshell or a pipeline stage whose own write it is ends there with 141, and a program it runs ends alone, as in both references. `trap '' PIPE` (or `13`) ignores it, as bash's does: the write fails instead and says so, `echo: write error: Broken pipe` and status 1, the shell goes on, and a program sees its failed write the same way. A trap for it runs once the command in progress has finished, after the same failed write, while a program still ends of it. A subshell or a stage keeps it ignored and drops a trap for it, as POSIX has it. Off Windows the shell's top level makes it the process's SIGPIPE, which the programs it starts inherit. busybox-w32 takes the trap too. `kill -PIPE` sends it like any other signal: a job's or the shell's trap for it runs, and with none the job ends with 141, which `wait` does not say, as busybox's does not |
| `kill PID` | `TerminateProcess` on Windows, a real signal elsewhere. busybox-w32 uses `TerminateProcess` only for KILL; every other signal injects a thread into the target that calls `ExitProcess(signal << 24)` (`win32/process.c:862-909`), so the parent sees which signal ended it |
| `kill -9`, `kill -TERM`, `kill -SIGTERM` | all accepted; a script writes the number and a person writes the name |
| `kill -0 %N`, `kill -0 PID` | **asks, and changes nothing**: 0 while the job or process is running, 1 once it has ended. It used to be one more signal, so the question ended what it asked about |
| `kill -STOP`, `-CONT`, `-TSTP` (or 18–22, but 22 on Windows is ABRT) | **refused by name**, status 1. Nothing here can suspend a process, and delivered the way every other signal is, STOP would end it. busybox-w32 has neither name either. `pkill` and `killall` refuse them too: all three read one table, `internal/proc/signal.go` |
| `kill -l` | lists the signals this shell can act on, not the whole POSIX set: busybox-w32's table, HUP, INT, QUIT, ILL, FPE, KILL, SEGV, PIPE, TERM and ABRT. ABRT is 22 on Windows, as MinGW numbers it and busybox-w32 lists it, and 6 elsewhere. ILL to ABRT were an invalid signal to `kill`, and a signal `trap` did not support. `trap` lists the traps in busybox's order, EXIT, the signals by number, then ERR, with bash's DEBUG and RETURN either side of it |
| a pid that has already exited | refused, not reported as killed — the check busybox makes with `GetExitCodeProcess` first |
| a job that has already ended | refused the same way. Both references accept `kill %1` for a job that has ended but not been reported; they refuse `kill $pid` once the process is gone, and here `$!` is `%1`, so the second is what a script is writing |
| pid `0` or negative | refused on Windows: those mean process groups, which Windows has not got in the POSIX sense. Passed through elsewhere |
| its failures | busybox's statuses, in this shell's words. No operand is `expected a job or a process id`, 1, where bash prints its usage with 2; a signal it does not know is `invalid signal 'X'`; an operand that is no number is `invalid pid 'X'`; one it could not signal is `cannot signal pid N: No such process` (or `Permission denied`), in strerror's words, where Windows said `The parameter is incorrect.`; and the status is how many operands failed, as busybox counts them, 255 at most. A job spec that names no job is 2, and since every spec is found before anything is sent, nothing is signalled. `%%` and `%-` with no such job are `No current job` and `No previous job`, as every ash says them |

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
a reserved word, and its expression is read with the script, as bash's parser
reads it, its words still unexpanded -- which also supplies the other thing an
applet could not know: whether the right-hand side was quoted, and therefore
whether `==` compares a pattern or a literal. Each operand is expanded when its
test is made: `[[ -z $x || $(f) ]]` runs `f` only when `x` is not empty.

| | |
| --- | --- |
| `==`, `=`, `!=` | the right side is a **pattern** unless quoted. `[[ abc == a* ]]` is true, `[[ abc == "a*" ]]` is false |
| `=~` | an extended regular expression, anchored nowhere |
| `<`, `>` | lexical comparison, **not redirection** -- which is a lexer question, and the reason `[[` had to become known to the lexer |
| `-eq -ne -lt -le -gt -ge` | numeric |
| unary tests, `-nt -ot -ef` | `test`'s own, through one exported entry point, because two copies of `-f` would drift |
| `&&`, `||`, `!`, `( )` | the conditional's own grammar, not the shell's, and operators only as written: `op='=='; [[ a $op a ]]` is a syntax error, as in bash |
| a malformed expression | a **syntax error** before anything runs, status 2, as bash's parser has it: `[[ -z ]]`, `[[ a b ]]`, `[[ ]]` |
| a test that cannot be made | **status 2**, so "that was not an answer" stays distinguishable from "the answer is no": `[[ a -lt 1 ]]` |

The expression goes on past the end of a line, as bash's does, and `[[` alone on
its line begins one. Its operators end the word before them -- `[[ b>a ]]` --
and a `]]` before the shell's own ends it: `[[ $x ]]|| echo empty`,
`[[ $x ]]>log`. A reserved word may follow `]]` with no separator,
`if [[ $x ]] then`.
`[[` is only a conditional at the **start of a command** -- `echo [[` prints two
ordinary words. Where busybox reads a binary test that bash's parser refuses,
`[[ -f == -f ]]` and `[[ ! == x ]]`, the strings are compared, as busybox has it.
`set -x` traces each test as it is made, as bash does: `+ [[ 5 == 5 ]]`.

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

All of `declare -A`, slices (`${a[@]:1}`), negative indices and `unset a[i]` work. This
paragraph said none of them did, which had been wrong for some time -- the first three
landed without it being updated, and the fourth landed on 2026-09-12.

A slice's negative length counts back from the end of a string, and one that ends before
the offset is bash's `substring expression < 0`, as a negative length of a list always is:
the rest of the line is not run and the script goes on with the next, status 1, under
`set -e` too, as bash goes on. The list's ended the script, and the string's was empty.

`declare -a a`, `declare -A m` and `local -a a` with no value leave the array declared
and unset, as bash leaves it: `declare -p` writes `declare -a a`, where `a=()` is
`declare -a a=()`, and anything stored in it makes it set. Under `set -u` an element that
is not there is unset, `${a[5]}` or `${m[k]}`, and so is the count of a name never set or
only declared, `${#a[@]}`, in busybox's words for any unset parameter; `${a[@]}`, an
operator's word and `${#a[5]}` of an array that has values stay quiet, as in bash.

`unset a[i]` **leaves a gap** rather than compacting, which is bash's behaviour and the
only safe one: compacting would shift every later index and silently change what every
subsequent read means. Until it was implemented it returned 0 and did nothing at all, so
a script that removed an element and carried on was quietly wrong -- the failure mode
AGENTS.md singles out.

### The line editor's history keys

Three ways to reach history, answering three questions, which is why there are three:

| Key | Walks | The question it answers |
| --- | --- | --- |
| Up / Down | everything, in order | the command was recent |
| `^R` | anywhere in the line, incrementally | a word from the middle is remembered |
| Page Up / Page Down | entries beginning with the text left of the cursor | how the command *started* is remembered |

The third is zsh's `history-beginning-search-backward`, and it is an **extension**:
busybox's line editor binds Page Up to nothing at all, so there is no parity to match here.
It earns its place because how a command started is how it is usually remembered — `git c`
and Page Up beats pressing Up eleven times.

The prefix is what lies **left of the cursor**, not the whole line, which is what makes the
key repeatable: the cursor stays where it was, so a second press narrows from the same
prefix. An empty prefix makes it Up, as it does in zsh. Nothing matching leaves the line
alone — a key that cleared what you had typed because it could not find it would be worse
than one that did nothing.

### The line editor's kill ring

`^K` kills to the end of the line, `^U` to the start, `^W` the word before the cursor, and
**`^Y` puts back** what any of them took. `^U` and `^W` existed and simply destroyed what
they removed, which is the half that makes them frightening to use -- nobody asks for a
kill ring by name, they notice that `^U` lost something and stop pressing it.

`^U` changed meaning with this: it now kills **backwards to the start of the line**, which
is readline's unix-line-discard, so `^U` with the cursor in the middle keeps the tail. It
cleared the whole line before -- a more destructive gesture wearing the same key.

The ring is readline's: the last ten kills, `M-d` (kill-word) among the keys that fill it,
and a kill straight after another joined to it, so two `^W` are one entry that `^Y` puts
back whole. **`M-y` straight after `^Y`** takes the yank back out and puts in the kill
before it, round the ring, and the ring stays turned for the next `^Y`, until a kill. It
held one entry, the newest, until 2026-10-02.

### The line editor's other readline keys

- `^B`, `^F`, `^P` and `^N` are the arrows, as busybox's editor and readline bind them.
  Like Right, `^F` at the end of the line takes the suggestion.
- The rest are bash's, since busybox's editor has none of them:
  - `^T` is transpose-chars. The character before the cursor goes over the one at it, and
    at the end of the line the two before the cursor swap.
  - `M-.` and `M-_` are yank-last-arg. They put in the last word of the line before, as
    `!$` would have it. Pressed again straight away, each reaches one line further back,
    and past the oldest line it puts in nothing.
  - `M-u`, `M-l` and `M-c` put the rest of the next word in upper case, lower case, or
    capitalised, a word being letters and digits as readline's are.
  - `C-_` and `C-x C-u` are undo: each takes back the last change to the line, and
    pressed again the one before. Characters typed one after another are one change, up
    to twenty of them, as readline joins them, and a key that only moves the cursor is
    none. `M-r` is revert-line, every change taken back. A line recalled from history
    starts its own undo.
  - `C-x C-e` is edit-and-execute-command: the line goes to a file and the file to
    `VISUAL`, or `EDITOR`, or `emacs` (`vi` in vi mode), and what the editor leaves is
    said on standard error and run as though typed, kept in history in the line's place.
    An editor that fails runs nothing.

### The line editor's vi mode

`set -o vi` is busybox's vi editing mode, from the next line on, and `set -o emacs` or
`set +o vi` leaves it; vi and emacs turn each other off, as bash's two modes do. A line
starts in insert mode, where the keys are the ones above. Escape goes to command mode, one
character back, and there the keys are busybox's (libbb/lineedit.c):

| Keys | Do |
| --- | --- |
| `i` `I` `a` `A` | insert at the cursor, at the start, after the cursor, at the end |
| `x` `X` | delete the character under the cursor, or the one before it |
| `w` `W` `e` `E` `b` `B` | the word motions: `w`, `e` and `b` stop at punctuation, the capitals only at blanks |
| `0` `$` `h` `l` Space Backspace | the start, the end, left and right |
| `j` `k` | the next and the previous history line |
| `d` or `c` with `w` `W` `e` `E` `b` `B` Space `$` | delete over the motion, and `c` goes on to insert; `dd` and `cc` take the whole line |
| `D` `C` | delete to the end, and `C` goes on to insert |
| `p` `P` | put back what the last deletion took, after the cursor or at it |
| `r` | replace the character under the cursor with the next key |

Enter, `^C`, `^D`, `^L`, `^U`, `^W`, `^N`, `^P`, Delete, the arrows, Home and End, Ctrl-Left
and Ctrl-Right, and Page Up and Page Down do what they do in insert mode. Any other key does
nothing in command mode, so a mistyped letter never lands in the line. A history line is
shown with the cursor at its start, in either mode, as busybox has it.

busybox-w32 reads the console's key events, where Escape is always a key of its own. This
editor reads what a terminal sends, which begins its sequences with Escape. So an Escape is
read with the next byte only when that is `[` or `O`, and one that nothing follows within
50 ms is the key on its own, the wait busybox's read_key makes on a terminal. Alt and a
letter arrives as Escape and the letter, so in vi mode it is the two keys. No counts, no
`u`, and `.` repeats nothing, as in busybox.

### Programmable completion

**`complete`, `compopt` and `compgen` are bash's**, busybox having no programmable
completion. `complete` keeps a specification per command -- its actions, `-W`, `-G`,
`-F`, `-C`, `-X`, `-P`, `-S` and `-o` options, and `-D`, `-E` and `-I` -- and
`complete -p` prints them back as bash does, so the output reads in again; `-r`
removes them. `compopt` turns a specification's `-o` options on and off, the running
completion's when it names none. A tool's completion script -- `gh completion -s
bash`, rustup's, npm's, git's -- runs to its end; each stopped at its first
`complete`, which was not found. Unlike bash, a `complete` in a subshell is the
session's too.

**Tab asks them first.** For an operand of a command with a specification -- its
own, its last path component's, or `-D`'s; `-E`'s on an empty line -- the line
editor runs it as bash's programmable completion does: its actions, `-G`, `-W`,
`-F` and `-C`, then `-X`, `-P` and `-S`. The function is called with the command,
the word and the word before it, and `COMP_WORDS`, `COMP_CWORD`, `COMP_LINE`,
`COMP_POINT`, `COMP_TYPE` and `COMP_KEY` set as bash sets them: the line split at
`COMP_WORDBREAKS`, which a session starts with bash's value, so `--opt=va` is three
words and the word is `va`. A function answering 124 has loaded a specification,
and it is looked up again. What `COMPREPLY` holds is put in as readline puts it:
one candidate whole, as it is unless `-o filenames` says it is a name, then followed
by a blank, or not under `-o nospace`; several as far as they agree, and listed,
unsorted under `-o nosort`. When it answers nothing, `-o default` and `-o
bashdefault` complete as the editor does without a specification, which is what
happens for a command with none.

**The bash-completion helpers the generated scripts call are builtins.** cobra's
scripts -- `gh`, `kubectl`, `docker`, `helm` -- read the line with
`_get_comp_words_by_ref` when the bash-completion package's `_init_completion` is
not there, and complete files with `_filedir`; neither is bash's, and on Windows the
package is seldom installed. `_get_comp_words_by_ref`, `_init_completion`,
`_filedir` and `__ltrim_colon_completions` are the package's 2.12 `_comp_get_words`,
`_comp_initialize`, `_comp_compgen_filedir` and `_comp_ltrim_colon_completions` under
the names the scripts call, setting `cur`, `prev`, `words` and `cword` in the calling
function's own variables. A function of the same name, as the package defines one,
is called in their place. `source <(gh completion -s bash)` completes `gh`'s
subcommands, flags and flag values.

### History, and `$!`

`HISTCONTROL` honours `ignorespace`, `ignoredups`, `ignoreboth` and `erasedups`, and
`HISTSIZE` caps the list, keeping the newest. Both were settable and had **no effect
whatever** until 2026-09-13, which matters more than the size of the change: a leading
space is how everyone keeps a token out of their history, so ` export TOKEN=...` silently
wrote the secret to disk. A feature that is absent is noticed; a habit that does nothing
is trusted anyway. Until 2026-10-02 it still did, half: such a line was kept out of
`history` but written to the history file and walked by the arrows. HISTCONTROL now
decides for all three. An unknown `HISTCONTROL` word and an unusable `HISTSIZE` are both
ignored rather than refused, so an rc file shared with bash cannot stop this shell
starting.

**`$!` is the job's pid**, as in both references: a background job is a process
(docs/design/background-processes.md), and `wait $!`, `kill $!`, `tasklist` and `taskkill`
all take the pid. The pid variables answer as bash's do. `$$` and `$PPID` inside a job are
still the shell's. `$BASHPID` is the job's own pid, the same one `$!` names; busybox has no
`$BASHPID`. `jobs -l` adds each job's pid (`[1] 12345 Running`), and `jobs -p` prints the
pids alone. A subshell is not a process of its own here, so its `$BASHPID` is the shell's,
where bash's differs.

**A job outlives the shell that started it**, as in both references: after `exit`, or at
the end of a script, it runs on, and a console window stays open until the jobs attached
to it have ended, which busybox-w32 does too (measured in conhost). **Except after
Ctrl-C.** A script that Ctrl-C ends takes its jobs, and their programs, with it -- once its
own INT and EXIT traps have run -- which is busybox-w32's answer, measured by pressing
Ctrl-C in its console with the job waited for and with it running beside a foreground
command. bash leaves them running, since POSIX has an asynchronous list ignore SIGINT, and
since either answer can surprise someone used to the other, the shell says which it gave
whenever there was a job to end:

    nemosh: Ctrl-C ended the script, and the background job it had running: [1] 3140
    hint: busybox ends a script's jobs on Ctrl-C too; bash would have left them running

On the shell's own stderr, where the person who pressed the key is, and not at all when no
job was running. A Ctrl-C at a prompt interrupts only what is in the foreground, and the
jobs carry on.

**Under `NEMOSH_JOBS=goroutine` a job is a goroutine** in the shell's own process, the way
every job was before the default changed, and it has no pid to report. `$!` is then a job
specification, `%1`, which keeps the two things `$!` is used for working, since `kill $!`
and `wait $!` both take `%N`; a number would have been a pid-shaped lie that `kill` would
apply to some other process. `jobs -p` prints `%1` too, and `$BASHPID` inside the job is the
shell's pid. The same holds for a runtime embedded with applets of its own, whose jobs
cannot be reproduced in another process.

**Backgrounding announces the job at a prompt.** A job that is a process is announced as
busybox announces one, `[1] 19676`. A goroutine job is announced as `[1] started; kill %1
to stop it`: it has no pid, and `%1` is not what someone who has only ever killed a pid
would guess. On stderr, where busybox also puts it, so that `x=$(cmd &)` collects nothing;
and only at a prompt, since a script wants its output rather than a commentary.

**A finished job is reported once**, at the next prompt. POSIX 2.9.3 removes a job from
the list once the shell has reported its status, so naming a job `Done` is what consumes
it: the notice before a prompt, or `jobs`, whichever asks first. A second `jobs` says
nothing, and `wait %N` afterwards answers 127: the job is not known any more, and 127 is
what POSIX gives a process id the shell does not know, and what busybox answers for
`wait $pid` here. It used to be 2.

The notice is bash's and busybox's default behaviour and needs no `set -b`; what `set -b`
asks for is the report *immediately*, in the middle of whatever is running, and that is
the part with no channel behind it and is still refused.

### History expansion

bash's, on interactive input only: readline's histexpand.c under the settings bash gives
it, ported rather than recalled, and measured against bash 5.3. busybox has none. It is a
textual rewrite done before the line is parsed, so the shell proper never sees an
unexpanded `!`. The expanded line is echoed on **stderr** before it runs, which is what bash
does and what keeps a redirected stdout holding only what the command wrote.

- **Events:** `!!`, `!n`, `!-n`, `!string`, `!?string?`, and `!#`, the line so far.
- **Word designators**, after a `:`, or without one when they begin with `^ $ * - %`:
  - `0`, `n`, `^`, `$`, `x-y`, `x*`, `x-` and `*`;
  - `%`, the word the last `!?string?` matched.

  The words are the shell's, so quotes stay in their word and an operator is a word of
  its own: `!$` of `echo "a b"|wc` is `wc`, and `!:1` is `"a b"`.
- **Modifiers**, each after a `:`:
  - `h t r e` take a path apart.
  - `q` and `x` quote.
  - `p` prints the line, records it, and does not run it.
  - `s/old/new/` substitutes, with `&` for old, any delimiter, and the last one optional at
    the end. An empty old is the last one.
  - `&` repeats the last substitution.
  - `g` or `a` repeat along the line, and `G` repeat in each word.
- `^old^new^` at the start of a line is `!!:s^old^new^`.

What is not an expansion is bash's too:
- **Single quotes protect and double quotes do not.** `echo '!!'` is two characters;
  `echo "!!"` is the previous command. The asymmetry looks like a bug until you rely on it.
- **A backslash protects the `!` after it**, and stays for the shell to remove.
- **A `!` that begins nothing is text:** at the end of the line, or before a blank, `=`
  or an operator. That is what keeps `[ x != y ]` working.
- **Nor is a `!` the shell uses:** `$!`, `${!name}` and `[!...]`, or one inside single
  quotes within a `$(...)`.
- A `#` that begins a word ends expansion for the rest of the line.

A reference that cannot be resolved is an **error and the line does not run**. The reason
is in bash's words: `event not found`, `bad word specifier`, `substitution failed`,
`unrecognized history modifier` or `no previous substitution`. Leaving the text as typed
would send `!vim` to PATH as a command name.

It also fixed something else: the **non-terminal interactive loop recorded no history at
all**, so `history` was empty whenever the shell was interactive without a terminal. Both
loops record the whole command now, as the edited one always did.

### Known divergences from bash/dash/ash

- **Parse before effects.** A syntax error anywhere in a script means none of it
  runs. bash and dash execute up to the error. `{echo bad;}` produces no output
  here; both references print nothing either but reach the command first. A heredoc
  the end of a script reaches before its delimiter is no error: it ends there, as in
  both references, and bash's warning, `here-document at line N delimited by
  end-of-file`, is said before the script runs. At a prompt the body goes on at the
  next line read.
- `~user` is left as written. `~` and `~/path` work.
- On Windows, which gives a process no umask, a shell's starts at 0002, busybox-w32's
  DEFAULT_UMASK, so `ls -l` and `stat` show a file 0664 and a directory 0775 as busybox's
  do; Git for Windows's bash starts at 0022. Elsewhere it is the process's own.
- A name `type` or `command -V` does not find is `x: not found` on stdout and 127, and
  `command -v`'s is 127, as busybox answers them; bash says `type: x: not found` on
  stderr, with 1, and its options to type keep its answer. A read-only variable refused is
  `x: is read only`, busybox's words for every refusal, where bash says `readonly
  variable`.
- An alias is substituted as its command runs, not as its line is read: its value is
  read as shell text there, with the rest of the command after it, as both references
  read it. So one defined earlier on the same line, or after a function that uses it
  was defined, is in force here and not yet in theirs, and a value holding `;` keeps
  to its own command's place: in `x | wc -l` all of x's commands are piped, where
  theirs pipe only the last.
- `${#@}` is not pinned; POSIX leaves it unspecified and the references disagree.
- A command name with a slash is a path, as busybox and POSIX have it, so a function
  defined with one -- ble.sh's `ble/is-array` -- is never what runs, and `type` and
  `command -v` say so. bash looks the name up among the functions first and calls it.
- `$"..."` is the double-quoted string, as bash reads it with no message catalog to
  translate it from: the `$` goes, in an operator's word too. busybox has no such string,
  and reads a `$` and a double-quoted string.
- A `$((` that no `))` closes is a syntax error, busybox's `missing '))'`; bash reads it
  again as a command substitution holding a subshell, so `$((cmd) 2>&1)` runs cmd.

## Applets

All 63 registered applets ship, plus `su`, `lsattr` and `chattr` on Windows. **Name presence is not option parity**, and the
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
| `/dev/zero` | endless zero bytes | discards | as above |
| `/dev/random`, `/dev/urandom` | random bytes | discards | as above |
| `/dev/clipboard` | the Windows clipboard as text | sets it; `>>` appends | as above |
| `/dev/stdin`, `/dev/stdout`, `/dev/stderr`, `/dev/fd/N` | this process's own descriptor | same | reported as a device |

`ls -l /dev/null` matches busybox-w32 to the column, including the major and minor
numbers where a size would be. Both are zero and honestly so: these are provided
by the shell rather than by a driver.

`/dev/random` and `/dev/urandom` are the same source. They differ on Linux because
one can block waiting for entropy; Windows has one source that does not block, so
the distinction has nothing to represent. Writing to zero or random is taken and
dropped, as Linux takes it and busybox-w32's `> /dev/zero` does; it was refused.

A name under `/dev` that is none of these is not there, to an applet, a redirection,
`ls` and `test` alike -- `cat: cannot open '/dev/nosuch': No such file or directory`,
`cannot create /dev/nosuch: nonexistent directory` -- as busybox-w32 answers; it was
"unsupported device". `/dev` itself is a directory, so reading it is `Is a directory`.

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

An applet's options may follow its operands, as busybox's getopt lets them: `ls dir -l` is
`ls -l dir`, and `grep TODO *.go -n` numbers its matches. A `--` ends them, and so does the
first operand when `POSIXLY_CORRECT` is set. The applets busybox reads in order still do:
`xargs`, `tr` and `basename`, whose getopt strings begin with `+`, and `dirname`, `head`,
`stty` and `killall`, which read their own.

| Applet | Options implemented | Unknown option is |
| --- | --- | --- |
| `base64` | `-d -i -w`; wraps at 76 like GNU, `-w0` not at all. `-d` refuses a character outside the alphabet, `invalid input` and 1, as GNU's does, unless `-i`; busybox's skips it and decodes the rest | refused by name |
| `ar` | verbs `x p t r`, plus `-o -v`, and `c`, taken and ignored as busybox takes it; the long-name table is read, never written | refused by name |
| `ascii` | none; the character table, read down in eight columns | refused by name |
| `arch` | none; the same name `uname -m` gives | refused by name |
| `cal` | `-m -y`, and `[[MONTH] YEAR]`; September 1752 is short, as in every cal. `-j` is refused | read as a number, so a bad one is refused |
| `dd` | `if= of= bs= ibs= obs= count= skip= seek= conv= status=`; an unknown operand is refused | refused by name |
| `ed` | `-s -p`; addresses including `/re/` and marks, and `a i c d p n l = s g v m t j k r w e f q Q h H`. Follows GNU, not busybox | refused by name |
| `dc` | `-e -f -x`; the stack machine, registers as stacks, `[strings]` and the conditionals. `!` refused | refused by name |
| `bc` | the POSIX language; `-s -q -w`. `-l`, `read()` and an obase above 16 refused by name | refused by name |
| `df` | `-h -H -k -m -B SIZE -P -T -t TYPE`, `-a` taken; the last of `-k -m -B` counts, `POSIXLY_CORRECT` counts 512-byte blocks, and blocks round to nearest, all as busybox has them. A drive letter is a filesystem, mounted at its root, and `-T` names what Windows calls it, NTFS or FAT32; a FILE that is not there has no mount point, status 1, as busybox says | refused by name |
| `stty` | `size`, `echo`, `-echo`, `sane`, `-a`; everything else refused by name | refused by name |
| `getopt` | `-o -l -n -q -Q -u -a -T -s`, and the old form where the first operand is the option string | refused by name |
| `ipcalc` | `-b -n -m -p -h -s`, and busybox's long options; a non-contiguous netmask is used as given; with neither -b, -n nor -p, an -m or -h is wanted and no NETMASK, asked before the address as busybox asks it | refused by name |
| `groups` | none; one name, the one `id -gn` gives -- a user other than this one is refused | refused by name |
| `killall` | `-l -q`, and a leading `-SIGNAL`; the name is matched whole, not as a pattern | refused by name |
| `link` | none; an existing LINK is refused rather than replaced | refused by name |
| `logname` | none; the login account, which under elevation differs from `whoami` | refused by name |
| `less` | `-N -S -I -E -F -h`; `-M -m -R -~` accepted. With no terminal it is `cat` | refused by name |
| `nproc` | `--all --ignore=N`; the answer never drops below 1 | refused by name |
| `pidof` | `-s -o PID[,PID]`; the name is matched whole, not as a pattern | refused by name |
| `truncate` | `-s SIZE` with `K M G` and `KB MB GB`, a leading `+` or `-`, and `-c` | refused by name |
| `sync` | none; on Windows each volume is flushed, which takes an elevated session, as busybox-w32's does | ignored with every other argument, and that is said, as busybox's plain `sync` says it |
| `shred` | `-f -u -z -n N -s SIZE`, `-v -x` taken; random passes then zeros, each flushed, and `-s` past the end too. A FILE that cannot be opened ends it, as busybox's xopen does | refused by name |
| `fsync` | `-d`, which is the same here; on Windows a file that cannot be written passes unflushed, as busybox-w32's does | refused by name |
| `ts` | `-i -s`, and a strftime FORMAT operand | refused by name |
| `reset` | none; on a terminal busybox-w32's sequence, then `stty sane`, and nothing when its output is not one | not read, as busybox reads none |
| `pipe_progress` | none; the input passes through whole, a dot on stderr for each read that comes in a later second than the one before, and a newline at the end, as busybox's | not read, as busybox reads none |
| `ttysize` | none; `[w] [h]` operands, and `80 24` without a terminal, as busybox's | an operand like any other, which prints nothing |
| `unlink` | none; a directory is refused | refused by name |
| `usleep` | none; a microsecond count | read as a number, so a bad one is refused |
| `uuidgen` | none; a version 4 identifier from a cryptographic source | refused by name |
| `base32` | `-d -i -w`; wraps at 76 like `base64` | refused by name |
| `cksum` | none; `<crc> <size> <name>`, the POSIX CRC | refused by name |
| `crc32` | none; eight hex digits, the IEEE CRC | refused by name |
| `basename` | `-a -s`, and the `basename PATH [SUFFIX]` form, a third operand refused | refused by name |
| `bunzip2`, `bzcat` | `-c -d -f -k -t`, and `-` for standard input; **decompress only** | refused by name |
| `cat` | `-n -b -v -e -t -A`, `-u` taken and ignored | refused by name |
| `chmod` | `-R -c -v -f`; octal and symbolic modes, `u+x,go-w`, and options after the operands unless `POSIXLY_CORRECT` is set. On Windows only the owner's write bit is kept, as the read-only attribute | read as the MODE, as busybox reads `-w`, so `-Z` is an invalid mode |
| `clear` | none; it writes `ESC[H ESC[J`, as busybox-w32's does, and Ctrl-L at the prompt writes the same | refused by name |
| `cmp` | `-s -l -n`, `--bytes --quiet --silent --verbose`, FILE2 stdin when it is not given, and SKIP1 SKIP2 with K M G; `a b differ: byte 5, line 2` on stdout, and `cmp: EOF on FILE` on stderr, as busybox's editors/cmp.c has them; 0 the same, 1 different, 2 trouble | refused by name |
| `comm` | `-1 -2 -3` | refused by name |
| `expand` | `-t -i`, `--tabs --initial`, columns counted in the cells a terminal draws | refused by name |
| `ftpget` | `-u -p -P -v`; `-c` accepted, resuming is not implemented | refused by name |
| `ftpput` | `-u -p -P -v -c` | refused by name |
| `factor` | none; numbers from operands or stdin | refused by name |
| `fold` | `-w -s -b`, `-w` from 1 to 10000 as busybox's reads it; columns counted as busybox's adjust_column counts them, a tab to the next multiple of eight, a backspace back one, a carriage return to the start, and `-b` every byte one; a UTF-8 character is one column, where busybox-w32's build counts its bytes | refused by name |
| `free` | `-b -k -m -g -h`, read from the first argument alone as busybox reads it, values rounded to the nearest and `-h` in busybox's `63.7G`; busybox's columns | refused by name |
| `cpio` | `-t -i -o -d -m -v -u -0 -F -H`, and busybox's long options, `--quiet` and `--to-stdout` among them; only `newc` is read or written, and `-o` needs `-H newc` to write it; `-o` wins over `-t`, and `-t` over `-i`, as in busybox. `-H` takes `newc` spelled out, as GNU's does; busybox's takes its first letter, `-H n` | refused by name |
| `cp` | `-a -d -P -L -H -p -f -i -n -l -s -T -t -u -v -r -R`, and busybox's long forms; a file is never copied onto itself, and a destination that is there is replaced, read-only or not, as busybox-w32 replaces it. `-r` follows symbolic links, where busybox copies them | refused by name |
| `cut` | `-b -c -f -F -d -O -s -D -n` and `--output-delimiter`; `-c` counts bytes, as busybox's does, `-F` splits where an extended regular expression matches, and a `-d` of a newline cuts lines | refused by name |
| `date` | `-d -D -I -r -R -u` and busybox's long forms; TIME in every form busybox's parse_datestr reads, and `%N` in FORMAT, with glibc's flags and widths, `%-d %_H %^a %10Y`, and `%q`. Setting the clock, `-s` or a TIME operand, is refused | refused by name |
| `diff` | `-u -U -q -s -i -w -b -B -N -L -T -t -a -r -S`; unified always; two directories compared as busybox's diffdir does (`Only in`, `Common subdirectories`, `-r`, `-N`, `-S`), and a directory against a file as the file of that name in it; a last line without a newline is another line, and said to have none; files holding a NUL only `differ`, but under `-a`; `-w`, `-b` and `-B` read lines as busybox's read_token does, so `-w` passes over all white space, `-b` a change in how much of it there is, and `-B` a hunk of empty lines only | refused by name |
| `dirname` | none needed; one NAME, a second refused | refused by name |
| `dos2unix` | `-u -d`; converts **in place** with a file operand | refused by name |
| `du` | `-a -s -d -c -h -k -m -b -l -x -H -L`; what the filesystem allocated, in kilobytes, each directory printed after what it holds and in the order it lists them. A directory and a file with several links are counted once unless `-l`, and a junction is a link as a symbolic link is. Of `-h -k -m`, `-H -L` and `-s -d` the last wins | refused by name |
| `echo` | `-n -e` | treated as text, which is what `echo` does |
| `env` | `-i -0 -u` and a lone `-`, their long forms, and `NAME=VALUE command` (an applet) | refused by name |
| `expr` | none; every argument is a term | read as a term, so a bad one is a syntax error |
| `find` | `-name -iname -path -ipath -regex -type f\|d\|l\|c -size -mtime -atime -ctime -mmin -amin -cmin -newer -empty -perm -inum -samefile -links -executable -prune -quit -print -print0 -maxdepth -mindepth -depth -xdev -follow`, `-H -L -P` before the PATHs, and the operators `-a -o ! -not -and -or ( )` | refused **before the walk** |
| `grep`, `egrep`, `fgrep` | `-i -n -v -r -R -l -L -c -q -w -x -F -o -s -h -H -E -G -m -A -B -C -e -f`, `--color[=WHEN]` accepted and ignored. A pattern is a POSIX basic expression, with GNU's `\+ \? \| \w \s \b \< \>`, unless `-E`. `egrep` is `grep -E` and `fgrep` is `grep -F`, as in busybox | refused by name, and a backreference in a pattern |
| `gzip`, `gunzip`, `zcat` | `-c -d -f -k -t -1`..`-9`, and `-` for standard input. A FILE that is not compressed is said and the next one read, status 1 at the end, as GNU's gzip does; busybox's stops at it | refused by name |
| `hd`, `hexdump` | `-b -c -d -o -x -C -v -e -f -n -s`, each format added in the order given; `-e`'s units and every conversion busybox's dump takes, `%_a %_A %_c %_p %_u` among them; `hd` is `-C` first | refused by name |
| `httpd` | `-p -h -a -v`; `-f` accepted, this always runs in the foreground | refused by name |
| `head` | `-n -c -q -v`, the `-N` form, and an attached value (`-n2`); a count may end in `b`, `k` or `m`, for 512, 1024 or 1048576, as busybox's | refused by name |
| `id` | `-u -g -G -n -r`, and their clusters; `-r` is the same id, Windows having no real one apart, and alone is refused as busybox refuses it | refused by name |
| `install` | `-c -d -D -p -s -v -b -o -g -m -t` and busybox's long forms; the mode is 0755 or `-m`'s, whatever the umask. `-o` and `-g` take a number or a name, on Windows this session's account or root, and change nothing there, as busybox-w32's chown does not. `-s` runs strip, which is a program and no applet, so it is not found and the status is 1 | refused by name |
| `ln` | `-s -f -n -b -S -v -T`; `TARGET... DIR`, and a lone `TARGET` linked into the working directory | refused by name |
| `iconv` | `-f -t -l -c -o` | refused by name |
| `join` | `-a -v -e -o -t -1 -2 -j`; two files sorted on their join fields, merged a key at a time as busybox's are, so lines out of order are not paired | refused by name |
| `ls` | busybox-w32's: `-1AaCxdgLHRFplinshrSXvctuQqk`, `-w N` (0 for no limit), `-T N`, `--full-time`, `--group-directories-first`, `--color[=always\|never\|auto]`; of `-C -x -l` the last wins, columns are as wide as busybox's, `total` counts blocks, and a directory's links on Windows are two and its subdirectories. The mode column is made up as `stat`'s is, busybox-w32's: read and write for everyone less the umask's group and other write, and run for a directory or a program | refused by name |
| `micro` | `-H -R`; one file at a time | refused by name |
| `mkdir` | `-m -p -v` and their long forms; `-v` names each directory made, a parent with its slash | refused by name |
| `mktemp` | `-d -q -u`, and an `XXXXXX` template | refused by name |
| `mv` | `-f -i -n -T -t -v` and busybox's long forms; of `-f -i -n` the last wins, and a read-only destination is replaced as busybox-w32 replaces one | refused by name |
| `nano` | `-H -R`; one file at a time | refused by name |
| `nc` | `-l -p -w`; `-e` **refused by name** | refused by name |
| `nl` | `-b a\|t\|n\|pBRE -i -s -v -w -p` and their long forms; numbers carry on from one file to the next, and `pBRE` is GNU's | refused by name |
| `od` | `-a -b -c -d -D -f -h -H -i -I -l -L -o -O -B -s -x -X -v`, `-t` of every kind and size busybox's has (`d o u x` of 1 2 4 8 bytes or `C S I L`, `f` of 4 or 8 or `F D`, `a`, `c`, and `z` after), `-A -N -j -S -w`, their long forms, and `--traditional`'s OFFSET and LABEL; each type a line of its own, several in columns as GNU od lays them out | refused by name |
| `paste` | `-s -d`; the delimiter list cycles, its escapes read as busybox's (`\t`, `\n`, `\\`, and `\0` for no delimiter) | refused by name |
| `pgrep` | `-l -x -v -P PPID`, `-e` taken; a regular expression on the process name, which `-P` makes optional | refused by name |
| `pkill` | `-x -v -e -l -P PPID` and a leading `-SIG`; a regular expression on the process name, which `-P` makes optional. `-e` says `NAME killed (pid N)`, and `-l` lists the signals | refused by name |
| `patch` | `-R -u -p -i -N -E -f -g`, `--dry-run` and busybox's long forms; `[ORIGFILE [PATCHFILE]]`; a /dev/null side creates or empties a file, and without -p a name is its last component; no fuzz | refused by name |
| `posixpath` | none | treated as a path operand |
| `printenv` | none | treated as a variable name |
| `ps` | none; `PID PPID THR RSS TIME COMMAND` | refused by name |
| `top` | `-b -n N -d SEC -s COL -f TEXT -o COLS -H -t` | refused by name |
| `printf` | format string | treated as the format, which is correct |
| `pwd` | `-L -P` both accepted | accepted |
| `readlink` | `-n -f -v`, and `-s -q`, the quiet that is the default; `-f` prints what `realpath` does, and FILE need not exist if its directory does | refused by name |
| `rev` | none; reverses runes, not bytes | refused by name |
| `realpath` | none | treated as a path operand |
| `rm` | `-f -i -r -R -v`; of `-f` and `-i` the later wins, `.` and `..` are refused, and a read-only file is removed as busybox-w32 removes one | refused by name |
| `rmdir` | `-p -v`, `--ignore-fail-on-non-empty` and the other long forms; `-v` names each directory before it is removed | refused by name |
| `sha1sum`, `sha256sum`, `sha384sum`, `sha512sum` | `-b -c -s -t -w`, as `md5sum` | refused by name |
| `sha3sum` | `-a 224\|256\|384\|512` (default 224), `-b -c -s -t -w`, as `md5sum` | refused by name |
| `sum` | `-r` (BSD, the default), `-s` (System V) | refused by name |
| `shuf` | `-n -e -i -z -o`; `-o FILE` is opened once the input is read, so it may be the input, as busybox's is | refused by name |
| `strings` | `-n -t -o -a -f` | refused by name |
| `awk` | the POSIX language; `-F -v -f --`, and busybox's `-e PROGRAM` (read with any `-f` in the order given, as one program), `-E FILE` (`-f` that ends the options) and `-W` (said to be ignored); operands mixing files and `VAR=VALUE` | refused by name |
| `sed` | `s/// p d q y = a i c h H g G x n N P D b t T : {} r w l`, s's flags `g p N i w` (`Ng` is the Nth match and every one after it, as GNU's is, where busybox's replaces them all), addresses (`N`, `$`, `/re/`, ranges, busybox's `addr,+N`, GNU's `0,/re/`, a range the first line can end, `!`), `-n -e -E -r -f -i[SUFFIX]`, and busybox-w32's `-b`, which keeps a line's carriage return as part of it, so `sed -b -i` writes a CRLF file back as CRLF | refused by name |
| `seq` | `-w -s`, and `LAST`, `FIRST LAST`, `FIRST INCREMENT LAST` as strtod reads them, fractions included; a zero increment refused | refused by name |
| `sleep` | duration operand | reported as an invalid duration |
| `ssl_client` | `-s -h -n`; `-e` accepted; the certificate is always verified | refused by name |
| `sha256sum`, `md5sum` | `-b -c -s -t -w`, as busybox's: `-b` marks a name with `*`, and `-c` reads the two-space, the `*` and the one-space spellings, counts every line, and ends a list with a failure with `WARNING: N of M computed checksums did NOT match`; `-s` and `-w` need `-c` | refused by name |
| `sort` | busybox's `-n -g -h -M -V -u -c -s -z -b -r -d -f -i -o -k -t`, keys with character offsets and their own letters; `-m -S -T` taken and ignored | refused by name |
| `stat` | `-c -t -L -f`: busybox's layout, its terse line, and every letter of its `-c` and `-f` formats, each with printf's flags, width and precision. On Windows the values are busybox-w32's: the volume's serial number for the device, the file's index for the inode, 4095 for the account's own files and 0 for a system account's, the mode made up as `chmod` makes it up, and the creation time for the change time | refused by name |
| `split` | `-l -b -a`, `-b` with `b k m g`; the bytes as they come, and as many letters as `-a` asks, `aa` upwards by default | refused by name |
| `su` | `-c -s -t -W -N`; Windows only, see **Elevation** | refused by name |
| `lsattr` | `-R -a -d -l`; Windows only, see **`lsattr` and `chattr`** | refused by name |
| `chattr` | `-R`, and `-` or `+` with `r h s a t n`; Windows only, see **`lsattr` and `chattr`** | refused by name |
| `tac` | none | refused by name |
| `tsort` | none; its words are paired across lines and an odd one out refused, and a cycle is said, `cycle at NAME`, and broken, the rest written with status 1, as busybox's does; one FILE. Items with no order between them come out in the order they were first read, an order of its own: busybox and GNU each have theirs | refused by name |
| `tar` | `-c -t -x -v -z -j -a -O -f -C -k -m -o -h -T -X`, busybox's long options, `--exclude`, `--strip-components`, `--no-recursion` and `--overwrite` among them, and a first argument without a dash as its letters, `tar cf a.tar dir`, as busybox's; `-f` a device too; the FILEs name what is listed or extracted, see **The archivers** | refused by name |
| `tail` | `-n -c -q -v -f -F -s`, the `-N` form, `+N` as the first argument, and an attached value (`-n2`, `-n+2`); a count may end in `b`, `k` or `m`, for 512, 1024 or 1048576, as busybox's; every FILE opened before any is printed, headers counted from the ones that opened, as busybox's tail_main has it; `-f` reads a FILE from its start again when it shrinks, and `-F` follows one replaced by its name | refused by name |
| `test`, `[` | POSIX expressions; on Windows `-x` is busybox-w32's execute bit: a directory, a name ending `.com .exe .sh .bat .cmd`, or a file that begins `#!` or is a program image, and not a DLL whatever it is called | an operand, per the POSIX one-argument rule |
| `tee` | `-a -i`; `-` is stdout, and a file that cannot be opened is named while the rest are written | refused by name |
| `touch` | `-a -c -d -f -h -m -r -t` and busybox's long forms; DATE is read as `date -d` reads it | refused by name |
| `tr` | `-d -s -c`, ranges, backslash escapes and the POSIX classes (`[:upper:]` and the rest, in code order). One operand without `-d` or `-s` is `missing operand`, 1, as GNU's is; busybox's copies its input as it is | `[=c=]` and `[c*n]` read as the characters written, as busybox reads them; a class name it does not know is refused by name |
| `true`, `false` | none, by definition | ignored, which POSIX requires |
| `uname` | `-a -i -m -n -o -p -r -s -v` | refused by name |
| `uniq` | `-c -d -u -i -z -f -s -w`, and an OUTPUT operand | refused by name |
| `unexpand` | `-t -a -f`, `--tabs --all --first-only`; busybox's expand.c: -t converts throughout unless -f, and a line that begins with a word has the run after it converted too | refused by name |
| `unix2dos` | `-d -u`; converts **in place** with a file operand | refused by name |
| `unzip` | `-l -v -t -p -j -n -o -q -K -d DIR -x`, an option anywhere, and after `-x` the members left out, as busybox reads them | refused by name |
| `uudecode` | `-o`; `-o -` writes to stdout | refused by name |
| `uuencode` | `[FILE] NAME`, and `-m` for busybox's `begin-base64` form, which `uudecode` reads back | refused by name |
| `wget` | `-O -P -U -T -q -S --header --spider`; `-c` and `-o` accepted | refused by name |
| `wc` | `-c -l -w -m -L` | refused by name |
| `whois` | `-h -p`; `-i` accepted | refused by name |
| `whoami` | none | refused by name |
| `winpath` | none | treated as a path operand |
| `xargs` | `-0 -a -E -e -I -i -n -P -p -r -s -t -x` and `--no-run-if-empty`, busybox's quoting of words, `-I` reading lines, `-P` running applets side by side, and busybox's statuses: 123, 124, 127 | refused by name |
| `xxd` | `-a -c -g -i -l -o -p -r -s`, busybox's formats over libbb's dump, `-i`'s C array, and `-r` with and without `-p`, seeking where stdout is a file and writing zeros up to an address elsewhere, as busybox does (busybox-w32's seek on a pipe succeeds without moving, and loses the gap) | refused by name |
| `yes` | none | treated as the string to repeat |

The six most recently added -- `tac`, `rev`, `nl`, `base64`, `sha256sum`,
`md5sum` -- were measured against **GNU coreutils**, not busybox: busybox's are
the small versions, and the behaviour people rely on, including the checksum
format printed in every release note, is GNU's. Each carries the observed output
in its test table. The checksum format is busybox's as well, and `-c`, `-s` and
`-b` have since been measured against busybox-w32 and follow it.

Three of these diverge from GNU on purpose, and say so where it matters:

- **`du` counts apparent sizes**, rounded up to a 1024-byte block, where GNU
  counts what the filesystem allocated. The two differ in both directions: a
  3000-byte file occupies 4096 on NTFS, and a 3-byte one may occupy nothing
  because it fits in the MFT record. Measured on one tree, GNU said 5 and this
  says 6. Go cannot read allocation size portably, and a `du` that silently means
  something slightly different from the one in a script is worse than one that is
  documented to mean apparent size. GNU spells this `--apparent-size`.
- **`ps` prints `PID` and `COMMAND` and nothing else.** No TTY, no STAT, no TIME,
  not the command line: Windows has no controlling terminal in the POSIX sense,
  and reading another process's command line means walking its PEB, which an
  ordinary session may not do for anything it does not own. A column of `?` per
  row would be worse than no column.

### Line endings, and which applets keep them

**The shell reads a Windows program's CRLF as a newline**, as busybox-w32 does: a
command substitution's trailing newlines go with the CR of each CRLF among them --
bash on Windows does that too -- so `v=$(cmd /c ver)` ends where its text does, and
a field split from an unquoted expansion ends before a CRLF's CR, so `set -- $(where
git)` has no CR on its words. A CR anywhere else is a character, and on Linux and
macOS every CR is one, as busybox and bash have it there.

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

**`sed`'s rule is per command, not per line**, which is why it has its own writer,
busybox's puts_maybe_newline. The pattern space printed at the end of a cycle, and by
`n`, `q`, `s///p` and `w`, ends as its input line did; `p`, `P`, `=`, `i`, `c`, `a` and
`r`'s lines always end theirs; `G` and `x` make the pattern space an ended line. A
newline an unterminated line did not get is paid before anything else is written:

```console
$ printf 'a\nb' | sed p
a
a
b
b      <- no newline here: the printed b ends as it came, p's b above ends
$ printf 'a\nb' | sed -n p
a
b
       <- p ends its line
```

`sed 2d` on `a\nb` deletes the second line, so the last output came from the first,
which *was* terminated, and both references answer `a\n`. The writer held every
newline back until the next write and forgave the last when the input's last line
had none: the printed pattern space came out the same, but `sed -n p` lost the
newline busybox ends the `b` with, and two outputs sharing a destination, as
`w /dev/stdout` shares the standard output's, would put two lines on one. busybox's `=` is not followed where it prints past an owed newline:
after a FILE whose last line had none, `sed = f1 f2` runs that line into the next
number there.

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

Both of these were carried as outstanding for the same reason -- they would have to
choose an output encoding for a file they rewrite -- and both are **done as of
2026-08-27**. `iconv` had already settled the question on 2026-08-22 and nobody went back
to collect: an encoding is named, never guessed. The name here is the file's own
byte-order mark, which is the writer stating what it wrote, so re-encoding to it is not a
guess either.

- **`sed` decodes, and `-i` writes the same encoding back.** It used to match nothing on a
  UTF-16 file and copy it through, because a regular expression cannot match across UTF-16
  code units. It now substitutes, and `sed -i` puts the file back as UTF-16LE or UTF-16BE
  with its mark, so a file Notepad wrote is still a file Notepad can open. Printed output
  is UTF-8, the rule `grep` already follows. A file with **no** mark is not decoded and so
  is not re-encoded -- `encodingBytes` has the identity for both directions, which keeps
  the byte-exact path exact rather than merely equivalent.

  (The older note here is still true and still worth keeping: until 2026-08-23 `sed -i`
  over a UTF-16 file *appended a byte*, because such a file's last byte is a NUL rather
  than a newline and every line-oriented filter terminated its output unconditionally. A
  24-byte file came back as 25.)
- **`wc -m` counts characters and `-c` still counts bytes.** The objection was that one
  pass cannot honestly do both. It can: the raw bytes are tallied on the way *into* the
  decoder, so `-c` comes from the tally and `-m`, `-l`, `-w` and `-L` from the decoded
  text, which is the only view in which they mean anything.

  A byte-order mark is not a character. `-c` counts its bytes and `-m` does not count it
  as one, which is the same rule `grep` follows when it strips a mark rather than trying
  to match `^` after it. GNU counts it; this is a deliberate divergence, and the reason is
  that a mark is the file saying what it is rather than part of what it says.

Measured against both references on a 26-byte UTF-16LE file holding `hello
world
`:

| | lines | words | chars | bytes |
| --- | --- | --- | --- | --- |
| nemosh | 2 | 2 | **12** | 26 |
| busybox-w32 | 2 | 2 | 26 | 26 |
| GNU coreutils | 2 | 2 | 26 | 26 |

Twelve is the number of characters in the file. The references say 26 because they do not
decode; on a plain ASCII file all three agree exactly, which is the property that matters
for every script that already exists. `busybox sed -i s/hello/goodbye/` over the same file
leaves it unchanged; this one substitutes and keeps the mark.

busybox-w32 reads none of these, so this is a feature beyond the reference rather
than a divergence from it.

### Options a script is most likely to reach for and not find

The list used to be `xargs -0`, `xargs -n`, `sort -k`, `grep -r` and `tail -c`.
All five are implemented, measured against GNU. What is still absent:

- **`ls -i -n -u -c`.** `-i` wants an inode number Windows does not keep, `-n` a
  numeric owner this build does not resolve, and `-u`/`-c` the access and change
  times, which NTFS records but which no sort here reads yet. `-t -S -r -R -d -F
  -A` landed on 2026-08-22, so `ls -ltr` works.
- **Nothing of `sed`.** This bullet listed `-i`, `-f`, `a i c y` and the hold space
  as absent and was simply stale: all of them landed on 2026-08-22 along with `{}`
  blocks, the multiline commands and branching. Measured against the built binary
  rather than trusted.
- **`grep --include`, `--exclude` and `-z`.** The first two need a name filter
  threaded through the `-r` walk; `-z` is a NUL-terminated-line mode. `-A -B -C
  -e -f -L` landed on 2026-08-22.

Every one of them is refused by name, so a script asking for it fails rather than
quietly getting something else.

Two deliberate near-misses worth naming:

- **`grep` reads a basic expression unless `-E`**, through the translation sed
  uses (sed_regex.go). It read every pattern as extended, so `grep 'a+b'`, `grep
  'x|y'` and `grep '('` all meant something else and `\(a\)` matched nothing. A
  backreference inside a pattern is refused by name: Go's regexp is RE2, which has
  none. `\<` and `\>` are read as `\b`, which is what they are at the edge of a
  word.
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
`-atime`, `-ctime`, `-mmin`, `-amin`, `-cmin`, `-newer`, `-empty`, `-perm`, `-inum`,
`-samefile`, `-links`, `-executable`. The time tests hold the age in whole seconds
against N days or minutes, as busybox's time_cmp does; the change time is the
creation time on Windows, which keeps no change time. `-perm`, `-inum`, `-samefile`
and `-links` ask what `stat` says: busybox-w32's made-up mode, the volume serial and
the file index, the count of hard links. `-executable` is `test -x`'s judgement.
`-name` matches the basename, not the path, because busybox
uses `fnmatch` without `FNM_PATHNAME` and a basename carries no separator for
`*` to cross; `-path` matches the whole path *with* the separator crossable, for
the same reason in reverse — which is why `-name` uses Go's `path.Match` and
`-path` cannot, since `path.Match` hard-codes a non-crossing `*`.
`-type` classifies `f`, `d`, `l`, and `c`; busybox also accepts `b`, `s`, and
`p`, refused by name here rather than answered as though a block device could
never match.

**Actions.** `-print`, `-print0`, `-quit`, `-exec`, `-ok` and `-delete`. An action
anywhere suppresses the implicit `-print`, which is what stops `find . -name x
-print` printing twice. `-exec CMD ARGS ;` runs CMD once for each entry, every `{}`
in ARGS that entry's path, and is true when CMD ends 0; `-exec CMD ARGS {} +` runs
it with the entries gathered, as many as a 30720-byte command line holds, the word
with the `{}` once for each, and a failure of the last run is find's status 1. `-ok`
asks first, the command line and `?` on stderr, an answer beginning `y` on stdin.
CMD is an applet, as `xargs`'s and `env`'s is: nothing outside the shell is run, and
a name no applet has is `find: CMD: No such file or directory` and false. `-delete`
removes an entry -- a directory only when it is empty, and never `.` -- and walks a
directory's entries before it, as `-depth` does; a failure is said and find goes on,
status 0, as busybox's does.

**Global options.** `-maxdepth` and `-mindepth`, which bound the traversal rather
than filter it: `-maxdepth 1` stops the walk from *reading* a subdirectory
instead of reading it and discarding the entries. `-depth` walks a directory's
entries before the directory itself, and `-xdev` goes into no directory on a volume
none of the PATHs is on, the volume serial number busybox-w32's stat reports. `-L` and
`-follow` follow every symbolic link and `-H` the PATHs alone, as busybox stats them; a
link back to a directory above is not gone into again, where busybox goes round until
the path is too long.

`-prune` is true and keeps the walk out of a directory it is true of, and `-regex`
matches the whole path against a basic regular expression, both as busybox's.

Still **refused before the first directory is read**: `-user`, `-group`, `-fstype`,
and the rest busybox's find has not got either.

```console
$ find . -fstype ntfs
find: unrecognized: -fstype
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

The rest is busybox's: `patching file F`, `creating F` and `removing F` go to
stdout; a file whose old side is /dev/null, or dated 1970 as `diff -N` dates one,
is created with its directories, and one whose new side is /dev/null is emptied,
or removed under -E; the file is the one `+++` names, `---`'s under -R; `\ No
newline at end of file` is honoured either way; and when a file's hunk fails the
next file is patched still, with the status 1 at the end. Two things are not
busybox's: its patch searches forward for a hunk's context, and it answers 0 to
input that holds no diff at all, which is refused here as GNU refuses it. An
empty patch changes nothing and succeeds.

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

`od` became busybox's od_bloaty on 2026-09-30: every `-t` kind and size, the letters in
busybox's fixed order, `-N -j -S -w`, the long forms and `--traditional`. Across about 200
invocations it matches busybox-w32 byte for byte, apart from six places that are chosen:

- **Several types stand in columns, as GNU od lays them out.** `od -A n -t c -t x1` puts
  each byte's number under its character. Oils records that layout, and scripts written
  on Linux expect it. busybox leaves each line as narrow as its own fields.
- **Floats are spelled as C spells them**: `1.0000000e+00` and `inf`. busybox-w32's msvcrt
  gives `1.0000000e+000` and `1.#INF000e+000`.
- **A size may be followed by another type, or by `z`**: `-t x1z`, `-t x1c`. busybox
  reads the size with bb_strtou, which refuses a letter after the digits and reports a
  4294967295-byte type, although its own comment gives `d4afL` as a string it reads.
- **A `-S` run that `-N` cuts off is printed at the address where it begins.** busybox
  and GNU both print the address one byte earlier, which for a run starting at offset 0
  is `1777777777777777777777`.
- **`-t fL` is refused.** A long double is ten bytes held in sixteen, and busybox-w32
  prints it through msvcrt as if it were a double.
- **`-A ''` is refused**, where busybox reads past the end of its radix table.

`hexdump` and `hd` became libbb's dump on 2026-09-30. Each of `-b -c -d -o -x -C` adds its
format in the order given, `-e` adds units of busybox's own format language, and `-f` adds each
line of a file as one. It matches busybox-w32 byte for byte on about 150 invocations, and on
10 MB of random bytes under `-C` (it takes 1.6 s against busybox's 2.2), apart from four choices:

- **`%s` stops at its byte count.** busybox passes the datum to printf without a precision, so
  `2/3 "%s"` prints until it finds a NUL, somewhere past the block.
- **Floats are spelled as C spells them**, as od's are.
- **An `-s` OFFSET exactly the FILE's length skips the FILE**, as util-linux's hexdump now does.
  busybox's old `>=` test dumps the whole FILE, at address OFFSET. On a pipe, busybox-w32 seeks,
  does not move, and dumps from byte 0 under OFFSET's address; this reads OFFSET bytes past.
- **`-f FILE` names one it cannot open** `cannot open 'FILE'`, where busybox writes `can't`.

Both dumps read a FILE a buffer at a time, and stdin no further than `-N` or `-n` goes, as
busybox's od turns its buffering off for: `{ od -N 4; cat; } < f` leaves `cat` the rest. They
write a buffer at a time and write it out before each FILE is opened. A missing FILE is named
after the lines that came before it, as busybox's are on a terminal.

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
absences (no multiple buffers, no replace, no mouse, no configuration file, no soft
wrap), because an editor that silently lacks replace is worse than one that says so,
and it names the languages it highlights for the same reason.

### Syntax highlighting, and why it is written here

Added 2026-08-24 for eleven languages: **Go, C, C++, Python, shell, Haskell, Prolog,
JSON, YAML, TOML and Markdown**. `nano -H` lists them, generated from the tables so the
list cannot claim a language with no rules.

**The alternative was measured before it was rejected.** micro's `pkg/highlight` is
importable -- three files, MIT, and no tcell at all, which corrects what the editor's
own plan assumed -- and inside this binary it costs 283 KiB plus a 44 KiB YAML corpus.
The 14 MiB ceiling has room for that. What settled it was the *other* ceiling:
`gopkg.in/yaml.v2` does **262 allocations in its own package init**, which would take
the total from 2275 to 2545 against a limit of 2750 and leave 205 for everything after.

Generating Go tables from micro's YAML at build time, keeping its rules and dropping
its parser, is not possible: `Def.rules` and the `rules`, `region` and `pattern` types
are all unexported, so a `Def` can only be built by `ParseDef`, which needs the YAML.

Measured after: **114 KiB and no init allocations at all** -- 2275, unchanged, because
the tables compile behind a `sync.Once` on first use. A third of A's size for none of
its startup cost.

**Wrapping is off, and that is a requirement rather than a preference.** tview's
TextArea has one `SetTextStyle` for all of its text and no per-run styling, so
highlighting happens *after* `Draw` by re-colouring cells -- which needs a screen-cell
to buffer-line map. With wrapping off, `line = rowOffset + row`. With it on, a screen
row is a *display* row and tview's line-start table is unexported, so there is no
mapping at all. The feasibility test that guards this includes the negative control.
micro does not wrap either.

Two things that would fail silently and so are asserted on a drawn frame:

- **The advance is `uniseg`'s**, the same package and rule tview uses: cluster width
  from `boundaries >> ShiftWidth`, except a tab, which is a flat `TabSize` and *not* an
  alignment to the next stop. Anything else and the colours slip a column on every
  indented line, or on every line after a CJK character -- which on this platform is a
  common line. uniseg was already linked, so using it directly cost a line in `go.mod`
  and no bytes.
- **A cell the widget styled differently is left alone**, read back with `Get`. That is
  how the selection and the cursor stay visible without this code knowing either
  exists.

Two engine decisions worth knowing. A **line comment is a pattern, not a region**: as a
region ending at `^$` it never closed, because the scan stops at the end of the line
before an empty match can happen, so the comment carried into every following line.
Measured before the tables were written. And **regions nest where the language says
so** -- Haskell's `{- {- -} -}` is one comment and C's `/* /* */` is not.

The two languages with a trap, both of which had a sample file written before their
rules. Haskell's character literal is a bounded pattern rather than a region, because a
prime is a legal identifier character and `f x' = x'` would otherwise open a string and
swallow the file; `{-#` is a pragma and not a comment. Prolog's `0'a` is the integer for
`a`, and the quote in it is not a delimiter -- it works because the scan is left to
right and the number rule matches at the `0`, one character before a quoted-atom region
could start.

**Go's regexp is RE2 and has no lookahead**, which the YAML and TOML key rules found by
panicking at `MustCompile`. They match the delimiter too, which reads fine; the
imperfection that leaves is that in `msg: hello: world` the second `hello:` also matches,
and telling them apart needs a parser rather than a rule.

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
- **The editor uses the width it is given**, which it did not until 2026-08-24 and which
was reported from a real terminal rather than found by a test. Two things were wrong and
neither was visible at 80 columns -- the one width the harness runs at.

The key legend was laid out by `footer(80)` **exactly once**, when the view was built, so
a wider terminal kept the 80-column answer: two rows of 61 characters with everything to
the right of them empty. It is now laid out for the width it is actually given, asked
again whenever that changes. Because the row count depends on the width -- seven labels
are two rows at 80 columns and one at 120 -- the layout resizes the legend's row to
match, which happens inside a draw function and therefore settles on the second frame. A
test asserts that settling rather than leaving the one blank row a mystery.

Stretching the columns to fill the width was tried first and looked wrong: seven labels
across two hundred columns left fifty blank characters between each one, which reads as
a bug. Compact columns and more of them per row is what nano does, and a compact legend
can only be as wide as its labels -- 109 characters for seven of them, which is the
honest ceiling.

And the title is now a **bar**, with a background across the full width, which is what
nano has. Before it was twenty characters of text on an otherwise blank line, so nothing
marked the top of the window as the editor's at all -- and that, rather than the legend,
is most of why the window looked like it was not using the terminal.

**Line numbers are on by default**, in a gutter down the left. nano needs `-l` for
this and micro does not; here both have it, for the same reason this editor highlights
by default where nano needs a config file to -- a default that differs from the original
in what it *shows* costs nobody their muscle memory, unlike one that differs in what a
key does. `-l` is accepted and does nothing, so asking for what is already on is not an
error.

The gutter is drawn by the widget that draws the text, not by a second widget beside it.
That is not a style preference: a `TextView` in a Flex would need the text area's scroll
offset copied into it every frame and, sitting to its left, would read that offset
*before* the area had clamped it -- so a fast scroll would show numbers a row out from
their lines. Drawing it inside means the offset is read after `TextArea.Draw` has run,
which is the one moment it is known to be settled. The width follows the line count
(three columns at 99 lines, four at 999) and the gutter is dropped entirely rather than
squeezing the text below twenty columns.

**Both spellings of every key are accepted.** `^_` did nothing on a real Windows
keyboard, and tcell explains why: there is no VT screen on Windows, so input goes through
the console API, and for a control character with Ctrl held tcell adds `0x60` back and
posts a *rune with `ModCtrl`* rather than a `Key` constant
(`console_win.go:725-736`). Which spelling arrives for a given physical key is a property
of the console. So each binding lists both, go-to-line answers to `^/`, `^_`, `^-` and
`M-G` -- nano's own help offers `^/` beside `^_` for the same reason -- and the footer
leads with the one that works rather than the one that reads better. Shift is ignored
when matching, because `_` needs Shift on this keyboard and `/` does not: that is a fact
about the layout, not about the binding.

**Go to line needs two Key constants**, because tcell has two input paths that disagree
about which constant a control chord is. Measured, after two wrong guesses:

- On a terminal, `input.go:450-452` posts `KeyCtrlSpace+Key(r)` for a control byte.
  `KeyCtrlSpace` is 64, so `0x1F` is `Key(95)` -- `KeyCtrlUnderscore`, exactly as named.
  The original binding was right here, and `^_` has always worked on a terminal.
- On Windows there is no VT screen, so `console_win.go` handles input, and for a control
  character whose modifier mask is *exactly* Ctrl it adds `0x60` back (`:725-736`).
  `^_` typed as Ctrl+`-` becomes `0x7F`, which tcell reads as Backspace -- so it deleted a
  character rather than doing nothing. Typed as **Ctrl+Shift+`-`** the mask is Ctrl|Shift,
  the addition is skipped, and it arrives as `Key(31)` -- `KeyUS`.

Both are bound now, so `^_` works on both. Letters never had this problem: `key.go:276`
maps `a`-`z` with `ModCtrl` onto `KeyCtrlA+n`, the same 64-based numbering, so the two
paths agree for them. Punctuation has no such mapping, which is why this binding and no
other was broken. `Esc` then `G` is bound as well -- it is what `M-` means on a terminal,
tcell already implements it there (`input.go:104-110`), and it is the only meta spelling
that survives the Windows path, which reports Alt with a letter as no character at all and
drops the event (`:741-744`).

Ctrl+Backspace is deliberately not bound. It is what Ctrl+`-` degenerates into on Windows,
but it is also a real editing key, and taking it to rescue a chord the console has already
mangled would cost more than it gains.

The test that missed this pressed `KeyCtrlUnderscore`, the same constant the code bound --
it proved the code equalled itself. It presses both constants now, and
`TestTcell_theTwoInputPathsNumberControlKeysDifferently` pins the arithmetic so a tcell
upgrade that renumbers either block fails loudly.

**`^G` is a panel, not a row.** It used to write the key list onto the message row,
immediately above the legend that already shows the key list, so help drew a second row
of key names under the first. `top` made the same mistake and `c5a1a44` fixed it the same
way: the keys are already on screen, so what a legend owes the reader is what is *not* --
the absences, with reasons, and which language the buffer was lexed as.

The panel also carries a **key reader**: any key pressed while it is open is named rather
than acted on, in the terms the binding table matches (`Key` constant against rune, and
the modifier mask). That is not a debugging affordance left in by accident. Two attempts
to guess what this console sends for `^_` were both wrong, and a terminal that can be
asked turns the next such question into a measurement.

**Replace is implemented**, and `-H` no longer lists it as absent. nano binds it to `^\`
and micro to `^R`; every match is confirmed with `y`/`n`/`a`/`q`.

That confirmation is why it was deferred. The prompt the editor already had collects a
line and fires on Enter, which is right for "what shall I search for" and wrong for "shall
I replace this one" -- pressing Enter after every `y` through forty matches is not a
feature. So there are two prompt kinds now, and the single-key one is checked *first*:
while a confirmation is open every key is an answer to it, including the letters that are
otherwise bindings. Without that ordering, answering a replace could quit the editor.

**The scan starts at the top of the buffer**, where nano starts at the cursor and wraps.
Starting at the top is the answer to "fix every occurrence in this file", which is what
replace is nearly always for, and it has no wrap condition to get wrong -- no question of
whether the run has come back round to where it began, and no way to be left wondering
whether some were missed.

Replacing a string with one that contains it terminates: the scan steps past what it
*wrote*, not past what it matched, so `a` to `aa` does not find the `a` it just produced.
An unrecognised key asks again rather than guessing -- guessing `n` would be safe and
guessing `y` would not, and a prompt that silently treats every stray key as "no" is one
people learn to distrust.

**Rewriting the buffer used to delete the file's last newline.** `^K` and `^U` rebuilt it
with `strings.Join`, which is not the inverse of the line split: the split drops the empty
element a terminating newline produces, correctly, and Join has no way to know it was ever
there. So cutting a line from a file that ended in a newline and saving wrote it back one
byte short. Found while writing replace, which rebuilds the buffer the same way and would
have inherited it. There is one helper for this now rather than the rule repeated at each
call site, because the next thing that rewrites the buffer would forget it otherwise.

**A terminal is required, and merely having a file on stdin is not enough.**
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

`tar` reuses this build's own gzip, so `tar -czf` needs no second program. It writes
no other compression: `-cj`, and `-ca` with a name that ends in bz2, xz or lzma, are
refused before the archive is opened, as Go has no bzip2 writer. Under `-a` a name
that ends in gz is gzip, a `.tgz` as a `.tar.gz`, as busybox reads the name.

**What `tar` takes**, as busybox selects it. Listing and extracting take the
FILEs named and what is under them, each a pattern matched against as many
leading components of an entry's name as the pattern has: `src/sub` takes
`src/sub/b.log`, and `src/*.txt` takes `src/a.txt`. `--exclude` and `-X` are
matched the same way, so `*.log` there matches a first component only.
`--strip-components` shortens the names as they are written; the FILEs select by
the names the archive holds, and `tar -t` lists them whole. Creating leaves out
what an exclusion matches at the start of any component. A file already there is
removed and written anew, so a link there is replaced rather than written
through; `--overwrite` writes into it, and `-k` stops the extraction at it.
Modification times are restored but under `-m`.

A name `tar -c` cannot store -- one that is not there, a file or a directory
that cannot be read -- is said and passed over, the rest stored, and the status
is 1 after busybox's closing words, `tar: error exit delayed from previous
errors`. The archive being written is not stored in itself.

A name is stored as it is given, and what is under a directory is joined to it
as busybox joins it, so `tar cf a.tar .` holds `./f.txt`. What busybox takes off
the front of a name -- slashes, a leading `../`, everything up to the last
`/../` -- is taken off, and said the first time: `tar: removing leading '../'
from member names`. On Windows a drive is taken off too, which busybox-w32
keeps, though no tar extracts `C:/x/f.txt` where it says.

`-C` is where creating finds the names, wherever it stands among the
arguments, as busybox changes to it once the archive is open; the archive and
`-T`'s list are found where tar began. A `-C` that is not there is refused
before the archive is opened, where busybox has already emptied it.

`-v` names each entry, and a second `-v`, or `-t` with one, gives busybox's long
line: the mode as `ls -l` has it, the owner and group, the size, the local time,
the name, and where a link points. The names go to stdout, but to stderr where
stdout carries the archive or, under `-O`, the data. Two answers are not
busybox-w32's. Its build lists the owner and group by number even where the
archive names them; busybox lists the names by default, and so does this. And
under `-O` busybox mixes the names into the data.

Two answers are not busybox's. A FILE that took nothing is said and fails the
command; busybox asks instead whether an entry it took is *spelled* like the
FILE, so `tar xf a.tar 'src/*.txt'` fails there having extracted what it
matched, and only the first such FILE is named. And a directory standing where a
file is extracted is `Is a directory` here, as busybox says on Linux, where
busybox-w32 says `Permission denied`.
`unzip` requires a file operand and says why: zip keeps its central directory at
the *end* of the file, so it cannot be read from a pipe. With neither `-o` nor
`-n`, an existing file is left alone and said so -- busybox prompts, and there is
no prompt here.

What `unzip` prints is busybox's: `Archive:  NAME`, then `   creating: DIR/` and
`  inflating: NAME` on standard output, and `-l`'s and `-v`'s tables column for
column, the archive's DOS dates month first. The total counts the entries listed,
where busybox's counts every entry in the archive. NAME may leave out its `.zip`,
`-d` makes one level and then must change into it, and a pattern is fnmatch's,
which a `*` crosses a slash in.

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
- **What it says is busybox's**: reading an archive ends with `N blocks` on stderr,
  unless `--quiet`, and writing one says nothing. A pattern is fnmatch's, so a `*`
  crosses a slash, and `-i --to-stdout` writes each file's data to stdout instead.
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
- **A FILE is taken in busybox's order**: stat'ed, then opened, then named, and the
  companion made only where nothing is -- `cannot open 'FILE.gz': File exists`. So
  `gzip f` a second time says f is not there, which is so, and a directory is
  `cannot open 'd': Is a directory`, where busybox-w32 says `Permission denied`.
  A FILE named `-` is standard input, to standard output.
- **The companion has the original's permissions**, less the umask's, so a private
  file's archive is private too; and `gunzip` gives the file it writes the time
  the data holds, the last member's, as busybox's does.
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

Each of the five says what busybox's says of data it cannot read, in its own name and
without the FILE's: `gunzip: invalid magic`, `corrupted data`, `unexpected end of
file`, `crc error`. `zcat` reads bzip2 as well as gzip, as busybox's does, the data's
first bytes choosing, and anything else is `no gzip/bzip2/xz magic`; xz it cannot
read. What follows a gzip member or a bzip2 stream, and is not another, is passed
over. Where busybox's `bunzip2` gives its decoder's return code, `bunzip error -3`,
this says what its `gunzip` says of the same fault.

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
- **A name is IANA's, or busybox-w32's**: `ASCII`, and a Windows code page by its
  number, `CP1252` for windows-1252 and `CP936` for GBK, for each code page there is
  a codec for here.

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
- **`join` is busybox's merge join.** Each file is read a set at a time, the
  lines in a row that share a key, so the files must be sorted on their join
  fields and lines out of order are not paired, as in busybox and POSIX; a loop
  over every pair of lines paired them. `-1` and `-2` are per-file fields: `join
  -1 2 -2 1` joins the second field of the first file to the first of the
  second. `-j` sets both; it is GNU's and busybox does not have it, offered
  because refusing a standard option is the worse divergence. An `-o` list of
  nothing but `0`s prints the key once for each, where busybox reads past the
  list and prints it more.
- **`ascii`'s column spacing is busybox's hand-tuned layout** — the gaps are 11,
  11, 9, 9, 9, 10, 10 characters, so it cannot come from one format string. The
  table is read *down*: the first column is 0–15, not 0,1,2 across. Reading it
  across is the obvious implementation and gives a completely different table.

`expand` and `unexpand` count the **cells a terminal draws**, as busybox's
unicode_strwidth does, so a tab after CJK text lands where it looks like it should;
busybox-w32 counts bytes, its unicode support being off. `fold` counts **runes**, so a
wrapped line is never cut through a character.
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

**`sum` omits the file name for a single operand but keeps the blank before
it**, `36979     1 `, as busybox's one format prints it with an empty name; GNU
leaves the blank out, and so did this. System V's `-s` always names what it read,
standard input as `-`, and `-r` wins over `-s` whichever comes first. A `-` named
is printed as `-` by `sum`, `cksum` and `crc32` alike; only standard input read
for want of an operand goes without a name.

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

### Who owns the terminal, and for how long

The line editor needs raw mode to see arrows and Ctrl-R. A command needs the opposite: echo
on, lines assembled, and Ctrl-C turned into an interrupt. **The editor borrows the terminal
for one line read and gives it straight back**, so a command run between two prompts finds
the terminal it expects.

Held across a command -- which it was until this was fixed -- a terminal reader gets no echo,
never sees a line at all, because Enter arrives as a carriage return with no line discipline
to translate it, and cannot be interrupted, because `os.Interrupt` on Windows is delivered by
the console only while `ENABLE_PROCESSED_INPUT` is set. `bc` at the prompt looked frozen, and
so would `dc`, `ed`, a bare `cat`, and the shell's own `read` and `select`.

Four places change console state, and a test lists them:

| where | for how long |
| --- | --- |
| the line editor | one line read |
| `--hold` | one key at exit, in the same function |
| virtual-terminal output mode | the whole session, deliberately: commands want colour too |
| `stty` | until told otherwise, because somebody asked |

Everything that reads keys -- `nano`, `micro`, `top`, `less` -- goes through tcell, which
saves the mode it finds and restores it on the way out.

### Streaming, and what waits for the end of its input

An applet that reads a stream either answers as lines arrive or gathers everything first, and
which one it is decides whether `tail -f log | ...` shows anything before the log stops
growing. Measured against busybox-w32 by feeding one line down a pipe and holding it open.

**These answer as the line arrives**: `cat`, `grep`, `awk`, `tr`, `nl`, `cut`, `rev`, `tee`,
`fold`, `expand`, `unexpand`, `ts`, `head`, `factor`, `iconv`, and the interactive `bc`, `dc`
and `ed`.

**These gather the whole input first**, and so do busybox's: `sed`, `uniq`, `sort`, `tac`,
`shuf`, `wc`, `tail`, `tsort`, `split`, `strings`, `base64`, `base32`, `od`, `hexdump`,
`xxd`, `uuencode` and the checksums. Some cannot do otherwise -- a digest has no partial
answer, and `tac` needs the end before it can start -- and the rest match the reference,
which block-buffers to a pipe the way C stdio does.

Two places this build differs from a reference, both deliberate:

- **`grep` streams where busybox's waits.** Better, and kept.
- **`awk` and `tr` stream where gawk buffers**, following busybox instead. awk flushes once
  per record, and tr once per newline, *unless* the output is a regular file -- nobody is
  watching a file being written, and a write per line there would be paid for nothing.

**A byte-order mark is looked for only in what has already arrived.** Reading three bytes to
check for one would block a stream whose first write is shorter, which is what typing a
single character and pressing Enter is.

### `awk`

The POSIX language: patterns and actions, `BEGIN`/`END`, ranges, fields and the
built-in variables, arrays with `SUBSEP` and `delete`, every control statement,
user-defined functions, the string and numeric built-ins, `printf`/`sprintf`, all
six `getline` forms, output redirection, and `-F`/`-v`/`-f`/`--` with operands
that mix file names and `VAR=VALUE`.

Each rule below was measured against gawk 5.4.1 and busybox-w32 1.38.0 before it
was written. Where the two disagree, the one followed is named.

**Text is counted in runes, not bytes.** `length("héllo")` is 5 here, and
`index`, `substr`, `match`, `RSTART`, `RLENGTH` and `split(s, a, "")` all agree
with it. The references do not settle this: busybox always counts bytes, while
gawk counts bytes under `LC_ALL=C` and runes under a UTF-8 locale. Runes are what
`wc -m`, `rev`, `fold` and `sed`'s `y///` already count here, and byte offsets
would let `substr` cut a UTF-8 sequence in half and emit invalid output.
`toupper`/`tolower` and `printf "%c"` follow the same rule, so `printf "%c", 233`
writes `é` rather than the lone byte `0xE9`.

**A command is an applet of this shell, and nothing else.** `system()`,
`print | cmd` and `cmd | getline` look their command up in the applet registry and
refuse anything else by name, because `internal/applets` never spawns an OS
process — the boundary `docs/design/windows-execution-model.md` sets and `xargs`
already draws. So `"sort" | getline` works and `system("c:/tool.exe")` does not.
Shell syntax inside such a command is **refused rather than approximated**: an
unquoted `;`, `|`, `&`, `<`, `>`, `$` or backtick is an error, since treating
`echo a; echo b` as `echo` with three arguments would be a wrong answer wearing
the costume of a right one. A quoted metacharacter is an ordinary character, so
`grep '$'` still works.

**A pipe collects and runs at close.** `print | "sort"` hands its applet
everything written to it when `close()` is called or the program ends, rather than
running concurrently. `print "x" | "sort"; print "direct"` therefore prints
`direct` first — busybox's order; gawk's is the other way round.

Where the references disagree and one was chosen:

| case | gawk | busybox | here |
| --- | --- | --- | --- |
| `gsub(/a*/, "-", "aaa")` | `1 -` | `2 --` | gawk — an empty match abutting the previous one is skipped |
| a replacement `\\` with no `&` after it | two backslashes | one | busybox, which is POSIX |
| `index(s, "")` | 1 | 0 | gawk, which is POSIX |
| `printf "%*d"` | supported | refused | gawk |
| `printf "%e"` exponent | `e+03` | `e+003` | gawk — busybox's is the MSVC runtime, not a rule |
| `printf "%d", 2147483648` | 64-bit | wraps to 32-bit | gawk |
| `printf "%.0f", 2.5` | `2` | `3` | gawk — half to even |
| a `printf` argument that is missing | fatal | empty/zero | busybox, which is POSIX |
| `"0x1A" + 0` | 0 | 26 | gawk — awk has no hex literal |
| `substr("hello", 2, 1e20)` | `ello` | empty | gawk |
| `print -2^2` | `-4` | `-4` (1.38.0) | `-4` — `^` binds tighter than unary minus |
| a missing input file | exit 2 | exit 1 | busybox: no program is broken, which this awk's other failures, gawk's 2, say |

`substr`'s out-of-range rule is the one worth stating outright, because it is not
what it looks like: **a start before the string moves to 1 and the length is
kept**, so `substr("hello", -2, 4)` is `hell` and not `h`. Arguments are truncated
toward zero rather than rounded.

**`for (k in a)` walks in insertion order.** POSIX leaves the order unspecified
and the two references answer differently; a stable order is chosen for the reason
`internal/shell/runtime/array_associative.go` gives for the shell's own arrays. A
program that depends on a *particular* order is depending on something POSIX does
not promise.

**`rand` is reproducible.** The sequence starts from seed 1 on every run, as both
references do, and `srand(x)` answers the previous seed. The numbers themselves
are this generator's and are not gawk's or busybox's — what is promised is the
range and that one seed gives one sequence.

Refused by not being implemented: gawk's extensions — `gensub`, `asort`, `switch`,
`@include`, `BEGINFILE`/`ENDFILE`, true multidimensional arrays beyond `SUBSEP`,
and a regular-expression `RS` (POSIX allows a single character or empty, and
busybox agrees). A plain `getline` reads only the file the record loop is reading
and answers 0 at its end rather than advancing to the next operand; gawk advances.

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

**The file commands and `l`.** `r FILE` queues FILE's lines to be written at the end of the
cycle, as `a` queues its text, and a FILE that cannot be read queues nothing; with two
addresses it is refused, as busybox refuses it. `w FILE` and s///'s `w` flag write the pattern
space to FILE, which is made empty when the run starts whether or not a line reaches it, and is
one file for every command that names it. FILE runs to the end of the line, as busybox reads
it. `l` writes the pattern space unambiguously, POSIX's form: `\\`, `\a \b \f \n \r \t \v`,
octal for a byte that is no printable character, a split with a backslash past 69 characters,
and `$` at the end. busybox accepts `l` and prints nothing; this prints what POSIX asks.

Still refused: `e`, and GNU's `first~step` addresses and `R`, `W` and `z`.

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

`head -z` stays refused: `-z` is a GNU-only NUL-terminated-line mode that is a real
choice rather than an oversight. `tail -f` came on 2026-09-30 with busybox's answers
to what it waited for: a FILE that shrinks is read from its start again, and `-F`
reopens one that its name has come to mean, reading the old one to its end first. On
Windows the FILE is opened sharing delete, so the program writing a log can rotate it
while tail follows; busybox-w32's open does not share delete, and `mv log log.1` under
its `tail -F` is `Device or resource busy`.

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

### `lsattr` and `chattr`

The pair busybox-w32 has for Windows' file attributes rather than ext2's flags,
registered on Windows alone, as `su` is: on Linux and macOS the names are
e2fsprogs', and an applet there would shadow the real one. `lsattr` prints the same
eleven columns, `R o e c S r h s a t n` -- a reparse point, offline, encrypted,
compressed, sparse, read only, hidden, system, archive, temporary and not indexed --
and the first is `l` for a symbolic link, `j` for a junction, `m` for a volume mounted
on a folder and `A` for an app execution alias. `-l` spells them out, `Hidden,
Archive`, or `---`. A directory's entries come in the order Windows gives them, `.`
and `..` first under `-a`, as busybox's readdir gives them; at the root of a volume,
which has neither, they come last, as busybox-w32 makes them up.

`chattr` changes the six that Windows lets a program change, `-` clearing and `+`
setting: `chattr +h -a f`. An `R` among the letters after a dash is `-R`, which goes
down through directories but past no link. What it cannot change is named with
strerror's reason, `chattr: cannot set the attributes of sub: Invalid argument` for a
temporary directory, and the rest are changed. The refusals are busybox's, in this
shell's words: no letter to change, one both set and cleared, a letter it has not
got.

A junction is a link to both, so neither goes down through one, though Go calls a
junction a directory. Both answer 0 when a FILE failed, as busybox-w32 answers whatever
happened; e2fsprogs, whose options these are, says 1, and so did these. A path under
a FILE is joined with one slash, as `cp` and `rm` join them, so `lsattr 'C:\'` lists
`C:/Windows` where busybox lists `C:\/Windows`.
