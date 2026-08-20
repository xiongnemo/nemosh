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
Windows-only paths are incompletely exercised, differential references are not
run end to end, and there is no product CI.

**How this ledger was corrected (2026-08-07).** A thirteen-agent audit found 25
defects, 18 of them rows in this file claiming more than the code did. Every one
was re-measured against a freshly built binary rather than against the test
suite, and every confirmed one was fixed; the rows below were then rewritten
from a second measurement of the fixed binary rather than from the old text.
Two audit findings did not survive re-measurement and are recorded as such: an
executable path past MAX_PATH launches (804 characters, measured), and the
clipboard round-trip failure did not reproduce in 25 runs.

**Two more found the same way (2026-08-07, after CI went green).** Re-measuring
this table's own claims against a fresh binary found two defects the suite could
not see. `times` reported `113371174m33.710s` for a shell that had just started,
because a CPU FILETIME is an elapsed amount and was read with
`syscall.Filetime.Nanoseconds`, which is for the instant form and subtracts the
1601-to-1970 offset first; the result underflows and wraps to a positive 215
years. Its test asserted only the `%dm%fs` shape, which the wrong value
satisfies. And a brace was treated as reserved wherever it appeared, so
`echo a}b`, `echo {}`, `echo x{1}y`, `f(){ echo a; }`, and every `$(( ))` inside
a brace group were syntax errors; busybox ash, dash, and bash accept all of
them. Both are fixed, and the second moved three existing expectations toward
the references rather than away.

The rule the corrections followed, and the one this ledger is now held to:
**a capability that is absent must fail loudly.** Most of what was found was not
missing so much as silently wrong -- `test -f x` answered false instead of
saying it had no file predicates, `${x%.txt}` expanded to its own six
characters, `wc -z FILE` selected no counts and exited 0, `printf '%d' 42`
printed `%!d(string=42)`, and `rmdir notes.txt` deleted the file. A green suite
cannot see any of that, which is why the remainder columns below name behaviour
measured against a binary and cite the command that measured it.

## Executive Status

| `v0-scope.md` section | Status | Current evidence | Blocking remainder |
| --- | --- | --- | --- |
| Product Target | **Partial** | Native Go CLI and multicall dispatch exist in `cmd/nemosh/main.go`; bundled applets are registered by `internal/applets/registry.go`; P0.4 implements the bounded Windows jobs/INT model; P0.5 integrates the path, launch, clipboard, encoding, and long-path boundaries. A direct `nemosh cat missing` and `cat missing` inside the shell now fail identically, through one shared mapping. Layered diagnostics and `NEMOSH_DEBUG=path,exec,fd` are in (`diagnostic.go`, `external_diagnostic.go`; measured: `NEMOSH_DEBUG=exec nemosh -c nosuchcommand` prints the hint and the searched directories). | `hash`, `ulimit`, `fg`, and `bg` are refused rather than implemented, each naming what busybox-w32 does with the same name (measured: status 126). |
| V0 Must Have | **Partial** | `-c`, a script operand, and redirected-stdin execution are tested in `cmd/nemosh/main_test.go` and `script_file_path_test.go`; P0.1 owns the FD table and snapshots, P0.2 concurrent `os.Pipe` pipelines, P0.3 typed groups/subshells/heredocs/functions, P0.4 background execution and the bounded Windows INT model, P0.5 the Windows boundaries, and both P1.1 halves — expansion and diagnostics — are now in. 143 behavior cases, 1193 Go test functions, 86.7% statement coverage. | Four builtins are refused rather than implemented, loudly and by name. Per-applet option matrices remain the largest uncovered surface; see the Applet Scope row. |
| Windows Path Gates | **Complete for the model's stated set** | `internal/pathmodel/model.go` and `pathmodel_test.go` cover drive, mount-alias, UNC, current-root, and virtual-root forms. `internal/shell/runtime/path_state.go` exposes them as `Runtime.ResolveNemoshPath`, which `cd`, `.`/`source`, redirection, device input, external lookup, and the applet `ProcessView` all call; `runtime/path.go` is now a thin adapter over that seam rather than a separate model. Native Windows integration tests now cover UNC current roots, the alias and virtual-root switches, the host-only UNC hint, and argv non-conversion. | Case resolution landed on 2026-08-07 (`path_case_windows.go`): `cd` stores the directory under the spelling the filesystem reports for each component, so `pwd` answers with the on-disk case and an unanswerable path silently keeps the spelling as typed, which is both halves of what the model asks for. FindFirstFile per component rather than GetLongPathName -- the latter is documented to expand 8.3 names and only corrects case incidentally, and measured on Windows 10 19045 it leaves `documents` lowercase where the directory on disk is `Documents`. Extended-length paths landed in Wave C, deliberately without a shared prefix layer; see the UTF-8/long-path row. |
| V0 Device Paths | **Complete for the v0 required set** | `internal/shell/runtime/device.go`, `device_fd.go`, `device_clipboard.go` and its `_windows.go`/`_other.go` backends; `device_test.go`, `device_fd_test.go`, `device_clipboard_test.go`, `device_clipboard_windows_test.go` cover `/dev/null`, descriptor-backed standard streams and `/dev/fd/N`, zero, random, urandom, and UTF-8 text `/dev/clipboard`. | The clipboard is Windows-only by design; off Windows the path resolves as a device and then reports that plainly. `/dev/tty` and PTY paths stay deferred, as the model says. |
| Execution Gates | **Complete** | Lookup order is implemented across `internal/shell/runtime/runtime.go`, `internal/shell/runtime/command.go`, `internal/shell/runtime/applet_override.go`, and `internal/shell/runtime/external.go`; fixed Windows suffix ordering, `NEMOSH_OVERRIDE_APPLETS`, `ComSpec` batch launch, `.sh`/shebang dispatch, exact runtime `PATH`, scoped Windows child-environment collapse, and runtime cwd/env propagation have focused tests. | Long paths landed in Wave C: a child launched from a directory over 258 UTF-16 characters falls back to its 8.3 short name, or is refused by length. |
| Job, Signal, And Error Gates | **Partial (P0.4 jobs/signals complete)** | Typed background nodes launch asynchronously into a session-owned bounded supervisor; `jobs` observes retained local records; `wait`/`wait %N` claim, retain on cancellation, and consume exact statuses. EXIT/INT traps, direct applet and foreground cancellation, production Windows Ctrl-Break acceptance, private-scope reaping, and honest root-close limitations have focused tests. | Layered diagnostic hints and `NEMOSH_DEBUG=path,exec,fd` remain P1.1 work. Full `fg`/`bg`, stopped jobs, process groups, Job Objects, ConPTY, and idle-input Ctrl-C remain explicitly outside P0.4. |
| Parser And Shell Semantic Gates | **Partial** | `parser.go`, `syntax_scan.go`, `parser_typed.go`, `parser_compound_typed.go`, `parser_group.go`, `parser_function.go`, and `ast.go` build a typed program before execution. Sequential lists (`;`), the one-line forms of `if`, `for`, `while` and `case`, `elif` chains, `!` pipeline negation, backquote command substitution, `$(( ))`, `>|`, and `<>` all parse; see the grammar row below for what each of those used to do instead. A brace is reserved only in command position (2026-08-07), so `echo a}b`, `echo {}`, and `f(){ echo a; }` behave as busybox ash, dash, and bash do; before that each was a syntax error, and arithmetic inside any brace group was unusable. | `hash`, `ulimit`, `fg`, and `bg` are refused with 126 and a reason (measured). `~user` is left as written. An alias value that is not a list of words is refused at definition time, because substitution happens after parsing. `set -b`, `-n`, and `-v` are refused with 2 and a reason, since parse-before-effects makes `-n` and `-v` unimplementable as specified. |
| Applet Scope Direction | **Partial** | Every initial candidate name is registered in `internal/applets/registry.go`, with implementations under `internal/applets/`. 88 applet behavior cases and 1060 Go test functions across the tree; 86.2% statement coverage in `internal/applets`. | Name presence is not semantic parity. The per-applet option matrices in `docs/testing/applet-test-inventory.md` (`ls -l`, `head -c`, `grep -r`, `find -name`, `xargs -0`, …) are still uncovered, and the options those applets do not implement are now refused rather than swallowed, so a script asking for one fails instead of getting something else. |
| Deferred Non-Goals | **Complete as a boundary observation** | No evidence was found that v0 completion depends on full REPL polish, native POSIX job control, MSYS/Cygwin argv conversion, WSL mounts, certification, or full BusyBox parity. | These remain non-goals; they must not be used to defer any existing must-have above. |

