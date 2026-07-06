# Pre-Implementation Plan

This document lists the steps to finish before writing the first production Go
runtime code. The goal is to start implementation with narrow interfaces, a test
shape, and no hidden platform decisions.

## Stop Criteria For Planning

Planning is complete when these artifacts exist and are internally consistent:

- `docs/design/v0-scope.md`
- `docs/design/windows-path-model.md`
- `docs/design/windows-execution-model.md`
- `docs/testing/behavior-test-format.md`
- `docs/testing/initial-behavior-cases.md`
- `docs/testing/applet-test-inventory.md`
- a first behavior corpus layout
- a first Go package layout proposal
- an initial applet milestone list

Do not wait for every post-v0 topic, REPL detail, or full applet roadmap before
starting code.

## Step 1: Freeze V0 Decision Table

Create a short decision table that maps each architecture decision to a source:

- busybox-w32 behavior or source
- BusyBox ash behavior or source
- POSIX requirement
- Nemosh-specific user decision
- explicit deferred item

This prevents later implementation from re-litigating settled points or
accidentally treating MSYS2/Cygwin as compatibility targets.

## Step 2: Define The Behavior Corpus Layout

Before implementation, define the test file format and directory layout.

Recommended layout:

```text
tests/behavior/
  posix/
    simple-command.toml
    expansion.toml
    redirection.toml
    pipeline.toml
  windows/
    path-roots.toml
    unc.toml
    devices.toml
    exec-lookup.toml
  applets/
    coreutils-smoke.toml
```

Each case should record:

- script or command input
- expected stdout/stderr/status
- platform requirements
- reference shells to compare
- known reference differences
- whether the behavior is POSIX, busybox-w32, or Nemosh-specific

The first corpus should include behavior already discussed:

- `/` as current root on Windows
- `//host/share` as UNC root
- `//host` targeted diagnostic
- no external argv path auto-conversion
- fixed suffix lookup
- CRLF script tolerance
- semicolon `PATH`
- environment case collision at spawn
- `/dev/null`, `/dev/zero`, `/dev/urandom`, `/dev/random`, `/dev/clipboard`

The initial case list is tracked in `docs/testing/initial-behavior-cases.md`.

## Step 3: Probe Reference Behavior Locally

Run small probes against local references and record observations before coding.

Required references:

- local busybox-w32 `ash`
- local busybox-w32 applets
- dash or bash on a POSIX environment where available

Probe outputs should be written into a report under `docs/research/` or into
test metadata. Avoid relying on memory from the design conversation.

## Step 4: Define Go Package Boundaries

Create a package layout proposal before `go mod init`.

Candidate layout:

```text
cmd/nemosh/
internal/shell/parse/
internal/shell/ast/
internal/shell/runtime/
internal/shell/expand/
internal/shell/fd/
internal/shell/jobs/
internal/platform/
internal/platform/windows/
internal/pathmodel/
internal/applets/
internal/applets/coreutils/
internal/testutil/behavior/
```

Interfaces to sketch before implementation:

- parser output and source spans
- shell state snapshot
- fd table operations
- path resolution and display
- executable lookup
- child process spawn
- applet registry and dispatch
- diagnostics and debug flags

## Step 5: Define Parser Milestone Slice

Because the chosen direction is an own parser, implementation should not begin
with the entire POSIX grammar.

Recommended slices:

1. Tokens, words, quoting, comments, and CRLF handling.
2. Simple commands, assignments, and redirects.
3. Pipelines and lists.
4. Parameter expansion and command substitution.
5. Here-docs.
6. `if`, loops, `case`, groups, functions, and subshells.
7. Error recovery and source spans.

Each slice should add behavior tests before runtime work depends on it.

## Step 6: Define Runtime Milestone Slice

Recommended order:

1. Shell state, variables, exported environment, and options.
2. Path model and current-root tracking.
3. fd table with stdio and redirection.
4. Builtin execution.
5. External spawn and executable lookup.
6. Pipelines.
7. State snapshots for subshell and command substitution.
8. Background jobs and `wait`.
9. Devices and virtual roots.

This order keeps Windows-specific substrate work visible instead of burying it
inside parser or applet code.

## Step 7: Define Applet Milestones

Start with applets that unblock shell tests and script use, then expand.

Every implemented applet must have behavior tests. The current inventory and
per-applet test rule are in `docs/testing/applet-test-inventory.md`.

Milestone A:

- `true`, `false`
- `echo`, `printf`
- `pwd`, `env`, `printenv`
- `[` / `test`

Milestone B:

- `cat`, `head`, `tail`, `wc`
- `basename`, `dirname`
- `ls`, `mkdir`, `rmdir`, `rm`, `touch`

Milestone C:

- `cp`, `mv`, `chmod`
- `grep`, `sed`, `find`, `xargs` if required by the corpus

Milestone D:

- Windows-native ACL/share/mount helper applets, such as `shares`, `nmount`, or
  ACL inspection tools.

## Step 8: Decide First Repository Commit Boundary

Before writing code, commit the research and design docs if the user approves.
Then code can start from a clean baseline.

Recommended first code commit after docs:

- `go.mod`
- `cmd/nemosh/main.go`
- empty applet registry
- behavior test harness skeleton
- no real shell semantics yet

## Step 9: Avoid Early Traps

- Do not add MSYS2-style argv path rewriting.
- Do not implement a custom mount namespace in v0.
- Do not claim `.bashrc` or `.zshrc` compatibility.
- Do not implement REPL first.
- Do not let applets bypass the Nemosh path/fd abstractions.
- Do not rely on Go `os/exec` defaults without documenting Windows handle and
  environment behavior.
- Do not expose `\\?\` as the primary user path model.

## Ready To Code Checklist

- [ ] `v0-scope.md` reviewed.
- [ ] Windows path model reviewed.
- [ ] Windows execution model reviewed.
- [ ] Open questions pruned to only real blockers.
- [ ] Behavior corpus format chosen.
- [ ] Initial behavior case list reviewed.
- [ ] Applet test inventory reviewed.
- [ ] Package layout chosen.
- [ ] Parser slice 1 accepted.
- [ ] Runtime slice 1 accepted.
- [ ] Applet Milestone A accepted.
- [ ] Docs committed or explicitly left uncommitted by user choice.
