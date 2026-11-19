package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// tree, tree(1)'s (Steve Baker's tree.c, list.c and unix.c), which takes the name of Windows'
// tree.com: that one answers `tree -L 2` with `Too many parameters`, and still runs as tree.com.
// busybox-w32 has no tree.
//
// The lines are the ones tree draws in a UTF-8 locale, as Debian's and Ubuntu's are: a bar and two
// no-break spaces, then a space, for a level with more to come, and ├── and └── before a name;
// --charset=ascii draws |, |-- and `--. Names are in byte order, as ls here sorts them and C.UTF-8
// collates them, a name with a dot first left out but with -a, and after the tree a blank line
// and `N directories, M files`, the top directories not counted. -s, -h, -p, -u, -g and -D put
// their fields in brackets before a name, the top one's too, as tree's fillinfo has them. A PATH
// that cannot be opened is `PATH [error opening dir]`, and the status 2.

type treeLines struct{ vertical, branch, corner string }

var (
	treeUTF8  = treeLines{"│\xc2\xa0\xc2\xa0", "├──", "└──"}
	treeASCII = treeLines{"|  ", "|--", "`--"}
)

type treeOptions struct {
	all, dirsOnly, fullPath, noIndent, follow, classify bool
	perms, owner, group, size, human, date              bool
	reverse, unsorted, dirsFirst, noReport              bool
	level                                               int
	ignore, only                                        []string
	lines                                               treeLines
}

const treeLongOptions = "dirsfirst=\x01 noreport=\x02 charset=\x03"

func newTreeApplet() Applet {
	return simpleApplet{name: "tree", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletLongOptions(ctx, args, treeLongOptions, "adfilFpugshDrU\x01\x02", "LIP\x03")
		if err != nil {
			return err
		}
		view := ProcessViewFromContext(ctx)
		for _, operand := range operands {
			if strings.EqualFold(operand, "/F") || strings.EqualFold(operand, "/A") {
				if host, err := resolveHostPath(view, operand); err != nil || !isDirectory(host) {
					return fmt.Errorf("%s is tree.com's: this is tree(1), which lists files already and draws with --charset=ascii; Windows' tree runs as tree.com", operand)
				}
			}
		}
		settings := treeOptions{
			all: options.has('a'), dirsOnly: options.has('d'), fullPath: options.has('f'), noIndent: options.has('i'),
			follow: options.has('l'), classify: options.has('F'), perms: options.has('p'), owner: options.has('u'),
			group: options.has('g'), size: options.has('s') || options.has('h'), human: options.has('h'), date: options.has('D'),
			reverse: options.has('r'), unsorted: options.has('U'), dirsFirst: options.has('\x01'), noReport: options.has('\x02'),
			level: -1, ignore: options.all('I'), only: options.all('P'), lines: treeUTF8,
		}
		if options.has('L') {
			if settings.level, err = strconv.Atoi(options.value('L')); err != nil || settings.level < 1 {
				return errors.New("Invalid level, must be greater than 0.")
			}
		}
		if charset := options.value('\x03'); options.has('\x03') && !strings.EqualFold(charset, "utf-8") && !strings.EqualFold(charset, "utf8") {
			settings.lines = treeASCII
		}
		if len(operands) == 0 {
			operands = []string{"."}
		}
		run := &treeRun{options: settings, out: bufio.NewWriter(stdout), umask: processFileModeMask(view)}
		for _, operand := range operands {
			run.root(view, operand)
		}
		run.report()
		if err := run.out.Flush(); err != nil {
			return err
		}
		if run.failed {
			return ExitStatus(2)
		}
		return nil
	}}
}

type treeRun struct {
	options     treeOptions
	out         *bufio.Writer
	umask       uint32
	dirs, files int
	failed      bool
	// levels are tree's dirs[]: for each level, 1 while more entries follow there, 2 at the last.
	levels    []int
	ancestors []fileID
}

func (r *treeRun) root(view ProcessView, operand string) {
	host, err := resolveHostPath(view, operand)
	var info os.FileInfo
	if err == nil {
		info, err = os.Lstat(host)
	}
	if err != nil {
		r.failed = true
		fmt.Fprintf(r.out, "%s [error opening dir]\n", operand)
		return
	}
	r.levels = r.levels[:0]
	isDir := r.write(0, host, operand, operand, info)
	if !isDir {
		r.files++
		return
	}
	r.list(host, operand, 1)
}

// list writes what the directory host holds, a level below its name.
func (r *treeRun) list(host, display string, level int) {
	if r.options.level >= 0 && level > r.options.level {
		return
	}
	if r.options.follow {
		if info, err := os.Stat(host); err == nil {
			if id, _, ok := fileIdentity(host, info); ok {
				if slices.Contains(r.ancestors, id) {
					return
				}
				r.ancestors = append(r.ancestors, id)
				defer func() { r.ancestors = r.ancestors[:len(r.ancestors)-1] }()
			}
		}
	}
	entries := r.entries(host)
	for len(r.levels) <= level {
		r.levels = append(r.levels, 0)
	}
	for index, entry := range entries {
		r.levels[level] = 1
		if index == len(entries)-1 {
			r.levels[level] = 2
		}
		childHost := filepath.Join(host, entry.Name())
		childDisplay := strings.TrimSuffix(display, "/") + "/" + entry.Name()
		name := entry.Name()
		if r.options.fullPath {
			name = childDisplay
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if r.write(level, childHost, childDisplay, name, info) {
			r.dirs++
			if info.IsDir() || r.options.follow {
				r.list(childHost, childDisplay, level+1)
			}
		} else {
			r.files++
		}
	}
	r.levels[level] = 0
}

// entries are what host holds that the options list, in the order they are listed.
func (r *treeRun) entries(host string) []os.DirEntry {
	read, _ := os.ReadDir(host)
	var kept []os.DirEntry
	for _, entry := range read {
		name := entry.Name()
		switch {
		case !r.options.all && strings.HasPrefix(name, "."):
		case treeMatches(r.options.ignore, name):
		case !treeIsDir(filepath.Join(host, name), entry) && (r.options.dirsOnly || len(r.options.only) > 0 && !treeMatches(r.options.only, name)):
		default:
			kept = append(kept, entry)
		}
	}
	if r.options.unsorted {
		return kept
	}
	slices.SortStableFunc(kept, func(a, b os.DirEntry) int {
		if r.options.dirsFirst {
			aDir, bDir := treeIsDir(filepath.Join(host, a.Name()), a), treeIsDir(filepath.Join(host, b.Name()), b)
			if aDir != bDir {
				if aDir {
					return -1
				}
				return 1
			}
		}
		order := strings.Compare(a.Name(), b.Name())
		if r.options.reverse {
			order = -order
		}
		return order
	})
	return kept
}

// treeMatches is whether name matches any of the patterns, each of which may be several, by |.
func treeMatches(patterns []string, name string) bool {
	for _, pattern := range patterns {
		for _, one := range strings.Split(pattern, "|") {
			if matched, _ := path.Match(one, name); matched {
				return true
			}
		}
	}
	return false
}

// treeIsDir is whether an entry is listed as a directory: one, or a link to one.
func treeIsDir(host string, entry os.DirEntry) bool {
	if entry.IsDir() {
		return true
	}
	if entry.Type()&os.ModeSymlink != 0 || entry.Type()&os.ModeIrregular != 0 {
		return isDirectory(host)
	}
	return false
}

func isDirectory(host string) bool {
	info, err := os.Stat(host)
	return err == nil && info.IsDir()
}
