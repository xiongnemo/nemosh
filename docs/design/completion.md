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

**Where candidates come from** is builtins and applets for a command word, and
the named directory for an operand. `PATH` is deliberately not walked: on Windows
that is dozens of directories and thousands of files, and the pause would be felt
on every Tab. This is a real gap against all three references and is listed below.

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
- **`PATH` executables for command completion.** Needs a cache with an
  invalidation story before it can be on the Tab path at all.
- **`~` and `$VAR` expansion**, which busybox has for `~` and which v1.1 already
  lists.
- **Real columns in the listing.** Candidates are joined by two blanks and left
  to wrap; busybox lays them out to the terminal width (`showfiles`, `:1279`).

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
