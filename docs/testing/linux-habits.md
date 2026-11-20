# What a Debian or Ubuntu user types, and what nemosh does with it

`ps aux` printed nemosh's own columns until 2026-11-20, and `ps -o pid` was `unknown option`.
Nothing in the test suite could see that: every test asked what the applet was written to do.
This page asks the other question. It is an audit of everyday invocations a Debian or Ubuntu
administrator types, taken on 2026-11-20 from the option parsers themselves, and each row says
what happens here.

The reference for these rows is Debian's tools -- coreutils, procps-ng, util-linux, iproute2,
iputils, findutils, grep, gawk and the like -- where busybox-w32 is the reference elsewhere.
Where Windows cannot answer, the row says so, and the applet refuses with the reason rather
than guess (AGENTS.md).

**Status:** `works` as on Debian; `partial` runs but differs, said how; `refused` says
`unknown option` or the like; `missing` is no such command, or a Windows program of that name
with other syntax. A commit that changes a row changes it here.

## Counts

Rows, not invocations: a row holds the invocations that share a status.

| status  | rows |
| ---     | ---  |
| works   | 92   |
| partial | 13   |
| refused | 6    |
| missing | 13   |

## Existing commands

| invocation | status | note |
| --- | --- | --- |
| `ps aux`, `ps -ef`, `ps -eo pid,etime,args --sort=-rss`, `ps axf`, `ps -p $$ -o comm=` | works | procps-ng's; `?` for an owner Windows will not show (5d8ebc49) |
| `ps -l`, `ps -elf` | refused | with the reason: F, PRI, NI, ADDR, WCHAN have nothing behind them |
| `grep -rn`, `-C/-A/-B`, `-e a -e b`, `-f FILE`, `-L`, `-c`, `--exclude-dir` | works | |
| `grep --line-buffered` | works | each line is written as it is found |
| `grep -P` | works | RE2's reading; lookaround and backreferences refused by name |
| `grep -z`, `grep -b` | works | GNU's --null-data and --byte-offset |
| `zgrep`, `zegrep`, `zfgrep` | works | gzip, bzip2 and xz by their magic, plain files as they are |
| `find -name -mtime +7 -delete`, `-type f -size +100M`, `-maxdepth -iname`, `-newer`, `-perm`, `-print0`, `-empty`, `-mmin`, `-regex` | works | |
| `find -user NAME`, `-group`, `-uid`, `-gid` | works | NAME as `ls -l` prints the owner |
| `find -printf '%T@ %p\n'`, `'%s %P\n'` and GNU's other directives | works | `%F %S %Z` refused |
| `pgrep -f`, `pkill -f` | works | a command line Windows will not show is matched by the name |
| `pgrep -a`, `-u`, `-n`, `-o`, `-c`, `-d`; `pkill -u -n -o` | works | |
| `pkill -9 NAME`, `killall -9 NAME`, `killall -q` | works | `killall -q -9`: the signal must come first |
| `kill -9 PID`, `kill -l`, `kill -s TERM` | works | |
| `du -sh *`, `du -h -d 1`, `du -sch` | works | |
| `du --max-depth=N`, `--exclude`, `--apparent-size`, `-X FILE` | works | GNU's long options |
| `df -h`, `df -hT`, `df -T` | works | |
| `df -x TYPE`, `df --output=...` | works | GNU's columns |
| `df -i` | partial | GNU's columns, `-` for each count: Windows keeps none to read |
| `ls -lah`, `-ltr`, `-lS`, `--color=auto`, `-d */`, `-i`, `-n` | works | |
| `ls --time-style=long-iso` | works | GNU's styles and +FORMAT |
| `tail -f`, `-F`, `-n +2`, `-f a b`; `head -n -5`, `head -c 100` | works | |
| `tail -f --pid=N`; `--lines=N`, `--follow=name` and GNU's other long options | works | stops once N has ended, as GNU's does |
| `free -h`, `free -m` | works | |
| `free -t`, `free -s N`, `free -w` | works | procps-ng's; buffers are zero, Windows keeping none apart |
| `uptime`, `uptime -s` | works | |
| `uptime -p` | works | procps-ng's pretty form |
| `top -b -n 1`, `top -d 1` | works | |
| `top -bn1` | works | letters bundle as getopt has them |
| `top -o %MEM` | works | an upper-case procps field is the sort; lower-case columns the layout |
| `top -p PID`, `top -u USER` | works | |
| `date +%F`, `-u`, `-R`, `-I`, `+%s`, `-d @N` | works | |
| `date -d yesterday`, `date -d '2 days ago'`, `touch -d '2 days ago'` | works | GNU's relative items, after busybox's formats |
| `watch -n 1`, `watch -t` | works | |
| `watch -d` | partial | accepted, highlights nothing |
| `cp -a`, `-r`, `-u`, `-l`; `rm -rf`; `ln -sfn` | works | |
| `mv -u` | works | |
| `rm -I` | works | |
| `cp --preserve=all`, `--no-preserve=mode`, `-a` keeping hard links | works | ownership as Windows keeps it; `context`, `xattr` by name refused |
| `tar xzvf`, `czf`, `tf`, `xf -C`, `--exclude`, `cJf` | works | |
| `tar -I zstd` | refused | no zstd |
| `sed -i`, `-i.bak`, `-n '10,20p'`, `-E`, `'/^#/d'` | works | |
| `sed -z` | works | GNU's --null-data |
| `awk -F: '{print $1}'`, `-v`, `printf`, `tolower`, `length(arr)` | works | |
| `awk gensub()` | works | busybox's awk has it (c23c5eb7) |
| `awk BEGINFILE {...}`, `ENDFILE {...}` | refused | by name, as the program is read (d3c84b48) |
| `awk strftime()`, `systime()`, `mktime()`, `and()`, `lshift()` | works | busybox's awk has them; gawk's UTC flag too |
| `awk asort()` | refused | `call to undefined function`; gawk's alone |
| `awk PROCINFO["sorted_in"]` | partial | an ordinary array; orders nothing |
| `awk -i inplace`, `awk --version`; `gawk` | refused | `gawk` missing |
| `sort -h`, `-k2,2n`, `-t: -k3 -n`, `-u`, `-V`, `-o` | works | |
| `sort -R` | works | equal keys kept together |
| `uniq -c`; `cut -d: -f1,3`, `-c1-10`; `tr -d`, `-s`; `wc -l`; `nl`; `xargs -I{} -P4 -n1 -0` | works | |
| `cut --complement` | works | |
| `diff -u`, `-r`, `-q`; `patch -p1` | works | |
| `diff -y`, `diff --color`, `diff --brief` and GNU's other long names | works | GNU's layout and colours, byte for byte |
| `less -N`, `less -S` | works | |
| `less -R` | works | colour drawn, as GNU's; busybox's strips it |
| `less +F`, `+G`, `+/pattern`, `+120`; `F` | works | a FILE followed as it grows |
| `less a b` | works | one at a time, `:n` `:p` `:x` |
| `chmod +x`, `-R 755`; `stat -c '%a %n'` | works | on Windows only write has an effect |
| `touch -d` | works | `date -d`'s formats and relative items |
| `dd ... status=progress` | partial | no progress shown |
| `history`, `fc -l`, `env`, `printenv`, `export -p`, `type`, `command -v` | works | |
| `which -a NAME` | works | every match, the one that runs first (bd3ce6a7) |
| `time`, `timeout 5`, `timeout -s KILL`, `sleep 1m`, `seq -w`, `yes`, `tee -a` | works | |
| `md5sum -c`, `sha256sum`, `base64 -d`, `xxd -r`, `od -c`, `strings -n 8` | works | |
| `wget -qO- URL` | works | |
| `wget -c` | partial | accepted, nothing resumed |
| `nc -zv HOST PORT` | works | netcat-openbsd's |
| `nc -l -p 8080` | partial | 127.0.0.1 only, one connection |
| `id -un`, `groups`, `whoami`, `uname -a`, `uname -r`, `nproc`, `arch` | works | |
| `rev`, `tac`, `shuf -n`, `split -l`, `comm -12`, `join`, `paste -sd+`, `fold -w`, `expand`, `iconv`, `truncate -s 0`, `mktemp -d`, `install -D`, `readlink -f`, `basename -s`, `dirname` | works | |
| `realpath --relative-to=DIR` | works | GNU's options |

