# V0 Readiness Ledger

This document is an evidence snapshot of the current working tree. It does not
define, narrow, defer, or add v0 requirements. `docs/design/v0-scope.md` remains
the authoritative v0 scope; when this ledger and that document disagree, the
scope document wins.

The ratings are deliberately bounded:

- **Complete**: implementation and focused tests cover the stated narrow item.
- **Partial**: useful implementation exists, but a material part of the scope
  gate is absent or not integrated.
- **Missing**: no operative implementation or meaningful test was found.
- **Contradicted**: current behavior or a readiness claim conflicts with the
  authoritative scope.

Passing tests are evidence only for behavior those tests exercise. The current
`go test ./...` result is green, but **green tests do not imply v0 completion**:
major must-haves have no implementation or acceptance test, Windows-only paths
are incompletely exercised, differential references are not run end to end, and
there is no product CI.

## Executive Status

| `v0-scope.md` section | Status | Current evidence | Blocking remainder |
| --- | --- | --- | --- |
| Product Target | **Partial** | Native Go CLI and multicall dispatch exist in `cmd/nemosh/main.go`; bundled applets are registered by `internal/applets/registry.go`; P0.4 implements the bounded Windows jobs/INT model. | Windows-first behavior is not integrated across path, launch, clipboard, Unicode/long-path, and diagnostic boundaries. Expansion and broader ash-like behavior remain incomplete. |
| V0 Must Have | **Partial** | `-c` and redirected-stdin execution are tested in `cmd/nemosh/main_test.go`; P0.1 owns the FD table and snapshots, P0.2 owns concurrent `os.Pipe` pipelines, P0.3 owns typed groups/subshells/heredocs/functions, and P0.4 owns asynchronous background execution, retained job lifecycle, `jobs`/`wait`, and bounded Windows INT behavior under `internal/shell/runtime/`; the applet registry is substantial. | Shell-wide Windows path/launch/device boundaries, complete selected expansion semantics, and corpus-wide differential/product CI remain absent or incomplete. |
| Windows Path Gates | **Partial (P0.5 Wave A integrated)** | `internal/pathmodel/model.go` and `pathmodel_test.go` cover drive, mount-alias, UNC, current-root, and virtual-root forms. `internal/shell/runtime/path_state.go` exposes them as `Runtime.ResolveNemoshPath`, which `cd`, `.`/`source`, redirection, device input, external lookup, and the applet `ProcessView` all call; `runtime/path.go` is now a thin adapter over that seam rather than a separate model. | Native Windows integration-test breadth is thin, notably UNC cwd/root. Extended-length paths are deferred to Wave C. No general argv conversion was found, which correctly preserves that non-goal. |
| V0 Device Paths | **Partial** | `internal/shell/runtime/device.go`, `device_fd.go`, `device_test.go`, and `device_fd_test.go` cover `/dev/null`, descriptor-backed standard streams and `/dev/fd/N`, zero, random, and urandom. | UTF-8 text `/dev/clipboard` is missing. |
| Execution Gates | **Partial / contradicted at Windows script boundary** | Lookup order is implemented across `internal/shell/runtime/runtime.go`, `internal/shell/runtime/command.go`, and `internal/shell/runtime/external.go`; fixed Windows suffix ordering, exact runtime `PATH`, scoped Windows child-environment collapse, and runtime cwd/env propagation have focused tests. | Applet override configuration is missing. `.bat`/`.cmd` and `.sh` are discovered but sent directly to `os/exec`, contradicting required `ComSpec` and interpreter handling. Long-path boundaries remain missing. |
| Job, Signal, And Error Gates | **Partial (P0.4 jobs/signals complete)** | Typed background nodes launch asynchronously into a session-owned bounded supervisor; `jobs` observes retained local records; `wait`/`wait %N` claim, retain on cancellation, and consume exact statuses. EXIT/INT traps, direct applet and foreground cancellation, production Windows Ctrl-Break acceptance, private-scope reaping, and honest root-close limitations have focused tests. | Layered diagnostic hints and `NEMOSH_DEBUG=path,exec,fd` remain P1.1 work. Full `fg`/`bg`, stopped jobs, process groups, Job Objects, ConPTY, and idle-input Ctrl-C remain explicitly outside P0.4. |
| Parser And Shell Semantic Gates | **Partial** | `parser.go`, `syntax_scan.go`, `parser_typed.go`, `parser_compound_typed.go`, `parser_group.go`, `parser_function.go`, and `ast.go` build a typed program before execution; typed words preserve quote, escape, parameter, command-substitution, redirect, and background-list structure. Brace groups, parenthesized subshells, heredocs, portable functions, and background-marked lists and compounds are typed and pipeline-aware. Parser limits and malformed/incomplete syntax are tested. | Expansion and redirection remain narrow, and exact grammar/runtime acceptance remains incomplete. Asynchronous background semantics remain a P0.4 job-runtime concern, not a parser gap. |
| Applet Scope Direction | **Partial** | Every initial candidate name is registered in `internal/applets/registry.go`, with implementations under `internal/applets/`; direct Go tests cover many applets. | Name presence is not semantic parity. Most initial candidates lack checked-in smoke and negative behavior cases required by `docs/testing/applet-test-inventory.md`. |
| Deferred Non-Goals | **Complete as a boundary observation** | No evidence was found that v0 completion depends on full REPL polish, native POSIX job control, MSYS/Cygwin argv conversion, WSL mounts, certification, or full BusyBox parity. | These remain non-goals; they must not be used to defer any existing must-have above. |