## Detailed Evidence Ledger

### Product must-haves and execution context

| Capability | Status | Exact implementation/test evidence | Readiness boundary |
| --- | --- | --- | --- |
| Non-interactive runner | **Complete** | `cmd/nemosh/main.go`, `cmd/nemosh/script_file.go`; `cmd/nemosh/main_test.go`, `cmd/nemosh/script_file_test.go`, `cmd/nemosh/script_file_path_test.go`, `cmd/nemosh/direct_applet_diagnostic_test.go` | `-c`, stdin, and `nemosh script.sh [args]` all dispatch, and the script operand goes through the shell's own path model, so every spelling the shell prints is one it takes back. A script file seeds `$0` from the operand as written and `$1…` from the rest; an unreadable one is status 127. Applet names still win over same-named files, matching `busybox cat`. An operand starting with `-` that is not `-c`, `-i`, or a bare `-` is an invalid option with status 2. |
| Runtime-owned cwd | **Complete for implemented operations** | `internal/shell/runtime/execution_state.go`, `path.go`, `external.go`; `execution_state_test.go`, `runtime_relative_io_test.go`, `runtime_external_test.go`, `applet_process_view_test.go` | This just-landed state no longer relies on process-global cwd, but broader pathmodel integration remains separate. |
| Runtime-owned environment | **Complete for isolation, child propagation, and the scoped Windows child block** | `internal/shell/runtime/execution_state.go`, `environment_child.go`, `assignment.go`, `external.go`; `environment_test.go`, `execution_state_test.go`, `assignment_test.go`, `runtime_external_test.go` | Shell/export state preserves exact-case names and empty values. Windows child serialization alone performs deterministic case-insensitive, latest-mutation-wins deduplication. Batch launch now reads `COMSPEC` from this table (`internal/shell/runtime/external_batch.go`). |
| Environment case/path fixes | **Complete for P0.1** | `Environment` stores exact names and mutation order; `environment_child.go` serializes Unix entries exactly and Windows entries by case-insensitive latest mutation; executable lookup still uses exact `PATH`. | Canonical `COMSPEC` handling and `.bat`/`.cmd` launch landed in P0.5 Wave B and read this table by its canonical name. |
| Unix runtime `PATH` | **Complete, platform-test limited** | `internal/shell/runtime/external.go`; non-Windows case in `internal/shell/runtime/runtime_external_test.go` | Relative and empty entries use runtime cwd. The focused test is skipped on Windows. Windows lookup and launch are proved separately by `external_suffix_windows_test.go`, `external_batch_windows_test.go` and `long_path_windows_test.go`. |
| Special-builtin assignments | **Complete for implemented special builtins; partial POSIX edge coverage** | `internal/shell/runtime/assignment.go`, `command.go`; `assignment_test.go`, `special_builtin_test.go` | Leading assignments persist for special builtins and remain temporary for regular commands. Fatal-error, redirect-only, and complete expansion-order semantics still need corpus gates. |
| Explicit snapshots | **Partial but strengthened** | `internal/shell/runtime/snapshot.go`, `fd_table.go`, `token_pipeline.go`, `execute_group.go`, `execute_ast.go`; `snapshot_fd_test.go`, `command_substitution_snapshot_test.go`, `group_execution_test.go`, `token_pipeline_concurrent_test.go`, `token_pipeline_fd_test.go`, `job_isolation_test.go`, `job_owner_cancellation_test.go` | Every active pipeline stage, parenthesized subshell, command substitution, and background worker is snapshotted. State, control flow, descriptor mappings, and private nested-job ownership are isolated while shared open descriptions retain exact-once ownership. Broader Windows path/launch integration remains separate. |
| Shell-owned FD table | **Complete for the portable P0.1 substrate and P0.2 applet/builtin pipeline use** | `internal/shell/runtime/fd_table.go`, `fd_description.go`, `redirect_parse.go`, `redirect_apply.go`, `token_pipeline.go`; `fd_table_test.go`, `fd_lifecycle_test.go`, `numbered_redirect_test.go`, `device_fd_test.go`, `snapshot_fd_test.go`, `token_pipeline_fd_test.go` | Numbered open/dup/close, left-to-right ordering, `/dev/fd/N`, applet/builtin stdio, snapshots, command substitution, pipeline endpoints, inherited shell FD 3, and external FD 0/1/2 are covered. Stage-local close/rebind of FD 3 does not mutate sibling or parent mappings. Arbitrary native-child inheritance above FD 2 is not portably promised. |

### Windows paths, devices, and process launch

Isolated pathmodel completeness must not be confused with runtime integration:

