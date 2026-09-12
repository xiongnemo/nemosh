package applets

import (
	"context"
	"fmt"
	"io"
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
		options, operands, err := parseAppletOptions(args, "hk", "")
		if err != nil {
			return err
		}
		rows, err := collectFilesystems(ProcessViewFromContext(ctx), operands, stderr)
		if err != nil {
			return err
		}
		return writeDiskFree(stdout, rows, options.has('h'))
	}}
}

func writeDiskFree(out io.Writer, rows []filesystemUsage, human bool) error {
	units := "1K-blocks      "
	if human {
		// The reference right-aligns this one, which is why it is not the same shape as
		// the block header.
		units = "     Size      "
	}
	var page strings.Builder
	page.WriteString("Filesystem           " + units + "Used Available Use% Mounted on\n")
	for _, row := range rows {
		fmt.Fprintf(&page, "%-20s %9s %9s %9s %3d%% %s\n",
			row.device,
			diskAmount(row.total(), human), diskAmount(row.used, human), diskAmount(row.available, human),
			row.percentUsed(), row.mount)
	}
	_, err := io.WriteString(out, page.String())
	return err
}

// diskAmount renders a byte count as blocks or as a human-readable size.
//
// **One decimal, always**, which is busybox's rule rather than GNU's: `df -h` says `111.8G`
// where `du -h` in this same shell says `112G`. The two commands really do differ, so
// humanBlocks is not reused here -- sharing it would make one of them wrong.
func diskAmount(bytes uint64, human bool) string {
	if !human {
		// Kilobyte blocks, rounded up: a file that occupies part of a block still
		// occupies the block.
		return fmt.Sprint((bytes + 1023) / 1024)
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
