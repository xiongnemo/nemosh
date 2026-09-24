# Manual Checks

What a person at a real Windows Terminal has to verify, because the suite
structurally cannot.

Nemosh has around 3,000 tests and they said nothing at all about the shell being
unusable by hand: every one of them fed input through a `strings.Reader`, which is
indistinguishable from a session that has already ended, so a prompt that held the
terminal in raw mode across every command it ran was invisible to all of them. That
bug is fixed. This list exists so the *class* is checked on purpose rather than
discovered by typing `bc` one evening.

Work down it after any change to `cmd/nemosh`'s session, the line editor, or an
applet that reads keys or lines.

## What is already covered -- do not spend time here

Measured, not assumed:

- **Raw mode entering and leaving correctly.** `raw_mode_console_windows_test.go`
  opens the real `CONIN$`, checks raw mode clears the three bits a command needs and
  that restore puts the mode back bit for bit. It skips where there is no console,
  which is every CI runner, but it **runs on the development machine** -- so
  `go test ./cmd/nemosh/` already covers it here.
- **Who is allowed to touch terminal state**, by `terminal_ownership_test.go`, and
  **for how long**, by `raw_mode_scope_test.go`.
- **Streaming through a pipe.** Verified by timestamping arrivals: `awk`, `tr`,
  `grep` and `cat` each emit a line per second from a one-line-per-second source
  rather than holding it all to the end. A console and a pipe take the same branch
  in `writerIsRegularFile`, so the terminal half needs no separate check.
- **Full-screen applet logic** -- `nano`, `micro`, `less`, `top` -- against tcell's
  simulation screen. What that cannot see is the real terminal: colour, width,
  resize, and whether Windows Terminal delivers the keys at all.

## A. Applets that read from the terminal

The class that broke. `bc` is confirmed working as of 2026-09-13; the rest share the
shape and each has its own input loop.

For each: start it at the prompt, type the input, confirm the answer comes back
**before** you send end-of-input.

| Type | Then | Expect |
| --- | --- | --- |
| `dc` | `5 3 + p` and Enter | `8` |
| `ed` | `a`, a line, `.`, `1p` | the line back |
| `cat` (no operands) | `hello` and Enter | `hello` echoed back at once |
| `grep hello` (no file) | `hello there` and Enter | the line back at once |
| `sort` (no file) | two lines, then Ctrl-Z Enter | both lines, sorted |
| `wc` (no file) | a line, then Ctrl-Z Enter | the counts |

**Broken looks like:** nothing echoes as you type; Enter does nothing; the answer
only appears after Ctrl-Z; or the applet ignores Ctrl-C.

Two answers that look wrong and are not, both checked on 2026-09-13: in `ed`, `q` on
a modified buffer answers `?` and wants a second `q` -- that is ed, and `Q` quits
outright. And the expected outputs above are what the applets really give when fed
the same input down a pipe, so a difference at the terminal is about the terminal.

`stty` is its own case -- it is *supposed* to change the terminal. Run `stty` alone
for a report, then confirm the prompt still echoes normally afterwards.

## B. The line editor, in Windows Terminal specifically

The editor decodes ANSI escape sequences (`lineedit_key.go`). Whether Windows
Terminal delivers them as expected is exactly what cannot be tested from here.

- **Arrows** left/right to move, up/down through history.
- **Home/End**, and **Ctrl-Left/Ctrl-Right** for word movement.
- **Delete** and **Backspace**.
- **Tab** completion: a command name, a path, a path containing a space, and a path
  with CJK characters in it. Twice in a row should list rather than beep forever.
- **Ctrl-R** reverse search, then Enter to accept and Ctrl-G to abort.
- **The kill ring**: Ctrl-W (word), Ctrl-U (line), Ctrl-K (to end), then **Ctrl-Y**
  to yank it back.
- **Ctrl-L** clears and redraws with the line intact.
- **Ctrl-A / Ctrl-E** to line start and end.

**Worth watching for specifically, because the fix changed it:** raw mode is now
entered and left around *every single line read* rather than once per session. Type
fast, and type *while a command is still running* (start `sleep 5`, keep typing).
Keystrokes should not be lost, duplicated, or echoed strangely at the mode switch.

