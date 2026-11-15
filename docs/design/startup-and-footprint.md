# Startup cost and memory footprint

Every number here was measured on the machine this was written on (Windows 10
19045, windows/amd64), with the release flags CI uses:
`CGO_ENABLED=0 go build -trimpath -ldflags "-s -w …"`. Where a figure is a
median it says so. This is a record of what was measured, what was changed, and
what was deliberately left alone with the price written next to it.

## The floor

A shell is a program you start constantly, so the question is not "is it fast"
but "how much of it is ours". Two reference points:

| binary | median startup |
| --- | --- |
| Go program whose `main` is `fmt.Println("hi")` | 4.4 ms |
| `busybox true` (scoop, 7.2 MB) | 7.8 ms |

4.4 ms is process creation plus Go runtime start. Nothing in this repository can
move it, so it is the number to measure against rather than zero.

## What was found and fixed

`nemosh --version` measured **29.2 ms** — six times the floor, and identical to
`nemosh -c 'echo hi'`, which is the tell: the cost was entirely fixed, paid
before anything looked at the arguments.

`GODEBUG=inittrace=1` located it in one package:

```
github.com/BurntSushi/toml/internal    22.000 ms clock    53,968 bytes    1,343 allocs
```

That package initialises three variables with `time.Now().Zone()`. On Windows the
first such call makes Go's runtime name the local zone by enumerating every
subkey under `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Time Zones`.
Confirmed independently by building a program whose only init is the same call:
19 ms and 1,334 allocations, the same shape.

Package init runs whether or not anything decodes a file, so `nemosh true`,
`nemosh --version` and every applet shim walked the registry for a timezone that
no completion spec has a datetime to need. `internal/completionspec` was the only
path by which the library entered the binary — `internal/testutil`, where the
behavior corpus uses it, is not linked into `nemosh` at all.

The fix is `internal/completionspec/toml.go`: a reader for the TOML the format
actually uses, with the library kept as the reference in a differential test.
See that file's comment for the design, and `toml_differential_test.go` for what
holds the subset to the real thing.

| | before | after |
| --- | --- | --- |
| startup, `--version` (median) | 29.2 ms | **6.7 ms** |
| package init, clock | 23.5 ms | 2.0 ms |
| package init, allocations | 1,675 | 322 |
| package init, bytes | 121,688 | 66,216 |
| binary, stripped | 4,740,608 | 4,518,912 |
| RSS, interactive session | 9,844 K | 9,660 K |

6.7 ms against a 4.4 ms floor accounts for itself: 2.0 ms of package init, and
about 0.3 ms of this shell's own work before the first prompt.

CI gates it. `.github/workflows/product.yml` has a **package init ceiling** on
allocations rather than on the clock, for the reason the binary size comment
gives: a wall-clock threshold on a shared runner flaps until it is ignored. The
old binary measures 1,673 against a ceiling of 900, so the gate catches exactly
the regression that motivated it.

## Measured and deliberately not taken

### Winsock, 1.5 ms and 165 KB

`net`'s init calls `poll.InitWSA()` — `WSAStartup`, which starts the Windows
socket library in a shell that never opens a socket. It is 1.5–2.0 ms across
eight samples, which is most of the 2.0 ms of init that remains.

`net` is linked for one reason: `golang.org/x/sys/windows/types_windows.go`
imports it for socket address types nothing here touches. Go links whole
packages, and package init is unconditional, so the only way out is to stop
importing `x/sys/windows` — used in ten files for about fifty symbols.

Priced: **1.5 ms of startup and 165 KB of binary** (measured by building a
program that imports `x/sys/windows` and nothing else, against the floor
program).

Not taken, because most of those symbols have `syscall` equivalents but two
groups do not, and both are the wrong things to hand-roll for 1.5 ms:

- `NewLazySystemDLL`, which loads with `LOAD_LIBRARY_SEARCH_SYSTEM32`.
  Reimplementing it is how DLL search-order hijacking gets introduced, and the
  version in `x/sys` is the reviewed one.
- `CreateWellKnownSid`, `WinBuiltinAdministratorsSid` and
  `GetCurrentProcessToken`, which are how `su` decides whether it already has
  administrator rights. A subtle mistake there does not fail loudly; it
  misreports, and `su` is the applet where that matters most.