## Detailed Evidence Ledger

### Product must-haves and execution context

| Capability | Status | Exact implementation/test evidence | Readiness boundary |
| --- | --- | --- | --- |
| Non-interactive runner | **Complete** | `cmd/nemosh/main.go`, `cmd/nemosh/script_file.go`; `cmd/nemosh/main_test.go`, `cmd/nemosh/script_file_test.go` | `-c`, stdin, and `nemosh script.sh [args]` all dispatch. A script file seeds `$0` from the operand as written and `$1…` from the rest; an unreadable one is status 127. Applet names still win over same-named files, matching `busybox cat`. An operand starting with `-` that is not `-c`, `-i`, or a bare `-` is an invalid option with status 2. |
| Runtime-owned cwd | **Complete for implemented operations** | `internal/shell/runtime/execution_state.go`, `path.go`, `external.go`; `execution_state_test.go`, `runtime_relative_io_test.go`, `runtime_external_test.go`, `applet_process_view_test.go` | This just-landed state no longer relies on process-global cwd, but broader pathmodel integration remains separate. |
| Runtime-owned environment | **Complete for isolation, child propagation, and the scoped Windows child block** | `internal/shell/runtime/execution_state.go`, `environment_child.go`, `assignment.go`, `external.go`; `environment_test.go`, `execution_state_test.go`, `assignment_test.go`, `runtime_external_test.go` | Shell/export state preserves exact-case names and empty values. Windows child serialization alone performs deterministic case-insensitive, latest-mutation-wins deduplication. Batch launch now reads `COMSPEC` from this table (`internal/shell/runtime/external_batch.go`). |
| Environment case/path fixes | **Complete for P0.1** | `Environment` stores exact names and mutation order; `environment_child.go` serializes Unix entries exactly and Windows entries by case-insensitive latest mutation; executable lookup still uses exact `PATH`. | Canonical `COMSPEC` handling and `.bat`/`.cmd` launch landed in P0.5 Wave B and read this table by its canonical name. |
| Unix runtime `PATH` | **Complete, platform-test limited** | `internal/shell/runtime/external.go`; non-Windows case in `internal/shell/runtime/runtime_external_test.go` | Relative and empty entries use runtime cwd. The focused test is skipped on Windows and does not prove Windows lookup/launch semantics. |
| Special-builtin assignments | **Complete for implemented special builtins; partial POSIX edge coverage** | `internal/shell/runtime/assignment.go`, `command.go`; `assignment_test.go`, `special_builtin_test.go` | Leading assignments persist for special builtins and remain temporary for regular commands. Fatal-error, redirect-only, and complete expansion-order semantics still need corpus gates. |
| Explicit snapshots | **Partial but strengthened** | `internal/shell/runtime/snapshot.go`, `fd_table.go`, `token_pipeline.go`, `execute_group.go`, `execute_ast.go`; `snapshot_fd_test.go`, `command_substitution_snapshot_test.go`, `group_execution_test.go`, `token_pipeline_concurrent_test.go`, `token_pipeline_fd_test.go`, `job_isolation_test.go`, `job_owner_cancellation_test.go` | Every active pipeline stage, parenthesized subshell, command substitution, and background worker is snapshotted. State, control flow, descriptor mappings, and private nested-job ownership are isolated while shared open descriptions retain exact-once ownership. Broader Windows path/launch integration remains separate. |
| Shell-owned FD table | **Complete for the portable P0.1 substrate and P0.2 applet/builtin pipeline use** | `internal/shell/runtime/fd_table.go`, `fd_description.go`, `redirect_parse.go`, `redirect_apply.go`, `token_pipeline.go`; `fd_table_test.go`, `fd_lifecycle_test.go`, `numbered_redirect_test.go`, `device_fd_test.go`, `snapshot_fd_test.go`, `token_pipeline_fd_test.go` | Numbered open/dup/close, left-to-right ordering, `/dev/fd/N`, applet/builtin stdio, snapshots, command substitution, pipeline endpoints, inherited shell FD 3, and external FD 0/1/2 are covered. Stage-local close/rebind of FD 3 does not mutate sibling or parent mappings. Arbitrary native-child inheritance above FD 2 is not portably promised. |

