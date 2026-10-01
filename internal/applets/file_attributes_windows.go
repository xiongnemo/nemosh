//go:build windows

package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// lsattr and chattr list and change a file's Windows attributes, not ext2's flags, as
// busybox-w32's do (e2fsprogs/lsattr.c, chattr.c and e2fs_lib.c, under
// ENABLE_PLATFORM_MINGW32). They are registered on Windows alone, as su is, since elsewhere the
// names belong to e2fsprogs.

// fileAttributeFlags are the attributes lsattr shows, in its column order, with the letter it
// shows for each and the name -l spells out. chattr can change the last six, from
// changeableAttributes on, which are all SetFileAttributes takes.
var fileAttributeFlags = []struct {
	value  uint32
	letter byte
	name   string
}{
	{windows.FILE_ATTRIBUTE_REPARSE_POINT, 'R', "Reparse_Point"},
	{windows.FILE_ATTRIBUTE_OFFLINE, 'o', "Offline"},
	{windows.FILE_ATTRIBUTE_ENCRYPTED, 'e', "Encrypted"},
	{windows.FILE_ATTRIBUTE_COMPRESSED, 'c', "Compressed"},
	{windows.FILE_ATTRIBUTE_SPARSE_FILE, 'S', "Sparse"},
	{windows.FILE_ATTRIBUTE_READONLY, 'r', "Read_Only"},
	{windows.FILE_ATTRIBUTE_HIDDEN, 'h', "Hidden"},
	{windows.FILE_ATTRIBUTE_SYSTEM, 's', "System"},
	{windows.FILE_ATTRIBUTE_ARCHIVE, 'a', "Archive"},
	{windows.FILE_ATTRIBUTE_TEMPORARY, 't', "Temporary"},
	{windows.FILE_ATTRIBUTE_NOT_CONTENT_INDEXED, 'n', "Not_Indexed"},
}

const changeableAttributes = 5

// ioReparseTagAppExecLink is IO_REPARSE_TAG_APPEXECLINK, which x/sys does not name: what an app
// execution alias such as WindowsApps\python.exe is.
const ioReparseTagAppExecLink = 0x8000001B

// fileEntry is what lsattr and chattr know of a file: its attributes, and what kind of reparse
// point it is, as busybox-w32 tells the four it knows apart (get_reparse_tag, win32/mingw.c).
type fileEntry struct {
	attributes uint32
	letter     byte
	name       string
	directory  bool
}

// readFileEntry is a file's entry, the file itself and not what a link names. A symbolic link, a
// junction and an app execution alias are links, as busybox-w32's lstat makes them, so neither
// applet goes down through one; a volume mounted on a folder is a directory. Go calls a
// junction a directory.
func readFileEntry(native string) (fileEntry, error) {
	info, err := os.Lstat(native)
	if err != nil {
		return fileEntry{}, err
	}
	entry := fileEntry{directory: info.IsDir()}
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		entry.attributes = data.FileAttributes
	}
	if entry.attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		return entry, nil
	}
	var found windows.Win32finddata
	name, err := windows.UTF16PtrFromString(native)
	if err != nil {
		return entry, nil
	}
	handle, err := windows.FindFirstFile(name, &found)
	if err != nil {
		return entry, nil
	}
	_ = windows.FindClose(handle)
	switch found.Reserved0 {
	case windows.IO_REPARSE_TAG_SYMLINK:
		entry.letter, entry.name, entry.directory = 'l', "Symbolic_Link", false
	case windows.IO_REPARSE_TAG_MOUNT_POINT:
		entry.letter, entry.name, entry.directory = 'j', "Junction", false
		if target, err := os.Readlink(native); err == nil && strings.HasPrefix(target, `\\?\Volume{`) {
			entry.letter, entry.name, entry.directory = 'm', "Mount_Point", true
		}
	case ioReparseTagAppExecLink:
		entry.letter, entry.name, entry.directory = 'A', "App_Exec_Link", false
	}
	return entry, nil
}

// short is the attributes as lsattr's eleven columns show them, a dash for each one unset.
func (e fileEntry) short() string {
	var out strings.Builder
	for index, flag := range fileAttributeFlags {
		switch {
		case index == 0 && e.letter != 0:
			out.WriteByte(e.letter)
		case e.attributes&flag.value != 0:
			out.WriteByte(flag.letter)
		default:
			out.WriteByte('-')
		}
	}
	return out.String()
}

