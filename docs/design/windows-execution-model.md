# Windows Execution Model

This document records initial Windows command lookup and process-launch decisions
for Nemosh. It is a design draft, not final implementation code.

## Goals

- Follow busybox-w32 behavior and implementation choices as the primary native
  Windows reference unless a subsystem explicitly chooses otherwise.
- Preserve Nemosh shell semantics while accepting necessary Windows execution
  boundaries.
- Avoid MSYS2-style argv rewriting for ordinary external program arguments.

## Command Lookup

Default lookup order should be BusyBox-style but configurable:

1. Shell functions and POSIX special builtins.
2. Regular shell builtins.
3. Bundled Nemosh applets.
4. External programs found through `PATH`.

Users must be able to override selected bundled applets with external programs,
similar in spirit to busybox-w32 `BB_OVERRIDE_APPLETS`. The final config syntax is
still open.

## Applet Dispatch And Distribution

Nemosh should ship as a BusyBox-style multi-call binary and support Scoop-first
Windows distribution. The busybox-w32 Scoop manifest downloads one `busybox.exe`,
creates a shim for `busybox`, runs `busybox --list`, and creates a shim for each
defined applet name. Nemosh should follow the same distribution shape unless a
future installer target requires hardlinks or symlinks.

Required invocation forms:

- `nemosh <applet> ...`
- applet-name shims such as `cat`, `ls`, and `grep` pointing to `nemosh.exe`
- shell standalone lookup preferring bundled applets unless overridden

## Windows Suffix Search

When a command name has no explicit executable suffix, Nemosh should follow the
busybox-w32 suffix order rather than arbitrary `PATHEXT` order:

```text
.com
.exe
.sh
.bat
.cmd
```

This makes command lookup deterministic across Windows machines while preserving
the main busybox-w32 behavior.

The suffix list alone is not the whole rule. busybox `add_win32_extension`
(`win32/mingw.c:2237`) tries the **bare name first** and appends suffixes only
when the name has neither an executable suffix nor a trailing dot — the dot
matters because Windows drops a trailing one when opening, so appending to it
would name a different file. A name like `notes.txt` is therefore still eligible
for `notes.txt.exe`, while `run.` is not.

Since Windows has no execute bit, a bare name is accepted when it carries an
executable suffix or when its first bytes say it is runnable (`win32/mingw.c:779`
with `has_exec_format` at `win32/mingw.c:487`). Nemosh implements the pragmatic
subset of that sniff — at least four bytes, then `#!` or `MZ`, with `.dll`
excluded by name — and deliberately does not walk the PE header. This is what
makes an extensionless shebang script findable at all.

As implemented: `internal/shell/runtime/external.go` and `external_format.go`.

## Batch Files

`.bat` and `.cmd` files are supported as external Windows commands by default.
They are not Nemosh scripts.

Execution should cross an explicit `cmd.exe`/`ComSpec` boundary, so batch file
syntax, variable expansion, quoting, and control operators are documented as cmd
semantics, not Nemosh semantics.

The boundary is spelled:

```text
"<ComSpec>" /d /s /c "<script> <arg> <arg>…"
```

`/d` suppresses AutoRun, and `/s` makes cmd strip exactly the outer quote pair
and take the remainder verbatim. Each of the script path and the arguments is
wrapped in `"` with any embedded `"` doubled, which is cmd's own convention.
`ComSpec` comes from the runtime environment table, falling back to
`%SystemRoot%\System32\cmd.exe`.

The whole line is handed to Windows through `syscall.SysProcAttr.CmdLine` rather
than through `exec.Cmd.Args`, because Go's `syscall.EscapeArg` emits `\"` and cmd
does not understand that escape. Go's own documentation names this case
(`os/exec`: "Notable exceptions are msiexec.exe and cmd.exe (and thus, all batch
files), which have a different unquoting algorithm").

Note that this boundary is not what makes a batch file *launch* — `CreateProcess`
already routes `.bat`/`.cmd` through the command processor implicitly, and that
implicit path reports exit status correctly. What it fixes is argument fidelity:
measured against a batch that reports `%~1` and `%*`, the implicit path lets an
argument containing `&` split the command line (the injection shape of
CVE-2024-24576), leaks Go's `\"` into `%~1` verbatim, and leaves AutoRun in play.