## Names that are another program here, or nothing

A name in this table that Windows has runs Windows' program today, with Windows' syntax.

| invocation | status | note |
| --- | --- | --- |
| `ping -c 4 HOST`, `-i`, `-W`, `-q`, `-6` | works | iputils'; Windows' still runs as `ping.exe` |
| `tree -L 2`, `-a`, `-d`, `-I`, `-h` | works | tree(1)'s; Windows' still runs as `tree.com` |
| `hostname -I`, `hostname -f`, `-s`, `-d`, `-i`, `-A` | works | Debian's; Windows' still runs as `hostname.exe` |
| `netstat -tulpn` | missing | Windows' netstat prints its usage; Debian's answer is `ss` |
| `ss -tulpn`, `ss -tlnp`, `ss -ant` | works | Recv-Q and Send-Q `-`; `ss -s` refused |
| `ip a`, `ip r`, `ip -br a`, `ip link`, `ip -4 -o a` | works | to read with; a change refused with the PowerShell for it |
| `traceroute`, `dig`, `host` | missing | Windows has `tracert` and `nslookup` |
| `lsof -i :80` | missing | |
| `column -t`, `-s,`, `-x`, `-N` | works | util-linux's |
| `pstree`, `pstree -p`, `pstree PID` | works | psmisc's; threads folded |
| `file NAME`, `file -i`, `file -b` | works | libmagic's words; past the first header fields left out |
| `tput cols`, `tput setaf 1`, `tput sgr0` | works | xterm-256color's |
| `tty`, `tty -s` | partial | `-s` and `not a tty`; a terminal's name refused, /dev/tty not yet a device |
| `man ls`, `man -k`, `apropos`, `whatis`, `info` | works | this shell's help, in man-db's words |
| `zip -r a.zip dir`, `-x`, `-j`, `-q` | works | Info-ZIP's; percents this deflate's |
| `nohup CMD &` | missing | jobs outlive the shell, not the console |
| `nice`, `renice` | missing | top's F7 and F8 step a priority class |
| `w`, `who`, `last` | missing | |
| `lscpu`, `lsblk`, `mount`, `vmstat`, `iostat` | missing | `nproc`, `df`, `free`, `top` |
| `envsubst`, `numfmt`, `fmt`, `pr`, `csplit`, `whereis` | missing | |
| `vi`, `vim` | partial | `vi` is busybox-w32's, being built a command at a time: moving with counts, `dd dw cw yy p x J` and the rest of the operators, registers and marks, `u ^R .`, inserting, `:w`, `:q`, `ZZ`; no search or `:s` yet; `vim` is PATH's, Git's |
| `sudo` | missing | decided against: see the support matrix's Elevation |
| `systemctl`, `journalctl`, `apt`, `jq`, `rsync`, `tmux`, `screen` | missing | not planned |

