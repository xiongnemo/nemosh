# V1 Scope

**V1.0 turns the finished v0 into something a person can install, trust, and
report bugs against — and, as it turned out, ships two of the features this
document had deferred.**

That second clause is a correction, made 2026-08-21, and it is written here
rather than buried because the sentence it replaces was load-bearing: *"V1.0 does
not add shell semantics."* Two things landed on `master` after it was written.

- **Line editing** — arrows, history, Ctrl-D/Ctrl-Z, the Meta bindings — which
  this document places in v1.1 below, and which is already in the Unreleased
  changelog.
- **`top`**, an interactive process monitor, which the plan it was built from
  called a v1.1 item *by this document's own rule*.

`master` is the nightly channel: every push publishes a prerelease, and Scoop
has been installing them on the development machine for weeks. So the features
are already in a user's hands, and "v1.0 excludes them" was never a description
of anything — it was a description of a branch nobody was going to cut.

**The decision, 2026-08-21: v1.0 is what is on master.** The alternative was
branching v1.0 from before those landed, which means shipping a first stable
release that is worse than the nightly people are already running, or tagging a
v1.0 whose binary contains features its own scope document excludes. Neither is
honest. The rule below is amended rather than quietly broken.

### The amended rule

The rule was never really "no features". It was **no feature that can destabilise
the shell's semantics before a release**, and that is what both of these respect:

- Line editing is confined to how a line is *read*, not what a line *means*. A
  pipe or a file still takes the previous line-at-a-time path, so every
  non-interactive behaviour — the whole differential corpus — runs through
  unchanged code.
- `top` is one applet. It reads and it draws; the only thing it can do to the
  system is `TerminateProcess`, which `kill` already did. It cannot change how a
  script parses or what a redirection means.

What the rule still forbids, and what stays deferred: new shell semantics —
completion, PTY, job control, `~user`. Those are v1.1 and beyond, unchanged.

### One premise this invalidated

The argument below for an in-house line editor is that it "avoids adding **this
project's first runtime dependency**". That is now moot: `top` draws with tview
and tcell, so the binary already carries them, along with `gdamore/encoding`,
`lucasb-eyer/go-colorful` and `rivo/uniseg`. The in-house editor shipped anyway
and works, so nothing needs undoing — but anyone revisiting the editor decision
should know that building on tcell is now nearly free, and that the reason not to
has gone.

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
  the job supervisor. **Done 2026-08-09.** `leak_test.go` walks twelve script
  shapes and one 200-command session; `handle_leak_windows_test.go` reads
  `GetProcessHandleCount` across 600 redirects and 300 pipelines, measuring the
  *second* identical batch so the Go runtime's own warm-up is behind the
  reading; `job_stress_test.go` covers the supervisor. This found a real defect:
  a finished background job never gave its slot back, so the 65th `foo &` in a
  session was refused permanently.
- **Pin GitHub Actions to commit SHAs.** Currently `actions/checkout@v4`,
  `actions/setup-go@v5`, and `msys2/setup-msys2@v2` are tags, which are mutable.
  Apply least-privilege permissions in the same pass.
- Performance baselines with regression thresholds. **Done 2026-08-09, and not
  as originally written.** Wall-clock thresholds were dropped: five consecutive
  startups on an idle machine measured 43, 45, 42, 26 and 43 ms, a 42% spread
  with nothing competing, and a gate that flaps on a shared runner gets ignored
  rather than fixed. What is gated instead is deterministic -- allocation
  ceilings per parse and per command (`allocation_test.go`, each about twice the
  measured figure, with the measurement recorded beside it), handle and
  goroutine counts as above, and a binary-size ceiling in `product.yml` at
  4.5 MiB against 3.78 MiB measured, which is under the 5.58 MiB an unstripped
  build produces and so catches `-s -w` falling out of the release flags.
  `BenchmarkParseScript` and `BenchmarkRunScript_pipeline` exist for humans to
  read; they do not fail a build.

### V1-C — Reproducible packaging and distribution