### Windows paths, devices, and process launch

Isolated pathmodel completeness must not be confused with runtime integration:

- **Isolated model: partial but substantial.** `internal/pathmodel/model.go` and
  `internal/pathmodel/pathmodel_test.go` cover drive forms, `/c`, `/mnt/c`, UNC
  shares, drive/UNC current roots, virtual-root preservation, host-only UNC
  rejection, and default-off Cygdrive behavior. Extended-length paths and more
  malformed forms remain.
- **Runtime integration: connected through one seam.** `internal/shell/runtime/path_state.go`
  resolves through `internal/pathmodel` and is the single seam shell-owned
  operations use: `cd` at `state_builtins.go:60`, `.`/`source` at `builtins.go:15`,
  redirection at `redirect_apply.go:38,63`, device input at `device_input.go:59`,
  executable lookup and child working directory at `external.go:82,100`, and the
  applet `ProcessView` at `internal/applets/process_view.go:25,34`.
  `runtime/path.go` delegates to that seam instead of carrying its own rules.
  Opt-in `/tmp` backing and Cygdrive conversion are implemented as policy in
  `path_state_windows.go` and `path_state_other.go`; `cd` emits the host-only UNC
  hint, covered by `path_shell_io_test.go`. What remains is native Windows
  integration-test breadth rather than a missing connection.

| Boundary | Status | Evidence | Required acceptance remainder |
| --- | --- | --- | --- |
| Accepted Windows spellings/current root/UNC | **Partial** | `internal/pathmodel/model.go`, `internal/pathmodel/pathmodel_test.go`, `internal/shell/runtime/path_state.go` and its `path_state*_test.go` family, `internal/shell/runtime/path_shell_io_test.go`, `internal/applets/path_test.go` | Shell-owned operations now share one model. Remaining work is native Windows integration-test breadth for UNC cwd/root and virtual roots, plus Wave C extended-length paths. |
| Device set | **Partial** | `internal/shell/runtime/device.go`, `redirect_apply.go`, `fd_table.go`; `device_test.go`, `device_fd_test.go` | `/dev/std*` and `/dev/fd/N` are descriptor-backed. UTF-8 text `/dev/clipboard` remains missing. |
| Native executable lookup | **Complete** | `internal/shell/runtime/external.go`, `external_format.go`; `external_format_test.go`, `external_suffix_windows_test.go`, `runtime_external_test.go` | Lookup follows busybox-w32 `add_win32_extension` (`win32/mingw.c:2237`): the bare name is tried first and accepted when it carries an executable suffix or sniffs executable, and only a name with neither suffix nor trailing dot gets `.com .exe .sh .bat .cmd` appended in that order. Sniffing is the pragmatic subset of busybox's PE walk — `#!` or `MZ`, `.dll` excluded. |
| Batch and command scripts | **Complete** | `internal/shell/runtime/external_batch.go`, `external_launch_windows.go`, `external.go`; `external_batch_test.go`, `external_batch_windows_test.go` | `.bat`/`.cmd` launch as `"<ComSpec>" /d /s /c "<script> <args…>"` through `SysProcAttr.CmdLine`, so cmd's own doubled-quote convention applies instead of Go's `\"`, an argument containing `&` cannot split the command line, and `/d` suppresses AutoRun. An operand carrying a line break or two or more `%` is refused with a diagnostic and status 126 before anything runs. No corpus fixture: the executor replaces the child environment wholesale (`internal/testutil/behavior/sandbox.go:42-47`), so `COMSPEC` would have to be hardcoded per machine. |
| `.sh` and shebang scripts | **Complete** | `internal/shell/runtime/external_script.go`, `external.go`; `external_script_test.go`, `tests/behavior/shell/windows/script-sh-dispatch.toml`, `tests/behavior/shell/windows/shebang-applet-interpreter.toml` | Shebang grammar follows busybox-w32 `parse_interpreter` (`win32/process.c:66`) and mapping follows `mingw_spawn_interpreter` (`win32/process.c:301`): `/bin/sh` and `/bin/nemosh` re-enter this binary as a child process, an applet name under a Unix directory is handed to it, anything else resolves through ordinary lookup, and a chain of interpreters gives up at the fifth. `#!/usr/bin/env python3` reaches the `env` applet, which runs applets only, so it reports `python3: not found`. CRLF is normalized in the parser; preserving a lone `\r` as data is still open. |
| Windows child environment block | **Complete for the scoped P0.1 boundary** | `internal/shell/runtime/execution_state.go`, `environment_child.go`, `external.go`; `environment_test.go`, `runtime_external_test.go` | Internal names remain exact-case; Windows spawn serialization deterministically keeps the latest case-insensitive mutation and preserves empty values. `COMSPEC` lookup and batch launch landed separately in P0.5 Wave B. |
| UTF-8/wide API and long paths | **Missing as an explicit boundary** | No shared Windows API/extended-length path layer or acceptance tests found. | Exercise non-ASCII argv/env/path and long-path filesystem/process launch through Windows boundaries. |
| Ctrl-C and debug diagnostics | **Partial (P0.4 Ctrl-C complete)** | `cmd/nemosh/signal.go`, `main.go`, and `session.go` install a fresh interrupt context for active shell/direct-applet execution; `signal_test.go`, `direct_applet_interrupt_test.go`, and `signal_windows_acceptance_test.go` cover injected and production Windows acceptance. | Idle prompt/input Ctrl-C is explicitly not a P0.4 guarantee. Layered path/exec/fd hints and `NEMOSH_DEBUG` remain P1.1. |

