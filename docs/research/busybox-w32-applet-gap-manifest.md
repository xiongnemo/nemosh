# BusyBox-w32 Applet Gap Manifest

This manifest is a name-level gap table between the local busybox-w32
reference and the current Nemosh applet registry. It is a planning artifact for
the broader v0 goal; name overlap does not imply full BusyBox parity.

Relevant scope and test rules:

- `docs/design/v0-scope.md` makes Nemosh a native Windows-first BusyBox-style
  shell and utility bundle, while explicitly saying the first implementation
  cut does not require full BusyBox applet parity.
- `docs/testing/applet-test-inventory.md` says no applet should be marked done
  without at least smoke and negative tests.
- `AGENTS.md` says Nemosh design should primarily reference busybox-w32.

## Provenance

Derived on 2026-07-07 from the local working tree:

- BusyBox-w32 source: `references/windows-compat/busybox-w32`, shallow clone at
  `b5c1253` per `docs/research/reference-clone-status.md`.
- BusyBox applet names: unique first macro argument from `//applet:` lines that
  contain `APPLET`, `APPLET_ODDNAME`, `APPLET_NOEXEC`, `APPLET_NOFORK`, or
  `APPLET_SCRIPTED` in `*.c` files under the clone.
- Nemosh applet names: constructor calls in `internal/applets/registry.go`,
  including literal names passed to `newTestApplet(...)`.

Current derived counts:

| Set | Count |
| --- | ---: |
| BusyBox-w32 declared applet names | 449 |
| Nemosh registry applet names | 38 |
| Name-present overlap | 36 |
| BusyBox-w32 names missing from Nemosh | 413 |
| Nemosh-only helper names | 2 |

## Name-Present In Nemosh

These names exist in both the busybox-w32 declaration set and the current
Nemosh registry. Treat them as implementation inventory, not full parity
claims; each applet still needs its own behavior matrix and tests before a
release milestone can call it complete.

| Applets |
| --- |
| `[`, `basename`, `cat`, `chmod`, `cp`, `date`, `dirname`, `echo`, `env`, `false`, `find`, `grep`, `head`, `ln`, `ls`, `mkdir`, `mv`, `printenv`, `printf`, `pwd`, `readlink`, `realpath`, `rm`, `rmdir`, `sed`, `sleep`, `sort`, `tail`, `test`, `touch`, `true`, `uname`, `uniq`, `wc`, `xargs`, `yes` |

## Known Partial Parity Notes

These notes record already-known subset choices from recent slices and design
docs. They are not an exhaustive parity audit.

| Applet | Current note |
| --- | --- |
| `date` | Supports current time output, `-u`, deterministic `-d @SECONDS`, and a small explicit `+FORMAT` subset including `%s`; system time setting, broad date parsing, RFC/ISO modes, file timestamp mode, and long options remain out of scope. |
| `ls` | Supports a small `-a`/`-l`/`-h` subset; columns, color, recursion, symlink arrows, owner/group/timestamp parity, totals, and broader Windows attribute behavior remain out of scope for that slice. |
| `ln` | Supports hard links and `-s`; options such as `-f`, `-n`, `-T`, backup modes, verbose output, directory handling, and target rewriting remain missing. |
| `readlink` | Supports `FILE` and `-n FILE`; canonicalization and many BusyBox options remain missing. |
| `realpath` | Supports `realpath FILE...` with missing-final-component behavior; option parsing and broader canonicalization parity still need later audit. |
| `sleep` | Supports BusyBox-style duration operands with `s`, `m`, `h`, and `d` suffixes; broader option compatibility and signal/interruption parity remain out of scope. |
| `sort` | Supports stdin/files, whole-line lexical sort, integer `-n`, `-r`, clustered short options for that subset, CRLF normalization, and status-2 option/open diagnostics; key fields, unique/stable/NUL/output-file/check modes, locale, month, human, and version sorting remain out of scope. |
| `uname` | Supports BusyBox-w32-style default/system output, clustered short options, and explicit unknown processor/platform fields; broader kernel/hardware parity is intentionally limited by the native Windows host. |
| `uniq` | Supports default adjacent duplicate collapse from stdin, `-`, and one input file, CRLF normalization, and status-2 diagnostics for invalid options, missing input files, and extra operands; count/duplicate/unique-only/case-insensitive/NUL/field-skip/char-skip/width modes and `[OUTFILE]` remain out of scope. |
| `xargs` | Present, but `-0` is currently rejected in tests; broader POSIX/BusyBox option coverage is incomplete. |

## Nemosh-Only Helpers

These are intentional Windows-native helpers rather than busybox-w32 applet
names.

| Applet | Purpose |
| --- | --- |
| `posixpath` | Convert Windows spelling to Nemosh POSIX-drive/UNC spelling at explicit helper boundaries. |
| `winpath` | Convert Nemosh POSIX-drive/UNC spelling to Windows spelling at explicit helper boundaries. |

## BusyBox-w32 Names Missing From Nemosh

These applets are declared by the local busybox-w32 reference but are not in the
current Nemosh registry. This table is tracking information only; it does not
mean every name should be pulled into the next slice.

