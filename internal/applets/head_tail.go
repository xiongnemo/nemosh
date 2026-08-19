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
		spec, paths, err := countArgs("head", args, 10, true)
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			return copyHeadOf(stdout, stdin, spec)
		}
		view := ProcessViewFromContext(ctx)
		for _, path := range paths {
			file, err := OpenProcessInput(ctx, view, path)
			if err != nil {
				return operandFailure(path, err)
			}
			copyErr := copyHeadOf(stdout, file, spec)
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
func copyHeadOf(stdout io.Writer, input io.Reader, spec countSpec) error {
	// `head -n -2` is everything but the last two, which cannot be answered without
	// reading to the end first. See head_tail_offset.go.
	if spec.allButLast {
		if spec.bytes {
			return copyAllButLastBytes(stdout, input, spec.count)
		}
		return copyAllButLastLines(stdout, input, spec.count)
	}
	if !spec.bytes {
		return copyHead(stdout, input, spec.count)
	}
	_, err := io.CopyN(stdout, input, int64(spec.count))
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
		// -c counts bytes here too now. It was head-only, and the asymmetry was
		// documented as deliberate -- claiming both without implementing both
		// would have been the kind of thing a script discovers the hard way. This
		// implements it instead.
		spec, paths, err := countArgs("tail", args, 10, true)
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			return copyTailOf(stdout, stdin, spec)
		}
		view := ProcessViewFromContext(ctx)
		for _, path := range paths {
			file, err := OpenProcessInput(ctx, view, path)
			if err != nil {
				return cannotOpen(path, err)
			}
			copyErr := copyTailOf(stdout, file, spec)
			closeErr := file.Close()
			if err := errors.Join(copyErr, closeErr); err != nil {
				return err
			}
		}
		return nil
	}}
}

// copyTailOf is the last `count` lines, or the last `count` bytes for -c.
//
//	$ printf '0123456789' | tail -c 3   ->  789
//
// No newline is added: -c deals in bytes, and inventing one would change the
// length of what was asked for.
func copyTailOf(stdout io.Writer, input io.Reader, spec countSpec) error {
	// `tail -n +10` starts at line 10 rather than ending there. This is the case the sign
	// was being dropped on; see head_tail_offset.go.
	if spec.fromStart {
		if spec.bytes {
			return copyBytesFromStart(stdout, input, spec.count)
		}
		return copyLinesFromStart(stdout, input, spec.count)
	}
	if !spec.bytes {
		return copyTail(stdout, input, spec.count)
	}
	data, err := io.ReadAll(input)
	if err != nil {
		return err
	}
	if len(data) > spec.count {
		data = data[len(data)-spec.count:]
	}
	_, err = stdout.Write(data)
	return err
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
// bareCountOption reads the `-3` form, and only that: a dash followed by digits
// and nothing else. `-n` and `-c` are handled by name, and anything with a
// letter in it is an option this build does not have rather than a count.
func bareCountOption(arg string) (int, bool) {
	if len(arg) < 2 || arg[0] != '-' {
		return 0, false
	}
	count, err := strconv.Atoi(arg[1:])
	if err != nil || count < 0 {
		return 0, false
	}
	return count, true
}

func lineCountArgs(applet string, args []string, defaultCount int) (int, []string, error) {
	spec, paths, err := countArgs(applet, args, defaultCount, false)
	return spec.count, paths, err
}

// countArgs reads -n, and for head also -c, which counts bytes rather than
// lines.
//
// -c is what a script reaches for to take the first kilobyte of something, and
// the two are exclusive by nature: the last one given wins, as it does in
// busybox, because both write into the same count.
func countArgs(applet string, args []string, defaultCount int, allowBytes bool) (countSpec, []string, error) {
	spec := countSpec{count: defaultCount}
	supported := []string{"-n"}
	if allowBytes {
		supported = append(supported, "-c")
	}
	for len(args) > 0 {
		// `-3` is the obsolete form POSIX still lists, and it is what everybody
		// types. busybox takes it; refusing it made `head -3` an error in a shell
		// whose whole point is that the muscle memory works.
		if digits, ok := bareCountOption(args[0]); ok {
			spec, args = countSpec{count: digits}, args[1:]
			continue
		}
		if args[0] != "-n" && !(allowBytes && args[0] == "-c") {
			break
		}
		flag := args[0]
		if len(args) < 2 {
			return countSpec{}, nil, fmt.Errorf("%s: requires a count", flag)
		}
		// parseCountSpec rather than Atoi: the sign is part of the request. See
		// head_tail_offset.go.
		parsed, parseErr := parseCountSpec(args[1], flag == "-c")
		if parseErr != nil {
			return countSpec{}, nil, parseErr
		}
		spec, args = parsed, args[2:]
	}
	paths, err := streamOperands(applet, args, supported...)
	if err != nil {
		return countSpec{}, nil, err
	}
	return spec, paths, nil
}