### Parser, semantics, pipelines, jobs, and errors

| Capability | Status | Evidence | Gap |
| --- | --- | --- | --- |
| Parser representation | **Complete for P0.3; partial for broader v0 grammar** | `internal/shell/runtime/ast.go`, `ast_word.go`, `parser.go`, `parser_typed.go`, `parser_compound_typed.go`, `parser_group.go`, `parser_function.go`, `parser_background.go`, `heredoc_parse.go`, and `lexer.go`; `ast_parser_test.go`, `background_parser_test.go`, `background_construct_test.go`, `group_parser_test.go`, `heredoc_test.go`, `function_test.go`, `parser_limits_test.go`, `parse_no_prefix_test.go`, and `typed_word_execution_test.go` | Typed program, list, pipeline, executable command, brace group, subshell, function definition, background wrapper, word, redirect/heredoc, if, loop, and case nodes are the sole execution source. Broader v0 expansion and command grammar remain partial. |
| Expansions | **Partial** | `internal/shell/runtime/expand.go`, `parameter_default.go`, `parameters.go`, `command_substitution.go`; `parameters_test.go`, `runtime_test.go` | Expansion phases, quoting/field splitting, parameter operators, arithmetic, status/special parameters, globbing, and diagnostics are incomplete. |
| Redirections/heredocs | **Partial** | `internal/shell/runtime/redirect_parse.go`, `redirect_apply.go`, `heredoc_parse.go`, `heredoc_expand.go`, `fd_table.go`; `redirect_parse_test.go`, `numbered_redirect_test.go`, `heredoc_test.go`, `runtime_relative_io_test.go` | Numbered `<`, `>`, `<&`, `>&`, close forms, `<<`, and `<<-` execute left to right. Append/read-write/clobber-control and redirect-only commands remain absent. |
| Pipelines | **Complete for bounded P0.2; partial for full v0 grammar and acceptance** | Active typed dispatch runs from `executeTypedScript` through `executeTypedPipeline` in `internal/shell/runtime/execute_pipeline.go`, then through shared `preparePipeline` and `executeTokenPipeline` in `token_pipeline.go`; `token_execution.go` retains legacy token infrastructure. Focused tests are `pipeline_test.go`, `token_pipeline_concurrent_test.go`, `token_pipeline_fd_test.go`, `token_pipeline_compatibility_test.go`, `token_pipeline_cancellation_test.go`, and `stream_serialization_test.go`. | Concurrent `os.Pipe` stages stream beyond pipe capacity, close writers for EOF, preserve explicit redirect precedence, isolate stage state/control/FD mappings, serialize shared writers, normalize expected early downstream closure, and select lexical default/`pipefail` status. Native child portable FD inheritance remains 0/1/2. Full corpus/differential and live cross-platform CI remain P1.2. |
| Command substitution | **Partial but meaningful** | `internal/shell/runtime/expand.go`, `snapshot.go`; `command_substitution_snapshot_test.go`, `typed_word_execution_test.go`, `execution_state_test.go` | Parsed nested `Script` nodes execute directly in isolated snapshots with newline trimming. Broader grammar, status, NUL, quoting, and FD behavior remain. |
| Control flow | **Partial** | `internal/shell/runtime/execute_ast.go`, `execute_compound.go`, `execute_group.go`, `function.go`; `control_test.go`, `case_test.go`, `compound_span_execution_test.go`, `group_execution_test.go`, `function_test.go` | Brace groups and function calls propagate control in the current runtime; functions consume return at the nearest function boundary, sourced files consume return at their own boundary, and subshells isolate escaping control. Case matching remains limited. |
| Special/stateful builtins | **Partial (jobs/wait complete for P0.4)** | Implementations under `internal/shell/runtime/`; tests include `special_builtin_test.go`, `exec_test.go`, `return_test.go`, `job_registry_test.go`, `job_scope_claim_test.go`, `job_interrupt_test.go`, and `job_failure_test.go`. | `jobs` and `wait` meet the frozen P0.4 contract. Full function/source/eval/getopts/read/umask breadth and POSIX error behavior remain broader v0 work. |
| Background jobs and wait | **Complete for P0.4** | `internal/shell/runtime/execute_ast.go`, `job_scope.go`, `job_supervisor.go`, `jobs.go`, `wait.go`, `snapshot.go`; `background_jobs_test.go`, `job_registry_test.go`, `job_supervisor_test.go`, `job_scope_claim_test.go`, `job_isolation_test.go`, `job_owner_cancellation_test.go`, `job_failure_test.go`, and `private_scope_cat_teardown_test.go`. | Complete typed units launch without waiting; one session-wide budget retains at most 64 unconsumed records across root/private scopes while IDs, claims, and visibility stay owner-local. `jobs` is observational; waits consume exact records only after success. Full terminal job control remains deferred. |
| Traps/signals | **Complete for P0.4** | `internal/shell/runtime/interrupt.go`, `trap.go`, `script.go`, `interactive.go`; `cmd/nemosh/signal.go`, `main.go`, `session.go`; focused trap, interrupt, direct-applet, external-helper, and Windows acceptance tests. | P0.4 supports exactly shell-level EXIT and INT for active execution/wait. TERM, targeted POSIX child SIGINT, process groups, Job Objects, ConPTY, and idle-input handling are not promised. |
| Errors and diagnostics | **Partial** | `internal/shell/runtime/script.go`, `external.go`, and focused runtime tests | Launch failures collapse to “not found”; required first-line/hint/debug layering and path/exec/fd details are absent. Parse-prefix execution also needs a parse-before-execute differential gate. |

