package applets

import (
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"strconv"
	"strings"
)

// odSpec is one of od's output types, busybox's tspec (coreutils/od_bloaty.c): a kind, one of
// d o u x f a c, the bytes a datum takes, and the digits it is printed in. hexl is -t's z, the
// line's bytes as text after it.
type odSpec struct {
	kind  byte
	size  int
	width int
	hexl  bool
}

// The digits a datum of so many bytes takes, od_bloaty.c's tables.
var (
	odOctalDigits    = [...]int{0, 3, 6, 8, 11, 14, 16, 19, 22}
	odSignedDigits   = [...]int{1, 4, 6, 8, 11, 13, 16, 18, 20}
	odUnsignedDigits = [...]int{0, 3, 5, 8, 10, 13, 15, 17, 20}
	odHexDigits      = [...]int{0, 2, 4, 6, 8, 10, 12, 14, 16}
)

// odLongSize is C's long, which -l and -t dL name: four bytes on Windows, as busybox-w32 has
// it, and eight elsewhere, as busybox has it there.
var odLongSize = map[bool]int{true: 4, false: 8}[runtime.GOOS == "windows"]

// parseOdTypes is busybox's decode_format_string: each type in text, one after another, so
// -t x1c is two.
func parseOdTypes(text string) ([]odSpec, error) {
	var specs []odSpec
	for rest := text; rest != ""; {
		spec, next, err := parseOdType(text, rest)
		if err != nil {
			return nil, err
		}
		specs, rest = append(specs, spec), next
	}
	return specs, nil
}

// parseOdType is busybox's decode_one_format: a kind, then a size, as a count of bytes or as
// C S I L for char, short, int and long, or F D for float and double; no size is an int or a
// double.
func parseOdType(whole, text string) (odSpec, string, error) {
	kind, rest := text[0], text[1:]
	spec := odSpec{kind: kind, size: 1, width: 3}
	switch kind {
	case 'd', 'o', 'u', 'x':
		spec.size = 4
		if size, next, ok := odTypeSize(rest, "CSIL", map[byte]int{'C': 1, 'S': 2, 'I': 4, 'L': odLongSize}); ok {
			if size != 1 && size != 2 && size != 4 && size != 8 {
				return odSpec{}, "", fmt.Errorf("invalid type string '%s'; %d-byte %s type is not supported", whole, size, "integral")
			}
			spec.size, rest = size, next
		}
		spec.width = map[byte][9]int{'d': odSignedDigits, 'o': odOctalDigits, 'u': odUnsignedDigits, 'x': odHexDigits}[kind][spec.size]
	case 'f':
		spec.size = 8
		if size, next, ok := odTypeSize(rest, "FDL", map[byte]int{'F': 4, 'D': 8, 'L': 16}); ok {
			// A long double is ten bytes held in sixteen, which nothing here can read.
			if size != 4 && size != 8 {
				return odSpec{}, "", fmt.Errorf("invalid type string '%s'; %d-byte %s type is not supported", whole, size, "floating point")
			}
			spec.size, rest = size, next
		}
		// FLT_DIG and DBL_DIG digits after the point, and eight more for the rest. busybox has
		// no float.h there, and its own FLT_DIG is 7.
		spec.width = map[int]int{4: 7 + 8, 8: 15 + 8}[spec.size]
	case 'a', 'c':
	default:
		return odSpec{}, "", fmt.Errorf("invalid character '%c' in type string '%s'", kind, whole)
	}
	if spec.hexl = strings.HasPrefix(rest, "z"); spec.hexl {
		rest = rest[1:]
	}
	return spec, rest, nil
}

// odTypeSize is the size at the start of text, a letter of letters or a count of bytes, and
// the text after it.
func odTypeSize(text, letters string, sizes map[byte]int) (int, string, bool) {
	if text != "" && strings.IndexByte(letters, text[0]) >= 0 {
		return sizes[text[0]], text[1:], true
	}
	digits := 0
	for digits < len(text) && isASCIIDigit(text[digits]) {
		digits++
	}
	if digits == 0 {
		return 0, text, false
	}
	size, err := strconv.Atoi(text[:digits])
	if err != nil {
		size = math.MaxInt32
	}
	return size, text[digits:], true
}

