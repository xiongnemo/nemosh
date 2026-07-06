# Test Suite Survey

The research goal is not to find one magic POSIX test suite. A credible shell
project needs layered tests: small internal tests, external behavioral suites,
differential tests against reference shells, and optional certification work.

## Test Layers

- Unit tests: parser, lexer, expansion, redirection planning, environment model,
  built-ins, path handling, and runtime primitives.
- Golden tests: script input plus expected stdout, stderr, exit status, file
  effects, and environment changes.
- Differential tests: run the same case against reference shells and compare
  behavior where POSIX is clear or where a compatibility mode is chosen.
- External suite tests: reuse existing shell-focused projects where licensing
  and portability allow.
- Fuzz tests: parser round-trips, expansion edge cases, quoting, glob patterns,
  and crash resistance.
- Integration tests: real processes, pipes, redirections, temporary files,
  terminals/PTYs, and platform-specific behavior.
- Certification tests: official Open Group/UNIX certification path, if the
  project later needs formal claims.

## Candidate Open-Source Suites

- ShellSpec: BDD framework for POSIX shells and common shells. Strong for
  writing project-owned behavior tests and useful on Linux, macOS, MSYS2,
  Cygwin, and other environments.
- Oils spec tests: valuable differential shell behavior corpus. Study metadata,
  compare-shells mechanism, and expected-failure handling.
- zsh tests: important for long-term zsh-level capability, especially expansion,
  completion-adjacent behavior, modules, and interactive assumptions.
- bash tests: useful for real-world compatibility and POSIX-mode comparisons,
  but must be filtered to avoid treating bash extensions as POSIX requirements.
- dash tests: useful for POSIX sh baseline and minimal implementation behavior.
- ksh/mksh/yash tests: useful for ambiguity checks and alternate historical
  semantics.
- Bats: useful for CLI-level integration tests, but it is bash-oriented and not
  itself a POSIX shell conformance framework.
- sharness: useful for Git-style integration tests of command-line tools.
- shelltest-style runners: useful to inspect, but must be evaluated for project
  health and coverage before adoption.

## Official Conformance Boundary

Open-source CI can provide strong evidence of behavioral compatibility, but it
does not equal official POSIX or UNIX certification.

Research should document:

- Which official Open Group test suites are applicable to shell/system claims.
- Whether access requires licensing, fees, or certification program enrollment.
- Which claims can be made without certification.
- Which claims should be avoided until official testing is performed.

Use conservative language in project documentation. Prefer claims like
"POSIX-oriented", "POSIX sh compatibility target", or "tested against POSIX
behavior suites" until formal certification is pursued.

## Test Metadata Model To Design Later

Every conformance-style test should be able to carry metadata:

```text
id
category
standard_reference
requires_platform
requires_shell_mode
expected_stdout
expected_stderr
expected_status
expected_filesystem_effects
known_failures
reference_shell_results
```

This model supports expected-failure management without hiding regressions.

## First Coverage Milestones

1. Parser and token recognition cases from POSIX grammar.
2. Quoting and expansion order cases.
3. Simple commands, assignments, environment export, and built-in lookup.
4. Redirection, here-documents, and fd duplication cases.
5. Pipelines, lists, subshells, command substitution, and exit status.
6. Special built-in error consequences.
7. Pathname expansion and pattern matching.
8. Traps, signals, and platform-specific expected failures.
9. Job control and PTY behavior as experimental or platform-gated tests.

## Evaluation Checklist For Each Suite

```text
Suite | License | Install method | Host dependencies | Runs on Linux | Runs on macOS | Runs on Windows native | Runs on MSYS2 | Runs on Cygwin | Coverage | CI suitability | Notes
```

Do not vendor a suite until license, maintenance state, and CI reliability are
understood.
