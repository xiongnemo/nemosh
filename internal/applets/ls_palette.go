package applets

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// LS_COLORS, read as GNU's ls reads it (ls.c's parse_ls_color and print_color_indicator), for
// when it is set: what Ubuntu's ~/.bashrc sets with `eval "$(dircolors -b)"`, and what a session
// here sets; see the runtime's session_defaults.go. Unset, ls keeps busybox's three colours,
// which read no LS_COLORS.
//
// A name is \e[CODEm NAME \e[0m, the code its type's -- di, ln, or for a link to nothing, ex,
// pi, so, bd, cd -- or, for a file that is no program, the first *SUFFIX it ends with, matched
// without regard to case, as Windows names files and as coreutils 9.2 matches when no two
// suffixes differ only in case; or fi, which by default is nothing at all. The first colour is
// preceded by \e[0m, once, as GNU's put_indicator does. tw, ow, st, su, sg and ca are taken and
// not applied: Windows has no sticky or set-id bit, and Go reports every directory there as
// writable by others, so ow would paint them all blue on green.

// errLsColors is GNU's word for an LS_COLORS it cannot read, which turns colour off.
var errLsColors = errors.New("unparsable value for LS_COLORS environment variable")

// lsPalette is LS_COLORS read.
type lsPalette struct {
	codes    map[string]string
	suffixes []lsSuffix
}

type lsSuffix struct{ suffix, code string }

// parseLsColors reads value, entries `xx=CODE` and `*SUFFIX=CODE` between colons.
func parseLsColors(value string) (*lsPalette, error) {
	palette := &lsPalette{codes: map[string]string{"lc": "\033[", "rc": "m", "rs": "0"}}
	for _, entry := range strings.Split(value, ":") {
		if entry == "" {
			continue
		}
		key, code, ok := strings.Cut(entry, "=")
		if !ok {
			return nil, errLsColors
		}
		code, ok = lsColorsCode(code)
		switch {
		case !ok:
			return nil, errLsColors
		case strings.HasPrefix(key, "*"):
			palette.suffixes = append(palette.suffixes, lsSuffix{strings.ToLower(key[1:]), code})
		case len(key) == 2:
			palette.codes[key] = code
		default:
			return nil, errLsColors
		}
	}
	return palette, nil
}

// lsColorsCode is a code with GNU's escapes taken: \e and the other C letters, \_ for a space,
// \NNN in octal, \xHH, and ^X for a control character.
func lsColorsCode(code string) (string, bool) {
	if !strings.ContainsAny(code, `\^`) {
		return code, true
	}
	var decoded strings.Builder
	for index := 0; index < len(code); index++ {
		char := code[index]
		switch {
		case char == '^' && index+1 < len(code):
			index++
			if code[index] == '?' {
				decoded.WriteByte(0x7f)
			} else {
				decoded.WriteByte(code[index] & 0x1f)
			}
		case char != '\\':
			decoded.WriteByte(char)
		case index+1 == len(code):
			return "", false
		default:
			index++
			if letter := strings.IndexByte(`abefnrtv_?\^`, code[index]); letter >= 0 {
				decoded.WriteByte("\a\b\033\f\n\r\t\v ?\\^"[letter])
				continue
			}
			base, digits, digitSet := 8, 3, "01234567"
			if code[index] == 'x' || code[index] == 'X' {
				base, digits, digitSet = 16, 2, "0123456789abcdefABCDEF"
				index++
			}
			end := index
			for end < len(code) && end-index < digits && strings.IndexByte(digitSet, code[end]) >= 0 {
				end++
			}
			number, err := strconv.ParseUint(code[index:end], base, 8)
			if err != nil {
				return "", false
			}
			decoded.WriteByte(byte(number))
			index = end - 1
		}
	}
	return decoded.String(), true
}

// colored is GNU's is_colored: set, and neither empty nor 0 nor 00.
func (p *lsPalette) colored(key string) bool {
	code := p.codes[key]
	return code != "" && code != "0" && code != "00"
}

// code is the colour of a file of info, at path, named name; "" for none.
func (p *lsPalette) code(name, path string, info os.FileInfo) string {
	if info == nil {
		return p.codes["mi"]
	}
	mode := info.Mode()
	switch {
	case mode&os.ModeSymlink != 0:
		if _, err := os.Stat(path); err != nil && p.colored("or") {
			return p.codes["or"]
		}
		return p.codes["ln"]
	case info.IsDir():
		return p.codes["di"]
	case mode&os.ModeNamedPipe != 0:
		return p.codes["pi"]
	case mode&os.ModeSocket != 0:
		return p.codes["so"]
	case mode&os.ModeCharDevice != 0:
		return p.codes["cd"]
	case mode&os.ModeDevice != 0:
		return p.codes["bd"]
	case isExecutableEntry(info) && p.colored("ex"):
		return p.codes["ex"]
	}
	// From the last, as GNU's list has each one put at its head: one added after the
	// database wins over the database's own.
	lower := strings.ToLower(name)
	for index := len(p.suffixes) - 1; index >= 0; index-- {
		if strings.HasSuffix(lower, p.suffixes[index].suffix) {
			return p.suffixes[index].code
		}
	}
	return p.codes["fi"]
}

// paint is text in the colour of a file of info at path, as GNU's ls writes it.
func (p *lsPalette) paint(text, name, path string, info os.FileInfo) string {
	code := p.code(name, path, info)
	if code == "" {
		return text
	}
	return p.codes["lc"] + code + p.codes["rc"] + text + p.end()
}

// end is what follows a coloured name: ec, or lc rs rc.
func (p *lsPalette) end() string {
	if end, set := p.codes["ec"]; set {
		return end
	}
	return p.codes["lc"] + p.codes["rs"] + p.codes["rc"]
}

// readLsColors reads LS_COLORS, when it is set and not empty, into the palette, and answers the
// stream to write to: one that writes the end sequence before the first colour. One it cannot
// read is said on stderr, and there is no colour, as in GNU's.
func (o *lsOptions) readLsColors(view ProcessView, stdout, stderr io.Writer) io.Writer {
	value, set := view.LookupEnv("LS_COLORS")
	if !set || value == "" {
		return stdout
	}
	palette, err := parseLsColors(value)
	if err != nil {
		fmt.Fprintf(stderr, "ls: %v\n", err)
		o.colored = false
		return stdout
	}
	o.palette = palette
	return &lsFirstColor{Writer: stdout, start: palette.codes["lc"], lead: palette.end()}
}

// lsFirstColor writes lead once, before the first start written: the end sequence before the
// first colour, as GNU's ls writes it.
type lsFirstColor struct {
	io.Writer
	start, lead string
	written     bool
}

func (w *lsFirstColor) Write(p []byte) (int, error) {
	if w.written {
		return w.Writer.Write(p)
	}
	at := strings.Index(string(p), w.start)
	if at < 0 {
		return w.Writer.Write(p)
	}
	w.written = true
	if _, err := io.WriteString(w.Writer, string(p[:at])+w.lead); err != nil {
		return 0, err
	}
	n, err := w.Writer.Write(p[at:])
	return at + n, err
}

// TerminalFile is the file under it, so ls asking whether it writes to a terminal, and how wide
// one, is answered by that; see terminalSource.
func (w *lsFirstColor) TerminalFile() *os.File { return stdoutFile(w.Writer) }
