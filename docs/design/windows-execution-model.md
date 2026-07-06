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

## Batch Files

`.bat` and `.cmd` files are supported as external Windows commands by default.
They are not Nemosh scripts.

Execution should cross an explicit `cmd.exe`/`ComSpec` boundary, so batch file
syntax, variable expansion, quoting, and control operators are documented as cmd
semantics, not Nemosh semantics.

Nemosh may adjust the executable path / `argv[0]` at the Windows process-launch
boundary when required by Windows APIs. It must not perform general path
auto-conversion on ordinary argv elements.

## Shell Scripts

`.sh` files and shebang scripts should be handled separately from batch files.
The default `.sh` behavior should execute through Nemosh or the requested
interpreter after the parser/runtime strategy is finalized.

Shebang handling should follow busybox-w32's pragmatic behavior: parse `#!`, map
Unix-style interpreter names such as `/bin/sh` through Nemosh's applet/interpreter
lookup where appropriate, and run `.sh` files without shebang through Nemosh by
default.

Shell script input should follow busybox-w32's tolerant CRLF behavior:

- Accept both LF and CRLF in script files and parser input.
- Normalize CRLF pairs to LF before shell grammar processing.
- Remove `\r` only when it is part of a `\r\n` pair; preserve lone `\r` as data
  unless a more specific rule applies.
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
- Map console Ctrl-C to shell-level `INT` where possible.
- Support `TERM` for Nemosh-managed child termination paths where meaningful.
- Report unsupported signal names clearly rather than pretending POSIX signal
  semantics exist for all native Windows processes.

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
