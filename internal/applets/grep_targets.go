package applets

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// What grep searches: its operands, a directory walked under -r, a device skipped. Out of
// grep_options.go for the size ceiling.

// grepTarget is one thing to search: a reader, and the name to print beside a
// match.
type grepTarget struct {
	name   string
	opener func() (io.ReadCloser, error)
	// walked is a file -r found in a directory operand, rather than one named.
	walked bool
}

// grepTargets expands the operands into things to search, walking directories
// when -r asked for it.
//
// A directory named without -r is skipped with a diagnostic rather than read,
// because reading one yields bytes that are not lines and GNU says so too.
func grepTargets(ctx context.Context, flags grepFlags, paths []string, stdin io.Reader, stderr io.Writer) ([]grepTarget, error) {
	view := ProcessViewFromContext(ctx)
	var targets []grepTarget
	for _, path := range paths {
		// Whether the operand is a directory is asked of the host, and a path the
		// host cannot answer for is not therefore unreadable: `/dev/clipboard`
		// resolves through the process view and has no host stat at all. So a
		// failure here means "not a directory", and the open below reports the
		// real error -- including a cancellation, which stat-ing first hid.
		// A device directory is walked from the table, and every device in it is
		// *skipped* rather than read. That skip is the whole point: /dev/zero returns
		// bytes for ever, so a grep that read it would never return. GNU grep skips
		// devices when recursing for exactly this reason -- and only when recursing, so
		// a device named directly is still read.
		if handled, err := collectDeviceTargets(view, path, flags.recursive, &targets); err != nil {
			return nil, err
		} else if handled {
			continue
		}
		if native, err := resolveHostPath(view, path); err == nil {
			if info, statErr := os.Stat(native); statErr == nil && info.IsDir() && flags.recursive {
				walked, walkErr := walkGrepTargets(ctx, flags, path, filepath.Clean(native), stderr)
				if walkErr != nil {
					return nil, walkErr
				}
				targets = append(targets, walked...)
				continue
			}
		}
		// A directory named *without* -r stays an ordinary target, so the open
		// reports it as the error it is. GNU prints `Is a directory` and exits 2;
		// warning and carrying on would have turned that into a success, and a
		// test already pinned the stricter answer.
		// A lone `-` is the stdin, decoded the same way a named file is:
		// `grep -r pattern . | grep -v x -` is the same question either way.
		if path == "-" {
			targets = append(targets, grepTarget{name: path, opener: readerTarget(stdin, flags.decompress)})
			continue
		}
		targets = append(targets, namedTarget(ctx, view, path, flags.decompress))
	}
	return targets, nil
}

// walkGrepTargets collects every file under a directory operand.
func walkGrepTargets(ctx context.Context, flags grepFlags, shown, native string, stderr io.Writer) ([]grepTarget, error) {
	var targets []grepTarget
	err := filepath.WalkDir(native, func(current string, entry fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			if !flags.noMessages {
				fmt.Fprintf(stderr, "grep: %s: %v\n", filepath.ToSlash(current), err)
			}
			return nil
		}
		if entry.IsDir() {
			// --exclude-dir names directories the walk does not go into; the operand itself
			// was asked for by name and is always gone into, as GNU has it.
			if current != native && !flags.names.takesDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !flags.names.takesFile(entry.Name()) {
			return nil
		}
		// Named the way the operand was, so the output reads as a path relative to
		// what was asked about rather than an absolute one: the operand as it was
		// written, then a slash unless it ends in one, as busybox's concat_path_file
		// joins them. `grep -r x .` names ./t/f; the join was cleaned, and said t/f.
		name := filepath.ToSlash(current)
		if relative, relErr := filepath.Rel(native, current); relErr == nil {
			name = filepath.ToSlash(shown)
			if !strings.HasSuffix(name, "/") {
				name += "/"
			}
			name += filepath.ToSlash(relative)
		}
		targets = append(targets, fileTarget(name, current))
		return nil
	})
	return targets, err
}

func namedTarget(ctx context.Context, view ProcessView, path string, decompress bool) grepTarget {
	return grepTarget{name: path, opener: func() (io.ReadCloser, error) {
		// A pattern is matched against characters, so a declared encoding is decoded, under
		// zgrep once the data is decompressed.
		if !decompress {
			return openProcessTextInput(ctx, view, path)
		}
		file, err := OpenProcessInput(ctx, view, path)
		if err != nil {
			return nil, err
		}
		return grepOpened(path, file, file, true)
	}}
}

func fileTarget(shown, native string) grepTarget {
	// The -r walk, which reaches files by host path rather than through the process view.
	return grepTarget{name: shown, walked: true, opener: func() (io.ReadCloser, error) {
		file, err := os.Open(native)
		if err != nil {
			return nil, err
		}
		return decodedCloser{Reader: decodeTextInput(file), closer: file}, nil
	}}
}

// showNames decides whether a match carries its filename.
//
// busybox's rule, and the reason `grep -r` output looks different from `grep file`:
// the name appears when more than one FILE was named, or when -r went into a
// directory, as busybox's grep_dir sets -H. -h suppresses it and -H forces it.
//
// A directory walked names its files even when it held only one: which file
// matched is the thing the reader does not know. Measured -- `grep -r hit rd` on
// a single-file directory prints `rd/f.txt:hit`. -r alone named them too, and
// `grep -r x file`, one file named, is `x` there and not `file:x`.
func (f grepFlags) showNames(operands int, targets []grepTarget) bool {
	switch {
	case f.noFilename:
		return false
	case f.withFilename, operands > 1:
		return true
	}
	return slices.ContainsFunc(targets, func(target grepTarget) bool { return target.walked })
}

// collectDeviceTargets handles an operand under /dev, and reports whether it did.
//
// Recursing over the directory collects nothing: every entry is a device, and reading one is what
// this exists to avoid. `grep -r x /dev` therefore finds no matches and says nothing, which is the
// honest report for a set of files it declined to read -- and is what GNU grep does, silently, for
// the same reason.
//
// Without -r the path is left to the ordinary opener, so `grep x /dev/clipboard` still reads the
// clipboard. The skip is about traversal, not about devices being unreadable.
func collectDeviceTargets(view ProcessView, path string, recursive bool, targets *[]grepTarget) (bool, error) {
	info, err := statDeviceOperand(view, path)
	if err != nil || info == nil {
		return false, err
	}
	if !info.IsDir() {
		// A device named directly: not handled here, so the ordinary path opens it.
		return false, nil
	}
	if !recursive {
		// A directory without -r is an error the opener reports, exactly as it is for a
		// directory on disk.
		return false, nil
	}
	return true, nil
}
