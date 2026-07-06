# V0 Scope

This document turns the current Nemosh design decisions into an implementation
scope. It is still allowed to change, but code should not start by ignoring these
boundaries.

## Product Target

Nemosh v0 is a native Windows-first, cross-platform, BusyBox-style shell and
utility bundle. The shell target is advanced ash-like behavior, not MSYS2,
Cygwin, PowerShell, or cmd compatibility.

Primary references:

- busybox-w32 for native Windows behavior and tradeoffs.
- BusyBox ash for shell/runtime structure.
- dash and POSIX for portable sh behavior.

Non-target references:

- MSYS2, Cygwin, and WSL1 are research references for edge cases, not
  compatibility targets.

## V0 Must Have

- Non-interactive script runner.
- Own shell parser, with `mvdan/sh/syntax`, BusyBox ash, dash, and bash used as
  references and test oracles.
- POSIX-style shell variables, expansions, redirection, pipelines, command
  substitution, functions, and control flow for the selected grammar slice.
- Shell-owned fd table for redirection, pipelines, child inheritance, and future
  `/dev/fd/N`.
- Explicit state snapshots for subshells, command substitution, and pipeline
  execution contexts.
- BusyBox-style bundled applet framework.
- Windows path model from `windows-path-model.md`.
- Windows execution model from `windows-execution-model.md`.
- Differential/golden behavior tests against local references.

## Windows Path Gates

- Display style defaults to `posix-drive`, e.g. `/c/Users/nemo`.
- Accept `/c/foo`, `/mnt/c/foo`, `C:/foo`, quoted/backslash Windows paths at input
  boundaries, and `//host/share/foo`.
- `/mnt/c` is a configurable syntactic alias, enabled by default. It is not a
  WSL mount namespace.
- `/cygdrive/c` is configurable and default-off.
- `/` means current root, matching busybox-w32. The current root can be a drive or
  UNC share.
- Virtual roots such as `/tmp` and `/dev` are enabled by default and resolved
  before current-root expansion. They must be configurable.
- Do not implement a user-owned mount table in v0.
- Host-level network browsing uses explicit future applets such as `shares` and
  `nmount`; `cd //host` remains invalid with a targeted hint.
- No general argv path auto-conversion for external native programs.
- Path conversion applies only to shell-owned operations, bundled applets,
  executable lookup, `argv[0]` launch details, and explicit helpers.
- Backslash remains POSIX shell escape syntax when unquoted.

## V0 Device Paths

Required:

- `/dev/null`
- `/dev/stdin`
- `/dev/stdout`
- `/dev/stderr`
- `/dev/zero`
- `/dev/urandom`
- `/dev/random`, as a non-blocking CSPRNG alias for `/dev/urandom`
- `/dev/clipboard`, text-only UTF-8 at the shell/applet boundary

Conditional:

- `/dev/fd/N` after the fd table is stable.

Deferred:

- `/dev/tty`
- `/dev/ptmx`
- `/dev/pts/*`
- `/proc`
- `/dev/tcp/*`

## Execution Gates

- Command lookup defaults to functions/special builtins/builtins, then bundled
  applets, then `PATH` external programs.
- Applet overrides must be configurable, similar in spirit to busybox-w32
  `BB_OVERRIDE_APPLETS`.
- Windows suffix order is fixed by default: `.com`, `.exe`, `.sh`, `.bat`, `.cmd`.
  Do not use arbitrary `PATHEXT` by default.
- `.bat` and `.cmd` are external Windows commands through an explicit
  `cmd.exe`/`ComSpec` boundary.
- `.sh` and shebang scripts follow BusyBox-style interpreter handling.
- Windows `PATH` uses semicolon-separated syntax.
- Environment variables are case-sensitive inside the shell. Windows child
  environment blocks are deduplicated case-insensitively at spawn boundaries.
- Empty environment values are preserved.
- The shell uses UTF-8 internally and Windows wide APIs at platform boundaries.
- Long paths are handled internally at Windows API boundaries.

## Job, Signal, And Error Gates

- Support `cmd &`, `wait`, and basic `jobs` for shell-launched background jobs.
- Defer full `fg`/`bg`, stopped jobs, POSIX process groups, and ConPTY job control.
- Support `trap ... EXIT`.
- Map console Ctrl-C to shell-level `INT` where possible.
- Be honest about unsupported signal semantics on Windows.
- Error diagnostics use layered output: POSIX-style first line, optional hint,
  debug details behind flags such as `NEMOSH_DEBUG=path,exec,fd`.

## Parser And Shell Semantic Gates

V0 should include enough grammar and runtime for real scripts:

- Simple commands and assignments.
- Quoting and escaping with POSIX backslash semantics.
- Parameter expansion subset needed by shell scripts.
- Command substitution.
- Arithmetic expansion if needed for test/app scripts.
- Redirections, here-docs, and pipelines.
- `if`, `case`, `for`, `while`, `until`, grouping, functions, and subshells.
- Special builtins: `break`, `continue`, `.`, `eval`, `exec`, `exit`, `export`,
  `readonly`, `return`, `set`, `shift`, `trap`, `unset`.
- Stateful regular builtins: `cd`, `command`, `getopts`, `jobs`, `read`, `umask`,
  `wait`.

Exact grammar order should be set by the implementation milestone plan and tests.

## Applet Scope Direction

V0 applet scope should be broader than shell builtins. The initial cut should
favor coreutils and script-critical tools, then expand by test demand.

Initial candidates:

- `[` / `test`
- `true`, `false`
- `echo`, `printf`
- `pwd`, `env`, `printenv`
- `cat`, `head`, `tail`, `wc`
- `basename`, `dirname`
- `ls`, `mkdir`, `rmdir`, `rm`, `cp`, `mv`, `touch`
- `chmod` with documented Windows semantics
- `grep`, `sed`, `find`, `xargs` if the v0 test corpus needs them early

Native ACL-aware applets should be designed as Windows-truthful utilities rather
than pretending POSIX mode bits fully capture Windows permissions.

## Deferred Non-Goals

- Full REPL polish, completion, plugins, and zsh-level interaction.
- Full native Windows POSIX job control.
- MSYS2/Cygwin argv conversion.
- WSL-like mount namespace.
- Full POSIX certification claims.
- Full BusyBox applet parity in the first implementation cut.
