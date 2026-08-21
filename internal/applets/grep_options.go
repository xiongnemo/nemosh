package applets

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// The rest of grep's options, measured from GNU. Split into its own file for the
// size ceiling, not because they are a separate idea.
//
//	$ grep -o b g.txt          ->  b            just the match
//	$ grep -c o g.txt          ->  1            count, not lines
//	$ grep -l foo g.txt        ->  g.txt        the name only
//	$ grep -q foo g.txt        ->  nothing, status 0
//	$ printf 'foo\nfoobar\n' | grep -w foo   ->  foo
//	$ printf 'foo\nfoobar\n' | grep -x foo   ->  foo
//	$ printf 'a\na\n' | grep -m1 a           ->  a
//	$ printf 'a.c\nabc\n' | grep -F a.c      ->  a.c
//	$ grep -r hit rd           ->  rd/f.txt:hit
//
// -r is the one that was most missed, and the one whose output shape matters: the
// filename prefix appears because more than one file is being searched, which is
// the same rule that governs it for several named operands.

// grepFlags is the whole option surface, in one place so the parser and the
// matcher cannot disagree about what was asked for.
type grepFlags struct {
	ignoreCase   bool
	invert       bool
	lineNumber   bool
	recursive    bool
	filesOnly    bool
	countOnly    bool
	quiet        bool
	wordMatch    bool
	lineMatch    bool
	fixedString  bool
	onlyMatching bool
	noMessages   bool
	noFilename   bool
	withFilename bool
	maxCount     int
}

// compile turns the pattern into a regular expression, honouring -F, -w and -x.
//
// -F is not "escape the pattern and carry on": with -w or -x it still has to be
// anchored, so the escaping happens first and the anchors wrap it.
func (f grepFlags) compile(pattern string) (*regexp.Regexp, error) {
	if f.fixedString {
		pattern = regexp.QuoteMeta(pattern)
	}
	switch {
	case f.lineMatch:
		pattern = "^(?:" + pattern + ")$"
	case f.wordMatch:
		// GNU's definition: the match must not be adjacent to a word character
		// on either side. \b would be close but is wrong for a pattern that
		// starts or ends with a non-word character.
		pattern = `(?:\A|\W)(?:` + pattern + `)(?:\z|\W)`
	}
	if f.ignoreCase {
		pattern = "(?i)" + pattern
	}
	return regexp.Compile(pattern)
}

// parseGrepFlags reads the letters this file adds, leaving the long options and
// the operands to the caller.
func parseGrepFlags(flags string, into *grepFlags) error {
	for _, flag := range flags {
		switch flag {
		case 'i':
			into.ignoreCase = true
		case 'v':
			into.invert = true
		case 'n':
			into.lineNumber = true
		case 'r', 'R':
			into.recursive = true
		case 'l':
			into.filesOnly = true
		case 'c':
			into.countOnly = true
		case 'q':
			into.quiet = true
		case 'w':
			into.wordMatch = true
		case 'x':
			into.lineMatch = true
		case 'F':
			into.fixedString = true
		case 'o':
			into.onlyMatching = true
		case 's':
			into.noMessages = true
		case 'h':
			into.noFilename = true
		case 'H':
			into.withFilename = true
		case 'E':
			// GNU's default here already is extended: Go's regexp is RE2, which
			// has no basic mode. So -E is accepted as a no-op and -G would be a
			// lie -- see the support matrix.
		default:
			return fmt.Errorf("unsupported grep option: -%c", flag)
		}
	}
	return nil
}

// grepTarget is one thing to search: a reader, and the name to print beside a
// match.
type grepTarget struct {
	name   string
	opener func() (io.ReadCloser, error)
}

// grepTargets expands the operands into things to search, walking directories
// when -r asked for it.
//
// A directory named without -r is skipped with a diagnostic rather than read,
// because reading one yields bytes that are not lines and GNU says so too.
func grepTargets(ctx context.Context, flags grepFlags, paths []string, stderr io.Writer) ([]grepTarget, error) {
	view := ProcessViewFromContext(ctx)
	var targets []grepTarget
	for _, path := range paths {
		// Whether the operand is a directory is asked of the host, and a path the
		// host cannot answer for is not therefore unreadable: `/dev/clipboard`
		// resolves through the process view and has no host stat at all. So a
		// failure here means "not a directory", and the open below reports the
		// real error -- including a cancellation, which stat-ing first hid.
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
		targets = append(targets, namedTarget(ctx, view, path))
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
			return nil
		}
		// Named the way the operand was, so the output reads as a path relative to
		// what was asked about rather than an absolute one.
		relative, relErr := filepath.Rel(native, current)
		name := filepath.ToSlash(filepath.Join(shown, relative))
		if relErr != nil {
			name = filepath.ToSlash(current)
		}
		targets = append(targets, fileTarget(name, current))
		return nil
	})
	return targets, err
}

func namedTarget(ctx context.Context, view ProcessView, path string) grepTarget {
	return grepTarget{name: path, opener: func() (io.ReadCloser, error) {
		// A pattern is matched against characters, so a declared encoding is decoded.
		return openProcessTextInput(ctx, view, path)
	}}
}

func fileTarget(shown, native string) grepTarget {
	// The -r walk, which reaches files by host path rather than through the process view.
	return grepTarget{name: shown, opener: func() (io.ReadCloser, error) {
		file, err := os.Open(native)
		if err != nil {
			return nil, err
		}
		return decodedCloser{Reader: decodeTextInput(file), closer: file}, nil
	}}
}

// showNames decides whether a match carries its filename.
//
// GNU's rule, and the reason `grep -r` output looks different from `grep file`:
// the name appears when more than one file is being searched. -h suppresses it
// and -H forces it.
func (f grepFlags) showNames(targets int) bool {
	switch {
	case f.noFilename:
		return false
	case f.withFilename, f.recursive:
		// -r always names the file, even when the tree turned out to hold only
		// one: the operand was a directory, so which file matched is the thing
		// the reader does not know. Measured -- `grep -r hit rd` on a
		// single-file directory prints `rd/f.txt:hit`, and counting targets got
		// this wrong.
		return true
	}
	return targets > 1
}

func parseMaxCount(value string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid max count: %s", value)
	}
	return parsed, nil
}
