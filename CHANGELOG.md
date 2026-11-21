# Changelog

Notable changes, newest first. Kept from `v0.1.0`, the first tagged release;
v0's development history lives in the commit log, which is detailed enough that
restating it here would only add a second version of the truth.

Versions follow `AGENTS.md`: an exact `vMAJOR.MINOR.PATCH` tag is a release, and
every push to `master` publishes a `vX.Y.Z-master-<commit>` prerelease whose
patch number is the commits since that tag.

## Unreleased

### Added

- **`bind`**, bash's: `bind '"\C-o": beginning-of-line'`, macros, `bind -x` with `READLINE_LINE`
  and `READLINE_POINT`, as fzf's Ctrl-T uses it, `-m KEYMAP`, `-r`, `-u`, `-q`,
  `-f FILE`, `set VARIABLE VALUE`, and `-l -p -P -s -S -v -V -X` printing what bash prints. The
  line editor reads the keys it binds before its own; a key bound to a readline function this
  editor does not carry out is refused. readline's `set-mark` (`C-@`, `M-SPC`),
  `exchange-point-and-mark` (`C-x C-x`), `shell-expand-line` (`M-C-e`), `vi-editing-mode`
  (`M-C-j`), `redraw-current-line`, `emacs-editing-mode`, `vi-movement-mode` and
  `vi-insertion-mode` came with it, which fzf's Alt-C macro runs; in bash mode `^H` is
  readline's backward-delete-char, as in Git's bash. It was refused outright, as not implemented.

- **`stty -g`**: the console's state as one word, which `stty WORD` puts back, as atuin's init
  saves the terminal and restores it around its search.

