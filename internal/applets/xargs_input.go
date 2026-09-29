package applets

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
)

// xargsReader is busybox's process_stdin and its kin: the words of one command line, each
// taking its bytes and a NUL of the room the line has. The word the room runs out in begins
// the next line, and the quote it is in carries over with it.
type xargsReader struct {
	input *bufio.Reader
	// quote is the quote a word is in, and escaped whether a backslash came last.
	quote   byte
	escaped bool
	pending []byte
	// eof is -E's word, which ends the input.
	eof *string
}

// words is process_stdin: no more than most words, in no more than room bytes, a blank ending
// one, and quotes and a backslash keeping blanks and each other in one. An empty quoted word
// is none, as busybox's is.
func (r *xargsReader) words(room, most int) ([]string, error) {
	var words []string
	word, fill := r.pending, len(r.pending)
	r.pending = nil
	for fill < room {
		c, err := r.input.ReadByte()
		ended := false
		switch {
		case err != nil:
			if !errors.Is(err, io.EOF) {
				return nil, err
			}
			if len(word) == 0 {
				return words, nil
			}
			ended = true
		case r.escaped:
			r.escaped = false
			word, fill = append(word, c), fill+1
		case r.quote != 0:
			if c != r.quote {
				word, fill = append(word, c), fill+1
			} else {
				r.quote = 0
			}
		case isCSpace(c):
			ended = len(word) > 0
		case c == '\\':
			r.escaped = true
		case c == '\'' || c == '"':
			r.quote = c
		default:
			word, fill = append(word, c), fill+1
		}
		if !ended {
			continue
		}
		if r.quote != 0 {
			return nil, errors.New("unmatched " + map[byte]string{'\'': "single", '"': "double"}[r.quote] + " quote")
		}
		if r.eof != nil && string(word) == *r.eof {
			_, err := io.Copy(io.Discard, r.input)
			return words, err
		}
		words, word, fill = append(words, string(word)), nil, fill+1
		if len(words) == most {
			return words, nil
		}
	}
	r.pending = word
	return words, nil
}

// nulWords is process0_stdin: each NUL ends a word, an empty one too, and nothing else does.
func (r *xargsReader) nulWords(room, most int) ([]string, error) {
	var words []string
	word, fill := r.pending, len(r.pending)
	r.pending = nil
	for fill < room {
		c, err := r.input.ReadByte()
		if err != nil {
			if !errors.Is(err, io.EOF) || len(word) == 0 {
				return words, ignoreEOF(err)
			}
			c = 0
		}
		if fill++; c != 0 {
			word = append(word, c)
			continue
		}
		words, word = append(words, string(word)), nil
		if len(words) == most {
			return words, nil
		}
	}
	r.pending = word
	return words, nil
}

// line is process_stdin_with_replace: the next line that is not empty, its leading blanks
// left off, or with -0 the next such NUL-ended string, and whether one fitted in room.
func (r *xargsReader) line(room int, end byte) (string, bool, error) {
	var line []byte
	for len(line) < room {
		c, err := r.input.ReadByte()
		if err != nil && !errors.Is(err, io.EOF) {
			return "", false, err
		}
		if len(line) == 0 && (err != nil || c == end || isCSpace(c)) {
			if err != nil {
				return "", true, nil
			}
			continue
		}
		if err != nil || c == end {
			return string(line), true, nil
		}
		line = append(line, c)
	}
	return "", false, nil
}

// xargsConsole is where -p reads its answers: the console, which is neither stdin nor stdout.
var xargsConsole = func() (io.ReadCloser, error) {
	return os.Open(map[bool]string{true: "CONIN$", false: "/dev/tty"}[runtime.GOOS == "windows"])
}

// ask is -p's question, busybox's xargs_ask_confirmation: ` ?...` after the command line, and
// a reply read at the console rather than from stdin, yes when it begins with y. With no
// console to ask at, the answer is no, as busybox-w32's getche has it.
func (x *xargsRun) ask() bool {
	fmt.Fprint(x.stderr, " ?...")
	console, err := xargsConsole()
	if err != nil {
		fmt.Fprintln(x.stderr)
		return false
	}
	defer console.Close()
	reply, _ := bufio.NewReader(console).ReadString('\n')
	return strings.HasPrefix(reply, "y") || strings.HasPrefix(reply, "Y")
}

// replaced is -I's command line: STR in each of ARGS, PROG among them, replaced with line.
func replaced(command []string, placeholder, line string) []string {
	out := make([]string, len(command))
	for index, word := range command {
		out[index] = strings.ReplaceAll(word, placeholder, line)
	}
	return out
}

func ignoreEOF(err error) error {
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}
