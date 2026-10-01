package applets

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
)

// df reports how much room each filesystem has left.
//
// Windows has no command that answers this in a form a script can read: `wmic` is gone,
// `fsutil volume diskfree` needs a drive and prints prose, and PowerShell's answer is an
// object. So this is one of the applets that is not a convenience but the only way.
//
// **A "filesystem" here is a drive letter**, and the mount point is its root -- `C:` mounted
// on `C:/`. That is what busybox-w32 reports too, and it is the honest mapping: Windows
// volumes really are mounted at a letter, and inventing `/` would name something that does
// not exist.
//
// The **percentage is of what this user can actually have**, not of the raw size: the total
// is used plus available, so quota and reserved space do not make a full disk read 80%.
// Rounded to nearest, which is the reference's rule -- `(used*100 + total/2) / total`.

// filesystemUsage is one row of the report, in bytes.
type filesystemUsage struct {
	device string
	mount  string
	// fsType is the filesystem's name, NTFS or FAT32, which -T prints and -t selects by.
	fsType string
	// used and available are what they say; total is their sum rather than the raw
	// capacity, for the reason above.
	used      uint64
	available uint64
}

func (f filesystemUsage) total() uint64 { return f.used + f.available }

// percentUsed rounds to nearest, as the reference does.
func (f filesystemUsage) percentUsed() int {
	total := f.total()
	if total == 0 {
		return 0
	}
	return int((f.used*100 + total/2) / total)
}

func newDfApplet() Applet {
	return simpleApplet{name: "df", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletOptions(ctx, args, "kmPThHa", "Bt")
		if err != nil {
			return err
		}
		view := ProcessViewFromContext(ctx)
		layout, err := diskFreeLayoutOf(view, options)
		if err != nil {
			return err
		}
		rows, err := collectFilesystems(view, operands, stderr)
		if layout.only != "" {
			rows = slices.DeleteFunc(rows, func(row filesystemUsage) bool { return row.fsType != layout.only })
		}
		if writeErr := writeDiskFree(stdout, rows, layout); writeErr != nil {
			return writeErr
		}
		return err
	}}
}

// diskFreeLayout is how df prints, busybox's options: the block it counts in -- -k's 1024, the
// 512 POSIXLY_CORRECT asks for, -m's MiB, -B's SIZE, and 0 for -h's sizes -- -P's headings,
// -T's type column, and -t's one type. -a is taken: every volume that is ready is listed.
type diskFreeLayout struct {
	unit        uint64
	posix, kind bool
	only        string
}

func diskFreeLayoutOf(view ProcessView, options appletOptions) (diskFreeLayout, error) {
	layout := diskFreeLayout{unit: 1024, posix: options.has('P'), kind: options.has('T'), only: options.value('t')}
	if _, strict := view.LookupEnv("POSIXLY_CORRECT"); strict {
		layout.unit = 512
	}
	// The last of -k, -m and -B is the one that counts, as busybox reads them.
	switch options.last("kmB") {
	case 'k':
		layout.unit = 1024
	case 'm':
		layout.unit = 1 << 20
	case 'B':
		size, err := parseSizeWithSuffix(options.value('B'))
		if bare := strings.ToUpper(options.value('B')); bare == "K" || bare == "M" || bare == "G" {
			size, err = parseSizeWithSuffix("1" + bare)
		}
		if err != nil || size == 0 {
			return layout, fmt.Errorf("invalid number '%s'", options.value('B'))
		}
		layout.unit = uint64(size)
	}
	if options.has('h') || options.has('H') {
		layout.unit = 0
	}
	return layout, nil
}

func writeDiskFree(out io.Writer, rows []filesystemUsage, layout diskFreeLayout) error {
	var page strings.Builder
	kind, use := "", "Use%"
	if layout.kind {
		kind = "Type       "
	}
	if layout.posix {
		use = "Capacity"
	}
	fmt.Fprintf(&page, "Filesystem           %s%-15sUsed Available %s Mounted on\n", kind, blockHeading(layout), use)
	for _, row := range rows {
		// A name too long for its column goes on a line of its own, as busybox puts it, but
		// for -P, which keeps a row to one line.
		device := fmt.Sprintf("%-20s", row.device)
		if len(row.device) > 20 && !layout.posix {
			device = row.device + "\n" + strings.Repeat(" ", 20)
		}
		page.WriteString(device)
		if layout.kind {
			fmt.Fprintf(&page, " %-10s", row.fsType)
		}
		fmt.Fprintf(&page, " %9s %9s %9s %3d%% %s\n",
			diskAmount(row.total(), layout.unit), diskAmount(row.used, layout.unit), diskAmount(row.available, layout.unit),
			row.percentUsed(), row.mount)
	}
	_, err := io.WriteString(out, page.String())
	return err
}

// blockHeading is the size column's heading: Size for -h, and otherwise the block, NNN-blocks,
// in K, M or G where it is a whole number of them, as busybox names it -- 1K-blocks -- and in
// full under -P, 1024-blocks.
func blockHeading(layout diskFreeLayout) string {
	if layout.unit == 0 {
		return "     Size"
	}
	unit, suffix := layout.unit, ""
	for _, letter := range []string{"K", "M", "G", "T"} {
		if layout.posix || unit < 1024 {
			break
		}
		// busybox rounds the remainder away, so 1536 is 2K.
		rounded := unit/1024 + (unit%1024*10+512)/1024/5
		unit, suffix = rounded, letter
	}
	return fmt.Sprintf("%d%s-blocks", unit, suffix)
}

// diskAmount renders a byte count as blocks or as a human-readable size.
//
// **One decimal, always**, which is busybox's rule rather than GNU's: `df -h` says `111.8G`
// where `du -h` in this same shell says `112G`. The two commands really do differ, so
// humanBlocks is not reused here -- sharing it would make one of them wrong.
func diskAmount(bytes uint64, unit uint64) string {
	if unit != 0 {
		// Blocks of the unit, rounded to nearest, as busybox's make_human_readable_str
		// counts them: -m's 26890.34 MiB is 26890.
		return fmt.Sprint((bytes + unit/2) / unit)
	}
	value := float64(bytes)
	for _, unit := range []string{"", "K", "M", "G", "T"} {
		if value < 1024 {
			if unit == "" {
				return fmt.Sprint(uint64(value))
			}
			return fmt.Sprintf("%.1f%s", value, unit)
		}
		value /= 1024
	}
	return fmt.Sprintf("%.1fP", value)
}
