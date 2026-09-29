package applets

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"strings"
)

// stat is busybox's (coreutils/stat.c): what the filesystem knows of each FILE, in busybox's
// layout, the terse one of -t, or a -c FORMAT, of a link itself unless -L follows it; and with
// -f, of the filesystem the FILE is on.
//
// It took only -c, with six specifiers, and refused the default layout as mostly fields Windows
// has not got. busybox-w32 has every one of them (win32/mingw.c:730-846): the volume's serial
// number is the device, the file's index on it the inode, and an owner is mapped to a uid, the
// mode made up from the attributes. So they are printed as it prints them.
func newStatApplet() Applet {
	return simpleApplet{name: "stat", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		options, paths, err := parseAppletOptions(ctx, args, "tLf", "c")
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			return missingOperand()
		}
		view := ProcessViewFromContext(ctx)
		failed := false
		for _, path := range paths {
			if err := ctx.Err(); err != nil {
				return err
			}
			var out strings.Builder
			if err := statOne(&out, view, path, options); err != nil {
				fmt.Fprintf(stderr, "stat: %v\n", err)
				failed = true
				continue
			}
			if _, err := io.WriteString(stdout, out.String()); err != nil {
				return err
			}
		}
		if failed {
			return ExitStatus(1)
		}
		return nil
	}}
}

// statOne is busybox's do_stat, or do_statfs with -f: the FILE in its format, or the default
// one when -c gave none.
func statOne(out *strings.Builder, view ProcessView, path string, options appletOptions) error {
	format, custom := options.value('c'), options.has('c')
	if options.has('f') {
		record, err := statFilesystemOf(view, path)
		if err != nil {
			return quotedError{action: "read file system information for", operand: path, err: err}
		}
		if !custom {
			format = statfsFormat(options.has('t'))
		}
		statPrint(out, format, func(spec cSpec, verb byte) string { return record.conversion(spec, verb, path) })
		return nil
	}
	record, err := statOperand(view, path, options.has('L'))
	if err != nil {
		return cannotStat(path, err)
	}
	if !custom {
		format = record.defaultFormat(options.has('t'))
	}
	statPrint(out, format, func(spec cSpec, verb byte) string { return record.conversion(spec, verb, path) })
	return nil
}

// statPrint is busybox's print_it (coreutils/stat.c:410-464): the format with each % and the
// flags, width and precision after it put to the conversion its letter names, and a newline at
// the end. %% is a %, whatever stands between; a % that ends the format is printed as it is,
// and then no newline is.
func statPrint(out *strings.Builder, format string, convert func(cSpec, byte) string) {
	for {
		at := strings.IndexByte(format, '%')
		if at < 0 {
			out.WriteString(format)
			out.WriteByte('\n')
			return
		}
		out.WriteString(format[:at])
		end := at + 1
		for end < len(format) && strings.IndexByte("#-+.I 0123456789", format[end]) >= 0 {
			end++
		}
		switch {
		case end == len(format):
			out.WriteByte('%')
			return
		case format[end] == '%':
			out.WriteByte('%')
		default:
			// glibc's I asks for the locale's digits, which in C's are these.
			out.WriteString(convert(parseCSpec(strings.ReplaceAll(format[at+1:end], "I", "")), format[end]))
		}
		format = format[end+1:]
	}
}

// statOperand is the FILE as busybox's lstat has it, or its stat with follow: the shell's own
// answer for a device of its own, and the host's for the rest.
func statOperand(view ProcessView, path string, follow bool) (statRecord, error) {
	info, err := statDeviceOperand(view, path)
	if err != nil {
		return statRecord{}, err
	}
	if info != nil {
		return deviceStatRecord(info), nil
	}
	native, err := resolveHostPath(view, path)
	if err != nil {
		return statRecord{}, err
	}
	return hostStatRecord(native, follow, processFileModeMask(view))
}

// deviceStatRecord is a device of the shell's as busybox-w32 makes up its own
// (win32/mingw.c:413-421, 791): on no volume, owned by whoever asks, and dated the epoch.
func deviceStatRecord(info fs.FileInfo) statRecord {
	record := statRecord{mode: statCharacter | uint32(info.Mode().Perm()), blockSize: 4096, links: 1}
	if info.IsDir() {
		record.mode, record.links = statDirectory|uint32(info.Mode().Perm()), 2
	}
	record.uid, record.gid, record.owner, record.group = currentStatOwner()
	record.access, record.modify, record.change = info.ModTime(), info.ModTime(), info.ModTime()
	return record
}

func statFilesystemOf(view ProcessView, path string) (statfsRecord, error) {
	native, err := resolveHostPath(view, path)
	if err != nil {
		return statfsRecord{}, err
	}
	return hostStatfs(native)
}
