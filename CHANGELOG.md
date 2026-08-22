# Changelog

Notable changes, newest first. Kept from `v0.1.0`, the first tagged release;
v0's development history lives in the commit log, which is detailed enough that
restating it here would only add a second version of the truth.

Versions follow `AGENTS.md`: an exact `vMAJOR.MINOR.PATCH` tag is a release, and
every push to `master` publishes a `vX.Y.Z-master-<commit>` prerelease whose
patch number is the commits since that tag.

### Fixed

- **`ftpput` had its two file names reversed.** `ftpget HOST [LOCAL] REMOTE` and
  `ftpput HOST [REMOTE] LOCAL` are mirrors -- in both, the file being *written* is
  named first -- and `ftpput` read the remote name off disk and uploaded it under the
  local one. With a single file operand the two names collapse to the same string,
  and every test that existed passed one operand or none, so nothing caught it until
  the applets were run against an FTP server the tests start.

### Tested

- **Three network applets had no end-to-end test, and `bunzip2`/`bzcat` had none at
  all.** Worth listing rather than quietly filling in, because the gap is the
  interesting part: `ftpget` and `ftpput` were covered only by a unit test on the
  passive-mode reply parser, `whois` only by its server-selection table,
  `ssl_client` not at all, and `bunzip2`, `bzcat` and `tar -j` by nothing -- the
  compression test file names them in its header comment and then tests only gzip.
- The FTP tests now run against a written-out server answering USER, PASS, TYPE,
  PASV, RETR and STOR, whose greeting is deliberately multi-line so that a client
  reading only the first line of a reply goes out of step and fails.
- `ssl_client` is asserted in the only direction its own design allows. A Go test
  server's certificate is signed by a throwaway CA no trust store knows, so a
  *successful* handshake would need the verification-skipping option this build
  refuses to have; the refusal is the property, so the refusal is the test. A second
  test drives the SNI name through and checks the server saw what `-n` gave.
- The bzip2 fixtures are byte literals from busybox, because Go has no bzip2 writer
  -- the same fact that keeps the `bzip2` name unregistered here -- so the input
  cannot be produced by the code under test.

## Unreleased

### Added

- **`cpio` and `ar`**, the last two of the archive set and the two with no library
  behind them: Go has no package for either format, so both headers are read and
  written here byte by byte. Both go through the same containment helper `tar` and
  `unzip` use, and both are run against the whole hostile-name table -- a shared
  helper is only shared if every caller reaches it.
- `cpio` archives a *list* rather than a tree, which is why it still exists:
  `find . -name '*.go' | cpio -o -H newc` takes exactly the names on stdin. `-0`
  reads `find -print0`'s output through the same splitter `xargs -0` uses. Only
  `newc` and its CRC variant are read, and the magic is checked *before* the rest of
  the header so an `odc` archive is named rather than reported as truncated.
- `ar`'s operation is a verb, and **only the first word is examined** -- which is a
  fix, not a preference. Scanning every argument meant a Windows temporary path,
  which contains a `p` in `AppData`, was read as the verb, had a letter removed from
  its middle, and came back to the option parser as `-C:\Users\...`. Every letter of
  that first word must also be one `ar` knows, so `ar libtest.a` refuses on its `l`
  the way GNU does instead of finding the `t` in the file name.
- `ar r` creates and refuses an existing archive rather than half-appending to it; a
  name too long for the sixteen-column header is refused and the partial archive
  deleted; a member whose name cannot be resolved is skipped rather than fatal,
  since aborting would cost every honest member after it. The long-name table is
  read, because a `.deb` has one.
- Verified in both directions against busybox-w32 v1.38.0: `cpio -t`, `-tv`, `ar t`,
  `ar tv` and `ar p` all match byte for byte, including cpio's `N blocks` line, and
  each build reads the other's archives.

- **The networking group, all seven of it**: `wget`, `nc`, `whois`,
  `ssl_client`, `httpd`, `ftpget` and `ftpput`. busybox-w32 keeps only these seven
  out of the dozens busybox has, because Windows lacks the APIs for the rest, so
  this is the whole group rather than a selection from it. Stock Windows has
  `curl.exe` and none of the others.