- **Isolated model: partial but substantial.** `internal/pathmodel/model.go` and
  `internal/pathmodel/pathmodel_test.go` cover drive forms, `/c`, `/mnt/c`, UNC
  shares, drive/UNC current roots, virtual-root preservation, host-only UNC
  rejection, and default-off Cygdrive behavior. Malformed forms beyond host-only
  UNC remain, as does resolving a path's real filesystem case. Extended-length
  paths are deliberately not modelled here at all; see the UTF-8/long-path row.
- **Runtime integration: connected through one seam.** `internal/shell/runtime/path_state.go`
  resolves through `internal/pathmodel` and is the single seam shell-owned
  operations use: `cd` at `state_builtins.go:58`, `.`/`source` at `builtins.go:15`,
  redirection at `redirect_apply.go:38,63`, device input at `device_input.go:60`,
  executable lookup at `external.go:99,117`, the child working directory at
  `external.go:23`, and the applet `ProcessView` at
  `internal/applets/process_view.go:28,37`.
  `runtime/path.go` delegates to that seam instead of carrying its own rules.
  Opt-in `/tmp` backing and Cygdrive conversion are implemented as policy in
  `path_state_windows.go` and `path_state_other.go`. The host-only UNC hint now
  comes from `pathmodel.HostOnlyUNCError` instead of a copy inside `cd`, so `cd`,
  redirection, and `.` word it identically and a hostless `//` is reported as
  malformed rather than advised to grow a share
  (`path_shell_io_test.go`, `path_state_unc_windows_test.go`).
  Native Windows breadth landed with it: UNC current roots
  (`path_state_unc_windows_test.go`), the cygdrive, mount-prefix, `/tmp`, and
  `/dev` switches (`path_settings_windows_test.go`), and argv non-conversion
  (`external_argv_test.go`). What remains is real-case resolution, which is a
  missing capability rather than missing coverage.

| Boundary | Status | Evidence | Required acceptance remainder |
| --- | --- | --- | --- |
| Accepted Windows spellings/current root/UNC | **Partial** | `internal/pathmodel/model.go`, `internal/pathmodel/pathmodel_test.go`, `internal/shell/runtime/path_state.go` and its `path_state*_test.go` family, `internal/shell/runtime/path_shell_io_test.go`, `path_state_unc_windows_test.go`, `path_settings_windows_test.go`, `external_argv_test.go`, `internal/applets/path_test.go` | Shell-owned operations share one model, and the native Windows tests now reach it: a UNC share is the current root and `pwd` keeps that spelling instead of falling back to the drive, cygdrive and a moved mount prefix change what is an alias, a disabled scheme degrades to an ordinary path rather than an error, `/tmp` is backed by the host temporary directory, and path-shaped argv is handed to a child unconverted. The quoted input form `"C:\Users\nemo"` works as well, which it did not until the double-quote backslash rule was fixed in the lexer. Remaining work is resolving a path's real filesystem case; extended-length paths landed in Wave C. |
| Device set | **Complete for the v0 required set** | `internal/shell/runtime/device.go`, `device_clipboard.go`, `device_clipboard_windows.go`, `device_clipboard_other.go`, `redirect_apply.go`, `fd_table.go`; `device_test.go`, `device_fd_test.go`, `device_clipboard_test.go`, `device_clipboard_windows_test.go` | `/dev/std*` and `/dev/fd/N` are descriptor-backed. `/dev/clipboard` is UTF-8 text over `CF_UNICODETEXT`: a read is a snapshot taken at open, a write replaces the whole slot at close, `>>` seeds itself with what is already there, and line endings translate both ways (CRLF on the clipboard, LF in the shell, a lone CR left as data). No corpus fixture: the clipboard is one machine-wide slot, so a fixture would clobber whatever the user had copied and could not put it back. The Go tests borrow and restore it byte for byte, and skip when it holds a format they cannot reproduce. |
| Native executable lookup | **Complete** | `internal/shell/runtime/external.go`, `external_format.go`; `external_format_test.go`, `external_suffix_windows_test.go`, `runtime_external_test.go` | Lookup follows busybox-w32 `add_win32_extension` (`win32/mingw.c:2237`): the bare name is tried first and accepted when it carries an executable suffix or sniffs executable, and only a name with neither suffix nor trailing dot gets `.com .exe .sh .bat .cmd` appended in that order. Sniffing is the pragmatic subset of busybox's PE walk — `#!` or `MZ`, `.dll` excluded. |
| Applet override | **Complete** | `internal/shell/runtime/applet_override.go`, `runtime.go`, `command.go`, `external_script.go`; `applet_override_test.go`, `applet_override_lookup_test.go`, `tests/behavior/shell/posix/applet-override-named.toml`, `tests/behavior/shell/posix/applet-override-all.toml` | `NEMOSH_OVERRIDE_APPLETS` reproduces busybox's `prefer_applet_internal` grammar (`libbb/appletlib.c:296`): `-` disables every applet, `+` disables one wherever an external exists, and a list disables names before the first `;` outright and names after it conditionally, separated by space, comma, or semicolon. One gated lookup answers dispatch, `command -v`, and shebang applet interpreters, so an overridden applet is reported absent and lookup falls through to `PATH`. Two deliberate differences from busybox, both recorded in `windows-execution-model.md`: the value is an ordinary shell variable rather than a process-environment read, so no `export` is needed; and the override is scoped to shell lookup, leaving `nemosh <applet>` and applet shims unconditional. `PATH` is searched only for the two forms whose answer depends on it. |
| Batch and command scripts | **Complete** | `internal/shell/runtime/external_batch.go`, `external_launch_windows.go`, `external.go`; `external_batch_test.go`, `external_batch_windows_test.go` | `.bat`/`.cmd` launch as `"<ComSpec>" /d /s /c "<script> <args…>"` through `SysProcAttr.CmdLine`, so cmd's own doubled-quote convention applies instead of Go's `\"`, an argument containing `&` cannot split the command line, and `/d` suppresses AutoRun. An operand carrying a line break or two or more `%` is refused with a diagnostic and status 126 before anything runs. No corpus fixture: the executor replaces the child environment wholesale (`internal/testutil/behavior/sandbox.go:42-47`), so `COMSPEC` would have to be hardcoded per machine. |
| `.sh` and shebang scripts | **Complete** | `internal/shell/runtime/external_script.go`, `external.go`; `external_script_test.go`, `tests/behavior/shell/windows/script-sh-dispatch.toml`, `tests/behavior/shell/windows/shebang-applet-interpreter.toml` | Shebang grammar follows busybox-w32 `parse_interpreter` (`win32/process.c:66`) and mapping follows `mingw_spawn_interpreter` (`win32/process.c:301`): `/bin/sh` and `/bin/nemosh` re-enter this binary as a child process, an applet name under a Unix directory is handed to it, anything else resolves through ordinary lookup, and a chain of interpreters gives up at the fifth. `#!/usr/bin/env python3` reaches the `env` applet, which runs applets only, so it reports `python3: not found`. CRLF is normalized once on the way into parsing and a lone `\r` stays data, covered by `tests/behavior/shell/posix/crlf-lone-carriage-return.toml`. |
| Windows child environment block | **Complete for the scoped P0.1 boundary** | `internal/shell/runtime/execution_state.go`, `environment_child.go`, `external.go`; `environment_test.go`, `runtime_external_test.go` | Internal names remain exact-case; Windows spawn serialization deterministically keeps the latest case-insensitive mutation and preserves empty values. `COMSPEC` lookup and batch launch landed separately in P0.5 Wave B. |
| UTF-8/wide API and long paths | **Complete** | `internal/shell/runtime/external_directory.go`, `external_launch_windows.go`; `external_directory_test.go`, `external_directory_windows_test.go`, `long_path_windows_test.go`, `external_encoding_test.go`, `lexer_utf8_test.go`; `tests/behavior/shell/posix/utf8-non-ascii-operands.toml` | There is deliberately no shared extended-length path layer, because Nemosh does not need one: Go's `os` package applies `\\?\` for every filesystem call and Nemosh's cwd is virtual rather than the process's, so reading, writing, and `cd` past MAX_PATH work without the path model changing. The one Win32 boundary the prefix cannot widen is `CreateProcess` `lpCurrentDirectory` — measured at 258 UTF-16 characters on Windows 10 19045 — which is retried as the 8.3 short name and otherwise reported by length rather than by Windows' opaque "The directory name is invalid". Lengths count wide characters, not UTF-8 bytes: a 642-byte directory of 258 CJK characters launches. Non-ASCII operands round-trip through the filesystem, argv, and the separately built child environment block. |
| Ctrl-C and debug diagnostics | **Partial (P0.4 Ctrl-C complete)** | `cmd/nemosh/signal.go`, `main.go`, and `session.go` install a fresh interrupt context for active shell/direct-applet execution; `signal_test.go`, `direct_applet_interrupt_test.go`, and `signal_windows_acceptance_test.go` cover injected and production Windows acceptance. | Idle prompt/input Ctrl-C is explicitly not a P0.4 guarantee. Layered path/exec/fd hints and `NEMOSH_DEBUG` remain P1.1. |

