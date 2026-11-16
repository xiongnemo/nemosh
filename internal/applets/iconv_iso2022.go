package applets

import "bytes"

// ISO-2022-JP, code pages 50220 to 50222, through MLang a character at a time, as win_iconv
// does it (busybox-w32's miscutils/iconv.c): kernel32's conversion of these differs between
// Windows versions in whether it ends a sequence with the escape back to ASCII. The mode is the
// character set in force and whether it is shifted out.

const (
	iso2022ShiftIn  = 0
	iso2022ShiftOut = 1
	iso2022SI       = 0x0F
	iso2022SO       = 0x0E
)

const (
	iso2022ASCII = iota
	iso2022Roman
	iso2022Kana
	iso2022JIS1978
	iso2022JIS1983
	iso2022JISX0212
)

type iso2022Escape struct {
	escape string
	length int // the bytes one character of its set takes
	set    uint32
}

// The escapes, in win_iconv's order; a JIS X 0208-1978 one is read as 1983.
var iso2022jpEscapes = [...]iso2022Escape{
	{"\x1B\x28\x42", 1, iso2022ASCII},
	{"\x1B\x28\x4A", 1, iso2022Roman},
	{"\x1B\x28\x49", 1, iso2022Kana},
	{"\x1B\x24\x40", 2, iso2022JIS1983},
	{"\x1B\x24\x42", 2, iso2022JIS1983},
	{"\x1B\x24\x28\x44", 2, iso2022JISX0212},
}

func iso2022Mode(set, shift uint32) uint32 { return set<<8 | shift }

func iso2022jpToWide(cs *iconvCharset, in []byte, wide []uint16) (int, int, error) {
	switch in[0] {
	case 0x1B:
		for _, escape := range iso2022jpEscapes {
			if len(in) < len(escape.escape) {
				if string(in) == escape.escape[:len(in)] {
					return 0, 0, errIconvIncomplete
				}
			} else if string(in[:len(escape.escape)]) == escape.escape {
				cs.mode = iso2022Mode(escape.set, iso2022ShiftIn)
				return len(escape.escape), 0, nil
			}
		}
		return 0, 0, errIconvIllegal
	case iso2022SO:
		cs.mode = iso2022Mode(cs.mode>>8&0xFF, iso2022ShiftOut)
		return 1, 0, nil
	case iso2022SI:
		cs.mode = iso2022Mode(cs.mode>>8&0xFF, iso2022ShiftIn)
		return 1, 0, nil
	}
	set, shift := cs.mode>>8&0xFF, cs.mode&0xFF
	if in[0] < 0x20 {
		// A control character puts it back to ASCII, though no escape said so.
		set, shift = iso2022ASCII, iso2022ShiftIn
	}
	length := iso2022jpEscapes[set].length
	if len(in) < length {
		return 0, 0, errIconvIncomplete
	}
	for _, b := range in[:length] {
		if b >= 0x80 {
			return 0, 0, errIconvIllegal
		}
	}
	var sequence []byte
	if shift == iso2022ShiftOut {
		// MLang cannot read a shift-out a character at a time; the kana escape says the same.
		sequence = append([]byte(iso2022jpEscapes[iso2022Kana].escape), in[:length]...)
	} else {
		sequence = append([]byte(iso2022jpEscapes[set].escape), in[:length]...)
	}
	var unused uint32
	read, made, ok := mlangToWide(&unused, cs.codePage, sequence, wide)
	if !ok || read != len(sequence) {
		return 0, 0, errIconvIllegal
	}
	// What came back as the byte that went in was the default character, unless it was ASCII.
	if made > 0 && wide[0] == uint16(in[0]) && cs.mode != iso2022Mode(iso2022ASCII, iso2022ShiftIn) {
		return 0, 0, errIconvIllegal
	}
	cs.mode = iso2022Mode(set, shift)
	return length, made, nil
}

func iso2022jpFromWide(cs *iconvCharset, wide []uint16, out []byte) (int, error) {
	var scratch [iconvCharMax]byte
	var unused uint32
	read, written, ok := mlangToMulti(&unused, cs.codePage, wide, scratch[:])
	if !ok || read != len(wide) {
		return 0, errIconvIllegal
	}
	converted := scratch[:written]
	set, escapeLength := uint32(iso2022ASCII), 0
	if written != 1 {
		found := false
		for _, escape := range iso2022jpEscapes[1:] {
			if bytes.HasPrefix(converted, []byte(escape.escape)) {
				set, escapeLength, found = escape.set, len(escape.escape), true
				break
			}
		}
		if !found {
			return 0, errIconvIllegal
		}
	}
	shift := uint32(iso2022ShiftIn)
	if escapeLength < len(converted) && converted[escapeLength] == iso2022SO {
		shift = iso2022ShiftOut
		escapeLength++
	}
	length := iso2022jpEscapes[set].length
	if set == iso2022ASCII && wide[0] >= 0x80 || len(converted) < escapeLength+length {
		return 0, errIconvIllegal
	}
	character := converted[escapeLength : escapeLength+length]
	var result []byte
	if cs.mode != iso2022Mode(set, shift) {
		if cs.mode&0xFF == iso2022ShiftOut {
			result = append(result, iso2022SI)
		}
		if set == iso2022ASCII {
			result = append(result, iso2022jpEscapes[iso2022ASCII].escape...)
		} else {
			result = append(result, converted[:escapeLength]...)
		}
	}
	result = append(result, character...)
	cs.mode = iso2022Mode(set, shift)
	return copy(out, result), nil
}

// iso2022jpFlush ends a conversion in ASCII, shifted in, as ISO-2022-JP has to end.
func iso2022jpFlush(cs *iconvCharset, out []byte) (int, error) {
	written := 0
	if cs.mode&0xFF != iso2022ShiftIn {
		out[written] = iso2022SI
		written++
	}
	if cs.mode>>8&0xFF != iso2022ASCII {
		written += copy(out[written:], iso2022jpEscapes[iso2022ASCII].escape)
	}
	return written, nil
}