- **TLS is native**, so `wget` needs no `ssl_client` in its pipeline the way
  busybox's does. `ssl_client` is still here for what the name means -- `openssl
  s_client` without openssl -- and `wget` does not use it.
- **A name the server chose is checked like a tar entry.** `wget` takes its output
  name from the URL, and a redirect chooses that URL; `httpd` takes its path from
  the request. Both go through `safeArchivePath`, the same helper `tar` and `unzip`
  use, so neither can write outside the working directory, reach a Windows device
  name, or land on a `..` after normalisation.
- `httpd` binds **127.0.0.1** unless `-a` says otherwise, where busybox binds every
  interface, and runs **no CGI**. `nc -e` is refused by name: running a program on a
  connection is a remote shell. FTP is passive-only, because active mode asks the
  server to connect back and nothing behind a router or a Windows firewall can
  accept that.
- `wget --header` is repeatable and `--spider` is a real HEAD rather than a GET
  with the body thrown away. These are the first long options in
  `internal/capability` that take a *detached* value, so `Command.ValueLong` was
  added alongside `ValueShort` and completion now knows the word after
  `wget --header` is nobody's operand.
- **`completions/wget.toml` was removed.** Its own comment said the point of the
  file was that one name can be two programs; a nemosh `wget` makes it three, and
  `internal/capability` -- which a test binds to behaviour by running the applet --
  now answers for the name. A second, unverified description of the same command is
  the one thing the completions rule forbids.
  `NEMOSH_OVERRIDE_APPLETS=wget` still reaches a real `wget.exe`, and
  `scripts/completions/generate.py wget` writes a spec for the one installed.
- **`nc` waits for the reply.** Returning as soon as either direction finished
  looks symmetric and is not: for a request and a response the write side always
  finishes first, so `nc` exited before reading a byte. Go's `select` chooses
  uniformly among ready cases, so the test that covered it was a coin flip per run;
  a manual `nc` against this build's own `httpd` is what showed the empty output.
- Two more bugs worth naming, because neither is about networking. **`<-ctx.Done()` in a
  goroutine leaks that goroutine forever** when the context is never cancelled: an
  uncancellable context's `Done()` is a nil channel and receiving from one never
  returns. The capability drift test found it by running `httpd -h DIR` with
  `context.Background()`, which also **bound port 8080 and served until the package
  timed out ten minutes later** -- so `httpd` joins `su` in the set of applets that
  test must not run, with its option claims measured by a test that gives it a
  second operand instead.

- **Seven more checksum applets**: `sha1sum`, `sha384sum`, `sha512sum`, `sha3sum`,
  `cksum`, `crc32` and `sum`. `md5sum` and `sha256sum` were the only two, and a
  clean Windows machine has none -- which is most of why anyone reaches for a
  checksum tool. 58 of 58 measured forms agree with busybox byte for byte, over
  six inputs including 100 KB of random data.
- `sha3sum` defaults to **224** bits, which is what busybox does; `-a` takes 224,
  256, 384 or 512 and refuses anything else rather than rounding to a width SHA-3
  has. SHA-3 is a different function from SHA-2, not a truncation of it, so all
  four widths were cross-checked against Go's `crypto/sha3` as well.
- `cksum` is not `crc32`: POSIX's CRC is a different polynomial walked the other
  way round, with the file length fed through afterwards and the result
  complemented. It cannot come from `hash/crc32`, whose tables are all reflected.
- `sum` carries both historical algorithms, which disagree for the same file:
  BSD rotates its accumulator and counts 1024-byte blocks, System V folds a byte
  total twice into sixteen bits and counts 512-byte blocks.
- One divergence: `sum` omits the name for a single operand and prints no
  trailing space, which is GNU's output. busybox prints a stray trailing space
  there.

- **`sed` is now essentially complete**: `{}` blocks, `y///`, `=`, `a`, `i`, `c`,
  the hold space (`h H g G x`), the multiline commands (`n N P D`), branching
  (`b t T` with `:` labels), `-f` and `-i`. Blocks group commands
  under one address, so `sed -n '/x/{p;q}'` works; `d` and `q` inside one end the
  whole cycle rather than just the block. `y///` transliterates by rune, not byte,
  and **refuses unequal lengths** where busybox silently ignores the unpaired
  characters. `-f` takes the script from a file. `-i` edits in place, `-i.bak`
  keeps the original, and each file is its own stream -- line numbers restart and
  an address range does not leak into the next file, which is GNU's behaviour
  where busybox leaks it.