Reversible: if `x/sys/windows` ever drops the `net` import, this cost disappears
with no work here.

### A 32 MiB reservation from the crypto packages, unavoidable

`crypto/internal/fips140/drbg.memory` is a 33,554,432-byte scratch buffer that
the FIPS entropy source touches to expose memory-access timings. It shows up as
commit charge:

| program | RSS | commit |
| --- | --- | --- |
| live Go program, no crypto | 5.4 M | 12.2 M |
| the same plus `crypto/md5` | 5.6 M | 44.7 M |
| the same plus `crypto/rand` | 5.6 M | 45.1 M |

Two things follow. It is **zero resident** — the pages are committed and never
touched, so it costs no RAM, only commit charge and the alarming number Task
Manager shows. And it arrives with `crypto/md5` alone, so it is the price of
shipping `md5sum` and `sha256sum`, not of `/dev/urandom`: dropping
`crypto/rand` from `device.go` would buy nothing. Confirmed by symbol
(`go tool nm`) in a binary whose only crypto import is `crypto/md5`.

## Memory footprint

RSS tracks binary size closely enough to be treated as the same lever: removing
222 KB of binary removed 184 KB of RSS, near enough 1:1. So the binary size
ceiling in CI is also the footprint ceiling, and there is no separate number to
watch.

Binary size is **not** a startup lever, which is worth knowing before anyone
spends a day on it for the wrong reason. Padding the floor program with a 2.9 MB
array to match this shell's image size left its startup unchanged — 4.6 ms
median at 4,567,040 bytes against 4.9 ms at 1,667,072, which is inside the noise
and nominally the wrong way round. Windows demand-pages the image, so the pages
a run never touches cost nothing to launch. The 165 KB priced above is therefore
worth 165 KB of footprint and no milliseconds; the 1.5 ms next to it is Winsock
initialising, not the image being larger.

Where the interactive session's 9.66 MB goes: 5.4 MB is the floor for any live
Go process, and most of the rest is image pages for a 4.5 MB binary. Of that
binary, `runtime` and `runtime.pclntab` are 2.9 MB and 1.35 MB; everything under
`internal/` is about 700 KB together. There is no allocation to reclaim here,
only features to delete.

## Completion, and why there is nothing to unload

Worth stating plainly, because "lazily loaded, and cached forever" sounds like a
leak and is not one here.

- The bundled specs are **18,957 bytes total**, embedded by `go:embed` as
  read-only image data. They are not on the heap; the OS pages them in on demand
  and can drop them again, and they are shared between concurrent `nemosh`
  processes because the image is.
- Nothing is parsed at startup, and no directory is scanned. `Registry.Lookup`
  keys on the file name, so a lookup is one `stat` and at most one open, and it
  happens on the first Tab **for that command**. A session that never completes
  `adb` never reads `adb.toml`.
- The cache holds only what was used, which in a real session is nought or one
  spec of a few hundred bytes. Evicting that would trade a measurable saving of
  nothing for re-reading a file on the keystroke path.

So the answer to "should completion unload" is that the thing that would be
unloaded is smaller than the bookkeeping to unload it, and it is not heap in the
first place.

## The keystroke path

Measured because a suggestion is recomputed after every character typed, which
is the one place in this shell where being slow is felt directly rather than
merely being slow. Ceilings live in `cmd/nemosh/perf_keystroke_test.go`.

| | time | allocations |
| --- | --- | --- |
| a line already in history | 556 ns | 0 |
| a full scan of 1,200 command names | 35 µs | 1 |
| twelve keystrokes of a whole word | — | 11 |

Nothing was changed here: it was already at 0–1 allocations per keystroke,
because `candidates()` hands back the index's slice rather than copying it and
the sources are all in memory. The measurements are kept so that stays true — a
stray `os.Stat` on this path would show up as a failing ceiling rather than as a
stutter someone has to reproduce.

## What the Windows metadata calls cost, measured 2026-08-20

`du` and `ls -l` were both reporting less than they claimed to: `du` counted
apparent size where the name means allocated, and `ls -l` printed three fields
where every other `ls` prints seven. Both wanted facts `os.FileInfo` does not
carry, so both reach the platform directly, and the price is worth having on
record before anyone reaches for it again.

