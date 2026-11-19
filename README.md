# Nemosh

A native Windows BusyBox-style shell and utility bundle, written in Go. One
binary, no runtime dependencies.

The shell target is advanced ash-like behavior. It is **not** MSYS2, Cygwin,
PowerShell, or cmd compatibility, and it makes no POSIX certification claim.

What makes it different from running BusyBox under an emulation layer: Nemosh
gives POSIX semantics over **native** Windows path, launch, device, and
interrupt boundaries. `C:/Users`, `/c/Users`, `/mnt/c`, UNC shares, and `cd D:` to where
that drive was left resolve through one model, and paths are reported as busybox-w32 reports
them, `C:/Users/nemo`, which any native program can open (`NEMOSH_PATHS=msys` reports
`/c/Users/nemo` instead); `.bat` and `.cmd` cross an explicit
`ComSpec` boundary with arguments that survive; `/dev/clipboard` is real; `cd`
answers with the case the filesystem actually reports. No argv translation layer
sits in between.

## Status

**Released.** An exact `vMAJOR.MINOR.PATCH` tag is a release, each one in
[`CHANGELOG.md`](CHANGELOG.md), and every push to `master` publishes a nightly
prerelease. See `docs/design/v1-scope.md` for what v1 covers and
`docs/design/v0-readiness.md` for the evidence ledger behind the v0 claim.

Read [`docs/support-matrix.md`](docs/support-matrix.md) before depending on
anything. It is measured, not aspirational, and it names the gaps. `find` takes
most of busybox's tests and actions, `-exec` and `-delete` among them, and
refuses the rest -- `-user`, `-group`, `-type b` -- before the walk begins, so a
pipeline never receives paths the expression did not select.

## Platforms

| Platform | Status | Binary published |
| --- | --- | --- |
| `windows/amd64` | **Supported** — the target | yes, `.zip` |
| `windows/arm64` | **Supported** — tested natively on CI's Arm runner | yes, `.zip` |
| `windows/386` | **Supported** — tested under WOW64 on CI's x64 runner | yes, `.zip` |

Windows is the only platform. Nemosh calls Windows APIs directly where one does
the job, as busybox-w32 does, and does not build for anything else; 32-bit Arm
is no target Go has had since 1.26. Until
2026-11-17 the Linux and macOS binaries were published as an unsupported
convenience, along with `.deb`, `.rpm` and `.apk` packages; the release v1.4.0
and the nightly `v1.4.66-master-b9608f763100` are the last that have them.

## Install

```powershell
scoop bucket add nemo https://github.com/xiongnemo/windows-binaries-scoop-bucket
scoop install nemo/nemosh            # the release line
scoop install nemo/nemosh-nightly    # built from master, one per push
```

Only the `nemosh` shim is installed. It is a multicall binary, and two of its
applets share a name with a Windows command that means something else --
`find.exe` takes a literal string and a file rather than a path and a predicate,
and `sort.exe` is not POSIX sort -- so shimming all of them would quietly change
what those names do. `nemosh --list` names the applets; to reach one directly:

```powershell
scoop shim add cat "$(scoop prefix nemosh)/nemosh.exe" cat
```

Forward slashes on purpose: PowerShell accepts them, and a backslash here is one
escape away from being eaten by whatever generates the documentation.

The download is verified against a published SHA-256. Artifacts are not
code-signed, so a browser download will raise SmartScreen; Scoop checks the
checksum instead.

## Build

```bash
bash scripts/build.sh          # dist/nemosh[.exe], version stamped in
bash scripts/build.sh --dev    # keep symbols for a debugger
bash scripts/build.sh -o path  # somewhere else
```

A plain `go build ./cmd/nemosh` also works, but reports
`v0.0.1-unknown-<commit>`: Go's build info records the commit, the time, and
whether the tree was modified, but **not the tag and not the branch**. Those are
build-time questions, so `scripts/version.sh` asks git and injects the answers,
and `scripts/build.sh` is the wrapper that does it for you. It also builds with
`-tags tcell_minimal`, as every release is, which leaves out the terminal
descriptions tcell carries for terminals Windows does not have.

A full clone is required — the version is derived from the nearest semver tag,
and a shallow clone silently yields the fallback base.

## Use

```console
$ nemosh -c 'echo hello'
$ nemosh script.sh arg1 arg2
$ nemosh                       # interactive, when stdin is a terminal
$ nemosh --version
$ nemosh --list                # every bundled applet, one per line
$ nemosh --migrate --dry-run   # what would move from where an older nemosh kept it; see Files
$ nemosh --help
```

Nemosh is a multicall binary. Invoked under an applet's name — directly, or
through a shim — it runs that applet:

```console
$ nemosh cat file.txt
$ nemosh winpath /c/Users
C:/Users
```

An interactive session reads `$ENV`, or `~/.nemoshrc` when `ENV` is not set.
[`docs/examples/nemoshrc`](docs/examples/nemoshrc) is one to copy: a prompt with
the git branch and the last status, a few aliases, and gh's and git's completion.

### Files

What nemosh keeps of yours is under your home directory, not in `%APPDATA%`. A
shell's own files stay where a shell's are, as bash's and busybox's do; the
completion specs you write are configuration, in the XDG layout on Windows too,
where fish and bash-completion keep theirs:

