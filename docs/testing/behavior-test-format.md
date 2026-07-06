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
- `kind`: `golden`, `differential`, `probe`, or `xfail`.
- `semantics`: `posix`, `busybox-w32`, `nemosh`, or `platform`.
- `platforms`: allowed platforms.
- `script` or `command`.
- `expect.status`.
- `expect.stdout` and `expect.stderr`, unless intentionally unspecified.

Optional:

- `references`: shells/applets to compare.
- `requires`: feature flags such as `windows`, `network-share`, `clipboard`,
  `case-sensitive-dir`, or `admin`.
- `files`: input fixture files to create.
- `env`: environment variables for the case.
- `cwd`: initial cwd.
- `known_differences`: reference-specific notes.
- `standard`: POSIX section or reference source.

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