**Nothing was added to the binary.** `golang.org/x/sys/windows` was already
linked — `internal/applets/id_windows.go` uses it for `id`, and `cmd/nemosh` for
the console work — so the dependency was paid for long ago:

| | measured | ceiling |
| --- | --- | --- |
| stripped binary | 4,854,272 bytes (4.63 MiB) | 5,767,168 (5.50 MiB) |
| package init allocations | 439 | 900 |

Startup is untouched by construction: nothing here runs at init, and both calls
sit behind an applet that has to be asked.

What it costs is per-file calls, over the 4,895 entries of `C:\Windows\System32`:

| | nemosh | busybox-w32 |
| --- | --- | --- |
| `ls` | 14 ms | 45 ms |
| `ls -l` | 276 ms | 45 ms |
| `du -s` | 132 ms | 43 ms |

So `ls -l` is roughly 57 µs a file and about six times busybox, and `du` about
three times. Two things are worth saying about that. The first is that it is a
directory with five thousand files in it; a listing of fifty costs about 3 ms,
which nobody can perceive. The second is that busybox is clearly doing something
cheaper for the owner column, and finding out what would be the next
optimisation — the obvious one, a single handle serving both the link count and
the security query, needs `READ_CONTROL` on the open and would silently degrade
every owner to `root` if that were refused, so it was not taken blind.

One caching decision already paid for itself: resolving an owner SID to a name
costs 170 µs per file, and 29 µs with a SID-to-name cache, because a directory
usually has one or two distinct owners. A domain account is the case to watch —
`LookupAccount` may go to a domain controller — and the cache turns that from
once per file into once per owner.

Two of my own inefficiencies were found the same way and fixed: `du` was calling
`filepath.Abs` per file, which is a `Getwd` syscall, to find the volume of a path
that already named one (150 ms → 132 ms).

Measured again 2026-09-29, when `du` became busybox's walk: the whole of System32, every
directory under it, `du -s .`:

| | time | blocks |
| --- | --- | --- |
| busybox-w32 | 2.0 s | 9543872 |
| nemosh | 0.83 s | 9543872 |
| nemosh before | 0.2 s | 10469192 |

It was faster because it never asked a file how many links it has, and so counted every
file linked twice inside System32 twice. Asking is a handle open per file, which busybox-w32's
stat makes too, and one open answers the links, the volume and the file's index together.

## What tview and tcell cost, measured 2026-08-21

`top` is built on tview, and the question asked before writing any of it was what
that costs a shell whose startup this document exists to protect. Go has no lazy
package init and `AGENTS.md` forbids a sidecar binary, so whatever the answer was,
it was going to be charged to `nemosh -c true` as much as to `top`.

| | before | after | ceiling |
| --- | --- | --- | --- |
| stripped binary | 4,885,504 B | **6,127,104 B (+1.18 MiB)** | 5,767,168 → raised to 7,340,032 |
| package init allocations | 440 | 1,710 (+1,270) | 900 → **raised to 2,100** |
| package init clock | 0.00 ms | 0.00 ms | — |
| `nemosh -c true` | 9.4 ms min | 9.1 ms min | — |

**The 85 KiB first reported here was wrong, and the mistake is worth keeping.**
The spike that measured it built `Table`, `Flex`, `TextView`, `InputField`,
`Modal`, `Pages`, `Application.Run` and `tcell.NewScreen` inside a function that
nothing called. The linker dropped all of it, and the 85 KiB was two
package-level variables holding a function value and a style. The real cost, once
the widgets are reachable from an applet the registry lists, is 1.18 MiB — which
is roughly the 1.5–2.5 MiB originally estimated and then argued away.

It was even "verified": a second, fuller spike measured 2 KiB *smaller* than the
trivial one, and that was read as agreement within noise. It should have been read
as the tell, because more code cannot produce less binary. **A spike has to be
reachable from main to measure anything**, and dead code measures the linker's
diligence rather than the dependency's weight.

The allocation count is the one that moved, and it is worth naming where it went:
**1,103 of the 1,270 are `gdamore/encoding`**, building 66 KB of legacy
character-set tables at init, resident for the life of every process. About thirty
more are terminfo descriptions registering themselves. On Windows tcell drives the
console API and consults terminfo for nothing, so most of that is paid and unused
here. tcell imports both unconditionally; there is no build tag that avoids them.

