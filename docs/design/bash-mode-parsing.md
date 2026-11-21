# Bash mode: reading a text a command at a time, and aliases that are syntax

Status: built, 2026-11-22. Milestone M6 of the Git bash compatibility plan. Run as bash --
`nemosh bash`, a `#!/bin/bash` script, `NEMOSH_COMPAT=bash` -- the shell reads as bash reads;
run as nemosh or sh it reads as it always has, and says so when an alias it will not read is why
a line is refused.

## What differs

nemosh reads a script whole before it runs any of it, and substitutes an alias as its command
runs, into a command already parsed. Both references read a command, run it, and read the next
-- bash's `parse_and_execute`, busybox's `cmdloop` -- and substitute an alias as their reader
meets its name, before the line's structure is decided. What a script sees of the difference:

- **A syntax error later in the file stops nothing before it.** bash runs every command up to
  the line it cannot parse and says the error there, status 2, with its EXIT trap; nemosh
  refused the whole file before its first line ran. A self-extracting installer, `exit 0` and
  then an archive, runs in bash and was refused here, and so was any file over the parser's
  1 MiB.
- **eval's text and a sourced file are read so too**: `eval $'echo a\nif then'` says a.
- **An alias can be syntax.** `alias LEFT='{'`, an alias for `if`, or one for the first half
  of a loop changes how the rest of the line parses in bash and in busybox, as POSIX 2.3.1
  allows. nemosh parsed the line first, so none of them worked.
- **expand_aliases is off in a bash that is not interactive** until the script turns it on.

## What the community makes of aliases that are syntax

They are a wart, by wide agreement: bash's manual says to use functions instead, Greg's wiki
calls aliases a hazard in scripts, ShellCheck warns of one used in the script that defines it
(SC2262), and Oils refuses an alias that is not a whole command on purpose. The other shells
read them: bash and busybox, measured, and dash, mksh and zsh by their documentation.

So the decision, 2026-11-22: bash mode reads them as bash does, since a script for bash may rely
on them, and nemosh and sh go on refusing them, as Oils does, with a hint on the line after the
syntax error that names the alias and bash mode.

## Reading a text in bash mode

`runScriptResult` hands every text bash mode runs to `runChunks` (script_chunks.go): a script,
`-c`, stdin, eval's text, a sourced file and an rc file (`SourceStartup`).

1. **Whole first.** The text is parsed whole and, when that succeeds, run as it always was, a
   line at a time: almost every script, at the cost it always had -- five thousand lines in
   31 ms before and after.
2. **A command at a time when it does not parse whole.** From the first line not yet run, the
   session's parser is asked whether the lines so far end in a whole command, at one line, two,
   four and so on rather than at every line (a session asks again from a command's first line
   at each line, which for a three-thousand-line loop took six seconds); an error narrows the
   question to the lines it took (`chunkWithin`). A whole command runs, `$?` carried on; any
   other error is said and ends the text, status 2 -- a script's EXIT trap run, eval and `.`
   going on as bash goes on outside its POSIX mode. The end of the input is parsed as a
   script's end, for a here-document's warning and a final `\`. Nothing after an `exit`, an
   `exec` or a fatal error is read.
3. **Raw lines.** The text is cut at `\n` before anything else, so a CRLF is never split; each
   chunk is parsed as text of its own, numbered from where it starts, so `$LINENO` is the line
   bash reports.

## Aliases read as the text is read

nemosh decides a text's structure in passes over its lines before any word is lexed
(`parseScript`: `collectHeredocs`, `numberedLogicalLines`, `compoundSpans`), so there is no
lexer to substitute into. In bash mode an alias is substituted by one more pass instead,
alias_read.go, over the text the heredoc pass leaves, which reads only as much of the grammar as
says where a command's name goes:

- after a newline or an operator, after the reserved words a command follows, after
  assignments and redirections, after a function header's `()` and a case pattern's `)`, and
  inside `$(...)`, backquotes and process substitutions;
- not in a case pattern, for's words, a `[[ ]]` or an arithmetic command, nor inside quotes,
  comments or a here-document's body;
- an alias's text is read on with the rest, its first word a command's name if the alias was in
  that place; text ending in a blank makes the next word an alias too, wherever it is -- for's
  name and words included, as bash reads them; an alias is not substituted again inside its
  own text.

The pass is made wherever bash mode parses with expand_aliases on and aliases defined: the text
read whole, each chunk, each line a session reads, and what is parsed as it runs -- a trap's
action, a here-document's `$(...)`. The substitution a command used to get as it ran is off in
bash mode, which would have read `alias ls='ls -F'` twice.

**An alias is in force from the next command.** bash reads a command, runs it, and reads the
next, so `alias x=...` is not in force on its own line and is on the next. nemosh reads the text
whole, so it runs what it read a line at a time (`runRead`, script_reread.go) and, when the
aliases the text was read with have changed, reads the rest again from where the command just
run ends -- the first line after it that the lines from the last known command boundary parse
whole at; none before the next statement, and that statement is part of the command just run,
as `x` in `fi; x` is. It is read again only when a changed alias is written in the rest where a
command's name could be, or in some alias's text: a `.bashrc` defining forty aliases and using
none of them is read once. A file defining alias after alias, each used further on, read its
rest again for every one -- fifty in a six-hundred-line file took 114 ms where it took 13 -- so
the rest is read whole again only while the lines parsed stay within three times the text's,
and a command at a time after that: 21 ms.

A function's body is read when its definition is, with the aliases of that moment, as bash
reads it; the copy of the aliases a definition kept for its body (alias_view.go) is what nemosh
and sh still use.

## As nemosh and sh

Nothing above applies. The text is parsed whole and refused whole, and an alias is substituted
as its command runs. When an alias is why a text is refused -- the aliases defined, with those
the text defines itself with `alias NAME=VALUE` found without running it, read as bash reads
them, make the text parse -- or when an alias's text does not parse as its command runs, the
syntax error is followed by

    hint: alias LEFT='{' is part of a command's syntax, which nemosh does not read on purpose;
    bash mode does: nemosh bash, or NEMOSH_COMPAT=bash

(alias_warning.go), in a script, eval's text and at a prompt.

## Known limits

- A here-document in an alias's text is taken out of that text as the alias is substituted,
  as the text's own were before the alias pass, and is one when its body is in the alias too --
  `hi`, as bash 5.3 and busybox say; Oils' case records an old bash's defect as bash's answer.
  One whose body is written after the alias's use, in the text, is not.
- An alias whose text holds a newline puts the line numbers after it out by as many lines.
- A syntax error's words are nemosh's: `incomplete script: missing fi`, where bash says
  `syntax error near unexpected token`.

## How it was measured

Against Git for Windows' bash 5.3 and busybox-w32, a script per case, and Oils' spec suite: 14
cases that need a text read a command at a time, among them trap EXIT with a parse error and
`exit` before code that does not parse, and the five of alias.test.sh whose alias is syntax.
eval's text and expand_aliases have no Oils case and were measured directly. Each step moved
only bash-name cases; `tests/oils/baseline-sh.json` holds the sh name where it was.