- `a`, `i` and `c` append, insert and replace. Their text is the one undelimited
  argument in sed, so a `;` inside it is text rather than a separator; `-n` does
  not suppress it, since it belongs to the script rather than the line; `a`
  survives a `d` that discards the line; and `c` on a range prints once as the
  range closes rather than once per line.
- The hold space and branching are what make sed more than a line filter, and
  every classic one-liner now works: `sed -n '1!G;h;$p'` reverses a file,
  `sed ':a;N;$!ba;s/
/ /g'` joins one, `sed -n 'N;P;D'` slides a two-line window.
- Branching forced the command tree to be flattened into one instruction list,
  because a label can sit inside a block a jump comes from outside and a recursive
  walk gives such a jump nowhere to land. That is what sed itself does. `D` uses
  the same machinery: it restarts the script without reading a line.
- `-i` had been deferred on the grounds that rewriting a file forces a choice of
  output encoding. It does not: sed here is byte-exact, so the bytes written back
  are the bytes read, transformed. That question arrives only if sed starts
  *decoding* UTF-16 on input, and stays deferred until then.

### Fixed

- **A lone `-` operand now means standard input.** POSIX gives it that meaning for
  every utility taking file operands, and it is how a script mixes a stream into a
  list of files -- `cat header.txt - footer.txt`. Eleven applets answered
  `No such file or directory` instead: `cat`, `head`, `tail`, `wc`, `grep`, `sed`,
  `sort`, `nl`, `rev`, `base64` and the checksums. `cut`, `uniq`, `paste` and
  `comm` each had their own private check for it; there is now one shared seam.
- Closing that operand does not close the shell's stdin, and the wrapper forwards
  cancellation rather than hiding it behind an `io.NopCloser` -- otherwise `cat -`
  alone would have stopped being interruptible.
- **`head` and `tail` carry on past an unreadable operand** instead of stopping at
  it, so `head -n1 a.txt nosuch b.txt` no longer silently drops `b.txt`. Status is
  1 either way.
- **`head`/`tail` no longer print a `==> name <==` header for a file they then
  fail to open**, which put the header above the error saying the file was missing.
- **A `-` operand in a `head`/`tail` header is spelled `standard input`**, which is
  what GNU prints and what busybox's own `head` prints. It said `-`, which reads as
  a file of that name. busybox's `tail` says `-` too, so this follows the
  consistent answer rather than the reference's inconsistency -- as it already
  does for the header rule itself, which is POSIX's and keys off the number of
  operands named rather than the number opened.
- **`sed s///i` folds case.** busybox has it, and this refused it incoherently: the
  flag splitter consumed the letter and the flag parser then rejected it, so the
  two halves of one parser disagreed about which flags exist.
- **`sed ''` is a valid no-op**, as it is in every reference. Refusing an empty
  script made `sed "$expr" file` fail whenever the variable was empty.
- **`sed` refuses line address 0 by name.** It reported `unsupported command 0`,
  blaming the command for a bad address. GNU's wording is used; busybox instead
  lets `0` parse as *no* address, so its `sed -n '0p'` prints every line.

## v1.1.0 - 2026-08-22

The applet option matrices `v1-scope.md` deferred to v1.1, chosen by measuring
what a script actually reaches for and finds missing rather than by working down
a list. Every expectation was checked against busybox-w32 v1.38.0, and the
per-applet counts below are forms that now agree with it byte for byte, exit
statuses and diagnostics included.

