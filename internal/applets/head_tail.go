package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
)

func newHeadApplet() Applet {
	return simpleApplet{name: "head", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		return runHeadTail(ctx, "head", args, stdin, stdout, stderr, copyHeadOf)
	}}
}

// runHeadTail is the shape head and tail share once their counts are parsed:
// stdin when there are no operands, and otherwise each file in turn under a
// header when there is more than one to tell apart.
//
// Shared because the header rule is the part worth having in one place. Two
// copies of "print a name above the lines, but only when it helps" is two
// answers eventually, and the pair already disagreed once -- -c was head's alone
// for a while.
func runHeadTail(ctx context.Context, applet string, args []string, stdin io.Reader, stdout, stderr io.Writer, copy func(io.Writer, io.Reader, countSpec) error) error {
	spec, headers, paths, err := headTailArgs(applet, args, 10, true)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		// stdin has no name, so -v cannot print one for it either.
		return copy(stdout, stdin, spec)
	}
	view := ProcessViewFromContext(ctx)
	headed := wantsHeader(headers, len(paths))
	failed := false
	for index, path := range paths {
		file, err := OpenProcessOperand(ctx, view, path, stdin)
		if err != nil {
			// An unreadable operand does not stop the rest. busybox reports it
			// and carries on to the next file, leaving status 1 behind -- so
			// `head -n1 a.txt nosuch b.txt` still prints b.txt, where returning
			// here silently dropped it.
			//
			// head names a missing operand one way and tail another, which is
			// what each reference does; operandFailure and cannotOpen differ only
			// in wording.
			reason := operandFailure(path, err)
			if applet == "tail" {
				reason = cannotOpen(path, err)
			}
			fmt.Fprintf(stderr, "%s: %v\n", applet, reason)
			failed = true
			continue
		}
		// The header comes *after* the open, so a file that could not be read
		// does not get one. It did, which put `==> nosuch <==` above the error
		// saying the file was not there.
		//
		// The blank separator still keys off the operand index rather than off
		// how many files succeeded, which is what busybox does: with the first of
		// two operands unreadable, the second still opens with a blank line.
		if headed {
			if _, err := io.WriteString(stdout, headTailHeader(path, index == 0)); err != nil {
				return err
			}
		}
		copyErr := copy(stdout, file, spec)
		closeErr := file.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return err
		}
	}
	if failed {
		return ExitStatus(1)
	}
	return nil
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

// copyHead writes the first count lines, each with the ending it arrived with.
//
// Fprintln here wrote a newline whatever the input had, so `head` added one to a
// file whose last line lacked it and turned every CRLF into LF. Both were measured
// against busybox and GNU, which preserve both; the second matters more on this
// platform, because `head build.log > first.txt` changing the line endings of the
// copy is a Windows-first shell corrupting the common case.
//
// scanLineWithEnding is eachLine's splitter, reused rather than reinvented -- head
// cannot use eachLine itself because it stops after count lines and eachLine reads
// to the end.
func copyHead(stdout io.Writer, input io.Reader, count int) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	scanner.Split(scanLineWithEnding)
	for i := 0; i < count && scanner.Scan(); i++ {
		if _, err := io.WriteString(stdout, scanner.Text()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func newTailApplet() Applet {
	return simpleApplet{name: "tail", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		// -c counts bytes here too now. It was head-only, and the asymmetry was
		// documented as deliberate -- claiming both without implementing both
		// would have been the kind of thing a script discovers the hard way. This
		// implements it instead.
		return runHeadTail(ctx, "tail", args, stdin, stdout, stderr, copyTailOf)
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

// copyTail writes the last count lines, each with the ending it arrived with. See
// copyHead for why the ending is kept rather than replaced.
func copyTail(stdout io.Writer, input io.Reader, count int) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	scanner.Split(scanLineWithEnding)
	// The tokens keep their endings, so this holds whole lines rather than lines
	// plus an assumption about how they ended.
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
		if _, err := io.WriteString(stdout, line); err != nil {
			return err
		}
	}
	return nil
}