- A single multicall binary.
- Windows Scoop-first: a manifest, and tests for clean install, shim
  invocation, upgrade, and uninstall. **Exercised for real rather than by test,
  2026-08-14.** `~/scoop/apps/nemosh-nightly/` on the development machine holds
  nine installed versions, 0.1.10 through 0.1.33 -- a clean install and eight
  upgrades -- plus the uninstall of the plain `nemosh` line and shim invocation
  on every use since. A CI test would be the weaker evidence: it can only run
  after a release exists, so it would be testing the published bucket rather than
  the change under review, and manifest breakage is already caught by the
  Excavator, which computes the hash from the published archive and fails when it
  cannot. What is genuinely uncovered is a machine with no Scoop and no prior
  install, and that belongs to V1-RC. **One shim, not one per applet** --
  decided 2026-08-09. `find` and `sort` are Windows commands with different
  syntax, and on a machine with busybox installed those names are already
  taken; an install that silently changes what a name means is worse than one
  that makes the user type `nemosh find`. `scoop shim add` covers the
  exception.
- Archives for the other platforms. **Done 2026-08-14**, and the wording above
  had to be resolved first: there are no *supported* Unix platforms, so on its own
  reading this bullet had nothing to satisfy. It is done anyway, because the
  README's first line says cross-platform and one binary under-delivers on that.
  `linux/amd64`, `linux/arm64`, `darwin/amd64` and `darwin/arm64` now ship as
  `.tar.gz` -- tar rather than zip because a zip does not carry the executable
  bit. Publishing is not promising: the release notes, the README table and the
  support matrix each say that only `windows/amd64` is supported and that a bug
  elsewhere is a report rather than a release blocker.
- SBOM. **Declined 2026-08-14.** A Go binary already records its module graph, so
  `go version -m nemosh.exe` reports every dependency and its hash -- two of them,
  `golang.org/x/sys` and `golang.org/x/term`. A formal SPDX or CycloneDX document
  would restate that in another syntax, and `govulncheck` already reports
  *reachable* vulnerabilities weekly, which is the thing an SBOM is usually wanted
  for. `cyclonedx-gomod` is one step away if a consumer ever asks for the format.
- `-trimpath` reproducible builds and checksums. Both done.
- **Provenance. Done 2026-08-14.** `actions/attest-build-provenance` signs each
  published archive with a short-lived OIDC token minted for that run, so the
  attestation says what a checksum cannot: not merely that the file is unaltered,
  but that *this workflow, at this commit* produced it. Verified with
  `gh attestation verify <file> --repo xiongnemo/nemosh`. It is attested before
  publishing, so a release cannot carry an artifact whose provenance is missing.
  This is not a substitute for code signing and does not quiet SmartScreen -- it
  answers a different question -- but it is the strongest origin claim available
  without a certificate, which was declined.
- Signing: declined. Recorded in `SECURITY.md`; Scoop verifies the checksum.
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
2. **Cancellation ownership. Settled 2026-08-14, and the premise above was
   wrong.** This said `copyWithContext` was redundant because `contextApplet`
   already wraps every applet's stdin, and proposed collapsing it to plain
   `io.Copy`.

   That holds for **stdin**, where the second wrap is a harmless extra hop. It
   does not hold for an **operand**: `cat file.txt` hands `OpenProcessInput`'s
   `*os.File` straight to the copy, and a file knows nothing about a context.
   Collapsing would have meant Ctrl-C could not interrupt `cat` on a large file
   -- silently, and only in the case a user is most likely to hit.

   So it stays, and the ownership is now stated rather than assumed: the registry
   makes stdin cancellable, `copyWithContext` makes everything else cancellable,
   and `cancellation_ownership_test.go` fails if either layer is removed --
   including a test that constructs `catApplet{}` directly, which is the check
   this entry asked for.

## Deferred, With The Release They Belong To

Deferring these is what makes v1.0 shippable. Each has a home.

**v1.1 — applet option matrices and `~user`. Shipped 2026-08-22, less `~user`.**

