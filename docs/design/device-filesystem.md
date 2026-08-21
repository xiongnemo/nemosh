# Simulating `/dev`

Whether `/dev` should be a filesystem rather than a set of magic strings, what
that would buy, and what it would cost. Written 2026-08-21 in answer to the
question directly; the measurements are from this machine on that date.

**Built 2026-08-22.** Option C was chosen and is done: `/dev` is a directory that
lists, globs, completes and can be walked, and every device answers `stat` from
the same table the opener reads. `docs/support-matrix.md` is the contract; this
document is the reasoning, kept because two of its decisions were reversed by
measurement while the work was under way.

Three things turned out differently from the estimate, and they are worth reading
before planning anything similar here:

- **`find /` never meets `/dev`.** The plan put an `fs.FS` spanning the real and
  synthetic namespaces at the centre of the work, on the assumption that a root
  walk would descend into `/dev` as it does on Linux. It cannot: `/` resolves to
  `/c`, the current drive's root, so `/dev` is a *sibling* top-level name.
  Measured with `ls / | grep -c '^dev$'`, which answers zero. That removed the
  splicing, the per-entry path translation and the performance risk together --
  about two thirds of the estimated cost of Option C, and the whole of its risk.
  What the walkers needed instead was a walk of one directory with no
  subdirectories.
- **`resolveHostPath` was already the choke point**, which is why Stage 1 reached
  `test` and `ls` through one helper rather than nineteen call sites.
- **`cd /dev` is refused**, where the plan said it should succeed. A working
  directory needs a native form for launching a child, and `/dev` has none; the
  reasoning is in the support matrix beside the behaviour.

The abstraction the plan named -- one `fs.FS` over the whole namespace -- was
therefore *not* built, and deliberately. Building it for a single one-level tree
that no real walk can reach would be the abstraction-for-one-tenant mistake this
document warns about below. It waits for `/proc`, which is when it starts paying.

## What exists now

`/dev` is a **namespace of exact string matches**, not a directory. Two things
implement it:

- `internal/pathmodel` and `path_state_*.go` recognise `/dev` and `/dev/...` as
  policy paths and resolve them to `ResolvedPath{Device: true}` with **no native
  path at all**.
- `internal/shell/runtime/device.go` and `device_fd.go` switch on the exact
  string in the redirection and open paths: `/dev/null`, `/dev/zero`,
  `/dev/random`, `/dev/urandom`, `/dev/clipboard`, `/dev/stdin`, `/dev/stdout`,
  `/dev/stderr`, `/dev/fd/N`.

## The gap, measured

A device is **openable but not observable**.

| | nemosh | busybox-w32 |
| --- | --- | --- |
| `echo x > /dev/null` | works | works |
| `cat /dev/null` | works | works |
| `head -c 4 /dev/zero` | works | works |
| `wc -c < /dev/null` | works | works |
| `test -e /dev/null` | **says no** | **exists** |
| `ls -l /dev/null` | **`is not a host path`** | **`crw-rw-rw- 0, 0 Jan 01 1970`** |
| `ls /dev` | `is not a host path` | `No such file or directory` |
| `cd /dev` | `not a directory` | not a directory |

Two rows are divergences from the reference and one is not. busybox answers for
the *entries* and declines the *directory* — it synthesises a character-device
stat for a name it knows and has no `/dev` to list. That is a deliberate shape,
not an oversight, and it is the shape to match.

And one thing already leaks the other way, which is the sharpest argument that
this is worth fixing:

```
test -e ./NUL      -> exists
test -e /dev/null  -> no
```

Windows resolves `NUL` in *every* directory, so the device that is not supposed
to be visible is, under a name nobody wants, while the one that is supposed to be
is not. `ls ./NUL` prints `./NUL`.

## One more measurement that shapes the options

Windows has real device objects and Go can stat them:

```
os.Stat("NUL")   -> mode=Dcrw-rw-rw- size=0
os.Stat("CON")   -> mode=Dcrw-rw-rw- size=0
os.OpenFile("NUL", O_WRONLY) -> works
```

`Dcrw-rw-rw-` against busybox's `crw-rw-rw-` for the same path: the platform
already answers this question for `/dev/null` and `/dev/tty` almost exactly the
way the reference does. It answers nothing for `/dev/zero`, `/dev/urandom`,
`/dev/clipboard` or `/dev/fd/N`, which have no Windows counterpart.

## Option A — a synthetic stat table

One table of device names to a synthetic `fs.FileInfo`: character-device mode,
`0666`, size zero, epoch mtime. The path model already flags `Device: true`; the
stat seam consults the table instead of the filesystem when it sees that flag.

**For**

- It is what the reference does, so the two divergences above close exactly, and
  `crw-rw-rw-` is reproduced rather than approximated.
- Small and contained. The device list already exists; this gives it a second
  column.