### Applets

`internal/applets/registry.go` registers all initial names listed in the v0 scope:
`[`, `test`, `true`, `false`, `echo`, `printf`, `pwd`, `env`, `printenv`,
`cat`, `head`, `tail`, `wc`, `basename`, `dirname`, `ls`, `mkdir`, `rmdir`,
`rm`, `cp`, `mv`, `touch`, `chmod`, `grep`, `sed`, `find`, and `xargs`.
Implementations are distributed across `internal/applets/core.go`, `files.go`,
`fs.go`, `fs_more.go`, `more_core.go`, `head_tail.go`, `wc.go`, `env.go`,
`chmod.go`, `grep.go`, `sed.go`, `find.go`, and `xargs.go`.

That is **complete by registered name only** and **partial for release readiness**.
Direct `_test.go` coverage exists for many implementations, but the checked-in
TOML corpus under `tests/behavior/applets/` covers only a minority. `ls` is the
one initial v0 candidate that now carries checked-in smoke and negative cases,
`tests/behavior/applets/coreutils/ls-basic.toml` and `ls-invalid-option.toml`;
`TestBehaviorAppletScriptCases_executeAgainstFreshNemosh` in
`internal/testutil/behavior/case_test.go` executes both against a freshly built
product binary. That evidence covers exactly those two cases and does not make
`ls`, the applet corpus, the differential runner, or product CI complete. The
initial v0 candidates still missing checked-in applet behavior cases
include `[`, `test`, `pwd`, `env`, `printenv`, `cat`, `head`, `tail`, `wc`,
`basename`, `dirname`, `mkdir`, `rmdir`, `rm`, `cp`, `mv`, `touch`,
`chmod`, `grep`, `sed`, `find`, and `xargs`. Registration must not be used as a
substitute for the smoke-plus-negative release rule in
`docs/testing/applet-test-inventory.md`.

### Behavior corpus and CI

| Capability | Status | Evidence | Gap |
| --- | --- | --- | --- |
| Behavior case format/parser | **Partial but substantial** | `docs/testing/behavior-test-format.md`, `internal/testutil/behavior/case.go`, `parse.go`, `runner.go`, `runner_test.go`, `case_test.go` | Checked-in tests discover and validate all behavior TOMLs and execute both shell cases and applet script cases against a fresh Nemosh binary. Differential reference fan-out and product CI remain absent. |
| Golden corpus | **Partial** | `tests/behavior/applets/` and `tests/behavior/shell/`; shell cases and applet script cases run against a freshly built `nemosh -c` product binary | Sparse applet coverage; applet cases that declare `command` rather than `script` still run in-process against the registry, so full-corpus product execution remains incomplete. |
| Differential runner | **Missing** | Planning exists in `docs/research/behavior-matrix.md`; case metadata can name references. | No executable fan-out, equivalent sandboxing, comparison, normalization policy, or divergence report against BusyBox-w32/ash/dash. |
| Product CI | **Missing** | `.github/workflows/research-probe.yml` runs `scripts/research/posix-smoke.sh`. | The workflow probes reference shells only. No workflow builds Nemosh, runs `go test ./...`, executes the corpus/differential runner, validates manifests, or tests native Windows plus Unix product behavior. |

