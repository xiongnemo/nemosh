# The Process View

How `top`, `ps`, `pgrep`, `pkill` and `kill` see other processes, what Windows will
answer, and what it will not answer at all.

## Why This Exists

Windows has no usable process monitor for an ordinary session. `ntop` shows a list
and little else. `btop++` demands elevation and refuses without it. Task Manager
is not a terminal program. The gap is specifically **rich and unprivileged**.

The gap turned out to be in the tools rather than in the platform, and that is the
finding this document exists to record.

## The Finding

`NtQuerySystemInformation(SystemProcessInformation)` returns the **whole process
table in one call, unelevated**. Per process, with no handle opened: thread count,
`CreateTime`/`UserTime`/`KernelTime`/`CycleTime`, image name, base priority, PID,
**PPID**, handle count, session, working set (total and private), virtual size,
pagefile usage, private page count, page and hard fault counts, and six IO
counters — followed inline by one record per thread. `golang.org/x/sys/windows`
already declares both the call and `SYSTEM_PROCESS_INFORMATION`.

**Opening handles is what costs privilege.** Measured on an unelevated session:

| probe | result |
| --- | --- |
| processes visible | 436 |
| PowerShell `Get-Process` with CPU time | **249** of 436 |
| PowerShell `Get-Process` with `Path` | **176** of 436 |
| `Win32_PerfRawData_PerfProc_Process` with CPU, threads and IO | **439 of 439**, PID 4 included |

PowerShell's gaps are not the kernel refusing. They are `OpenProcess` per process.
The data is there for anyone who asks the kernel once instead of asking each
process in turn.

## htop's POSIX Calls Against What Windows Answers

| htop feature | Linux source | Windows source | verdict |
| --- | --- | --- | --- |
| process list | `/proc/*/stat` | `SystemProcessInformation` | **full** |
| CPU% per process | `utime+stime` delta ÷ `/proc/stat` | CPU delta ÷ wall clock ÷ processors | **full** |
| RES / VIRT | `/proc/pid/statm` | `WorkingSetSize` / `VirtualSize` | **full** |
| MEM% | RES ÷ `MemTotal` | working set ÷ `GlobalMemoryStatusEx` | **full** |
| threads | `/proc/pid/task/*` | inline thread array | **full**, cheaper than Linux |
| PPID / tree | `/proc/pid/stat` | `InheritedFromUniqueProcessID` | **full**, with the caveat below |
| per-core meters | `/proc/stat` | `SystemProcessorPerformanceInformation` | **full+**, also DPC and interrupt time |
| memory / commit | `/proc/meminfo` | `GlobalMemoryStatusEx`, `GetPerformanceInfo` | **full** |
| uptime | `/proc/uptime` | `GetTickCount64` | **full** |
| IO rates | `/proc/pid/io`, often restricted | the table's transfer counts | **full+**, better than Linux |
| handle count | — | `HandleCount` | Windows-only, and the commonest Windows leak |
| SHR | `statm` | no equivalent split | **substitute**: private working set |
| swap per process | `status` `VmSwap` | `PagefileUsage`, commit charge | **different measure**, labelled `COMMIT` |
| state R/S/D/Z | `/proc/pid/stat` | derived from thread state and wait reason | **approximation**, no zombie |
| nice / renice | `getpriority`/`setpriority` | `BasePriority`, `SetPriorityClass` | **coarse**: six classes, own processes only |
| kill | `kill(2)` | `TerminateProcess` | **partial**: never graceful |
| user column | `status` Uid + `getpwuid` | `OpenProcessToken`, refused for other users | **partial**, ~40% of rows |
| command line | `/proc/pid/cmdline` | `NtQueryInformationProcess(ProcessCommandLineInformation)` | **partial**, same-user only |
| affinity | `sched_getaffinity` | `GetProcessAffinityMask`, needs a handle | **partial** |
| TTY column | `/proc/pid/stat` `tty_nr` | no controlling terminal exists | **impossible** |
| load average | `/proc/loadavg` | does not exist | **impossible** |
| cgroup column | `/proc/pid/cgroup` | job objects are not analogous | **impossible** |
| CPU temperature | `/sys` | needs WMI or a driver | out of scope |

### The Three Impossible Ones

**TTY.** Windows has no controlling terminal. A console is not owned by a process
in the way a tty is, and there is nothing to report.

