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

The class that broke. `bc` is confirmed working as of 2026-09-13 and `bc -l` as of
2026-10-06; the rest share the shape and each has its own input loop. The four
filters and `find -ok` are v1.4's -- a filter now flushes when its input runs dry
rather than at every line -- and have not been checked by hand yet.

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
| `bc -l` | `s(1)` and Enter, then `scale` | `.84147098480789650665` at once, then `20` |
| `tr a-z A-Z` (no file) | `hello` and Enter | `HELLO` at once |
| `cut -c1-3` (no file) | `hello` and Enter | `hel` at once |
| `head -n 2` (no file) | two lines | each back at once, and the prompt after the second |
| `sed s/o/0/` (no file) | two lines, then Ctrl-Z Enter | the first answer as the second line goes in: sed reads a line ahead to know the last, as busybox's does |
| `find . -name '*.txt' -ok echo {} \;` among a few `.txt` files | `y` or `n` and Enter at each `echo ./a.txt ?` | `y` runs it, `n` skips it |

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
- **Tab in the spellings added on 2026-11-15**, not yet checked by hand: `cd /c/Users/`
  and `ls /mnt/c/` list; `./` and Tab offers the scripts and programs beside you and not
  the text files; `ls "My Do` and Tab completes inside the quote and closes it after a
  file; `echo $HO` and Tab gives `$HOME/`; `sudo gi` and Tab gives `sudo git`.
- **A name not found**, not yet checked by hand: `gti status` at the prompt answers
  "did you mean `git`?"; with a `deploy.ps1` on PATH, `deploy` says how to run it.
- **Programmable completion**, confirmed 2026-10-06. After `source <(gh completion -s
  bash)`, `gh pr ` and Tab twice lists checkout, checks, close and the rest, and
  `gh pr ch` and Tab completes `check`, a second Tab listing checkout and checks. With
  git's `git-completion.bash` sourced, `git che` and Tab lists cherry, cherry-pick and
  checkout, and `git checkout ma` completes `master`.
- **Ctrl-R** reverse search, then Enter to accept and Ctrl-G to abort.
- **The kill ring**: Ctrl-W (word), Ctrl-U (line), Ctrl-K (to end), then **Ctrl-Y**
  to yank it back.
- **Ctrl-L** clears and redraws with the line intact, and `clear` clears. Both write
  `ESC[H ESC[J` since v1.4, so look at each in Windows Terminal and in a conhost
  window; confirmed 2026-10-06.
- **Ctrl-A / Ctrl-E** to line start and end.

**Worth watching for specifically, because the fix changed it:** raw mode is now
entered and left around *every single line read* rather than once per session. Type
fast, and type *while a command is still running* (start `sleep 5`, keep typing).
Keystrokes should not be lost, duplicated, or echoed strangely at the mode switch.

**Shell integration marks**, not yet checked by hand. In Windows Terminal's settings turn
`autoMarkPrompts` off, so a missing mark is not filled in by the terminal, and turn on
`showMarksOnScrollbar`. Then: `true`, `false`, an empty line, and Ctrl-C at a half-typed
line each leave a mark on the scrollbar, green after `true`, red after `false`, plain after
the other two; Ctrl-Up and Ctrl-Down (scrollToMark) jump between prompts; and
selectCommand and selectOutput take a command and its output. After `cd C:/Windows`, and
after `cd` to a directory with a space in its name, Duplicate Tab opens there. Ctrl-L, a Tab
listing and Ctrl-R leave the prompt where it was. `NEMOSH_SHELL_INTEGRATION=never` leaves
no mark, and in a conhost window, with `=always`, nothing is drawn of them.

**`title`**, not yet checked by hand. `title 构建中` names the tab in Windows Terminal, unless
the profile sets `suppressApplicationTitle`, and the window in a conhost one; `title` prints
it back. Beside busybox's `title`, `title a b` takes only `a` in both.

