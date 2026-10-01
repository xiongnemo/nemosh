package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Comparing directories, as busybox's diffdir does. Each directory's files are listed by the
// names under it, sorted, and the two lists walked together: a name in both is compared, and
// one in either alone is `Only in DIR: NAME`, NAME as it is under DIR, as busybox writes it.
// Without -r a directory under either is a name too, and two of them are `Common
// subdirectories: A and B`. With -r it is gone into instead, but not where the other side has
// no directory of the name: that is a difference, and nothing is said of it, as busybox says
// nothing. -N reads a file one side has not as empty, named /dev/null in the header, and goes
// into a directory the other side has not. -S FILE begins both lists at FILE.
//
// A directory and a file compare the file of the same name in the directory. diff refused a
// directory as a FILE it could not read, and -r was taken and ignored.

// diffOperands compares the operands: two files, a file and the same name in a directory, or
// two directories.
func (r diffRequest) diffOperands(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	view := ProcessViewFromContext(ctx)
	leftDir, rightDir := diffIsDirectory(view, r.left), diffIsDirectory(view, r.right)
	switch {
	case leftDir && rightDir:
		return r.diffDirectories(ctx, view, stdin, stdout)
	case leftDir:
		r.left = diffJoin(r.left, filepath.Base(filepath.FromSlash(r.right)))
	case rightDir:
		r.right = diffJoin(r.right, filepath.Base(filepath.FromSlash(r.left)))
	}
	return r.run(ctx, stdin, stdout)
}

func diffIsDirectory(view ProcessView, name string) bool {
	if name == "-" {
		return false
	}
	native, err := resolveHostPath(view, name)
	if err != nil {
		return false
	}
	info, err := os.Stat(native)
	return err == nil && info.IsDir()
}

// diffJoin is busybox's concat_path_file: one slash between, however many the directory ends in.
func diffJoin(directory, name string) string {
	return strings.TrimRight(directory, "/") + "/" + name
}

func (r diffRequest) diffDirectories(ctx context.Context, view ProcessView, stdin io.Reader, stdout io.Writer) error {
	roots := [2]string{r.left, r.right}
	var natives [2]string
	for side, root := range roots {
		native, err := resolveHostPath(view, root)
		if err != nil {
			return ExitStatusMessage(2, operandFailure(root, err))
		}
		natives[side] = native
	}
	differ := false
	var lists [2][]string
	for side := range 2 {
		lists[side] = r.diffListing(natives[side], natives[1-side], &differ)
		sort.Strings(lists[side])
		for len(lists[side]) > 0 && r.start != "" && lists[side][0] < r.start {
			lists[side] = lists[side][1:]
		}
	}
	for len(lists[0]) > 0 || len(lists[1]) > 0 {
		order := 1
		switch {
		case len(lists[1]) == 0:
			order = -1
		case len(lists[0]) > 0:
			order = strings.Compare(lists[0][0], lists[1][0])
		}
		side := 0
		if order > 0 {
			side = 1
		}
		name := lists[side][0]
		lists[side] = lists[side][1:]
		if order == 0 {
			lists[1] = lists[1][1:]
		}
		if order != 0 && !r.treatAbsentAsEmpty {
			fmt.Fprintf(stdout, "Only in %s: %s\n", roots[side], name)
			differ = true
			continue
		}
		pair := r
		pair.left, pair.right = diffJoin(roots[0], name), diffJoin(roots[1], name)
		pair.absent = [2]bool{order > 0, order < 0}
		err := pair.diffEntry(ctx, natives, name, stdin, stdout)
		if errors.Is(err, ErrExitFalse) {
			differ = true
		} else if err != nil {
			return err
		}
	}
	if differ {
		return ErrExitFalse
	}
	return nil
}

// diffEntry compares one name the two directories share, or under -N one of them has.
func (r diffRequest) diffEntry(ctx context.Context, natives [2]string, name string, stdin io.Reader, stdout io.Writer) error {
	var infos [2]os.FileInfo
	for side := range 2 {
		if !r.absent[side] {
			infos[side], _ = os.Stat(filepath.Join(natives[side], filepath.FromSlash(name)))
		}
	}
	// The side that has not the name is taken to be what the other is.
	for side := range 2 {
		if r.absent[side] {
			infos[side] = infos[1-side]
		}
	}
	names := [2]string{r.left, r.right}
	kinds := [2]string{diffKind(infos[0]), diffKind(infos[1])}
	switch {
	case kinds[0] == "directory" && kinds[1] == "directory":
		_, err := fmt.Fprintf(stdout, "Common subdirectories: %s and %s\n", names[0], names[1])
		return err
	case kinds[0] == "" || kinds[1] == "":
		side := 0
		if kinds[0] != "" {
			side = 1
		}
		_, err := fmt.Fprintf(stdout, "File %s is not a regular file or directory and was skipped\n", names[side])
		return err
	case kinds[0] != kinds[1]:
		_, err := fmt.Fprintf(stdout, "File %s is a %s while file %s is a %s\n", names[0], kinds[0], names[1], kinds[1])
		return err
	}
	return r.run(ctx, stdin, stdout)
}

func diffKind(info os.FileInfo) string {
	switch {
	case info == nil:
		return ""
	case info.IsDir():
		return "directory"
	case info.Mode().IsRegular():
		return "regular file"
	}
	return ""
}

// diffListing is the names of what is under a directory, slash-separated. Without -r a
// directory under it is a name; with -r it is gone into, but not where the other side has no
// directory of the name and -N is not given, which is a difference said nowhere.
func (r diffRequest) diffListing(root, other string, differ *bool) []string {
	var names []string
	var walk func(relative string)
	walk = func(relative string) {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			return
		}
		for _, entry := range entries {
			name := entry.Name()
			if relative != "" {
				name = relative + "/" + name
			}
			info, err := os.Stat(filepath.Join(root, filepath.FromSlash(name)))
			if err != nil || !info.IsDir() || !r.recursive {
				names = append(names, name)
				continue
			}
			if !r.treatAbsentAsEmpty {
				counterpart, err := os.Stat(filepath.Join(other, filepath.FromSlash(name)))
				if err != nil || !counterpart.IsDir() {
					*differ = true
					continue
				}
			}
			walk(name)
		}
	}
	walk("")
	return names
}
