package applets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// `ls -l` printed a mode, a size and a name. No timestamp, no link count, no owner -- so the
// column people actually read was missing, and the output was three fields where every other
// `ls` prints seven.
//
// The layout is busybox-w32's, derived from its output rather than guessed:
//
//	-rw-rw-r--    1 nemo     nemo        200001 Aug 20 11:28 big.txt
//	drwxrwxr-x    3 nemo     nemo             0 Aug 20 02:08 dir
//	-rwxrwxr-x    2 root     root       6090176 Jun 12 16:29 explorer.exe
//
// mode, then the link count right-aligned in five, the owner and group left-aligned in eight,
// the size right-aligned in ten, `MMM DD HH:MM`, and the name. -g leaves the owner out, -n
// prints ids for both, -h a size of seven, and --full-time the date and time in full.
//
// The group repeats the owner, which is what busybox does and is worth stating because it is
// not what Windows would say: the real primary group of a file owned by a local account is
// `None`, measured. A column reading `None` would look like a fault, and it carries nothing a
// script can use either way.
//
// The time is always `HH:MM`, where GNU switches to a year for anything over six months old.
// busybox does not, and a script parsing the column benefits from one shape.
//
// The mode is still Go's -- `-rw-rw-rw-` where busybox says `-rw-rw-r--` and uutils' own tests
// expect `(r[w-]x){3}`. Three answers to a question Windows does not really have; this one at
// least follows from something (os.FileMode), and it is not what this change was about.

// The two date columns, both twelve characters wide so the name always starts in the same
// place. Measured against busybox-w32 with crafted timestamps: the day is zero-padded, and the
// year replaces the time for anything more than six months old or more than an hour in the
// future. This printed `Apr  4 10:00` for a file from 2024, where busybox prints `Apr 04  2024`
// -- so a listing of an old directory said nothing about which year anything was from.
const (
	lsTimeLayout = "Jan 02 15:04"
	lsYearLayout = "Jan 02  2006"
	// lsRecentWindow is how far back the time is still worth more than the year. GNU's
	// rule, and busybox follows it.
	lsRecentWindowMonths = -6
	lsFutureAllowance    = time.Hour
)

// formatLongEntry builds one `ls -l` line, as busybox's display_single lays it out: the mode, the
// link count in four, the owner and the group each in eight -- only the group under -g, and
// ids under -n -- the size in nine, or seven for -h, the time, and the name. The inode and
// blocks columns -i and -s put before it are lsEntryPrefix's.
func formatLongEntry(entry lsEntry, options lsOptions) string {
	name := lsDisplayName(entry, options)
	mode := lsModeString(entry.path, entry.info, options.umask)
	size := lsSizeText(entry.info.Size(), options)
	// A link says so in the first column and names its target, which is what busybox does
	// and what this did not: the ten junctions in a home directory came out as
	// `?rw-rw-rw-` with a size of 0 and no target at all. A link's size is the length of
	// that target, which is POSIX and which busybox also prints, and its indicator is the
	// target's, after it.
	if target, ok := linkTarget(entry.path, entry.info); ok {
		// `lrwxrwxrwx`, not the target's own bits: a link's permissions are not
		// consulted for anything, and every ls prints them wide open.
		mode = "lrwxrwxrwx"
		size = lsSizeText(int64(len(target)), options)
		name = paintLsName(lsNameText(entry.name, options), entry.info, options.colored) + " -> " + target
		if followed, err := os.Stat(entry.path); err == nil {
			name += classifyLsSuffix(followed, options)
		}
	}
	var line strings.Builder
	fmt.Fprintf(&line, "%-10s %4d ", mode, entry.links())
	owner := longEntryOwner(entry.path, entry.info)
	switch uid, gid := entry.ids(); {
	case options.numeric && options.groupOnly:
		fmt.Fprintf(&line, "%-8d ", gid)
	case options.numeric:
		fmt.Fprintf(&line, "%-8d %-8d ", uid, gid)
	case options.groupOnly:
		fmt.Fprintf(&line, "%-8s ", owner)
	default:
		fmt.Fprintf(&line, "%-8s %-8s ", owner, owner)
	}
	switch {
	case entry.info.Mode()&os.ModeDevice != 0:
		// A device has no size, and busybox and GNU ls both put its major and minor
		// numbers in that column instead. Ours are zero and honestly so: these devices
		// are provided by the shell rather than by a driver, so there is no pair of
		// numbers to report. `0,   0` is exactly what busybox prints for /dev/null.
		line.WriteString("   0,   0 ")
	case options.human:
		fmt.Fprintf(&line, "%7s ", size)
	default:
		fmt.Fprintf(&line, "%9s ", size)
	}
	when := entry.when(options)
	if options.fullTime {
		line.WriteString(when.Format("2006-01-02 15:04:05 -0700"))
	} else {
		line.WriteString(lsTimeColumn(when, time.Now()))
	}
	line.WriteString(" " + name)
	return line.String()
}

