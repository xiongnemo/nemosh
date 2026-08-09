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

**What kind of thing can go there** is `operandKind` (`complete_spec.go`).
busybox's `cd` rule, with the command name looked up in a table rather than
compared inline. `cd`, `mkdir` and `rmdir` take directories; everything else
takes any path. The bar for adding to that table is that a regular file could
*never* have been meant -- narrowing is only safe when the omitted candidates
were impossible, and a command that merely prefers directories does not qualify.

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
- **Listing on the second Tab rather than the first**, with real columns. Nemosh
  currently inserts the shared prefix and lists immediately, joined by two
  blanks; busybox and bash both wait for a second Tab and lay out columns to the
  terminal width.
- **Fish-style inline suggestion.** See below.

## Inline suggestion, and what it would need

The grey-text-ahead-of-the-cursor behaviour is not completion. Completion answers
"what could go here" on demand; suggestion answers "what did you most likely mean"
on *every keystroke*, and shows one answer speculatively. Fish draws it from
history first and falls back to completions.

Nothing in the current design forbids it, and three of the four pieces exist:

- The editor already redraws the whole line on every keystroke
  (`lineedit_draw.go`), which is the expensive prerequisite most line editors
  lack.
- History is already kept and already searched for Up/Down.
- `\033[90m` and a reset are all the rendering it needs.

What is missing is honest accounting rather than machinery:

1. **The suggestion occupies columns it does not own.** `promptColumns` and
   `cursorColumns` decide where the cursor goes; grey text drawn past the cursor
   has to be erased before the cursor is placed, or every subsequent redraw is
   off by its width -- the same class of defect as the prompt measuring eleven
   columns for a two-column `$ `.
2. **It must never be committed by accident.** Enter has to submit the typed
   line, not the suggestion; only an explicit key (fish uses Right/End) accepts
   it.
3. **It runs on every keystroke**, so the lookup has to be bounded. History
   search is fine; a directory read per keystroke is not, which is why the first
   version should suggest from history only.
4. **It has to survive wrapping**, since a suggestion is what most often pushes a
   line past the terminal width.

The screen-model tests already assert what a row looks like rather than which
bytes were sent, which is exactly the harness this needs: a suggestion that
leaves the cursor one column off would be visible there and invisible to a
`strings.Contains` assertion.
