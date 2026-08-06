# Behavior Test Format

Nemosh uses project-owned behavior tests with machine-readable metadata. The
format should preserve the useful BusyBox testsuite model: each case is a small
shell/app fragment that proves one assertion, but Nemosh records platform and
reference semantics explicitly.

## Reference Patterns

BusyBox testsuite:

- Tests live under the applet name or in `<applet>.tests` files.
- A test is a shell fragment.
- The fragment exits successfully when the assertion passes.
- Metadata is lightweight comments such as `FEATURE:` and `XFAIL`.

Nemosh adaptation:

- Keep tests small and assertion-focused.
- Store metadata in TOML so Windows/POSIX differences are explicit.
- Do not copy GPL test contents into Nemosh. Use reference suites as design and
  differential oracles, not as vendored source unless licensing is reviewed.

## File Layout

```text
tests/behavior/
  shell/posix/
  shell/windows/
  applets/coreutils/
  applets/windows/
  fixtures/
```

## TOML Shape

```toml
id = "shell.posix.simple-command.echo-status"
area = "shell"
kind = "golden"
semantics = "posix"
platforms = ["windows", "linux", "darwin"]
references = ["posix", "busybox-ash", "dash", "bash-posix"]

script = '''
echo ok
'''
stdin = ""
cwd = "work"
requires = []

[env]
EXAMPLE = "value"
EMPTY = ""

[files]
"work/input.txt" = "fixture contents\n"

[expect]
status = 0
stdout = "ok\n"
stderr = ""

[notes]
standard = "POSIX.1 shell command language"
why = "A simple command exits with the command status and writes stdout."
```

## Fields

Required:

- `id`: stable dotted identifier.
- `area`: `shell`, `path`, `exec`, `env`, `fd`, `applet`, or `platform`.
- `kind`: `golden`, `differential`, `probe`, or `xfail`. Today the runner only
  records this value; it does not act on it. In particular `xfail` does **not**
  invert or tolerate a failure, so a checked-in case must state what Nemosh
  actually does. Behavior that a reference requires but Nemosh does not implement
  belongs in the readiness ledger as a gap, not in the corpus as a red case.
- `semantics`: `posix`, `busybox-w32`, `nemosh`, or `platform`.
- `platforms`: allowed platforms.
- Exactly one of `script` or `command`. `command` is an array whose first item
  is an applet name; `script` is passed to a freshly built `nemosh -c` process.
- `expect.status`.
- `expect.stdout` and `expect.stderr`. All three expectation fields must be
  present, including when their values are zero or empty.

Optional:

- `references`: shells/applets to compare.
- `requires`: feature flags such as `windows`, `network-share`, `clipboard`,
  `case-sensitive-dir`, or `admin`.
- `files`: input fixture files to create.
- `env`: environment variables for the case.
- `cwd`: initial cwd.
- `stdin`: exact standard input, defaulting to empty.
- `notes.standard`: POSIX section or reference source.
- `notes.why`: what the case proves, including any recorded difference from a
  reference. `[notes]` accepts these two keys and no others.

There is deliberately no top-level `known_differences` key and no top-level
`standard` key: unknown fields are rejected by `internal/testutil/behavior/case.go`,
so reference differences belong in `notes.why`.

Unknown fields are errors. `cwd` and every `files` key must be a non-empty,
safe relative path: absolute paths, volume-qualified paths, and paths that
escape through `..` are rejected. Fixture files and `cwd` are prepared under a
new temporary root for each script case. The parent process cwd and environment
are never changed.

Script subprocesses receive only the variables listed in `env`; values are
passed exactly, including explicit empty values. This makes the environment
deterministic instead of inheriting the developer or CI environment.

Platform and requirement gates are evaluated before sandbox setup. A platform
that does not match the host, or a requirement the harness does not support,
produces an explicit skip. A completed process reports status, stdout, and
stderr. Failure to prepare or launch the case is a harness error, never a
synthetic shell status code.

## Test Semantics Tags

Use `posix` only when the behavior is required by POSIX shell or utilities.

Use `busybox-w32` when Nemosh deliberately follows busybox-w32 native Windows
behavior, such as current-root path handling or fixed suffix lookup.

Use `nemosh` when the behavior is a Nemosh extension, such as `/dev/clipboard` or
future virtual network mounts.

Use `platform` for behavior directly inherited from Windows, such as ACL access
checks or filesystem case sensitivity.

## Comparison Policy

- POSIX-tagged shell tests should pass against dash or another POSIX reference
  where possible.
- busybox-w32-tagged Windows tests should be compared against local busybox-w32
  `ash` or applets.
- Nemosh extension tests are golden tests first; references may not exist.
- Reference differences must be recorded instead of hidden.
