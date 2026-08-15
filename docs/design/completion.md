# Tab completion

## The question every shell answers differently

Completion is not one problem. It is: *what is the word under the cursor*, *what
kind of thing can go there*, *where do the candidates come from*, and *how is the
choice presented*. Three shells were read before any of this was written, and
they disagree mainly on the second question.

### ash, through busybox's line editor

All of it is `libbb/lineedit.c`, about 830 lines of the file. `build_match_prefix`
(`:1131`) rewrites the line into the word being completed and returns one of
three modes: executables, directories, or files.

It does not parse. It rewrites a byte array: mark every `\c` and quoted span with
a `QUOT` bit (`:1155`), delete everything up to the last command delimiter --
`;` `&` `|` `&&` `||`, carefully not `>&` `<&` `>|` -- drop backquoted spans, drop
up to `(` or `{`, then keep the last word split on `` <>|&= ``. Doing it on bytes
with a marker bit is how it survives quoting without a parser, and it is a good
trick.

The mode is chosen at `:1235`. It is worth quoting in full, because it is the
entirety of busybox's per-command knowledge:

```c
if (int_buf[i] == ' ' && command_mode == FIND_EXE_ONLY
 && (char)int_buf[0] == 'c' && (char)int_buf[1] == 'd' && i == 2)
        command_mode = FIND_DIR_ONLY;              /* lineedit.c:1245 */
```

One command, spelled out inline. Everything else is: first word means
executables, later words mean files.

Candidates come from applet names (`:984`), the shell's builtins, and every
`PATH` directory -- but only for executables, and that scan skips directories
(`:1068`); directory mode skips files (`:1075`). Interaction: first Tab inserts,
second Tab lists in computed columns (`showfiles`, `:1279`), unique match gets a
trailing blank unless it ends in `/` (`:1513`), ambiguous inserts the longest
common prefix and beeps (`:1468`). Inserted text is backslash-escaped by
`quote_special_chars` against the set at `:1330`.

### bash, in two layers

Readline owns the interaction -- `show-all-if-ambiguous`, `menu-complete`,
`completion-ignore-case`, `completion-query-items` -- and its default is plain
filename completion.

Above it sits programmable completion. On Tab, bash looks for a compspec: the
exact command name, then for a pathname the full path and then the basename,
then the `-D` default, then alias expansion and retry, and failing all of that
readline's filename completion. The registered function receives `COMP_LINE`,
`COMP_POINT`, `COMP_KEY`, `COMP_TYPE`, `COMP_WORDS`, `COMP_CWORD`, and
positionally the command, the current word and the previous word; it answers in
`COMPREPLY`. Words are split on `COMP_WORDBREAKS`, whose default includes `:` and
`=`, which is a well-known source of trouble.

Two mechanisms are worth remembering. `-o` gives ordered fallbacks --
`dirnames`, `bashdefault`, `default`, `plusdirs`, and `filenames` for
filename-style quoting. And returning **124** means "I have just redefined the
compspec, try again", which is how the bash-completion project lazy-loads a
per-command file from a single `complete -D` registration.

The architectural point: bash ships almost no completion knowledge. It ships a
protocol, and roughly fifteen thousand lines of shell script live outside it.

### zsh, through compsys

`compinit` autoloads functions from `fpath` and caches to `.zcompdump`. A file
beginning `#compdef git` binds itself to `git`.

Three ideas the others do not have. Completers are **strategies, chained** --
`zstyle ':completion:*' completer _complete _match _approximate` runs exact, then
glob, then typo-tolerant, each only if the last found nothing. Every candidate
carries a **tag** (`files`, `directories`, `options`, `hosts`), which is what
allows grouping and per-category styling. And configuration is addressed by
**context string**, `:completion:<function>:<completer>:<command>:<argument>:<tag>`,
so any behaviour can be set at any granularity.

`_arguments` is the piece worth stealing conceptually: a command's option grammar
is declared once and drives completion positionally.

## What this shell does

Nemosh sits closest to ash, and deliberately so -- the knowledge is in the shell,
not in a corpus of scripts a Windows user would have to install separately. The
differences from busybox are where busybox is arguably wrong rather than merely
small.

