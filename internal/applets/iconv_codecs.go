package applets

// How long a character is, and how one becomes UTF-16 and back, for each kind of character set
// win_iconv knows (busybox-w32's miscutils/iconv.c), function for function.

func singleByteLength(*iconvCharset, []byte) (int, error) { return 1, nil }

func doubleByteLength(cs *iconvCharset, in []byte) (int, error) {
	length := 1
	if cs.leads[in[0]] {
		length = 2
	}
	if len(in) < length {
		return 0, errIconvIncomplete
	}
	return length, nil
}

// multiByteLength is GB18030's, the one multibyte code page win_iconv can measure: one byte,
// two, or four.
func multiByteLength(cs *iconvCharset, in []byte) (int, error) {
	if cs.codePage != 54936 {
		return 0, errIconvIncomplete
	}
	lead := in[0]
	switch {
	case lead <= 0x7F:
		return 1, nil
	case lead >= 0x81 && lead <= 0xFE && len(in) >= 2 && (in[1] >= 0x40 && in[1] <= 0x7E || in[1] >= 0x80 && in[1] <= 0xFE):
		return 2, nil
	case lead >= 0x81 && lead <= 0xFE && len(in) >= 4 && in[1] >= 0x30 && in[1] <= 0x39:
		return 4, nil
	}
	return 0, errIconvIncomplete
}

func utf8CharLength(_ *iconvCharset, in []byte) (int, error) {
	length := 0
	switch lead := in[0]; {
	case lead < 0x80:
		length = 1
	case lead&0xE0 == 0xC0:
		length = 2
	case lead&0xF0 == 0xE0:
		length = 3
	case lead&0xF8 == 0xF0:
		length = 4
	case lead&0xFC == 0xF8:
		length = 5
	case lead&0xFE == 0xFC:
		length = 6
	default:
		return 0, errIconvIllegal
	}
	if len(in) < length {
		return 0, errIconvIncomplete
	}
	return length, nil
}

func eucjpCharLength(_ *iconvCharset, in []byte) (int, error) {
	between := func(b byte) bool { return b >= 0xA1 && b <= 0xFE }
	switch lead := in[0]; {
	case lead < 0x80:
		return 1, nil
	case lead == 0x8E:
		if len(in) < 2 {
			return 0, errIconvIncomplete
		} else if in[1] < 0xA1 || in[1] > 0xDF {
			return 0, errIconvIllegal
		}
		return 2, nil
	case lead == 0x8F:
		if len(in) < 3 {
			return 0, errIconvIncomplete
		} else if !between(in[1]) || !between(in[2]) {
			return 0, errIconvIllegal
		}
		return 3, nil
	default:
		if len(in) < 2 {
			return 0, errIconvIncomplete
		} else if !between(lead) || !between(in[1]) {
			return 0, errIconvIllegal
		}
		return 2, nil
	}
}

func kernelToWide(cs *iconvCharset, in []byte, wide []uint16) (int, int, error) {
	length, err := cs.length(cs, in)
	if err != nil {
		return 0, 0, err
	}
	// MultiByteToWideChar takes a byte past ASCII as ASCII; win_iconv refuses it.
	if cs.codePage == 20127 && in[0] >= 0x80 {
		return 0, 0, errIconvIllegal
	}
	made := multiToWide(cs.codePage, mbToWideFlags(cs.codePage), in[:length], wide)
	if made == 0 {
		return 0, 0, errIconvIllegal
	}
	return length, made, nil
}

func kernelFromWide(cs *iconvCharset, wide []uint16, out []byte) (int, error) {
	flags, askDefault := uint32(0), false
	if !nullDefaultCharPage(cs.codePage) {
		askDefault = true
		if cs.flags&iconvTranslit == 0 {
			flags |= wcNoBestFitChars
		}
	}
	written, usedDefault, _ := wideToMulti(cs.codePage, flags, wide, out, askDefault)
	if written == 0 || usedDefault && cs.flags&iconvTranslit == 0 {
		return 0, errIconvIllegal
	}
	// The result has to be one character of the target, read back as one.
	if length, err := cs.length(cs, out[:written]); err != nil || length != written {
		return 0, errIconvIllegal
	}
	return written, nil
}

func mlangToWideChar(cs *iconvCharset, in []byte, wide []uint16) (int, int, error) {
	length, err := cs.length(cs, in)
	if err != nil {
		return 0, 0, err
	}
	read, made, ok := mlangToWide(&cs.mode, cs.codePage, in[:length], wide)
	if !ok || read != length {
		return 0, 0, errIconvIllegal
	}
	return length, made, nil
}

