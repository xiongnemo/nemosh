package applets

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// dumpKind is what a print unit prints, libbb's F_ flags.
type dumpKind byte

const (
	dumpText      dumpKind = iota // text alone
	dumpAddress                   // %_a and %_A: the offset, in d o or x
	dumpBlank                     // a conversion past the end of the input: blanks as wide
	dumpEscaped                   // %_c: the character, or C's escape, or three octal digits
	dumpChar                      // %c
	dumpFloat                     // %e %E %f %g %G
	dumpSigned                    // %d %i
	dumpPrintable                 // %_p: the character, or a dot
	dumpString                    // %s
	dumpNamed                     // %_u: the character, or its name, or two hex digits
	dumpUnsigned                  // %o %u %x %X
)

// dumpPrint is libbb's PR: one conversion, the text before it, which only a unit's first has,
// and the text after it up to the next.
type dumpPrint struct {
	kind   dumpKind
	before string
	// spec is the flags, width and precision, as printf reads them.
	spec cSpec
	verb byte
	// long is the l's before the verb, for a signed number of eight bytes.
	long  int
	after string
	bytes int
	// nospace is set on the last print of a unit that repeats when its text ends in a blank,
	// which the last repeat leaves off: 2/1 "%02x " prints "61 62", not "61 62 ".
	nospace bool
}

// rewrite is libbb's rewrite, before the block size is known: each unit's conversions, and
// the bytes each takes.
func (u *dumpUnit) rewrite() error {
	text, conversions := u.text, 0
	for at := 0; at < len(text); {
		percent := strings.IndexByte(text[at:], '%')
		if percent < 0 {
			u.prints = append(u.prints, dumpPrint{kind: dumpText, before: text[at:]})
			break
		}
		print := dumpPrint{before: text[at : at+percent]}
		spec := at + percent + 1
		end, precision := dumpFlagsEnd(text, spec, u.bytes != 0)
		print.spec = parseCSpec(text[spec:end])
		next, err := print.conversion(u, text, end, precision)
		if err != nil {
			return err
		}
		if print.kind != dumpAddress && u.bytes != 0 {
			if conversions++; conversions > 1 {
				return errors.New("byte count with multiple conversion characters")
			}
		}
		at = next + strings.IndexByte(text[next:]+"%", '%')
		print.after = text[next:at]
		u.prints = append(u.prints, print)
	}
	if u.bytes == 0 {
		for _, print := range u.prints {
			u.bytes += print.bytes
		}
	}
	return nil
}

// dumpFlagsEnd is where a conversion's flags, width and precision end, and the precision, or
// -1. With a byte count the precision is not needed, and a dot is passed over as a flag.
func dumpFlagsEnd(text string, at int, counted bool) (int, int) {
	flags := "#-+ 0123456789"
	if counted {
		flags = "." + flags
	}
	for at < len(text) && strings.IndexByte(flags, text[at]) >= 0 {
		at++
	}
	if counted || at == len(text) || text[at] != '.' {
		return at, -1
	}
	at++
	start := at
	for at < len(text) && isASCIIDigit(text[at]) {
		at++
	}
	if at == start {
		return at, -1
	}
	precision, _ := strconv.Atoi(text[start:at])
	return at, precision
}

// conversion reads the conversion at text[at:], and reports where the text after it begins.
func (p *dumpPrint) conversion(u *dumpUnit, text string, at, precision int) (int, error) {
	bad := fmt.Errorf("bad conversion character %%%s", text[at:])
	if at == len(text) {
		return 0, bad
	}
	counts, width := "", 1
	switch verb := text[at]; {
	case verb == 'c':
		p.kind, p.verb, counts = dumpChar, 'c', "\x01"
	case verb == 'l' || strings.IndexByte("diouxX", verb) >= 0:
		for at < len(text) && text[at] == 'l' && p.long < 2 {
			p.long, at = p.long+1, at+1
		}
		if at == len(text) || strings.IndexByte("diouxX", text[at]) < 0 {
			return 0, fmt.Errorf("bad conversion character %%%s", text[at:])
		}
		p.verb, p.kind, counts = text[at], dumpSigned, "\x08\x04\x02\x01"
		if p.verb != 'd' && p.verb != 'i' {
			p.kind, counts = dumpUnsigned, "\x04\x02\x01"
		}
	case strings.IndexByte("eEfgG", verb) >= 0:
		p.kind, p.verb, counts = dumpFloat, verb, "\x08\x04"
	case verb == 's':
		p.kind, p.verb, p.bytes = dumpString, 's', u.bytes
		if u.bytes == 0 {
			if precision < 0 {
				return 0, errors.New("%s needs precision or byte count")
			}
			p.bytes = precision
		}
		return at + 1, nil
	case verb == '_' && at+1 < len(text):
		switch text[at+1] {
		case 'A', 'a':
			if at+2 == len(text) || strings.IndexByte("dox", text[at+2]) < 0 {
				return 0, bad
			}
			u.ending = u.ending || text[at+1] == 'A'
			p.kind, p.verb = dumpAddress, text[at+2]
			return at + 3, nil
		case 'c', 'p', 'u':
			p.kind = map[byte]dumpKind{'c': dumpEscaped, 'p': dumpPrintable, 'u': dumpNamed}[text[at+1]]
			p.verb, counts, width = 'c', "\x01", 2
		default:
			return 0, bad
		}
	default:
		return 0, bad
	}
	p.bytes = int(counts[0])
	if p.kind == dumpSigned && p.long == 0 {
		// A %d with no count is an int, as util-linux's is and bb_dump_size counts it.
		p.bytes = 4
	}
	if u.bytes != 0 {
		if strings.IndexByte(counts, byte(u.bytes)) < 0 || u.bytes > 8 {
			return 0, fmt.Errorf("bad byte count for conversion character %s", text[at:])
		}
		p.bytes = u.bytes
	}
	return at + width, nil
}

// fill is the rest of libbb's rewrite, once the block size is known: the last unit of a
// format with no count is repeated to fill the block, and a repeating unit's last print leaves
// off its trailing blank the last time.
func (f *dumpFormat) fill(block int) {
	for index, unit := range f.units {
		if index == len(f.units)-1 && f.bytes < block && !unit.setReps && unit.bytes > 0 {
			unit.reps += (block - f.bytes) / unit.bytes
		}
		if unit.reps > 1 && len(unit.prints) > 0 {
			last := &unit.prints[len(unit.prints)-1]
			rendered := last.before + last.after
			if last.kind != dumpText {
				rendered = last.after
			}
			last.nospace = rendered != "" && isCSpace(rendered[len(rendered)-1])
		}
	}
}
