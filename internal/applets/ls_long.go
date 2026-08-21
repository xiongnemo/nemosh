package applets

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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
// the size right-aligned in ten, `MMM DD HH:MM`, and the name.
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

// formatLongEntry builds one `ls -l` line. path is where the file really is, which is what the
// link count and owner have to be asked about; name is what gets printed.
func formatLongEntry(path, name string, info os.FileInfo, size string, sizeField int) string {
	links := 1
	if count, ok := fileLinkCount(path); ok {
		links = count
	}
	owner := longEntryOwner(path, info)
	mode := lsModeString(info)
	// A link says so in the first column and names its target, which is what busybox does
	// and what this did not: the ten junctions in a home directory came out as
	// `?rw-rw-rw-` with a size of 0 and no target at all. A link's size is the length of
	// that target, which is POSIX and which busybox also prints.
	if info.Mode()&os.ModeDevice != 0 {
		// A device has no size, and busybox and GNU ls both put its major and minor
		// numbers in that column instead. Ours are zero and honestly so: these devices
		// are provided by the shell rather than by a driver, so there is no pair of
		// numbers to report. `0,   0` is exactly what busybox prints for /dev/null.
		size = "0,   0"
	}
	if target, ok := linkTarget(path, info); ok {
		// `lrwxrwxrwx`, not the target's own bits: a link's permissions are not
		// consulted for anything, and every ls prints them wide open.
		mode = "lrwxrwxrwx"
		size = strconv.Itoa(len(target))
		name += " -> " + target
	}
	return fmt.Sprintf("%s%5d %-8s %-8s%*s %s %s",
		mode, links, owner, owner, sizeField, size,
		lsTimeColumn(info.ModTime(), time.Now()), name)
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

func writeLongEntry(stdout io.Writer, path, name string, info os.FileInfo, size string, sizeField int) error {
	_, err := fmt.Fprintln(stdout, formatLongEntry(path, name, info, size, sizeField))
	return err
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
func lsModeString(info os.FileInfo) string {
	// Perm().String() is always ten characters with a leading `-`, so its tail is exactly
	// the nine permission characters.
	permissions := info.Mode().Perm().String()[1:]
	switch {
	case info.IsDir():
		return "d" + permissions
	case info.Mode()&os.ModeNamedPipe != 0:
		return "p" + permissions
	case info.Mode()&os.ModeSocket != 0:
		return "s" + permissions
	case info.Mode()&os.ModeDevice != 0:
		if info.Mode()&os.ModeCharDevice != 0 {
			return "c" + permissions
		}
		return "b" + permissions
	default:
		return "-" + permissions
	}
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