So the gate tripped and the property it guards did not move. That parting of proxy
and property is the thing to remember about this measurement: the ceiling was
introduced to catch a package doing *work* before `main` — it caught toml spending
22 ms in the Windows registry — and it cannot tell that from a package merely
allocating a table quickly. It is kept on allocations because a clock threshold
flaps on a shared runner, and the ceiling was raised with this note rather than the
number being changed quietly.

The `gdamore/encoding` tables were taken out of init on 2026-11-16 by forking the module;
see "Package init, 2,773 allocations to 1,101" below.

## Timing a start without a number to hold it to, 2026-11-15

Every gate in this file was on allocations, because a clock threshold on a shared runner
flaps. That left the start's actual time watched by nothing, and the proxy had already
parted from the property once (the tview section above). `tools/startbench` times it
anyway, by never comparing a time against a number:

- **Against a base, not a ceiling.** CI builds `HEAD^` the same way and times both
  binaries on the same runner in the same minute. Whatever the machine is doing lands on
  both.
- **Interleaved.** Base and head run in pairs, five pairs discarded first (the first run
  of a new file on Windows waits for Defender) and forty kept, and which goes first is
  chosen afresh for each pair. Forty of one and then forty of the other would charge a
  runner's slow minute to whichever went second.
- **Medians, past a ratio and a floor, three times.** A scenario is slower only when
  head's median is past both 10% and 1 ms (5 ms for the long one) over base's, and only
  if the two rounds run after that say so too -- the rule the Oils suite has for a case
  that differs.
- **Checked against itself.** The self-check times head against head, which can only
  differ by noise; a slowdown there is reported as a false alarm. The timing reports and
  does not fail until enough CI runs of the self-check have shown the threshold sits
  outside the noise.

The three scenarios are `--version` (the start alone), `-c true` (that, plus a shell
built and one applet run in it), and two thousand commands in a loop, which is where a
cost paid per command rather than per start shows.

Two copies of one build on this machine, which is the noise and nothing else:

| | base | head | |
| --- | --- | --- | --- |
| `--version` | 7.75 ms | 8.00 ms | +3.2% |
| `-c true` | 25.54 ms | 25.75 ms | +0.8% |
| 2000 commands | 53.23 ms | 53.69 ms | +0.9% |
| self-check, `--version` | 7.79 ms | 7.44 ms | −4.4% |
| self-check, 2000 commands | 53.02 ms | 54.06 ms | +1.9% |

The init count is close to deterministic and not quite: the same binary run twice once
moved one allocation each into `net/http` and `net/netip`, and totals have read three
apart. Hence a ceiling with room rather than an exact figure, and growth listed by
package rather than asserted.

### `$PPID` was more than half of every start

The first run answered a question nobody had asked: `-c true` took 25 ms where
`--version` took 8, and this file had recorded 9.1 ms for it in August. `-c :` cost the
same, so it was not the applet; a short PATH and a missing HOME changed nothing. A CPU
profile of building the shell in-process put 98% of it in one call, `os.Getppid`, for
`$PPID`. On Windows that takes a snapshot of every process on the machine
(`CreateToolhelp32Snapshot`) and walks it for this one's entry, so its price is the
machine's process count -- 13.7 ms here -- and every script, every `nemosh -c` and every
applet shim paid it, since all of them build a shell's state.

The snapshot reads the parent from the process's own basic information, and
`NtQueryInformationProcess` hands over the same field in one call
(`parent_process_windows.go`); a test holds the two answers equal.

| | before | after |
| --- | --- | --- |
| `--version` | 7.52 ms | 7.47 ms |
| `-c true` | 25.36 ms | **10.68 ms** |
| 2000 commands | 51.98 ms | 37.65 ms |

The allocation gate could never have seen this: the call allocates almost nothing, and it
runs after `main` begins.

### Package init, 2,773 allocations to 1,101

With the ceiling nine allocations from tripping, the three largest contributors were
looked at again, and each turned out to be work that could be taken out of init rather
than moved behind a lock -- which is what the ceiling's comment had rightly refused for the
registry and the capability rows, since nearly every run needs those.