### Parser, semantics, pipelines, jobs, and errors

| Capability | Status | Evidence | Gap |
| --- | --- | --- | --- |
| Parser representation | **Complete for P0.3; partial for broader v0 grammar** | `internal/shell/runtime/ast.go`, `ast_word.go`, `parser.go`, `parser_typed.go`, `parser_compound_typed.go`, `parser_group.go`, `parser_function.go`, `parser_background.go`, `heredoc_parse.go`, and `lexer.go`; `ast_parser_test.go`, `background_parser_test.go`, `background_construct_test.go`, `group_parser_test.go`, `heredoc_test.go`, `function_test.go`, `parser_limits_test.go`, `parse_no_prefix_test.go`, and `typed_word_execution_test.go` | Typed program, list, pipeline, executable command, brace group, subshell, function definition, background wrapper, word, redirect/heredoc, if, loop, and case nodes are the sole execution source. Sequential lists, one-line compounds, `elif`, `!`, backquotes and `$(( ))` were added on 2026-08-07; five scans that run before the lexer each had to learn to step over an arithmetic expansion whole, because `$((1<<4))` carries a `<<` the heredoc collector read as a redirect. |
| Expansions | **Complete for the selected v0 set** | `internal/shell/runtime/expand.go`, `parameter_default.go`, `pathname_expansion.go`, `pattern.go`, `arithmetic.go`, `arithmetic_lex.go`, `backquote.go`, `expansion_state.go`; `parameter_operator_test.go`, `field_splitting_test.go`, `pathname_expansion_test.go`, `arithmetic_test.go`, `backquote_test.go`, `pattern_test.go`, and five `tests/behavior/shell/posix/` cases | Parameter expansion covers the 2.6.2 operators and `${#name}`; field splitting follows 2.6.5 including a custom and an empty IFS; pathname expansion follows 2.6.6 with quote provenance deciding what globs; arithmetic follows 2.6.4 over the C operator set; command substitution takes both spellings. What each of those did before is in the wave note below. Absent and refused rather than silent: `${x/a/b}` and the other operators outside 2.6.2 are a `bad substitution` with status 2, arithmetic assignment is refused by name, and `~user` is left as written. |
| Redirections/heredocs | **Partial** | `internal/shell/runtime/redirect_parse.go`, `redirect_apply.go`, `heredoc_parse.go`, `heredoc_expand.go`, `fd_table.go`; `redirect_parse_test.go`, `numbered_redirect_test.go`, `heredoc_test.go`, `runtime_relative_io_test.go` | Numbered `<`, `>`, `>>`, `<&`, `>&`, close forms, `<<`, and `<<-` execute left to right, and a redirect-only `exec` rebinds the shell's own descriptors for the rest of the script (`exec_redirect_test.go`) — it used to report success, create nothing, and leave output where it was. `>|` and `<>` are refused as unsupported, with status 2. |
| Pipelines | **Complete for bounded P0.2; partial for full v0 grammar and acceptance** | Active typed dispatch runs from `executeTypedScript` through `executeTypedPipeline` in `internal/shell/runtime/execute_pipeline.go`, then through shared `preparePipeline` and `executeTokenPipeline` in `token_pipeline.go`; `token_execution.go` holds the shared expansion and dispatch path both routes
reach, not legacy code: `runParsedWords` is where every simple command is
expanded and dispatched. Focused tests are `pipeline_test.go`, `token_pipeline_concurrent_test.go`, `token_pipeline_fd_test.go`, `token_pipeline_compatibility_test.go`, `token_pipeline_cancellation_test.go`, and `stream_serialization_test.go`. | Concurrent `os.Pipe` stages stream beyond pipe capacity, close writers for EOF, preserve explicit redirect precedence, isolate stage state/control/FD mappings, serialize shared writers, normalize expected early downstream closure, and select lexical default/`pipefail` status. Native child portable FD inheritance remains 0/1/2. Full corpus/differential and live cross-platform CI remain P1.2. |
| Command substitution | **Partial but meaningful** | `internal/shell/runtime/expand.go`, `snapshot.go`; `command_substitution_snapshot_test.go`, `typed_word_execution_test.go`, `execution_state_test.go` | Parsed nested `Script` nodes execute directly in isolated snapshots with newline trimming, in both the `$( )` and the backquote spelling (`backquote.go`, `backquote_test.go`) — the older spelling was not implemented at all and reached the command line as its own literal text. An unquoted result is field-split. NUL handling and the substitution's own status remain. |
| Control flow | **Complete for the selected v0 grammar** | `internal/shell/runtime/execute_ast.go`, `execute_compound.go`, `execute_group.go`, `function.go`, `parser_case_lines.go`, `parser_elif.go`, `pattern.go`; `control_test.go`, `case_test.go`, `case_one_line_test.go`, `elif_test.go`, `sequential_list_test.go`, `pipeline_negation_test.go`, `assignment_prefix_test.go` | Brace groups and function calls propagate control; functions consume return at the nearest function boundary, sourced files at their own, and subshells isolate escaping control. Case arms match by POSIX 2.13.1 pattern with `\|` alternatives, not by string equality with a special case for a lone `*`. A leading assignment no longer hides `break`, `continue`, `exit`, `return` or `exec` from dispatch -- `while true; do V=x break; done` used to run forever. | Loop `break n` and `continue n` counts are not implemented; the operand is ignored. |
| Special/stateful builtins | **Partial** | Implementations under `internal/shell/runtime/`; tests include `special_builtin_test.go`, `exec_test.go`, `exec_redirect_test.go`, `set_builtin_test.go`, `errexit_nounset_test.go`, `trap_builtin_test.go`, `cd_builtin_test.go`, `command_v_test.go`, `return_test.go`, and the P0.4 job tests. | `set` parses the POSIX options and refuses an unknown one; `-e` and `-u` act, and `$-` and `set -o` report the rest. `trap` lists, arms and resets. `cd` honours HOME, `-`, PWD and OLDPWD. A redirect-only `exec` rebinds the shell. `command -v` reaches PATH. `:` exists. `alias`, `unalias`, `type` and `local` landed on 2026-08-07. **Absent, and loudly so:** `hash`, `ulimit`, `fg`, `bg`, `times` and `let` report `not found` with 127. `fg` and `bg` match busybox-w32, which compiles them out on Windows (`#if JOBS`, and `JOBS` is 0 under `ENABLE_PLATFORM_MINGW32`, shell/ash.c:246-252). `ulimit` deliberately does *not* match it: busybox-w32 keeps the name and stubs `shell_builtin_ulimit` to `return 1` (shell/shell_common.c), so `ulimit -n` there fails with no message and no reason — the silent-stub pattern this ledger's rule exists to forbid. **Stored but inert:** the `-a`, `-b`, `-C`, `-n`, `-v` and `-x` options are remembered and reported by `$-` and `set -o` but nothing reads them; `-f` and `-e` and `-u` do act. |
| Background jobs and wait | **Complete for P0.4** | `internal/shell/runtime/execute_ast.go`, `job_scope.go`, `job_supervisor.go`, `jobs.go`, `wait.go`, `snapshot.go`; `background_jobs_test.go`, `job_registry_test.go`, `job_supervisor_test.go`, `job_scope_claim_test.go`, `job_isolation_test.go`, `job_owner_cancellation_test.go`, `job_failure_test.go`, and `private_scope_cat_teardown_test.go`. | Complete typed units launch without waiting; one session-wide budget retains at most 64 unconsumed records across root/private scopes while IDs, claims, and visibility stay owner-local. `jobs` is observational; waits consume exact records only after success. Full terminal job control remains deferred. |
| Traps/signals | **Complete for P0.4** | `internal/shell/runtime/interrupt.go`, `trap_builtin.go`, `builtins.go`, `script.go`, `interactive.go`; `cmd/nemosh/signal.go`, `main.go`, `session.go`; `trap_builtin_test.go` plus focused interrupt, direct-applet, external-helper, and Windows acceptance tests. | Exactly shell-level EXIT and INT. A `trap` operand naming a real signal this shell cannot deliver, such as TERM, says so rather than calling the name invalid; one that is not a signal at all keeps bash's `invalid signal specification`. TERM delivery, targeted POSIX child SIGINT, process groups, Job Objects, ConPTY, and idle-input handling are not promised. |
| Errors and diagnostics | **Partial** | `internal/shell/runtime/script.go`, `external.go`, `runtime.go` (`AppletFailure`), `internal/applets/diagnostic.go`; focused runtime tests plus `cmd/nemosh/direct_applet_diagnostic_test.go` | An applet's failure is reported identically whether it ran inside the shell or was invoked directly, through one shared mapping; direct dispatch used to drop the applet-name prefix and print nothing at all for a failure carrying its own status. A capability that is absent now fails loudly rather than answering false — the rule the 2026-08-07 corrections followed. **Absent:** launch failures still collapse to “not found” with no hint, and the layered first-line/hint/debug contract and `NEMOSH_DEBUG=path,exec,fd` have not started. |