- No new abstraction, so nothing invites `/proc` and `/sys` in behind it.
- The risky path -- opening and reading -- is untouched. A bug in this can only
  make `ls` or `test` wrong, never corrupt a redirect.

**Against**

- **Nineteen stat call sites** across ten files in `internal/applets` call
  `os.Stat` or `os.Lstat` directly. Either they each learn about devices, which
  is nineteen chances to forget, or a shared seam is introduced first -- and that
  seam is most of the work in Option C anyway.
- Still not enumerable, so `ls /dev` stays an error. Matching the reference here
  means matching it in being unhelpful: there is no way to discover what devices
  exist except reading documentation.
- Two sources of truth unless the table is shared with the open dispatch. The
  list of devices must be one list, or `test -e /dev/zero` will one day disagree
  with `cat /dev/zero`.

## Option B — map to the native device where Windows has one

`/dev/null` becomes native `NUL`, `/dev/tty` becomes `CON`, and the ordinary
filesystem path handles both. No synthesis for the two that matter most.

**For**

- `ls -l /dev/null` prints what Windows itself says, and cannot drift from it.
- A child process can be handed a path it can open, which a synthetic stat can
  never do. `NUL` is what every Windows program already expects.
- Removes code rather than adding it, for those two names.

**Against**

- It covers **two** of the nine. `/dev/zero`, `/dev/urandom`, `/dev/clipboard`
  and `/dev/fd/N` still need something else, so the result is two mechanisms
  where there was one -- and the seam between them is exactly where a
  divergence will hide.
- `NUL` is magic in *every* directory, so a native mapping puts a name into the
  path model that behaves unlike every other name in it. The `./NUL` row above
  is that behaviour leaking already; blessing it would make it a feature.
- `CON` is not `/dev/tty`. It is not the controlling terminal in the POSIX
  sense, there is no controlling terminal here, and `docs/design/v1-scope.md`
  records that `/dev/tty` is deferred *by the device model itself*. Mapping it
  would quietly promise something the shell cannot keep.

## Option C — a device filesystem behind one interface

An interface with `Open`, `Stat` and `ReadDir`; `/dev` is the first
implementation; the path model dispatches on `Device: true`.

**For**

- **One table, one source of truth.** The open dispatch, the stat answer and the
  directory listing all read the same list, so they cannot disagree.
- `ls /dev` works, which is how a person discovers what is there. This is better
  than the reference rather than equal to it, and it is the only option that
  makes the feature discoverable at all.
- Testable with no operating system: a fake device tree is a struct.
- There is an obvious second tenant. `internal/proc` already reads the whole
  process table, so `/proc/<pid>/` is a small step once the seam exists -- and
  that is when the abstraction pays for itself rather than costing.

**Against**

- **The largest change of the three**, and the one that touches the path model,
  which every applet depends on. The nineteen stat sites still have to route
  through it; the interface does not remove that work, it only gives it one
  destination.
- `ls /dev` **diverges from the reference**, which answers
  `No such file or directory`. That needs justifying in
  `docs/support-matrix.md` rather than shipping as a surprise, and this project's
  rule is that busybox-w32 is the primary reference unless a different one is
  chosen deliberately.
- It invites scope. `/proc` is defensible; `/sys`, `/dev/disk/by-id` and a
  writable `/dev` are not, and an interface makes each of them look like a small
  addition.
- `find /` and `du /` meet a synthetic tree and have to decide whether to
  recurse into it. Every one of those is a separate decision, and getting one
  wrong means a `find` that never terminates or a `du` that reports a machine
  with infinite disk.

## Recommendation

**Option A, with the table shared with the open dispatch, and a shared stat seam
introduced first.** Then Option C only if `/proc` is actually wanted.

The reasoning is the ordering rather than the destination. What is wrong today is
narrow and measurable: two divergences from the reference, on `test -e` and
`ls -l`, for paths this shell already opens correctly. Option A closes exactly
those, and the shared stat seam it needs is the part of Option C worth having on
its own -- nineteen direct `os.Stat` calls is the real problem, and it is a
problem whether or not `/dev` is ever a filesystem.

Building C first means adopting an abstraction for one tenant, diverging from the
reference on `ls /dev` before the cheaper divergences are closed, and taking on
the `find /` question for no present gain.

B is rejected as a design but kept as a **test oracle**: `os.Stat("NUL")` is what
Windows says a null device looks like, and the synthetic answer in Option A
should be checked against it rather than invented.

## What would have to be decided before writing any of it

- **Does `ls /dev` list, or say it does not exist?** The reference says the
  latter. Listing is more useful and is a divergence to record.
- **What do `find /` and `du /` do at `/dev`?** Not recursing is almost certainly
  right, and "almost certainly" is not a specification.
- **Is `/dev` writable?** `> /dev/clipboard` already works, so partly. Whether
  `touch /dev/foo` refuses -- and with what message -- is unspecified today.
- **`/dev/tty`.** Deferred, and this document does not change that.
