package applets

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// xargs builds command lines from its input and runs them.
//
// It used to accept no options at all, which made the one form everybody actually
// types -- `find … -print0 | xargs -0 rm` -- fail on the first word. The options
// here are GNU's, measured:
//
//	$ printf 'a\nb\nc\n' | xargs -n2 echo          ->  a b / c
//	$ printf 'a\0b\0'    | xargs -0 echo           ->  a b
//	$ printf 'x\ny\n'    | xargs -I{} echo "[{}]"  ->  [x] / [y]
//	$ printf ''          | xargs -r echo empty     ->  nothing
//
// -0 is the one that matters most, because it is the only way a filename with a
// blank in it survives the trip. Splitting on whitespace -- the default, and all
// this ever did -- turns `My Documents` into two arguments, which is how
// `xargs rm` comes to delete the wrong thing.
type xargsApplet struct{}

func newXargsApplet() Applet { return xargsApplet{} }

func (xargsApplet) Name() string { return "xargs" }

func (xargsApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	options, command, err := parseAppletOptions(args, "0rt", "nI")
	if err != nil {
		return err
	}
	if len(command) == 0 {
		command = []string{"echo"}
	}
	items, err := readXargsItems(stdin, options.has('0'))
	if err != nil {
		return err
	}
	if len(items) == 0 && options.has('r') {
		// GNU's --no-run-if-empty. Without it, `xargs rm` on an empty list runs
		// `rm` with no operands and reports an error nobody asked for.
		return nil
	}
	batches, err := xargsBatches(options, command, items)
	if err != nil {
		return err
	}
	for _, batch := range batches {
		if options.has('t') {
			// GNU echoes the command to stderr before running it, which is what
			// makes -t usable for checking what a pipeline is about to do.
			fmt.Fprintln(stderr, strings.Join(batch, " "))
		}
		if err := runXargsBatch(ctx, batch, stdout, stderr); err != nil {
			return err
		}
	}
	return nil
}

// xargsBatches decides how the items are grouped into command lines.
func xargsBatches(options appletOptions, command, items []string) ([][]string, error) {
	if options.has('I') {
		// -I runs the command once per item, substituting wherever the
		// placeholder appears -- including inside a larger word, which is why
		// `-I{} echo "[{}]"` gives `[x]` rather than `[ x ]`.
		placeholder := options.value('I')
		if placeholder == "" {
			return nil, fmt.Errorf("replacement string cannot be empty")
		}
		batches := make([][]string, 0, len(items))
		for _, item := range items {
			batch := make([]string, len(command))
			for index, word := range command {
				batch[index] = strings.ReplaceAll(word, placeholder, item)
			}
			batches = append(batches, batch)
		}
		return batches, nil
	}
	perBatch := max(len(items), 1)
	if options.has('n') {
		parsed, err := strconv.Atoi(options.value('n'))
		if err != nil || parsed < 1 {
			return nil, fmt.Errorf("invalid number of arguments: %s", options.value('n'))
		}
		perBatch = parsed
	}
	if len(items) == 0 {
		// One run with no items, which is what xargs does by default: `xargs
		// echo` on empty input prints a blank line.
		return [][]string{command}, nil
	}
	var batches [][]string
	for start := 0; start < len(items); start += perBatch {
		end := min(start+perBatch, len(items))
		batch := append(append([]string(nil), command...), items[start:end]...)
		batches = append(batches, batch)
	}
	return batches, nil
}

// readXargsItems splits the input, on NUL for -0 and on whitespace otherwise.
func readXargsItems(stdin io.Reader, nulSeparated bool) ([]string, error) {
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	if nulSeparated {
		scanner.Split(scanNulSeparated)
	} else {
		scanner.Split(bufio.ScanWords)
	}
	var items []string
	for scanner.Scan() {
		items = append(items, scanner.Text())
	}
	return items, scanner.Err()
}

// scanNulSeparated is bufio.ScanLines with NUL for the separator. A trailing item
// without one is still an item, the way a final line without a newline is.
func scanNulSeparated(data []byte, atEOF bool) (int, []byte, error) {
	if index := bytes.IndexByte(data, 0); index >= 0 {
		return index + 1, data[:index], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func runXargsBatch(ctx context.Context, batch []string, stdout, stderr io.Writer) error {
	applet, ok := DefaultRegistry.Lookup(batch[0])
	if !ok {
		return commandNotFound(batch[0])
	}
	// Nothing is fed to the child's stdin: xargs consumed it to build the command
	// line, and handing over what is left would make `xargs cat` read its own
	// input.
	return applet.Run(ctx, batch[1:], bytes.NewReader(nil), stdout, stderr)
}