// write puts a line's data as spec shows them, each field with the blank busybox's formats
// begin with. chunk is a whole number of datums, the last block padded with zeros to one.
// pad is blanks shared among the fields of a whole line, fields of them, as GNU od shares
// them: the field k from the end has pad*k/fields less those before it.
func (s odSpec) write(out *strings.Builder, chunk []byte, fields, pad int) {
	for at, left := 0, fields; at+s.size <= len(chunk); at, left = at+s.size, left-1 {
		out.WriteString(strings.Repeat(" ", pad*left/fields-pad*(left-1)/fields))
		datum := chunk[at : at+s.size]
		switch s.kind {
		case 'c':
			fmt.Fprintf(out, "%4s", dumpCharName(datum[0]))
		case 'a':
			out.WriteString(odNamedChar(datum[0]))
		case 'f':
			out.WriteString(" " + odFloat(datum, s.width))
		default:
			var padded [8]byte
			copy(padded[:], datum)
			value := binary.LittleEndian.Uint64(padded[:])
			switch shift := 64 - 8*uint(s.size); s.kind {
			case 'd':
				fmt.Fprintf(out, " %*d", s.width, int64(value<<shift)>>shift)
			case 'o':
				fmt.Fprintf(out, " %0*o", s.width, value)
			case 'u':
				fmt.Fprintf(out, " %*d", s.width, value)
			case 'x':
				fmt.Fprintf(out, " %0*x", s.width, value)
			}
		}
	}
}

// dumpCharName is -t c, busybox's print_ascii: C's escape for NUL and \a \b \t \n \v \f \r,
// the character if it prints, and three octal digits otherwise.
func dumpCharName(b byte) string {
	names := map[byte]string{
		0: `\0`, '\a': `\a`, '\b': `\b`, '\t': `\t`,
		'\n': `\n`, '\v': `\v`, '\f': `\f`, '\r': `\r`,
	}
	if name, found := names[b]; found {
		return name
	}
	if b >= 0x20 && b < 0x7f {
		return string(rune(b))
	}
	return fmt.Sprintf("%03o", b)
}

// odNamedChar is -t a, busybox's print_named_ascii: the byte's low seven bits, by name when
// they are not a graphic character.
func odNamedChar(b byte) string {
	names := [...]string{"nul", "soh", "stx", "etx", "eot", "enq", "ack", "bel", "bs", "ht", "nl",
		"vt", "ff", "cr", "so", "si", "dle", "dc1", "dc2", "dc3", "dc4", "nak", "syn", "etb", "can",
		"em", "sub", "esc", "fs", "gs", "rs", "us", "sp"}
	switch b &= 0x7f; {
	case b == 0x7f:
		return " del"
	case b > ' ':
		return "   " + string(rune(b))
	}
	return fmt.Sprintf("%4s", names[b])
}

// odFloat is a float or a double as C's %.*e has it, two digits of exponent at least, and inf
// and nan as C spells them, where busybox-w32's msvcrt writes 1.#INF000e+000.
func odFloat(datum []byte, width int) string {
	var value float64
	digits := width - 8
	if len(datum) == 4 {
		value = float64(math.Float32frombits(binary.LittleEndian.Uint32(datum)))
	} else {
		value = math.Float64frombits(binary.LittleEndian.Uint64(datum))
	}
	text := strconv.FormatFloat(value, 'e', digits, 64)
	if math.IsInf(value, 0) || math.IsNaN(value) {
		text = map[bool]string{true: "inf", false: "nan"}[math.IsInf(value, 0)]
		if math.Signbit(value) {
			text = "-" + text
		}
	}
	return fmt.Sprintf("%*s", width, text)
}