One thing that did *not* need doing is worth recording: the shell language is
finished. Of 46 constructs probed, 45 work — including process substitution,
`${!x}` indirection, arrays, `getopts`, `{1..3}`, `set -C` and arithmetic `for`.
The only absentee is `select`, which `v0-readiness.md` already argues is three
lines someone can write. Every remaining gap was in an applet.

### `find` — the operators

- **`-a`, `-o`, `!`, `-and`, `-or`, `-not` and parentheses**, with POSIX
  precedence. Without them `find` was a single-predicate filter rather than find:
  `find . -name a -o -name b` is a day-one idiom and it was refused.
- `!` was worse than refused. It does not begin with a dash, so path collection
  took it as a *path operand* and `find . ! -name x` answered
  `!: No such file or directory` — blaming a file for an operator. Path
  collection now stops at `!`, `(` and `)`.
- Where busybox is lax, this is not. `find . )` prints the whole tree there and
  then complains; an unpaired paren, a dangling `-o` and an empty group are all
  refused here, as GNU refuses them.
- **New tests:** `-iname`, `-path`, `-ipath`, `-size`, `-mtime`, `-newer`,
  `-empty`. **New actions:** `-print0`, to pair with `xargs -0`. **New global
  options:** `-maxdepth` and `-mindepth`, which bound the traversal rather than
  filter it, so `-maxdepth 1` stops the walk reading a subdirectory at all.
- Two deliberate divergences, both measured. `-size` divides by the unit and
  rounds up, which POSIX states outright and GNU does for every suffix, where
  busybox compares raw bytes against `N*unit` — so busybox's `-size 1k` finds
  only a file of exactly 1024 bytes. And `-newer` keeps NTFS's full timestamp
  precision where busybox truncates to whole seconds.
- `-exec` and `-delete` stay refused: one needs the execution model and quoting
  rules, the other a decision about a destructive default. 24 forms agree.

### `ls` — sorting and descending

- **`-t`, `-S`, `-r`, `-R`, `-d`, `-F` and `-A`**, all previously refused by
  name, which meant `ls -ltr` failed on its options.
- Three rules were measured rather than chosen: the **last** sort option wins, so
  `ls -S -t` orders by time; **`-a` beats `-A`** in either order, where GNU lets
  the later one win; and the name is the tie-break for every key, with `-r`
  reversing the tie-break too, so two listings of an unchanged directory cannot
  differ.
- `-R`'s header path is built from the operand as spelled, so `ls -R .` says
  `./sub` exactly as `find .` says `./a.txt`, with a forward slash even on
  Windows. That settles a question a golden case had held open since the uutils
  port, whose Windows branch asserts a backslash.
- 18 forms agree. The 19th, `ls -d -l` on a directory, differs only by the mode
  bits and link count that `ls -l` already differed by. `-i`, `-n`, `-u` and `-c`
  stay refused.

### `head` and `tail` — attached values and file headers

- **`head -n2` works.** It was refused while `head -2` and `head -n 2` both
  worked — the worst shape a gap can take, since a user cannot predict which of
  three spellings the shell has. The cause was two option parsers in one package;
  head and tail were on the one that matches whole argument strings and so cannot
  express a letter carrying a value. Also `-c2`, `-n+2`, `-c+3`, `-n-2`.
- **More than one file operand now gets a `==> name <==` header.** There were
  none, so `head *.log` gave lines with no way to tell which file each came from.
  `-q` suppresses them, `-v` forces one for a single file, and stdin gets none.
- Diagnostics match the reference to the character: `head -n2c` answers
  `invalid number '2c'` where this said `invalid count: 2c`. 24 forms agree.

### `grep` — context lines

- **`-A`, `-B`, `-C`**, with `--` between non-contiguous groups. `-B` is why this
  needs state: whether a line is context is not known when it is read, only once
  a later line matches.
- Measured rules: `-A0` prints no separator at all, even between distant groups,
  where GNU does print one; and the separator **spans files**, so the printer's
  state has to outlive a single file. `-c`, `-l`, `-L` and `-q` ignore context,
  but `-o` does not.
