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
| works   | 56   |
| partial | 8    |
| refused | 12   |
| missing | 29   |

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
| `find -user NAME` | refused | |
| `find -printf` | refused | |
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
| `tail --pid=N` | refused | |
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
| `cp --preserve=all` | refused | |
| `tar xzvf`, `czf`, `tf`, `xf -C`, `--exclude`, `cJf` | works | |
| `tar -I zstd` | refused | no zstd |
| `sed -i`, `-i.bak`, `-n '10,20p'`, `-E`, `'/^#/d'` | works | |
| `sed -z` | works | GNU's --null-data |
| `awk -F: '{print $1}'`, `-v`, `printf`, `tolower`, `length(arr)` | works | |
| `awk gensub()` | works | busybox's awk has it (c23c5eb7) |
| `awk BEGINFILE {...}`, `ENDFILE {...}` | refused | by name, as the program is read (d3c84b48) |
| `awk strftime()`, `systime()`, `asort()` | refused | `call to undefined function` |
| `awk PROCINFO["sorted_in"]` | partial | an ordinary array; orders nothing |
| `awk -i inplace`, `awk --version`; `gawk` | refused | `gawk` missing |
| `sort -h`, `-k2,2n`, `-t: -k3 -n`, `-u`, `-V`, `-o` | works | |
| `sort -R` | works | equal keys kept together |
| `uniq -c`; `cut -d: -f1,3`, `-c1-10`; `tr -d`, `-s`; `wc -l`; `nl`; `xargs -I{} -P4 -n1 -0` | works | |
| `cut --complement` | works | |
| `diff -u`, `-r`, `-q`; `patch -p1` | works | |
| `diff -y`, `diff --color` | refused | |
| `less -N`, `less -S` | works | |
| `less -R` | partial | strips colour rather than showing it |
| `less +F` | refused | opened as a file |
| `less a b` | partial | one stream, no `:n` |
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
| `ping -c 4 HOST` | missing | Windows' PING.EXE: `Option -c requires administrative privileges` |
| `tree -L 2` | missing | Windows' tree.com: `Too many parameters` |
| `hostname -I`, `hostname -f` | missing | Windows' hostname.exe: `Use the Network Control Panel Applet` |
| `netstat -tulpn` | missing | Windows' netstat prints its usage; Debian's answer is `ss` |
| `ss -tulpn` | missing | |
| `ip a`, `ip r` | missing | |
| `traceroute`, `dig`, `host` | missing | Windows has `tracert` and `nslookup` |
| `lsof -i :80` | missing | |
| `column -t` | missing | |
| `pstree` | missing | `ps axf` draws the tree |
| `file NAME` | missing | |
| `tput cols`, `tput setaf 1`, `tty` | missing | |
| `man ls`, `info`, `apropos` | missing | `help ls` and `ls --help` answer |
| `zip -r a.zip dir` | missing | `unzip` is here |
| `nohup CMD &` | missing | jobs outlive the shell, not the console |
| `nice`, `renice` | missing | top's F7 and F8 step a priority class |
| `w`, `who`, `last` | missing | |
| `lscpu`, `lsblk`, `mount`, `vmstat`, `iostat` | missing | `nproc`, `df`, `free`, `top` |
| `envsubst`, `numfmt`, `fmt`, `pr`, `csplit`, `whereis` | missing | |
| `vi`, `vim` | missing | `nano`, `micro`, `ed` |
| `sudo` | missing | decided against: see the support matrix's Elevation |
| `systemctl`, `journalctl`, `apt`, `jq`, `rsync`, `tmux`, `screen` | missing | not planned |

## The shell itself

| habit | status | note |
| --- | --- | --- |
| `bash script.sh`, `bash -c`, `#!/bin/bash`, `#!/usr/bin/env bash` | missing | PATH first: WSL's System32\bash.exe where WSL is installed |
| `sh -c`, `#!/bin/sh` | works | |
| `!!`, `!$`, `^old^new`, Ctrl-R | works | |
| `ll`, `la`, `l`; `ls` and `grep` in colour | missing | no default aliases; `ls` colours only with `--color` |
| `LS_COLORS`, `dircolors` | missing | |
| `$BASH_VERSION`, `$BASH_VERSINFO` | missing | |
| `$COLUMNS`, `$LINES` at the prompt | missing | `checkwinsize` is recorded only |
| `fg`, `bg`, Ctrl-Z | refused | will not be: nothing here can suspend a job |
| `~/.bashrc`, `~/.bash_aliases` | missing | `~/.nemoshrc`, by design |
| `/proc/cpuinfo`, `/etc/os-release`, `/dev/tty` | missing | `/dev/null`, `/dev/clipboard`, `/tmp` are here |