## The shell itself

| habit | status | note |
| --- | --- | --- |
| `bash script.sh`, `bash -c`, `#!/bin/bash`, `#!/usr/bin/env bash` | works | this shell, as busybox-w32's; WSL's System32\bash.exe ran them in Linux before |
| `sh -c`, `#!/bin/sh` | works | |
| `!!`, `!$`, `^old^new`, Ctrl-R | works | |
| `ll`, `la`, `l`; `ls` and `grep` in colour | works | Ubuntu's aliases in a session; `NEMOSH_DEFAULTS=off` for none |
| `LS_COLORS`, `dircolors` | partial | ls reads LS_COLORS, and a session has dircolors' database; no `dircolors` command |
| `$BASH_VERSION`, `$BASH_VERSINFO` | works | bash 5.2's when run as bash; unset as sh or nemosh |
| `$COLUMNS`, `$LINES` at the prompt | works | before each prompt, as `checkwinsize` has them |
| `fg`, `bg`, Ctrl-Z | refused | will not be: nothing here can suspend a job |
| `~/.bashrc`, `~/.bash_aliases` | missing | `~/.nemoshrc`, by design; Ubuntu's `~/.bashrc` runs there, below |
| `/proc/cpuinfo`, `/etc/os-release`, `/dev/tty` | missing | `/dev/null`, `/dev/clipboard`, `/tmp` are here |

