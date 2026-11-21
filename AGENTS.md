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
- `third_party/gdamore-encoding` is a fork of the module tcell imports, which go.mod's
  `replace` links in place of upstream: its character sets are generated data rather
  than 1,103 allocations in `init` on every start. Its README says what changed and how
  to follow upstream. `third_party/lucasb-eyer-go-colorful` is go-colorful without its
  `HexColor`, which linked `database/sql/driver` and `encoding/json` for nothing tcell uses.
  `third_party/ulikunitz-xz` takes its SHA-256 check from CNG rather than `crypto/sha256`,
  which brought Go's FIPS entropy source and its 32 MiB of commit into every process, and
  `third_party/x-sys` is x/sys without the one method that imported `net`, whose init
  started Winsock on every start. The price of a `replace` is that
  `go install github.com/xiongnemo/nemosh/cmd/nemosh@latest` refuses to build; nothing
  here documents that way of installing, and the release artifacts are unaffected.
- The published artifact must be the same binary CI verified. No rebuild at
  release time.
- **Windows is the only platform, from 2026-11-17, in three architectures**:
  windows/amd64, windows/arm64 and windows/386 are each built, tested and
  published -- arm64 natively on GitHub's Arm runner, 386 under WOW64 on the x64
  one. windows/arm, 32-bit, is no target Go has had since 1.26. Nothing is written
  for another GOOS -- no `_other` or `!windows` file, no `runtime.GOOS` branch --
  and a Windows API is called directly where one does the job, as busybox-w32
  does. Until then Linux and macOS were build-and-test checks on CI, with binaries
  published as an unsupported convenience; the lessons that were only about them
  are gone from this file.
