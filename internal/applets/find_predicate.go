package applets

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

// findTypeLetters are the entries this walk can classify. busybox also accepts
// b, s, and p; refusing them by name is better than answering as though a block
// device could never match.
// c joined f, d and l when /dev became listable: the shell now produces character devices, so
// `-type c` is a question with a real answer and refusing it would state a limitation that is no
// longer true. b, p and s stay out deliberately -- Windows has no block devices, and pipes and
// sockets are not reached as entries by any walk here, so a predicate for them would always answer
// "none" where the honest answer is "this shell cannot classify that".
const findTypeLetters = "fdlc"

// parsePredicate reads one test, a global option, or an action.
//
// A word this build does not implement is refused here, before the walk, which
// is the property TestFind_refusesAnUnsupportedExpression exists to hold: a
// caller piping into `xargs rm` must never receive paths from an expression that
// was never valid.
func (p *findParser) parsePredicate() (findNode, error) {
	operand := p.next()
	switch operand {
	case "-print":
		p.hasAction = true
		return findPrint{terminator: '\n'}, nil
	case "-print0":
		// The pairing with `xargs -0`, and the reason it exists: a NUL is the
		// one byte a path cannot contain.
		p.hasAction = true
		return findPrint{terminator: 0}, nil
	case "-name", "-iname":
		return p.namePredicate(operand)
	case "-path", "-ipath":
		return p.pathPredicate(operand)
	case "-type":
		return p.typePredicate(operand)
	case "-size":
		return p.sizePredicate(operand)
	case "-mtime":
		return p.mtimePredicate(operand)
	case "-newer":
		return p.newerPredicate(operand)
	case "-empty":
		return findEmpty{}, nil
	case "-maxdepth", "-mindepth":
		return p.depthOption(operand)
	}
	return nil, fmt.Errorf("unsupported expression: %s", operand)
}

func (p *findParser) namePredicate(operand string) (findNode, error) {
	pattern, err := p.argument(operand)
	if err != nil {
		return nil, err
	}
	if _, err := path.Match(pattern, ""); err != nil {
		return nil, fmt.Errorf("%s: bad pattern %q: %w", operand, pattern, err)
	}
	return findName{pattern: pattern, fold: strings.HasPrefix(operand, "-i")}, nil
}

func (p *findParser) pathPredicate(operand string) (findNode, error) {
	pattern, err := p.argument(operand)
	if err != nil {
		return nil, err
	}
	fold := strings.HasPrefix(operand, "-i")
	if fold {
		pattern = strings.ToLower(pattern)
	}
	matcher, err := compileFindPathPattern(pattern)
	if err != nil {
		return nil, fmt.Errorf("%s: bad pattern %q: %w", operand, pattern, err)
	}
	return findPath{matcher: matcher, fold: fold}, nil
}

func (p *findParser) typePredicate(operand string) (findNode, error) {
	letter, err := p.argument(operand)
	if err != nil {
		return nil, err
	}
	if len(letter) != 1 || !strings.Contains(findTypeLetters, letter) {
		return nil, fmt.Errorf("-type: unsupported type %q; this shell classifies only %s", letter, describeFindTypes())
	}
	return findType{letter: letter[0]}, nil
}

func (p *findParser) sizePredicate(operand string) (findNode, error) {
	value, err := p.argument(operand)
	if err != nil {
		return nil, err
	}
	comparison, digits := splitFindComparison(value)
	unit, digits, err := splitFindSizeUnit(digits)
	if err != nil {
		return nil, fmt.Errorf("-size: invalid size %q", value)
	}
	count, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || count < 0 {
		return nil, fmt.Errorf("-size: invalid size %q", value)
	}
	return findSize{comparison: comparison, count: count, unit: unit}, nil
}

func (p *findParser) mtimePredicate(operand string) (findNode, error) {
	value, err := p.argument(operand)
	if err != nil {
		return nil, err
	}
	comparison, digits := splitFindComparison(value)
	days, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || days < 0 {
		return nil, fmt.Errorf("invalid number %q", value)
	}
	return findMtime{comparison: comparison, days: days, now: time.Now()}, nil
}

// -newer is resolved at parse time: the operand is a path, so a missing one is
// an expression error rather than a per-entry failure repeated for every file.
func (p *findParser) newerPredicate(operand string) (findNode, error) {
	name, err := p.argument(operand)
	if err != nil {
		return nil, err
	}
	host, err := resolveHostPath(p.view, name)
	if err != nil {
		return nil, operandFailure(name, err)
	}
	info, err := os.Stat(host)
	if err != nil {
		return nil, operandFailure(name, err)
	}
	return findNewer{than: info.ModTime()}, nil
}

// -maxdepth and -mindepth are global options rather than tests: they bound the
// traversal wherever they appear and contribute a true term to the expression.
func (p *findParser) depthOption(operand string) (findNode, error) {
	value, err := p.argument(operand)
	if err != nil {
		return nil, err
	}
	depth, err := strconv.Atoi(value)
	if err != nil {
		return nil, fmt.Errorf("invalid number %q", value)
	}
	if depth < 0 {
		return nil, fmt.Errorf("%s: %d is not a non-negative depth", operand, depth)
	}
	if operand == "-maxdepth" {
		p.expression.maxDepth = depth
	} else {
		p.expression.minDepth = depth
	}
	return findTrue{}, nil
}

// splitFindComparison takes the leading + or - that turns an exact test into a
// greater-than or less-than one.
func splitFindComparison(value string) (byte, string) {
	if value == "" {
		return '=', value
	}
	if value[0] == '+' || value[0] == '-' {
		return value[0], value[1:]
	}
	return '=', value
}

// splitFindSizeUnit takes the trailing unit suffix. An absent suffix is 512-byte
// blocks, which is what POSIX makes the default.
func splitFindSizeUnit(value string) (int64, string, error) {
	if value == "" {
		return 0, "", fmt.Errorf("empty size")
	}
	units := map[byte]int64{'c': 1, 'w': 2, 'b': 512, 'k': 1024, 'M': 1024 * 1024, 'G': 1024 * 1024 * 1024}
	last := value[len(value)-1]
	if last >= '0' && last <= '9' {
		return 512, value, nil
	}
	unit, ok := units[last]
	if !ok {
		return 0, "", fmt.Errorf("unknown unit %q", string(last))
	}
	return unit, value[:len(value)-1], nil
}

func describeFindTypes() string {
	return "f (regular file), d (directory), l (symbolic link), c (character device)"
}
