package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
)

func newHeadApplet() Applet {
	return simpleApplet{name: "head", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		count, bytes, paths, err := countArgs("head", args, 10, true)
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			return copyHeadOf(stdout, stdin, count, bytes)
		}
		view := ProcessViewFromContext(ctx)
		for _, path := range paths {
			file, err := OpenProcessInput(ctx, view, path)
			if err != nil {
				return operandFailure(path, err)
			}
			copyErr := copyHeadOf(stdout, file, count, bytes)
			closeErr := file.Close()
			if err := errors.Join(copyErr, closeErr); err != nil {
				return err
			}
		}
		return nil
	}}
}

// copyHeadOf takes the first count lines, or bytes under -c.
//
// Bytes are copied rather than scanned, so `head -c 512` on a binary yields the
// first 512 bytes of it and not the first 512 bytes of something a line scanner
// thought it saw.
func copyHeadOf(stdout io.Writer, input io.Reader, count int, bytes bool) error {
	if !bytes {
		return copyHead(stdout, input, count)
	}
	_, err := io.CopyN(stdout, input, int64(count))
	if errors.Is(err, io.EOF) {
		// Fewer bytes than asked for is not a failure: it is a short file.
		return nil
	}
	return err
}

func copyHead(stdout io.Writer, input io.Reader, count int) error {
	scanner := bufio.NewScanner(input)
	for i := 0; i < count && scanner.Scan(); i++ {
		if _, err := fmt.Fprintln(stdout, scanner.Text()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func newTailApplet() Applet {
	return simpleApplet{name: "tail", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		count, paths, err := lineCountArgs("tail", args, 10)
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			return copyTail(stdout, stdin, count)
		}
		view := ProcessViewFromContext(ctx)
		for _, path := range paths {
			file, err := OpenProcessInput(ctx, view, path)
			if err != nil {
				return cannotOpen(path, err)
			}
			copyErr := copyTail(stdout, file, count)
			closeErr := file.Close()
			if err := errors.Join(copyErr, closeErr); err != nil {
				return err
			}
		}
		return nil
	}}
}

func copyTail(stdout io.Writer, input io.Reader, count int) error {
	scanner := bufio.NewScanner(input)
	lines := make([]string, 0, count)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > count {
			lines = lines[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return err
		}
	}
	return nil
}

// lineCountArgs consumes `-n COUNT` and then refuses anything else that looks
// like an option, rather than letting it reach the file opener and be reported
// as a missing file.
func lineCountArgs(applet string, args []string, defaultCount int) (int, []string, error) {
	count, _, paths, err := countArgs(applet, args, defaultCount, false)
	return count, paths, err
}

// countArgs reads -n, and for head also -c, which counts bytes rather than
// lines.
//
// -c is what a script reaches for to take the first kilobyte of something, and
// the two are exclusive by nature: the last one given wins, as it does in
// busybox, because both write into the same count.
func countArgs(applet string, args []string, defaultCount int, allowBytes bool) (count int, bytes bool, paths []string, err error) {
	count = defaultCount
	supported := []string{"-n"}
	if allowBytes {
		supported = append(supported, "-c")
	}
	for len(args) > 0 && (args[0] == "-n" || (allowBytes && args[0] == "-c")) {
		flag := args[0]
		if len(args) < 2 {
			return 0, false, nil, fmt.Errorf("%s: requires a count", flag)
		}
		parsed, parseErr := strconv.Atoi(args[1])
		if parseErr != nil || parsed < 0 {
			return 0, false, nil, fmt.Errorf("invalid count: %s", args[1])
		}
		count, bytes, args = parsed, flag == "-c", args[2:]
	}
	paths, err = streamOperands(applet, args, supported...)
	if err != nil {
		return 0, false, nil, err
	}
	return count, bytes, paths, nil
}