busybox-w32 cannot be copied here: it never reads `%COMSPEC%` for launch, relying
instead on MSVCRT `spawnve`, which Go has no equivalent for.

Two operand shapes cannot be carried across this boundary at all and are refused
with a diagnostic and status 126, before anything runs:

- a line break, `\r` or `\n`, which cmd has no way to represent in one command;
- two or more `%`, because a `%NAME%` pair naming a *defined* variable expands
  and then splits across argv. A lone `%` is provably literal and passes through;
  whether a name is defined is not knowable from the operand alone, so the second
  `%` is the deterministic tripwire.

There is no behavior-corpus fixture for batch: the corpus executor replaces the
child environment wholesale (`internal/testutil/behavior/sandbox.go:42-47`), so a
fixture would have to hardcode `COMSPEC` for one machine. Coverage is in
`internal/shell/runtime/external_batch_windows_test.go` instead.

As implemented: `internal/shell/runtime/external_batch.go` and
`external_launch_windows.go`.

Nemosh may adjust the executable path / `argv[0]` at the Windows process-launch
boundary when required by Windows APIs. It must not perform general path
auto-conversion on ordinary argv elements.

## Shell Scripts

`.sh` files and shebang scripts are handled separately from batch files. A `.sh`
file executes through Nemosh, and a shebang script through the interpreter it
names.

Shebang handling should follow busybox-w32's pragmatic behavior: parse `#!`, map
Unix-style interpreter names such as `/bin/sh` through Nemosh's applet/interpreter
lookup where appropriate, and run `.sh` files without shebang through Nemosh by
default.

The grammar is `parse_interpreter` (`win32/process.c:66`), reproduced exactly:

- The first 99 bytes are the whole window (busybox's `interp->buf[100]` less the
  NUL). A first line that does not end inside it is not a shebang.
- At least four bytes, then a literal `#!`, then a `\n` within the window.
- The interpreter is the first `" \t\r\n"`-delimited token, and its basename must
  be non-empty.
- At most **one** option follows, taken as a single argument up to `\r` or the
  newline and trimmed; `#!/bin/sh -x -y` passes `-x -y` as one word, as on Linux.
- A `.sh` file (case-insensitive) with no usable shebang falls back to `/bin/sh`.

Mapping is `mingw_spawn_interpreter` (`win32/process.c:301`) with `unix_path`
(`win32/mingw.c:2569`). An interpreter whose dirname is `/bin`, `/usr/bin`,
`/sbin`, or `/usr/sbin` names the Unix world rather than the filesystem, so it is
answered without touching disk:

- `sh` or `nemosh` re-enters this binary against the script. POSIX says a script
  run as a command executes in a new shell, and since Windows has no `fork`, that
  is a child process — which is why ordinary `nemosh script.sh [args]` dispatch is
  a prerequisite for this whole section.
- Any other registered applet name is handed to this binary as that applet.

Anything else resolves through ordinary lookup: the path as written first, then,
for a Unix path, a `PATH` search by basename. argv is rebuilt busybox-style as
`[option?, absolute script path, arguments…]` with the caller's `argv[0]` dropped,
and a chain of interpreters gives up at the fifth (busybox's `++level > 4` ELOOP
guard), reported as status 126. An interpreter that cannot be found is 127.

Known limitation: `#!/usr/bin/env python3` resolves to the `env` applet, because
`/usr/bin` is a Unix path and `env` is registered. Nemosh's `env` runs applets
only (`internal/applets/env.go`), so that shebang reports `python3: not found`.
This is busybox-faithful in its dispatch and simply inherits `env`'s current
scope; widening `env` to external programs is separate work.

As implemented: `internal/shell/runtime/external_script.go`, with end-to-end
coverage in `tests/behavior/shell/windows/script-sh-dispatch.toml` and
`shebang-applet-interpreter.toml`.

Shell script input should follow busybox-w32's tolerant CRLF behavior:

- Accept both LF and CRLF in script files and parser input.
- Normalize CRLF pairs to LF before shell grammar processing.
- Remove `\r` only when it is part of a `\r\n` pair; preserve lone `\r` as data
  unless a more specific rule applies. Normalization runs exactly once, at the
  entry to parsing (`normalizeLineEndings`, `internal/shell/runtime/syntax_scan.go`),
  because `strings.ReplaceAll` is not idempotent over a run of carriage returns:
  `one\r\r\n` collapses to `one\r\n` on a first pass and loses the surviving `\r`
  on a second. Trimming a logical line uses the cutset `" \t\n"` rather than
  `strings.TrimSpace`, whose cutset would eat a leading or trailing lone `\r`.
  A trailing lone `\r` does end line-continuation detection in
  `syntax_continuation.go`, and that is correct — bash treats `\r` as an ordinary
  word character, so `echo a &&\r` is not a continued line there either.
- Do not enable global Windows text mode for all file and applet I/O. Applets
  remain byte-oriented by default and implement text behavior individually.

## Environment Variables

Nemosh should keep POSIX shell variable semantics inside the shell and apply
Windows-specific normalization only at import/export boundaries.

Shell variable namespace:

- Case-sensitive.
- `foo`, `FOO`, and `Foo` are distinct shell variables.
- `export` marks the exact shell variable name; it does not merge names that
  differ only by case.

Initial Windows environment import:

- Follow busybox-w32's spirit by canonicalizing normal Windows environment names
  into shell-friendly spellings, especially mixed-case names such as `Path`,
  `ComSpec`, and `ProgramData`.
- Prefer canonical exported spellings for known Windows variables such as
  `PATH`, `COMSPEC`, `SYSTEMROOT`, `WINDIR`, `TEMP`, `TMP`, `USERPROFILE`,
  `APPDATA`, `LOCALAPPDATA`, `PROGRAMDATA`, and `PATHEXT`.
- `TMPDIR` may fall back to `TMP` then `TEMP`; `HOME` may fall back to the user's
  Windows profile/home mapping.

Child process environment construction:

- Build a Windows environment block with case-insensitive deduplication.
- Known canonical Windows names win over non-canonical spellings.
- For unknown names that collide only by case, use deterministic last
  assignment/export wins behavior.
- Do not warn by default for case collisions; reserve diagnostics for strict or
  debug modes.

Nemosh bundled applets should use the shell environment view when invoked
in-process. External native programs receive the Windows-deduplicated child
environment block.

`PATH` on Windows should use semicolon-separated Windows syntax at the shell
boundary. Nemosh command lookup must understand `;` as the Windows `PATH`
separator instead of trying to use POSIX `:`, which conflicts with drive paths
such as `C:/...`.

## Job Control And Signals

Native Windows v0 should follow the lightweight busybox-w32 direction. The
busybox-w32 ash source explicitly notes that job control does not work even
though the `jobs` builtin is available. Nemosh v0 should support asynchronous
commands and waiting without promising full POSIX terminal job control:

- Support `cmd &` for background process launch.
- Support `wait` for known child processes.
- Support a basic `jobs` view for shell-launched background jobs.
- Defer `fg`, `bg`, stopped jobs, foreground terminal process groups, and ConPTY
  integration to the interactive/PTY milestone.

Trap/signal support should be useful but honest on Windows:

- Implement `trap ... EXIT`.
- Map console Ctrl-C to shell-level `INT` for the current Nemosh foreground
  execution or `wait`, then create a fresh context for the next interactive entry.
- Go on Windows cannot direct `os.Interrupt` to one child or process group.
  Foreground external cancellation therefore uses `exec.CommandContext` process
  termination after the shell context is canceled; it is not targeted Ctrl-C.
- Windows `CTRL_BREAK_EVENT` can target an isolated process group. On the tested
  Windows/Go toolchain it reaches Go's `os.Interrupt` notification channel.
  Automated acceptance uses this only as a production `signal.Notify` boundary
  test; it is not a promise of POSIX SIGINT delivery to one external child.
- Ctrl-C while Nemosh is idle in prompt/input reading is not a P0.4 guarantee;
  active foreground execution and `wait` are the supported interruption points.
- P0.4 supports exactly `EXIT` and `INT` traps. It does not promise `TERM`, POSIX
  signal delivery, process groups, Job Objects, or ConPTY terminal job control.
- Shell close seals new root-job launches but does not implicitly wait for, cancel,
  or kill existing root jobs. Process exit still ends in-process job supervision,
  so post-process-exit survival is not promised.

