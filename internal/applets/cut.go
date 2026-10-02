package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"regexp/syntax"
)

// cut is busybox's (coreutils/cut.c): `cut {-b|-c LIST | -f|-F LIST [-d SEP] [-s]} [-D]
// [-O SEP] [FILE]...`, with --output-delimiter for -O, and -n taken and ignored. -b and -c cut
// bytes, as busybox's -c does. -f cuts fields at each -d, a tab unless given, and -F at each
// match of -d taken as an extended regular expression, a run of blanks unless given. What is
// printed is joined by -O: -d for -f and a blank for -F unless given, and for -b and -c nothing
// unless given, put only between ranges that do not touch. -s drops a line with no delimiter in
// it. The list is sorted unless -D, which keeps it as given and counts each range's fields from
// the start of the line. A -d of a newline cuts lines rather than fields.
//
// It took -b -c -f -d -s. -d's first character is the delimiter, and the whole of it what goes
// between the fields, as busybox has it.
func newCutApplet() Applet {
	return cutApplet{}
}

type cutApplet struct{}

func (cutApplet) Name() string {
	return "cut"
}

func (cutApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	spec, operands, err := parseCutArgs(ctx, args)
	if err != nil {
		return writeCutDiagnostic(stderr, "cut: "+err.Error())
	}
	// runCutInputs reports each unreadable operand as it goes, so what comes back is already
	// either a context error or the final exit status.
	return runCutInputs(ctx, ProcessViewFromContext(ctx), spec, operands, stdin, stdout, stderr)
}

// cutSpec is what cut's options asked for.
type cutSpec struct {
	// fields is -f or -F, rather than -b or -c; lines is -f with a -d of a newline.
	fields, lines    bool
	suppress, noSort bool
	// delim is -f's delimiter and pattern -F's.
	delim   byte
	pattern *regexp.Regexp
	odelim  string
	ranges  []cutRange
}

func parseCutArgs(ctx context.Context, args []string) (cutSpec, []string, error) {
	options, operands, err := parseAppletLongOptions(ctx, args, map[string]string{"output-delimiter": "O"}, "sDn", "bcfFdO")
	if err != nil {
		return cutSpec{}, nil, err
	}
	var list byte
	for _, letter := range []byte("bcfF") {
		if !options.has(letter) {
			continue
		}
		if list != 0 {
			return cutSpec{}, nil, errors.New("options -b, -c, -f and -F are mutually exclusive")
		}
		list = letter
	}
	if list == 0 {
		return cutSpec{}, nil, errors.New("expected a list of bytes, characters, or fields")
	}
	spec := cutSpec{fields: list == 'f' || list == 'F', suppress: options.has('s'), noSort: options.has('D')}
	delim, hasDelim := options.value('d'), options.has('d')
	if !spec.fields {
		if spec.suppress {
			return cutSpec{}, nil, errors.New("-s requires -f or -F")
		}
		if hasDelim {
			return cutSpec{}, nil, errors.New("-d DELIM requires -f or -F")
		}
	}
	switch {
	case options.has('O'):
		spec.odelim = options.value('O')
	case list == 'F':
		spec.odelim = " "
	case hasDelim:
		spec.odelim = delim
	case spec.fields:
		spec.odelim = "\t"
	}
	switch {
	case list == 'F':
		if !hasDelim {
			delim = "[[:space:]]+"
		}
		if spec.pattern, err = compileCutPattern(delim); err != nil {
			return cutSpec{}, nil, err
		}
	case !hasDelim:
		spec.delim = '\t'
	case delim != "":
		spec.delim = delim[0]
		spec.lines = spec.delim == '\n'
	}
	if spec.ranges, err = parseCutList(options.value(list), spec.noSort); err != nil {
		return cutSpec{}, nil, err
	}
	return spec, operands, nil
}

// compileCutPattern is -F's delimiter as busybox's regcomp takes it, extended, and as its
// regexec searches with REG_NOTBOL and REG_NOTEOL: an anchor matches nowhere, each search
// starting inside the line. Of the matches that begin first, the longest is taken, as POSIX's.
func compileCutPattern(pattern string) (*regexp.Regexp, error) {
	tree, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, fmt.Errorf("bad regex '%s': %v", pattern, err)
	}
	neverAnchored(tree)
	compiled, err := regexp.Compile(tree.String())
	if err != nil {
		return nil, fmt.Errorf("bad regex '%s': %v", pattern, err)
	}
	compiled.Longest()
	return compiled, nil
}

func neverAnchored(tree *syntax.Regexp) {
	switch tree.Op {
	case syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText:
		*tree = syntax.Regexp{Op: syntax.OpNoMatch}
		return
	}
	for _, sub := range tree.Sub {
		neverAnchored(sub)
	}
}

// cut never raises xfunc_error_retval, so both its usage deaths and its
// per-operand `retval = EXIT_FAILURE` land on the libbb default of 1
// (libbb/default_error_retval.c:16). Only sort asks for 2.
func writeCutDiagnostic(stderr io.Writer, message string) error {
	if _, err := fmt.Fprintln(stderr, message); err != nil {
		return err
	}
	return ExitStatus(1)
}
