package applets

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
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

// usageBlocks is what the file occupies, in du's blocks.
//
// The allocated size where the platform will say -- a ten-byte file inside one NTFS cluster
// occupies four kilobytes, and reporting 1 for it made `du` a slower `wc -c`. busybox-w32
// answers 4 and so does this now. Where the allocation cannot be read the apparent size is
// used instead, rounded up, because a number that is optimistic beats no number at all. See
// file_details_windows.go for the call and what it costs.
func usageBlocks(path string, size int64, isDir bool) int64 {
	if allocated, ok := allocatedSize(path, size, isDir); ok {
		return (allocated + duBlock - 1) / duBlock
	}
	return (size + duBlock - 1) / duBlock
}

func newDuApplet() Applet {
	return simpleApplet{name: "du", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		options, paths, err := parseAppletOptions(args, "sh", "")
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			paths = []string{"."}
		}
		view := ProcessViewFromContext(ctx)
		for _, path := range paths {
			// A device tree has no blocks: every entry is synthetic, so the total is
			// zero and saying so is more useful than refusing the path. `du -s /dev`
			// answering 0 is also what `du` on a real /dev reports, since a device
			// occupies no data blocks there either.
			if handled, err := reportDeviceUsage(ctx, stdout, view, path, options.has('h')); err != nil {
				return err
			} else if handled {
				continue
			}
			native, err := resolveHostPath(view, path)
			if err != nil {
				return err
			}
			// Cleaned, which on Windows also settles the separators. Without it
			// `native` kept the forward slashes the shell resolves to while
			// WalkDir handed back backslashes, so no path ever matched its own
			// parent: every total was one block and the names came out as
			// `dutest/C:/Users/...`.
			native = filepath.Clean(native)
			if err := reportUsage(ctx, stdout, stderr, native, path, options.has('s'), options.has('h')); err != nil {
				return err
			}
		}
		return nil
	}}
}

// reportUsage walks one operand.
//
// A file it cannot read is reported and the walk continues. That is GNU's
// behaviour and the useful one: a permission-denied directory somewhere under a
// tree should not turn a total into nothing, and the diagnostic on stderr keeps
// the number on stdout honest about being partial.
func reportUsage(ctx context.Context, stdout, stderr io.Writer, native, shown string, summarise, human bool) error {
	// A file operand is answered before the walk, because the walk cannot answer it. Only
	// directories are ever recorded in `order` below -- a file's blocks are charged to its
	// parents and not to itself, which is right for a file *inside* a tree and wrong for
	// one named on the command line. So `du somefile` printed nothing at all and exited 0,
	// and `du -sh somefile` printed `0K` for a file with bytes in it. Both silent.
	if info, err := os.Stat(native); err == nil && !info.IsDir() {
		return writeUsageLine(stdout, usageBlocks(native, info.Size(), false), shown, human)
	}
	totals := map[string]int64{}
	var order []string
	walkErr := filepath.WalkDir(native, func(current string, entry fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			fmt.Fprintf(stderr, "du: cannot read %s: %v\n", filepath.ToSlash(current), err)
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			fmt.Fprintf(stderr, "du: cannot stat %s: %v\n", filepath.ToSlash(current), err)
			return nil
		}
		blocks := usageBlocks(current, info.Size(), entry.IsDir())
		if entry.IsDir() {
			order = append(order, current)
		}
		// Charged to every directory above it, which is what makes the
		// non-summary form's numbers cumulative.
		for directory := current; ; directory = filepath.Dir(directory) {
			if entry.IsDir() || directory != current {
				totals[directory] += blocks
			}
			if directory == native || filepath.Dir(directory) == directory {
				break
			}
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}
	if summarise {
		return writeUsageLine(stdout, totals[native], shown, human)
	}
	// Deepest first, root last. GNU prints a directory after the ones inside it,
	// and the property that depends on is `du | tail -1` being the total -- a
	// common enough idiom that pre-order would be a silent difference.
	for index := len(order) - 1; index >= 0; index-- {
		directory := order[index]
		name := shown
		if directory != native {
			name = filepath.ToSlash(filepath.Join(shown, strings.TrimPrefix(directory, native+string(filepath.Separator))))
		}
		if err := writeUsageLine(stdout, totals[directory], name, human); err != nil {
			return err
		}
	}
	return nil
}

func writeUsageLine(stdout io.Writer, blocks int64, name string, human bool) error {
	if human {
		_, err := fmt.Fprintf(stdout, "%s\t%s\n", humanBlocks(blocks), name)
		return err
	}
	_, err := fmt.Fprintf(stdout, "%d\t%s\n", blocks, name)
	return err
}

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
		options, paths, err := parseAppletOptions(args, "", "c")
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
		for _, path := range paths {
			native, err := resolveHostPath(view, path)
			if err != nil {
				return err
			}
			info, err := os.Stat(native)
			if err != nil {
				return cannotOpen(path, err)
			}
			line, err := formatStat(options.value('c'), path, info)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintln(stdout, line); err != nil {
				return err
			}
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

// reportDeviceUsage answers for a device path, and reports whether it was one.
//
// One line, whether or not -s was given: there are no subdirectories under /dev to report
// separately, so the summary and the full walk have the same answer and printing it twice would
// only suggest otherwise.
func reportDeviceUsage(ctx context.Context, stdout io.Writer, view ProcessView, path string, human bool) (bool, error) {
	counted := false
	handled, err := walkDeviceRoot(view, path, func(string, fs.DirEntry) error {
		counted = true
		return ctx.Err()
	})
	if err != nil || !handled || !counted {
		return handled, err
	}
	return true, writeUsageLine(stdout, 0, path, human)
}
