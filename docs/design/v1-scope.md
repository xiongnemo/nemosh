# V1 Scope

**V1.0 does not add shell semantics. It turns the finished v0 into something a
person can install, trust, and report bugs against.**

## Where This Came From, Including One Correction

This document had to be rewritten once, and the reason belongs at the top.

A first draft was derived bottom-up from v0's own remainder columns, and made
the applet option matrices, `~user`, and completion stages M0–M2 the v1
must-have. That draft was wrong about intent, not about facts. The original
roadmap already existed — not in the repository, but in the oh-my-opencode
session that built v0 (`ses_0c8d3d890ffe3aHXZ0MIUfV3O1`, 2026-07-06 to
2026-08-01). On **2026-07-18** the question "到 v1 的完整计划是啥" was answered
there with a full route, and its conclusion was the opposite of that draft:

> v1.0 不新增大型 shell 语义，而是把完成的 v0 变成可信赖产品。

That answer also froze the v0 route as `P0.4 → P0.5 → P1.1 → P1.2 → v0 全 scope
审计`, which is now complete, and flagged exactly one product fork — whether
v1.0 includes the full interactive REPL and completion — as needing a decision.
**That fork was never answered at the time.** It was resolved on **2026-08-07**
in favour of a merge: the release route as written, plus the two correctness
debts that should not cross a release boundary.

So the sources are:

- the 2026-07-18 roadmap above, for the shape of V1-A/B/C/RC;
- the **Remaining debt** of `.omo/plans/input-cancellation-io-number-fixes.md:101-117`,
  for the two debts folded in;
- the six **Deferred Non-Goals** of `docs/design/v0-scope.md:151-158`;
- the remainder columns of `docs/design/v0-readiness.md`, which supply the
  v1.1 backlog rather than the v1.0 scope.

## What V0 Established That V1 Must Not Regress

Load-bearing properties, not achievements to restate. Each constrains how v1
work may be written.

- **A capability that is absent must fail loudly.** This is the rule the v0
  readiness audit was corrected under, and it is why `hash`, `ulimit`, `fg`,
  `bg`, and `set -b`/`-n`/`-v` refuse with a reason and a non-zero status rather
  than approximating. A release does not get to soften this into silence.
- **Parse before effects.** A script is parsed in full before any of it runs.
- **busybox-w32 is the primary behavior reference**, then BusyBox ash, then dash
  and POSIX, verified against `references/windows-compat/busybox-w32`.
- **Green tests do not imply completion.** Both defects found on 2026-08-07 —
  `times` reporting 215 years, and a brace treated as reserved outside command
  position — passed the entire suite. Shape assertions hide wrong values.
- **The 250 pure-LOC ceiling** per production `.go` file, and the differential
  corpus against busybox-w32/ash, dash, and bash.

## V1.0 Must Have

### V1-A — Freeze the release contract

Measured 2026-08-07: **`README.md`, `LICENSE`, `CHANGELOG.md`, `SECURITY.md`,
and any install document are all absent**, and both `nemosh --version` and
`nemosh --list` answer `invalid option`. A public repository without a licence
is the most consequential gap in this document; it is not a documentation
nicety, it decides whether anyone may legally use the thing.

- Supported OS and architecture matrix, and the initial applet manifest, both
  stated as a contract rather than inferred from the registry.
- Compatibility statement and the known-divergence list. v0 already records
  divergences per behavior case; this surfaces them as a user-facing table.
- SemVer policy, release candidates, tagging, and rollback/yank rules.
- A single source of version truth, surfaced by `nemosh --version`, and
  `nemosh --list` over the same manifest the applet-freshness gate already
  checks.
- `README.md`, install documentation, `CHANGELOG.md`, `LICENSE`, third-party
  notices, `SECURITY.md`.

### V1-B — Product CI, security, and performance

Product CI already exists (`.github/workflows/product.yml`, both lanes green)
and is the one item of this section that v0 delivered. The rest is new:

- `govulncheck` and a dependency audit in the same workflow.
- Fuzzing where the parser actually is: shell word parsing, path resolution, and
  quoting. The brace defect found on 2026-08-07 is the shape of bug fuzzing
  finds.
- Resource-leak and long-running stress coverage for goroutines, handles, and
  the job supervisor.
- **Pin GitHub Actions to commit SHAs.** Currently `actions/checkout@v4`,
  `actions/setup-go@v5`, and `msys2/setup-msys2@v2` are tags, which are mutable.
  Apply least-privilege permissions in the same pass.
- Performance baselines with regression thresholds: startup, `-c`, a pipeline, a
  representative applet, memory/handle/goroutine counts, and binary size.

### V1-C — Reproducible packaging and distribution

- A single multicall binary.
- Windows Scoop-first: generate the manifest, generate shims from the
  authoritative applet list, and test clean install, shim invocation, upgrade,
  and uninstall.
- Archives or packages for the supported Unix platforms.
- `-trimpath` reproducible builds, checksums, SBOM, signing, provenance.
- **The published artifact must be the same binary CI verified.** No rebuild at
  release time.

### V1-RC — Clean-machine acceptance

On a fresh Windows install and a supported Unix host: install, upgrade,
downgrade/rollback, uninstall; multicall and every shim; the behavior corpus;
Unicode, long paths, CRLF, environment-variable case collision; the job and
signal limits and the known compatibility divergences; package checksum and
signature; and every command in the documentation, executed as written.

Passing all of that freezes the compatibility table, generates release notes,
and ships `v1.0.0`.

### Folded in: two correctness debts