## Fd Table And Shell State

Nemosh should own a POSIX-like fd table instead of relying on incidental Go or
Windows process-global handle behavior. The fd table maps shell fd numbers to
Windows handles, Go files, pipes, consoles, and virtual devices. It is the basis
for redirection, pipelines, `exec 3>file`, child handle inheritance, and
`/dev/fd/N`.

Because native Windows has no `fork`, subshells, command substitution, and
pipeline segments should run against explicit shell-state snapshots. Variables,
functions, options, cwd/root state, traps, and fd mappings must be copied or
derived deliberately so child execution does not leak state back to the parent
unless POSIX shell semantics require it.

## Pipeline Status

Default pipeline exit status should follow POSIX: the pipeline status is the
status of the last command. Provide `set -o pipefail` as an opt-in extension.

## Parser Strategy

The current chosen direction is to write a Nemosh parser rather than adopting
`mvdan/sh/syntax` as the implementation parser. `mvdan/sh/syntax` and ash remain
important references for grammar coverage, test cases, and AST/runtime boundary
design.

## Unicode And Long Paths

Nemosh should use UTF-8 as its shell, script, applet, and diagnostic encoding.
At the Windows API boundary it should call wide-character UTF-16 APIs and convert
to/from UTF-8. This avoids ANSI code page dependence while keeping shell scripts
portable and readable.

Nemosh can force UTF-8 for its own parser, applets, config files, diagnostics,
and internal path model. It cannot force arbitrary external native programs to
interpret argv, stdin, stdout, or files as UTF-8. For child processes, Nemosh may
provide a UTF-8-friendly environment and console setup, but external program
behavior is program-specific.

Long Windows paths should be handled internally. Users continue to write Nemosh
paths such as `/c/...`, native forward-slash paths such as `C:/...`, and UNC
paths such as `//host/share/...`. When Windows APIs require an extended-length or
NT-style prefix, Nemosh should add it at the platform boundary rather than
exposing `\\?\` syntax as the normal shell path form.

## Error Diagnostics

Default errors should be concise and shell-like, with optional hints when Nemosh
can identify a Windows-specific mistake. Detailed path/exec/fd diagnostics should
be gated behind debug flags.

Recommended layers:

- First line: POSIX-style terse error, suitable for scripts and tests.
- Optional hint line: actionable Nemosh/Windows hint for common mistakes, such as
  host-only UNC paths, unquoted backslashes, disabled virtual roots, or applet
  override behavior.
- Debug mode: verbose details controlled by a flag or environment variable such
  as `NEMOSH_DEBUG=path,exec,fd`, including Win32 error codes, resolved native
  paths, lookup candidates, and handle inheritance choices.

## Startup Configuration

Configuration should combine a Nemosh-native config with ash-style `ENV` support:

- Load system and user Nemosh config, including `~/.nemoshrc`, for Nemosh-specific
  options such as path aliases and virtual roots.
- In interactive shell mode, support POSIX/ash-style `ENV` expansion to select an
  rc file.
- Optionally try `.profile` for login/profile behavior, but do not claim `.bashrc`
  or `.zshrc` compatibility.

## Globbing And Permissions

Windows glob matching should be filesystem-aware: follow the directory's actual
case-sensitivity behavior where possible. Ordinary Windows directories are
case-insensitive; case-sensitive directories should be respected.

Permission support should expose native ACL-aware applets over time. Basic
BusyBox-style synthesized mode bits may still be needed for POSIX-facing applets,
but Windows ACL inspection/modification should be treated as the truthful native
backend rather than pretending that POSIX mode bits fully describe access.

## Correctness Tests

The v0 correctness strategy should use a differential behavior corpus backed by
golden tests. Compare POSIX-facing behavior against references such as
busybox-w32 ash, BusyBox ash, dash, and bash where applicable. Windows-specific
behavior must be marked explicitly in test metadata so native Windows semantics
are not mistaken for portable POSIX requirements.

## Applet Scope

The applet roadmap should start broader than shell builtins: include coreutils
and script-critical utilities early enough that Nemosh behaves like a real
BusyBox-style bundle. The exact v0 cut still needs a milestone list, but the
direction is broader coreutils coverage rather than shell-only.