### Applets

`internal/applets/registry.go` registers all initial names listed in the v0 scope:
`[`, `test`, `true`, `false`, `echo`, `printf`, `pwd`, `env`, `printenv`,
`cat`, `head`, `tail`, `wc`, `basename`, `dirname`, `ls`, `mkdir`, `rmdir`,
`rm`, `cp`, `mv`, `touch`, `chmod`, `grep`, `sed`, `find`, and `xargs`.
Implementations are distributed across `internal/applets/core.go`, `files.go`,
`fs.go`, `fs_more.go`, `more_core.go`, `head_tail.go`, `wc.go`, `env.go`,
`chmod.go`, `grep.go`, `sed.go`, `find.go`, and `xargs.go`.

That is **complete by registered name only** and **partial for release readiness**.
Every initial v0 candidate now carries at least one checked-in smoke case and one
checked-in negative case under `tests/behavior/applets/`, laid out to mirror the
reference tree — `coreutils/`, `findutils/` (`find`, `grep`, `xargs`), and
`editors/` (`sed`).
`TestBehaviorAppletScriptCases_executeAgainstFreshNemosh` in
`internal/testutil/behavior/case_test.go` executes the `script` cases against a
freshly built product binary, so the expectations below were measured, not assumed.

That satisfies the smoke-plus-negative rule in
`docs/testing/applet-test-inventory.md` and nothing more. Two lines carried in the
`script` cases:

- `pwd` and `pwd -`-style operands are resolved by the shell's `pwd` builtin
  (`internal/shell/runtime/runtime.go:171`), not by the applet, so the two `pwd`
  fixtures pin the builtin; the applet is covered by
  `internal/applets/more_core_test.go`.