These are the merge half of the 2026-08-07 decision. Both are cheap, both are
correctness rather than features, and neither should cross a release boundary.

1. **Windows real-console cancellation acceptance.** Interrupt coverage today
   exercises anonymous pipes and regular files. A test that allocates a real
   child console and opens `CONIN$` would exercise `ReadConsole` plus
   `CancelSynchronousIo` — the path a user actually hits. A one-off
   `GenerateConsoleCtrlEvent` probe passed during the v0 review for both `cat`
   forms but was never checked in. This is also where the three interactive
   interrupt tests skipped on non-Windows
   (`cmd/nemosh/interactive_interrupt_platform_test.go`) gain a native
   counterpart.
2. **Cancellation ownership.** `copyWithContext` in `internal/applets/files.go`
   is redundant: `contextApplet` in `registry.go` already wraps every applet's
   stdin, so `cat` carries two prechecking wrappers. Reverting it to plain
   `io.Copy` keeps the suite green. No wrong behavior today, but the ambiguity
   about which layer owns cancellation is exactly the kind of thing a release
   should not inherit. Collapse it, or pin it with a test that constructs
   `catApplet{}` directly.

## Deferred, With The Release They Belong To

Deferring these is what makes v1.0 shippable. Each has a home.

**v1.1 — applet option matrices and `~user`.**
39 applets are registered (`internal/applets/registry.go:52-92`) and every
initial candidate name resolves, but *name presence is not semantic parity*. The
per-applet tables in `docs/testing/applet-test-inventory.md` are the
specification, and the options they name (`ls -l`, `head -c`, `grep -r`,
`find -name`, `xargs -0`, `sort -k`, `cut -f`) are largely uncovered. v0 already
made the failure mode safe — an unimplemented option is refused rather than
swallowed — which is precisely why this can ship as-is and be filled in after.
`~user` joins it: `echo ~root` prints `~root` today (measured), and resolving
another account's profile directory needs `SHGetKnownFolderPath` per SID or
`ProfileImagePath`, with no portable equivalent.

**v1.2 — completion M0 through M2.**
`docs/research/autocomplete-feasibility.md:299-327` holds the staged plan and
its conclusion stands: a Nemosh-owned semantic engine behind an editor adapter.
M0 freezes contracts, M1 builds read-only seams so completion can query what
execution would resolve *without executing*, and M2 is a synchronous engine
behind a noninteractive harness. M2's exit criterion is that the core matrix
passes with no terminal editor at all, which is why it stops there.

**v2 — the interactive tier.**
M3 is the editor bakeoff between `go-readline-ny` and `reeflective/readline`,
and it would add **this project's first runtime dependency**; that is its own
decision with its own written justification, not a line item here. M4–M6, the
full raw-terminal REPL, ConPTY and PTY, `/dev/tty`, history suggestions, and
plugins follow it.

## Explicitly Still Non-Goals

Carried from `v0-scope.md:151-158`, unchanged, and not usable to defer anything
above:

- MSYS2/Cygwin argv conversion. v0 confirmed by measurement that argv reaches a
  child unconverted, and that is intended.
- WSL-like mount namespace; `shares`/`nmount` remain Milestone D sketches.
- Full POSIX certification claims.
- Full BusyBox applet parity. The "Later BusyBox-Style Roadmap"
  (`applet-test-inventory.md:76-89`) — checksums, archiving, `awk`, `vi`, `bc`,
  networking — stays out; `awk` and `vi` are standalone projects.
- Windows ACL tooling.

### Job control, and where the reference draws its line

`fg` and `bg` refuse, and the refusal names why: job control needs a terminal
process group, which Windows does not have. busybox-w32 does not implement them
either — their builtin-table entries sit under `#if JOBS`
(`shell/ash.c:12050` and `:12081`) and `JOBS` is 0 under
`ENABLE_PLATFORM_MINGW32` (`shell/ash.c:247-253`), so they are compiled out.

The reference does draw a useful line, though. It defines a second macro,
`JOBS_WIN32`, whose comment says it "doesn't enable job control, just some
job-related features" (`shell/ash.c:244-245`), and gates `jobs` and `kill` on
`JOBS || JOBS_WIN32` (`shell/ash.c:12094-12097`) so they survive on Windows.
That is the same line P0.4 already drew here by shipping `jobs` and `wait` but
not `fg`/`bg`.

A bounded subset above `JOBS_WIN32` and below POSIX — foregrounding a retained
background job over Job Objects and ConPTY, without stopped jobs, `SIGTSTP`, or
process groups — is reachable, and belongs with the v2 interactive tier because
it needs the same terminal ownership. Anything less than that subset keeps
refusing rather than pretending.

### Also measured and deliberately left

- `/dev/tty` and PTY device paths, deferred by the device model itself.
- A shared extended-length path prefix layer. Wave C landed long paths without
  one; a child launched from a directory over 258 UTF-16 characters falls back
  to its 8.3 short name or is refused by length. Making that uniform is a
  reasonable v1.1 candidate but was not required by v0.

## Critical Path

```text
V1-A  release contract, licence, --version/--list
  +   two folded correctness debts
  ↓
V1-B  CI hardening, govulncheck, fuzz, pinned actions, baselines
  ↓
V1-C  reproducible build, Scoop manifest, SBOM, signing
  ↓
V1-RC clean-machine acceptance
  ↓
v1.0.0
  ↓
v1.1  applet option matrices, ~user
  ↓
v1.2  completion M0–M2
  ↓
v2    editor bakeoff, full REPL, PTY, bounded job control
```
