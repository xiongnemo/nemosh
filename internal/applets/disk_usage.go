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
// **The totals are derived from apparent sizes**, rounded up to a block, and will
// not always equal GNU's. GNU reports what the filesystem allocated -- st_blocks
// -- and that differs from the file's length in both directions: a 3000-byte file
// occupies 4096 on NTFS, while a 3-byte one may occupy nothing at all because it
// fits inside the MFT record. Measured on a two-file tree, GNU said 5 and the
// apparent-size arithmetic says 6.
//
// Go's os.FileInfo does not carry allocation size portably, so the choice is
// between a number derived from what is knowable and a platform-specific syscall
// per file. This is the first, and says so, because a `du` that silently means
// something slightly different from the `du` in a script is worse than one that
// is documented to mean apparent size. GNU spells this `--apparent-size`.
const duBlock = 1024

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
		blocks := (info.Size() + duBlock - 1) / duBlock
		if entry.IsDir() {
			// A directory's own entry counts as one block, which is what makes an
			// empty tree report 1 rather than 0 -- and what makes the totals here
			// close to GNU's rather than merely the sum of file sizes.
			blocks = 1
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
			if value < 10 && value != float64(int64(value)) {
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
