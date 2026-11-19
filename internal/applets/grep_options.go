package applets

import (
	"errors"
	"fmt"
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
	// in busybox, GNU and POSIX; see compile. perl is -P, Perl's syntax as RE2 reads it.
	extended bool
	perl     bool
	// nullData is -z, lines ended by a NUL rather than a newline, read and written; byteOffset
	// is -b, each line's offset in its input before it. See grep_nul.go.
	nullData   bool
	byteOffset bool
	// decompress is zgrep's: what an input holds is read through its decompressor.
	decompress   bool
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
	// color is --color's, names the globs that choose what -r reads, and binary what a binary
	// file is read as; see grep_long.go and grep_binary.go.
	color  colorWhen
	names  grepNameFilters
	binary grepBinaryMode
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
	if f.perl && f.fixedString {
		return nil, errors.New("conflicting matchers specified")
	}
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
		case f.perl:
			if err := perlPatternRefusal(pattern); err != nil {
				return nil, err
			}
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
			into.extended, into.perl = false, false
		case 'P':
			into.perl = true
		case 'z':
			into.nullData = true
		case 'b':
			into.byteOffset = true
		case 'a':
			into.binary = grepBinaryText
		case 'I':
			into.binary = grepBinarySkip
		default:
			return invalidOption(flag)
		}
	}
	return nil
}

func parseMaxCount(value string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid max count: %s", value)
	}
	return parsed, nil
}
