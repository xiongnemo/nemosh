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
- **Completion knows what a command can take.** `cd`, `mkdir` and `rmdir` offer
  directories only, so `cd al` completes to `alpha/` instead of stopping at the
  `alp` it shares with `alpine.txt`. busybox has this rule for `cd` alone and
  spells it out inline; here the command name is looked up.
- **A completed name is usable.** Blanks and shell metacharacters are escaped on
  insertion and unescaped before matching, so `cd Prog` produces
  `cd Program\ Files/` — one operand — and a second Tab continues from it.
- **Completion ignores case on Windows**, because NTFS does. `cd prog` offers
  `Program Files/`, spelled as it is on disk.
- **Inline suggestion.** What the line would most likely become is drawn ahead of
  the cursor in grey, from history first and command names second, and Right or
  End takes it. It is never in the buffer, so Enter submits what was typed and
  could not do otherwise; it is cut to the columns left on the row, so it can
  never wrap; and it is only offered at the end of a line.
- **The line is drawn in colour.** A command this shell carries is green and one
  it does not is red; an option the command accepts is cyan and one it does not
  is yellow; the word being edited is underlined until a blank ends it. Every
  choice is in one struct, `defaultPalette()`.
- **Colour absent turns both off** rather than degrading them, because a grey
  suggestion rendered as ordinary text would put characters on screen that are
  not in the line. `NO_COLOR`, `TERM=dumb`, and `NEMOSH_COLOR=always|never`.
- **Option completion.** `ls -<TAB>` offers `--color -1 -a -h -l`, from
  `internal/capability` -- one table that both completion and the renderer read,
  bound to the applets' real behaviour by a test that runs each of them. Writing
  that test found `id` credited with a `-U` it does not have, and `chmod` and
  `sed` documented as refusing unknown options when they do not.
- `docs/design/completion.md` records how ash, bash and zsh each solve this and
  which of their ideas this shell took; `docs/design/suggestion.md` covers the
  suggestion engine and the rendering.
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
- `ls -1`, which busybox has and which is among the most-typed options there is.
  This `ls` always writes one entry per line, so `-1` asks for the format
  already in use; `-C` stays refused, because columns are genuinely absent.
- `grep --color[=WHEN]` is accepted and ignored, which is exactly what busybox
  does (its option table maps it to a pseudo-flag nothing reads). Refusing it
  broke `alias grep='grep --color=auto'` -- a line in almost every rc file --
  and only interactively, since `$ENV` is not read for `-c`.
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

- **File completion works in a real session at all.** The editor was handed the
  shell's own view of the working directory — `/c/Users/...` on Windows — which
  `os.ReadDir` cannot open, so completing a filename found nothing from the
  first prompt onwards, however correct the rest of it was. Every test built the
  editor with a native path, so the two vocabularies never met.
- **An operator where a word belongs no longer crashes the shell.**
  `for i in a|b; do :; done` panicked on a nil dereference, taking the session
  with it, and so did `case | in x) :;; esac` and a redirect inside a case
  pattern. Found by the parser fuzzer's exploration run; all three now answer
  `syntax error: unexpected |`, and the inputs are seeds from here on.
- **`rm` finishes the job and says what stopped it.** A failure on one operand
  abandoned every operand after it, so `rm -rf a b c` with one file in use left
  `c` in place and named nothing but `a` — the shape a Windows cleanup script
  hits whenever something is still running. It now continues, as POSIX and
  busybox do, and reports every failure. The diagnostic names the file rather
  than the operand (`cannot remove 'b/held.exe'`, not `'b'`), and a directory
  left non-empty by a failure below it is not reported a second time.
- **`rm` refuses a directory without `-r`.** It deleted an empty one, because
  `os.Remove` unlinks a directory without complaint — so the shell was more
  destructive than the reference it follows. `rm d` and `rm -f d` now both
  answer `rm: 'd' is a directory` and exit 1, as busybox and POSIX do; `-f`
  never excused it there either.
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
