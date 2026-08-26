# AGENTS.md

## References

- Nemosh design should primarily reference busybox-w32 behavior and implementation choices unless a different reference is explicitly selected for a specific subsystem.

## Versioning

Adapted from `xiongnemo/go-tview-template`, which is where this convention was
written down first. It is repeated here because it was needed twice and found
neither time: this repository is the authority for how Nemosh is versioned.

Version format for build and release artifacts:

```text
v{major}.{minor}.{patch}-{branch}-{commit12}[-dirty]
```

- Derive `major.minor.patch` from the nearest exact semver tag matching
  `vMAJOR.MINOR.PATCH`, **plus the number of commits since that tag**. So the
  patch number advances by itself with each commit and is never hand-maintained.
- Ignore prerelease/dev tags such as `v0.1.2-dev-abcdef123456` when computing
  the base. They must not become the base for future patch calculations.
- `v0.0.1` is the local fallback base before any exact semver tag exists. It is
  for local and dev builds only, never for a package-manager update channel.
- To bump major/minor or reset the patch base, create and push a new exact
  semver tag on the desired base commit. That commit then builds as the tag; the
  next commit builds as patch + 1.
- **A clean build of exactly a tagged commit renders bare**: `v0.1.0`, with no
  branch or commit. That build *is* the release, and qualifying it would leave
  one version wearing three spellings — a release named `v0.1.0`, an artifact
  named `nemosh-v0.1.0-release-703714b2edcc-…`, and a binary reporting a third.
  Dirty is never hidden, even on a tag, and the `v0.0.1` fallback never renders
  bare because nobody tagged it.
- Sanitize branch names before placing them in version strings or artifact
  names. Use a 12-character commit hash. Append `-dirty` when uncommitted
  changes are part of the build.
- **Only two tag shapes exist**, and the difference is the mechanism rather than
  a convention: an exact `vMAJOR.MINOR.PATCH` base, and a
  `vX.Y.Z-branch-commit12[-dirty]` build marker that `--exclude='*-*'` keeps out
  of base computation. Measured: with only a `v0.1.0-release-57b96893ed4d` tag
  reachable, `git describe --exclude='*-*'` reports "No names found" and every
  build falls back to `v0.0.1` forever. The shapes therefore cannot be unified.
  `.github/workflows/tag-guard.yml` rejects a third shape; a repository ruleset
  on `refs/tags` is what would prevent one being pushed at all.
- Inject build metadata with `-ldflags -X` into `internal/version`. That package
  falls back to `debug.ReadBuildInfo()` when ldflags are absent, so `go run` and
  plain `go build` still report useful VCS metadata.
- Expose the version through `nemosh --version`, and keep it usable without a
  terminal.

**Before adding CI prereleases, package-manager manifests, or Scoop update
support, ask whether to start the formal version line.** That question was asked
and answered on 2026-08-07: the first exact semver tag is **`v0.1.0`**, the MVP
baseline, taken once v0 was complete and audited. `v1.0.0` is tagged only after
V1-RC clean-machine acceptance passes; see `docs/design/v1-scope.md`.

CI must check out with `fetch-depth: 0`, because the version is derived from
tags and a shallow clone silently yields the fallback.

## Builds And Packaging

- The release target is one executable, `CGO_ENABLED=0`, no runtime sidecars.
- Release builds use `-trimpath` and `-ldflags="-s -w ${VERSION_LDFLAGS}"`.
- Generated artifacts go under `dist/`, with checksums.
- The published artifact must be the same binary CI verified. No rebuild at
  release time.
- Keep cross-build matrices opt-in. Do not build every target
  `go tool dist list` reports; a new Go release may add targets this repository
  has not validated.
- Windows is the supported platform. Linux and macOS are build-and-test checks,
  not support commitments; see the support matrix in `README.md`.