| What | Where | Up to 1.5 |
| --- | --- | --- |
| rc file | `$ENV`, or `~/.nemoshrc` when `ENV` is not set | the same |
| history | `~/.nemosh_history`, or the file `HISTFILE` names | the same |
| completion specs | `$XDG_CONFIG_HOME/nemosh/completions/<cmd>.toml`, by default `~/.config/nemosh/completions` | `%APPDATA%\nemosh\completions` |

`XDG_CONFIG_HOME` is honoured when it is an absolute path, as the XDG
specification has it, and ignored otherwise; the home is `HOME`, or
`USERPROFILE` without it. `XDG_STATE_HOME` does not move history, as it does not
move bash's. A completion spec is looked for in the new place first and the old
one after, so each is found where it is. The nightlies 1.5.2 and 1.5.3 kept
history in `~/.local/state/nemosh/nemosh_history`, or
`$XDG_STATE_HOME/nemosh/nemosh_history`; a file there is used when there is no
`~/.nemosh_history`, and `nemosh --migrate` moves it back.

Where an item is only at its old place, it is used there, and an interactive
session says so once, on stderr, before its first prompt. `nemosh --migrate`
moves it: `--dry-run` prints the plan and touches nothing, the move never
overwrites (the same file on both sides removes the old copy; a different one
keeps both and ends with status 1), a second run has nothing to do, and it is
refused while another nemosh is running, since that one would go on writing to
the old place. The shell you run it from does the same until it exits.

### Diagnostics

Failures carry a first line, a targeted hint, and optional detail:

```console
$ nemosh -c 'nosuchcommand'
nosuchcommand: not found
hint: no directory on PATH holds nosuchcommand; `command -v nosuchcommand` answers the same question
```

At a prompt a name that is nearly one this shell can run is answered with it
(``hint: did you mean `git`?``), a `NAME.ps1` on PATH is named as a PowerShell
script with how to run it, and a `command_not_found_handle` function, as in bash,
takes over the answer altogether.

`NEMOSH_DEBUG=path,exec,fd` adds resolution traces on the channel you name.

## Design

Nemosh follows **busybox-w32** first, then BusyBox ash, then dash and POSIX.
Claims are verified against the source, never against memory; the clone lives at
`references/windows-compat/busybox-w32` and is not distributed with this
project.

Two rules govern the implementation:

- **A capability that is absent must fail loudly.** `ulimit`, `fg`, `bg`,
  `suspend`, `bind`, and `set -b`/`-m`/`-v` refuse with a reason and a non-zero
  status rather than approximating. An applet option that is not implemented is refused, not
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
go vet ./...
NEMOSH_DIFFERENTIAL=strict go test -race -shuffle=on -count=1 ./...
```

plus a 250 pure-line ceiling per production `.go` file. `AGENTS.md` has the
details, including why green tests are not taken as evidence of correctness.

Contributions are under Apache-2.0 by section 5 of the license; there is no
separate CLA.

## Not planned

Recorded so the question is answered once rather than reopened.

**A shim per applet.** Scoop could create `cat`, `grep`, `find` and the rest
pointing at the same binary. It will not, and the reason is not effort.

Scoop's shims share one flat directory, so a name belongs to whichever package
wrote it last. BusyBox's own manifest shims every applet its `--list` reports; on
the machine this was written on that is **171 shims** already pointing at
`busybox.exe`, `cat`, `grep`, `find`, `sort` and `ls` among them. Nemosh has 160
applets and **148 of those names are busybox-w32's too**. Installing it with
per-applet shims would silently repoint them at a different implementation, and
uninstalling either package afterwards would take the other's names with it.
Anyone installing a Unix toolbox on Windows is likely to have BusyBox already,
which makes this the expected case rather than an edge one.

Two of the names are worse than merely contested: `find` and `sort` are real
Windows commands with unrelated syntax -- `find.exe` takes a quoted string and a
file -- so shadowing them breaks scripts that never asked for this shell. An
install that changes what a name means is worse than one that makes you type
`nemosh find`; `scoop shim add` covers the case where someone wants one anyway.

**Applet parity with BusyBox.** 160 applets on Windows against busybox-w32's
179, 148 of them by the same name, and `flock`, `watch`, `which`, `time`,
`timeout` and `kill` are builtins. The archivers, the compression formats but
`lzop` and `compress`'s `.Z`, the text tools, `awk`, `bc` and the whole networking
group busybox-w32 keeps are all in now; what stays out includes `vi`, `make`,
`man`, `crond` and `crontab`, `dpkg` and `rpm`, `inotifyd`, and busybox-w32's
`jn` and `drop`, whose names are left unregistered so PATH still finds a real one.
What matters more than the count is that an option this shell does not implement is
refused by name rather than doing something else, which `docs/support-matrix.md`
records applet by applet.

## License

Apache-2.0. See [`LICENSE`](LICENSE), [`NOTICE`](NOTICE), and
[`THIRD-PARTY-NOTICES.md`](THIRD-PARTY-NOTICES.md). Changes are listed in
[`CHANGELOG.md`](CHANGELOG.md); to report something exploitable, see
[`SECURITY.md`](SECURITY.md).

BusyBox and busybox-w32 are GPL-2.0 and are **not** included here. No BusyBox
code was copied, nothing of theirs is distributed, and what is taken from them
is behaviour — which copyright does not reach. What that means precisely, how a
behaviour is established, where the wording deliberately differs, and why this
is not a strict clean room, are all set out in
[`docs/design/reference-methodology.md`](docs/design/reference-methodology.md).
