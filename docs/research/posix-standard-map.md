# POSIX Standard Map

Primary source: The Open Group Base Specifications Issue 8, POSIX.1-2024.

Core online anchors:

- Shell Command Language: `https://pubs.opengroup.org/onlinepubs/9799919799/utilities/V3_chap02.html`
- `sh` utility: `https://pubs.opengroup.org/onlinepubs/9799919799/utilities/sh.html`
- Shell and Utilities table of contents: `https://pubs.opengroup.org/onlinepubs/9799919799/utilities/contents.html`
- Open Brand testing overview: `https://www.opengroup.org/openbrand/testing`
- UNIX certification overview: `https://www.opengroup.org/certifications/unix`

## Scope Model

Research must distinguish three layers:

- POSIX shell language: parsing, quoting, expansion, redirection, command
  execution, variables, functions, built-ins, traps, and exit behavior.
- POSIX shell utility: command-line interface and execution modes of `sh`.
- POSIX operating environment: process, filesystem, permissions, signals,
  terminal, job control, file descriptors, and required utilities.

The first layer can be implemented in Go across platforms. The third layer is
not natively available on Windows and must be researched as compatibility policy,
not assumed as a normal OS substrate.

## Shell Command Language Sections To Map

- 2.1 Shell Introduction: execution overview and command processing order.
- 2.2 Quoting: backslash, single quotes, double quotes, and dollar-single-quotes
  in Issue 8.
- 2.3 Token Recognition: lexical rules, comments, operators, and alias timing.
- 2.4 Reserved Words: context-sensitive recognition.
- 2.5 Parameters and Variables: positional, special, shell variables, exported
  environment, assignment syntax, and readonly behavior.
- 2.6 Word Expansions: tilde, parameter, command, arithmetic, field splitting,
  pathname expansion, and quote removal order.
- 2.7 Redirection: fd mapping, here-documents, open/append/dup/read-write
  semantics, and error handling.
- 2.8 Exit Status and Errors: command status, shell errors, special built-in
  consequences, and non-interactive shell exits.
- 2.9 Shell Commands: simple commands, pipelines, lists, compound commands, and
  functions.
- 2.10 Shell Grammar: grammar productions and lexical conventions.
- 2.11 Job Control: foreground/background jobs, process groups, and terminal
  control.
- 2.12 Signals and Error Handling: traps, signal defaults, and ignored signals.
- 2.13 Shell Execution Environment: subshells, functions, command substitution,
  variable scope, current directory, umask, and open files.
- 2.14 Pattern Matching Notation: glob semantics and locale-sensitive matching.
- 2.15 Special Built-In Utilities: special failure semantics and lookup order.

## Built-In Categories To Research

Special built-ins require priority because POSIX gives them different error and
assignment behavior. Research should map at least:

- `break`, `continue`, `.`, `eval`, `exec`, `exit`, `export`, `readonly`,
  `return`, `set`, `shift`, `times`, `trap`, `unset`
- Regular built-ins commonly needed by scripts: `cd`, `command`, `echo`, `fc`,
  `fg`, `bg`, `getopts`, `jobs`, `kill`, `pwd`, `read`, `test`, `ulimit`,
  `umask`, `wait`

For each built-in, record:

- POSIX utility page and required options
- Whether it mutates shell state
- Whether it depends on OS features missing or different on Windows
- How dash, bash, zsh, and mksh/yash behave in edge cases

## Windows Semantic Risk Areas

- Process model: POSIX shells rely on fork-like semantics; Go and native Windows
  do not provide fork.
- File descriptors: POSIX redirection uses integer fd behavior, close-on-exec,
  dup, pipes, and inherited descriptors; Windows handles differ.
- Signals: POSIX signal numbers, process groups, terminal-generated signals,
  and trap behavior do not map cleanly to native Windows.
- Job control: POSIX job control depends on sessions, process groups, tty
  foreground control, and signals like SIGTSTP/SIGCONT.
- Paths: drive letters, UNC paths, separators, case sensitivity, symlinks, and
  MSYS2/Cygwin path conversion all need separate policy.
- Permissions and ownership: mode bits, executable checks, uid/gid, sticky bits,
  and symlink permissions are only partially represented on Windows.
- Newlines and text mode: CRLF behavior can leak through external tools and
  pseudo-terminal layers.
- Executable lookup: POSIX PATH search differs from PATHEXT, file extensions,
  shebang handling, and Windows app execution.

## Research Output Format

For each standard topic, produce a compact table with:

```text
Topic | POSIX requirement | Undefined/implementation-defined points | Reference shells | Windows notes | Test sources
```

This table should remain descriptive until the design phase. Do not turn it into
implementation policy prematurely.