// linkTarget is where a link points, spelled the way this shell spells paths.
//
// os.Readlink rather than anything platform-specific: it resolves a Windows junction as well
// as a symlink, which was worth checking rather than assuming -- the mode bits do not say so.
func linkTarget(path string, info os.FileInfo) (string, bool) {
	if !isSymbolicLink(info) {
		return "", false
	}
	target, err := os.Readlink(path)
	if err != nil {
		return "", false
	}
	return filepath.ToSlash(target), true
}

// ownerNames caches the account a SID or uid belongs to.
//
// A directory listing usually has one or two distinct owners, and resolving one costs a
// lookup that may go to a domain controller. Measured over 4,895 files in System32: 170µs per
// file without this and 29µs with it, the remainder being the per-file security query that
// cannot be cached.
var ownerNames = map[string]string{}

func cachedOwnerName(key string, resolve func() string) string {
	if name, ok := ownerNames[key]; ok {
		return name
	}
	name := resolve()
	ownerNames[key] = name
	return name
}

// lsTimeColumn is the date column: the time for something recent, the year otherwise.
//
// now is a parameter so a test can pin the boundary without waiting for it.
func lsTimeColumn(when, now time.Time) string {
	if when.After(now.Add(lsFutureAllowance)) || when.Before(now.AddDate(0, lsRecentWindowMonths, 0)) {
		return when.Format(lsYearLayout)
	}
	return when.Format(lsTimeLayout)
}

// lsModeString is the ten-character mode column: one character for what the entry is, then
// nine for its permissions.
//
// os.FileMode.String() cannot be used for a *column*, which is the trap this fell into. It
// emits one character per set bit from a fixed list, so a thing that is two things at once
// gets two: a OneDrive folder is a directory *and* a reparse point, and Go answered
// `d?r-xr-xr-x` -- eleven characters, shifting every column after it one place right. Found by
// running `ls -alh` in a home directory that has OneDrive in it, which is a shape no temporary
// directory in a test was ever going to produce.
//
// The permissions are made up as stat makes them up, by currentPermissions: on Windows
// busybox-w32's, read and write for everyone less the umask's group and other write, and run
// for a directory or a program. They were Go's, `-rw-rw-rw-` where busybox-w32 and this build's
// own stat both said `-rw-r--r--`; a set-id or sticky bit is now shown where it stands too.
func lsModeString(native string, info os.FileInfo, umask uint32) string {
	kind := uint32(0o100000)
	switch {
	case info.IsDir():
		kind = 0o040000
	case info.Mode()&os.ModeNamedPipe != 0:
		kind = 0o010000
	case info.Mode()&os.ModeSocket != 0:
		kind = 0o140000
	case info.Mode()&os.ModeCharDevice != 0:
		kind = 0o020000
	case info.Mode()&os.ModeDevice != 0:
		kind = 0o060000
	}
	// A device or a pipe keeps the bits it was described with: /dev/null is crw-rw-rw-.
	if !info.IsDir() && !info.Mode().IsRegular() {
		return statModeString(kind | bitsOfFileMode(info.Mode()))
	}
	return statModeString(kind | currentPermissions(native, info, umask))
}

// longEntryOwner names the account in the owner column.
//
// A device is not on disk, so there is no security descriptor to read and the lookup would fall back
// to `root` -- which put `root` beside a device and `nemo` beside every real file in the same
// listing, which reads as a fault rather than as a distinction. busybox fills its synthetic stat
// with the current uid and prints the current user for both; the current account is also the honest
// answer, since it is the one that can read and write the device.
func longEntryOwner(path string, info os.FileInfo) string {
	if info.Mode()&os.ModeDevice != 0 {
		if name := accountName(); name != "" {
			return name
		}
		return "root"
	}
	return fileOwnerName(path)
}