## Ordered Completion Waves

The waves order existing v0 must-haves; they do not create new v0 scope.

### P0.1 - Execution context and FD table, implemented

Build on the runtime-owned cwd/environment and assignment fixes in
`internal/shell/runtime/execution_state.go` and `assignment.go`. Introduce the
shell-owned descriptor model required by `v0-scope.md`, route redirection,
devices, snapshots, child inheritance, and command substitution through it, and
move Windows environment deduplication to the child-spawn boundary.

**Evidence:** numbered descriptors have tested open/dup/close and left-to-right
ordering semantics; descriptor mappings are snapshot-isolated with shared open
descriptions and exact-once cleanup; applets, builtins, command substitution,
devices, concurrent pipeline stages, and external FD 0/1/2 use the table. Runtime
snapshots do not mutate host cwd/environment; special-builtin assignments remain
persistent; regular command assignments remain temporary; empty environment
values survive; Windows `PATH`/`Path` collision tests prove exact-case shell
state and deterministic child-block deduplication; `/dev/fd/N` is enabled.

**Portable boundary:** arbitrary native-child descriptors above 2 are not
promised because Go's Unix `ExtraFiles` is not a portable Windows CRT descriptor
contract. P0.2 uses this table for active pipeline endpoints; P0.3 still owns
the complete grammar and AST.

**Closure verification:** focused and repository-wide race/shuffle tests, vet,
formatting, diagnostics, hands-on Windows CLI scenarios, and independent goal,
quality, security, QA, and context reviews passed after quote-provenance
regressions were added for single-quoted, backslash-protected, and explicitly
empty redirect operands.

### P0.2 - Real pipelines, implemented

Active parsed pipelines run from `executeTypedScript` through
`executeTypedPipeline` in `internal/shell/runtime/execute_pipeline.go`, then use
the shared `preparePipeline` and `executeTokenPipeline` implementation in
`token_pipeline.go`. The retained `token_execution.go` path is legacy token
infrastructure, not active typed dispatch. All stages are snapshotted before launch, adjacent
stages are connected with FD-table-owned `os.Pipe` endpoints, and stage
goroutines start without waiting for upstream completion. Closing each stage FD
table closes its pipe writer and delivers EOF downstream. Explicit redirects
are applied after pipeline endpoint binding and therefore take precedence.

**Bounded guarantees:** `token_pipeline_concurrent_test.go` covers concurrent
startup, 2 MiB streaming beyond pipe capacity, EOF, expected early reader exit,
stderr separation, explicit redirect precedence, prelaunch state snapshots,
incoming saved status, stage control isolation, and lexical default/`pipefail`
status. `token_pipeline_fd_test.go` covers inherited shell FD 3, stage-local
close/rebind isolation, and serialized writes to a shared borrowed description.
`stream_serialization_test.go` covers serialization when pipeline stages share
the same borrowed stdout/stderr writer. `token_pipeline_compatibility_test.go`
covers native-to-applet and applet-to-native flow plus native producer early-close
normalization. `token_pipeline_cancellation_test.go` covers cancellation closure
that unblocks non-cooperative pipe reads and full writes; Windows additionally
uses `CancelIoEx` through `pipe_interrupt_windows.go`.

**Boundaries:** native child portable descriptor inheritance remains limited to
FD 0/1/2; shell FD 3 coverage applies to builtins/applets and stage mapping
isolation, not arbitrary native-child inheritance. The retained
`internal/shell/runtime/pipeline.go` argv helper is not the active parsed
pipeline path. Unix cancellation support is compile-verified in this working
tree, but live Unix product CI, the complete behavior corpus, differential
comparison, and product workflows remain P1.2.

**Closure verification:** focused P0.2 tests and full
`go test -race -shuffle=on ./...`, `go vet ./...`, `go build ./...`, and Windows
CLI QA passed. The P0.2 production files remain below the 250-line limit; the
largest, `token_pipeline.go`, is 126 lines. Independent reviews recorded goal
`bg_2df65873` PASS, QA
`bg_12b6aafc` PASS, context `bg_a43004eb` implementation PASS with stale docs,
final quality `bg_d4d53c35` PASS, and final robustness `bg_ca3b495d` PASS. This
ledger update closes the stale-doc finding only; it does not mark v0 ready.

P0.3 typed syntax is implemented and closed for the selected parser wave,
including functions, heredocs, groups, subshells, and background-list intent.

### P0.3 - Parser AST, heredocs, functions, and subshells

Replace raw-line execution with typed syntax sufficient for the selected v0
grammar, including redirections/heredocs, functions, grouping, subshells, and
background/pipeline structure. Retain the scope’s freedom to order exact grammar
milestones by implementation tests.

