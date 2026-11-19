package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/rivo/uniseg"
)

// file, file(1)'s, saying what a FILE is in libmagic's words as Debian's file 5.44 says them;
// busybox-w32 has none. The text classes and their line terminators are ascmagic's (file_text.go),
// a script is named by its #!, a Windows executable as the msdos magic describes it, and an ELF,
// the archives, the compressions, the images and the databases by their first words and the
// fields of their headers this reads (file_magic.go) -- libmagic says more of some, an ELF's
// linking and an SQLite file's version, which is left out rather than guessed at.
//
// -b leaves the names out, -i says the MIME type and charset, --mime-type the type, -L follows a
// link, and the names are aligned after their colons but with -N. A FILE that cannot be opened is
// said so of, `cannot open`, and the status stays 0, as file's does without -E.

const fileLongOptions = "brief=b mime=i dereference=L no-pad=N mime-type=\x01"

type fileOptions struct{ brief, mime, mimeType, follow, noPad bool }

func newFileApplet() Applet {
	return simpleApplet{name: "file", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletLongOptions(ctx, args, fileLongOptions, "biLNh\x01", "")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return errors.New("Usage: file [-bLNi] [--mime-type] FILE...")
		}
		settings := fileOptions{brief: options.has('b'), mime: options.has('i'), mimeType: options.has('\x01'), follow: options.has('L'), noPad: options.has('N')}
		widest := 0
		for _, operand := range operands {
			widest = max(widest, uniseg.StringWidth(operand))
		}
		view := ProcessViewFromContext(ctx)
		out := bufio.NewWriter(stdout)
		for _, operand := range operands {
			description, mime := describeFile(view, operand, settings.follow)
			if !strings.Contains(mime, "charset=") {
				// What is not text is binary to -i, an inode and an executable alike.
				mime += "; charset=binary"
			}
			answer := description
			switch {
			case settings.mimeType:
				answer, _, _ = strings.Cut(mime, ";")
			case settings.mime:
				answer = mime
			}
			if !settings.brief {
				padding := ""
				if !settings.noPad {
					padding = strings.Repeat(" ", widest-uniseg.StringWidth(operand))
				}
				answer = operand + ":" + padding + " " + answer
			}
			out.WriteString(answer + "\n")
		}
		return out.Flush()
	}}
}

// describeFile is what a FILE is, and its MIME type: by its kind first, and a regular file by what
// it holds.
func describeFile(view ProcessView, operand string, follow bool) (string, string) {
	host, err := resolveHostPath(view, operand)
	var info os.FileInfo
	if err == nil {
		if follow {
			info, err = os.Stat(host)
		} else {
			info, err = os.Lstat(host)
		}
	}
	if err != nil {
		return fmt.Sprintf("cannot open `%s' (%s)", operand, causeText(err)), "application/x-not-regular-file"
	}
	switch mode := info.Mode(); {
	case mode.IsDir():
		return "directory", "inode/directory"
	case isSymbolicLink(info):
		target, _ := os.Readlink(host)
		target = strings.ReplaceAll(target, `\`, "/")
		if _, err := os.Stat(host); err != nil {
			return "broken symbolic link to " + target, "inode/symlink"
		}
		return "symbolic link to " + target, "inode/symlink"
	case mode&fs.ModeCharDevice != 0:
		return "character special", "inode/chardevice"
	case mode&fs.ModeNamedPipe != 0:
		return "fifo (named pipe)", "inode/fifo"
	case info.Size() == 0:
		return "empty", "inode/x-empty"
	}
	file, err := os.Open(host)
	if err != nil {
		return fmt.Sprintf("cannot open `%s' (%s)", operand, causeText(err)), "application/x-not-regular-file"
	}
	defer file.Close()
	head := make([]byte, 64<<10)
	n, _ := io.ReadFull(file, head)
	head = head[:n]
	var tail []byte
	if size := info.Size(); size >= 4 {
		tail = make([]byte, 4)
		if _, err := file.ReadAt(tail, size-4); err != nil {
			tail = nil
		}
	}
	if description, mime, ok := describeMagic(head, tail); ok {
		return description, mime
	}
	return describeText(head)
}
