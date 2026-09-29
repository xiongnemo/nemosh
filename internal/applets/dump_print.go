package applets

import (
	"encoding/binary"
	"math"
	"runtime"
	"strconv"
	"strings"
)

// render is one print of a unit over the datum at block[at:], as libbb's display prints it,
// its trailing blank left off when last and the print is nospace.
func (p *dumpPrint) render(block []byte, at int, address int64, last bool) string {
	spec := p.spec
	var buffer [8]byte
	datum := buffer[:]
	if at < len(block) {
		copy(datum, block[at:min(len(block), at+p.bytes)])
	}
	body := ""
	switch p.kind {
	case dumpText:
		body = ""
	case dumpAddress:
		body = cInteger(spec, p.verb, false, uint64(address))
	case dumpBlank:
		body = cPad(cSpec{width: spec.width, precision: -1}, "", "", false)
	case dumpEscaped:
		body = dumpEscapedChar(spec, datum[0])
	case dumpChar:
		body = cPad(spec, "", string(datum[0]), false)
	case dumpPrintable:
		c := datum[0]
		if c < 0x20 || c > 0x7e {
			c = '.'
		}
		body = cPad(spec, "", string(c), false)
	case dumpNamed:
		body = dumpNamedChar(spec, datum[0])
	case dumpString:
		text := block[min(at, len(block)):min(len(block), at+p.bytes)]
		if end := strings.IndexByte(string(text), 0); end >= 0 {
			text = text[:end]
		}
		body = cString(spec, string(text))
	case dumpFloat:
		value := math.Float64frombits(binary.LittleEndian.Uint64(datum))
		if p.bytes == 4 {
			value = float64(math.Float32frombits(binary.LittleEndian.Uint32(datum)))
		}
		body = cFloat(spec, p.verb, value)
	case dumpSigned:
		value := p.signed(datum)
		magnitude := uint64(value)
		if value < 0 {
			magnitude = -magnitude
		}
		body = cInteger(spec, p.verb, value < 0, magnitude)
	case dumpUnsigned:
		body = cInteger(spec, p.verb, false, binary.LittleEndian.Uint64(datum))
	}
	out := p.before + body + p.after
	if last && p.nospace {
		out = out[:len(out)-1]
	}
	return out
}

// signed is a %d's datum: of eight bytes, as busybox passes it, a long long that %d reads the
// low four bytes of and %ld as many as C's long has.
func (p *dumpPrint) signed(datum []byte) int64 {
	value := int64(binary.LittleEndian.Uint64(datum))
	shift := 64 - 8*uint(p.bytes)
	value = value << shift >> shift
	if p.bytes == 8 && (p.long == 0 || p.long == 1 && runtime.GOOS == "windows") {
		value = int64(int32(value))
	}
	return value
}

// dumpEscapedChar is %_c, busybox's conv_c: C's escape for NUL and \a to \r, the character
// if it prints, and three octal digits otherwise.
func dumpEscapedChar(spec cSpec, c byte) string {
	switch {
	case c == 0 || c >= 7 && c <= 13:
		return cString(spec, `\`+string("0abtnvfr"[max(int(c)-6, 0)]))
	case c >= 0x20 && c <= 0x7e:
		return cPad(spec, "", string(c), false)
	}
	return cString(spec, strconv.FormatUint(uint64(c)|0o1000, 8)[1:])
}

// dumpNamedChar is %_u, busybox's conv_u: the name of a control character, the character if
// it prints, and hex digits for the rest.
func dumpNamedChar(spec cSpec, c byte) string {
	names := [...]string{"nul", "soh", "stx", "etx", "eot", "enq", "ack", "bel", "bs", "ht", "lf",
		"vt", "ff", "cr", "so", "si", "dle", "dc1", "dc2", "dc3", "dc4", "nak", "syn", "etb", "can",
		"em", "sub", "esc", "fs", "gs", "rs", "us"}
	switch {
	case c <= 0x1f:
		return cString(spec, names[c])
	case c == 0x7f:
		return cString(spec, "del")
	case c < 0x7f:
		return cPad(spec, "", string(c), false)
	}
	return cInteger(spec, 'x', false, uint64(c))
}

// cSpec is a printf conversion's flags, width and precision.
type cSpec struct {
	minus, plus, space, hash, zero bool
	width, precision               int
}

// parseCSpec reads flags, a width and a precision as printf does, and passes over what
// follows them.
func parseCSpec(text string) cSpec {
	spec, at := cSpec{precision: -1}, 0
	for ; at < len(text) && strings.IndexByte("-+ #0", text[at]) >= 0; at++ {
		switch text[at] {
		case '-':
			spec.minus = true
		case '+':
			spec.plus = true
		case ' ':
			spec.space = true
		case '#':
			spec.hash = true
		default:
			spec.zero = true
		}
	}
	number := func() int {
		start := at
		for at < len(text) && isASCIIDigit(text[at]) {
			at++
		}
		value, _ := strconv.Atoi(text[start:at])
		return min(value, 1<<20)
	}
	spec.width = number()
	if at < len(text) && text[at] == '.' {
		at++
		spec.precision = number()
	}
	return spec
}

// cPad is a conversion at its width: blanks before it, or after it with -, or zeros between
// its sign and its digits with 0, where zeros may stand.
func cPad(spec cSpec, sign, body string, zeros bool) string {
	fill := spec.width - len(sign) - len(body)
	switch {
	case fill <= 0:
		return sign + body
	case spec.minus:
		return sign + body + strings.Repeat(" ", fill)
	case spec.zero && zeros:
		return sign + strings.Repeat("0", fill) + body
	}
	return strings.Repeat(" ", fill) + sign + body
}

// cString is %s: no more than the precision's bytes of text.
func cString(spec cSpec, text string) string {
	if spec.precision >= 0 && len(text) > spec.precision {
		text = text[:spec.precision]
	}
	return cPad(spec, "", text, false)
}

// cInteger is %d %i %o %u %x %X, the number's magnitude and whether it is negative.
func cInteger(spec cSpec, verb byte, negative bool, magnitude uint64) string {
	base := 10
	switch verb {
	case 'o':
		base = 8
	case 'x', 'X':
		base = 16
	}
	digits := strconv.FormatUint(magnitude, base)
	if verb == 'X' {
		digits = strings.ToUpper(digits)
	}
	if spec.precision == 0 && magnitude == 0 {
		digits = ""
	}
	if spec.precision > len(digits) {
		digits = strings.Repeat("0", spec.precision-len(digits)) + digits
	}
	sign, signed := "", verb == 'd' || verb == 'i'
	switch {
	case negative:
		sign = "-"
	case signed && spec.plus:
		sign = "+"
	case signed && spec.space:
		sign = " "
	case spec.hash && verb == 'o' && !strings.HasPrefix(digits, "0"):
		digits = "0" + digits
	case spec.hash && base == 16 && magnitude != 0:
		sign = "0" + string(verb)
	}
	return cPad(spec, sign, digits, spec.precision < 0)
}