- **`/dev/tty`**, the console: `CONIN$` read and `CONOUT$` written, handed to a program as the
  console's own handles, so `fzf </dev/tty` has the terminal. Git's bash's `No such device or
  address` where there is no console.

- **`cygpath`**, Git for Windows' -- MSYS2's -- over this shell's path model: `-u` `/c/Users`, `-w`
  `C:\Users`, `-m` `C:/Users`, `-a`, `-p` for a list, each as Git's 3.6 converts it, for a script
  written for Git's bash; zoxide's init calls `cygpath -w` before every prompt.

- **`PROMPT_DIRTRIM`**, bash's: `\w` keeps that many trailing directories and puts `...` for the
  rest, `~/.../d/e`, by bash's own rule for when a path is left whole.

- **`declare -t`**, bash's trace attribute: `declare -ft f` has f inherit the DEBUG and RETURN
  traps, as every function does under `set -T`, and keep the attribute when it is defined again;
  `declare -F` lists it as `declare -ft f`, and on a variable `declare -p` shows it. It was an
  invalid option, the first thing bash-preexec runs.

- **Bash mode**, the start of following Git for Windows' bash where bash and busybox part:
  the shell run as bash -- `nemosh bash`, a `#!/bin/bash` script, a command named bash -- is in
  it, and so is the nemosh name with **`NEMOSH_COMPAT=bash`** in the environment, so a Windows
  Terminal profile can have it without a `bash.exe` on PATH standing in front of Git's own.
  `sh` keeps busybox's way whatever the variable says. What follows bash in it is in
  `docs/support-matrix.md`, Bash mode, as each place moves. A value the variable does not know
  is said on stderr.
  - **A session in bash mode reads bash's startup files and then nemosh's**: `~/.bashrc`, or a
    login's `~/.bash_profile`, `~/.bash_login` or `~/.profile`, and then `$ENV` or
    `~/.nemoshrc`, so one `~/.bashrc` serves Git's bash and this shell. Its history is
    `~/.bash_history`, and `~/.nemosh_history` is read after it. What nemosh's file changes of
    what bash's set -- a variable, an alias, a function, an option, a completion -- is said on
    one line of stderr.
  - **A session in bash mode reads readline's inputrc**, `~/.inputrc`, before its startup files,
    with `set`, bindings, `set keymap`, `$if`/`$else`/`$endif` and `$include`; `bind -f` reads one so.
  - **A prompt in bash mode is decoded and then expanded**, bash's order, so a Windows path a
    `$(...)` in PS1 prints is drawn as printed, where busybox's order made a newline of its
    `\n`; PS4 is decoded too.
  - **`export -a` and `export -A` in bash mode**, bash's, which mise's hook runs; busybox's
    illegal option, which ended the script, stays the answer as nemosh and sh.
  - **A double quote in `$((...))` is removed in bash mode**, as bash removes it -- starship's
    PS0 sets its timer with one -- where busybox, and nemosh and sh, call it a syntax error.
  - **`eval --` ends eval's options in bash mode**, so starship's `eval -- "$(starship init bash
    --print-full-init)"` runs no command named `--`, busybox's answer, which nemosh and sh keep.
  - **A write to a read-only variable in bash mode is bash's error**, `x: readonly variable`,
    and no longer ends the script: an assignment statement abandons the rest of its line, status
    1, a prefix assignment is passed over and its command runs, and export, readonly, declare,
    local, unset, `for`, `read` and `((...))` answer 1. busybox ends the script, status 2, and
    nemosh and sh still do.
  - **An operand that is no name, in bash mode, is bash's error**, `` export: `1x=2': not a
    valid identifier ``: the other operands are dealt with, status 1, and the script goes on;
    `unset 1x` is no error and `unset -v 1x` is. busybox's `bad variable name` ends the script,
    and still does as nemosh and sh. `declare` says it so under every name, and declares its
    other operands, where it stopped at the first.
  - **A special builtin's error in bash mode is bash's, which the script survives**: `.` of a
    file not there, a syntax error in eval's text, `set -Z` and local outside a function answer
    with bash's words and status and the line goes on; return outside a function says so,
    status 2, where it ended the script as exit does; and break or continue outside a loop says
    so. busybox ends the script, or says nothing, and nemosh and sh still do.
  - **An expansion that fails in bash mode abandons its line, as in bash**, status 1, and the
    script goes on: an arithmetic error whatever `set -e` says, a bad substitution and `${1:=x}`
    unless `set -e` ends it there, in bash's words. `${x?}` and `set -u` still end the script,
    with bash's status 1. busybox ends the script, status 2, and nemosh and sh still do.
  - **Builtins in bash mode say their failures as bash's do**, with bash's statuses: cd's
    `cd: DIR: No such file or directory`, 1, and `too many arguments`; type's and command -V's
    `not found` on stderr, 1; kill's usage and `not a pid or valid job spec`; `numeric argument
    required` for exit, return, shift and break, exit going on with the line as bash 5.3's does;
    alias's and unalias's colon; getopts's usage.
  - **Six of bash's semantics in bash mode**: `. FILE ARGS` keeps a list FILE set with `set --`,
    and gives the caller's back after a shift alone; `[[ A -eq B ]]` evaluates A and B as arithmetic; trap lists `SIGINT`; `unset f`, with no
    variable f, unsets the function; `x=1 >/bad` assigns x though the redirection fails; and a
    function named `a/b` is called by that name. Each is busybox's the other way, and stays so as
    nemosh and sh.
  - **`$-`, the `set -o` listing and alias's listing in bash mode are bash's**: `hBc` with bash's
    `-h` and `-B` and in bash's order, `set -o` by name with bash's tab, and `alias a='it'\''s'`.
  - **type and command -V in bash mode describe an alias and a function as bash's do**: "ll is
    aliased to `ls -l'", and a function's definition after `is a function`.
  - **getopts in bash mode is bash's**: OPTIND names the word until its last letter is read, a
    flag leaves OPTARG unset, `set --` and shift leave the parse where it was, and the
    diagnostics are bash's, `$0: illegal option -- z`. busybox's getopts stays nemosh's and sh's.
  - **A bash-mode script with a syntax error later in it runs up to the error**, read a command
    at a time as bash reads one, the error ending it there, status 2, with its EXIT trap; text after
    an `exit` is never read, so a self-extracting installer's archive after `exit 0` is no error,
    nor a file over the parser's 1 MiB. eval's text and a sourced file are read so too. As nemosh
    and sh such a script is refused whole, as before.
  - **A bash-mode script expands no aliases until `shopt -s expand_aliases`**, as bash's
    non-interactive shell; a session expands them from the start. busybox's expands them
    everywhere, and nemosh and sh still do.
  - **An alias in bash mode is substituted as the text is read**, as bash reads it, so one for
    `{`, `(`, a reserved word or half a loop is syntax: `alias e_='for i in 1 2 3; do echo '` and
    then `e_ $i; done` is a loop. One defined is in force from the next command, the rest of the
    text read again from there, and a function's body keeps the aliases it was read with, in a
    session and an rc file as in a script. nemosh and sh substitute as the command runs, as before.
  - **As nemosh and sh an alias that is syntax is refused with a hint**: the syntax error it makes
    is followed by `hint: alias LEFT='{' is part of a command's syntax, which nemosh does not read
    on purpose; bash mode does`, in a script, eval's text and at a prompt. Oils refuses such an
    alias too, and ShellCheck warns of one.
  - **A background job in bash mode lists the shell's traps, and runs ERR under `set -E`**, as
    bash's does: `{ trap; } &` shows them, without running them, and `{ false; } &` runs the ERR
    trap for the false, though `false &` does not. busybox's job lists only what it ignores.
  - **printf in bash mode writes what it read of a number**, as bash's does: `printf %d 3abc` is 3,
    and one too large is the largest the conversion holds, each said in bash's words, status 1.
    busybox writes zero, and nemosh and sh still do.
  - **umask in bash mode is bash 5.3's**: it starts at 0022, as Git's bash does; a symbolic mode
    takes X, s, t and a class to copy as POSIX has them, `umask =rx` meaning all three classes;
    and a mode refused is said in bash's words, status 1. nemosh and sh keep busybox's, 0002.
  - **Unquoted `$@` and `$*` in bash mode are joined by IFS and split**, as bash's are, so with
    IFS=x the parameters `ax` and `b` are a, an empty field and b, and `set -- one '' two;
    IFS=zx; for i in $*` keeps the empty one. busybox splits each parameter on its own, and
    nemosh and sh still do; an array's unquoted `${a[@]}` and `${a[*]}`, being bash's alone, are
    bash's under every name.
  - **`$@` where nothing splits it is joined by a space in bash mode**, whatever IFS is, as bash
    joins it: in an assignment, a case word, `[[ ]]`, a here-string and an operator's word, so with
    IFS=: `s="$@"` is `x y`. busybox joins it by IFS's first character, as `$*`, and nemosh and sh
    still do.
  - **The operands of break, continue, exit, return and shift in bash mode are bash 5.3's**:
    `break x` ends the script, status 2, where it said so twice and went on; `--` before an
    operand is passed over; a second operand is `too many arguments`, the rest of the line
    abandoned, status 2, and a `-c` command string ended, 1; `break 0` leaves every loop, status 1; `shift -1` is out of range,
    status 1; and break or continue in a function no longer reaches its caller's loop. busybox
    reads them otherwise, and nemosh and sh still do.
  - **A compound command's failed redirection in bash mode runs the ERR trap and `set -e`**, as
    bash 5.2's does: `set -e; { cat; } < nofile` and a loop's `done < nofile` end the script.
    busybox goes on, and nemosh and sh still do.
  - **An argument like an assignment has its tildes expanded in bash mode**, as bash expands
    them outside its POSIX mode: `make PREFIX=~/x` and `echo x=~:~` give the home directory.
    busybox and POSIX leave the tilde, and nemosh and sh still do.
  - **echo -e in bash mode decodes what bash's does**: an octal escape only with its 0, so `\1` and
    `\123` stay as written, and `\E` is escape. busybox's reads a bare `\NNN` too, and nemosh and
    sh still do.
  - **test and `[` in bash mode read their arguments as bash's do**: `[ -a file ]` is -e, `[ -z
    -a ] ]` the AND of two words, four or more arguments bash's grammar, and a refusal bash's words,
    `a: integer expected`, `x: unary operator expected`, `` missing `]' ``. busybox refuses the
    first two, and nemosh and sh still do.
  - **type and command -v in bash mode describe as bash's do**: `eval is a shell builtin`, POSIX's
    special ones `special` only under `set -o posix`; and command -v and -V describe every name
    they are given, 0 when any was found, where busybox's stop at the first.
  - **`${x?}` and `set -u` end a bash-mode command string with 127**, as bash 5.3's `-c` exits,
    and a function it calls; a script, standard input and a subshell still end with 1.
  - **read and wait in bash mode refuse what they cannot take as bash's do**: read's bad `-n`,
    `-u` or `-t` value is status 1, an option it has not got comes with its usage, and a bad name
    is `not a valid identifier`; `wait zzz` is `not a pid or valid job spec`, 1, and a pid that is
    no child is said, 127. `wait -f`, bash's, is taken under every name, where it was refused.
  - **A redirection's word in bash mode is expanded as an argument is**, as bash expands it:
    braces, splitting and globbing too, so `> one-*` writes the one file it matches, and a word
    that comes to none or several, `> $f` with a blank in f, is `$f: ambiguous redirect`, status 1,
    and the command does not run. A colon in IFS splits nothing there, so `IFS=:` leaves `> $f`
    with a drive in it whole, as MSYS's colonless paths are in Git's bash. busybox makes one path
    of it, and nemosh and sh still do, as bash mode does under `set -o posix`.
  - **A session in bash mode announces only its own jobs**: one a subshell, a `$(...)` or a
    pipeline stage puts in the background says nothing, as in bash, where atuin's `(atuin ... &)`
    printed `[1] PID` before every prompt. busybox-w32 announces them, and nemosh and sh still do.

- **`vi`** and **`view`**, busybox-w32's vi with vim's ways where the two differ on a key a
  Debian user knows, every case measured against both; `fc` with no `EDITOR` and `C-x C-e` in
  vi mode open it, where they found nothing. It moves -- `h j k l`, words, lines, brackets, paragraphs, `f t`,
  the screen, searches (`/ ? n N * #`), with counts, each landing where busybox's and vim's do --
  edits with the operators
  `d c y < >`, `x s D C r R J ~ p P o O`, registers and marks, undoes and redoes (`u ^R U`),
  repeats (`.`), runs ex commands (`:s :d :y :p :set` and line addresses), edits several files
  (`:n :e :r`), runs commands (`:!` and `:r !`), reads `~/.exrc`, selects (`v V`, vim's), inserts
  (`i a I A`, autoindent, `^W ^U`),
  writes and quits (`:w :wq :x :q :q! ZZ ZQ`) with busybox's status line and messages, and keeps
  a CRLF file CRLF. `vi -H` lists what it has; any other key says it is not implemented.
  `NEMOSH_OVERRIDE_APPLETS=vi` runs PATH's.

- **`zip`, Info-ZIP's**: `zip -r a.zip dir -x '*.o'`, `-j`, `-q`, `-D`, `-0`...`-9`, and an existing
  archive updated in place. There was unzip and no zip.

- **`man`, `apropos`, `whatis` and `info`**: this shell's own help as man-db would put it.
  `man ls` is `help ls`, paged on a terminal; `man -k` and `apropos` search the commands'
  summaries. There are no man pages on Windows to format.

- **`tput` and `tty`**: `tput setaf 1`, `tput sgr0`, `tput cols` and the rest of xterm-256color's
  terminfo, the VT the Windows console speaks; `tty -s`, and `not a tty` from a pipe. A
  terminal's name is refused for now, /dev/tty not yet being a device here.

- **`file`**, in Debian's libmagic's words: text by encoding and line terminators, scripts by
  their `#!`, Windows executables in full, and ELF, archives, compressions, images, PDF and SQLite
  by their headers; `-b`, `-i`, `--mime-type`, `-L`, `-N`. There was no file.

- **`pstree`, psmisc's**: `pstree`, `-p`, `-a`, `-A`, `-U`, `-l`, `-n`, `-T`, `-c` and a PID, with
  identical branches folded into `N*[...]` and threads as `{name}`. There was no pstree.

- **`column`, util-linux's**: `mount | column -t`, `column -t -s,`, `-x`, `-o`, `-R`, `-N`, and the
  columns of a list without `-t`. There was no column.

- **`ping`, iputils'**: `ping -c 4 HOST`, `-i`, `-W`, `-w`, `-s`, `-t`, `-q`, `-n`, `-6`, in iputils'
  words, with no administrator. It ran Windows' PING.EXE, which answers `-c` with `Option -c
  requires administrative privileges`; that one still runs as `ping.exe`.

- **`ss`, iproute2's**: `ss -tulpn` and the rest, every socket with the process that owns it,
  from Windows' socket tables. Recv-Q and Send-Q are `-`, which Windows does not say. There was
  no ss, and Windows' netstat answers `-tulpn` with its usage.

- **`ip`, iproute2's, to read with**: `ip a`, `ip -br a`, `ip link`, `ip r`, `-4`, `-6`, `-o` and
  `show dev NAME`, in iproute2's fields, from Windows' adapters and route table. A change is
  refused with the PowerShell command that makes it. There was no ip.

- **`tree`, tree(1)'s**: `-L`, `-a`, `-d`, `-I`, `-P`, `-h`, `-s`, `--dirsfirst` and the rest, in tree's
  UTF-8 lines. It ran Windows' tree.com, which answers `tree -L 2` with `Too many parameters`;
  that one still runs as `tree.com`.

- **`hostname`, Debian's**: `-I` every address but the loopback's, `-f`, `-s`, `-d`, `-i`, `-A`.
  It ran Windows' hostname.exe, which answers `-I` with `sethostname: Use the Network Control
  Panel Applet`; that one still runs as `hostname.exe`. Setting the name is refused with the
  reason.

- **`less +F`, `+G`, `+/PATTERN`, `+N`, the `F` key, and `:n`/`:p`/`:x`**, GNU's: start at the
  end, a match or a line, or following a FILE as it grows until ^C; several FILEs are paged one
  at a time, each kept where it was, where they were run together. `+F` was opened as a FILE.

- **`find -printf FORMAT`**, GNU's, measured against GNU findutils 4.11.0: the names, depth,
  size, mode, owner and identity directives, the times in every form GNU has, widths,
  precisions and escapes. `%F`, `%S` and `%Z` are refused. It was `unrecognized: -printf`.

- **`find -user NAME`, `-group`, `-uid` and `-gid`**: NAME as `ls -l` prints the owner, in any
  case, or a number as `ls -n` prints it. A NAME that is no account is refused before the walk.
  It was `unrecognized: -user`.

- **`cp --preserve[=LIST]` and `--no-preserve=LIST`**, GNU's: mode, ownership, timestamps, links
  or all, read in order with `-p` and `-a`, so `cp -a --no-preserve=timestamps` keeps the rest.
  `links` keeps a file with two names one file in the copy, and `-a` now does too, as GNU's `-a`
  is `-dR --preserve=all`. It was `unknown option`.

- **`diff -y` and `diff --color`**, GNU's side-by-side format and colours, byte for byte against
  GNU diffutils 3.12, with `-W`, `--suppress-common-lines` and `--left-column`; and GNU's long
  names for diff's letters, `--brief`, `--recursive`, `--ignore-case` and the rest. Each was
  `unknown option`.

- **`tail -f --pid=PID`**, GNU's: following stops once that process has ended, after reading
  what it wrote. head and tail take GNU's long options too, `--lines`, `--bytes`, `--quiet`,
  `--silent`, `--verbose`, and tail's `--follow[=name]`, `--retry` and `--sleep-interval`. Each
  was `unknown option`.

- **awk has `systime`, `strftime`, `mktime`, `and`, `or`, `xor`, `compl`, `lshift` and
  `rshift`**, which busybox's awk has and this one called undefined functions: `awk '{ print
  strftime("%F %T"), $0 }'` stamps a log. strftime takes every conversion `date` does, and both
  it and mktime take gawk's last argument for UTC.

- **`nc -zv HOST PORT`**, netcat-openbsd's port check, which is Debian's nc: whether each port or
  range takes a connection, said on stderr with -v, status 0 when any does. It was `unknown
  option -- z`.

- **`sed -z`**, GNU's `--null-data`: lines ended by a NUL, read and written, so `find -print0 |
  sed -z s/a/b/ | xargs -0` keeps names whole. It was `unknown option -- z`.

- **realpath takes GNU's options**: `--relative-to=DIR`, `--relative-base=DIR`, `-m`, `-e`, `-s`,
  `-q` and `-z`. busybox's takes none, so every word was a path and `realpath --relative-to=. x`
  looked for a file of that name.

- **`rm -I`**, GNU's: one question before removing more than three files or removing
  recursively, `rm: remove 4 arguments?`. It was `unknown option -- I`.

- **`mv -u`**, GNU's `--update`: a file is moved only over an older one, or where there is none.
  It was `unknown option -- u`; cp had it already.
- **`sort -R`**, GNU's random sort: an order that differs from run to run, lines with the same
  key kept together, `-k2,2R` as well. It was `unknown option -- R`.
- **`cut --complement`**, GNU's: what LIST does not name, `cut -d: --complement -f2`. It was
  `unknown option`.
- **`ls --time-style`**, GNU's: `full-iso`, `long-iso`, `iso`, `locale` and `+FORMAT`, a newline
  parting the format for an old file from the one for a recent one. It was `unknown option`.
- **df takes GNU's `-x`, `--output` and `-i`**: `df -hT -x tmpfs`, `df
  --output=source,size,pcent,target`, and `-t` more than once. `--output` and `-i` print GNU's
  columns; the inode fields are `-`, as GNU's df prints them for a filesystem with none, since
  Windows keeps no inode count an unelevated session may read.
- **free takes procps-ng's `-t`, `-w`, `-s` and `-c`**: a Total row, buffers and cache apart, and
  the table again every so many seconds or so many times, `free -h -s 2`. Each was `unknown
  option`. The unit is the first of `-b -k -m -g -h` given, so `free -th` is in -h's units; it
  read the first letter alone, and `-t` there would have meant kilobytes.
- **`uptime -p`**, procps-ng's pretty form that Ubuntu's uptime prints: `up 2 weeks, 3 days, 22
  hours, 15 minutes`. It was `unknown option -- p`. `--pretty` and `--since` are taken too.
- **top bundles its letters and takes procps-ng's `-p`, `-u` and `-o FIELD`**: `top -bn1`, `top
  -p 1234`, `top -u nemo`, `top -o %MEM`. Each was refused: the letters were matched a word at a
  time, and `-o` is this top's column list. An upper-case procps field, `-o %MEM` or `-o -PID`,
  is a sort now; a list of this top's lower-case columns is still the layout.
- **`date -d` and `touch -d` read GNU's relative dates**: `date -d yesterday`, `date -d '2 days
  ago'`, `date -d 'next friday'`, `date -d 'tomorrow 09:00'`, `touch -d 'last monday' f`. Each was
  `invalid date`. busybox's formats are tried first and read as before; the relative items follow
  GNU's parse_datetime -- calendar days and months, a day of the week at midnight, `ago` turning
  the item before it round.
- **`zgrep`, `zegrep` and `zfgrep`**, gzip's, which busybox has not got: `zgrep error
  /var/log/syslog*` reads `syslog.2.gz` and `syslog` alike. Data that begins with gzip's, bzip2's
  or xz's magic is read through zcat's decompressor and any other as it is, as `gzip -cdf` passes
  it; grep's options all apply, but `-r`, which gzip's zgrep refuses too.
- **`grep -z` and `grep -b`**, GNU's `--null-data` and `--byte-offset`: `find -print0 | grep -z
  x | xargs -0` keeps a name with a newline in it whole, and a NUL ending a line does not make the
  input binary; `grep -ob PATTERN` says where each match begins.
- **`grep -P` and `grep --line-buffered`**: `-P` is Perl's syntax as RE2 reads it, which is
  most of what people write with it -- `\d`, `\w`, `\b`, lazy quantifiers, `(?i)`, named groups --
  and lookaround and backreferences, which RE2 has not got, are refused by name, status 2.
  `tail -f log | grep --line-buffered x` was `unknown option`; each line was written as it was
  found already, and the option is taken now.
- **du takes GNU's long options**: `du -h --max-depth=1`, `du -sh --exclude=node_modules .`, `du
  --apparent-size`, `du -X patterns`. It took none, so each was `unknown option`. `--apparent-size`
  counts what files hold and rounds the sum up to the unit, as GNU's does; `--exclude` matches an
  entry's name, or with a slash in the pattern the path printed, and counts none of what it leaves
  out.
- **`pgrep -f` and `pkill -f` match the command line**, and pgrep and pkill take the rest of
  procps-ng's options that people type: `pgrep -af node`, `pkill -f 'node server.js'`, `pgrep -u
  nemo`, `pgrep -n`, `-o`, `-c`, `-d,`. `-f` was refused, as reading a command line needs a handle
  Windows will not give for another account's process; such a process is matched by its name
  now, as procps matches one whose command line is empty, which is the rule `ps` follows.
- **awk has `gensub`**, which busybox's awk has and this one called an undefined function:
  `gensub(/(a)(b)/, "\\2\\1", "g", s)` answers s with its matches replaced and leaves s as it was.
  As busybox reads it, `"g"` and any other text are every match, N the Nth alone, `\0` to `\9`
  and `&` the match and its groups.
- **`ps` takes procps-ng's options**: `ps aux`, `ps -ef`, `ps -eo pid,etime,args --sort=-rss`,
  `ps axf`, `ps -p $$ -o comm=`, with procps' columns, headers, widths and time formats -- `-e -A
  -f -o -p -q -u -U -C -N -H -w`, BSD's `a x u f w h r o p U k`, a bare pid list, and `--pid
  --ppid --user --sort --format --forest --deselect --no-headers --width`. USER is `?` and the
  command its image name in brackets where Windows will not let the process be opened, which is
  most of another account's; TTY is `?`. Nothing selected is status 1, so `ps -p $pid >/dev/null`
  asks whether it runs, and ps now lists the process it runs in. `-l` is refused with the reason.
  Without options it prints what it did. `ps aux` takes 90 ms over 524 processes, where
  busybox-w32's plain `ps` takes 150.

### Changed

- **A session sets aside a `TERM` it inherits**, as Git's bash exports `xterm-256color`: a
  session is the Windows console's, which sets none, so its programs and nemosh's own vi, nano,
  less and top take the console for what it is. It says so, `nemosh: set aside inherited
  TERM=xterm-256color; export TERM in ~/.nemoshrc to keep it`, unless the rc file sets one;
  `NEMOSH_DEFAULTS=off` keeps it, and a script's environment is passed through as it was. A
  `TERM` set in the shell now reaches those applets, which read the process's.

- **`ls` reads `LS_COLORS`**, as GNU's does, by type and by suffix, and **a session exports
  dircolors' database**, as Ubuntu's `~/.bashrc` does with `eval "$(dircolors -b)"`: an archive is
  red, a picture magenta, a sound cyan, a backup grey. Unset, ls keeps busybox's three colours;
  `NEMOSH_DEFAULTS=off` sets none.

- **`COLUMNS` and `LINES` at a prompt**: a session sets them to the terminal's size before each
  prompt, as bash's `checkwinsize` does, so `echo $COLUMNS` answers. `checkwinsize` was recorded
  and did nothing; `shopt -u checkwinsize` leaves them alone.

- **A session has Ubuntu's aliases**, which its `~/.bashrc` gives every account: `ll`, `la`, `l`,
  and `ls`, `grep`, `egrep` and `fgrep` in colour on a terminal. A script has none; the rc file,
  read after, can `unalias` any; `NEMOSH_DEFAULTS=off` in the environment is a session without
  them. The example rc file's own three are gone.

- **`bash` and `sh` are this shell**, as both are busybox-w32's: `bash build.sh`, `bash -c`, `sh
  -c`, and a script beginning `#!/bin/bash` or `#!/usr/bin/env bash`, whatever PATH holds. Git's
  bash ran where Git was installed, and where WSL was, its System32\bash.exe ran the script in
  Linux. Run as bash, `BASH_VERSION` and `BASH_VERSINFO` are bash 5.2's, so a script's `[ -z
  "$BASH_VERSION" ] && exec bash "$0" "$@"` runs it once more, not for ever.
  `NEMOSH_OVERRIDE_APPLETS=bash` runs PATH's again.

- **`less -R` draws colour**, as GNU's does: `git log --color | less -R` and `ls --color=always
  | less -R` show their colours, the 16, 256 and RGB ones and the attributes, and a search finds
  a coloured word. It took the sequences out, as busybox's `-R` does. With no terminal, `-R`
  copies them through.

- **`grep -w` of one plain pattern is 21 times as fast**: `grep -cw err` of 88 MB takes 151 ms,
  where it took 3.27 s and busybox's takes 0.98. The pattern is searched for and the bytes either
  side of it looked at, where the regexp -w made began with an alternation Go could not take a
  literal from; `-i` searches the line lowered, as it does without -w.
- **diff chooses the edit script busybox's chooses**: it is busybox's Unix diff of Hunt and
  McIlroy, ported, where it was a longest common subsequence of its own, which on 17 of 400
  generated pairs removed and added other lines than busybox did. Lines are classed by
  busybox's hash, so two lines that differ and share it are matched by neither; `-d`, which was
  taken and ignored, has stone try every line of a class, where it gives up after
  max(256, √n); and `-i` lowers A to Z alone, as busybox's does, so `Ä` and `ä` differ under it.
- **`read` of a pipe the shell was handed is twice as fast**: `seq 1 20000 | nemosh -c 'while
  read l; do ...; done'` takes 181 ms where it took 370. A read that could wait started a
  goroutine to end it on Ctrl-C; a command's goroutine is held to its thread now, which Ctrl-C
  reaches without one.

### Fixed

- **A compound command's redirection that failed was said on the wrong line**: `{ cat; } <
  nofile` on line 2 said `line 1`, the line of the command before it. It is said on the line
  the compound begins on, as busybox-w32 says it.

- **A compound command whose redirection's word failed to expand opened the empty path**:
  `{ echo in; } > ${x?oops}` said `x: oops` and then `cannot create : nonexistent directory`. The
  error is answered alone now, as a simple command's is and as both references answer it.

- **`trap - 99` said 99 was a signal this shell does not deliver**: a number below 1 or above 64 is
  no signal at all, an invalid signal specification, as both references say. In bash mode one
  operand that names no condition, `trap foo`, is bash's usage, status 2.

- **`"${X-it's}"` was printed as written**: an apostrophe that closes nothing in a double-quoted
  default is a character of it, `it's`, as busybox reads it; after a pattern operator,
  `"${foo%d'}"`, it is the syntax error both references make of it.

- **printf refused C's length modifiers**: `%ld`, `%zu`, `%hhd`, `%Lf` printed nothing from there on,
  status 1; they are passed over, as both references pass them over, and j and t with them, as bash
  does.

- **printf read a floating operand as Go reads a number**: `1_0` was 10, `0x1.8` and `0x10` were no
  number, and `'3.5 '` was 3.5 with status 0. It reads one as C's strtod does, as both references
  read it, and a blank before the end is no number. And a format that ends inside a conversion,
  `a%` or `%5`, writes what came before it and fails there, status 1; it was written as it stood.

- **A background job forgot what the shell ignores**: after `trap '' TERM` a job lists the trap and
  a `kill` of it is ignored, as POSIX has a subshell keep it and bash keeps it; the job started with
  no traps at all, and the TERM ended it.

- **`cat <<\` at a prompt was a syntax error**: a here-document's operator continued onto the next
  line takes its delimiter from it, at a prompt as in a script, as both references read it.

- **fc in a `bind -x` command left out the newest entry**, taking it for the fc's own line as at a
  prompt, so fzf's Ctrl-R never offered the command just run; it lists through the newest there,
  as bash's does.

- **`builtin fc` and `command fc` ran Windows' `fc.exe`**, so fzf's Ctrl-R, which reads the history
  with `builtin fc -lnr`, found none; both run the shell's fc now.

- **`[[ -t 1 ]]` said no on a console**, where `[ -t 1 ]` said yes: `[[` asked none of the shell's
  streams. bash-preexec's DEBUG hook returns at once when `[[ ! -t 1 ]]`, so no preexec ever ran,
  and atuin, which carries bash-preexec, recorded no history.

- **`! [[ A || B ]]` was refused**, `missing ]]`: after `!` the conditional's `||`, `&&` and `|`
  were taken for the pipeline's, so direnv's whole stdlib failed to parse, and direnv run with
  this shell as its bash loaded no `.envrc`. And `echo ! [[ x`, where `!` is an argument, opened a
  conditional that nothing closed.

- **`declare -f` printed `cmd < <(list)` as `cmd <<(list)`**, a here-document, so the function did
  not read back; and a background job, started from the functions as printed, failed with a
  syntax error once one such was defined -- carapace's completer has one, and atuin starts a job
  before every prompt. A process substitution is set apart from its operator, as bash prints it.

- **`bind '"KEYS": ""'` was refused** as an empty key sequence: an empty macro binds the keys to
  nothing, as atuin's init binds its chain key, three times over at every start.

- **`stty -echo`, `stty` and `stty size` failed on a real console**: `-echo` was read as the
  options e, c, h and o and refused, and the window was asked of the input handle, which
  Windows answers Bad file descriptor. Both work now, seen in a console the robot drove.

- **A login profile's alias was replaced by the session's default**: `alias ll='...'` in
  `~/.profile` gave way to Ubuntu's `ll`, which a login session set after its profiles. The
  defaults come before every startup file now, so any of them can redefine one.

- **`shopt -u promptvars` changed nothing**: a prompt, PS0 and `${x@P}` were expanded
  whichever way it was set, so a prompt drawn through `${PS1@P}` ran any command substitution a
  directory's name held. Off, a prompt is decoded and nothing in it expanded, as in bash.

- **`$(trap -p DEBUG)` was empty**: a subshell listed the shell's signal traps but not its DEBUG
  and ERR traps, which it does not run, so `saved=$(trap)` lost them and starship and
  bash-preexec could not find a DEBUG trap already set to run beside theirs. A subshell lists
  them until it sets or resets its own, as bash's does.

- **A trap's own commands set `$PIPESTATUS`**: with a DEBUG trap, `true | false; echo
  ${PIPESTATUS[*]}` said 0, and an ERR trap left its own status where the failed pipeline's
  should be. A trap leaves it as the command before it set it, as bash does -- starship's and
  bash-preexec's pipe statuses read it from such a trap.

- **A quoted newline in a `${...}` inside double quotes was a syntax error**:
  `a="${X-"one<newline>two"}"` refused the whole script as missing a `}`, fzf's `completion.bash`
  among them; the same without the outer quotes worked. It is one word across the line now, as in
  busybox and bash.

- **`(cmd &)` ran nothing**: a job a subshell started was ended with the subshell, or with the
  script once it had ended, so `( long-task & )`, the idiom for starting something nothing waits
  for, did nothing. It runs on now, as in busybox and bash; a Ctrl-C still ends it.

- **A prompt that failed to expand ran nothing typed at it**: with `PS1='${y?boom}> '` every
  command was abandoned for the error, `PS1=` too, so the session could not be put right; a
  failing PS0 cost the command it came before. The error is said, the prompt drawn as written,
  and the command runs, as busybox and bash have it.

- **A quote inside a prompt's `$(...)` was printed**: `PS1='[$(echo "hi")]'` drew `["hi"]`,
  `${y:-"a b"}` kept its quotes and `$(echo ")")` came apart, in PS1, PS2, PS0 and `${x@P}`
  alike. The quote is the substitution's now, as in busybox and bash: `[hi]`.

- **An applet run straight from the command line found no program**: `nemosh xargs prog`,
  `nemosh env NAME=v prog`, awk's `system()` under `nemosh awk`, said not found for any program,
  where the same command at a prompt ran it. They run as they do in the shell now.

- **A `return` at an rc file's top level ended the shell**: Ubuntu's `/etc/bash.bashrc` opens
  with `[ -z "${PS1-}" ] && return`, and given it as the rc a session closed before its first
  prompt. The rc file, `~/.profile` and `/etc/profile` are read as `.` reads a file now, so the
  return leaves only the file, as in bash and busybox's ash.

- **An interactive shell had no `PS1`**, so a startup file asking `[ -z "${PS1-}" ]` whether it
  was interactive was told no: Ubuntu's `/etc/bash.bashrc` returned at its first line, and its
  `/etc/profile` never read it. A session has `PS1` and `PS2` before its startup files now, its
  default prompts, unexported, as bash's has; the prompt looks as it did.

- **`nano -R` and `micro -R` could change the buffer**; only saving was refused. `^K` and `^U`
  cut and pasted lines, and the keys the text area binds for itself, `^D ^K ^W ^U ^X ^V ^Z ^Y`
  and Alt-Backspace, edited. Read-only lets through only the keys that move, scroll or select
  now, Alt-B and Alt-F among them, which it had refused.

- **`kill 13` ended process 12**: Windows ignores a pid's low two bits, so a pid no process has
  opened its neighbour, and a typo was a different process killed -- by kill, pkill, killall,
  timeout and top alike, and top's priority keys changed it. The process opened is checked against
  the pid now, and one that is not it is `No such process`. busybox-w32's kill still does this.

- **awk's `BEGINFILE` and `ENDFILE` rules never ran, and said nothing**: read as busybox reads
  them, a pattern on a variable nobody set, where the documentation said they were refused. A
  rule that begins with either is refused as the program is read now, `BEGINFILE is gawk's`;
  elsewhere the names are variables still, as they are in busybox's.
- **`which -a` failed with what it was asked for on PATH**: `-a` was looked for as a command,
  so `which -a python` printed one path and said 1. It lists every match now, the one that would
  run first, as busybox's does -- a builtin or applet by its name, then each program PATH holds
  under it -- and refuses any other option as busybox's does, `which: unknown option -- x`.
- **`${v//old/new}` took seconds on a long value**: it tried the pattern from every place to every
  place after it, making both into runes each time, so replacing in 3,000 characters took 5.8 s and
  215 MiB, where busybox takes 39 ms. A pattern with nothing of a pattern's in it is searched for:
  6 ms. The other patterns are matched as before.
- **A pattern with a character outside ASCII matched nothing** in grep's and sed's basic regular
  expressions, in an escape of awk's and in `find -path`: `grep 中`, `grep -w 中文`, `sed s/中/X/`
  and `sed -n '/测试/p'` found and changed nothing, `find -path '*文件*'` found no file, and
  `awk '/caf\é/'` was `invalid UTF-8`. Each byte was copied as the character of that number, `中`
  as `ä¸­`. They match as busybox-w32's do; `grep -E` and `-F` were right already.
- **comm and paste read every operand whole first**, so neither wrote a line until its inputs
  ended, and `paste - -` of 19 MB peaked at 109 MiB, comm of two 4 MB files at 48, where busybox's
  peak at 1.1. Both read a line at a time now, at 20 MiB, and run in a third to a half of the time
  they took: that paste in 89 ms where it took 304, that comm in 45 where it took 205.
- **uniq wrote nothing until its input ended**, so `tail -f log | uniq` showed nothing, and
  `uniq -d` of 19 MB peaked at 66 MiB where busybox's peaks at 1.1. It writes each run as the next
  begins, its output going out before any read that may wait, and peaks at 19 MiB in 61 ms where
  it took 102.
- **base64 and base32 read their whole input before writing a byte**: `base64 -d` of 26 MB
  peaked at 161 MiB of commit, where busybox's peaks at 1.1, as the input was read, cleaned into a
  copy and decoded into a third. Both stream now, at 13 MiB, the Go runtime's own, and faster:
  that decode takes 131 ms where it took 246, and encoding 19 MB 33 ms where it took 216. A decode
  that meets invalid input writes what it decoded before it, then says `invalid input`, as GNU's
  does; it wrote nothing.
- **`for f in <(echo a) <(echo b)` read only the first**: both files are made before the first
  turn, and the first command of that turn removed them both, so the second turn's `cat` said
  `No such file or directory`. They last the loop now, as bash's do.
- **`while read l; do ...; done < <(cmd)` left its file in %TEMP%**, one each time it ran: the
  first command inside the loop removed every substitution so far, the loop's input among them,
  which Windows refuses while it is open, and the file was forgotten. 306 had piled up on the
  machine this was found on. Each construct removes its own substitutions when it is done, as
  `>(cmd)`'s pipes are let go of.
- **Ctrl-C did nothing to a command waiting on a share that had stopped answering**: Ctrl-C
  one second into `ls //192.0.2.30/share/x`, where nothing answers, gave the prompt back 102.4 s
  later, when Windows gave up, and busybox-w32's back in 17 ms. An applet runs in the shell's
  process, and the goroutine waiting in CreateFile did not see the interrupt. The goroutine
  running a command is held to its thread now, and Ctrl-C cancels the call that thread waits in,
  and each after it that begins to wait: the prompt is back 21 ms after it for that `ls`, whose
  stat waits in two calls in turn, and 5 ms for `cat` -- at the prompt, in a script, in a
  pipeline and for an applet run directly -- and the command says nothing of the call.
  A script's INT trap runs afterwards, and a `read` in it waits for its answer, and `tee -i` goes
  on reading. Windows ends most such waits, not every one -- on one of three CI runners the same
  stat went on waiting -- and one it does not end lasts as it did. `NEMOSH_PIN=off` leaves every
  call to wait for Windows, as before.
- **diff of two large files never finished**: it filled a table of every line of one against
  every line of the other, which for Go's ssa/opGen.go against rewriteAMD64.go is 103 GB, and
  one such diff had committed 9.4 GB when it was found eleven hours on. It takes 0.18 s and
  54 MiB, where busybox takes 0.56 s, and prints what busybox prints, byte for byte. Its output is
  written a buffer at a time, where a write a line took that diff 24 s.
- **awk's `delete` walked the whole array**, so emptying one of 327,666 elements in a
  `for (k in c) delete c[k]` had not finished after 90 s, where busybox's takes 3. It takes 2.
- **awk's arrays held the lines their keys came from**: `c[$1]` kept the whole line `$1` was cut
  from, two of them a key, as Go's map keeps the string it is handed. A key is copied once, an
  element set again is written where it stands, and a value is 32 bytes where it was 40.
  Counting 88 MB of Go source by first field peaks at 96 MiB where it peaked at 132, and keeping
  19 MB of lines in `a[NR]` at 116 where it was 155; both run a little faster. busybox's take 39
  and 75: Go lets its heap reach twice what it keeps before collecting, a setting of the whole
  process.
- **diff of many like lines wanted 2.6 GB**: a 19 MB file against its sorted copy peaked at
  2,661 MiB, where busybox's peaks at 701, as each of Hunt and McIlroy's candidates took 24 bytes
  in a list copied whole to grow. They are busybox's 12 bytes, in blocks never copied, and that
  diff peaks at 564 MiB and takes 25 s, where it took 29 and busybox takes 42.
- **`diff -U0` numbered a hunk of additions alone `-0,0`**, which says before the first line,
  and a hunk of removals alone `+0,0`. Each is numbered by the line it follows, `@@ -2,0 +3,2 @@`,
  as busybox numbers it.

## v1.6.0 - 2026-11-19

Quicker, and wget and cp as busybox's are. A start loads the nine DLLs a bare Go program loads,
where it loaded seventeen; on 92 MB of Go's source `tr a-z A-Z` takes 0.12 s, `grep -ci` 0.24 s
and `cmp` 58 ms, where busybox-w32 takes 0.55, 1.23 and 1.4 s, sort peaks at 152 MB where
busybox's peaks at 193, and `ls -l` of System32 takes 213 ms where it took 277. wget saves under
the name asked for, shows every redirect under `-S`, opens `-O /dev/null` and draws busybox's
progress meter; cp onto a network share copies by itself, so a link dropped mid-copy leaves a
short file rather than one of the full size whose end is zeros; Ctrl-C no longer prints Go's
error text; and completion specs are kept in the XDG layout, which `nemosh --migrate` moves them
to. Against v1.5.0, both as CI built them and measured on one machine with tools/startbench:
`nemosh -c true` takes 5.8 ms where it took 8.1, 2,000 commands 17.8 ms where they took 20.0, and
a session waiting for input commits 13.3 MiB where it committed 13.7; the binary is 11,254,272
bytes, 36 KB more than v1.5.0's. Oils: 2152 of the 2548 cases bash passes, 84.5%, as before.

### Changed

- **Your completion specs are kept in `~/.config/nemosh/completions`**, or
  `$XDG_CONFIG_HOME/nemosh/completions`, the XDG layout under the home directory, on Windows too,
  where fish and bash-completion keep theirs. `%APPDATA%\nemosh\completions`, where releases up
  to 1.5 looked first, is looked in after it, so each spec is found where it is; a session says
  so once when one is there, and `nemosh --migrate` merges the directory into the new one.
- History stays in `~/.nemosh_history`, and `~/.nemoshrc` and `$ENV` stay where they were: a
  shell's own files are where a shell's are, as bash's and busybox's are. The nightlies 1.5.1 to
  1.5.3 moved history to `~/.local/state/nemosh`; a file 1.5.2 or 1.5.3 left there is used when
  there is no `~/.nemosh_history`, a session says so once before its first prompt, and
  `nemosh --migrate` moves it back. 1.5.1 named it `history` and lasted under an hour; that file
  is not looked for.

- **Every start is about 2.5 ms quicker and 1.3 MB smaller**: USER and LOGNAME come from
  `%USERNAME%`, or GetUserNameW, and `$UID` from ntdll, where os/user and advapi32 loaded eight
  DLLs into every process, `nemosh cat` and every job's included. A started shell has the nine a
  bare Go program has. id and whoami name the same user as before.
- **tr is 18 times faster with ASCII sets**: `tr a-z A-Z` of 92 MB takes 0.12 s, where it took
  2.2 s and busybox takes 0.55. A byte table, 64 KiB at a time, answers what the character-at-a-time
  path answers, which UTF-8 sets and `-c` still take.
- **cmp is ten times faster**: 92 MB alike in 58 ms, where it took 568 ms and busybox takes 1.4 s.
  What both files hold alike is passed over a block at a time; the byte where they part, `-l`'s
  list and the EOF are as before.
- **grep holds 10 MB less**: a line it does not print is matched where the reader holds it, and is
  never made a string. `grep -c func` over 92 MB collects once where it collected 36 times, peaks
  at 14.6 MB where it peaked at 25, and takes 240 ms where it took 310.
- **grep -i of a plain pattern is seven times faster**: `grep -ci error` of 92 MB takes 235 ms,
  where it took 1.76 s and busybox takes 1.23. The line's ASCII is lowered and searched as bytes;
  a pattern with a k or an s, whose case folds outside ASCII, still asks the regexp of a line
  with anything but ASCII in it, so the answers are the same.
- golang.org/x/text/width is no longer linked: which characters are wide is a table of 120
  ranges generated from it (`go generate ./internal/textgrid`), which says the same of every rune.
  The binary is 15.9 KB smaller. The table is Unicode 15.0.0, which x/text answers for under the
  Go in go.mod, the one the release is built with, so a character is as wide as v1.5.0 said; x/text
  answers for 17.0.0 under go1.27, and the generator now refuses to run under any Go but go.mod's.
- **sort holds half what it did**: 92 MB of lines peaks at 152 MB, where it peaked at over 270
  and busybox's at 193. A file is read whole in one read of its size, each line is a view into
  that block, and the first file's lines are not copied a second time.
- **ls -l is a quarter faster**: an entry the listing has described is not asked GetFileAttributesEx
  again; its size and times come from the handle its owner is read through. `ls -l` of System32
  takes 213 ms where it took 277, and `ls -lR` of Go's source 1.22 s where it took 1.58.

### Added

- **`nemosh --migrate [--dry-run]`** moves what an older nemosh kept elsewhere to where this one
  keeps it: the completion specs, and a nightly's history. `--dry-run` prints the plan and touches
  nothing; a move never overwrites -- the same file on both sides removes the old copy, a
  different one keeps both and ends with status 1 -- and is copied, checked by size and SHA-256,
  and only then removed when it crosses volumes. A second run has nothing to do, and it is
  refused while another nemosh is running.

- **wget draws busybox's progress meter** on a console unless `-q` is given: the file's name,
  how far through it is, a bar as wide as the console leaves, the size so far and the time left,
  `- stalled -` when nothing has come for five seconds, redrawn once a second. busybox draws it
  on a pipe too; this one only where somebody is watching.

### Fixed

- **A cp onto a share that the link dropped under could leave a file of the full size whose end
  was zeros**, which looks like a finished copy: CopyFile2 sizes the file before it copies, and
  Windows closed it once the link came back. cp copies to and from a share itself now, a
  megabyte at a time, so a copy cut short is a short file, as busybox's is, and Ctrl-C ends it at
  the next megabyte rather than after the piece CopyFile2 had under way, which was 24 s on a link
  at 1 MB/s. mv across volumes still copies by CopyFile2, which keeps a file's streams; when that
  fails and what it made cannot be removed, it says what CopyFile2 met rather than `No such file
  or directory`.
- **`wget -O /dev/null`, `-O C:/dir/file` and an absolute `-P` were refused**, as `reserved
  device name`, `drive-qualified entry name` or `absolute entry name`, and so were ftpget's
  LOCAL_FILE and uudecode's `-o` spelled that way: a name the user typed was held to what an
  archive entry may name, since 2026-08-23. They are opened as written, as busybox's are. The URL's
  last element and a uuencoded file's begin line, which come from the other end, are still
  checked. `-P` no longer makes its directory, and applies only without `-O`, as busybox's.
- **wget named a file by where its redirects ended**, so a GitHub release asset was saved as
  `823ee083-9620-4fe0-bedf-fa097b85b694`. It is named by the URL asked for, as busybox names it
  before it sends a request: `nemosh-v1.5.0-windows-amd64.zip`.
- **`wget -S` showed only the last response**, so a chain of redirects was invisible. It prints
  each response's status line and fields as it arrives, a 301 or a 302 too, as busybox does.
- **httpd's listing linked a name with `#` or `%` in it to nothing**: the name was the href as it
  stood, so `a#b.txt` asked for `a` and was not found, and `100%.txt` was a bad escape and a 400.
  Each link is the name percent-encoded, a directory's slash after it.
- **Ctrl-C printed Go's own error text**: `cp: context canceled`, `mv: context canceled`, and
  `wget: use of closed network connection`. An applet that Ctrl-C or a signal to its job ended
  says nothing now, as a busybox applet the signal ends says nothing, and the status is still
  130, or 128 and the signal. A refused connection is `cannot connect to remote host
  (127.0.0.1:1): Connection refused`, busybox's words with the port, where it was `cannot fetch
  URL: dial tcp 127.0.0.1:1:` and Windows' sentence about the target machine. nc, ssl_client and
  whois give the same reason after the address.
- **`ls -l` on a NAS called the owner `root`**: the files' owner is the NAS's own account,
  S-1-5-21-<its domain>-1000, which this machine cannot name, and a name that could not be looked
  up was root. It is its number, `1000`, as busybox prints it and as this build's `stat` and
  `ls -n` already said.

## v1.5.0 - 2026-11-18

Windows only, in three architectures, and built on Windows itself: TLS is Schannel's, sockets
Winsock's, checksums and random bytes CNG's and code pages the system's, as busybox-w32's are,
and windows/arm64 and windows/386 are built and tested beside windows/amd64. Against v1.4.0,
measured on one machine with tools/startbench: the binary is 11.0 MB where it was 15.7, a
session waiting for input commits 13.2 MiB where it committed 47.0, package init allocates 414
times where it allocated 2,732, and `nemosh -c true` takes 8.7 ms where it took 25.9 -- 2,000
commands 19.9 ms where they took 54.2. And day-to-day use: Tab, the prompt, startup, and what
the shell says when a name is not found. Oils: 2153 of the 2548 cases bash passes, 84.5%.

### Changed

- **Windows is the only platform.** Nemosh no longer builds for Linux or macOS, and no
  release carries their `.tar.gz` archives or the `.deb`, `.rpm` and `.apk` packages, which
  were an unsupported convenience; v1.4.0 and the nightly `v1.4.66-master-b9608f763100` are
  the last that have them. What was written for those platforms goes, and a Windows API is
  called directly where one does the job, as busybox-w32 does.
- **wget follows redirects, credentials and timeouts as busybox's does**: sixteen redirects,
  Basic credentials from the URL and not to another host, `-T` for the connection and each
  read, 900 seconds unless given -- it was thirty for the whole download, which a large one
  outlived -- and `-S` prints every header field. **httpd answers ranges and dates** without
  net/http, and refuses an oversized request with 414 or 431. **ipcalc's PREFIX** is the bits
  a holed netmask has set, and `-h` for an address with no name fails, as busybox's does. The
  sockets beneath them are Winsock's, and the binary is 1.3 MB smaller.
- **TLS is Windows's own, Schannel**, as busybox-w32's is, for `wget` and `ssl_client`: TLS
  1.3 where Windows has it, the system's roots as before, and a refused certificate says why
  -- `signed by unknown authority`, `not valid for NAME`, `has expired`. When `ssl_client`'s
  input ends it no longer sends close_notify, as busybox's does not. The binary is 1.9 MB
  smaller again.
- **`tail` reads a file back from its end**: `tail -n1` of a 27 MB file takes 9 ms where it
  took 37 (busybox 32), and a file with a line too long to scan is tailed rather than refused.
- **`chmod -R` reads and writes only what it must**: over 3,000 files `chmod -R 644` takes 14
  ms where it took 228 (busybox 352), since a file's first bytes are read only for `-v` and
  `-c` to say its mode, and its attributes written only when the read-only bit changes.
- **mv to another drive keeps what a rename keeps**: a file's alternate data streams and its
  hidden and system bits, which the copy across volumes dropped, as well as its time; and
  Ctrl-C ends a long one.
- **A session commits 13 MB, where it committed 45**: the 32 MB Go's crypto packages set
  aside in every process, and never touched, went with them. Random bytes are RtlGenRandom's
  and the checksums CNG's now, as busybox-w32's are; output is the same, byte for byte.
- **iconv converts as busybox-w32's does, through Windows's own code pages**: every code
  page Windows has can be named, by busybox's 358 aliases or by number, and `iconv -l` lists
  those aliases. A character the target has not got is `conversion error: Illegal byte
  sequence` after what came before it is written, input that is no character is too, and
  `-c` passes over both, where it put `\x1a` in for one and U+FFFD for the other. A name
  Windows has not got is `iconv_open error: Invalid argument`, busybox's words. The binary
  is 760 KB smaller for the tables that went.
- **Paths are reported as busybox-w32 reports them**, `C:/Users/nemo`: `~`, `$HOME`,
  `pwd`, `$PWD`, `$OLDPWD`, `dirs`, the prompt and `realpath`. A native program handed
  `~/x` can now open it; `nvim ~/.ssh/config` opened `\c\Users\...`. `NEMOSH_PATHS=msys`
  keeps `/c/Users/nemo`. Every spelling is still read.
- **Ctrl-Z at the prompt turns the line's backslashes into slashes**, as busybox-w32's
  editor does, so a pasted `C:\Users\nemo` becomes a path. `NEMOSH_CTRL_Z=eof` makes it
  end input on an empty line, as before.
- **awk's `sub` and `gsub` read backslashes in a replacement as busybox-w32 does from
  FRP-6354**, which follows gawk's actual behaviour: `\\` is two backslashes.
- **`nano +42 file` opens at line 42**, and `+LINE,COLUMN` at a column of it, as nano does;
  it was taken for a second file and refused.
- **`cd D:` changes drive**, to where D: was left, as busybox-w32 has it: `D:`, `D:foo` and
  `D:..` are relative to the directory each drive was last at, its root if it has not been
  visited, and a program the shell starts is handed those directories, so `cmd /c "cd D:"`
  agrees. They were refused, "drive-relative Windows paths are not supported". At a prompt,
  a `cd` that fails on `C:Users` says that `C:\Users` unquoted loses its backslash.
- **grep takes GNU's `--include`, `--exclude` and `--exclude-dir`** for what `-r` reads, and
  `-a`, `-I` and `--text`. A binary file that matches is said to on stderr, `grep: FILE:
  binary file matches`, as GNU's is, where its bytes were printed at the terminal. And
  `--color` colours, GNU's colours, where it was accepted and ignored: `alias grep='grep
  --color=auto'`, nemoshrc's example among them, does what it says.
- **less draws what has arrived while more is coming**, as busybox's does, so `make | less`
  and `tail -f log | less` show the first page at once; with no terminal it writes a line as
  it comes. `-R` takes colour sequences out, as busybox's does, where they were drawn as they
  came; a control character is its letter reversed, a tab goes to the next stop of eight,
  and a wide character takes two columns.
- **nano's `^X` asks "Save modified buffer?"**, as nano does: `Y` writes the buffer under
  the name it offers, Enter accepting it, and leaves; `N` leaves without; `^C` goes back.
  `^O` asks for the name the same way, so `^O` and Enter writes, and another name writes a
  copy. micro's `^Q` asks micro's question. A second `^X` used to discard the buffer.
- **The example rc loads gh's and git's completion the first time Tab is pressed** after
  either, not at every start: sourced eagerly they cost a shell about 117 ms to open, and
  now about 1. `docs/examples/nemoshrc` shows the `complete -D` loader.
- **A session reads `~/.nemoshrc` where `ENV` is not set.** `ENV`, where it is set, is
  still what is read; `docs/examples/nemoshrc` is one to start from.
- **Where a program was found on PATH is remembered**, as busybox's ash remembers it, and not
  looked for again at every run: with 31 directories on PATH, 1000 runs of `command -v git`
  took 3.7 s and take 45 ms, where busybox takes 0.14 s, and each run of a program late on
  PATH starts about 5 ms sooner. `hash` lists what was found and `hash -r` forgets it, which
  they did not do; any assignment to PATH forgets it too. As in both references, a program
  put in an earlier directory of PATH since is not what runs until `hash -r`.
- **Ctrl-C at the prompt is the program's to handle**: the console sends it to the program
  as it sends the shell, and the shell now waits for the program to end rather than ending
  it, so ping prints its statistics and a nested cmd, pwsh or python stops what it runs and
  stays open, where it was closed. busybox-w32 ends the program, and a script's still is, so
  Ctrl-C stops a script at once. `NEMOSH_CTRL_C=kill` ends it at the prompt too, as busybox
  does, and `NEMOSH_CTRL_C=program` leaves it to the program in a script as well.
- **`ls` asks each file once**: `ls -l` read a file's first bytes twice and opened it a third
  time for its owner, and a plain `ls` stat'd every entry for `-L`'s sake. Over System32,
  `ls` takes 17 ms where it took 88, and `ls -l` 247 where it took 433 (busybox 379); `ls
  -lR` of Go's source tree 1.5 s where it took 2.6 (busybox 1.7). And `ls -l` matches
  busybox-w32's listing line for line: a subdirectory's size was 0, one this account cannot
  open showed others' permissions, and a junction had one link where `ls` and `stat` both
  count its target's subdirectories and two, as busybox does.
- **du takes a file's size from its directory's listing**, as it takes which file it is, where
  it stat'd every entry and opened every one: `du -s` of Go's source tree takes 167 ms where it
  took 806 (busybox 1.6 s), and of 15.5 GB of scoop's apps 0.8 s where it took 3.3 (busybox
  6.6), every line of `du -a` the same. `rm -r` asks nothing of what its directory listed, which
  it stat'd twice: 2,000 files in 126 ms where it took 174 (busybox 353).
- **`${x#pat}`, `##`, `%` and `%%` are a search where the pattern is a literal with a `*`
  around it**, `${p##*/}` and `${f%.*}` among them, and any other pattern is matched without
  converting it again for each cut point: 10,000 `${p##*/}` of an 80-character path take 90 ms
  where they took 279 (busybox 83, bash 136). Finding the operator in `${name...}` reads the
  byte there rather than trying 23 operators at every byte of the name, so a loop of short
  trims takes 111 ms where it took 135 (bash 139).
- **Ctrl+Backspace deletes a word, and Ctrl+Delete the word after the cursor**, as in
  PowerShell, where Ctrl+Backspace deleted a character and Ctrl+Delete nothing; Shift+Left and
  Shift+Right move a character, where they moved a word. Ctrl+Backspace arrives as `^H`, which
  a terminal set to erase with `^H` sends for Backspace too; `NEMOSH_CTRL_H=backspace` makes it
  one there.

### Added

- **windows/arm64 and windows/386 builds**, beside windows/amd64, each tested on CI as
  itself: arm64 natively on GitHub's Arm runner, 386 under WOW64. A Snapdragon machine runs
  nemosh without emulating x64, and a 32-bit Windows 10 runs it at all. Running the suite as
  386 found what was right only because an int was 64 bits -- an array index, head's and
  wc's counts, `date -D %s`, `expr substr`, `top`'s process table -- and each is 64 bits on
  every build now. 32-bit Arm is no target Go has had since 1.26.
- **wget reads its proxy from the shell**: `http_proxy` and `https_proxy`, as busybox's does,
  from what the shell exports rather than the environment nemosh started with, so
  `http_proxy=... wget URL` works; `no_proxy`, a CONNECT tunnel for https, and `-Y off` or
  `--proxy off` to go direct. Only an `http://` proxy is spoken to.
- **`title`**, busybox-w32's builtin: `title TEXT` sets the console's title, a tab's in Windows
  Terminal, and `title` prints it. Where the console cannot be asked it says why, with status
  1, where busybox's says nothing.
- **Windows Terminal's shell integration**: in Windows Terminal a session marks each prompt,
  each command and how it ended (OSC 133), so the scrollbar shows them and a command and its
  output can be jumped to and selected, and says its directory (OSC 9;9), where a duplicated
  tab or a split pane now opens. `NEMOSH_SHELL_INTEGRATION=never` turns it off. A prompt
  holding an escape ended `ESC \` is measured right; its backslash was a column.
- **Tab completion**: paths in every spelling `cd` reads (`/c/...`, `/mnt/c/...`,
  `/tmp/...`); a program typed by its path (`./deploy<TAB>`); inside a quote already
  opened (`"My Do<TAB>`); `$VAR` and `${VAR`; the command after `sudo`, `env`, `xargs`,
  `time` and the other wrappers; and PATH programs by the launcher's own suffixes, `.sh`
  among them.
- **Bracketed paste**: what is pasted goes into the line as it is, newlines and tabs
  too, and runs when Enter is pressed. A line holding a newline is drawn on its rows.
- **`disown`, `logout` and `enable`** as bash has them; `suspend` is refused with a
  reason. **`help NAME`** and **`NAME --help`** describe a builtin.
- **`command_not_found_handle`**; at a prompt, "did you mean" for a name one or two
  edits from one this shell can run; and a hint for a PowerShell script.
- **`HISTIGNORE`** and **`HISTTIMEFORMAT`**; history files in bash's format, with
  `#epoch` lines, read and written, and a command of several lines kept whole under
  HISTTIMEFORMAT.
- `UID`, `EUID` and `OSTYPE`; `sh` and `bash` fall back to this shell when PATH has none;
  `env`, `xargs`, `find -exec` and awk run programs; `time` before a compound command;
  `#!/usr/bin/env NAME` scripts; `trap ... KILL` and bash's `trap -p` listing.

### Fixed

- **`tail -n 1000000000000` of a pipe ended the shell**, and so did `head -n -1000000000000`:
  each made room for that many lines before reading one, 16 TB. They hold what has arrived.
- **`iconv -c` kept a `?` now and then** that it should have passed over: the flag Windows
  sets when it puts in the default character was handed over as a number a statement before
  the call, so a stack that grew moved the flag away from where Windows wrote it -- into stack
  that had been given back. A test of the tree now refuses that shape anywhere.
- **`${x:1:9223372036854775807}` was nothing**, the length added to the offset and wrapped;
  it is the rest of the value.
- **Ctrl-C in nano or micro left the editor at once, and the buffer with it**, unasked:
  tview stops an application on Ctrl-C by itself. It cancels a question now, and at the
  buffer says where the cursor is, as nano's `^C` does.
- **A case or a function definition inside a group, a subshell or a function made parsing
  quadratic**: every bracket read the group from its start to ask whether it was a case
  pattern's. A case of 1000 arms in a function took 593 ms to parse and takes 22;
  `git-completion.bash` takes 36 ms rather than 54, which an rc that sources it pays at
  every start. A word was also built a byte at a time by copying it, which a long one paid
  for quadratically; its run of the line is sliced now.
- **A command in a loop did work it did not need**: a plain word went through brace
  expansion and the field builder to come back as itself, every command made a new
  `$PIPESTATUS`, and every variable read in arithmetic made a context and an error. A loop
  iteration of `[ $i -lt N ]; true; i=$((i+1))` allocates 36 times rather than 108, and ten
  thousand of them take 72 ms rather than 103.
- **Every start was 14 ms slower than it needed to be on Windows**: `$PPID` was found by
  walking a snapshot of every process on the machine. It is asked of the process itself
  now, and `nemosh -c true` takes 11 ms rather than 25; applet shims gain the same.
- **`exec 3>log` inside a function, eval, `.` or a `{ }` group is still in force after
  it**, as busybox-w32 has it; fd 3 was gone the moment the call ended.
- `${a[0]%[0-5]}` and other element operators with brackets in their word.
- A subscript that leads back to its own name is reported as recursion rather than
  overflowing the stack; sed's pattern and hold spaces are bounded under fuzzing.

### Tested

- **The start is timed in CI**, against the commit before, interleaved on one runner;
  `tools/startbench` reports it beside the package init allocations it holds to a
  ceiling.
- **Package init does a sixth of the work it did**: 414 allocations rather than 2,732. The
  applets' help text is built when asked for, the registry allocates once rather than per
  applet, no package-level map literal is left -- a test refuses one -- and tcell's
  `gdamore/encoding` is forked into `third_party/` with its character sets generated as data.
  The ceiling CI holds it to is 780, down from 2750.

## v1.4.0 - 2026-10-07

Bash compatibility, measured case by case. The Oils project's spec suite is vendored and
runs in CI, and nemosh passes **2140 of the 2548 cases bash 5.3 passes, 84.0%**, where it
passed 1488 of 2551, 58.3%, the day the suite went in. The order of authority is the
project's: where busybox-w32 has the construct, busybox decides, and of the other 408
cases 117 are answered the way the files record of ash. The bash corpus grew from 113
cases to 318 and the parser corpus to 182; one bash gap is left, `$BASHPID` inside a
subshell, and no parser gap.

### Changed

- **The shell's messages name the script and the line**, as bash's do:
  `script.sh: line 4: cd: cannot cd to nope: No such file or directory`. At a prompt they
  say `nemosh:` alone.
- **A command abandoned for an error takes the rest of its line with it**, status 1, and
  the script goes on with the next, as bash's does: a pattern that matches nothing under
  `shopt -s failglob`, an assignment that cannot be made, an indirection that cannot be
  followed, and `${x:1:-10}`, whose length ends before its offset,
  `substring expression < 0`.
- **More of what POSIX makes fatal ends a script**, as busybox ends it: a special
  builtin's usage error and its failed redirection, a syntax error in what `eval` or `.`
  reads, a `.` FILE that cannot be read, `unset` of a name no variable can have, `local`
  outside a function, and an arithmetic error in an array's subscript. An error in a
  builtin that is not special ends only the builtin.
- **busybox's words and statuses.** A name `type` or `command -V` does not find is
  `x: not found` on stdout and 127, and `command -v`'s is 127; a read-only variable
  refused is `x: is read only`; `wait` for a pid that is no child says nothing; `cd` that
  fails is `cannot cd to DIR` and 2; a count that is no number is `Illegal number`. The
  phrasing is nemosh's own throughout, `cannot` where busybox says `can't`.
- **On Windows a shell's umask starts at 0002**, busybox-w32's, so `ls -l` and `stat` show
  a file 0664 as busybox's do; it was 0022. On Linux and macOS it is the process's own,
  and it reaches the files the shell, its applets and its children make.
- **On Windows `PATH` takes a colon between its directories** as well as a semicolon, as
  busybox-w32's does, and a `/bin/NAME` that is not there runs NAME.
- **A Windows program's CRLF is a newline** to command substitution and field splitting:
  `x=$(where git)` has no carriage return in it.
- **sed, cut, head and tr flush their output when their input runs dry**, not at every
  line: a file is written a chunk at a time, and `tail -f log | sed ...` still shows a
  line as it comes.
- **ed writes a CRLF file back as CRLF**, every line of it, a line added included; a file
  of mixed endings goes back byte for byte.
- **lsattr and chattr end 0** when a FILE fails, as busybox-w32's do; **clear** writes
  `ESC[H ESC[J`, and Ctrl-L the same; **awk** ends 1 for an input file that will not open.
- **`set -e` does not reach into a command substitution**, unless
  `shopt -s inherit_errexit` asks, as in both references.

### Added

- **Measurement.** The Oils spec suite, vendored under `tests/oils` with its licence, and
  a harness that runs it on nemosh, bash and busybox; CI holds nemosh to its baseline,
  strict on Windows. `docs/testing/oils-spec.md` has the numbers, file by file.
- **The language.** Namerefs, `declare -n`; `$"..."`; `${x~}` and `${x~~}`; `${var@P}`,
  `@K` and `@k`; `$(< file)`; `n>&m-` and `<>`; `[[ -o name ]]`, `[[ -R name ]]` and
  `test -v`; `$SRANDOM`, `$BASH_COMMAND`, `$BASH_SUBSHELL`, `SHELLOPTS` and `BASHOPTS`; a
  new shell's `SHLVL`, `OPTIND`, `PS4` and `HOSTNAME`; `trap ... DEBUG`; `time` and
  `timeout`, busybox's commands; a function's name nearly any word, and its body any
  compound command; sparse indexed arrays, and associative arrays in bash's hash order;
  `globstar`, `GLOBIGNORE`, and extended patterns that expand what is inside their groups.
- **Options.** `shopt` knows every name bash 5.3 has, and does `lastpipe`,
  `expand_aliases`, `xpg_echo`, `localvar_inherit`, `shift_verbose`, `failglob` and
  `inherit_errexit`; `set` takes bash's option names, `+H` and `posix` among them;
  `set -o vi` is busybox's vi editing mode.
- **Builtins.** `.` searches PATH and takes arguments; `cd` and `pwd` take `-L` and `-P`;
  `command -V` and `-p`; `hash`; `builtin`; `export -n`; `readonly -p` and `-a`; `kill -s`
  and busybox-w32's whole signal table, ILL, FPE, SEGV, PIPE and ABRT added, for `trap`
  too; `umask -S`; `read -t 0`; `wait -p`; `trap -P`; `getopts` as busybox's; `pushd`,
  `popd` and `dirs` as bash's. `bind` is a builtin this shell does not implement, 126,
  with the reason.
- **The interactive shell.** History expansion is bash's, with word designators and
  modifiers; `history` takes bash's options, and `fc` lists, edits and runs again;
  `HISTCONTROL`, `HISTSIZE` and `HISTFILE`. Prompts know bash's escapes, and
  `PROMPT_COMMAND` and `PS0` run. `--rcfile`, `--norc`, `--noprofile` and `--login`. The
  line editor binds `^B ^F ^P ^N` as busybox's does, and readline's `^T`, `M-.`,
  `M-u M-l M-c`, its kill ring with `M-y`, undo with `C-_`, `C-x C-u` and `M-r`, and
  `C-x C-e`.
- **Programmable completion.** `complete`, `compopt` and `compgen`, bash's, and Tab asks a
  command's specification first, as readline does, `COMP_WORDS` and the rest set as bash
  sets them. The bash-completion helpers that generated scripts call are builtins, so
  `source <(gh completion -s bash)` completes gh, and git's own script completes git.
- **Applets.** `bzip2` compresses, and `tar -cj` writes bzip2; `unxz`, `xzcat`, `unlzma`
  and `lzcat`, and `xz` and `lzma` with `-d`, read, as busybox's do, `zcat` reads xz, and
  `tar -J` and `--lzma` read and write; `bc -l` loads upstream's maths library, the one
  busybox's bc ports, and answers as busybox's does to the last digit; `uptime`; `flock`
  and `watch`, builtins since they run a command, a flock here and one of busybox-w32's
  locking different bytes, so that neither keeps the other out; `find` with `-exec`,
  `-ok`, `-delete`, `-prune`, `-regex`, `-quit`, `-depth`, the `-mmin` family, `-perm`,
  `-inum`, `-samefile`, `-links`, `-executable`, `-xdev`, `-H`, `-L` and `-wholename`;
  `install`, `sync`, `fsync`, `shred`, `ttysize`, `reset`, `egrep`, `fgrep` and
  `pipe_progress`; and `lsattr` and `chattr` for Windows' file attributes, as busybox-w32
  has them.
- **busybox's options**, applet by applet, measured against busybox-w32: sort, seq, cat,
  uniq, env, du, nl, cut, split, od, hexdump and hd, xargs (its quoting, `-I`, real `-P`),
  tail `-f -F`, stat, xxd, cmp, expand and unexpand, ls, cp, rm, mv, ln, chmod's symbolic
  modes, date, touch, readlink `-f`, tee, diff (`-w -b -B -L -T -t`, and directories),
  free, fold, paste, join, the md5sum family's `-c`, patch, uuencode `-m`, dd, grep
  `-m -o -w -s`, sed's `addr,+N`, GNU's `0,/re/` and `-b`, awk `-e -E -W` and `ENVIRON`,
  gzip and gunzip keeping a FILE's mode and time, unzip, iconv's code pages, cpio, df,
  pgrep and pkill `-v -e -P`, and `id -r`. An option may follow the operands, and a long
  option is taken by any prefix that names it alone, as getopt reads them.
- **CI.** The fuzz targets, fifteen of them, run every Sunday for five minutes each.

### Fixed

- **Two hangs**: `0**72**7` in arithmetic, and nested extended patterns.
- **`rm -rf ""` emptied the working directory.** An empty operand names nothing now.
- **A pipeline stage, a subshell and the shell itself end as SIGPIPE ends them** when
  their reader has gone, instead of writing on for ever.
- **`2>&1` keeps an external command's stdout and stderr in order.**
- **A function may recurse 10000 deep**; it stopped at 128.
- **An unclosed `[` in a word is looked up, not matched**: `[` no longer read its
  directory.
- **Silent wrong answers in the language**, each measured against bash: `IFS` starting
  empty, `x="$@"`, `"${a[@]}"` of an empty array, an operator on one array element, a
  partly quoted pattern in `[[ ]]`, `$(< file)`, a failed `=~` leaving `BASH_REMATCH`, an
  array literal over several lines, and the arithmetic of bases up to 64 and shift counts.
- **Constructs that stopped a whole script at the parse**: `lib::fn()` and `my-fn()`, a
  heredoc's delimiter beside a `;`, `(( x << 2 ))`, `${x:(-1)}`, `${s:$i:2}`, a function
  body that is a bare `if`, and many smaller ones, each now read as busybox and bash read
  it.

### Tested

- About 600 tests more, 2976 in all; the bash corpus at 318 cases and the parser corpus at
  182; the Oils suite gating CI on Windows, and its numbers checked in.
- Applets' outputs compared with busybox-w32's, line for line, for each option above, and
  `bc -l` on 300 random calls.

## v1.3.0 - 2026-09-25

Bash compatibility, measured rather than assumed. Every change below was checked against
busybox-w32 v1.38.0 first and, where busybox has not got the construct, against GNU bash
5.3; the order of authority is the project's, so where busybox has it, busybox decides.
Two corpora hold the answers: a bash corpus of 113 cases, each naming the reference it
follows, and a parser corpus of 181 written from the POSIX grammar. Two bash gaps are left
(`trap DEBUG`, and `$BASHPID` inside a subshell) and one parser gap (`case a in esac)`).

### Changed

- **Background jobs are processes.** A `cmd &` starts this binary again, as
  `nemosh --job <handle>`, the way busybox-w32 starts `sh --fs`, and hands it the job's
  program and the shell's state over an inherited pipe. So `$!` is a real pid, and
  `wait $!`, `kill $!`, `tasklist` and `taskkill` all take it. A job's own
  `trap ... TERM` runs when it is sent TERM. `kill -9` ends the job's Job Object, and
  with it every program the job started. At a prompt a job is announced as `[1] 12345`,
  busybox's form, and `$BASHPID`, `jobs -l` and `jobs -p` are new with it.

  Three things a script can notice. A job outlives the shell that started it, as in both
  references, so a console window stays open until the jobs attached to it have ended;
  measured in conhost, busybox-w32 does the same. A script that Ctrl-C ends takes its jobs
  with it, though, as busybox-w32's does, where bash would leave them running; the shell
  names the jobs it ended and says which of the two answers that was. And a job
  costs about 4-6 ms and 11 MB where a goroutine cost microseconds
  (docs/design/startup-and-footprint.md). `NEMOSH_JOBS=goroutine` brings the goroutine
  back: `$!` is then `%1` again and a job is announced as `[1] started; kill %1 to stop
  it`, as in 1.2.

- **grep reads a pattern as a basic regular expression**, as POSIX, GNU and busybox do,
  and `-E` makes it extended. Every pattern used to be extended, so `grep 'a+b'` wanted
  one or more a's, `grep 'x|y'` was an alternation, and `grep '\(a\)b'` matched nothing.
  A script written against the old reading needs `-E`. GNU's `\w \W \s \S \b \B` and
  `\< \>` now work in grep and sed alike.

- **An error POSIX makes fatal ends a script.** Assigning to a readonly name, in any of
  the ways a name is written (`R=2`, `R=2 cmd`, `export`, `local`, `unset`, `for R in`,
  `$((R=5))`), and `export a/b` end a non-interactive shell with status 2, as in busybox;
  they answered 1 and carried on. At a prompt only the line is abandoned. `set -u; echo
  $nope` typed at a prompt used to close the terminal.

- **`exit` exits**, from a function, a sourced file and `eval`. `die() { echo "$1" >&2;
  exit 1; }` stopped at the function, and the script went on with status 0. `break` and
  `continue` in `eval` reach the loop, as they do in a function.

- **A job ended by `kill` says so.** Its status is 128 plus the signal, 137 for KILL and
  143 for TERM, and `jobs` names it `Killed` or `Terminated`; it said `Done(1)`. `wait`
  for a job or pid the shell does not know is 127, as POSIX says, where it was 2.
  `kill -0` asks about a job or a pid and changes nothing; it used to end what it asked
  about. `kill -STOP` and `-CONT` are refused by name: nothing here can suspend a
  process, and they used to end it.

- **`unset name` leaves a function of that name alone**, busybox's reading of POSIX, and
  `unset -f` removes the function. `unset -f f` looked for a variable called `-f`.

### Added

- **The shell's own command line**, in busybox's shape:
  `nemosh [-ils] [-|+aCeEfnux] [-|+o NAME]... [-c COMMAND [NAME [ARG]...] | SCRIPT
  [ARG]...]`. A script beginning `#!/bin/sh -e` is started as `nemosh -e script`, and that
  exited 2 before running a line. `-l` reads /etc/profile and ~/.profile first; `-n`
  checks the syntax and runs nothing; `-s` gives standard input its positional
  parameters. `$-` says how the shell was started: `c`, `s`, or `is` at a prompt.

- **Options.** `set -euo pipefail` in one cluster: an `o` among the letters takes the next
  argument, and the line was "illegal option -o", which under `-e` ended every strict
  script on its first line. `set -E` and `set -T`; `set -o ignoreeof`; `nohiddenglob` and
  `nohidsysglob`, which keep files with Windows' Hidden attribute, or Hidden and System,
  out of pathname expansion; `shopt -s nocasematch`; `local -`, which keeps a function's
  `set -e` inside it. `set -o vi` and `set -m` are refused by name rather than called
  illegal.

- **Traps.** `trap ... ERR` runs where `set -e` would act. `trap ... RETURN` runs as a
  function returns or a sourced file finishes. `trap ... HUP`, `QUIT` and `TERM` run when
  `kill` sends the signal to a job, and when it reaches a script from outside it: a real
  `kill` off Windows, and on Windows the console closing, a logoff or a shutdown, after
  which the script stops, runs its TERM trap and then its EXIT trap. `trap -p`, `trap -l`
  and `trap --`. A subshell's own EXIT trap runs as the subshell ends, and `$(trap)` sees
  the parent's traps, so `saved=$(trap); ...; eval "$saved"` works.

- **Variables.** `$LINENO`, with PS4 expanded, so `PS4='+$LINENO: '` traces line numbers;
  `$FUNCNAME` as a name and as a stack; `${BASH_SOURCE[0]}`, `$BASH_LINENO` and `caller`,
  so a script can find its own directory; `EPOCHSECONDS` and `EPOCHREALTIME`; `$_`;
  `$BASHPID`.

- **Declarations and arrays.** `x+=y` and `a+=(...)`; `declare -i -l -u -g -A`, with
  `declare -A m=([k]=v ...)` and `a=([2]=two [0]=zero)`; `declare -f`, which prints a
  function back in bash's shape, so `nemosh -c "$(declare -f f); f"` hands a function on;
  `declare -F`; `local` with `declare`'s options; `declare -p` printing only the elements
  an array has.

- **Expansions.** `${x@Q}` and the other transformations, `@E @U @u @L @A @a`;
  `${!prefix@}` as a list; `[[ -v name ]]`; an unquoted group in `[[ $x =~ ^([0-9]+):
  ]]`, with the quoted parts of a regex taken literally, as bash 3.2 and later take
  them; `$(( a[i] ))` and `(( count[$w]++ ))`; arithmetic that follows a name to the
  expression it holds, so `x=2+3; echo $((x*2))` is 10; and `$(( ))`, which is 0.

- **Redirections.** `>(cmd)`, a named pipe the command reads while you write, so `tee
  >(sha256sum > sum) > copy` and `exec > >(tee -a log) 2>&1` work. `{fd}>file`,
  `{fd}<file` and `exec {fd}>&-`; `>&$fd`, whose descriptor is a word; `>&file`; `|&`; a
  heredoc into a loop, `done <<EOF`; and a loop redirected and piped, `done < in | sort`.

- **`coproc`**, bash's: `coproc [NAME] command` runs the command with a pipe to its stdin
  and one from its stdout, on `${NAME[1]}` and `${NAME[0]}`.

- **Builtins.** `printf -v var`, `%(fmt)T`, a `*` width or precision, and `"'A"` as the
  code of A; a non-number is written as 0 and printf carries on, as POSIX says. `type -t
  -p -P -a`, in the order a command is actually looked up. `getopts` reads grouped
  letters, attached arguments and `--`, and has its silent mode. `wait` takes several
  operands, a pid, and `-n`. `jobs` inside a pipeline or a command substitution lists the
  shell's jobs, so `kill $(jobs -p)` and `jobs | wc -l` see them.

- **Applets.** `tr` has the POSIX classes, so `tr '[:upper:]' '[:lower:]'` lowercases,
  and `tr -s SET1 SET2`. `date +%j`, `+%V` and the rest of glibc's strftime set, which
  `ts` and `printf %()T` share. `grep -G`.

- **The line editor.** Page Up and Page Down walk the history that starts with what is
  left of the cursor, zsh's history-beginning-search, and a pasted entry's indentation
  does not hide it. The highlighter reads a line with the shell's own lexer, so reserved
  words, the name of a function being defined, and case patterns each have a colour of
  their own, where they were drawn as commands that do not exist.

- **The parser.** A compound command as a condition, `if case ... esac; then`, and a
  condition of several commands: `if cd "$d"; [ -f x ]; then` decided on the `cd` alone.
  A compound after `&&`, `||`, `&` or `|`, as `[ -n "$x" ] && case ...`. A brace group
  after a reserved word, as `then { ...; }` and `! { ...; }`.

### Fixed

- `export PATH=$PATH:/x` cut PATH at its first space, which on Windows is nearly always
  there (`C:/Program Files`), and exported the rest as a name. The same was true of
  `readonly`, `local`, `declare` and `typeset`.
- `${x:-"a b"}` kept its quotes, and `${path%"/"}` and `${s#"a"}` removed nothing.
- `case $x in` split `$x` and matched its first field, and a quoted `*` in a pattern
  still matched anything.
- Unquoted `$@` kept each parameter whole, `"$*"` joined with a space whatever IFS said,
  and `"${@}"` was one word.
- `[[:upper:]]` and the other bracket classes matched nothing: in `case`, in `[[ ]]`, in
  `${x//...}` and in globs.
- `$((RANDOM % 6))` was always 0.
- `if true; false; then` took the yes branch, and a bare `until` read as `while`.
- `(set -e; false; echo x) || y` stopped at false.
- `IFS=, parts=($line)` stored the text `parts=(...)`.
- `declare -A m=([a]=1 [b]=2)` made an empty map, `a=([2]=two)` stored the text, and an
  element removed with `unset 'a[1]'` came back.
- `local a` over a global array wrote into the caller's array, `local -r` protected
  nothing, and a local over an exported name did not reach children.
- `export X` with X unset gave children an empty X, and `unset` kept a name's attributes.
- `$((1+1))` in a heredoc ran a command called `1+1`, and backquotes in one were left as
  they were.
- `[ "(" = "(" ]`, `[ ! = ! ]` and `test !` were syntax errors.
- `getopts` stopped at `-ab` and at an unknown option.
- `$_` was a path for the whole session.
- `set -x` did not trace an assignment-only command.
- A profile's EXIT trap ran as the profile finished, and `exit` in a profile ended only
  the profile.
- `for i in 1 2; do echo a& done`, `if ...; then x & fi`, `a) x & ;;` and `{ x & }` were
  syntax errors.
- `a() { case a in a) echo bingo;; *) echo hmm;; esac; }` did not parse, and neither did
  a function with a `case` and then an `||` in its body.
- `pwd` into a closed pipe reported "pipeline downstream closed".
- Up and then Down at the prompt lost the line that was being typed.
- A read that Ctrl-C ended could touch its file after the read had returned. The race
  detector found it on a Windows runner.
- The release job reported success when the Scoop bucket had packaged nothing: GitHub's
  release list lagged behind its uploads. It now waits for the list to show the asset
  before telling the bucket.

### Tested

- The bash corpus and the parser corpus above, with a gap file beside each. A gap that
  closes fails its test until the case moves across, so the corpus records what is fixed
  rather than what was hoped.
- CI runs the runtime suite under both job launchers on all three runners. The job
  processes in the tests are a nemosh the test binary builds once, and on Windows the test
  binary's Job Object ends them if it dies: one left by a hung run was found still running
  six hours later.

## v1.2.0 - 2026-09-16

### Added

- **Backgrounding says which job it became, and how to stop it.** `sleep 20 &` answers
  `[1] started; kill %1 to stop it`. busybox writes a job number and a process id; the
  number here is real and the pid is not, since a background job is a goroutine -- which is
  why `$!` already answers `%1`. So the line carries the handle that works, and the sentence
  is there because `%1` is not what somebody who has only killed a pid would guess.

- **A job that has ended says so at the next prompt.** bash and busybox both report this
  without `set -b`, which asks for something else -- the report *immediately*, mid-command,
  and that is still refused. Reporting is what consumes the job, whether the notice or
  `jobs` does it first.

- **`less`.** With nowhere to page to it is `cat`, which is what every less does and what
  makes it safe to write in a script: `less f | head -3` copies its input through, and a
  program downstream never has to know a pager was in the way. On a terminal it pages, with
  the keys people already have in their fingers, and a search that does not wrap -- because
  wrapping would make "no more" indistinguishable from "none at all".

- **`ed`.** The editor a *script* can drive: `nano` and `micro` need a terminal, and a shell
  script that has to change three lines of a file had nothing else to reach for on Windows.
  Addresses including `/re/`, `?re?` and marks; `a i c d p n l = s g v m t j k r w e f q Q
  h H`; and `s` sharing sed's regular expressions so the two commands cannot drift apart.

  This is the **one applet that follows GNU rather than busybox-w32**, and deliberately:
  busybox's ed answers `unimplemented command` to `n`, cannot search backwards, and does not
  wrap a forward search, so matching it would have pinned down a subset rather than the
  language.

- **`bc`.** The POSIX calculator language: variables, arrays, `if`, `while`, `for`, `define`
  with autos and recursion, `print`, and exact arithmetic at whatever precision `scale` asks
  for. `-l`, the maths library, is refused rather than approximated -- those functions are
  series expansions, and a wrong one is wrong in the last digits of an answer that still
  looks right, which is the worst shape an error can take in a calculator.

  One divergence from busybox, following POSIX and GNU: **`^` binds tighter than unary
  minus**, so `-2^2` is -4 here and 4 there.

- **`dc`, and the arbitrary-precision decimal arithmetic under it.** A number is an integer
  and a scale rather than a float, so `0.1` is exactly a tenth and `scale=30; 1/3` really has
  thirty digits. The scale of every result is specified rather than incidental -- with
  `scale=0`, `1.5 + 1.5` is `3.0` while `1.50 - 1.5` is `0`, and `2.5 * 2.5` is `6.2` -- and
  all of those were measured against busybox-w32 before being written.

- **`df`, `dd` and `stty`.** Windows has no command that answers `df` in a form a script can
  read -- `wmic` is gone, `fsutil` prints prose and PowerShell returns an object -- so this
  is one of the applets that is not a convenience but the only way. `dd` refuses an operand
  it does not know, where busybox prints its usage and exits 0, so a typo like `cnt=3`
  copies the whole file and reports success there. `stty` is scoped to the two questions a
  Windows console can answer, the window size and whether it echoes, and refuses the rest by
  name rather than accepting settings it cannot make.

- **`cal`, `getopt` and `ipcalc`.** `cal` matches busybox-w32 byte for byte across seventy
  calendars, including the eleven days September 1752 does not have and the line of spaces
  a month ending in a blank week prints. `getopt` canonicalises a command line for
  `eval set -- "$(getopt ...)"`, quoting the output as the enhanced version does and *not*
  quoting it in the old form, as the original did. `ipcalc` answers the network numbers as
  `KEY=value` lines in a fixed order.

- **Twelve more applets**: `nproc`, `arch`, `logname`, `groups`, `uuidgen`, `usleep`,
  `truncate`, `link`, `unlink`, `ts`, `pidof` and `killall`. All twelve are in busybox-w32
  and none is on a clean Windows machine.

  Three of them carry a decision rather than a fact. `logname` is deliberately **not**
  `whoami`: under elevation the second answers `root` and the first still answers the
  account, which is the only reason to have both. `pidof` and `killall` match a name
  **whole**, where `pgrep` and `pkill` match a pattern -- `killall sh` must not take `bash`
  with it. And `truncate` accepts GNU's relative `+N`/`-N` sizes, which busybox refuses,
  while refusing `<`, `>`, `/` and `%` by name rather than approximating them.

- **`awk`.** The POSIX language: patterns and actions, `BEGIN`/`END`, ranges, fields and
  the built-in variables, arrays, every control statement, user-defined functions with
  arrays by reference, the string and numeric built-ins, `printf`/`sprintf`, all six
  `getline` forms, output redirection, and `-F`/`-v`/`-f` with operands mixing file names
  and `VAR=VALUE`. It was the last name a script actually reaches for.

  Built in ten staged commits with the **value model first**, because awk's string /
  number / strnum rule decides every comparison in every program and cannot be retrofitted.
  Every rule was measured against gawk 5.4.1 and busybox-w32 1.38.0 *before* it was
  written, which is what turned up the dozen places the two disagree -- all listed in
  `docs/support-matrix.md` with the one that was followed and why.

  Two deliberate differences from both: **text is counted in runes**, so
  `length("héllo")` is 5, matching what `wc -m`, `rev` and `fold` already do here; and a
  command in `system()`, `print | cmd` or `cmd | getline` is **an applet of this shell**,
  because `internal/applets` never spawns an OS process. Shell syntax inside such a
  command is refused rather than guessed at.

- **A kill ring in the line editor.** `^K` kills to the end of the line and `^Y` puts back
  what `^K`, `^U` or `^W` took. `^U` and `^W` destroyed what they removed until now.
  `^U` also changed meaning to readline's unix-line-discard -- backwards to the start of
  the line, keeping the tail -- where it used to clear the whole line.

- **`shopt -s autocd`**: a bare directory name changes directory. Off by default, and
  checked only *after* command lookup has failed, so a command always wins -- many trees
  hold a directory called `test`, and `test` is a command. A path-shaped word like `./a`
  is not autocd'd, which is bash's behaviour.
- **`CDPATH`**: `cd src` finds it under any entry, with the current directory tried last.
  A path that says where it means -- absolute, `./`, `../`, `~` -- is never searched for.
  The search is silent until the last candidate, so a miss still reports once.
- **`~+`, `~-`, `~N`, `~+N` and `~-N`**, naming a place on the directory stack. `~-` is
  where `cd -` would go; the numbered forms count from the current directory and from the
  far end, the same two directions `pushd +N` and `pushd -N` use.

- **History expansion**: `!!`, `!n`, `!-n`, `!string`, `!?text?`, `!$`, `!^`, `!*` and
  `^old^new`, on interactive input. Single quotes protect and double quotes do not; a
  backslash escapes; a `!` before a blank, an `=` or the end of a line is text, so
  `[ x != y ]` still works. The expanded line is echoed on stderr before it runs, and a
  reference that resolves to nothing is an error rather than being left as typed.

- **`select name in words; do ... done`** -- the last of the 46 shell constructs the v1.1
  probe measured and the only one it found missing. The menu and the prompt go to stderr,
  so `x=$(select ...)` captures the answer rather than the list; `REPLY` is the line as
  typed; an out-of-range number gives an empty name rather than an error; and a blank line
  reprints the menu. `PS3` is the prompt. Thirteen forms measured against bash.
- **`mapfile` and `readarray`**, one builtin under two names, with `-t -n -s -O -d -u`.
  The delimiter is kept unless `-t` asks otherwise, which is bash's rule and the opposite
  of what people expect. `-C` callbacks are refused rather than ignored.

- **`pushd`, `popd` and `dirs`.** Position zero of the stack is not stored -- it is read
  from the shell -- so it cannot go stale when `cd` moves. `+N` counts from the current
  directory and `-N` from the far end; `pushd +N` rotates where `popd +N` removes, which
  is bash's behaviour and easy to conflate. A subshell gets a copy, so `(pushd /tmp)`
  leaves the parent where it was.

- **Replace in the editor**, which `-H` had listed as absent since the day it was
  written. nano binds `^\` and micro `^R`; every match is confirmed with `y`/`n`/`a`/`q`.
  The scan starts at the top of the buffer rather than at the cursor, so there is no wrap
  condition and no way to wonder whether some were missed.

- **`sed` reads UTF-16, and `sed -i` writes it back.** It used to match nothing on a file
  Notepad or PowerShell 5.1 wrote and copy it through, because a regular expression
  cannot match across UTF-16 code units. `-i` re-encodes to the file's own byte-order
  mark, so the file stays the kind of file it was. Output to a pipe is UTF-8, the rule
  `grep` already follows.
- **`wc -m` counts characters on UTF-16** and `-c` still counts bytes, from one pass: the
  raw bytes are tallied on the way into the decoder. Both were recorded as outstanding on
  the grounds that one pass could not honestly do both.

- **Line numbers in the editor**, in a gutter down the left, on by default for both
  names. `-l` is accepted for nano's sake and does nothing, since they are already on.
  The width follows the line count, and a terminal too narrow to afford it loses the
  gutter rather than the text.

### Fixed

- **`$?` at the prompt was always zero.** `false` and then `echo $?` answered 0, while the
  prompt beside it showed 1 -- the two were reading different things, and only the invisible
  one was right. The edited session took the Runtime by value, and the three methods that
  carry interactive state between commands have pointer receivers, so each command ran
  correctly on a copy and the status was then thrown away. Everything else survived the copy
  because it already sat behind a pointer.

- **Ctrl-C could not stop a command that was producing output.** `seq 1 100000000` ran to
  the end: measured, the interrupt arrived after 200ms and it wrote 888 MB anyway, then
  reported 130 as though it had been stopped. Almost every applet is built from a signature
  with no context in it, so nothing inside a loop can see a cancellation; the write fails
  now instead, which reaches all of them at the one place applets are dispatched.

- **Ctrl-C inside an applet needed a second keypress, and that keypress was spent noticing
  it.** A cancellation was only ever checked between reads, which is no use to an applet
  already sitting in one -- a console read returns when a line arrives and not before. A
  blocked read is now ended outright. Measured against a real console first: a read blocked
  for 500ms, CancelIoEx answered success, and it returned at once.

- **Being interrupted was reported as a failure.** `bc` answered `bc: context canceled`, a
  Go error string shown to somebody who had pressed a key. Every other applet returns what
  its scanner failed with and the shell answers 130 in silence; `bc` alone printed it.

- **Ending a command with Ctrl-Z left two spare prompts.** Two causes, one keypress. A
  console read in cooked mode hands back a line ending CRLF, and the editor decoded that as
  two Enters -- so one leftover terminator became two empty commands. It is one Enter now,
  and the terminator a command left behind is not read as a keypress at all: in raw mode,
  which is the only mode the editor reads in, a pressed Enter is a bare carriage return.

- **`dc` read one more line after `q`.** `for reader.Scan() && !m.quit` evaluates the left
  operand first, so `q` set the flag and the loop then waited for a keystroke before looking
  at it. Off a pipe that costs nothing, which is why no test saw it.

- **`jobs` answered `Done` every time it was asked, forever.** POSIX removes a job from the
  list once the shell has reported its status, and both references do; here the table only
  ever grew, and `jobs` stopped describing what was actually running.

- **`less` never paged inside the shell, and `[ -t 1 ]` never said yes on a terminal.** Both
  asked `stream.(*os.File)`, which is false for every applet the shell runs -- so `less`
  behaved as though its output were a pipe and printed the file, and the one question
  `[ -t 1 ]` exists to answer was answered wrongly. A guard now fails the build on a sixth
  occurrence of this, since it has cost five.

- **A search in `less` moved to the match and drew it like any other line.** Every part of a
  line that matches is highlighted now, as busybox does, and an empty match highlights
  nothing -- `x*` would otherwise paint the whole file.

- **Leaving a reverse search redrew the first line of a two-line prompt.** Only the row the
  search took over was ever given back, so a two-line prompt -- which is the default --
  looked like a three-line one.

- **The prompt held the terminal raw, so no command could read from it.** Typing `bc` in
  Windows Terminal gave a shell that took no input and ignored Ctrl-C. The line editor
  entered raw mode when the session started and left it when the session ended, so every
  command ran inside it -- with no echo, with Enter arriving as a carriage return that no
  line discipline turned into a line, and with the console bit that makes Ctrl-C an interrupt
  switched off. It applied to `dc`, `ed`, a bare `cat`, and the shell's own `read` and
  `select` too. The borrow is now one line read long.

- **Interactive `bc` hung, and the cause reached further than bc.** It read standard input to
  the end before evaluating anything, and a terminal has no end. `dc` had the same line, and
  `less` would have read the user's typing as a file's contents.

  Auditing every applet that reads a stream found two more faults. The byte-order-mark check
  peeked three bytes, so typing one character and pressing Enter -- two bytes -- blocked
  `bc`, `dc` and `ed` after they had already read the line. And `awk` and `tr` held their
  output until the input ended, where busybox's stream; `tail -f log | awk '...'` showed
  nothing until the log stopped growing.

  The rest of the "waits for the end" list matches busybox exactly and is left alone, which
  `docs/support-matrix.md` now sets out applet by applet.

- **`!!` was drawn red while being typed.** The colour asks "can this shell run a
  command of this name?", and `!!` is not a command name -- so nothing was ever going
  to find it. It is now judged by what it will *become*: `!!` after `ls -la` is green,
  `!!` after a command that cannot run stays red, and `!zzz` with no such event stays
  red because the line will be refused rather than run.

- **`HISTCONTROL` and `HISTSIZE` were settable and did nothing.** A leading space is how
  people keep a token out of their history, so ` export TOKEN=...` silently wrote the
  secret to disk. `ignorespace`, `ignoredups`, `ignoreboth` and `erasedups` all work now,
  and `HISTSIZE` caps the list keeping the newest.
- **`$!` was empty.** It names the job now -- `%1`, a job specification rather than a
  process id, because a background job here is a goroutine and has no pid. `kill $!` and
  `wait $!` both work, which is what it is used for.

- **The non-terminal interactive loop recorded no history**, so `history` was empty
  whenever the shell was interactive without a terminal -- and, once expansion landed,
  `!!` answered "event not found" on the very path that had just run a command. Both
  loops record the whole command now.

- **`unset a[i]` returned success and did nothing.** The subscript was never parsed, so
  `a[1]` was looked up as a variable name, not found, and deleting a name that is not
  there succeeds -- a script that removed an element and carried on was silently wrong.
  It now removes the element and leaves a gap, as bash does, so later indices do not
  shift. Associative keys too, and `unset a` now clears the array rather than only the
  scalar of that name.
- **`cd` printed a raw Win32 error.** `cd nosuchdir` said
  `GetFileAttributesEx C:\...: The system cannot find the file specified.` where every
  shell says `No such file or directory`. The mapping the applets already use is shared
  with the shell now, so a missing directory reads the same whichever found it.

- **The editor deleted the file's final newline when a line was cut or pasted.** The
  buffer was rebuilt with `strings.Join`, which is not the inverse of the line split --
  the split drops the empty element a terminating newline produces and Join cannot know
  it was there. Saving then wrote the file back one byte short. Found while writing
  replace, which rebuilds the buffer the same way.

- **Go to line did nothing on Windows.** tcell has two input paths that disagree about
  which `Key` constant a control chord is. A terminal posts `KeyCtrlSpace+Key(r)` for a
  control byte and `KeyCtrlSpace` is 64, so `0x1F` is `Key(95)` -- `KeyCtrlUnderscore`,
  as named, and `^_` had always worked there. The Windows console has no VT screen, and
  for a control character whose modifier mask is exactly Ctrl it adds `0x60` back: `^_`
  as Ctrl+`-` becomes `0x7F` and reads as Backspace, so it *deleted*. As Ctrl+Shift+`-`
  it keeps its mask and arrives as `Key(31)` -- `KeyUS`. Both constants are bound now.
- Letters were never affected: `key.go:276` maps `a`-`z`+`ModCtrl` onto `KeyCtrlA+n`,
  the same numbering, so both paths agree for them. Punctuation has no such mapping.
- `Esc` then `G` is bound too, as nano's `M-G` -- the only meta spelling that survives
  the Windows path, which reports Alt with a letter as no character and drops the event.
- The test that missed this pressed the same constant the code bound, so it proved only
  that the code equalled itself. It presses both now, and the constant arithmetic is
  pinned so a tcell upgrade that renumbers either block fails loudly.
- **`^G` help drew a second row of key names under the legend.** It wrote to the message
  row, which sits directly above the legend that already lists every key. It is a panel
  now, listing what is *absent* and why -- the same fix `c5a1a44` made for `top`.

- **`^_` did nothing in nano on Windows.** Reported from a real keyboard. tcell has no
  VT screen on Windows, so input goes through the console API, which for a Ctrl'd
  control character adds `0x60` back and posts a rune with `ModCtrl` instead of a `Key`
  constant -- and for the chords that should produce `0x1F`, nothing arrived at all.
  Every binding now accepts both spellings, go-to-line answers to `^/`, `^_`, `^-` and
  `M-G`, and the footer leads with `^/`.

- **The editor did not use the width of a wide terminal.** Reported from a real terminal;
  invisible to every test, because the harness runs at 80 columns -- the one width the
  old code was right at. The key legend was laid out by `footer(80)` exactly once when
  the view was built, so a 200-column terminal kept the 80-column answer and drew key
  hints in the leftmost 61 columns. It is now laid out for the width it is given and
  asked again when that changes, and because the row count depends on the width the
  layout resizes the legend's row to match.
- The title is a **bar** now, with a background across the full width, as nano's is.
  Before it was twenty characters on an otherwise blank line, which is most of why the
  window looked like it was not using the terminal.
- **Seven applets added a newline to a file whose last line had none, and six turned
  every CRLF into LF.** `sed`, `rev`, `head`, `tail`, `fold`, `expand` and `unexpand`;
  measured against busybox *and* GNU, which agree with each other and disagreed with
  all seven. One shared root cause: `eachLine` used `bufio.ScanLines`, which throws
  the terminator away, so it reported a newline for every line -- including a final
  line that had none and a CRLF line whose carriage return it had already eaten.
- **`sed -i` wrote that byte to the file**, so a script that matched nothing still
  grew the file. A UTF-16 file was the sharpest case, its last byte being a NUL: 24
  bytes came back as 25, which also made the support matrix's claim that `sed` "copies
  the file through" false. It now round-trips byte-identical.
- The CRLF half mattered more on this platform: a Windows-first shell that rewrites
  every CRLF file it filters is corrupting the common case, since
  `head build.log > first.txt` should not change the copy's line endings.
- `sed` needed its own writer, because its rule is not per line -- the newline is
  omitted only on the very last thing written, and `p` writes the final line twice
  with the *first* of the two terminated. The terminator therefore travels with the
  write and records which input line produced it.
- One divergence resolved in busybox's favour: on a file with no final newline,
  `sed 2q` preserves it under busybox and adds a byte under GNU.
- **`printf %q` left a comma unescaped.** bash escapes it -- measured, `printf %q a,b`
  answers `a\,b` -- because a comma separates a brace expansion. The comma was in the
  *safe* set, which is the one direction that function is not allowed to err in: its
  own comment says the cost of an unnecessary backslash is nothing and the cost of a
  missing one is a command that does something else.
- **The editor counted one line too many for every file ending in a newline.**
  `strings.Split("a
b
", "
")` answers three elements because a newline is a
  terminator and Split treats it as a separator, so `^_ 9999` on a sixty-line file
  answered "Line 61". One helper now does the splitting for both the line count and
  the cursor arithmetic, and an empty buffer stays one empty line rather than zero --
  zero would divide by the line count in the search wrap.
- **`tar -c -x` chose an operation rather than refusing.** A switch on the three
  letters in order meant `tar -c -x -f a.tar src` created the archive and ignored the
  `-x`; somebody who typed both meant one of them and got the other half the time.
  Exactly one of `-c -t -x` is now required, as busybox requires and as this build's
  own `cpio` already did for `-t -i -o`.
- **`tar -C DIR` created the directory instead of requiring it.** It appeared as a
  side effect of writing the first entry, so `tar -xf a.tar -C /tpm` made a new
  directory rather than reporting the misspelling. `-C` means "change to this
  directory", and changing to one that is not there is an error.
- **An embedded NUL defeated two containment checks**, found by fuzzing:
  `NUL<NUL>.txt` passed the reserved-device test because the stem compared is
  `nul<NUL>`, and `evil.<NUL>` passed the trailing-dot test because the name ends in a
  NUL rather than a dot. `safeArchivePath` now refuses a NUL anywhere in a name, and
  the cpio reader cuts at the first one as a C reader would.
- **`writeArHeader` wrote headers `readArHeader` rejects** (a negative size), and a
  cpio name field that is entirely NUL produced a nameless entry. Both from the same
  fuzz run.
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

### Added, earlier in the same cycle

- **Syntax highlighting in the editor**, for eleven languages: Go, C, C++, Python,
  shell, Haskell, Prolog, JSON, YAML, TOML and Markdown. `nano -H` and `micro -H` list
  them, generated from the tables so the list cannot claim a language with no rules.
- **Written here rather than imported, and the alternative was measured first.** micro's
  `pkg/highlight` is importable -- three files, MIT, no tcell -- and costs 283 KiB plus a
  44 KiB YAML corpus in this binary, which the size ceiling has room for. What settled it
  was the init ceiling: `gopkg.in/yaml.v2` does 262 allocations in its own package init,
  which would leave 205 of the 2750 budget for everything else. Generating Go tables from
  micro's YAML at build time is impossible -- `Def.rules` and its rule types are
  unexported, so a `Def` can only come from `ParseDef`, which needs the YAML.
- Measured after: **114 KiB and no init allocations at all**, because the tables compile
  behind a `sync.Once` on first use. A third of the size for none of the startup cost.
- **Soft wrap is now off**, which is a requirement and not a preference: highlighting
  re-colours cells after tview draws them, and that needs one screen row to be one buffer
  line. With wrapping on, tview's line-start table is unexported and there is no mapping.
  Long lines scroll sideways, as they do in micro.
- The cell advance uses `uniseg` -- already linked, so no new bytes -- with tview's own
  flat `TabSize`, because anything else slips the colours a column on every indented line
  and on every line after a CJK character. A cell the widget styled differently is left
  alone, which is how the selection and cursor stay visible.

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

### Fixed, earlier in the same cycle

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
