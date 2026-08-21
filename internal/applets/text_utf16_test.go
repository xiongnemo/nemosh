package applets

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// Reading UTF-16, and refusing to guess.
//
// The interesting tests here are the two boundaries and the two refusals. A chunk that ends between
// the halves of a surrogate pair is the case a naive decoder gets wrong, and it gets it wrong
// invisibly: an emoji becomes two replacement characters somewhere past the four-thousandth byte of
// a file, which no small test would ever reach. The refusals matter more: a file with no byte-order
// mark must come through untouched, because guessing is how a binary gets rewritten.

// utf16Bytes encodes text the way Windows writes it.
func utf16Bytes(text string, bigEndian, bom bool) []byte {
	var out bytes.Buffer
	if bom {
		if bigEndian {
			out.Write([]byte{0xFE, 0xFF})
		} else {
			out.Write([]byte{0xFF, 0xFE})
		}
	}
	for _, unit := range utf16Units(text) {
		if bigEndian {
			out.Write([]byte{byte(unit >> 8), byte(unit)})
			continue
		}
		out.Write([]byte{byte(unit), byte(unit >> 8)})
	}
	return out.Bytes()
}

func utf16Units(text string) []uint16 {
	var units []uint16
	for _, r := range text {
		if r <= 0xFFFF {
			units = append(units, uint16(r))
			continue
		}
		r -= 0x10000
		units = append(units, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
	}
	return units
}

func decodeAll(t *testing.T, data []byte) string {
	t.Helper()
	out, err := io.ReadAll(decodeTextInput(bytes.NewReader(data)))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	return string(out)
}

func TestDecodeTextInput_readsWhatAByteOrderMarkDeclares(t *testing.T) {
	const text = "hello\nworld\n文字\n"
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{name: "UTF-16LE with a BOM", data: utf16Bytes(text, false, true), want: text},
		{name: "UTF-16BE with a BOM", data: utf16Bytes(text, true, true), want: text},
		// The mark is consumed. Left in, it is a zero-width character that makes
		// `grep '^hello'` miss the first line of anything Notepad wrote.
		{name: "UTF-8 with a BOM", data: append([]byte{0xEF, 0xBB, 0xBF}, text...), want: text},
		{name: "plain UTF-8", data: []byte(text), want: text},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := decodeAll(t, test.data); got != test.want {
				t.Fatalf("decoded %q, want %q", got, test.want)
			}
		})
	}
}

// Without a mark, nothing is decoded. This is the decision, not a limitation to be fixed later.
func TestDecodeTextInput_leavesUndeclaredBytesAlone(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "UTF-16LE with no BOM", data: utf16Bytes("hello\n", false, false)},
		{name: "UTF-16BE with no BOM", data: utf16Bytes("hello\n", true, false)},
		// GBK, which the existing posture already passes through untouched.
		{name: "GBK", data: []byte{0xd6, 0xd0, 0xce, 0xc4, 0x0a}},
		// A binary that happens to start with something BOM-adjacent but is not one.
		{name: "a binary starting FF FD", data: []byte{0xFF, 0xFD, 0x00, 0x01, 0x02}},
		{name: "empty", data: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := decodeAll(t, test.data); got != string(test.data) {
				t.Fatalf("rewrote undeclared bytes: %x became %x", test.data, got)
			}
		})
	}
}

// A surrogate pair split across a read, which is where a decoder that converts chunk by chunk
// silently produces two replacement characters instead of one emoji.
func TestDecodeTextInput_surrogatePairAcrossAChunkBoundary(t *testing.T) {
	// Every offset around the read boundary rather than the one I calculated, because I
	// calculated it wrongly: 2047 characters put the pair wholly inside the second chunk, so
	// the first version of this test passed with the boundary handling deliberately disabled.
	// A range costs nothing and cannot be off by one.
	for padding := 2040; padding <= 2050; padding++ {
		text := strings.Repeat("a", padding) + "🚀tail\n"
		for _, bigEndian := range []bool{false, true} {
			got := decodeAll(t, utf16Bytes(text, bigEndian, true))
			if got == text {
				continue
			}
			t.Fatalf("padding %d bigEndian %v: the pair did not survive the read boundary; %d runes decoded, want %d, rocket present: %v",
				padding, bigEndian, len([]rune(got)), len([]rune(text)),
				strings.ContainsRune(got, '🚀'))
		}
	}
}

// Odd trailing bytes and a dangling surrogate are reported rather than dropped.
func TestDecodeTextInput_truncatedInput(t *testing.T) {
	// A file cut mid-code-unit.
	data := utf16Bytes("hi", false, true)
	got := decodeAll(t, data[:len(data)-1])
	if !strings.HasPrefix(got, "h") {
		t.Fatalf("decoded %q, want it to start with what was whole", got)
	}
	if !strings.ContainsRune(got, '�') {
		t.Fatalf("decoded %q, want the truncation marked rather than dropped", got)
	}
}

// And the byte-exact applets stay byte-exact, which is the other half of the decision.
func TestCat_staysByteExactOverUTF16(t *testing.T) {
	data := utf16Bytes("hello\n", false, true)
	applet, ok := DefaultRegistry.Lookup("cat")
	if !ok {
		t.Fatal("cat is not registered")
	}
	var stdout, stderr bytes.Buffer

	// When
	if err := applet.Run(t.Context(), nil, bytes.NewReader(data), &stdout, &stderr); err != nil {
		t.Fatalf("cat: %v (%s)", err, stderr.String())
	}

	// Then -- `cat a > b` copies a file; it does not reinterpret one.
	if !bytes.Equal(stdout.Bytes(), data) {
		t.Fatalf("cat rewrote its input: %x became %x", data, stdout.Bytes())
	}
}
