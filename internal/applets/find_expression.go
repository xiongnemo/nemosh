package applets

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
)

// findExpression is the parsed form of everything after the paths. Predicates
// combine with an implicit AND, which is the only operator POSIX requires and
// the only one implemented here.
type findExpression struct {
	predicates []findPredicate
}

type findPredicate struct {
	// name is the operand's spelling, kept so a diagnostic can name what the
	// user actually wrote rather than an internal enum.
	name    string
	pattern string
	letter  byte
}

// findTypeLetters are the entries this walk can classify. busybox also accepts
// b, c, s, and p; refusing them by name is better than answering as though a
// block device could never match.
// c joined f, d and l when /dev became listable: the shell now produces character devices, so
// `-type c` is a question with a real answer and refusing it would state a limitation that is no
// longer true. b, p and s stay out deliberately -- Windows has no block devices, and pipes and
// sockets are not reached as entries by any walk here, so a predicate for them would always answer
// "none" where the honest answer is "this shell cannot classify that".
const findTypeLetters = "fdlc"

// parseFindArguments splits paths from the expression and validates the whole
// expression before any walking starts. That ordering is the point: the old
// implementation walked first and reported `-name` as a missing file
// afterwards, so a caller had already received every path in the tree.
func parseFindArguments(args []string) ([]string, findExpression, error) {
	var paths []string
	index := 0
	for ; index < len(args); index++ {
		if strings.HasPrefix(args[index], "-") {
			break
		}
		paths = append(paths, args[index])
	}
	if len(paths) == 0 {
		paths = []string{"."}
	}

	var expression findExpression
	for ; index < len(args); index++ {
		operand := args[index]
		switch operand {
		case "-print":
			// The default action. Accepted so an explicit spelling works, and
			// otherwise a no-op, because there is no other action to suppress.
		case "-name":
			pattern, err := findOperandArgument(args, index, operand)
			if err != nil {
				return nil, expression, err
			}
			if _, err := path.Match(pattern, ""); err != nil {
				return nil, expression, fmt.Errorf("-name: bad pattern %q: %w", pattern, err)
			}
			expression.predicates = append(expression.predicates, findPredicate{name: operand, pattern: pattern})
			index++
		case "-type":
			letter, err := findOperandArgument(args, index, operand)
			if err != nil {
				return nil, expression, err
			}
			if len(letter) != 1 || !strings.Contains(findTypeLetters, letter) {
				return nil, expression, fmt.Errorf("-type: unsupported type %q; this shell classifies only %s", letter, describeFindTypes())
			}
			expression.predicates = append(expression.predicates, findPredicate{name: operand, letter: letter[0]})
			index++
		default:
			return nil, expression, fmt.Errorf("unsupported expression: %s", operand)
		}
	}
	return paths, expression, nil
}

func findOperandArgument(args []string, index int, operand string) (string, error) {
	if index+1 >= len(args) {
		return "", fmt.Errorf("%s: requires an argument", operand)
	}
	return args[index+1], nil
}

func describeFindTypes() string {
	return "f (regular file), d (directory), l (symbolic link), c (character device)"
}

// matches reports whether one walked entry satisfies every predicate.
func (e findExpression) matches(displayPath string, entry fs.DirEntry) bool {
	for _, predicate := range e.predicates {
		if !predicate.matches(displayPath, entry) {
			return false
		}
	}
	return true
}

func (p findPredicate) matches(displayPath string, entry fs.DirEntry) bool {
	switch p.name {
	case "-name":
		// The basename, never the path: busybox uses fnmatch without
		// FNM_PATHNAME, and a basename carries no separator for `*` to cross.
		matched, err := path.Match(p.pattern, path.Base(filepath.ToSlash(displayPath)))
		return err == nil && matched
	case "-type":
		return matchesFindType(p.letter, entry)
	}
	return true
}

func matchesFindType(letter byte, entry fs.DirEntry) bool {
	if entry == nil {
		return false
	}
	mode := entry.Type()
	switch letter {
	case 'f':
		return mode.IsRegular()
	case 'd':
		return mode.IsDir()
	case 'l':
		return mode&fs.ModeSymlink != 0
	case 'c':
		return mode&fs.ModeCharDevice != 0
	}
	return false
}
