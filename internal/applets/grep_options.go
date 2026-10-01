package applets

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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
// filename prefix appears because a directory was searched, as it does for several
// named operands; see showNames.

// grepFlags is the whole option surface, in one place so the parser and the
// matcher cannot disagree about what was asked for.
type grepFlags struct {
	ignoreCase  bool
	invert      bool
	lineNumber  bool
	recursive   bool
	filesOnly   bool
	countOnly   bool
	quiet       bool
	wordMatch   bool
	lineMatch   bool
	fixedString bool
	// extended is -E. Without it a pattern is a POSIX basic regular expression, as it is
	// in busybox, GNU and POSIX; see compile.
	extended     bool
	onlyMatching bool
	noMessages   bool
	noFilename   bool
	withFilename bool
	// maxCount is -m's count, and limited is whether -m was given at all, so -m0 selects
	// nothing rather than everything.
	maxCount int
	limited  bool
	// withoutMatch is -L: the files that did *not* match, which is -l inverted.
	withoutMatch bool
	// afterContext and beforeContext are -A and -B; -C sets both.
	afterContext  int
	beforeContext int
	// patterns holds every pattern, whether it came from the operand, from -e,
	// or from a -f file. patternsGiven records that -e or -f was used at all, so
	// that an empty -f file means "no pattern" rather than letting the first
	// operand become one.
	patterns      []string
	patternsGiven bool
}

// compile turns the patterns into one regular expression, honouring -F, -w and
// -x.
//
// Several patterns are an alternation, and each is escaped and anchored
// *separately* before being joined. Doing it the other way round is a real bug
// rather than a style choice: `-F -e a.c -e b` escaped as one string would
// escape the `|` this builds, and `-x -e a -e b` anchored as one alternation
// would give `^a|b$`, which matches any line containing b.
//
// -F is likewise not "escape and carry on": with -w or -x the escaped pattern
// still has to be anchored, so the escaping happens first and the anchors wrap
// it.
func (f grepFlags) compile() (*regexp.Regexp, error) {
	if len(f.patterns) == 0 {
		// No pattern matches nothing, which is what an empty -f file asks for.
		// A regexp that cannot match is clearer here than a nil to guard at every
		// use.
		return regexp.Compile(`$.^`)
	}
	parts := make([]string, 0, len(f.patterns))
	for _, pattern := range f.patterns {
		switch {
		case f.fixedString:
			pattern = regexp.QuoteMeta(pattern)
		case !f.extended:
			// A basic expression, translated as sed's is. Handed to Go as it was, `a+b`
			// needed a+ where every grep matches the three characters, `x|y` was an
			// alternation, `(` a syntax error, and `\(a\)` matched nothing.
			translated, err := translateBasicRegex(pattern)
			if err == nil {
				_, err = regexp.Compile(translated)
			}
			if err != nil {
				return nil, asBadRegex(pattern, err)
			}
			pattern = translated
		default:
			// Each is compiled alone first, so the one refused is the one named.
			if _, err := regexp.Compile(pattern); err != nil {
				return nil, asBadRegex(pattern, err)
			}
		}
		switch {
		case f.lineMatch:
			pattern = "^(?:" + pattern + ")$"
		case f.wordMatch:
			// GNU's definition: the match must not be adjacent to a word
			// character on either side. \b would be close but is wrong for a
			// pattern that starts or ends with a non-word character. The word is
			// the group named grepWord, which is what -o prints; see grepWords.
			pattern = `(?:\A|\W)(?P<` + grepWord + `>` + pattern + `)(?:\z|\W)`
		default:
			pattern = "(?:" + pattern + ")"
		}
		parts = append(parts, pattern)
	}
	joined := strings.Join(parts, "|")
	if f.ignoreCase {
		joined = "(?i)" + joined
	}
	return regexp.Compile(joined)
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
		case 'L':
			into.withoutMatch = true
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
			into.extended = true
		case 'G':
			into.extended = false
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
			targets = append(targets, grepTarget{name: path, opener: readerTarget(stdin)})
			continue
		}
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

func namedTarget(ctx context.Context, view ProcessView, path string) grepTarget {
	return grepTarget{name: path, opener: func() (io.ReadCloser, error) {
		// A pattern is matched against characters, so a declared encoding is decoded.
		return openProcessTextInput(ctx, view, path)
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

func parseMaxCount(value string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid max count: %s", value)
	}
	return parsed, nil
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
