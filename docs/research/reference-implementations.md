# Reference Implementations

This document lists reference projects to clone for research and the questions
to ask of each. Use shallow clones first. Keep all cloned code in a separate
reference workspace, not mixed with project source.

## Suggested Clone Layout

```text
references/
  shells/
    zsh/
    bash/
    dash/
    mksh/
    yash/
    busybox/
    oils/
  go-shells/
    mvdan-sh/
  windows-compat/
    msys2-runtime/
    newlib-cygwin/
    busybox-w32/
    busybox-mingw/
```

WSL1 should be researched through Microsoft documentation, issues, runner image
behavior, and black-box experiments. It should not be treated as a fully
available source reference.

## Clone Targets

```powershell
git clone --depth 1 https://github.com/zsh-users/zsh references/shells/zsh
git clone --depth 1 https://git.savannah.gnu.org/git/bash.git references/shells/bash
git clone --depth 1 https://git.kernel.org/pub/scm/utils/dash/dash.git references/shells/dash
git clone --depth 1 https://github.com/MirBSD/mksh references/shells/mksh
git clone --depth 1 https://github.com/magicant/yash references/shells/yash
git clone --depth 1 https://git.busybox.net/busybox references/shells/busybox
git clone --depth 1 https://github.com/oils-for-unix/oils references/shells/oils
git clone --depth 1 https://github.com/mvdan/sh references/go-shells/mvdan-sh
git clone --depth 1 https://github.com/msys2/msys2-runtime references/windows-compat/msys2-runtime
git clone --depth 1 https://sourceware.org/git/newlib-cygwin.git references/windows-compat/newlib-cygwin
git clone --depth 1 https://github.com/rmyorston/busybox-w32 references/windows-compat/busybox-w32
# Identify and clone a maintained BusyBox MinGW/native Windows reference if it
# differs materially from busybox-w32.
```

If a host blocks one transport, record the failure and use the official mirror
documented by the project.

## What To Study In Each Reference

- zsh: interactive architecture, completion, history, modules, globbing,
  parameter expansion, prompt behavior, and test organization.
- bash: real-world script compatibility, bash extensions, job control, traps,
  startup behavior, and POSIX mode differences.
- dash: small POSIX shell implementation, parser/runtime boundaries, special
  built-ins, redirection, and error behavior.
- mksh/yash: alternate POSIX/ksh-influenced behavior, strictness, and tests for
  ambiguous shell semantics.
- BusyBox ash: compact embedded implementation and portability lessons through
  BusyBox ecosystem constraints.
- Oils: spec test format, differential testing against existing shells, semantic
  modeling, and modernization tradeoffs.
- mvdan/sh: Go parser/AST/interpreter API design, formatter tradeoffs, and pure
  Go limitations around fork, real PIDs, and file descriptors.
- MSYS2 runtime: path conversion, process spawning, pseudo-POSIX filesystem and
  terminal behavior, signals, and fork emulation inherited from Cygwin lineage.
- Cygwin/newlib-cygwin: POSIX compatibility layer design, fork/exec, pty, signal,
  uid/gid, symlink, fd, and path translation behavior.
- busybox-w32: native Windows command and shell compromises, executable lookup,
  path handling, and lightweight portability decisions.
- BusyBox MinGW/native Windows reference: compare build assumptions, Win32 API
  boundaries, command applet behavior, and whether it differs from busybox-w32.
- WSL1: syscall translation model, filesystem boundary behavior, Windows/Linux
  process interop, path interop, and runner feasibility.

## Behavior Matrix To Build

For each researched behavior, record:

```text
Behavior | POSIX text | zsh | bash | dash | mksh/yash | Oils tests | Windows native | MSYS2 | Cygwin | WSL1 | Notes
```

Start with behavior families that drive architecture:

- Subshells and command substitution
- Pipelines and exit status
- Redirection and fd lifetime
- Special built-in errors
- Assignment/export/readonly behavior
- Signal/trap behavior
- Job control and terminal foregrounding
- PATH and executable lookup
- Pathname expansion and locale matching
- Here-documents and newline handling