**`time {` over lines**, not yet checked by hand: type `time {` and Enter, and the
prompt is a continuation one; `sleep 1`, Enter, `}`, Enter, and the report comes,
`real 0m 1.0...`. The keyword over a compound is new since v1.4.0, and the prompt's
reading of where a group ends is a layer of its own.

## C. Interrupts and end of input

`os.Interrupt` on Windows is only delivered while `ENABLE_PROCESSED_INPUT` is set,
which raw mode clears -- this is half of what made `bc` look frozen.

- **Ctrl-C on an empty prompt**: new prompt, shell survives.
- **Ctrl-C part-way through a typed line**: line abandoned, shell survives.
- **Ctrl-C during `sleep 30`**: returns to the prompt promptly.
- **Ctrl-C inside `bc`**: behaves, and does not kill the shell with it.
- **Ctrl-D on an empty prompt**: the shell exits.
- **Pasting at the prompt**, not yet checked by hand: paste two lines, say `echo one`
  and `echo two`, and both appear on the line, the second on a row of its own, with
  nothing run until Enter; paste a line holding a tab and no completion list appears.
  Windows Terminal supports bracketed paste; in a classic conhost window check whether it
  does, and if not, that a paste runs line by line as it did before rather than going
  wrong some new way.
- **Ctrl-Z at the prompt**, not yet checked by hand: paste `cd C:\Users\nemo`, press
  Ctrl-Z, and the line reads `cd C:/Users/nemo` with the cursor where it was; on an empty
  line nothing happens. With `NEMOSH_CTRL_Z=eof` set in `~/.nemoshrc`, Ctrl-Z on an empty
  line exits, as it did until 2026-11-15. Check the key arrives at all in a classic conhost
  window as well as in Windows Terminal.
- **Ctrl-Z then Enter** as end-of-input to an applet reading stdin (the Windows
  spelling of Ctrl-D).
- **Closing the console window under a script**: run a script with
  `trap 'echo term >> log' TERM`, `trap 'echo exit >> log' EXIT` and a `sleep 20`, in a
  window of its own, and close the window. The log should say term, then exit. No test
  can close a console, so this is the only check that the trap survives Windows ending
  the process a few seconds later.

## D. Job control

- `sleep 20 &` then `jobs` -- `[1]+  Running`. A second `sleep 20 &` lists as `[2]+`
  first and `[1]-` after it; `kill %-` ends the first.
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

Since 2026-11-16 tcell links `third_party/gdamore-encoding`, whose character sets are
generated tables rather than built at start; its tests hold every byte and rune to
upstream's, but only a terminal shows the result. `top` and `less` should draw as before,
box lines and all. On a Unix terminal, run `top` once more under `LANG=C` and once under
an ISO-8859-1 locale, which are where tcell consults those tables at all.

An applet is handed a frame since then (`applet_frame.go`), which is its context and its
streams. Ctrl-C during `seq 1 100000000 > /dev/null` still has to stop it at once with
130, and `xargs -n1 echo < file` and `find . -exec echo {} \;` still run, since those reach
the shell through the frame.

`less` draws as its input arrives since 2026-11-16: `ping -t localhost | less` shows the
first replies at once and the bottom line says it is waiting rather than (END); `q` leaves
and the prompt comes back with ping gone. `ls --color=always | less` shows a reversed `[`
where each colour starts, and `ls --color=always | less -R` the names alone. A file with
Chinese in it lines up, the character after a wide one in its own column.

`cd D:` changes drive since 2026-11-16, on a machine with a second drive: `cd D:/somewhere`,
`cd C:/Windows`, then `cd D:` is back at D:/somewhere, `cd C:` stays, and `cmd /c "cd D:"`
prints `D:\somewhere`. `cd C:\Users` unquoted fails with a hint that the backslash was an
escape; `cd 'C:\Users'` works.

`nano +5 file` opens with the cursor on line 5, and `^K` there cuts line 5. With
`docs/examples/nemoshrc` as `~/.nemoshrc`, a new window opens without the pause gh's and git's
completion cost, and the first `git che` and Tab offers checkout and cherry-pick.

