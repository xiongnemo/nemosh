package applets

import "unicode/utf16"

// Writing back what was read, which is the half `decodeTextInput` left out.
//
// Decoding was enough for `grep`, which only has to *print* what it found, and the rule
// there is that output is UTF-8 (see text_utf16.go). It is not enough for an applet that
// rewrites a file: `sed -i` over a UTF-16 file must put UTF-16 back, or a Notepad file
// silently becomes a UTF-8 one and every other tool that reads it sees something else.
//
// That is the choice `iconv` settled on 2026-08-22 and that `sed -i` and `wc -m` were
// recorded as waiting for -- **an encoding is named, never guessed**. Here the name comes
// from the file's own byte-order mark, which is the writer stating what it wrote, so
// re-encoding to it is not a guess either. A file with no mark is not decoded and so is
// not re-encoded: its bytes pass through untouched, as they always did.

// textEncoding is what a byte-order mark said the bytes were.
type textEncoding int

const (
	// encodingBytes is the absence of a mark: not text this code claims to understand,
	// and therefore bytes to be moved rather than characters to be counted.
	encodingBytes textEncoding = iota
	encodingUTF8BOM
	encodingUTF16LE
	encodingUTF16BE
)

// detectTextEncoding reads the mark at the front of a buffer, if there is one.
func detectTextEncoding(data []byte) textEncoding {
	for _, mark := range byteOrderMarks {
		if len(data) < len(mark.prefix) || !equalBytes(data[:len(mark.prefix)], mark.prefix) {
			continue
		}
		switch {
		case !mark.utf16:
			return encodingUTF8BOM
		case mark.bigEndian:
			return encodingUTF16BE
		default:
			return encodingUTF16LE
		}
	}
	return encodingBytes
}

// markOf is the byte-order mark an encoding is written with.
//
// The mark is re-emitted because it was consumed on the way in. Dropping it would leave a
// UTF-16 file that Notepad opens as mojibake -- the bytes would be right and the file
// would be unreadable, which is a worse outcome than not having edited it.
func (e textEncoding) markOf() []byte {
	switch e {
	case encodingUTF8BOM:
		return []byte{0xEF, 0xBB, 0xBF}
	case encodingUTF16BE:
		return []byte{0xFE, 0xFF}
	case encodingUTF16LE:
		return []byte{0xFF, 0xFE}
	}
	return nil
}

// encode turns UTF-8 text back into the encoding, mark and all.
func (e textEncoding) encode(text []byte) []byte {
	if e == encodingBytes {
		return text
	}
	out := append([]byte{}, e.markOf()...)
	if e == encodingUTF8BOM {
		return append(out, text...)
	}
	// utf16.Encode splits astral characters into a surrogate pair, so an emoji survives
	// the round trip as the two code units it was.
	for _, unit := range utf16.Encode([]rune(string(text))) {
		if e == encodingUTF16BE {
			out = append(out, byte(unit>>8), byte(unit))
			continue
		}
		out = append(out, byte(unit), byte(unit>>8))
	}
	return out
}