- **`-e` and `-f`** supply patterns, so several can be given and a pattern
  starting with a dash stops looking like an option. Each is escaped and anchored
  separately before being joined — `-F -e a.c` escaped as one string would escape
  the `|` doing the joining, and `-x -e a -e b` anchored as one alternation would
  match any line containing `b`.
- **`-L`** is `-l` inverted, and inverts the exit status with it. 30 forms agree.
  `--include`, `--exclude` and `-z` stay refused.

### `sed` — addresses

- **Addresses**: `N`, `$`, `/re/`, the ranges `N,M` and `/a/,/b/`, and a trailing
  `!`. **Commands**: `p`, `d`, `q` beside the existing `s///`, separated by `;` or
  a newline. **Options**: `-n`, `-e`, `-E`/`-r`. None of it existed — `sed -n` was
  refused as an unsupported *script*, the first argument being assumed to be one.
- One measurement made the old shape unsalvageable rather than incomplete:
  **several file operands are one stream.** Line numbers run on across the
  boundary and `$` is the last line of the last file, so `sed -n '3p' f1 f2`
  answers the third line overall. sed was applying the script per file, which did
  not matter while it had no addresses and would have been silently wrong the
  moment it had.
- `$` needs one line of lookahead, not the whole input: sed is a filter, and
  reading a log into memory to find where it ends is not what a filter does.
- 30 forms agree, including `no address after comma` and `unmatched '/'`.
- `-i` stays refused, and it is the one worth naming: it has to choose an
  **output** encoding for the file it rewrites, which is the same decision already
  deferred for sed's UTF-16 reading, and settling it inside a convenience flag
  would settle it by accident. `-f`, `a`/`i`/`c`, `y///`, `{}`, hold space and
  branching stay refused too.

### Devices

- **Windows only.** Linux and macOS have a real `/dev`, so those builds use it and
  this shell provides nothing under it -- a synthetic eight-entry directory would
  shadow the machine's own devices. Held by a pair of tests that fail on opposite
  platforms rather than by a comment.
- The exception is `/dev/stdin`, `/dev/stdout`, `/dev/stderr` and `/dev/fd/N`,
  which every platform gets: they name this shell's descriptors rather than
  hardware, and after a redirect those are not the process's.
- **`/dev` is a directory.** `ls /dev` lists the devices, `echo /dev/*` expands,
  `/dev/<TAB>` completes, and `find /dev`, `du -s /dev` and `grep -r /dev` all
  work. busybox-w32 answers `No such file or directory` for `ls /dev`; listing is
  a deliberate divergence, because without it the only way to learn which devices
  exist is to read a document.
- **A device is observable, not only openable.** `test -e /dev/null` was false
  while `cat /dev/null` worked, and `ls -l /dev/null` refused the path. Both now
  answer, and the long listing matches busybox to the column -- `crw-rw-rw-`, the
  current user, and the major and minor numbers where a size would be.
- `find -type c` selects character devices, which the shell can now produce.
  `realpath /dev/../dev/zero` answers `/dev/zero`.
- **`grep -r` never reads a device.** `/dev/zero` returns bytes for ever, so a
  recursive grep that read it would not return; GNU grep skips devices when
  recursing for the same reason. A device named directly is still read.
- `cd /dev` is refused with the reason: a working directory needs a native form
  and `/dev` has none. The message no longer says "not a directory", which would
  contradict `test -d /dev`.
- One list drives opening, stat and listing, so `test -e /dev/zero` cannot
  disagree with `cat /dev/zero`. See `docs/design/device-filesystem.md`.

### Interactive shell

- **`~/` completes.** Tab offered nothing for a tilde: completion works on the
  text as typed, tilde expansion belongs to word expansion, and there was no
  route between them, so `~/` was read as a directory literally called `~`.
  `~` alone completes to `~/`; `~user` still offers nothing, because this shell
  does not resolve another account's profile directory.
