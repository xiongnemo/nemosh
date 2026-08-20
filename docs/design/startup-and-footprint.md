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
