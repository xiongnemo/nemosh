package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// du reports how much disk a tree uses, and stat reports what one file is.

// duBlock is the unit du counts in. GNU's default is 1024-byte blocks, which is
// why `du -s .` on a 17KB tree says 17 rather than 17408.
//
// The totals are what the filesystem *allocated*, which is what the name means. They used to
// be apparent sizes rounded up to a block, and that was documented as a deliberate
// simplification on the grounds that Go cannot read allocation size portably -- true of
// os.FileInfo, and not true of the platform underneath it. See file_details_windows.go.
const duBlock = 1024

// humanBlocks is GNU's -h: the largest unit that leaves a number under 1024,
// with one decimal below 10. `du -sh` on the tree measured said `17K`.
func humanBlocks(blocks int64) string {
	value := float64(blocks)
	for _, unit := range []string{"K", "M", "G", "T"} {
		if value < 1024 {
			// A whole number keeps its decimal: both references print `4.0K` and this
			// printed `4K`. Above ten the decimal goes, which is GNU's rule; busybox
			// keeps one there too and says `96.7K` where GNU and this say `97K`.
			if value < 10 {
				return fmt.Sprintf("%.1f%s", value, unit)
			}
			return fmt.Sprintf("%d%s", int64(value), unit)
		}
		value /= 1024
	}
	return fmt.Sprintf("%.1fP", value)
}

// humanReadable is busybox's make_human_readable_str: value blocks of blockSize bytes, divided by
// unit and rounded to the nearest, or for a unit of 0 scaled to the largest of K M G T P E with
// one decimal, `4.0K`, and a bare number under 1024.
func humanReadable(value, blockSize, unit uint64) string {
	if value == 0 {
		return "0"
	}
	if blockSize > 1 {
		value *= blockSize
	}
	if unit != 0 {
		return strconv.FormatUint((value+unit/2)/unit, 10)
	}
	suffixes := []string{"", "K", "M", "G", "T", "P", "E"}
	index, fraction := 0, uint64(0)
	for value >= 1024 && index < len(suffixes)-1 {
		index++
		fraction = (value%1024*10 + 1024/2) / 1024
		value /= 1024
	}
	if fraction >= 10 {
		value, fraction = value+1, 0
	}
	if index == 0 {
		return strconv.FormatUint(value, 10)
	}
	return fmt.Sprintf("%d.%d%s", value, fraction, suffixes[index])
}

// stat reports what a file is, through `-c` and a format string.
//
// Only `-c` is implemented, and only the specifiers below. The default output --
// GNU's multi-line block with inode numbers, device ids, permission bits in two
// notations and three timestamps -- is mostly fields Windows either does not have
// or reports through an entirely different API. Printing a block of zeroes and
// question marks would be the kind of answer a script cannot tell from a real
// one.
//
//	%n  name as given      %s  size in bytes
//	%F  file type          %f  raw mode, in hex
//	%y  modification time  %Y  the same as a Unix timestamp
func newStatApplet() Applet {
	return simpleApplet{name: "stat", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(ctx, args, "", "c")
		if err != nil {
			return err
		}
		if !options.has('c') {
			return fmt.Errorf("only the -c FORMAT form is implemented; the default output is mostly fields Windows does not have")
		}
		if len(paths) == 0 {
			return missingOperand()
		}
		view := ProcessViewFromContext(ctx)
		stated := true
		for _, path := range paths {
			native, err := resolveHostPath(view, path)
			if err != nil {
				return err
			}
			info, err := os.Stat(native)
			if err != nil {
				// Of several operands, one that is not there is named and the rest described; see
				// operand_reporter.go.
				if len(paths) == 1 || !reportOperand(ctx, cannotOpen(path, err)) {
					return cannotOpen(path, err)
				}
				stated = false
				continue
			}
			line, err := formatStat(options.value('c'), path, info)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintln(stdout, line); err != nil {
				return err
			}
		}
		if !stated {
			return ExitStatus(1)
		}
		return nil
	}}
}

// formatStat expands the format, refusing a specifier it does not implement
// rather than leaving it on the line.
//
// Leaving `%i` as the literal text `%i` is the failure mode worth avoiding: a
// script would put it in a filename and never find out why.
func formatStat(format, name string, info os.FileInfo) (string, error) {
	var out strings.Builder
	for index := 0; index < len(format); index++ {
		if format[index] != '%' || index+1 >= len(format) {
			out.WriteByte(format[index])
			continue
		}
		index++
		switch format[index] {
		case 'n':
			out.WriteString(name)
		case 's':
			fmt.Fprintf(&out, "%d", info.Size())
		case 'F':
			out.WriteString(statFileType(info))
		case 'f':
			fmt.Fprintf(&out, "%x", uint32(info.Mode().Perm()))
		case 'y':
			out.WriteString(info.ModTime().Format("2006-01-02 15:04:05.000000000 -0700"))
		case 'Y':
			fmt.Fprintf(&out, "%d", info.ModTime().Unix())
		case '%':
			out.WriteByte('%')
		default:
			return "", fmt.Errorf("unsupported format specifier: %%%c", format[index])
		}
	}
	return out.String(), nil
}

func statFileType(info os.FileInfo) string {
	switch {
	case info.IsDir():
		return "directory"
	case info.Mode()&os.ModeSymlink != 0:
		return "symbolic link"
	case !info.Mode().IsRegular():
		return "special file"
	}
	return "regular file"
}
