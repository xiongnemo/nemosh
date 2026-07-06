# Initial Research Findings

This document records facts already checked during the first research pass. It
is not a final design decision log.

## POSIX And Certification

- The current primary standard target is The Open Group Base Specifications
  Issue 8, POSIX.1-2024.
- The Shell Command Language page is publicly readable and includes the expected
  shell sections: quoting, token recognition, reserved words, parameters,
  expansions, redirection, exit status, commands, grammar, job control, signals,
  execution environment, pattern matching, and special built-ins.
- The `sh` utility has a separate POSIX page and must be researched alongside
  the language chapter.
- Open Group certification and official test suites are separate from open-source
  regression testing. Public CI can support compatibility claims, but it should
  not be described as official POSIX or UNIX certification.

## Useful Open-Source References

- `mvdan/sh` is a mature Go shell parser, formatter, and interpreter. Its README
  explicitly notes pure Go limitations around fork-like behavior, PIDs, and file
  descriptors.
- `oils-for-unix/oils` has a spec test format with metadata and compare-shells
  support. It is useful as a model for differential shell testing.
- `shellspec/shellspec` supports many POSIX-style shells and has existing Linux,
  macOS, MSYS2, Cygwin, and other platform workflows.
- `bats-core/bats-core` is useful for CLI integration testing, but it is a Bash
  testing framework rather than a POSIX conformance framework.
- zsh, bash, dash, mksh/yash, and BusyBox ash should be compared by behavior
  family rather than treated as one compatibility target.

## Windows Compatibility References

- MSYS2 has `msys2/setup-msys2` for GitHub Actions and `msys2/msys2-runtime` as
  a source reference. Its runtime inherits heavily from Cygwin design.
- Cygwin can be installed on GitHub Actions using Chocolatey plus `cyg-get`, or
  by evaluating dedicated setup actions. Its source reference is
  `newlib-cygwin`.
- GitHub hosted Windows images include Git Bash and MSYS2 in current runner image
  documentation, but PATH and shell availability should still be probed in CI.
- WSL behavior should be researched through docs, issues, runner experiments,
  and black-box tests. Treat WSL1 as a behavior reference, not as a normal source
  repository to clone.
- The intended Windows target is POSIX shell semantics on Windows, not cmd or
  PowerShell semantics. WSL1, Cygwin, MSYS2, busybox-w32, and a BusyBox
  MinGW/native Windows reference should be used to study viable compromises.

## Confirmed Scope Direction

- Start script-first and non-interactive. REPL, line editing, completion,
  history, prompts, and other interactive features remain required later.
- Treat zsh-level capability as the long-term target, but do not make zsh
  interactive richness part of the v0 implementation gate.
- Evaluate `mvdan/sh` before deciding parser strategy. Existing Go parser and
  AST reuse may accelerate the project, but the runtime still needs careful
  Windows POSIX design.
- Use conservative compatibility language before any official certification.
  Open-source CI supports compatibility evidence, not Open Group certification.

## Immediate CI Direction

- Start with a research probe workflow before building a full conformance suite.
- The probe should gather runner information and run a tiny POSIX smoke script
  under Linux, macOS, Git Bash, MSYS2, and Cygwin.
- Keep WSL as an experimental follow-up until setup reliability, WSL1/WSL2
  selection, and distro availability are measured.
- Do not make any platform-specific job required until it is stable across
  repeated runs.

## Near-Term Research Tasks

1. Clone the listed reference implementations into `references/` with shallow
   clones and record any inaccessible upstreams or mirror substitutions.
2. Build a behavior matrix for the first ten architecture-driving shell topics:
   subshells, command substitution, pipelines, redirection, special built-ins,
   assignment/export, traps, job control, PATH lookup, and here-documents.
3. Evaluate ShellSpec and Oils spec tests for licensing, setup cost, platform
   support, and expected-failure management.
4. Run the research probe workflow and append observed runner differences here.
