# Security Policy

## Supported versions

| Version | Supported |
| --- | --- |
| latest release | yes |
| `-master-` prereleases | no — these are nightly builds of whatever is on master |
| anything older | no |

There is one release line. A fix goes into the next release rather than being
backported, because there is nothing yet to backport to.

## Reporting a vulnerability

Use GitHub's private reporting: **Security → Report a vulnerability** on
<https://github.com/xiongnemo/nemosh>. That keeps the report unpublished while
it is being looked at.

Please do not open a public issue for something exploitable. An ordinary bug —
a wrong exit status, a missing option — is not that, and a normal issue is the
right place for it.

Include what a fix would need: the version from `nemosh --version`, the Windows
build, and the smallest input that shows the problem.

Expect an acknowledgement within a week. This is one person's project, not a
company, and it is honest to say that rather than promise a schedule that will
not be kept.

## What counts

Nemosh is a shell. It runs what it is told to run, so "a script I wrote deleted
my files" is the shell working. What would be a vulnerability is Nemosh doing
something the script did **not** say:

- **A command running that the script did not name.** The clearest example this
  project has already had: `find . -name '*.tmp' | xargs rm` received every file
  in the tree, because `find` ignored `-name` and printed everything. That was
  fixed before any release, and it is exactly the shape to report.
- **Argument corruption at the Windows launch boundary.** An operand containing
  `&`, a quote, or `%` reaching a child differently than it was written. The
  `ComSpec` path exists to prevent this and is tested for it.
- **A path escaping where it was pointed.** `/c/…`, `/mnt/c`, UNC and
  drive-relative forms all resolve through one model; a spelling that reaches
  outside what it names is a defect of that model.
- **A privilege answer that is wrong.** `id -u` reports 0 only when the process
  is elevated *and* the Administrators group is enabled in its token. A prompt
  uses that to decide what to warn about.
- **Anything that makes Nemosh execute attacker-controlled text as a command.**
  A prompt is the live example: `PS1` is expanded on every draw, so expansion
  happens **before** the backslash escapes are rendered, deliberately — the
  other order would feed a directory name back into the parser and a directory
  named `$(...)` would run.

## What does not

- **Applet options that are not implemented.** They are refused by name with a
  non-zero status. A script asking for one fails; it does not get something
  else. `docs/support-matrix.md` records which is which, measured.
- **`hash`, `ulimit`, `fg`, `bg`, `set -b`/`-n`/`-v`.** Refused with a reason.
- **Linux and macOS behaviour.** Build-and-test targets, not supported ones.
- **Terminal state after a crash.** Raw mode is restored on every path out
  including a panic; if you find one where it is not, that is an ordinary bug
  and welcome as an issue.

## Supply chain

The binary links Go's standard library, `golang.org/x/term`, and
`golang.org/x/sys/windows` — nothing else. `github.com/BurntSushi/toml` is used
by the test harness and is not in the binary.

`govulncheck` runs on every push and weekly, and reports only vulnerabilities
this binary can actually reach.

Release artifacts carry a SHA-256 file beside them, and the published binary is
the one CI verified — there is no rebuild at release time. Artifacts are **not**
code-signed, so Windows SmartScreen will warn on a browser download; installing
through Scoop verifies the checksum instead.

BusyBox is a behaviour reference and none of its code is here; see
[`docs/design/reference-methodology.md`](docs/design/reference-methodology.md).