| | allocations | what |
| --- | --- | --- |
| applet usage text | −417 | 153 entries with an option map each, read only by `--help` and `help NAME`; built on first use now, as the builtins' text already was |
| applet registry | −152 | one wrapper per applet, each an allocation as it went into the map's interface; one list now, and a lookup hands out a pointer into it, so it allocates nothing |
| `gdamore/encoding` | −1,103 | the tview section's 48%: four legacy character sets built in `init`. Forked into `third_party/`, the tables generated as data the linker lays down, and tested against upstream's algorithm for every byte and every rune |

Measured here with `tools/startbench`, each against the commit before it. The time barely
moved -- `--version` about 0.4 ms faster after the fork, which is about what the trace's
1.0 ms of clock for `gdamore/encoding` rounded from -- because these were tables built
quickly; the ceiling is back to catching a table getting longer, at 1200 on the runner.
The binary is 6 KB larger for the tables.

Wrapping each applet at lookup instead of at registration was the first idea, and it cuts
the same 152 from init. It would have charged one allocation to every command run instead,
which is the wrong trade for a shell, and why the list.

### The applet dispatch, seven allocations to one or none

Running an applet allocated seven times before the applet did anything: the two context
values that carry the shell to it as ProcessView and ProgramRunner, a copy of the whole
`Runtime` into each of their interfaces, the stdin and stdout wrappers, and a method value
inside the stdout one. `applet_frame.go` puts all of it in one object, the frame, which is
the context, and whose reader and writer are pointers into it. A frame is never changed once
made; the shell keeps its last four and hands one out again for a call that is the same --
the same context, and a `Runtime` equal to the byte.

| | allocations |
| --- | --- |
| dispatching `true`, before | 8, one of them the test's own argv |
| dispatching `true`, the same call again | **0** |
| a call unlike the last four | 1, the frame |
| a loop iteration, `while [ $i -lt N ]; do true; i=$((i+1)); done` | 142 → 130 → **108** |

At first the loop did not hit the remembered frames: every command cloned the descriptor
table (`withAppliedRedirectsFor`), redirections or none, so no two calls had the same
streams, and the loop's two applet calls went from seven allocations to one each -- the
130. A command with no redirections runs in the shell's own descriptors now, which is what
busybox does anyway: the copy was also why `exec 3>log` inside a function was gone when the
function returned. Without the clone and its closing, an iteration is 108, and its applet
calls are handed the frames they had the time before; a test holds that.

Reusing one mutable frame per call was the first design, and the audit for it is why the
frames are immutable instead. An applet's context and streams outlive it more often than one
would guess: the editor and `less` leave a goroutine waiting on `ctx.Done()`, `nc` returns
with its copy goroutines still reading, and `net/http`'s transport holds a request's context
a moment after the body is read. Each of those would have read a frame while the next call
was rewriting it -- and an interface read half-way through being rewritten is not stale
data, it is a type paired with the wrong pointer. A frame that is never changed can be held
for as long as anything likes.

## What a background job that is a process costs, measured 2026-09-24

Under `NEMOSH_JOBS=process` (background-processes.md) every `&` starts this binary again.
These are wall-clock times for the whole script, the best of three, on the machine the
rest of this file was measured on:

| script | goroutine jobs | process jobs |
| --- | --- | --- |
| `for i in 1 .. 10; do :& done; wait` | 0.108 s | 0.183 s |
| `for i in $(seq 100); do :& done; wait` | 0.107 s | 0.535 s |
| 20 jobs calling a function, with an array and an associative one set | 0.092 s | 0.217 s |

That is about 4-6 ms a job over the shell's own start, the process launch plus the
state written, read and parsed again. It is less than the 9 ms a cold start costs, since
the image is already in the file cache. busybox-w32 runs the ten-job line in 0.148 s. Its
hundred-job line did not finish within five minutes here, and it was stopped.

A running job process holds about 11 MB of working set (five `sleep 4 &` jobs, measured
with `tasklist`: 11.2-11.5 MB each), which is this shell's interactive footprint less
the line editor. Sizing the default-flip decision in those terms: a script that keeps
a hundred jobs running at once would hold about a gigabyte.