- What Tab inserts stays spelled `~/` rather than being rewritten to an absolute
  path, which is what bash does. The escaping that protects a blank in
  `Program Files` no longer escapes that leading tilde -- it did, which produced
  a line that looked completed and could not run.

### Process monitor

- **F1 opens a panel instead of writing a line in the status bar.** The line was
  enough to remind someone of a binding they already knew and no use at all for
  the terms, which is what people actually ask about: the headers are four letters
  each and several name Windows concepts with no POSIX counterpart.
- **Every column now carries a one-line explanation**, kept beside the column
  itself so the legend cannot fall behind the table, and the panel lists all of
  them rather than only the ones the current window is wide enough to show. RSS,
  PRIV and COMMIT are three different memory numbers and now say so.
- The panel also explains the colours, and what is deliberately absent with the
  reason -- load average, TTY, nice values, disk-only IO, a gentle kill signal.

### Fixed

- **Reading `/dev/clipboard` immediately after writing it could fail.** Windows 10
  and later run a clipboard history service that opens the clipboard on every
  change, so the moment just after a write is when a read is most likely to lose
  the race. Measured: with no pause at all the read-back failed 30 times out of
  30; with one millisecond it succeeded every time. The read now retries, which is
  the remedy the open path already used for the same reason.

### Text encodings

- **`grep` reads the UTF-16 that Windows writes.** A byte-order mark is honoured
  on a named file, on stdin, and through the `-r` walk, in both byte orders, and a
  UTF-8 BOM is consumed rather than left to break `grep '^first'` on the first
  line of anything Notepad saved.
- Only on a BOM: no heuristics. A file that declares nothing is left alone,
  because guessing an encoding is how a binary gets rewritten. UTF-16 without a
  mark stays unread, which is where ripgrep draws the line too.
- Only in `grep`: `cat`, `head`, `tail` and the rest stay byte-exact, because
  `cat a > b` copies a file rather than reinterpreting one. `sed` and `wc -m` are
  named in `docs/support-matrix.md` as still outstanding, with the reason.

## v1.0.0 - 2026-08-21

The first stable release. What it contains beyond v0.1.0 is below; what it
promises is in `docs/support-matrix.md`, which states the divergences from
busybox-w32 rather than leaving them to be discovered.

Two features landed after `docs/design/v1-scope.md` had deferred them, and that
document records the decision rather than hiding it: line editing and `top`.
UTF-16 input is deliberately not here -- byte-exact, never corrupted, and not
interpreted, which is also what busybox-w32 does. Measured; see the support
matrix.

### Process monitor

- **`top`.** An htop-shaped process monitor that needs no administrator rights,
  which is the gap on this platform: `ntop` shows a list, `btop++` refuses to run
  unelevated, and Task Manager is not a terminal program. One system call --
  `NtQuerySystemInformation(SystemProcessInformation)` -- answers with CPU,
  memory, threads, handles, parentage and IO for every process on the machine
  without opening one of them, and opening handles is what costs privilege.
- A drawn table with per-processor meters, sorting by any column, a tree by
  parentage, `/` to search, F4 to filter, tagging, and F9 to kill. `-b` prints
  one plain sample instead, which is what a pipe, a script and a test get.
- F7 and F8 step a process one place along the priority ladder -- `idle` through
  `high`. Realtime is not reachable by keypress: a process there outranks the
  kernel's own input threads. Processes this session does not own refuse by name.
- `-H` shows a row per thread, with each thread's own id, priority, state and
  CPU. The columns that describe a process -- memory, handles, IO -- are blank on
  a thread row rather than repeated.
- The IO columns are named IO and not disk. Windows counts every byte a process
  moves through any handle in one set of counters, so a process reading a pipe
  would read as one thrashing the drive; per-process disk figures need ETW and an
  administrator. See `docs/design/process-view.md`.
- No load average, and no CPU temperature: Windows has neither, and the nearest
  analogues measure something else.
- `ps` moves onto the same data source and grows `PID PPID THR RSS TIME COMMAND`.

### Fixed