func mlangFromWideChar(cs *iconvCharset, wide []uint16, out []byte) (int, error) {
	var scratch [iconvCharMax]byte
	read, written, ok := mlangToMulti(&cs.mode, cs.codePage, wide, scratch[:])
	if !ok || read != len(wide) {
		return 0, errIconvIllegal
	}
	if length, err := cs.length(cs, scratch[:written]); err != nil || length != written {
		return 0, errIconvIllegal
	}
	return copy(out, scratch[:written]), nil
}

// unicodeUnit reads the unit at in in the byte order the character set has now: a mark read
// with //BOM's flag may have swapped it.
func unicodeUnit(cs *iconvCharset, little int, in []byte, width int) uint32 {
	page := cs.codePage
	if cs.mode&unicodeSwapped != 0 {
		page ^= 1
	}
	var unit uint32
	for index := range width {
		shift := 8 * uint(width-1-index)
		if page == little {
			shift = 8 * uint(index)
		}
		unit |= uint32(in[index]) << shift
	}
	return unit
}

func utf16ToWide(cs *iconvCharset, in []byte, wide []uint16) (int, int, error) {
	if len(in) < 2 {
		return 0, 0, errIconvIncomplete
	}
	unit := unicodeUnit(cs, 1200, in, 2)
	if cs.flags&iconvUseBOM != 0 && cs.mode&unicodeBOMDone == 0 {
		cs.mode |= unicodeBOMDone
		if unit == 0xFFFE {
			cs.mode |= unicodeSwapped
			return 2, 0, nil
		} else if unit == 0xFEFF {
			return 2, 0, nil
		}
	}
	wide[0] = uint16(unit)
	switch {
	case unit >= 0xDC00 && unit <= 0xDFFF:
		return 0, 0, errIconvIllegal
	case unit >= 0xD800 && unit <= 0xDBFF:
		if len(in) < 4 {
			return 0, 0, errIconvIncomplete
		}
		second := unicodeUnit(cs, 1200, in[2:], 2)
		if second < 0xDC00 || second > 0xDFFF {
			return 0, 0, errIconvIllegal
		}
		wide[1] = uint16(second)
		return 4, 2, nil
	}
	return 2, 1, nil
}

func utf16FromWide(cs *iconvCharset, wide []uint16, out []byte) (int, error) {
	written := 0
	if cs.flags&iconvUseBOM != 0 && cs.mode&unicodeBOMDone == 0 {
		cs.mode |= unicodeBOMDone
		written = putUnicodeUnit(cs.codePage == 1200, out, 0xFEFF, 2)
	}
	units := 1
	if wide[0] >= 0xD800 && wide[0] <= 0xDBFF {
		units = 2
	}
	for index := range units {
		written += putUnicodeUnit(cs.codePage == 1200, out[written:], uint32(wide[index]), 2)
	}
	return written, nil
}

func putUnicodeUnit(little bool, out []byte, unit uint32, width int) int {
	for index := range width {
		shift := 8 * uint(width-1-index)
		if little {
			shift = 8 * uint(index)
		}
		out[index] = byte(unit >> shift)
	}
	return width
}

func utf32ToWide(cs *iconvCharset, in []byte, wide []uint16) (int, int, error) {
	if len(in) < 4 {
		return 0, 0, errIconvIncomplete
	}
	point := unicodeUnit(cs, 12000, in, 4)
	if cs.flags&iconvUseBOM != 0 && cs.mode&unicodeBOMDone == 0 {
		cs.mode |= unicodeBOMDone
		if point == 0xFFFE0000 {
			cs.mode |= unicodeSwapped
			return 4, 0, nil
		} else if point == 0x0000FEFF {
			return 4, 0, nil
		}
	}
	if point >= 0xD800 && point <= 0xDFFF || point > 0x10FFFF {
		return 0, 0, errIconvIllegal
	}
	return 4, codePointToUTF16(point, wide), nil
}

func utf32FromWide(cs *iconvCharset, wide []uint16, out []byte) (int, error) {
	written := 0
	if cs.flags&iconvUseBOM != 0 && cs.mode&unicodeBOMDone == 0 {
		cs.mode |= unicodeBOMDone
		written = putUnicodeUnit(cs.codePage == 12000, out, 0xFEFF, 4)
	}
	return written + putUnicodeUnit(cs.codePage == 12000, out[written:], utf16ToCodePoint(wide), 4), nil
}
