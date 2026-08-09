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

**A capability that is absent must fail loudly.** `hash`, `ulimit`, `fg`, `bg`,
and `set -b`/`-n`/`-v` refuse with a reason and a non-zero status rather than
approximating. Anything landing partially refuses the part it cannot do.

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