The option matrices are done, and were chosen by measurement rather than by
working down the list below: the shell was probed for 46 language constructs and
each applet for the options a script actually reaches for. **45 of the 46
constructs already worked**, `select` being the only absentee, so there was no
shell work to do at all and every remaining gap was in an applet. What the probe
found, in the order it was fixed: `find` had no boolean operators and read `!` as
a path operand; `ls` refused `-t -S -r -R -d -F -A`, so `ls -ltr` failed on its
options; `head -n2` was refused while `head -2` and `head -n 2` worked, and
multi-file `head` printed no `==> name <==` headers at all; `grep` had no context
lines; `sed` was an `s///` filter with no `-n`, no addresses, no `p` and no `d`.
See `CHANGELOG.md` for what each answers now and `docs/support-matrix.md` for the
contract.

**`~user` did not ship and is not scheduled.** `echo ~root` prints `~root`, as it
did; resolving another account's profile directory needs `SHGetKnownFolderPath`
per SID or `ProfileImagePath`, with no portable equivalent, and nothing measured
in the probe asked for it. It waits for someone who wants it.

The list of what remained, kept as written before the work:

Line editing was in this list and has shipped; it is in v1.0 by the correction at
the top of this document. The reasoning that moved it here from v2 on 2026-08-08
is kept below because it is still the argument for why it had to come before
completion, and because the double-width acceptance requirement it names is still
an acceptance requirement.

The three things a user asks for separately turn out to be one thing:

- arrow keys and history recall,
- Ctrl-D and Ctrl-Z exiting the shell,
- Tab completion.

All three need the terminal in raw mode with the shell reading keys rather than
lines. Measured: through a pipe neither Nemosh nor busybox treats `0x04` or
`0x1a` as end of input -- busybox reports 127 for both, exactly as Nemosh does.
What makes them work in busybox is `libbb/lineedit.c`, 3322 lines of raw-mode
editor. Nemosh reads cooked lines and never calls `MakeRaw`, so it has no place
to catch a key at all.

Leaving this in v2 meant a completion *engine* would land in v1.2 with no Tab to
trigger it, and a daily user would have no arrow keys until after a 1.0. The
ordering was technical rather than a product judgement, and it inverted them.

The double-width accounting that breaks busybox's own backspace over CJK -- one
column deleted for a two-column character -- is an acceptance requirement here,
not something to discover afterwards.

Then, as before:
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
ConPTY and PTY, `/dev/tty`, history suggestions, plugins, and the bounded job
control described below.

The editor bakeoff between `go-readline-ny` and `reeflective/readline` is no
longer what unblocks v1.1: an in-house editor covering the keys above avoids
adding **this project's first runtime dependency** for a feature set busybox
implements itself. Adopting one remains open, and remains its own decision with
its own written justification -- but it is now an improvement rather than a
prerequisite.

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
V1-A  release contract, licence, --version/--list          done
  +   two folded correctness debts                         done
  ↓
V1-B  CI hardening, govulncheck, fuzz, pinned actions       done
  ↓
V1-C  reproducible build, Scoop manifest, provenance        done
  ↓
      line editing, and top                                done, unplanned
  ↓
V1-RC clean-machine acceptance                              the remaining gate
  ↓
v1.0.0                                                      released 2026-08-21
  ↓
      /dev as a directory, ~/ completion, top's F1 panel     done, unplanned
  ↓
v1.1  applet option matrices                                released 2026-08-22
      (~user dropped: nothing measured asked for it)
  ↓
v1.2  completion M0–M2
  ↓
v2    PTY, /dev/tty, history suggestions, bounded job control
```

### What V1-RC has to cover that it did not when it was written

The acceptance list below says "every command in the documentation, executed as
written". Two additions now fall under that, and neither existed when it was
drafted:

- **`top`'s help and the process-view document.** The IO columns, the `-o` field
  list, the width tiers, and the claim that `-H` shows threads. That last one was
  false until 2026-08-21: the flag was accepted, documented, sampled the thread
  records, and produced identical output. It is the shape of defect this gate
  exists to catch, and it was caught by reading the scope document rather than by
  any test.
- **The line editor on a real console**, including the double-width backspace
  over CJK that breaks busybox's own editor. That was already named as an
  acceptance requirement; it is now an acceptance requirement for *this* release.
