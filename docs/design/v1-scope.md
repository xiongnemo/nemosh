# V1 Scope

This document turns what v0 deliberately left undone into an implementation
scope. Like `docs/design/v0-scope.md` it is allowed to change, but code should
not start by ignoring these boundaries.

**Where this comes from.** Nothing here is invented. Every item traces to one of
four places, and the trace is given so a reader can check it rather than trust
it:

- the six **Deferred Non-Goals** of `docs/design/v0-scope.md:151-158`;
- the **remainder columns** of `docs/design/v0-readiness.md`, which are measured
  against a built binary rather than against the test suite;
- the **Remaining debt** section of the closure record at
  `.omo/plans/input-cancellation-io-number-fixes.md:101-117`;
- the staged plan in `docs/research/autocomplete-feasibility.md:299-327` and the
  per-applet matrices in `docs/testing/applet-test-inventory.md`.

There was no prior v1 plan. `.omo` holds exactly one file and it is a closure
record for a shipped v0 slice, not a forward plan.

## What v0 Established That V1 Must Not Regress

These are load-bearing properties, not achievements to restate. Each one
constrains how v1 work may be written.

- **A capability that is absent must fail loudly.** This is the rule the v0
  readiness audit was corrected under, and it is why `hash`, `ulimit`, `fg`,
  `bg`, and `set -b`/`-n`/`-v` refuse with a reason and a non-zero status
  instead of doing something approximate. Any v1 feature that lands partially
  refuses the part it cannot do; it does not guess.
- **Parse before effects.** A script is parsed in full before any of it runs.
  This is why `set -n` and `set -v` are unimplementable as specified, why alias
  substitution happens at dispatch rather than during tokenization, and why a
  malformed line late in a script produces no output from the lines before it.
  V1 must not introduce a construct that requires executing as it parses.
- **busybox-w32 is the primary behavior reference**, then BusyBox ash, then dash
  and POSIX. Claims are verified against the clone at
  `references/windows-compat/busybox-w32`, never against memory.
- **Green tests do not imply completion.** Both defects found on 2026-08-07 —
  `times` reporting 215 years, and a brace being reserved outside command
  position — passed the whole suite. Shape assertions in particular hide wrong
  values; bound the value too.
- **The 250 pure-LOC ceiling** per production `.go` file, and the differential
  corpus against busybox-w32/ash, dash, and bash.

## V1 Must Have

The four items below are the ones v0's own remainder columns name. They are
ordered by how much they cost a user who hits them.

### 1. Applet option matrices

**Status: the largest uncovered surface.** 39 applets are registered
(`internal/applets/registry.go:52-92`) and every initial candidate name resolves,
but *name presence is not semantic parity*. The per-applet required-test tables
in `docs/testing/applet-test-inventory.md` — Milestones A through D — are the
specification, and the options they list (`ls -l`, `head -c`, `grep -r`,
`find -name`, `xargs -0`, `sort -k`, `cut -f`, `tr` ranges) are largely
uncovered.

v0 already made the failure mode safe: an option an applet does not implement is
refused rather than swallowed, so a script asking for one fails instead of
silently getting something else. V1 turns refusals into implementations, matrix
by matrix, in the milestone order that document already sets.

Acceptance: each applet's row in the inventory has a checked-in test file per
the "Per-Applet Test File Rule", and every option the row names either works or
refuses by name.

### 2. `~user` tilde expansion

**Status: left as written.** `echo ~root` prints `~root` (measured). Bare `~`
and `~/path` work. `~user` requires resolving another account's profile
directory, which on Windows means `SHGetKnownFolderPath` per SID or reading
`ProfileImagePath`, and has no portable equivalent.

Acceptance: `~user` resolves for a local account that exists, and refuses by
name for one that does not — not silently passed through.

### 3. Windows real-console cancellation acceptance

**Status: debt item 1 of the closure record.** Current interrupt coverage
exercises anonymous pipes and regular files. A test that allocates a real child
console and opens `CONIN$` would exercise `ReadConsole` plus
`CancelSynchronousIo` directly, which is the path a user actually hits. A one-off
`GenerateConsoleCtrlEvent` probe passed during the v0 review for both `cat`
forms but was never checked in.

This is also where the three interactive interrupt tests skipped on non-Windows
(`cmd/nemosh/interactive_interrupt_platform_test.go`) would gain a native
counterpart.

### 4. Cancellation ownership

**Status: debt item 2 of the closure record.** `copyWithContext` in
`internal/applets/files.go` is redundant: `contextApplet` in `registry.go`
already wraps every applet's stdin, so `cat` carries two prechecking wrappers.
Reverting it to plain `io.Copy` keeps the suite green. No wrong behavior today,
but which layer owns cancellation is ambiguous. Either collapse it or pin it
with a test that constructs `catApplet{}` directly.