**Completed foundation:** parsing completes before effects and produces typed
program/list/pipeline/command/word/redirect/control-flow nodes used directly by
execution. Command substitutions execute their parsed nested `Script` rather
than serialized source. Shared input, token, compound, group, subshell, and substitution-depth
ceilings bound parsing, and CLI batch/interactive readers stop allocation at the
same input-size contract. Brace groups and parenthesized subshells are typed executable
commands with whole-command redirects and pipeline-stage isolation. Typed heredoc redirects
retain delimiter words, quote-removal and expansion policy, tab-stripping mode, bodies, and
encounter order; bodies are collected FIFO before execution and bind owned in-memory readers
through the descriptor table in lexical redirect order. Quoted delimiters preserve literal
bodies, unquoted bodies use the supported parameter and command-substitution expansion subset,
and `<<-` strips leading TAB bytes only. Portable `name() compound-command` definitions store
typed bodies and invocation-time redirects, register without body execution, replace prior
definitions, and execute in the current shell with temporary positional parameters and explicit
function-return boundaries. Runtime snapshots clone the function registry while sharing immutable
typed bodies, so subshells, command substitutions, and pipeline stages inherit functions without
leaking child definitions or redefinitions. Active `&` marks complete and-or lists and typed
function/compound nodes without source serialization; same-line lists retain source order.
At P0.3 closure, marked nodes intentionally executed synchronously with their real status. That
temporary parser/runtime boundary has been superseded by P0.4 asynchronous execution, retained
job lifecycle, `jobs`, `wait`, signals, and reaping. Interactive trailing `&` is complete input,
while trailing pipelines, logical operators, redirect operands, and pending heredocs continue
across physical lines. P0.3 is **Complete** for its bounded parser milestone.

**Pass criteria:** parse completes before execution; typed nodes preserve quote,
redirection, and background-list structure; quoted/unquoted and tab-stripped
heredocs pass; function definition/call/positional parameters/return pass;
grouping and subshell state isolation pass; `&` placement, nested constructs,
interactive completion, and temporary synchronous execution are tested;
malformed and incomplete syntax has status 2 and does not execute a valid prefix.

**Closure verification:** focused parser, resource-limit, no-prefix, interactive,
group, heredoc, function, and background tests passed with repeated adversarial
runs. Full `go test -race -shuffle=on -count=1 ./...`, `go vet ./...`,
`go build ./...`, fresh-binary shell-corpus execution, Windows CLI QA,
format/diff checks, LSP diagnostics, and the 250-pure-LOC production-file gate
passed. Independent review initially found late parser-budget checks, malformed
and-or acceptance, forgeable group placeholders, and incomplete background
coverage; those issues were fixed and final skeptical review `bg_fadbb498`
returned PASS. This closes P0.3 only; it does not mark v0 ready.

### P0.4 - Jobs and signals, implemented

Implement `cmd &`, a job registry, process lifecycle, operand-aware `wait`, and
basic `jobs`; wire console Ctrl-C to shell `INT` where Windows permits and keep
unsupported semantics explicit.

**Completed foundation:** complete typed background and-or lists and background
compound/function nodes snapshot state and descriptors before launch, register
before starting their goroutine, and return launch status 0 without replacing the
foreground status. A count-only session supervisor admits at most 64 retained
records across root and private scopes; each owner retains its own monotonic IDs,
record map, claims, `jobs` visibility, and wait operands. Completed records retain
their status and capacity until a successful exact wait consumes them. Canceled
waits release claims without consuming records; duplicate identities and
consume-vs-private-teardown races release supervisor capacity exactly once.

Background workers clear inherited traps and default FD 0 to null unless redirected.
Pipelines retain lexical default/`pipefail` status. Workers, subshells, command
substitutions, and pipeline stages own hierarchical private scopes; normal and
failed setup/admission paths cancel and drain nested jobs before releasing their
FD tables. Context-aware in-process `cat` copying prevents `/dev/zero` to
`/dev/null` from defeating private teardown. Root close remains deliberately
seal-only: EXIT may inspect/wait known jobs, but close neither implicitly waits,
cancels, nor kills live root records, and process-exit survival is not promised.

Shell-owned interrupt provenance maps active execution/wait cancellation to 130
without treating an ordinary status 130 as INT. INT traps remain installed across
events with non-reentrant dispatch; EXIT remains one-shot and `exec` suppresses it
through final close. Batch, interactive, and both direct-applet forms use fresh
controller contexts. Windows Ctrl-Break acceptance exercises production
`signal.Notify`; external cancellation uses `exec.CommandContext` termination and
does not claim targeted POSIX SIGINT.

