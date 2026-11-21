# Bash mode: reading a script a command at a time

Status: proposed, 2026-11-21. Milestone M6 of the Git bash compatibility plan; nothing here is
built yet.

## What differs

nemosh reads a script whole before it runs any of it, and substitutes aliases after a command
is parsed. Both references read a command, run it, and read the next -- bash's
`parse_and_execute`, busybox's `cmdloop` -- and substitute an alias as the lexer meets its
name. That is nemosh's own design, not either reference's, and the plan's E class: run as bash,
the shell is to follow bash; run as nemosh or sh it keeps what it does.

What a script sees of the difference:

- **A syntax error later in the file stops nothing before it.** bash runs every command up to
  the line it cannot parse, and says the error there; nemosh refuses the whole file before the
  first line runs, `incomplete script: missing fi`. A self-extracting installer, `exit 0` and
  then an archive, runs in bash and is refused here.
- **An alias can expand into syntax.** `alias ll='{'`, an alias whose text is `if`, `for x in`,
  or the first half of a loop, changes how the rest of the line parses in bash. Here the line
  is parsed first and the alias substituted into a command already parsed, so none of them
  works: Oils' alias.test.sh has six such cases failing.
- **expand_aliases is off in a non-interactive bash** unless the script turns it on; nemosh
  substitutes in a script as at a prompt.

What it does not change: an alias defined on a line is not in force on that line, nor inside
the compound it is in, nor in a function body defined before it -- nemosh already gives each
line a copy of the aliases as it begins (alias_view.go), and keeps that.

## The machinery already there

A session reads this way. `ParseSessionInput` parses what has been read so far and answers
`ErrIncompleteScript` while a command is still open -- a quote, a here-document, a compound,
a `\` at a line's end -- and the session reads another line and tries again. Line numbers
carry on from the lines before (`parseSessionAt`, `interactive.linesRead`). A here-document's
body is read with the command it belongs to, since the command is incomplete until its
delimiter.

So the change is to feed a script to that loop a line at a time instead of to `parseScriptAt`
whole, in bash mode only.

## The design

1. **Chunks.** `runScriptResult`, in bash mode, cuts the source into lines and keeps a buffer:
   a line is appended, the buffer parsed with the session's parser, numbered from the first
   line not yet run. `ErrIncompleteScript` reads the next line; any other error is said with
   the line number it names and ends the script, status 2, the commands before it having run,
   as bash ends it -- measured 2026-11-21, a file and `-c` alike: `echo before; if then`
   says before, then the syntax error, and exits 2; a complete parse is executed with
   `executeStatements`, and the buffer emptied. At the end of the source an incomplete buffer
   is the unterminated-construct error, said there.
2. **Flow.** `exit`, `return` from a sourced file, `exec`, and a fatal error end the loop:
   nothing after them is read, which is what lets the installer's archive go unparsed.
   `set -n` reads and parses each chunk without running it.
3. **The same for eval, `.`, `-c` and a trap's action.** bash runs all of them through
   `parse_and_execute`, a command at a time. They share `runScriptResult`, so they share the
   loop.
4. **Aliases while reading.** With the buffer parsed as it grows, the lexer can substitute an
   alias where a command word begins, as both references do: the alias's text replaces the
   word in the input, and lexing goes on through it, so `{`, `(`, a reserved word or half a
   loop in the text is syntax. The aliases consulted are the ones in force when the chunk is
   read, alias_view.go's copy. A trailing blank makes the next word a candidate too, as now.
   Recursion is stopped by the alias being substituted, as bash stops it.
5. **expand_aliases.** In bash mode a shell that is not interactive starts with it off, as
   bash does; `shopt -s expand_aliases` turns it on, and a session has it on.

Run as nemosh or sh, none of this applies: the whole-file parse, its up-front refusal and
runtime substitution stay, and the 3,000-odd existing tests hold them there.

## To measure before building

- Where bash's `$LINENO` and the error's line number fall for a command spanning lines, and
  for a here-document.
- Whether bash parses a function body's aliases when the definition is read (it does, by its
  documentation) -- and so whether alias_view.go's per-definition copy is still right once the
  lexer substitutes.
- The cost: parsing a 10,000-line script as 10,000 chunks, against one parse today. The session
  path re-parses its buffer from the start of the open command only, so a long compound is the
  case to time.

## Steps, each a commit with its Oils cases

1. expand_aliases off in a non-interactive bash-mode shell.
2. Chunked execution of a script file and `-c` in bash mode, with the end-of-input and
   line-number behaviour measured first; Oils' 13 whole-parse cases as acceptance.
3. eval, `.` and trap actions through the same loop.
4. Alias substitution in the lexer in bash mode; Oils' alias.test.sh cases as acceptance.

Each moves only bash-name cases; `tests/oils/baseline-sh.json` holds the sh name where it is.