## Job Control: What Is Reachable On Windows

`fg` and `bg` refuse today, and the refusal names why: job control needs a
terminal process group, which Windows does not have. busybox-w32 does not
implement them either — their builtin-table entries sit under `#if JOBS`
(`shell/ash.c:12050` and `:12081`), and `JOBS` is 0 under
`ENABLE_PLATFORM_MINGW32` (`shell/ash.c:247-253`), so they are compiled out.

**The reference draws the line somewhere useful, though.** busybox-w32 defines a
second macro, `JOBS_WIN32`, whose own comment says it "doesn't enable job
control, just some job-related features" (`shell/ash.c:244-245`). `jobs` and
`kill` are gated on `JOBS || JOBS_WIN32` (`shell/ash.c:12094-12097`) and so do
survive on Windows — which is the same line Nemosh's P0.4 already drew by
implementing `jobs` and `wait` but not `fg`/`bg`.

V1 does not adopt "full native Windows POSIX job control"; that stays a
non-goal. What is reachable, and what a v1 proposal would have to argue for
separately, is a bounded subset above `JOBS_WIN32` and below POSIX: bringing a
retained background job's output to the foreground and waiting on it, over Job
Objects and ConPTY, without stopped jobs, without `SIGTSTP`, and without process
groups. Anything less than that subset should keep refusing rather than pretend.

`hash` stays refused on a different ground: command lookup is not cached, so
there is nothing to remember or forget. busybox-w32 does implement it, over a
hash table this shell does not have. Implementing `hash` means first adding a
lookup cache, which is a performance decision, not a compatibility one.

`ulimit` stays refused: Windows has no `getrlimit`, and busybox-w32 does not
implement it either — it keeps the name and returns 1 with no message
(`shell/shell_common.c`, the `#else` of `#if !ENABLE_PLATFORM_MINGW32`). Nemosh
saying so is strictly better than returning 1 silently.

## Interactive Shell And Completion

**Status: v0 non-goal, feasibility already researched, no code written.**
`docs/research/autocomplete-feasibility.md` is a complete plan and its
conclusion stands: a Nemosh-owned semantic completion engine behind an editor
adapter, with the engine owning parsing, replacement ranges, Windows path and
command semantics, providers, ranking, cancellation, and caching.

V1 adopts stages **M0 through M2** of that document's staged plan and no
further:

- **M0** — freeze the request, candidate, insertion, offset, visibility, and
  budget contracts; build fixtures; measure current input behavior. No
  dependency decision.
- **M1** — reusable read-only seams: applet and command enumeration, resolver
  reuse, immutable shell state, one path policy. Exit criterion is that
  completion can query exactly what execution would resolve *without executing*.
- **M2** — a synchronous semantic engine behind a noninteractive harness:
  cursor analysis, path/command/variable providers, quoting and insertion,
  ranking, caps, Unicode conversion.

M2's exit criterion is the reason to stop there: the core matrix passes with no
terminal editor at all. **M3, the editor bakeoff between `go-readline-ny` and
`reeflective/readline`, adds the project's first runtime dependency and must be
its own decision with its own written justification.** M4 through M6 follow that
decision, not this document.

## Explicitly Still Non-Goals

Carried forward from `v0-scope.md:151-158`, unchanged, and not usable to defer
anything above:

- MSYS2/Cygwin argv conversion. v0 confirmed by measurement that argv reaches a
  child unconverted, and that is the intended behavior.
- WSL-like mount namespace. `nmount` remains a Milestone D sketch, not a v1
  commitment.
- Full POSIX certification claims.
- Full BusyBox applet parity. The "Later BusyBox-Style Roadmap"
  (`applet-test-inventory.md:76-89`) — checksums, archiving, `awk`, `vi`, `bc`,
  networking — stays out. `awk` and `vi` in particular are standalone projects.
- Full native Windows POSIX job control, as qualified above.
- Plugins, and zsh-level interaction beyond M2.

Two more that v0 measured and deliberately left:

- `/dev/tty` and PTY device paths, deferred by the device model itself.
- A shared extended-length path prefix layer. Wave C landed long paths without
  one; a child launched from a directory over 258 UTF-16 characters falls back
  to its 8.3 short name or is refused by length. Making that uniform is a
  reasonable v1 candidate but was not required by v0.

## Open Question For Prioritization

Items 1 through 4 of **V1 Must Have** are independent and can land in any order.
Item 1 is by far the largest and is the one a user is most likely to hit; the
completion work is the largest overall and the only part that risks a
dependency. Which of those leads is a product call, not a technical one.