// long is the attributes as -l spells them out, or --- when there are none.
func (e fileEntry) long() string {
	var names []string
	for index, flag := range fileAttributeFlags {
		if e.attributes&flag.value == 0 {
			continue
		}
		if index == 0 && e.name != "" {
			names = append(names, e.name)
			continue
		}
		names = append(names, flag.name)
	}
	if len(names) == 0 {
		return "---"
	}
	return strings.Join(names, ", ")
}

// directoryNames are what a directory holds, in the order Windows gives them, which is the order
// both applets visit them in, as busybox's readdir does. . and .. come first, as FindFirstFile
// gives them, except at the root of a volume, where there are none and busybox-w32 makes both
// up after the rest.
func directoryNames(native string) ([]string, error) {
	directory, err := os.Open(native)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	names, err := directory.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	if clean := filepath.Clean(native); filepath.Dir(clean) == clean {
		return append(names, ".", ".."), nil
	}
	return append([]string{".", ".."}, names...), nil
}

// lsattr is `lsattr [-Radl] [FILE]...`: the attributes of each FILE, or of what a directory
// holds. -a includes the names that start with a dot, . and .. among them; -d lists a
// directory itself; -R goes down into each directory it lists, under a heading of its name;
// -l spells the attributes out. With no FILE it lists the working directory.
type lsattrRequest struct {
	all, itself, long, recurse bool
	stdout, stderr             io.Writer
	failed                     bool
}

func newLsattrApplet() Applet {
	return simpleApplet{name: "lsattr", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletOptions(ctx, args, "Radl", "")
		if err != nil {
			return err
		}
		request := lsattrRequest{all: options.has('a'), itself: options.has('d'), long: options.has('l'),
			recurse: options.has('R'), stdout: stdout, stderr: stderr}
		if len(operands) == 0 {
			operands = []string{"."}
		}
		view := ProcessViewFromContext(ctx)
		for _, operand := range operands {
			native, err := resolveHostPath(view, operand)
			if err == nil {
				err = request.operand(ctx, native, operand)
			}
			if err != nil {
				request.report(cannotStat(operand, err))
			}
		}
		return request.status()
	}}
}

func (r *lsattrRequest) operand(ctx context.Context, native, shown string) error {
	entry, err := readFileEntry(native)
	if err != nil {
		return err
	}
	if entry.directory && !r.itself {
		r.directory(ctx, native, shown)
		return nil
	}
	r.list(shown, entry)
	return nil
}

func (r *lsattrRequest) directory(ctx context.Context, native, shown string) {
	names, err := directoryNames(native)
	if err != nil {
		r.report(cannotOpen(shown, err))
		return
	}
	for _, name := range names {
		if ctx.Err() != nil {
			return
		}
		// joinOperand is concat_path_file, which names what is under a FILE in lsattr's lines.
		path, nativePath := joinOperand(shown, name), filepath.Join(native, name)
		entry, err := readFileEntry(nativePath)
		if err != nil {
			r.report(cannotStat(path, err))
			continue
		}
		if name[0] == '.' && !r.all {
			continue
		}
		r.list(path, entry)
		if entry.directory && r.recurse && name != "." && name != ".." {
			fmt.Fprintf(r.stdout, "\n%s:\n", path)
			r.directory(ctx, nativePath, path)
			fmt.Fprintln(r.stdout)
		}
	}
}

func (r *lsattrRequest) list(shown string, entry fileEntry) {
	if r.long {
		fmt.Fprintf(r.stdout, "%-28s %s\n", shown, entry.long())
		return
	}
	fmt.Fprintf(r.stdout, "%s %s\n", entry.short(), shown)
}

// report says what could not be read and goes on, as busybox does. The status is 1 after one,
// where busybox's is 0 whatever happened: e2fsprogs' lsattr, whose options these are, says 1.
func (r *lsattrRequest) report(err error) {
	r.failed = true
	fmt.Fprintf(r.stderr, "lsattr: %v\n", err)
}

func (r *lsattrRequest) status() error {
	if r.failed {
		return ExitStatus(1)
	}
	return nil
}