```text
[[
acpid
add-shell
addgroup
adduser
adjtimex
ar
arch
arp
arping
ascii
ash
awk
base32
base64
bash
bbconfig
bc
beep
blkdiscard
blkid
blockdev
bootchartd
brctl
bunzip2
busybox
bzcat
bzip2
cal
cdrop
chat
chattr
chcon
chgrp
chown
chpasswd
chpst
chroot
chrt
chvt
cksum
clear
cmp
comm
conspy
cpio
crc32
crond
crontab
cryptpw
cttyhack
cut
dc
dd
deallocvt
delgroup
deluser
depmod
devfsd
devmem
df
dhcprelay
diff
dmesg
dnsd
dnsdomainname
dos2unix
dpkg
dpkg-deb
drop
du
dumpkmap
dumpleases
ed
egrep
eject
envdir
envuidgid
ether-wake
expand
expr
factor
fakeidentd
fallocate
fatattr
fbset
fbsplash
fdflush
fdformat
fdisk
fgconsole
fgrep
findfs
flash_eraseall
flash_lock
flash_unlock
flashcp
flock
fold
free
freeramdisk
fsck
fsck.minix
fsfreeze
fstrim
fsync
ftpd
ftpget
ftpput
fuser
getenforce
getfattr
getopt
getsebool
getty
groups
gunzip
gzip
halt
hd
hdparm
hexdump
hexedit
hostid
hostname
httpd
hush
hwclock
i2cdetect
i2cdump
i2cget
i2cset
i2ctransfer
iconv
id
ifconfig
ifdown
ifenslave
ifplugd
ifup
inetd
init
inotifyd
insmod
install
ionice
iostat
ip
ipaddr
ipcalc
ipcrm
ipcs
iplink
ipneigh
iproute
iprule
iptunnel
jn
join
kbd_mode
kill
killall
killall5
klogd
lash
last
less
link
linux32
linux64
linuxrc
load_policy
loadfont
loadkmap
logger
login
logname
logread
losetup
lpd
lpq
lpr
lsattr
lsblk
lsmod
lsof
lspci
lsscsi
lsusb
lzcat
lzma
lzop
lzopcat
make
makedevs
makemime
man
matchpathcon
md5sum
mdev
mesg
microcom
mim
minips
mkdosfs
mke2fs
mkfifo
mkfs.ext2
mkfs.minix
mkfs.reiser
mkfs.vfat
mknod
mkpasswd
mkswap
mktemp
modinfo
modprobe
more
mount
mountpoint
mpstat
mt
nameif
nanddump
nandwrite
nbd-client
nc
netcat
netstat
nice
nl
nmeter
nohup
nologin
nproc
nsenter
nslookup
ntpd
nuke
od
openvt
partprobe
passwd
paste
patch
pdpmake
pdrop
pgrep
pidof
ping
ping6
pipe_progress
pivot_root
pkill
pmap
popmaildir
poweroff
powertop
ps
pscan
pstree
pwdx
raidautorun
rdate
rdev
readahead
readprofile
reboot
reformime
remove-shell
renice
reset
resize
restorecon
resume
rev
rfkill
rmmod
route
rpm
rpm2cpio
rtcwake
run-init
run-parts
runcon
runlevel
runsv
runsvdir
rx
script
scriptreplay
seedrng
selinuxenabled
sendmail
seq
sestatus
setarch
setconsole
setenforce
setfattr
setfiles
setfont
setkeycodes
setlogcons
setpriv
setsebool
setserial
setsid
setuidgid
sh
sha1sum
sha256sum
sha384sum
sha3sum
sha512sum
showkey
shred
shuf
slattach
smemcap
softlimit
split
ssl_client
ssl_server
start-stop-daemon
stat
strings
stty
su
sulogin
sum
sv
svc
svlogd
svok
swapoff
swapon
switch_root
sync
sysctl
syslogd
tac
tar
taskset
tc
tcpsvd
tee
telnet
telnetd
tftp
tftpd
time
timeout
top
tr
traceroute
traceroute6
tree
truncate
ts
tsort
tty
ttysize
tunctl
tune2fs
ubiattach
ubidetach
ubimkvol
ubirename
ubirmvol
ubirsvol
ubiupdatevol
udhcpc
udhcpc6
udhcpd
udpsvd
uevent
umount
uncompress
unexpand
unit
unix2dos
unlink
unlzma
unlzop
unshare
unxz
unzip
uptime
users
usleep
uudecode
uuencode
uuidgen
vconfig
vi
vlock
vmstat
volname
w
wall
watch
watchdog
wget
which
who
whoami
whois
xxd
xz
xzcat
zcat
zcip
```

## Next Slice Guidance

Prefer small script-critical coreutils next, especially missing names already
called out by earlier review as common shell-script dependencies:

```text
cut tr tee du df seq expr od hexdump
```

Each slice should start from busybox-w32 source/tests, add focused failing tests,
implement the smallest compatible behavior, run diagnostics and tests, then
record any intentional parity gap here or in a per-applet note.
