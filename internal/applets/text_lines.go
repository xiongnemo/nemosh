package applets

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Line-oriented filters a clean Windows machine does not have.
//
// Measured against GNU coreutils rather than busybox, because busybox's own
// versions are the small ones and the behaviour people rely on is GNU's. The
// reference used throughout is the coreutils build installed on the machine this
// was written on; each applet's comment records what was observed.

// tac reverses the order of lines.
//
// The trailing-newline case is the one worth getting right, and it is not
// obvious. GNU, measured:
//
//	$ printf 'a\nb' | tac | od -c
//	0000000   b   a  \n
//
// The final line had no newline, and after reversal it comes first -- still
// without one. So `tac` does not add a separator that was not there; it moves the
// lines and leaves the newlines where they attach. Adding one would change the
// byte count of a round trip through tac twice.
func newTacApplet() Applet {
	return simpleApplet{name: "tac", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		_, paths, err := parseAppletOptions(args, "", "")
		if err != nil {
			return err
		}
		return eachTextInput(ctx, paths, stdin, func(reader io.Reader) error {
			lines, finalNewline, err := readLinesWithEnding(reader)
			if err != nil {
				return err
			}
			for index := len(lines) - 1; index >= 0; index-- {
				if _, err := io.WriteString(stdout, lines[index]); err != nil {
					return err
				}
				// Every line gets a newline except the one that did not have one
				// -- which, reversed, is the first thing printed.
				if index != len(lines)-1 || finalNewline {
					if _, err := io.WriteString(stdout, "\n"); err != nil {
						return err
					}
				}
			}
			return nil
		})
	}}
}

// rev reverses the characters of each line.
//
// By rune, not by byte. util-linux's rev is byte-oriented in the C locale and
// would cut a multi-byte character in half; reversing runes is what anyone means
// and what leaves the output valid UTF-8.
func newRevApplet() Applet {
	return simpleApplet{name: "rev", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		_, paths, err := parseAppletOptions(args, "", "")
		if err != nil {
			return err
		}
		return eachTextInput(ctx, paths, stdin, func(reader io.Reader) error {
			return eachLine(reader, func(line string, ending string) error {
				// By character for UTF-8 and by byte for anything else, because
				// rewriting bytes it cannot read is how this destroyed a GBK file.
				// See text_encoding.go.
				_, err := io.WriteString(stdout, reverseText(line)+ending)
				return err
			})
		})
	}}
}

// nl numbers lines.
//
// GNU's default is `-bt`: only non-empty lines are numbered, and an empty one
// gets the same width in blanks. Measured:
//
//	$ printf 'a\n\nb\n' | nl
//	     1  a
//
//	     2  b
//
// Six columns, right-aligned, then a tab. `-ba` numbers every line, which is the
// form people reach for and the reason the default surprises them.
func newNlApplet() Applet {
	return simpleApplet{name: "nl", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(args, "", "b")
		if err != nil {
			return err
		}
		// Three styles: `t` numbers non-empty lines and is the default, `a`
		// numbers every line, `n` numbers none. Three is one more state than a
		// boolean holds, which is how the first version of this numbered every
		// non-empty line under -bn.
		style := options.value('b')
		switch style {
		case "", "t", "a", "n":
		default:
			return fmt.Errorf("unsupported numbering style: %s", style)
		}
		number := 0
		return eachTextInput(ctx, paths, stdin, func(reader io.Reader) error {
			// A newline rather than the ending the input had, which is the one
			// place here that deliberately does *not* preserve it. nl produces a
			// new document -- every line gains a number and a tab -- rather than
			// reproducing the input, so normalising its own output is reasonable,
			// and it is what busybox does: measured, `nl` on a CRLF file answers LF
			// and on a file with no final newline adds one.
			return eachLine(reader, func(line, _ string) error {
				if style == "n" || (style != "a" && strings.TrimSpace(line) == "") {
					// A skipped line gets the number field's width in blanks plus
					// its tab, so the text still starts in one column. Measured
					// from GNU: `printf 'a\nb\n' | nl -bn` writes seven spaces.
					_, err := io.WriteString(stdout, "       "+line+"\n")
					return err
				}
				number++
				_, err := fmt.Fprintf(stdout, "%6d\t%s\n", number, line)
				return err
			})
		})
	}}
}