- Distribution is Scoop, through
  [`xiongnemo/windows-binaries-scoop-bucket`](https://github.com/xiongnemo/windows-binaries-scoop-bucket).
  The publish job dispatches that bucket's Excavator when it finishes, rather
  than leaving the bucket to notice on its hourly cron -- a cron sweep packages
  only whichever release is newest when it fires, so releases published between
  two sweeps were being skipped outright.
- **The bucket computes the hash itself, and that is the point.** The release
  job knows the SHA-256 already and could write the manifest directly, which
  would be faster; it does not, because a job that both produces a file and
  declares its hash agrees with itself no matter what went wrong in between.
  Anything that replaces the Excavator has to keep hashing the published
  artifact.

## Tests And Gates

Before handing off substantive changes, all of:

```bash
gofmt -l .                                    # empty
go vet ./... && GOOS=linux go vet ./...
GOOS=darwin go build ./...
NEMOSH_DIFFERENTIAL=strict go test -race -shuffle=on -count=1 ./...
```

plus the 250 pure-LOC ceiling per production `.go` file:

```bash
for f in $(git ls-files '*.go' | grep -v _test.go); do
  n=$(grep -v '^\s*$' "$f" | grep -v '^\s*//' | wc -l)
  [ "$n" -gt 250 ] && echo "OVER: $f $n"
done
```

Strict TDD: one failing test first, captured, then the minimum production
change. Every commit uses `git commit --no-gpg-sign`.

**Green tests do not imply correctness.** Two defects found on 2026-08-07 passed
the entire suite: `times` reported 215 years because its test asserted only the
`%dm%fs` shape, and a brace was treated as reserved outside command position.
Assert values, not just shapes, and re-measure claims against a built binary
rather than against the suite.

**The local gate is Windows-only, and CI is not.** A green run here says nothing
about ubuntu and macos, which run the same suite. Four commits went in on
2026-08-22 before anyone looked, and all four failed CI for one reason:

> **A directory's apparent size is platform-dependent.** Windows reports 0,
> Linux and macOS report the directory block — 4096 and 96 respectively.

So `find . -size -1c` matched the walk root on Windows and not elsewhere, and
`ls -S` put a subdirectory last here and first there. Both were assertions that
had quietly encoded a Windows fact as the answer. When a test compares sizes,
either constrain it to files (`-type f`, a fixture with no subdirectory) or
expect a different answer per platform — and **check `gh run list` after pushing**
rather than after four pushes. Anything reading `os.FileInfo` off a directory
deserves the same suspicion: size, mode bits, and link count all differ.

It happened again on 2026-08-23, worse and for a different reason: **six** pushes
went red on the same step, the binary size ceiling, and the local gate cannot see
it because the ceilings live only in `product.yml`. The full gate in this file
runs `gofmt`, `vet`, the cross-platform vet, the darwin build, the suite and the
250-line loop — it does **not** build a release binary or measure one. So a stage
that adds a dependency (`net/http` cost 2.07 MiB on its own) passes locally every
time and fails on every runner. After a stage that adds an import nothing in this
tree had before, measure it:

```bash
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w $(bash scripts/version.sh --ldflags)"   -o dist/nemosh.exe ./cmd/nemosh
wc -c < dist/nemosh.exe                                  # against product.yml's ceiling
GODEBUG=inittrace=1 dist/nemosh.exe --version 2>&1 | grep '^init '
```

**The specific trap, a third time, on 2026-08-23: a test that samples the machine.**
`internal/proc` implements listing on Windows only and answers
`ErrListUnsupported` elsewhere rather than guessing, so *any* test that reaches
`top`, `ps` or `free` for real passes here and fails on both other runners. The guard
already exists and is worth copying rather than reinventing --
`startTop` in `top_view_test.go` skips on `runtime.GOOS != "windows"`, and
`top_batch_test.go` now has `requireProcessSampling` for the same reason. Before
writing a test that reads the process table, ask which of the three machines can
answer it.

Cross-compiling the tests catches the *build* half of this and nothing else:

```bash
GOOS=linux go test -c -o /dev/null ./internal/applets/    # compiles, says nothing about behaviour
GOOS=darwin go test -c -o /dev/null ./internal/applets/
```

Running the suite under WSL was tried and does not work here: the Debian image has
Go 1.23 against this module's 1.26, and fetching a toolchain needs network the
sandbox does not have. So CI is the only real cross-platform check, which is the
reason to read the run list rather than to assume.

**Per-package coverage understates anything driven from another package.**
`go test -coverprofile ./internal/applets/` reported `device_walk.go` at 14.3% and it
is really at 61.9%: the `/dev` walk needs a view that only `internal/shell/runtime`
implements, so `internal/shell/runtime/device_walk_test.go` is what exercises it and
the applets profile cannot see that. The same is true of anything reached only
through the shell. Before concluding a file is untested, check with:

```bash
go test -coverpkg=./internal/applets/ -coverprofile=c.out ./internal/shell/runtime/
```

and note that a `-coverpkg` run over *several* test packages writes one profile per
binary, so summing blocks by hand double-counts and reads far too low. `go tool cover
-func` merges them correctly; hand-rolled awk over the raw profile does not.

**The clipboard tests fail at random on the development machine, and it is not
your change.** Windows 10 and later run a clipboard history service that opens the
clipboard on every change, and any clipboard manager does the same, so
`/dev/clipboard` loses the race whenever something else is holding it. The failure
is `Thread does not have a clipboard open` and it is almost perfectly random:
measured 2026-08-21, the same test at the same commit passed once and failed four
times in five runs, while `v0.1.0` -- which predates every change that day --
failed five times out of five. **A single bisect run per commit will lie to you.**
It did: it identified an unrelated UTF-16 commit as the cause, convincingly,
because each commit was tried once.

If a clipboard test fails, run it five times at HEAD and five times at a commit
from before your work before concluding anything. What was actually wrong -- the
read not retrying, where the open already did -- was found by measuring the write
and read separately: with no pause between them the read failed 30 times out of
30, and with one millisecond it succeeded 20 times out of 20.

**A capability that is absent must fail loudly.** `hash`, `ulimit`, `fg`, `bg`,
and `set -b`/`-n`/`-v` refuse with a reason and a non-zero status rather than
approximating. Anything landing partially refuses the part it cannot do.

**A test that synthesises its own input can agree with itself and be wrong.**
`tcell.KeyCtrlUnderscore` is 95, and *nothing produces 95*: a terminal sends
`0x1F` for `^_`, which tcell turns into `Key(31)` — `KeyUS`. The `KeyCtrl*` block
is numbered from `KeyCtrlA = 65`, so those constants are the letters, and a
Ctrl'd letter only reaches them because `key.go:276` maps `a`-`z`+`ModCtrl` onto
them. Punctuation gets no such mapping. The editor bound `KeyCtrlUnderscore`, the
test pressed `KeyCtrlUnderscore`, both agreed, and the key had never worked on any
platform — found only when it was pressed on a real keyboard. Where a test builds
the event it asserts on, check that a device can produce that event.

Same area, second trap: on Windows tcell has **no VT screen**, so input goes
through `console_win.go`, which for a control character with Ctrl held adds `0x60`
back and posts a rune (`:725-736`). `^_` is therefore `0x1F + 0x60 = 0x7F`, which
tcell reads as Backspace — so `^_` on Windows *deletes* rather than doing nothing.
Alt with a letter is reported with no character at all and the event is dropped
(`:741-744`). Escape-then-key is the only meta spelling that survives both.

## Documentation Hygiene

After changing commands, flags, applet coverage, build scripts, release
packaging, or versioning, check whether `README.md` and the support matrix need
updating. Keep comments that explain intent; rewrite a stale one rather than
deleting the context.

## Not Yet Applicable

`go-tview-template`'s TUI, raw-TTY, and keybinding conventions do not apply to
today's line-oriented REPL, but they govern the v2 interactive tier — the editor
bakeoff, completion UI, and terminal-state restoration. Consult them then rather
than reinventing.