- Distribution is Scoop, through
  [`xiongnemo/windows-binaries-scoop-bucket`](https://github.com/xiongnemo/windows-binaries-scoop-bucket).
  The publish job announces each release to
  [`xiongnemo/packages`](https://github.com/xiongnemo/packages), which dispatches
  the bucket's Excavator once GitHub's releases list shows the Windows asset,
  rather than leaving the bucket to notice on its hourly cron -- a cron sweep
  packages only whichever release is newest when it fires, so releases published
  between two sweeps were being skipped outright. Until 2026-10-22 this job
  dispatched the bucket itself; one announcement now reaches every channel.
- **The bucket computes the hash itself, and that is the point.** The release
  job knows the SHA-256 already and could write the manifest directly, which
  would be faster; it does not, because a job that both produces a file and
  declares its hash agrees with itself no matter what went wrong in between.
  Anything that replaces the Excavator has to keep hashing the published
  artifact.
- Until 2026-11-17 the Linux binaries were also packaged as `.deb`, `.rpm` and
  `.apk` with nFPM and indexed for apt/dnf/apk at https://package.anzupop.com by
  `xiongnemo/packages`. They went with the platform; the dispatch stays, for Scoop.

## Tests And Gates

Before handing off substantive changes, all of:

```bash
gofmt -l .                                    # empty
go vet ./...
NEMOSH_DIFFERENTIAL=strict go test -race -shuffle=on -count=1 ./...
```

plus the 250 pure-LOC ceiling per production `.go` file:

```bash
for f in $(git ls-files '*.go' | grep -v _test.go | grep -v '^third_party/'); do
  n=$(grep -v '^\s*$' "$f" | grep -v '^\s*//' | wc -l)
  [ "$n" -gt 250 ] && echo "OVER: $f $n"
done
```

`third_party/` is excluded because it is upstream's code and a generated table, not this
project's. Each fork there is a module of its own, which `./...` does not reach, so the gate
vets and tests each by name, with the flags in its `vet.flags` if it has one (x-sys's turns
off `unsafeptr`, which reports what its generated syscall wrappers must do):

```bash
for module in third_party/*/; do go -C "$module" vet $(cat "${module}vet.flags" 2>/dev/null) ./... && go -C "$module" test ./...; done
```

Strict TDD: one failing test first, captured, then the minimum production
change. Every commit uses `git commit --no-gpg-sign`.

**A change to how the shell behaves also runs the Oils spec suite.** `tests/oils` is
Oils' own test cases, vendored as a measuring instrument (`tests/oils/README.md`), and
Windows CI holds nemosh to `tests/oils/baseline.json` with it: a case that starts
passing fails the build as surely as one that stops. After such a change, record where
the shell now stands, and commit the baseline and the page it renders with the change:

```bash
NEMOSH_OILS=update go test ./internal/testutil/oilsspec/ -run TestOilsSpec -count=1 -timeout 30m -v
NEMOSH_OILS=update NEMOSH_OILS_AS=sh go test ./internal/testutil/oilsspec/ -run TestOilsSpec -count=1 -timeout 30m -v
git add tests/oils/baseline.json tests/oils/baseline-sh.json docs/testing/oils-spec.md
```

The second run is nemosh as `sh`, held to `tests/oils/baseline-sh.json`. Run as bash --
`nemosh bash`, a `#!/bin/bash` script, or nemosh with `NEMOSH_COMPAT=bash` -- the shell is in
bash mode and follows bash where bash and busybox part (`r.BashMode()`, and the table in
`docs/support-matrix.md`, Bash mode), and as sh keeps busybox's way; that baseline is what
proves a change made for bash mode left the sh name where it was. A difference that is not a
conflict between the two references -- a defect both agree on, or something only bash has --
is not put behind the mode.

Where the suite is not to be run on the development machine, run it on CI instead: a pre-check
branch whose strict run fails uploads the baselines it would record as the `oils-proposed`
artifact (`tests/oils/README.md` has the two commands that take them), and those go into the
commit that moved the cases.

It takes about a minute here. `NEMOSH_OILS=strict` is what CI runs, and a case that
differs is run twice more, alone, before either mode believes it. The headline -- of the
cases bash passes, how many nemosh passes -- is in `docs/testing/oils-spec.md`.

**Green tests do not imply correctness.** Two defects found on 2026-08-07 passed
the entire suite: `times` reported 215 years because its test asserted only the
`%dm%fs` shape, and a brace was treated as reserved outside command position.
Assert values, not just shapes, and re-measure claims against a built binary
rather than against the suite.

It went further than that on 2026-09-13: the prompt held the terminal in raw mode
across every command it ran, so `bc` typed at the prompt was unusable, and not one
of ~3,000 tests noticed -- every one of them fed input through a `strings.Reader`,
which is indistinguishable from a session that has already ended. What a person has
to check by hand, and what is already covered so they need not, is
`docs/testing/manual-checks.md`.

**The local gate is not CI.** The runner is a different Windows machine, which owns
its files differently, puts its temporary directory under `HOME`, and has other
programs on `PATH`; four commits went in on 2026-08-22 before anyone looked, and all
four were red. **Check `gh run list` after pushing**, not after four pushes.

On 2026-08-23 **six** pushes went red on the same step, the binary size ceiling, and
the local gate cannot see it because the ceilings live only in `product.yml`. The
full gate in this file runs `gofmt`, `vet`, the suite and the 250-line loop -- it
does **not** build a release binary or measure one. So a stage
that adds a dependency (`net/http` cost 2.07 MiB on its own) passes locally every
time and fails on every runner. After a stage that adds an import nothing in this
tree had before, measure it:

```bash
CGO_ENABLED=0 go build -trimpath -tags tcell_minimal -ldflags "-s -w $(bash scripts/version.sh --ldflags)"   -o dist/nemosh.exe ./cmd/nemosh
wc -c < dist/nemosh.exe                                  # against product.yml's ceiling
go run ./tools/startbench -head dist/nemosh.exe -base dist/nemosh-base.exe -self-check
```

`tools/startbench` is what CI runs in its "package init and startup time" step: the
package init allocations against a ceiling, a waiting session's commit charge against
another (32 MiB; Go's crypto packages alone cost 32 of their own, committed and never
touched, until they went), and the time of four command lines against a base binary -- build `HEAD` into `dist/nemosh-base.exe` from a worktree to compare a
working tree with what it started from. Both binaries must be named `nemosh-*`; one
called `head.exe` answers to its name and runs the `head` applet. Locally the init count
reads about 32 higher than on the runner, so compare it against a base built here rather
than against the ceiling. Time is never compared against a number, only against the
base, interleaved, in rounds; see `docs/design/startup-and-footprint.md`. A scenario 10%
and a floor slower in every round fails the build; a commit that is slower on purpose
says why in a `Startup-Cost-Accepted: <why>` trailer and is reported instead.

**windows/386 is where an int is 32 bits**, and the only place. Running the suite
as 386 the first time, on 2026-11-17, found strptime's `%s` range a constant no int
held, an array index that went to `a[-1]`, head's counts read to the largest int,
and `top`'s thread and processor records read at Go's size where Windows lays them
at C's -- C aligns a LARGE_INTEGER to 8, Go on 386 an int64 to 4 -- along with a
command-line length WOW64 hands a 32-bit process as 0 minus 8. A count, an offset
or an index the references keep in 64 bits is an int64 here whatever it is built
for, and an array of a Windows struct is stepped at the size C gives it.
`GOARCH=386 go test ./...` runs on this machine, WOW64 running it; arm64 runs only
on the runner, and its ints and structs are amd64's. CI's `architectures` job runs
both on every push.

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

**A test that asserts on a background job can pass by winning a race.**
`TestRuntime_outerJobBecomesDoneAfterNestedScopeDrains` failed once on macos-latest
on 2026-09-13 and was green everywhere else, which reads like a flake and was really
the test never holding its property. Its nested applet returned on `ctx.Done()`, and
`commandSubstitutionScript` ends with `cancelAndDrain`, so the substitution cancelled
it immediately and the `release` channel was holding nothing open: the drain, the
job's own write and the job finishing all ran concurrently with the assertion.

**The way to check is to inject a delay**, which is cheap and decisive — a
`time.Sleep(50 * time.Millisecond)` before the assertion made it fail every time, and
report `[1] Done` rather than the `[1] Running` it claims to test, so on a slow enough
runner it had been testing nothing. Injecting the same pause at the other nine
`<-started` sites in that package changed nothing, which is how one real defect was
told apart from nine sound tests.

A background job's output is ordered against the test's own only by a channel, never
by being written first. If a helper applet selects on `ctx.Done()`, ask who cancels
that context: in a command substitution, a group, or a pipeline stage, something does,
and then the release channel is decoration. Blocking on release alone is also what a
real command that does not die on cancel does.

**An applet never spawns an OS process.** Launching belongs to
`internal/shell/runtime`, which owns the suffix search, `ComSpec` batch launch, argument
quoting, long-path fallback, cancellation and job scope that go with it; every production
`os/exec` import in the tree is there. An applet that must run something asks `commandFor`
(`program_runner.go`): an applet of that name first, as busybox-w32 prefers one, and
otherwise the `ProgramRunner` the shell put in the context, which launches it with all of the
above -- env, xargs, `find -exec` and awk's `system()` and pipes all go that way.
`TestApplets_doNotSpawnProcesses` enforces this. It is
about *spawning*: `syscall` and `x/sys/windows` are used freely here for file metadata and
process inspection, and the first draft of that guard banned them and was wrong.
See `docs/design/windows-execution-model.md`.

**A capability that is absent must fail loudly.** `ulimit`, `fg`, `bg`, `suspend`,
`bind`, `enable -n`, and `set -b`/`-m`/`-v` refuse with a reason and a non-zero status
rather than approximating. Anything landing partially refuses the part it cannot do.

**A fixture must not depend on where the machine puts things.** `dirs` abbreviates the
home directory to `~`, and the directory-stack tests hid a temporary path by substituting
its full spelling. On this machine `HOME` is not a prefix of the temp directory and the
substitution worked; on a GitHub runner it *is* one, so every path came back as
`~/AppData/Local/Temp/...` and eighteen cases failed. Green here, red there, twice over --
the Product and Release workflows both run the suite. Pin whatever the assertion depends
on (here, `HOME=` to somewhere that cannot be a prefix) rather than assuming the layout
this machine happens to have.

**A pid a test makes up is a real process's.** top's filter test built three processes,
pids 11, 12 and 13, and looked their details up through the cache top uses, which opens
the process a pid names. Windows ignores a pid's low two bits -- `OpenProcess` of 11
opens 8, of 13 opens 12 -- so the test read the command lines of whatever held those
numbers. It had passed since August; then on a runner the process at 8 had `nemosh` in
its command line, the filter for nemosh took `chrome.exe` with it, and the commit about
to be tagged v1.4.0 went red. Code that asks the system about a process takes the asking
as a parameter, and a test that makes processes up passes one that knows nothing, as
`noDetails` in `top_model_test.go` does.

**A test that synthesises its own input can agree with itself and be wrong.**
The editor bound `^_` as `tcell.KeyCtrlUnderscore` and the test pressed
`tcell.KeyCtrlUnderscore`. Both agreed, and the key did nothing on Windows —
found only when someone pressed it on a real keyboard. Where a test builds the
event it asserts on, it proves the code equals itself and nothing more; check
that a device can produce that event, or drive it through the layer that does.

**tcell has two input paths and they do not agree.** Measured, after two wrong
guesses:

- On a terminal (`input.go:450-452`) a control byte becomes
  `NewEventKey(KeyCtrlSpace+Key(r), 0, ModCtrl)`. `KeyCtrlSpace` is 64, so `0x1F`
  is `Key(95)` — `KeyCtrlUnderscore`, exactly as named. The `KeyCtrl*` names are
  correct here.
- On Windows, where the terminal screen cannot start, input goes through `console_win.go`,
  which for a control character whose modifier mask is *exactly* Ctrl adds `0x60`
  back and posts a rune (`:725-736`). `^_` typed as Ctrl+`-` is therefore
  `0x1F + 0x60 = 0x7F`, which tcell reads as Backspace — it *deletes*. Typed as
  Ctrl+Shift+`-` the mask is Ctrl|Shift, the addition is skipped, and it arrives
  as `Key(31)` — `KeyUS`, a different constant for the same chord.

So a Ctrl chord may need both spellings bound. Letters are safe by accident:
`key.go:276` maps `a`-`z`+`ModCtrl` onto `KeyCtrlA+n`, which is the same 64-based
numbering, so both paths agree. Punctuation has no such mapping and does not.
Alt with a letter is reported by the Windows console with no character at all and
the event is dropped (`:741-744`), so Escape-then-key is the meta spelling that
survives both — and on the terminal path tcell already implements it
(`input.go:104-110` sets `ModAlt` on the key after Escape).

**Which path a Windows console takes was measured on 2026-11-21, and it is the
terminal one.** tcell 2.13.10's `NewScreen` tries its terminal screen first
(`screen.go:312-320`), which `tscreen_win.go` gives a Windows console as
`xterm-truecolor`, and on Windows 10's conhost a probe logging every event got
`*tcell.baseScreen`: keys written into the console by a robot arrived through
`input.go`. So **Escape and a key typed within 50 ms of it are that key with
`ModAlt`** -- a quick `<Esc>:wq` is Alt+`:` -- and a lone Escape arrives 50 ms
later; Ctrl+K is `KeyCtrlK` with `ModCtrl`. `console_win.go` is the fallback, and
the `^_` measurements above were taken on it, so both spellings stay bound. A robot
writing key-*up* records as well doubles every key on the terminal path (Up then
Ctrl+Space, Backtab then Tab); write key-down records alone.

## Documentation Hygiene

After changing commands, flags, applet coverage, build scripts, release
packaging, or versioning, check whether `README.md` and the support matrix need
updating. A change to what an everyday Debian invocation does updates its row in
`docs/testing/linux-habits.md`, and the counts at the top of that page. Keep comments that explain intent; rewrite a stale one rather than
deleting the context.

## Not Yet Applicable

`go-tview-template`'s TUI, raw-TTY, and keybinding conventions do not apply to
today's line-oriented REPL, but they govern the v2 interactive tier — the editor
bakeoff, completion UI, and terminal-state restoration. Consult them then rather
than reinventing.