- The per-applet option matrices in `docs/testing/applet-test-inventory.md`
  (`ls -l`, `head -c`, `grep -r`, `find -name`, `xargs -0`, …) are still
  uncovered. Two cases per applet is the floor, not the target, and it does not
  make the applet corpus, the differential runner, or product CI complete.

#### Applet failure statuses and diagnostic shapes

Applet diagnostics name the operand as the user wrote it, never the resolved host
path; `internal/applets/diagnostic.go` carries the three BusyBox shapes and cites
the reference line for each. Failure statuses follow the reference rather than one
house rule: `sort` exits 2 because `sort_main` sets `xfunc_error_retval = 2`
(`coreutils/sort.c:468`), while `cut` and `uniq` never touch it and so exit 1 on
the `libbb/default_error_retval.c:16` default. `cut` also holds its status across
an unreadable operand and continues to the next, where `sort` aborts on the first
by design -- the comment saying so is at `coreutils/sort.c:568-570`, above the
`xfopen_stdin` at 571.

An applet names a non-default status by returning `applets.ExitStatusMessage`,
which carries the status *and* the diagnostic; the dispatch seam
(`AppletFailure`, `internal/shell/runtime/runtime.go:214`) prints the message
under the applet name and then returns the status. `cmd/nemosh/main.go` calls
the same function, so a direct `nemosh cat missing` fails exactly the way
`cat missing` inside the shell does; it used to print nothing at all. A bare `applets.ExitStatus` carries no
diagnostic and so stays silent. Before that seam existed an applet
got either a shell-printed message or a chosen status, never both, so anything
that did not print for itself was pinned to 1. Five applets take a status other
than the default today, counting `sed` below: `grep` exits 2 on any error so that 1 stays reserved for
`no match` (`findutils/grep.c:718-719`), `env` and `xargs` exit 127 for a command
that is not found (`libbb/executable.c:117-122`, `findutils/xargs.c:385-390`), and
`[` exits 2 on a missing `]` (`coreutils/test.c:897-901`) as `test` does on a
syntax error, and `sed` exits 1 when an operand cannot be read but keeps going
to the next (`editors/sed.c:1061-1063`).

Known divergences that are **deliberate and unfixed**, so a reader does not mistake
the corpus for full parity:

- Nemosh has no usage text. Where BusyBox reaches `bb_show_usage`, Nemosh prints a
  one-line diagnostic instead. The status matches; the output does not.
- `cut`'s range diagnostics differ beyond wording. BusyBox routes `-c -`, `-c ''`
  and `-c 1,,2` to usage output and `-c a` to `invalid number`, and spells a bad
  range as a pair (`invalid range 0-0`, `coreutils/cut.c:386`). Nemosh reports all
  of them as `invalid range <part as written>`.
- `sort -z` (NUL-terminated lines) is accepted by BusyBox and rejected by Nemosh.
- `uniq OUTFILE` (`coreutils/uniq.c:78`) is unimplemented; Nemosh rejects a second
  operand that BusyBox would write to.
- `cp` of a directory copies nothing rather than saying `omitting directory`, and
  `rm` removes an empty directory that BusyBox would refuse without `-r`.
- `cat` on a directory says `Is a directory`. BusyBox-w32 is self-inconsistent
  here: `mingw_fopen` converts `EACCES` to `EISDIR` (`win32/mingw.c:313`) so its
  `head`/`wc` agree, but `mingw_open` skips the conversion for `O_RDONLY`
  (`win32/mingw.c:265`) so its `cat` says `Permission denied`. Nemosh uses one
  wording for every reader.
- `env` and `xargs` dispatch registered applets only. BusyBox falls back to
  `execvp`, so `env python3 …` works there and reports `not found` here. Two
  consequences: the wording stays `not found` rather than BusyBox's `cannot execute
  'NAME': No such file or directory`, because no `execvp` ran and claiming `ENOENT`
  would misdescribe the mechanism; and the 126 half of the SUSv3 table — a command
  found but not runnable — is unreachable, so only the 127 half is implemented.
- `test`'s `-O` and `-G` ask whether the effective user owns a file. busybox-w32
  answers them from a stat that reports one fixed owner for everything
  (`win32/mingw.c:749`), so on Windows the question degrades to "does it exist".
  Nemosh gives that same answer on every platform rather than one that means
  something different on each. `-x` follows the executable-suffix rule
  busybox-w32 synthesises a mode bit from (`win32/mingw.c:780-784`).
- `sed` implements `s///` with the `g` and numeric flags and nothing else: no
  `-n`, no `-e`, no addresses, no commands beyond substitution. An unknown flag
  is named rather than swallowed.
- `printf` does not implement the `%a` conversion or a `*` width taken from an
  operand.

Six entries that used to sit in this list were defects rather than divergences,
and were fixed on 2026-08-07 rather than documented:

- `test` and `[` were a string-only stub — one- and two-argument string tests
  plus `=` and `!=` — and everything else evaluated **false**, so `test -f x` and
  `test 1 -lt 2` reported failure while looking implemented. They walk the whole
  POSIX 2.14 grammar now.
- `sed` read stdin only, so `sed 's/a/b/' f.txt` exited 1 with no diagnostic and
  the operand was neither used nor refused.
- `printf` forwarded its format and operands to Go's `Fprintf`, so
  `printf '%d\n' 42` printed `%!d(string=42)` with status 0.
- `echo` read no options, so `echo -n abc` printed `-n abc`.
- `mkdir` took `-p` as an operand and created a directory named `-p`; `rmdir`
  called `os.Remove`, so `rmdir notes.txt` deleted the file and exited 0.
- Missing operands failed silently in ten applets, and unknown options were
  swallowed: `wc -z FILE` selected no counts and exited 0, `touch -z` created a
  file called `-z`, `basename -z /a/b` printed `-z`. `chmod`'s bad-mode wording
  also moved to busybox's message-first, quoted form.

### Behavior corpus and CI

