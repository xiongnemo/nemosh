package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// du is busybox's (coreutils/du.c): `du [-aHLdclsxhmk] [FILE]...`, FILE `.` when there is none.
// Each is walked in the order its directories list their entries, and a directory is printed
// after what is inside it with the sum of its own blocks and theirs, so `du | tail -1` is the
// total. -a prints files too, -s only each FILE, -d N nothing deeper than N, and -c a total at
// the end. Sizes are kilobytes, -m's megabytes, -b's bytes or -h's 4.0K and 1.5M, rounded as
// busybox rounds. A file with several links is counted once unless -l; -x keeps to FILE's
// filesystem; -H follows a link named as FILE and -L every one, a junction as well as a
// symbolic link, as busybox-w32 takes both for links. Of -h -k -m, of -H -L and of -s -d, the
// last given wins.
//
// It took -s and -h, printed a tree's directories in the reverse of the order it found them,
// and -h said 12K and 0.0K where busybox says 12.0K and 0.
func newDuApplet() Applet {
	return simpleApplet{name: "du", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		options, paths, err := parseAppletOptions(ctx, args, "aHkLsxlcbhm", "d")
		if err != nil {
			return err
		}
		view := ProcessViewFromContext(ctx)
		run := &duRun{ctx: ctx, stdout: stdout, stderr: stderr, all: options.has('a'), bytes: options.has('b'),
			oneFS: options.has('x'), countLinks: options.has('l'), maxDepth: math.MaxInt, unit: 1024,
			seen: map[fileID]bool{}}
		if _, posix := view.LookupEnv("POSIXLY_CORRECT"); posix {
			run.unit = 512
		}
		if run.bytes {
			run.unit = 1
		}
		switch options.last("hkm") {
		case 'h':
			run.unit = 0
		case 'k':
			run.unit = 1024
		case 'm':
			run.unit = 1024 * 1024
		}
		follow := 0
		switch options.last("HL") {
		case 'H':
			follow = 1
		case 'L':
			follow = math.MaxInt
		}
		switch options.last("sd") {
		case 's':
			run.maxDepth = 0
		case 'd':
			depth, err := strconv.ParseUint(options.value('d'), 10, 31)
			if err != nil {
				return fmt.Errorf("invalid number '%s'", options.value('d'))
			}
			run.maxDepth = int(depth)
		}
		if len(paths) == 0 {
			paths = []string{"."}
			// -H has no FILE to follow.
			if follow == 1 {
				follow = 0
			}
		}
		var total int64
		for _, path := range paths {
			if err := ctx.Err(); err != nil {
				return err
			}
			run.follow = follow
			// A device tree has no blocks: every entry is synthetic, so it is 0, which is also
			// what `du` on a real /dev reports.
			if handled, counted, err := deviceTreeUsage(ctx, view, path); err != nil {
				return err
			} else if handled {
				if counted {
					run.print(0, path)
				}
				continue
			}
			native, err := resolveHostPath(view, path)
			if err != nil {
				return err
			}
			total += run.walk(native, path)
		}
		if options.has('c') {
			run.print(total, "total")
		}
		if run.failed {
			return ExitStatus(1)
		}
		return run.ctx.Err()
	}}
}

type duRun struct {
	ctx                                   context.Context
	stdout, stderr                        io.Writer
	all, bytes, oneFS, countLinks, failed bool
	maxDepth, depth, follow               int
	// unit is what a size is shown in, bytes; 0 is -h's scaled form.
	unit   int64
	device uint64
	// seen are the directories and the files with several links met so far, each counted once.
	seen map[fileID]bool
	// inside are the directories being walked where links are followed, under -l, which counts
	// a directory by every way into it but the ones from inside itself.
	inside []fileID
}

// fileID names a file whichever of its links reaches it: its device and its index there.
type fileID struct{ device, index uint64 }

