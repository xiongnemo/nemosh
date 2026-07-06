# Research Decision Notes

This document records user-confirmed direction that should guide the next
research and design pass. It is still not a final implementation spec.

## Confirmed Direction

- Start with non-interactive script execution for v0 research and testing. REPL
  and interactive shell features remain required later and must not be forgotten.
- Native Windows support should target POSIX shell semantics, not normal
  PowerShell or cmd semantics. This is an intentional novelty goal, even though
  it is difficult.
- Nemosh's product target is an advanced BusyBox ash-like native shell and
  utility bundle, not an MSYS2/Cygwin-compatible runtime.
- Windows POSIX semantics should be studied against WSL1 behavior, Cygwin,
  MSYS2, busybox-w32, and BusyBox MinGW/native Windows references, but
  MSYS2/Cygwin remain non-target references only.
- Windows path semantics should use busybox-w32 as the primary native Windows
  reference, with `/c/foo` as Nemosh's short drive alias, `/mnt/c/foo` as a
  semantically distinct configurable mount-prefix alias, and UNC paths
  represented as `//host/share/path`.
- Final sample config and path documentation must include comments explaining
  each accepted path scheme separately; do not collapse `/c` and `/mnt/c` into a
  single vague alias setting.
- The parser strategy is not settled. `mvdan/sh` should be evaluated seriously,
  but not adopted blindly before checking whether its parser/interpreter model
  can support the required Windows POSIX runtime semantics.
- Public compatibility language can be conservative. Use terms such as
  POSIX-oriented or POSIX sh compatibility target until official certification is
  actually pursued.

## Design Implications

- v0 should prioritize a script-first execution core, golden tests, and
  differential behavior tests before investing in line editing, completion,
  history, prompts, plugins, or zsh-like interactive features.
- REPL must be tracked as a later milestone because zsh-level capability is a
  long-term goal, not a discarded requirement.
- Windows path, process, fd, signal, pty, symlink, newline, executable lookup,
  and permission behavior are core design topics, not edge cases.
- POSIX utilities are in scope for the v0 product direction as a BusyBox-style
  bundle. The runtime design should account for both shell execution and
  bundled applets instead of assuming an external MSYS2/Cygwin userland.

## Open Follow-Up

- Decide whether `mvdan/sh` is parser-only, parser-plus-AST, or unsuitable after
  evaluating syntax coverage, AST stability, interpreter assumptions, and license
  fit.
- Decide whether native Windows mode emulates POSIX paths internally or exposes a
  documented translation layer at command boundaries.
- Decide the minimum v0 applet set and whether applets are built into the same
  binary, exposed through symlinks/shims, or both.
- Decide the first public repository visibility and release claim language before
  publishing CI badges or compatibility tables.