| Capability | Status | Evidence | Gap |
| --- | --- | --- | --- |
| Behavior case format/parser | **Complete for the v0 corpus** | `docs/testing/behavior-test-format.md`, `internal/testutil/behavior/case.go`, `parse.go`, `runner.go`, `runner_test.go`, `case_test.go` | Checked-in tests discover and validate every behavior TOML and execute both shell cases and applet script cases against a fresh Nemosh binary. The `[differential]` table records where a reference is expected to disagree, and a missing `why` is a validation error. |
| Golden corpus | **Partial** | `tests/behavior/applets/` and `tests/behavior/shell/`; shell cases and applet script cases run against a freshly built `nemosh -c` product binary | Every v0 applet has a smoke and a negative case; 88 applet cases and 51 shell cases are checked in, so several applets carry more than two. The per-applet option matrices in `docs/testing/applet-test-inventory.md` are still uncovered. Applet cases that declare `command` rather than `script` still run in-process against the registry, so full-corpus product execution remains incomplete. |
| Differential runner | **Complete for the golden shell corpus** | `internal/testutil/behavior/differential.go`, `differential_test.go` | Each golden shell case runs against the reference shells it names -- busybox `sh` and `ash`, dash, bash and `bash --posix` -- through the same sandbox the Nemosh executor uses, with the environment replaced wholesale so neither side inherits what the other does not. Status and stdout are compared; stderr is not, because diagnostic wording is where shells differ most and are least required to agree. A missing reference is a skip, and a reference may name the only platform it exists on -- `busybox-w32` means the Windows port, and a `busybox` on a Linux PATH is a different program that answers a Windows question wrongly. `NEMOSH_DIFFERENTIAL=strict` makes an *undeclared* divergence a failure, which is what CI runs. A deliberate divergence is declared per case in a `[differential]` table with a required reason. A declared divergence that stops happening is reported and fails only under `NEMOSH_DIFFERENTIAL=audit`: whether a reference disagrees depends on which build of it is installed, and CI proved that -- Ubuntu's dash agrees with Nemosh on two cases where the dash shipped with Git for Windows does not, so gating on it across platforms would mean the declarations could only ever be right for one machine. 159 case/reference pairs compared on Windows, 157 on Linux. | Applet cases are not compared, because busybox's applets are the reference implementation rather than a peer. |
| Product CI | **Complete** | `.github/workflows/product.yml` | Builds and tests on windows-latest and ubuntu-latest, installs busybox and dash so the differential runner has references, and holds gofmt, `go vet` on windows/linux/darwin, `go test -race -shuffle=on`, the corpus against a freshly built binary, the differential runner in strict mode, applet manifest freshness, and the 250-pure-LOC ceiling. The research probe workflow next to it still exercises reference shells only and is not evidence about Nemosh. Observed green on both runners (run 31160907517, 2026-08-07): every step succeeded on windows-latest and ubuntu-latest. | Getting there took five rounds, and each round found something no local run could have. gofmt failed on all 394 Go files because git's default `core.autocrlf` on a Windows runner converts on checkout and `.gitattributes` only covered `*.sh`. The runner's TEMP sits under an 8.3 alias, `C:\Users\RUNNER~1\...`, which exposed both an over-reach in the new case resolution -- it adopted a *different* name rather than a differently-cased one -- and twelve older tests, already failing on master, that compared realpath's answer to the raw `t.TempDir()` string. Then Linux, where this suite had never run: `rmdir` on a non-empty directory said "File exists" because only the Windows side overrode ENOTEMPTY, an applet-override test assumed `/bin/cat` does not exist, one of the three corpus executors ignored a case's platform list, and the differential runner resolved `busybox-w32` to a Linux busybox and then asked it a Windows question. Three interactive interrupt tests skip on Linux with a reason: that path is Windows-shaped by design and judging it needs a Linux machine to measure on. |

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
`bg_53e10134` PASS, and context `bg_84b6d198` PASS. That closed P0.4 only.

**Correction (2026-08-07).** This section used to end by saying P0.5 had closed
as well and P1.1 was next. That was wrong on both counts: P0.5's Wave B had a
batch-quoting defect and its Wave A had a script-operand one, and P1.1's own
subject -- expansion -- had four gaps of its own. All of them have since been
fixed and the waves rewritten below.

### P0.5 - Windows boundaries

Integrate `internal/pathmodel` across shell-owned filesystem and lookup
operations; implement device clipboard, `ComSpec` batch launch, `.sh`/shebang
dispatch, UTF-8/wide boundaries, and long-path handling.

**Pass criteria:** native Windows tests cover all required input path forms,
drive/UNC current roots, configurable aliases and virtual roots, host-only UNC
hints, no general argv conversion, `/dev/clipboard` UTF-8 text I/O, fixed suffix
order, batch quoting/status, shebang/CRLF scripts, non-ASCII argv/env/path, and
long filesystem/process paths.

**Complete.** All three sub-waves landed, and the breadth the pass criteria ask
for is now on native Windows: UNC current roots and the drive-versus-share
question `pwd` answers (`path_state_unc_windows_test.go`), the cygdrive,
mount-prefix, `/tmp`, and `/dev` switches (`path_settings_windows_test.go`), the
host-only UNC hint across `cd`, redirection, and `.`, and argv non-conversion,
which had only ever been recorded as an absence (`external_argv_test.go`). The
launch, encoding, and long-path halves are covered by the rows above.

Checking "all required input path forms" against a fresh binary rather than
against the test suite found one of them broken: the lexer treated backslash as
an unconditional escape inside double quotes, so `cat "C:\Users\nemo\file.txt"`
— the quoted form `windows-path-model.md:32` offers as the alternative to
forward slashes — reached the path model as `C:Usersnemofile.txt`. The path
model was never at fault and no path test could have caught it. Fixed to the
POSIX and busybox-w32 rule, where the backslash stays special only before `$`,
a backquote, a double quote, another backslash, and a newline
(`lexer_double_quote_test.go`,
`tests/behavior/shell/posix/double-quote-backslash.toml`).

The Windows Path Gates row stays **Partial**, but no longer because of P0.5: what
is missing there is real-case resolution, a capability `windows-path-model.md`
asks for and nothing implements. It is not on this section's pass criteria.

P0.5 is ordered as three sub-waves so path integration lands before launch and
encoding boundaries depend on it:

| Sub-wave | Content | Status |
| --- | --- | --- |
| Wave A | Route shell-owned filesystem and lookup operations through `internal/pathmodel`. | **Complete.** `Runtime.ResolveNemoshPath` is the single seam. One operand had been missing from it: a script named on the command line went straight to `os.ReadFile`, so `nemosh /c/dir/build.sh` failed with 127 while `pwd` in that same shell printed exactly that spelling (`cmd/nemosh/script_file_path_test.go`). |
| Wave B | `/dev/clipboard`, fixed suffix lookup, `ComSpec` batch launch, `.sh`/shebang/CRLF dispatch, applet override configuration. | **Complete.** Suffix lookup, batch launch through `ComSpec`, `.sh`/shebang dispatch, CRLF handling down to a preserved lone `\r`, `/dev/clipboard`, and `NEMOSH_OVERRIDE_APPLETS` all landed. The batch half was wrong until 2026-08-07: every operand was quoted on the way to cmd, so `%1` arrived as `"release"` and `if "%1"=="release"` never matched. The existing coverage all read `%~1`, whose whole job is to strip quotes, so it could not have caught this. Quoting is conditional now, matching cmd.exe and busybox's `quote_arg` (`win32/process.c:123-128`), and the new cases read `%1`. |
| Wave C | UTF-8/wide API boundaries and internal long-path handling. | **Complete.** One genuine gap — a child could not be launched from a working directory over 258 UTF-16 characters — carried by `external_directory.go`. A later audit reported the *image* path as a second gap; re-measured, an 804-character image path launches, because Go's `os/exec` reaches CreateProcess with a wide path and there is nothing left for Nemosh to widen. What the audit was right about was coverage: neither long-path test launched anything at all. Two now do (`long_path_windows_test.go`). |

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