`nano` leaves as nano does since 2026-11-16. Next to the nano Scoop installs, press the
same keys in both on a file with a change: `^X` asks "Save modified buffer?", `Y` offers
the file's name, Enter writes it and leaves; `N` leaves without writing; `^C` at the
question goes back to the buffer. `^O` then Enter writes and adds no blank line. `^C` at
the buffer must **not** leave -- it shows the cursor's line and column. The tests drive
both spellings of Ctrl-C tcell can deliver; only a real console says which one it sends.

`watch -n 1 date`, confirmed 2026-10-06: the screen clears every second under
`Every 1.0s: date`, the date at its right edge. Widen or narrow the window and the
date moves to the new edge at the next run; one that stays near column 60 means the
width was not read. Ctrl-C gives the prompt back at once, and `$?` is 130.

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
- **A program an applet runs**, not yet checked by hand. `git ls-files | head -3 |
  xargs git log -1 --oneline --` and `find . -maxdepth 1 -name '*.md' -exec git log -1
  --oneline -- {} \;` print in git's colours, as git run at the prompt does, since the
  program is handed the console; and Ctrl-C during `echo 127.0.0.1 | xargs ping -n 30`
  gives the prompt back as soon as ping has printed its statistics. Until 2026-10-08
  each of these was `not found`.
- **Ctrl-C reaches the program at the prompt**, not yet checked by hand. `ping -t
  localhost` and Ctrl-C prints ping's statistics, then the prompt. In a nested `cmd`,
  `pwsh` and `python`, Ctrl-C during something long (`ping -t localhost`,
  `Start-Sleep 30`, `import time; time.sleep(30)`) stops that and leaves you in cmd,
  pwsh or python, where it closed them before. `notepad` is still ended by Ctrl-C,
  which notepad never sees. `NEMOSH_CTRL_C=kill` puts back busybox's way, so the
  nested cmd closes again; in `nemosh -c 'ping -n 30 localhost; echo after'` Ctrl-C
  ends ping without its statistics and `after` is not printed, unless
  `NEMOSH_CTRL_C=program`.
- **A `#!/usr/bin/env` script**, not yet checked by hand. A file `t.py` holding
  `#!/usr/bin/env python` and `print("hi")`, run as `./t.py`, runs the Python PATH
  finds -- and a Python that is only Windows' Store alias may open the Store instead,
  which is the thing to look for. With Git installed, a `#!/usr/bin/env bash` script
  runs Git's bash, and `echo $BASH_VERSION` in it says so. Until 2026-10-07 both were
  `env: NAME: not found`.
- **A path handed to a native editor**, not yet checked by hand at a prompt.
  `nvim ~/.ssh/config`, `code ~/.gitconfig` and `notepad "$(realpath README.md)"` open
  the file, and the prompt's `\w` still shortens home to `~`. Until 2026-10-19 the
  shell reported `/c/Users/...`, and nvim opened an empty `\c\Users\...` on the
  current drive; `NEMOSH_PATHS=msys` still does that, which is the thing to compare.
  Measured through a script with the built binary on 2026-10-19: `readable=1` for
  `~/.ssh/config` and for `$(realpath ...)`, `readable=0` under `NEMOSH_PATHS=msys`.
- **CJK input** typed at the prompt, and a CJK filename completed with Tab. The
  editor decodes multi-byte runes incrementally, so a character split across two
  reads is the interesting case.

## G. Two windows at once

- **`flock`**, confirmed 2026-10-06. In one window `flock "$TEMP/demo.lock" sleep 20`;
  in another, `flock -n "$TEMP/demo.lock" echo got; echo $?` prints `1` alone, and
  `flock "$TEMP/demo.lock" echo got` waits until the first is done, Ctrl-C ending the
  wait at once with 130. A busybox-w32 flock is no test of it: the two lock different
  bytes of the file, and do not keep each other out.

## If something fails

Capture, in this order: what you typed, what appeared, and `nemosh --version`. For
anything terminal-shaped, `stty` before and after is usually the whole story --
a mode that came back different is the signature of the bug this file exists for.
