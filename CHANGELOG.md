# Changelog

Notable changes, newest first. Kept from `v0.1.0`, the first tagged release;
v0's development history lives in the commit log, which is detailed enough that
restating it here would only add a second version of the truth.

Versions follow `AGENTS.md`: an exact `vMAJOR.MINOR.PATCH` tag is a release, and
every push to `master` publishes a `vX.Y.Z-master-<commit>` prerelease whose
patch number is the commits since that tag.

## Unreleased

### Interactive shell

- **Line editing.** A real terminal now gets arrows, history recall, Home/End,
  Delete, Ctrl-A/E/U/W/L, and the Meta word bindings readline defines
  (Alt-D, Alt-Backspace, Alt-B, Alt-F, Ctrl-Left, Ctrl-Right). A pipe or a file
  keeps the previous line-at-a-time path.
- **Ctrl-D and Ctrl-Z exit** on an empty line. With text in the buffer Ctrl-D
  deletes forward instead, so it cannot discard a half-typed line.
- **Tab completion** over builtins, applets, and file paths. A single candidate
  is inserted and followed by a blank, except after a directory; several insert
  only what they share and are listed.
- **Backspace over a wide character removes all of it.** The editor edits by
  rune and measures by column, which busybox's own editor conflates — there,
  backspacing over a two-column CJK character leaves half of it on screen.
- **`$ENV` is sourced** for an interactive shell, which is what POSIX specifies
  and what busybox reads. A machine already configured for busybox needs no
  change.
- **The default prompt has colour**, and `PS1` is expanded on every draw, so a
  substitution reporting a git branch or an exit code works.

### Builtins and applets

- `help` lists builtins and applets. Without it, `help` reached Windows'
  `help.exe`, whose console-code-page output arrives as mojibake in a UTF-8
  terminal.
- `history`, with `-c`.
- `id`, reporting privilege the way busybox-w32 does: `id -u` is 0 only when the
  process is elevated and the Administrators group is enabled in its token.
- `export` with no operands, and `export -p`, list exported variables. Both
  printed nothing before ([#10](https://github.com/xiongnemo/nemosh/issues/10)).
- `ls --color[=always|never|auto]`, matching busybox's escapes byte for byte.
  `auto` is resolved against the stream being written to, so an alias using it
  is safe to pipe.
- `set -o nocaseglob`, which matters more on Windows than elsewhere because NTFS
  does not distinguish case.
- `uname -r` and `-v` report the real version. They were hardcoded to `unknown`,
  so `uname -a` read `unknown unknown` in the middle.
- `nemosh --version`, `--list`, and `--help`. All three answered
  `invalid option` before.

### Fixed

- **A finished background job frees its slot.** A slot was released only by
  `wait`, and a script need never call it, so the 65th `foo &` in a session was
  refused with `job limit reached` and so was every one after it, permanently.
  BusyBox starts its 101st without complaint. The limit now bounds jobs that are
  still running; finished ones are swept when the space is needed, so `jobs`
  still shows them as Done below the limit.
- **An external command inherits the console.** Every child was being handed a
  pipe, which turned `help.exe`'s output into replacement characters and made
  anything checking isatty — colours, progress bars, pagers — turn itself off.
- **A wrapped line is redrawn correctly.** Past the terminal width the prompt's
  row is above the cursor, and the redraw was painting over the wrong rows.
- **A prompt is measured by what it draws.** Escape sequences were counted as
  columns, so a coloured `$ ` measured 11 instead of 2 and the first keystroke
  landed nine cells too far right.
- **`$?` reaches a command substitution.** `$(echo $?)` answered zero after a
  failure, which is how a prompt's failure indicator silently stopped working.
  Backquotes in a prompt are substituted now too.
- **A command name that cannot be a filename reports `not found`** rather than a
  raw `CreateFile` failure. Found by the parser fuzzer.
- `alias ..`, `alias ...` and `alias ~` are accepted. The name rule was the one
  for variables, which rejected the first three aliases anyone writes.

### Project

- Apache-2.0, with `NOTICE` and third-party notices.
- `docs/design/reference-methodology.md` states exactly what is taken from
  busybox and what is not, and says plainly that this is not a strict clean
  room.
- `docs/support-matrix.md`, measured against a built binary rather than read off
  the source.
- Every push to `master` publishes a prerelease with a checksum; an exact semver
  tag publishes a full release.
- `govulncheck` on every push and weekly; fuzzing over the parser and the
  pattern matcher, with the corpus checked in.
- Leak, stress and performance-baseline coverage: goroutine counts across
  repeated scripts and one long session, Windows handle counts across repeated
  redirects and pipelines, and allocation ceilings for parsing and running.
  These gate on counts rather than on wall-clock time, which flaps too much on a
  shared runner to be worth failing a build over; binary size is checked in CI
  for the same reason.
- Scoop manifests in
  [`xiongnemo/windows-binaries-scoop-bucket`](https://github.com/xiongnemo/windows-binaries-scoop-bucket).

## v0.1.0 — 2026-08-08

The MVP baseline: v0 complete and audited, taken once the shell, the applet
bundle, and the Windows path, launch, device, job and signal boundaries were all
in and the readiness ledger had been re-measured against a built binary.

See `docs/design/v0-readiness.md` for what that audit found and fixed — including
two defects a green test suite could not see, `times` reporting 215 years and a
brace treated as reserved outside command position.