**Pass criteria:** background launch returns without waiting; `jobs` reports
live/completed shell-launched jobs; `wait` and `wait <job>` return tested status;
resources are reaped; EXIT and INT behavior is tested for shell and child;
Windows limitations are documented in diagnostics rather than silently
emulated. Full `fg`/`bg`, stopped jobs, process groups, and ConPTY control remain
deferred exactly as scoped.

**Closure verification:** deterministic RED-to-GREEN regressions covered
asynchronous launch, status retention/claims, session-wide admission, failed
admission ID preservation, exact capacity release, private owner cancellation,
failed setup cleanup, FD close errors, `/dev/zero` teardown, INT provenance and
persistence, EXIT/`exec`, direct applets, and the Windows production signal
boundary. Focused race/shuffle tests passed repeatedly, including final `count=20`
resource/lifecycle runs. Final `go test -race -shuffle=on -count=1 ./...`, the
standalone behavior corpus, `go vet ./...`, `go build ./...`, gofmt, changed-file
LSP diagnostics, and the 250-pure-LOC production-file gate passed. Two independent
fresh-binary six-scenario QA runs passed with hard process deadlines and complete
binary/helper teardown. Final post-fix reviews returned goal `bg_bfe492ad` PASS,
quality `bg_31bfa68a` PASS, resource `bg_a2118929` PASS, QA
`bg_53e10134` PASS, and context `bg_84b6d198` PASS. This closes P0.4 only; P0.5
remains the next ordered v0 wave.

### P0.5 - Windows boundaries

Integrate `internal/pathmodel` across shell-owned filesystem and lookup
operations; implement device clipboard, `ComSpec` batch launch, `.sh`/shebang
dispatch, UTF-8/wide boundaries, and long-path handling.

**Pass criteria:** native Windows tests cover all required input path forms,
drive/UNC current roots, configurable aliases and virtual roots, host-only UNC
hints, no general argv conversion, `/dev/clipboard` UTF-8 text I/O, fixed suffix
order, batch quoting/status, shebang/CRLF scripts, non-ASCII argv/env/path, and
long filesystem/process paths.

P0.5 is ordered as three sub-waves so path integration lands before launch and
encoding boundaries depend on it:

| Sub-wave | Content | Status |
| --- | --- | --- |
| Wave A | Route shell-owned filesystem and lookup operations through `internal/pathmodel`. | **Complete.** `Runtime.ResolveNemoshPath` is the single seam; see the runtime-integration note above. Remaining native Windows test breadth is tracked there, not here. |
| Wave B | `/dev/clipboard`, fixed suffix lookup, `ComSpec` batch launch, `.sh`/shebang/CRLF dispatch, applet override configuration. | **Partial.** Suffix lookup, batch launch through `ComSpec`, and `.sh`/shebang dispatch are complete; see the three Windows launch rows above. `/dev/clipboard`, applet override configuration, and lone-`\r` preservation remain. |
| Wave C | UTF-8/wide API boundaries and internal long-path handling. | **Not started.** |

Explicitly outside P0.5: general argv path conversion, user mount namespaces,
P1.1 diagnostics, and interactive PTY or job-control work.

### P1.1 - Expansion and diagnostics

Complete the selected parameter/command/arithmetic expansion behavior and
shell-word phases needed by the v0 corpus. Implement the required layered error
contract and `NEMOSH_DEBUG=path,exec,fd` details.

**Pass criteria:** quoting, field splitting, positional/special parameters,
selected parameter operators, command substitution, and arithmetic cases are
golden/differential tested; not-found, non-executable, directory, bad format,
redirection, parse, and Windows launch failures have stable first lines,
targeted hints, and opt-in debug details without leaking debug output by default.

### P1.2 - Behavior corpus and product CI

Turn the existing format and sparse corpus into a release gate, then add local
reference comparison and product workflows. Fill smoke and negative cases for
every initial v0 applet before treating registry presence as readiness.

**Pass criteria:** a checked-in runner discovers every
`tests/behavior/**/*.toml` case; golden results run on supported platforms;
differential adapters compare Nemosh with selected local references and report
intentional skips/divergences; CI builds and runs unit plus behavior tests on
native Windows and supported Unix runners; manifest freshness is checked; a
green workflow refers to product tests, not only the research probe.

## Generated Artifact Cleanup

The audit found repository-root `nemosh.exe` and runtime-test residue
`internal/shell/runtime/created.txt`, `internal/shell/runtime/output.txt`, and
`internal/shell/runtime/first second`; the bounded verification follow-up
removed those untracked generated files.
Other local artifacts are not implementation evidence:

- `bin/nemosh.exe` is an intentionally ignored build-output location
- `.crush/crush.db`
- `.crush/logs/crush.log`
- `.omo/` session/continuation state, only if local recovery history is no longer
  needed

Do not classify untracked Go files, behavior cases, or `go.sum` as disposable
without review; they may be substantive current work rather than generated
residue.