- **`out=$(cmd)` reported success whatever cmd did.** POSIX gives a command that
  is nothing but assignments the exit status of its last command substitution,
  which is what makes `out=$(cmd) || die` work. The status was discarded, so the
  commonest error check in shell never fired, in any script. Checked against bash
  across nine shapes, including `x=$(false) y=$(true)` where the last
  substitution wins and a readonly target where the assignment failure wins.
- **A pipe whose reader leaves early no longer complains.** `cmd | head -1`
  printed `write /dev/stdout: The pipe is being closed` when the producer was a
  separate process. POSIX delivers SIGPIPE and the writer dies quietly; Windows
  has no SIGPIPE, so the write fails and the error has to be recognised for what
  it is. The classifier already existed and one path had not been given it.
  busybox-w32 is silent here; measured in the same nested shape.
- **`printf` and `echo -e` had no hex escapes**, so `printf` of an ANSI sequence
  emitted the characters back. Octal is corrected too, and the two applets differ
  on purpose: XSI specifies the zero-prefixed form for echo and POSIX specifies
  the bare form for printf, so busybox implements both and the same characters
  mean different things in the two commands. Thirteen escape forms now match it.
- **Long options were invisible to completion.** Every spec has carried
  `value-long` and `file-long` since it was written -- curl declares a hundred and
  eighty of the first and thirty of the second -- and the loader validated them,
  and nothing read them: the check for "does this option take the next word"
  required a word of exactly two characters. So `curl -o ` offered files and
  `curl --output ` did not, and the word after `adb --one-device serial` was
  counted as an operand, which cost adb its subcommand list.
- `split` now stops when interrupted part-way through writing its parts.
- **Process start times were reported in 1811.** A Windows FILETIME counts from
  1601 and the conversion treated it as counting from 1970. Every reader used the
  value relatively -- a cache key, an identity check against pid reuse, and "is
  this parent older than its child" -- so all three were correct with a constant
  369-year offset and no test could see it.
- The earliest kernel threads report a boot-relative tick count rather than a
  date, because they are created before the clock is set. They are reported as
  unknown rather than as the year 2185.
- An input field in `top` read its own keystrokes as commands -- the `q` in
  `sqlservr` quit the shell -- because the key handler was registered on the
  application rather than on the table.

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
- **The line is drawn in colour.** A command that will run is green and one that
  will not is red; an option the command accepts is cyan and one it does not is
  yellow; the word being edited is underlined until a blank ends it. Every choice
  is in one struct, `defaultPalette()`.
- **"Will run" means PATH too**, along with this session's aliases and functions,
  so `wsl` and `ll` are green rather than red. PATH is read once on a background
  goroutine and rebuilt only when the variable changes — 78 directories and 9,917
  files measured at 16ms, which is nothing once and far too much per keystroke.
  Until that read finishes a name is drawn plainly rather than red, because red
  turning green a moment later is a colour you learn to ignore.
- **Colour absent turns both off** rather than degrading them, because a grey
  suggestion rendered as ordinary text would put characters on screen that are
  not in the line. `NO_COLOR`, `TERM=dumb`, and `NEMOSH_COLOR=always|never`.
- **Command completion reaches PATH**, along with aliases and functions, so `gi`
  finishes to `git`. It reads the index the suggestion engine already builds, so
  the old objection — walking 78 directories and 9,917 files on every Tab — no
  longer applies. A program is offered as `wsl`, though both `wsl` and `wsl.exe`
  are recognised.
- **Choices are listed in columns**, down rather than across, as busybox lays
  them out, and ordered without regard to case on Windows — byte order put
  `WFS WMIADAP WMIC` ahead of `wait` and `wc`.
- **A listing that would fill the screen asks first.** `w` has 118 answers and a
  bare Tab about two thousand; `Display all 118 possibilities? (y or n)`, which
  is what bash does and for the same reason.
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
- **Process control is now stated as a contract.** `docs/support-matrix.md` has a
  table of what the shell can do to a job or a process and what it will not, with
  the reason. The short version: ending something maps onto cancelling a context,
  so `kill %N` is honest; resuming needs a door that opens both ways, and there
  is none — Go cannot freeze a goroutine from outside and Windows has no
  `SIGSTOP` even for a real process. `fg` and `bg` now say that instead of
  blaming process groups, which was true and was the second reason.
