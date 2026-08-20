package applets

import (
	"fmt"
	"io"
	"os"
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

const lsTimeLayout = "Jan _2 15:04"

// formatLongEntry builds one `ls -l` line. path is where the file really is, which is what the
// link count and owner have to be asked about; name is what gets printed.
func formatLongEntry(path, name string, info os.FileInfo, size string) string {
	links := 1
	if count, ok := fileLinkCount(path); ok {
		links = count
	}
	owner := fileOwnerName(path)
	return fmt.Sprintf("%s%5d %-8s %-8s%10s %s %s",
		info.Mode().String(), links, owner, owner, size,
		info.ModTime().Format(lsTimeLayout), name)
}

func writeLongEntry(stdout io.Writer, path, name string, info os.FileInfo, size string) error {
	_, err := fmt.Fprintln(stdout, formatLongEntry(path, name, info, size))
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

// lsTimeOf is here rather than inline so a test can pin the layout without a clock.
func lsTimeOf(when time.Time) string { return when.Format(lsTimeLayout) }