**Load average.** There is no runnable-thread average anywhere in the kernel. The
nearest analogue is the *processor queue length* performance counter, which counts
threads waiting for a processor at an instant rather than averaged over a minute.
Publishing that under the name "load average" would be putting a different measure
behind a familiar label, so the header shows none and `--help` says why. This is
the same posture `ps` takes for TTY: an absent column beats a fabricated one.

**cgroups.** Job objects can limit a set of processes, but nothing enumerates
"which job is this process in" the way a cgroup path does.

## Two Windows Facts That Shape The Code

**PID reuse and no reparenting.** Windows never reparents an orphan, and a dead
process's id goes back into the pool. So a parent id can name a stranger. Identity
is therefore `(PID, CreateTime)` everywhere in `internal/proc`, and the tree's rule
is that **a parent must be older than its child** — which removes the wrong
adoptions and, because "older than" is a strict order, makes a cycle impossible
rather than merely guarded against.

**Idle is inside kernel time.** `SYSTEM_PROCESSOR_PERFORMANCE_INFORMATION` reports
`KernelTime` *including* `IdleTime`. Reading kernel time as system time shows a
sleeping machine at full occupancy; system time is busy minus user.

## Two Traps Worth Recording

**The fixed-record information classes want an exact multiple.**
`SystemProcessorPerformanceInformation` answers `STATUS_INFO_LENGTH_MISMATCH`
unless the buffer is an exact multiple of the record size. A grow-and-retry loop
that doubles never *becomes* a multiple, so a generously sized buffer fails for
ever — a half-megabyte buffer for sixteen small records was the reason nothing
worked at all. That query is sized in whole records.

**`ProcessCommandLineInformation` needs only limited access.** The old way to read
another process's command line is to read its PEB, which needs `PROCESS_VM_READ`
and is refused for nearly everything. Information class 60, on Windows 8.1 and
later, answers with `PROCESS_QUERY_LIMITED_INFORMATION` — a much lower bar, and
the reason the `COMMAND` column shows real command lines for same-user processes.

## Structure

- `internal/proc/sample.go` — platform-free types. A snapshot is the whole system
  at an instant, because that is the shape one call answers in.
- `sample_windows.go`, `sample_walk_windows.go` — the call and the walk over
  `NextEntryOffset`, with buffers reused between samples.
- `totals_windows.go` — `GlobalMemoryStatusEx`, `GetPerformanceInfo`,
  `GetTickCount64`, declared through `NewLazySystemDLL`.
- `rates.go`, `tree.go` — **pure**. Two snapshots in, percentages and parentage
  out. This is where the tests are, because a monitor's arithmetic is where the
  believable-looking mistakes live.
- `detail_windows.go` — the three answers that need a handle, each `(value, ok)`,
  cached by identity so a refresh does not re-attempt 400 denied opens per second.
- `internal/applets/top_model.go` — the view state, with no terminal in it, so
  sorting and filtering can be tested by pressing keys at a struct.
- `top_batch.go` — the plain form, which is what a script and the corpus can use.
- `top_view.go` — the only file that mentions tview.

## Getting A Terminal From Inside The Shell

An applet is handed an `io.Reader` and an `io.Writer` by the fd table, so that
redirection works. tcell needs real console handles. Both seams already existed and
were built for other reasons:

- `descriptorWriter.TerminalFile` in `internal/shell/runtime/fd_stream.go`, added
  so `ls` could decide whether to lay out columns.
- the stdin lease in `internal/shell/runtime/external_stdin.go`, added so an
  external process could be handed the real console.

`top` needs exactly those two, which is the argument for having put them in the fd
table rather than in the callers. The lease matters for a second reason: the
shell's own reader thread may be parked on a console read, and two readers on one
console input handle means keys go to whichever asked first.

## Where The Drawn Form Will Not Appear

`top` draws when its output is a Windows console and prints one plain sample
otherwise, saying which it chose. One case surprises people and is worth naming:

**Git Bash / mintty gets the plain form.** mintty is a Cygwin pseudo-terminal,
which is implemented as a *named pipe* rather than as a console, and Go's
`term.IsTerminal` asks `GetConsoleMode`, which fails on a pipe. So from a mintty
window the answer is honestly "not a terminal" even though a person is plainly
looking at one. Windows Terminal, conhost, `cmd.exe` and PowerShell all provide a
real console and get the drawn form.

