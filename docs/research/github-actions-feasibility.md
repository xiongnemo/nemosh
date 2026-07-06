# GitHub Actions Feasibility

The CI research goal is to learn which behaviors can be tested on hosted runners
and which need nightly, self-hosted, or manual environments.

## Required Runner Families

- Linux: primary POSIX-like execution environment and broad package availability.
- macOS: BSD userland differences, filesystem case behavior, default shell
  differences, and terminal behavior.
- Windows native: pure Go executable behavior without POSIX compatibility layer.
- Windows MSYS2: POSIX-like userland and runtime behavior through MSYS2.
- Windows Cygwin: POSIX compatibility layer behavior through Cygwin.
- WSL experimental: useful for behavior comparison, but treat as optional until
  hosted-runner reliability and setup cost are proven.

## Initial CI Matrix Shape

```yaml
strategy:
  fail-fast: false
  matrix:
    include:
      - os: ubuntu-latest
        env: linux
      - os: macos-latest
        env: macos
      - os: windows-latest
        env: windows-native
      - os: windows-latest
        env: windows-msys2
      - os: windows-latest
        env: windows-cygwin
```

Keep WSL as a separate experimental workflow until setup and stability are
measured.

## Job Categories

- Fast required jobs: Go unit tests, parser tests, platform-independent golden
  tests, and lint/static checks after code exists.
- Platform required jobs: core integration tests on Linux, macOS, Windows native,
  MSYS2, and Cygwin.
- Experimental jobs: WSL, PTY-heavy tests, job control tests, and long external
  suite runs.
- Nightly jobs: large differential corpus, fuzz smoke, broad reference shell
  matrix, and slow external suites.

## Windows Setup Notes

- MSYS2: use `msys2/setup-msys2`; install shells and POSIX utilities through
  `pacman`; run shell tests under the `msys2 {0}` shell when needed.
- Cygwin: use a Cygwin setup action or Chocolatey plus `cyg-get`; cache only
  after confirming cache correctness; account for path differences.
- Git Bash: useful as an additional sanity check, but do not treat it as a full
  POSIX layer reference.
- WSL: hosted runner images may expose WSL commands, but distro availability,
  WSL1 versus WSL2 behavior, startup latency, and permissions need explicit
  experiments.
- Native Windows: run Go tests from PowerShell or cmd and avoid assuming `/bin/sh`
  exists.

## Artifacts And Reports

Each CI job should eventually upload:

- Go test output
- Conformance summary
- External suite summary
- Known-failure report
- Platform info: OS version, shell versions, Go version, path mode, locale

Standardize report names so failures can be compared across runners.

## Feasibility Experiments To Run First

1. Print runner environment details for Linux, macOS, and Windows.
2. Install and invoke ShellSpec on Linux, macOS, MSYS2, and Cygwin.
3. Run a tiny golden shell behavior corpus against reference shells installed on
   each environment.
4. Verify how Windows native tests create processes, pipes, temporary files,
   symlinks, and executable scripts.
5. Measure WSL setup and whether WSL1 behavior can be reliably selected.
6. Capture and compare newline, path, and executable lookup behavior.

## Required Research Outputs

```text
Workflow | Runner | Setup action | Dependencies | Runtime shell | Required/experimental | Known risks | Next action
```

Do not make a job required until it is stable for several consecutive runs.