// walk is busybox's du(): what native holds, in 512-byte blocks or under -b bytes, printed when
// it is a directory, or a file under -a or named as FILE, no deeper than -d.
func (r *duRun) walk(native, shown string) int64 {
	if r.ctx.Err() != nil {
		return 0
	}
	info, err := os.Lstat(native)
	if err != nil {
		r.fail(operandFailure(shown, err))
		return 0
	}
	if r.oneFS {
		// The entry itself, before a link is followed, as busybox compares it.
		id, _, _ := fileIdentity(native, info)
		if r.depth == 0 {
			r.device = id.device
		} else if id.device != r.device {
			return 0
		}
	}
	if isLink(info) && r.follow > r.depth {
		if info, err = os.Stat(native); err != nil {
			r.fail(operandFailure(shown, err))
			return 0
		}
		// Past a link -H followed, everything is followed, as busybox converts -H to -L.
		if r.follow == 1 {
			r.follow = math.MaxInt
		}
	}
	counted, guard := r.counted(native, info)
	if counted {
		return 0
	}
	sum := r.size(native, info)
	if info.IsDir() {
		entries, err := readDirectoryInOrder(native)
		if err != nil {
			r.fail(cannotOpen(shown, err))
			return sum
		}
		if guard != nil {
			r.inside = append(r.inside, *guard)
		}
		for _, entry := range entries {
			r.depth++
			sum += r.walk(filepath.Join(native, entry.Name()), duSubpath(shown, entry.Name()))
			r.depth--
		}
		if guard != nil {
			r.inside = r.inside[:len(r.inside)-1]
		}
	} else if !r.all && r.depth != 0 {
		return sum
	}
	if r.depth <= r.maxDepth {
		r.print(sum, shown)
	}
	return sum
}

// counted is whether info was met before, unless -l: a directory, or a file with several links.
// busybox asks that of whatever has more than one link, which on Linux every directory has, and
// busybox-w32 measured the same: `du t/a t/a` prints t/a once. Knowing costs a handle open on
// Windows, as busybox-w32's stat makes one.
//
// Under -l a directory where links are followed is refused only from inside itself, so a loop
// of links ends where it begins; busybox walks one until the system says ELOOP. What is
// returned with it is the directory to add to inside while it is walked.
func (r *duRun) counted(native string, info os.FileInfo) (bool, *fileID) {
	guarded := info.IsDir() && r.follow > r.depth
	if r.countLinks && !guarded {
		return false, nil
	}
	id, links, ok := fileIdentity(native, info)
	switch {
	case !ok || !info.IsDir() && links < 2:
		return false, nil
	case r.countLinks:
		return slices.Contains(r.inside, id), &id
	case r.seen[id]:
		return true, nil
	}
	r.seen[id] = true
	return false, nil
}

func (r *duRun) size(native string, info os.FileInfo) int64 {
	if r.bytes {
		return entrySize(native, info)
	}
	allocated, ok := allocatedBytes(native, info)
	if !ok {
		allocated = entrySize(native, info)
	}
	return (allocated + 511) / 512
}

// print is busybox's print(): a fixed unit rounds a size up, as coreutils does, by half a unit
// before the division rounds it to the nearest.
func (r *duRun) print(size int64, name string) {
	blockSize := int64(512)
	if r.bytes {
		blockSize = 1
	}
	if r.unit != 0 {
		size += (r.unit - 1) / (512 * 2)
	}
	fmt.Fprintf(r.stdout, "%s\t%s\n", humanReadable(uint64(size), uint64(blockSize), uint64(r.unit)), name)
}

func (r *duRun) fail(err error) {
	fmt.Fprintf(r.stderr, "du: %v\n", err)
	r.failed = true
}

// isLink is whether du takes info for a link: a symbolic one, or a junction, which Go reports
// as irregular and busybox-w32 counts as a link.
func isLink(info os.FileInfo) bool {
	return info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0
}

// deviceTreeUsage is whether path is the shell's device tree, which it walks, and whether that
// held anything to count.
func deviceTreeUsage(ctx context.Context, view ProcessView, path string) (bool, bool, error) {
	counted := false
	handled, err := walkDeviceRoot(view, path, func(string, fs.DirEntry) error {
		counted = true
		return ctx.Err()
	})
	return handled, counted, err
}

// duSubpath is busybox's concat_subpath_file: name under dir, with no second slash.
func duSubpath(dir, name string) string {
	if strings.HasSuffix(dir, "/") {
		return dir + name
	}
	return dir + "/" + name
}

// readDirectoryInOrder is a directory's entries in the order it lists them, as readdir gives
// them to busybox; os.ReadDir sorts them.
func readDirectoryInOrder(native string) ([]os.DirEntry, error) {
	directory, err := os.Open(native)
	if err != nil {
		return nil, err
	}
	entries, err := directory.ReadDir(-1)
	return entries, errors.Join(err, directory.Close())
}
