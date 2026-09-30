package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func newLsApplet() Applet {
	return simpleApplet{name: "ls", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, paths, err := lsArgs(args, optionsPermute(ProcessViewFromContext(ctx)))
		if err != nil {
			return err
		}
		// `auto` is resolved against the stream actually being written to, not
		// at parse time: that is what makes `alias ls='ls --color=auto'` safe to
		// pipe, colouring a terminal and staying plain into grep.
		options.colored = colorEnabled(options.color, stdout)
		// A terminal shows a `?` for what it cannot, as busybox's ls turns -q on for one.
		options.printable = options.printable || stdoutIsTerminal(stdout)
		if len(paths) == 0 {
			paths = []string{"."}
		}
		listed, err := listOperands(ctx, stdout, ProcessViewFromContext(ctx), paths, options)
		if err != nil {
			return err
		}
		if !listed {
			return ExitStatus(1)
		}
		return nil
	}}
}

// listDirectory reads one directory and lays it out, descending afterwards when
// -R asked for it.
//
// headed says whether a `path:` header belongs above the entries. Only -R sets
// it, so a plain `ls dir` is unchanged.
func listDirectory(stdout io.Writer, target, display string, options lsOptions, headed bool) error {
	entries, err := os.ReadDir(target)
	if err != nil {
		return operandFailure(display, err)
	}
	items := make([]lsEntry, 0, len(entries)+2)
	if options.lsShowsDotEntries() {
		items = append(items, lsDotEntries(target)...)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !options.lsShowsHidden() && strings.HasPrefix(name, ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		item := lsEntry{name: name, info: info, path: filepath.Join(target, name)}
		// -L follows every link, and asks about what each points at.
		if followed, err := os.Stat(item.path); err == nil && options.follow == lsFollowAll && isSymbolicLink(info) {
			item.info, item.followed = followed, true
		}
		items = append(items, item)
	}
	if headed {
		if _, err := fmt.Fprintf(stdout, "%s:\n", display); err != nil {
			return err
		}
	}
	if err := writeLsEntries(stdout, items, options); err != nil {
		return err
	}
	if !options.recursive {
		return nil
	}
	return descendLsDirectories(stdout, display, items, options)
}

// writeLsEntries lays out a directory's worth of entries.
//
// Extracted so that a directory the shell provides and a directory on disk are laid out by the same
// code. Two copies of a layout is two layouts eventually, and `ls /dev` looking unlike `ls .` would
// suggest the entries were a different kind of thing than they are.
func writeLsEntries(stdout io.Writer, items []lsEntry, options lsOptions) error {
	recordLsFacts(items, options)
	sortLsEntries(items, options)
	// `total N` heads a directory listing and not a list of file operands, which is what
	// both references do, and it heads -s's as it heads -l's; see ls_facts.go.
	if options.long || options.blocks {
		if _, err := fmt.Fprintln(stdout, lsTotal(items, options)); err != nil {
			return err
		}
	}
	if !options.long {
		return writeLsNames(stdout, items, options, lsWantsColumns(options, stdout))
	}
	for _, item := range items {
		if err := printLsEntry(stdout, item, options); err != nil {
			return err
		}
	}
	return nil
}

// lsDotEntries are `.` and `..`, which -a lists and which this omitted entirely.
//
// os.ReadDir does not report them -- it is a directory *contents* call -- so they are added
// back. Both references list them, and `ls -la` without them cannot show a directory's own mode.
func lsDotEntries(target string) []lsEntry {
	var entries []lsEntry
	for name, path := range map[string]string{".": target, "..": filepath.Dir(target)} {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		entries = append(entries, lsEntry{name: name, info: info, path: path})
	}
	return entries
}

type lsEntry struct {
	name string
	info os.FileInfo
	// path is where the file really is. The long form has to ask the filesystem two more
	// questions -- the link count and the owner -- and neither can be asked of a name.
	path string
	// device is a directory the shell provides, /dev, whose entries come from the view.
	device bool
	// followed says info is what a link points at; record is the stat ls_facts.go makes, when
	// recorded says it was.
	followed bool
	record   statRecord
	recorded bool
}

func printLsEntry(stdout io.Writer, entry lsEntry, options lsOptions) error {
	line := lsDisplayName(entry, options)
	if options.long {
		// The whole line is busybox-w32's layout; see ls_long.go.
		line = formatLongEntry(entry, options)
	}
	_, err := fmt.Fprintln(stdout, lsEntryPrefix(entry, options)+line)
	return err
}

// listDeviceDirectory prints the contents of a directory the shell provides rather than the disk.
//
// Separate from listPath because there is no native path to walk: the entries come from the view,
// and each already knows what it is. The layout is the same one listPath uses, so `ls /dev` and
// `ls .` look alike -- a listing that laid itself out differently would suggest the entries were a
// different kind of thing than they are.
func listDeviceDirectory(stdout io.Writer, view ProcessView, target string, options lsOptions) error {
	entries, err := readDirProcessPath(view, target)
	if err != nil {
		return operandFailure(target, err)
	}
	items := make([]lsEntry, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !options.all && strings.HasPrefix(name, ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return operandFailure(target, err)
		}
		items = append(items, lsEntry{name: name, info: info, path: target + "/" + name})
	}
	return writeLsEntries(stdout, items, options)
}