**The word under the cursor** is found by `completionStart`
(`cmd/nemosh/lineedit_buffer.go`), scanning forwards and honouring backslash
escapes, so `My\ Documents/re` is one word. Forwards is not a preference: walking
back from the cursor, an escaped blank is indistinguishable from a separating
one. This is a separate question from `wordStart`, which Ctrl-W uses and which
*does* step back over blanks -- sharing the one boundary is what made Tab after
any command plus a blank do nothing at all.

**What kind of thing can go there** comes from `internal/capability`, below.
busybox's `cd` rule, with the command name looked up in a table rather than
compared inline. `cd`, `mkdir` and `rmdir` take directories; everything else
takes any path. The bar for adding to that table is that a regular file could
*never* have been meant -- narrowing is only safe when the omitted candidates
were impossible, and a command that merely prefers directories does not qualify.

**An option is offered from the same table.** `ls -<TAB>` answers
`--color -1 -a -h -l`. When no option matches, this falls back to paths, which is
bash's `-o bashdefault` idea and what keeps a file named `-1.18-windows.xml`
reachable: nothing matches `-1.1`, so the file is offered instead.

**Where candidates come from** is, for a command word, everything the shell can
run: builtins, applets, this session's aliases and functions, and `PATH`. For an
operand it is the named directory.

`PATH` used to be left out, on the grounds that walking it -- 78 directories and
9,917 files, measured -- would be felt on every Tab. That was right and is now
obsolete: the suggestion engine already builds an index of it, once per `PATH`
and on a background goroutine, so completion reads a map. `gi` finishes to `git`.

A program is offered under the name a person types. `wsl.exe` is *recognised* as
both `wsl` and `wsl.exe`, since either can be typed, but only `wsl` is offered --
a column holding both reads as though there were two programs.

**The listing is laid out in columns**, down rather than across, following
busybox's `showfiles` (`:1279`), and ordered the way matching works: without
regard to case on Windows, because byte order put `WFS WMIADAP WMIC` ahead of
`wait` and `wc` on a `PATH` full of system programs spelled in capitals.

**A listing that would fill the screen asks first**, which is bash's behaviour
and for bash's reason. `w` has 118 answers here and a bare Tab has about two
thousand. Printing those unasked scrolls the session away; refusing outright
takes the decision from someone who may want to look. The question is the only
option that does neither, and it is answerable because completion runs inside the
key loop rather than beside it.

**Matching ignores case on Windows** and does not elsewhere, because NTFS does
not distinguish it and a shell that does is contradicting the directory it just
read. busybox-w32 makes the same split at `:1039`.

**Insertion is escaped** against busybox's special set, and the typed word is
unescaped before matching. On Windows a blank in a path is the common case, and
inserting `Program Files/` raw produces a line naming two operands, neither of
which exists.

The set is busybox's, but it was re-measured against *this* parser rather than
inherited on trust. Every one of `` ` `` `'` `"` `\` `$` `(` `)` `&` `;` `|` `<`
`>` breaks it outright; `#` at the start of a word begins a comment and swallows
the line; and `*` `?` `[` are the ones a naive check calls safe, because they are
harmless until something matches -- with three files present, a bare `a*b`
expanded to `a-b aXb azb` rather than naming the file called `a*b`. `~` is safe
today and will not be once tilde expansion lands, so it is escaped now.

**A dash-leading name is rewritten to `./name`**, which escaping cannot do
anything about. Quoting is resolved by the shell; the operand/option split
happens afterwards, inside the applet, so `ls -l \-1.18-windows.xml` and
`ls -l '-1.18-windows.xml'` both fail identically and only `./` works. bash and
busybox hand back the bare name and leave a command that cannot run. Applied to
operands only, never to a command word, and never when a directory part is
already present.

## One table, two features

`internal/capability` says what each command takes: its short options, its long
options, and whether an operand can only be a directory. Completion offers from
it, and the renderer colours from it, so the two cannot disagree about what
exists -- an option Tab offers is an option drawn as accepted, because one place
says so.

What makes it worth having rather than another thing to keep up to date is that
it is **bound to behaviour**. `capability_test.go` runs every applet with every
option the table claims and fails if one is refused, and runs each with `-Z` and
fails if it is accepted. A table that merely looked consistent with the code
would drift within a release; this one cannot drift without a red build.

