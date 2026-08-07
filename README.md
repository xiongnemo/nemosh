# Nemosh

A native Windows-first, cross-platform, BusyBox-style shell and utility bundle,
written in Go. One binary, no runtime dependencies.

The shell target is advanced ash-like behavior. It is **not** MSYS2, Cygwin,
PowerShell, or cmd compatibility, and it makes no POSIX certification claim.

What makes it different from running BusyBox under an emulation layer: Nemosh
gives POSIX semantics over **native** Windows path, launch, device, and
interrupt boundaries. `/c/Users`, `/mnt/c`, UNC shares, and drive-relative
current roots resolve through one model; `.bat` and `.cmd` cross an explicit
`ComSpec` boundary with arguments that survive; `/dev/clipboard` is real; `cd`
answers with the case the filesystem actually reports. No argv translation layer
sits in between.

## Status

**Pre-release.** v0 is complete and audited; v1.0 is release hardening. See
`docs/design/v1-scope.md` for what that covers and
`docs/design/v0-readiness.md` for the evidence ledger behind the v0 claim.

Read [`docs/support-matrix.md`](docs/support-matrix.md) before depending on
anything. It is measured, not aspirational, and it names the gaps — including
one that matters: **`find` does not honour `-name`, `-type`, or any other
expression**, and prints the whole tree before failing. Do not pipe `find` into
a command with effects.

## Platforms

| Platform | Status |
| --- | --- |
| `windows/amd64` | **Supported** — the target |
| `linux/amd64` | Build and test only, not a support commitment |
| `darwin/*` | Compile check only |
| `windows/arm64` | Untested |

## Install

Scoop packaging is v1.0 work and does not exist yet. Until then, build from
source.

## Build

```bash
go build -o nemosh ./cmd/nemosh
```

For a release-shaped build, with the version stamped in:

```bash
go build -trimpath -ldflags "-s -w $(bash scripts/version.sh --ldflags)" \
  -o dist/nemosh ./cmd/nemosh
```

A full clone is required — the version is derived from the nearest semver tag,
and a shallow clone silently yields the fallback base.

## Use

```console
$ nemosh -c 'echo hello'
$ nemosh script.sh arg1 arg2
$ nemosh                       # interactive, when stdin is a terminal
$ nemosh --version
$ nemosh --list                # every bundled applet, one per line
```

Nemosh is a multicall binary. Invoked under an applet's name — directly, or
through a shim — it runs that applet:

```console
$ nemosh cat file.txt
$ nemosh winpath /c/Users
C:/Users
```

### Diagnostics

Failures carry a first line, a targeted hint, and optional detail:

```console
$ nemosh -c 'nosuchcommand'
nosuchcommand: not found
hint: no directory on PATH holds nosuchcommand; `command -v nosuchcommand` answers the same question
```

`NEMOSH_DEBUG=path,exec,fd` adds resolution traces on the channel you name.

## Design

Nemosh follows **busybox-w32** first, then BusyBox ash, then dash and POSIX.
Claims are verified against the source, never against memory; the clone lives at
`references/windows-compat/busybox-w32` and is not distributed with this
project.

Two rules govern the implementation:

- **A capability that is absent must fail loudly.** `hash`, `ulimit`, `fg`,
  `bg`, and `set -b`/`-n`/`-v` refuse with a reason and a non-zero status rather
  than approximating. An applet option that is not implemented is refused, not
  swallowed.
- **Parse before effects.** A script is parsed in full before any of it runs, so
  a syntax error late in a file produces no output from the lines before it.
  This diverges from bash and dash deliberately.

Further reading: `docs/design/` for scope and the Windows path and execution
models, `docs/research/` for the reference measurements behind them, and
`AGENTS.md` for the conventions any change is held to.

## Contributing

Every change is test-first, and the full gate must pass:

```bash
gofmt -l .                                    # empty
go vet ./... && GOOS=linux go vet ./...
GOOS=darwin go build ./...
NEMOSH_DIFFERENTIAL=strict go test -race -shuffle=on -count=1 ./...
```

plus a 250 pure-line ceiling per production `.go` file. `AGENTS.md` has the
details, including why green tests are not taken as evidence of correctness.

Contributions are under Apache-2.0 by section 5 of the license; there is no
separate CLA.

## License

Apache-2.0. See [`LICENSE`](LICENSE), [`NOTICE`](NOTICE), and
[`THIRD-PARTY-NOTICES.md`](THIRD-PARTY-NOTICES.md).

BusyBox and busybox-w32 are GPL-2.0 and are **not** included here. No BusyBox
code was copied; they are consulted as behavior references only.