- **`pgrep` and `pkill`.** A clean Windows machine cannot find a process by
  pattern at all: it ships `tasklist` and `taskkill`, neither of which takes one.
  The pattern is a regular expression on the executable's name, matched with or
  without its suffix and without regard to case, and an *empty* pattern is
  refused outright — `pkill ""` would match every process on the machine. `-f` is
  refused rather than quietly matching the name instead, because reading another
  process's command line on Windows needs privileges an ordinary session has not
  got. Process listing and termination live in one place, `internal/proc`, so the
  `kill` builtin and the `pkill` applet cannot disagree about what killing means.
- **`kill`, as a builtin.** `kill %1` stops a background job, `kill PID`
  terminates a real process, `kill -9`/`-TERM`/`-SIGTERM` are all accepted, and
  `kill -l` lists what the shell can act on. It is a builtin because `%N` names a
  job and only the shell has the job table -- which is exactly why busybox's is
  one too. A job here is a goroutine with no pid, so the signal arrives as a
  cancellation of the job's own context; an external command in that job was
  launched under that context, so a real process still dies. It does not claim
  the job, so a later `wait %N` still finds it.
- `head -3` and `tail -2`, the obsolete count form POSIX still lists and everyone
  types. busybox takes it; refusing it made muscle memory an error.
- **`tr`, `tee`, `seq`, `clear`, `whoami`, `mktemp`, and a `which` builtin.**
  Measured, a clean Windows machine ships only `certutil`, `clip`, `curl`, `fc`,
  `findstr`, `more`, `robocopy`, `tar`, `timeout`, `where` and `whoami` — every
  one of these was simply unavailable there. `tr -d '\r'` matters twice over on
  Windows, since nothing else in the bundle strips a carriage return. `which` is
  a builtin rather than an applet so that its answer is the shell's own lookup
  and cannot disagree with what typing the name would run.
- **`cp -r`.** Copying a directory was the one thing this bundle could not do at
  all, by any combination of applets. Without `-r` a directory now answers
  `cp: omitting directory 'src'` and exits 1; with it, a destination that does
  not exist *becomes* the copy and one that already exists takes it underneath —
  measured against busybox rather than assumed.
- `cat -n`, `head -c`, `uniq -c`, `basename -a`, and `mv -f`, all matching
  busybox's output byte for byte. `tail -c` is deliberately still refused: head
  counts bytes and tail does not, and claiming otherwise is the kind of thing a
  script discovers the hard way.
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

### Project

- **Binaries for Linux and macOS.** `linux/amd64`, `linux/arm64`, `darwin/amd64`
  and `darwin/arm64` now ship alongside Windows, as `.tar.gz` — tar rather than
  zip because a zip does not carry the executable bit. Publishing is not
  promising: only `windows/amd64` is supported, and the release notes, README and
  support matrix each say so beside the download.
- **Build provenance.** Every published archive carries a signed attestation
  tying it to the workflow and commit that produced it. `gh attestation verify
  <file> --repo xiongnemo/nemosh`. It is not code signing and does not quiet
  SmartScreen; it answers a different question.
- No SBOM, deliberately: a Go binary records its own module graph, so
  `go version -m nemosh.exe` lists every dependency and hash — there are two —
  and `govulncheck` already reports the reachable vulnerabilities weekly.

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
- **An elevated shell calls itself root.** Under gsudo, `id -u` answered 0 while
  every name still came from the Windows account, so `id` read `uid=0(nemo)` and
  a prompt using `\u` read `nemo` beside a `\$` reading `%` — one process giving
  two answers about itself. The name now follows the number, as busybox's does:
  `getpwuid(0)` is `root` there and the account name otherwise. `\u` takes it
  from the identity rather than from `$USERNAME`, which Windows leaves untouched
  when a process is elevated.
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