Writing it caught three things immediately: `id` was credited with a `-U` it does
not have, and `chmod` and `sed` turned out not to refuse unknown options at all,
because their first operand is the mode and the script -- so `-Z` comes back as
`invalid mode` rather than as an unknown option. The support matrix's third
column said otherwise for those two, and measurement won.

Builtins carry an operand kind and no option claims. Nothing here measures a
builtin's options yet, and an unchecked claim is exactly the kind that goes stale
quietly.

## Backslashes, not quotes

PowerShell quotes a completed name; bash, zsh and busybox all backslash-escape
it. This follows the second group, and the reason is not habit.

PowerShell has no choice. On Windows the backslash is the path separator, so it
cannot also be the escape. This shell is POSIX-shaped -- backslash escapes and
`/` separates, which is why `C:/Users` works here and the backslash spelling does
not -- so the constraint that decides it for PowerShell does not apply.

What decides it here is that completion has to compose with itself. A backslash
escape is per-character and local: the word boundary scanner already understands
it, so a second Tab continues from `My\ Do` without any further machinery. A
quote is neither. It has to be inserted *before* text the user already typed, and
then either closed or not -- close it and every later Tab has to notice the
cursor is inside a string and complete there; leave it open and the line is
broken until the user finishes it. zsh does the first properly, and it is a large
amount of machinery.

The honest middle path, if quoting is ever wanted, is zsh's: **preserve whatever
the user started**. If the word already opens with a quote, complete inside it
and close it; otherwise escape. That keeps one rule -- follow the user -- rather
than two competing ones. It is listed below rather than done.

## Not done, in the order it is worth doing

- **Option completion** (`ls -<TAB>`). The obvious idea -- derive it from the
  applet manifest -- does not work: `internal/appletmanifest` compares name
  lists and carries no option data, and only nine of the forty applets use the
  shared `parseAppletOptions`, so there is no single place the truth lives. Doing
  this honestly means either declaring options per applet and binding the
  declaration to behaviour with a test that fails when they drift, or moving
  every applet onto the shared parser first. The second is the better order.
- **`~` and `$VAR` expansion**, which busybox has for `~` and which v1.1 already
  lists.

- **Quote-preserving completion.** If the word already opens with a quote,
  complete inside it and close it, instead of escaping. zsh's rule, and the only
  version of "support quotes" that does not end up with two competing schemes.
- **Persistent history**, which is what would make inline suggestion strong.

## Listing on the first Tab

A deliberate divergence, not an omission, and worth recording because it looks
like a missing feature from the busybox side.

busybox rings the bell on the first Tab and lists only on the second
(`lastWasTab`, `:1383`). With an empty word -- `cd ` and Tab, the commonest case
there is -- there is no common prefix to insert, so that first press produces no
visible result at all, and seeing what is available costs two keystrokes. bash is
the same by default, and `show-all-if-ambiguous` exists precisely because people
turn it off.

Nemosh inserts the shared prefix and lists at once. The bell is kept for the one
case that has no visible result to give: no candidates at all. That is the case
silence genuinely cannot express -- an empty directory and a broken Tab look
identical without it, which is exactly how a defect that made every argument
uncompletable went unnoticed.

## Inline suggestion

Built, and written up separately in [`suggestion.md`](suggestion.md): it shares
the capability table with completion but is a different feature, answering on
every keystroke rather than on request.

## Host names

`ssh <TAB>` completes machines rather than files, from `~/.ssh/config`. It is the
first operand kind that does not come from the filesystem at all, which changes
two rules.

**The source is deliberately narrow.** `~/.ssh/config` only: it is the file a
person curates on purpose, so its names are the ones they meant. `known_hosts` is
*not* read — it is a machine-written cache, it is the file most likely to be
enormous, and under `HashKnownHosts yes` its entries are hashed and unreadable by
design, so it would answer richly on one machine and not at all on the next.
`/etc/hosts` is read off Windows and not on it: measured, the Windows one ships
with every line commented out and usually stays that way, and it is the file most
likely to have been replaced wholesale with an ad-blocking list of tens of
thousands of names. Entries pointing at `0.0.0.0` are skipped for the same
reason — a name installed in order *not* to reach is not somewhere to connect.