// readLinesWithEnding splits input into lines and reports whether the last one
// ended with a newline, which is the fact tac needs and nothing else records.
func readLinesWithEnding(reader io.Reader) ([]string, bool, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, false, err
	}
	if len(data) == 0 {
		return nil, false, nil
	}
	text := string(data)
	finalNewline := strings.HasSuffix(text, "\n")
	if finalNewline {
		text = text[:len(text)-1]
	}
	return strings.Split(text, "\n"), finalNewline, nil
}

// eachLine calls back once per line with the line and the ending that followed
// it, so a file round-trips unchanged.
//
// That promise is the whole point of handing the ending out separately, and for a
// long time it was not kept. The scanner used bufio.ScanLines, which throws the
// terminator away, so this reported `"\n"` for every line -- including a final
// line that had none, and including a CRLF line whose `\r` ScanLines had already
// eaten. The old comment here said the final line's ending "is not knowable from
// Scanner", and that was the mistake: it is knowable, by keeping the terminator in
// the token, which is what scanLineWithEnding does.
//
// Both halves were measured wrong against busybox *and* GNU on a three-byte file
// with no final newline and on a CRLF file:
//
//	rev head tail fold expand unexpand  --  added a newline, and turned CRLF into LF
//
// The second half matters more on this platform than the first. A Windows-first
// shell that rewrites every CRLF file it filters is corrupting the common case:
// `head build.log > first.txt` should not change the line endings of the copy.
//
// `line` is unchanged by the fix, only `ending` -- for CRLF the line was already
// `\r`-free, and for an unterminated final line it was already the bare text. So
// the callers that discard the ending (nl, shuf, tsort, strings, join, diff) are
// untouched, and they were the ones already agreeing with busybox.
func eachLine(reader io.Reader, handle func(line, ending string) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	scanner.Split(scanLineWithEnding)
	for scanner.Scan() {
		line, ending := splitLineEnding(scanner.Text())
		if err := handle(line, ending); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// scanLineWithEnding is bufio.ScanLines except that the token keeps its
// terminator. Keeping it is the only way the *last* line's ending is knowable,
// because at end of input there is nothing left to ask.
func scanLineWithEnding(data []byte, atEOF bool) (int, []byte, error) {
	if index := bytes.IndexByte(data, '\n'); index >= 0 {
		return index + 1, data[:index+1], nil
	}
	if atEOF && len(data) > 0 {
		// The final line, unterminated. Handing it back with no ending is what
		// lets a caller reproduce the file exactly.
		return len(data), data, nil
	}
	return 0, nil, nil
}

// splitLineEnding separates a line from the terminator scanLineWithEnding kept.
//
// A lone `\r` is *not* an ending: classic Mac OS used one and nothing has for
// twenty years, while a bare `\r` inside a line is a real character -- a progress
// bar written into a log, for instance -- and eating it would lose data.
func splitLineEnding(token string) (line, ending string) {
	switch {
	case strings.HasSuffix(token, "\r\n"):
		return token[:len(token)-2], "\r\n"
	case strings.HasSuffix(token, "\n"):
		return token[:len(token)-1], "\n"
	default:
		return token, ""
	}
}

// maxTextLine is generous rather than unlimited: a line longer than this is more
// likely a binary file being read as text than a line somebody wrote.
const maxTextLine = 4 * 1024 * 1024

// eachTextInput runs body over stdin when there are no operands, and over each
// named file otherwise -- the shape every filter here shares, and the shape the
// existing ones spell out one at a time.
//
// The operand is opened through the process view rather than with os.Open,
// because the shell's path spelling is not the host's; that seam is crossed in
// exactly one place per applet and this is it.
func eachTextInput(ctx context.Context, paths []string, stdin io.Reader, body func(io.Reader) error) error {
	if len(paths) == 0 {
		return body(stdin)
	}
	view := ProcessViewFromContext(ctx)
	for _, path := range paths {
		file, err := OpenProcessOperand(ctx, view, path, stdin)
		if err != nil {
			return cannotOpen(path, err)
		}
		bodyErr := body(file)
		closeErr := file.Close()
		if err := errors.Join(bodyErr, closeErr); err != nil {
			return err
		}
	}
	return nil
}