Distinguishing a mintty pty from an ordinary pipe is possible -- the trick is to
ask the pipe its name through `GetFileInformationByHandleEx(FileNameInfo)` and look
for the `msys-…-pty` pattern that Cygwin gives its ptys -- and it is not done here.
Drawing escape sequences into something that turns out to be `| head` is a worse
failure than printing text into something that turns out to be a terminal.

## The Wrapper That Hid The Console

The first run of this in a real PowerShell window printed

```
top: standard input is not a terminal, so no key can be read; printing one sample
```

in a console with a keyboard attached. Nothing was wrong with the console, the
lease, or the detection: `Registry` wraps every applet's stdin in a
`contextReader` so a long read can be cancelled, that wrapper forwarded `Read`
and nothing else, and the request for the console stopped there. The applet then
reported, accurately, what it had been told.

**This is the third time the same shape has bitten**, and the pattern is worth
naming because it will keep happening as long as streams are wrapped:

| wrapper | added for | hid | symptom |
| --- | --- | --- | --- |
| `descriptorWriter` | fd redirection | `TerminalFile` | `ls` never gridded, `--color=auto` never coloured |
| `synchronizedWriter` | concurrent writes | `TerminalFile` again | the same, after the first fix |
| `contextReader` | cancelling a parked read | `LeaseStdinFile` | `top` never drew |

Each failure is silent by construction. The wrapper answers a capability question
with "no" rather than with an error, the caller does something reasonable with the
wrong answer, and the result looks like a deliberate choice. That is why
`terminalFileOf` walks a chain instead of testing one hop, and why
`contextReader` now forwards the lease.

The rule this leaves: **a stream wrapper must forward every capability of what it
wraps, not only the method it was written for.** `internal/applets/stdin_lease_test.go`
holds it in place for stdin, including an assertion that the type the registry
actually hands an applet can be asked for the console -- the check that would have
caught all three.

## What Reading htop's Source Corrected

htop is GPL-2.0, so it stands here exactly as busybox does: read to learn what a
monitor *does*, never copied. `docs/design/reference-methodology.md` draws that
line, and this is the second subsystem to sit on the read-only side of it.

Its `Action.c` holds the real key table, and reading it corrected three things
that had been guessed:

- **`space` tags a process; it does not fold a branch.** Folding is `+`, `-` and
  `=`. Having `space` fold is precisely the kind of error that makes a familiar
  tool feel broken, and no amount of testing our own behaviour would have found
  it.
- **Searching and filtering are different commands.** `/` and F3 search, jumping
  to a match and leaving the list whole; `\` and F4 filter, hiding what does not
  match. One key was doing both here.
- **The sort keys people actually use are letters**: `P` for CPU, `M` for memory,
  `T` for time, `N` for pid. Digits are top's convention; both are accepted now.

Three more bindings were adopted because their reasons apply here too. `Z` pauses
updates -- a list that reorders every second cannot be read carefully, and reading
carefully is what someone does immediately before killing something. `[` and `]`
change priority alongside F7 and F8. And `p` toggles the full path against the bare
name, which lands exactly on this platform's split between what a handle answered
and what the process table always knows.

Two pieces of htop's architecture are worth recording as roads not taken. Its
`Machine` struct keeps a monotonic clock reading *and* the previous sample's, so
rates never depend on the wall clock; Go's `time.Time` carries a monotonic reading
and `Sub` uses it, so `Snapshot.Taken` is already safe in the same way, for free.
And htop's platform layer is an explicit interface of some forty
`Platform_*` functions -- `Platform_getLoadAverage` among them -- which is what a
port implements. `internal/proc` is that interface here, and the difference is
instructive: htop requires every platform to answer `getLoadAverage`, so a port
without one has to invent a number. Being a single-platform tool means being
allowed to say there isn't one.

## What Is Not Done

- `F7`/`F8`/`[`/`]` priority change is recognised and reports that it is not wired.
- `/` search is recognised and says that it is not wired; F4 filtering works.
- Tagging with `space` marks rows, and a multiple kill does not act on them yet.
- No configuration file yet, so the column layout is the default one.
- Per-thread rows are sampled under `-H` but not yet rendered as rows.