Both `Host` and `HostName` are read. They answer different halves of one
question: `Host` is the short alias someone wrote the file to have, `HostName` is
what it resolves to and is also frequently typed. fish reads only `Host`;
bash-completion's `_known_hosts_real` reads both, and is right — an alias whose
real name is never offered makes the file look half-read. Patterns (`Host *`,
`prod-*`, `!name`) and ssh's substitution tokens (`%h`) are skipped: they
configure a set, and completing one puts a name on the line that resolves to
nothing.

**There is no fallback to paths.** Everywhere else a specific completion that
finds nothing falls through to the ordinary one — that is what keeps a file
named `-1.18-windows.xml` reachable. A host is not a path, so `ssh nonexist<TAB>`
answering `notes.txt` would be an answer from the wrong universe rather than a
wider guess. Nothing to offer rings the bell.

### Knowing that `ssh -i` wants a file

The word after an option is not always an operand, and which it is cannot be read
off the word itself. The capability table therefore carries two subsets of each
command's option letters: the ones that consume the following word, and the ones
whose consumed word is a path.

- `ssh -i <TAB>` → files, because `-i` takes an identity file.
- `ssh -p <TAB>` → **nothing**, because `-p` takes a port and this shell cannot
  guess one. Offering the current directory there would be the wrong universe.
- `ssh -i key <TAB>` → hosts again. `key` belongs to `-i`, so the host has not
  been given yet.
- `ssh myhost <TAB>` → nothing. The second operand is a command run on the far
  side; bash-completion answers it by opening a real connection to the remote
  machine, which this shell will not do behind anyone's back.

zsh has this because `_arguments` declares an argument type per option;
bash-completion hand-writes it as a `case $prev in` per command. Here it is data
in the one table both completion and the suggestion renderer already read.

### The one unmeasured row

Every other row in `internal/capability` is bound to behaviour by a test that
runs the command. `ssh` cannot be run to check, so its row is marked `External`
and was transcribed from the usage synopsis the installed program prints for
itself — OpenSSH_10.2p1, 2026-08-15 — rather than from memory or a web page. The
bracket groups in that synopsis map exactly onto the three columns: the flags
that stand alone, the ones with an argument, and the five whose argument is
spelled as a file.

`TestOnlyExternalRowsAreUnmeasured` holds the line. `External` is an escape
hatch, and a row nobody can check is easier to write than one that has to survive
being run, so the pressure is always to widen it; the test makes widening a
decision someone has to make on purpose.

## Where the three shells keep this knowledge

Recorded because the design above is a choice among these, not an invention.

| | Completion comes from | Knows `-i` takes a file | Cached | Descriptions | Inline suggestion |
| --- | --- | --- | --- | --- | --- |
| **bash** | `bash-completion`'s shell functions, `complete -F _ssh ssh`, reading `COMP_WORDS` and writing `COMPREPLY` | hand-written `case $prev in` per command | no — `known_hosts` is re-read on every Tab | none; readline shows bare words | none built in |
| **zsh** | compsys `_ssh`, an `_arguments` grammar declaring every option and its argument type | yes, from that grammar | yes, `_store_cache`/`_retrieve_cache` | yes, and candidates are grouped under tag headings | a plugin, zsh-autosuggestions, with `history`/`completion`/`match_prev_cmd` strategies |
| **fish** | declarative `complete` lines in `share/completions/*.fish` | yes, `-r` and `-F` declare it | some functions cache their own | yes, one column in the pager | built in, history-first, grey, accepted with → or End |

Two things worth taking from that table. zsh's per-option argument types are the
right shape for the `ssh -i` problem, and are what the capability table now
carries. And zsh-autosuggestions' `completion` strategy — ask the completer and
show its first answer — is what makes a suggestion possible for a host never
typed before; it is adopted here only for sources already in memory, because
nothing on the keystroke path may touch the filesystem.

**Neither fish nor bash labels which source a host came from.** Checked:
`__fish_print_hostnames.fish` merges `/etc/hosts`, `/etc/fstab`, `known_hosts`
and the ssh configs and emits them with no attribution, and both host lines in
`completions/ssh.fish` carry the same description, `Remote`. fish distinguishes
its two sources by *order* instead — the history-derived one is `-k`, kept at the
top. zsh is the shell that labels, by grouping candidates under a tag heading.
Labelling here would mean giving the listing a description column or group
headings; today it is busybox's `showfiles` layout, which has neither.