## Ubuntu's .bashrc, line by line

What an administrator gets by copying Ubuntu's `/etc/skel/.bashrc` into `~/.nemoshrc`, taken on
2026-11-21 from the bash package's own copy (`cmd/nemosh/testdata/README.md` says where), and
measured with the built binary in a scratch HOME, a console robot reading the prompt back.
`TestSession_runsUbuntusStockStartupFiles` runs it, Ubuntu's `/etc/bash.bashrc` and its
`~/.profile`. It runs to the end and says nothing.

| line | status | note |
| --- | --- | --- |
| `case $- in *i*) ;; *) return;; esac` | works | a script reads no rc file; a `return` at an rc file's top level leaves only the file, as in bash, where it ended the shell until 2026-11-21 |
| `HISTCONTROL=ignoreboth` | works | the history, the arrows and the file |
| `shopt -s histappend` | works | the file is appended to a command at a time either way, and a prompt says so |
| `HISTSIZE=1000`, `HISTFILESIZE=2000` | partial | the file keeps 2000; a session keeps at most 999, busybox's `MAX_HISTORY` |
| `shopt -s checkwinsize` | works | on already |
| `[ -x /usr/bin/lesspipe ] && eval "$(SHELL=/bin/sh lesspipe)"` | missing | the test is false, as in busybox-w32: `/usr/bin/X` is an applet only as a command name or after `#!`; there is no lesspipe, and less reads no `LESSOPEN` |
| `debian_chroot`, `/etc/debian_chroot` | works | no such file, so no chroot in the prompt, as on a machine that is not one |
| `case "$TERM" in xterm-color\|*-256color)`, the title in `PS1` | partial | a Windows console sets no `TERM`, so no colour and no title; with `TERM=xterm-256color` both are drawn, which Git's bash sets |
| `force_color_prompt=yes`, uncommented | partial | turns colour off: `[ -x /usr/bin/tput ]` is false, as above, though `tput setaf 1` works |
| `PS1='${debian_chroot:+($debian_chroot)}\u@\h:\w\$ '` | works | in place of nemosh's two-line prompt |
| `[ -x /usr/bin/dircolors ]`, `eval "$(dircolors -b)"`, the ls and grep aliases | works | the test is false and the block is skipped, but a session has those aliases and dircolors' `LS_COLORS` already |
| `alias ll='ls -alF'`, `la`, `l` | works | |
| `alias alert='notify-send ...'` | partial | defined; running `alert` is `notify-send: not found`, 127 |
| `[ -f ~/.bash_aliases ] && . ~/.bash_aliases` | works | when there is one |
| `if ! shopt -oq posix; then . /usr/share/bash-completion/bash_completion` | works | posix is off; neither file is there, so nothing is read, and completion is nemosh's own |
| Ubuntu's `/etc/bash.bashrc`: `[ -z "${PS1-}" ] && return` | works | an interactive shell has `PS1` before its startup files; until 2026-11-21 it had none and the session closed at this line |
| Ubuntu's `~/.profile` under `bash -l` | works | `$BASH_VERSION` is set, so it reads `~/.bashrc` |
