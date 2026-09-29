package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

// sort is busybox's (coreutils/sort.c): `sort [-nghMVucszbrdfim] [-o FILE] [-k KEY]... [-t CHAR]
// [FILE]...`. Lines are compared by each -k KEY in turn, as the key's own letters say or else
// the ones given outside any key, and lines whose keys all tie are compared whole, byte by byte,
// unless -s keeps them in the order they came. -r reverses whichever comparison decided. -u keeps
// the first of each run whose keys tie, -c only says whether the input was in order, and -z reads
// and writes lines that end in NUL. -o FILE is opened once every input has been read, so `sort
// -o f f` sorts f in place. -S, -T and -m are taken and change nothing, as busybox takes them.
//
// It took -n -r -u -f -b and one -k of whole fields. Lines whose keys tied came out in no
// particular order, where busybox compares them whole; -n read an integer, so 1.5 and 1.25 were
// both 1; and a key joined its fields with a space, whatever -t had split them at.
func newSortApplet() Applet {
	return simpleApplet{name: "sort", runContext: runSort}
}

func runSort(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	spec, paths, err := parseSortArgs(ctx, args)
	if err != nil {
		return writeSortDiagnostic(stderr, "sort: "+err.Error())
	}
	lines, err := readSortInputs(ctx, ProcessViewFromContext(ctx), paths, spec.flags&sortZero != 0, stdin)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return writeSortDiagnostic(stderr, inputDiagnostic("sort", err))
	}
	if spec.flags&sortCheck != 0 {
		return spec.check(lines, stderr)
	}
	compare := func(left, right string) int { return spec.compare(left, right, true) }
	if spec.flags&sortStable != 0 {
		slices.SortStableFunc(lines, compare)
	} else {
		slices.SortFunc(lines, compare)
	}
	if spec.flags&sortUnique != 0 {
		lines = spec.unique(lines)
	}
	out := stdout
	if spec.output != "" {
		file, err := openSortOutput(ctx, spec.output)
		if err != nil {
			return writeSortDiagnostic(stderr, "sort: "+err.Error())
		}
		defer file.Close()
		out = file
	}
	return spec.write(lines, out)
}

// parseSortArgs reads busybox's options, `nghMVucszbrdfimS:T:o:k:*t:` with at most one -o and
// one -t. sort has no long option.
func parseSortArgs(ctx context.Context, args []string) (sortSpec, []string, error) {
	options, paths, err := parseAppletOptions(ctx, args, "nghMVucszbrdfim", "STokt")
	if err != nil {
		return sortSpec{}, nil, err
	}
	var spec sortSpec
	for index := range len(sortFlagLetters) {
		if options.has(sortFlagLetters[index]) {
			spec.flags |= 1 << index
		}
	}
	// -b outside a key strips a key's trailing blanks as well as its leading ones.
	if spec.flags&sortBlanks != 0 {
		spec.flags |= sortTrailingBlanks
	}
	switch {
	case len(options.all('o')) > 1:
		return sortSpec{}, nil, errors.New("multiple output files specified")
	case len(options.all('t')) > 1:
		return sortSpec{}, nil, errors.New("incompatible tabs")
	case options.has('t') && len(options.value('t')) != 1:
		return sortSpec{}, nil, errors.New("bad -t parameter")
	case options.has('t'):
		spec.separator = options.value('t')[0]
	}
	spec.output = options.value('o')
	for _, text := range options.all('k') {
		key, err := parseSortKey(text)
		if err != nil {
			return sortSpec{}, nil, err
		}
		spec.keys = append(spec.keys, key)
	}
	// With no key the line is one, compared as the options outside a key say.
	if len(spec.keys) == 0 {
		spec.keys = []sortKey{{field: [2]int{1, 0}}}
	}
	if !oneOrdering(spec.flags) || slices.ContainsFunc(spec.keys, func(key sortKey) bool { return !oneOrdering(key.flags) }) {
		return sortSpec{}, nil, errors.New("unknown sort type")
	}
	return spec, paths, nil
}

// oneOrdering is whether flags ask for at most one of -n -g -h -M -V, which busybox's
// compare_keys dies on the second of.
func oneOrdering(flags sortFlag) bool {
	orderings := flags & sortOrderings
	return orderings&(orderings-1) == 0
}

func readSortInputs(ctx context.Context, view ProcessView, paths []string, zero bool, stdin io.Reader) ([]string, error) {
	if len(paths) == 0 {
		return readSortLines(stdin, zero)
	}
	var lines []string
	for _, path := range paths {
		input, err := OpenProcessOperand(ctx, view, path, stdin)
		if err != nil {
			return nil, inputFailure(path, err)
		}
		fileLines, readErr := readSortLines(input, zero)
		closeErr := input.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return nil, inputFailure(path, err)
		}
		lines = append(lines, fileLines...)
	}
	return lines, nil
}

// readSortLines splits input at newlines, a CR before one dropped, or with -z at NULs.
func readSortLines(input io.Reader, zero bool) ([]string, error) {
	delimiter := byte('\n')
	if zero {
		delimiter = 0
	}
	reader := bufio.NewReader(input)
	var lines []string
	for {
		line, err := reader.ReadString(delimiter)
		if line != "" {
			line = strings.TrimSuffix(line, string(delimiter))
			if !zero {
				line = strings.TrimSuffix(line, "\r")
			}
			lines = append(lines, line)
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			return lines, nil
		}
		return nil, err
	}
}

func openSortOutput(ctx context.Context, path string) (*os.File, error) {
	native, err := resolveHostPath(ProcessViewFromContext(ctx), path)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(native, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return nil, cannotOpen(path, err)
	}
	return file, nil
}

func (spec sortSpec) write(lines []string, out io.Writer) error {
	terminator := "\n"
	if spec.flags&sortZero != 0 {
		terminator = "\x00"
	}
	writer := bufio.NewWriter(out)
	for _, line := range lines {
		writer.WriteString(line)
		writer.WriteString(terminator)
	}
	return writer.Flush()
}

// check is -c: nothing is written, and the first line out of order is named on stderr, as
// busybox's `Check line N` names it, with status 1. Under -u a line equal to the one before it is
// out of order too.
func (spec sortSpec) check(lines []string, stderr io.Writer) error {
	limit := 0
	if spec.flags&sortUnique != 0 {
		limit = -1
	}
	for index := 1; index < len(lines); index++ {
		if spec.compare(lines[index-1], lines[index], true) > limit {
			fmt.Fprintf(stderr, "Check line %d\n", index)
			return ExitStatus(1)
		}
	}
	return nil
}

// unique keeps the first of each run of lines whose keys tie, compared without the whole-line
// tie break, as busybox's -u compares them.
func (spec sortSpec) unique(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	kept := lines[:1]
	for _, line := range lines[1:] {
		if spec.compare(kept[len(kept)-1], line, false) != 0 {
			kept = append(kept, line)
		}
	}
	return kept
}

// sort is the one reader entitled to 2: sort_main sets `xfunc_error_retval = 2`
// before parsing options (coreutils/sort.c:468), so every bb_show_usage and
// every xfopen_stdin death after it carries that status.
func writeSortDiagnostic(stderr io.Writer, message string) error {
	if _, err := fmt.Fprintln(stderr, message); err != nil {
		return err
	}
	return ExitStatus(2)
}