## C. Interrupts and end of input

`os.Interrupt` on Windows is only delivered while `ENABLE_PROCESSED_INPUT` is set,
which raw mode clears -- this is half of what made `bc` look frozen.

- **Ctrl-C on an empty prompt**: new prompt, shell survives.
- **Ctrl-C part-way through a typed line**: line abandoned, shell survives.
- **Ctrl-C during `sleep 30`**: returns to the prompt promptly.
- **Ctrl-C inside `bc`**: behaves, and does not kill the shell with it.
- **Ctrl-D on an empty prompt**: the shell exits.
- **Ctrl-Z then Enter** as end-of-input to an applet reading stdin (the Windows
  spelling of Ctrl-D).
- **Closing the console window under a script**: run a script with
  `trap 'echo term >> log' TERM`, `trap 'echo exit >> log' EXIT` and a `sleep 20`, in a
  window of its own, and close the window. The log should say term, then exit. No test
  can close a console, so this is the only check that the trap survives Windows ending
  the process a few seconds later.

## D. Job control

- `sleep 20 &` then `jobs` -- `[1] Running`.
- **`$!` is a process**: `sleep 30 &`, then `tasklist /FI "PID eq $!"` names it, and
  `taskkill /PID $! /F` ends it, after which `wait $!` answers.
- **`kill -0` in a loop**: `sleep 3 & p=$!; while kill -0 $p 2>/dev/null; do echo alive;
  sleep 1; done` prints alive about three times and stops.
- **`trap TERM` inside a job**: `( trap 'echo got-term' TERM; sleep 2; echo after ) &
  sleep 1; kill $!` -- got-term, then after, and `wait $!` answers 0. The `sleep 1` is
  what gives the job time to set its trap: a `kill` straight after the `&` reaches it
  first, and it ends by the signal, as it does in bash.
- **The way back**: `NEMOSH_JOBS=goroutine nemosh -c 'sleep 1 & echo $!'` prints `%1`.
- `wait` -- returns when it finishes.
- `kill %1` on a running background job.
- `fg` and `bg` **should refuse loudly**, with a reason and a pointer to the support
  matrix. Nothing here can suspend a job, so that refusal is correct, not a bug.

## E. Full-screen applets on a real terminal

`less` on a long file, `top`, `nano` and `micro` on a file. For each:

- keys arrive (arrows, Page Up/Down, `q` or the editor's quit key),
- colour renders rather than printing escape sequences literally,
- **resizing the window** redraws instead of corrupting,
- and on exit **the terminal is given back**: the prompt echoes normally and Ctrl-C
  still works. That last one is the same bug class as A.

## F. Windows entry points

These only exist off a GUI launch or a real clipboard, so no test reaches them.

- **`su`**, which is the only thing that exercises the other two. An elevated shell
  gets a console of its own that dies with the process, so `su` launches the child
  with `--attach-console PID` to join this one and `-N` to hold it open at exit.
  Neither is a thing to type: `--attach-console` names a console a hand-typed PID
  has no business joining, and it is deliberately absent from `--help`. Run
  `su -c 'ls'` and check the output is still readable rather than gone with the
  window.
- **`/dev/clipboard`**: `echo hi > /dev/clipboard`, then paste somewhere; and
  `cat /dev/clipboard` after copying text elsewhere. Note this overwrites whatever
  you had on the clipboard. It is also known to fail at random when a clipboard
  manager or Windows clipboard history is holding the clipboard -- see AGENTS.md.
- **Launching real programs**: an `.exe` that wants the console, since that is the half
  a pipe cannot stand in for. The rest of this was measured on 2026-09-15 and needs no
  hand: a `.bat` through ComSpec, a path with spaces in it, and a 522-character path
  all worked through `-c`, which uses the same launching and the same path handling.
- **CJK input** typed at the prompt, and a CJK filename completed with Tab. The
  editor decodes multi-byte runes incrementally, so a character split across two
  reads is the interesting case.

## If something fails

Capture, in this order: what you typed, what appeared, and `nemosh --version`. For
anything terminal-shaped, `stty` before and after is usually the whole story --
a mode that came back different is the signature of the bug this file exists for.
