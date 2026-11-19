package applets

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// df's GNU options, which busybox's df has not got: --output[=FIELDS], -i, more than one -t, and
// -x TYPE. Debian's df is GNU's, and `df --output=source,size,pcent,target` and `df -hT -x tmpfs`
// are typed at it.
//
// --output and -i print GNU's columns rather than busybox's: each as wide as its heading, what is
// in it and GNU's least, numbers to the right and names to the left, the last unpadded. Windows
// keeps no count of inodes an unelevated session may read, so those fields are `-`, which is
// what GNU's df prints for a filesystem that has none, vfat's.

var (
	dfAllFields   = []string{"source", "fstype", "itotal", "iused", "iavail", "ipcent", "size", "used", "avail", "pcent", "file", "target"}
	dfInodeFields = []string{"source", "itotal", "iused", "iavail", "ipcent", "target"}
)

// dfOutputFields takes --output out of args, which the letters are read from: its fields, all
// of them for --output alone, and what is left. --output's list is only after `=`, as GNU's is,
// so `df --output /` is every field of the filesystem / is on.
func dfOutputFields(args []string) ([]string, []string, error) {
	var fields []string
	rest := make([]string, 0, len(args))
	for index, arg := range args {
		switch {
		case arg == "--":
			return fields, append(rest, args[index:]...), nil
		case arg == "--output":
			fields = dfAllFields
		case strings.HasPrefix(arg, "--output="):
			fields = nil
			for _, field := range strings.Split(strings.TrimPrefix(arg, "--output="), ",") {
				if !slices.Contains(dfAllFields, field) {
					return nil, nil, fmt.Errorf("--output: field '%s' unknown", field)
				}
				if slices.Contains(fields, field) {
					return nil, nil, fmt.Errorf("--output: field '%s' used more than once", field)
				}
				fields = append(fields, field)
			}
		default:
			rest = append(rest, arg)
		}
	}
	return fields, rest, nil
}

// selectFilesystems keeps -t's types, when any was given, and leaves out -x's, without regard to
// case: Windows names them NTFS and FAT32 where a Linux reader types ntfs.
func selectFilesystems(rows []filesystemUsage, layout diskFreeLayout) []filesystemUsage {
	named := func(names []string, row filesystemUsage) bool {
		return slices.ContainsFunc(names, func(name string) bool { return strings.EqualFold(name, row.fsType) })
	}
	var kept []filesystemUsage
	for _, row := range rows {
		if (len(layout.only) == 0 || named(layout.only, row)) && !named(layout.exclude, row) {
			kept = append(kept, row)
		}
	}
	return kept
}

// dfField is a column of GNU's: its heading, its least width, whether it is a number, and what
// a row holds in it.
func dfField(name string, layout diskFreeLayout) (string, int, bool, func(filesystemUsage) string) {
	none := func(filesystemUsage) string { return "-" }
	switch name {
	case "source":
		return "Filesystem", 14, false, func(row filesystemUsage) string { return row.device }
	case "fstype":
		return "Type", 4, false, func(row filesystemUsage) string { return row.fsType }
	case "itotal":
		return "Inodes", 5, true, none
	case "iused":
		return "IUsed", 5, true, none
	case "iavail":
		return "IFree", 5, true, none
	case "ipcent":
		return "IUse%", 4, true, none
	case "size":
		return strings.TrimSpace(blockHeading(diskFreeLayout{unit: layout.unit})), 5, true,
			func(row filesystemUsage) string { return diskAmount(row.total(), layout.unit) }
	case "used":
		return "Used", 5, true, func(row filesystemUsage) string { return diskAmount(row.used, layout.unit) }
	case "avail":
		return "Avail", 5, true, func(row filesystemUsage) string { return diskAmount(row.available, layout.unit) }
	case "pcent":
		return "Use%", 4, true, func(row filesystemUsage) string { return fmt.Sprintf("%d%%", row.percentUsed()) }
	case "file":
		return "File", 0, false, func(row filesystemUsage) string {
			if row.file == "" {
				return "-"
			}
			return row.file
		}
	}
	return "Mounted on", 0, false, func(row filesystemUsage) string { return row.mount }
}

// writeDiskFreeFields prints the fields named, GNU's way.
func writeDiskFreeFields(out io.Writer, rows []filesystemUsage, layout diskFreeLayout, fields []string) error {
	table := make([][]string, len(rows)+1)
	widths := make([]int, len(fields))
	numeric := make([]bool, len(fields))
	for column, name := range fields {
		heading, least, number, cell := dfField(name, layout)
		numeric[column] = number
		widths[column] = max(least, len(heading))
		table[0] = append(table[0], heading)
		for index, row := range rows {
			text := cell(row)
			table[index+1] = append(table[index+1], text)
			widths[column] = max(widths[column], len(text))
		}
	}
	var page strings.Builder
	for _, line := range table {
		for column, text := range line {
			if column > 0 {
				page.WriteByte(' ')
			}
			switch {
			case numeric[column]:
				fmt.Fprintf(&page, "%*s", widths[column], text)
			case column == len(line)-1:
				page.WriteString(text)
			default:
				fmt.Fprintf(&page, "%-*s", widths[column], text)
			}
		}
		page.WriteByte('\n')
	}
	_, err := io.WriteString(out, page.String())
	return err
}