**Expansion half: complete (2026-08-07).** Field splitting, pathname expansion,
the parameter operators, arithmetic, and backquote substitution all landed, each
with a `tests/behavior/shell/posix/` case: `field-splitting.toml`,
`pathname-expansion.toml`, `parameter-operators.toml`,
`arithmetic-expansion.toml`, `backquote-substitution.toml`. What each of them
did before is worth recording, because none of it was visible to a green suite:
an unquoted expansion was never split, so `for f in $list` looped once; nothing
globbed, so `ls *.txt` handed ls the pattern; every parameter operator outside
`-` and `:-` expanded to its own literal text; and `$(( ))` and backquotes were
syntax errors.

**Diagnostics half: not started.** The layered first-line/hint/debug contract and
`NEMOSH_DEBUG=path,exec,fd` are absent. Launch failures still collapse to
"not found" without a hint, and there is no opt-in detail channel.

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

## Known gaps in the shell language, measured 2026-08-18

Found by running a differential sweep against bash over about 140 constructs rather
than by reading the code, which is why they had gone unnoticed: each is a form
nothing in the corpus exercised. What follows is what the sweep found and did *not*
get fixed, with the workaround where there is one, so the next reader starts from a
list rather than from a surprise.

Everything else the sweep turned up was fixed in the same pass: `read` taking no
options at all, `x=$(cmd)` field splitting its value, `${VAR:-${DEFAULT}}`,
`$(...)` inside an expansion or arithmetic, `${@:2:2}` and the other list
operators, sparse arrays, negative subscripts, `$$`, `${#1}`, `**`, `,`,
`base#digits`, `$'...'`, `<<<`, `&>`, `((expr))`, `for ((;;))`, `for name`,
`declare`/`typeset` with associative arrays, `shopt` with globstar, the `function`
keyword, `;;&` and `;&`, `case (pattern)`, a redirection after a compound's
closer, the extended pattern operators, `BASH_REMATCH`, and `<(cmd)`.

Three things a bare `(` inside `$( )` broke, all found while adding `<(cmd)` and
all fixed with it: a subshell (`$( (cd x && pwd) )`), an extended pattern group,
and a case arm's pattern. The substitution scanner counted only the parentheses
of `$(`, so the first `)` belonging to anything else ended the body early.

| gap | workaround | why it is not done |
| --- | --- | --- |
| `true \| case a in a) x ;; esac` -- piping into a *case* | pipe into a `while`, `if`, `for` or `until`, all of which work; or `{ case ...; esac; }` | The case-arm line pass finds a `case` only at the start of a line. The other four compounds are done |
| a *multi-arm* case inside a brace group | a case at the top level takes any number of arms; a single-arm one works inside a group | The second pattern's `)` meets a fourth scan with its own opinion about brackets. Three of the four now ask whether a `case` is open |
| `>(cmd)` -- the *output* half of process substitution | `cmd1 > file; cmd2 < file`, or a pipe | Refused by name rather than approximated. `<(cmd)` works, as a real temporary file rather than `/dev/fd/63`, which Windows has no equivalent of; the input form's consumer reads a file the command has already finished writing, and that trade does not carry over to writing into one. See process_substitution.go |
| `$LINENO` | none | No AST node carries a position. A `$LINENO` that is always 1 would send someone to the wrong line with confidence, which is worse than its being unset |
| `$!` -- the last background process id | `wait` with no argument | Background jobs here are goroutines, not processes, so there is no pid to report. A job number would be a different thing wearing the same name |
| an *unquoted* group in a `[[ =~ ]]` regex | quote it: `[[ x =~ "(b)" ]]`, which works here though bash reads a quoted regex as a literal | Same cause as the row above: the parenthesis is read as a bracket before the condition is parsed. The captures themselves are kept now, in BASH_REMATCH |
| `${x:-"}"}` keeps its quotes | none | Quote removal inside an operand word. The `}` inside quotes correctly does not end the expansion, which is the half that matters |
| `${a[-9]}` gives empty where bash errors | none needed | Deliberate: it is the same answer `${a[9]}` gives, and being consistent about "not an element" matters more here than matching bash's choice to distinguish the two |
| two heredocs on one line -- `cat <<A; cat <<B` | put them on separate lines | The delimiter scan collects one per line |
| `select name in ...; do ... done` | a `while` loop with `read` and a `case` | Not implemented. It is an interactive menu construct, and the loop it expands to is three lines someone can write |
| `coproc` | a named pipe, or two redirections | Not implemented. It needs a bidirectional child, which on Windows means deciding on pipes before deciding on this |
| `trap -l` | `kill -l`, which works | The signal list is only wired to kill |
| `du` reports *apparent* size in 1 KiB blocks, not disk usage | `wc -c` or `stat -c %s` for apparent size, which is what this already gives | The name means disk usage, and disk usage is the allocated size: busybox-w32 says `4` for a ten-byte file inside one NTFS cluster where this says `1`. Windows reports allocation through `FILE_STANDARD_INFORMATION` or `GetCompressedFileSize`, neither of which `os.FileInfo` exposes, so this needs the same Windows-API decision as the `ls -l` row below. The shape is right and the number is optimistic |
| `ls -l` prints mode, size and name, with no timestamp, owner, group or link count | `stat` for one file's detail | The timestamp is the column people read and is cheap and portable, so it is the part worth adding. Owner, group and link count are not: Go exposes none of them on Windows, and busybox-w32 answers `root`/`nemo` per file by mapping the real owner SID. Printing the current user for every file would be a fabricated answer in a column scripts trust, which is worse than a column that is not there. Found while porting uutils `test_ls.rs`, whose Windows branch pins the mode string as `[-dl](r[w-]x){3}` -- a third answer again, since busybox says `-rw-rw-r--` and Go's FileMode says `-rw-rw-rw-` |

The three at the bottom of that table were found after it was first written, by asking
directly rather than through the sweep -- which is the honest note to leave: the sweep
covered about 140 constructs and is a net, not a proof.
