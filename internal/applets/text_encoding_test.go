package applets

import (
	"strings"
	"testing"
)

// The bytes of 中文测试 in CP936, which is what Notepad still writes on a Chinese Windows and
// what most .bat files on such a machine are. Not valid UTF-8, and that is the whole point:
// `[]rune(string)` turns each of these bytes into U+FFFD, so anything that decoded and then
// re-encoded destroyed the file.
const gbkSample = "\xd6\xd0\xce\xc4\xb2\xe2\xca\xd4"

func TestReverseText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "ascii", in: "abc", want: "cba"},
		// By character, not by byte: this is what busybox gets wrong, and what the rune
		// path exists for.
		{name: "utf-8 by character", in: "中文", want: "文中"},
		{name: "mixed", in: "a中b", want: "b中a"},
		{name: "empty", in: "", want: ""},
		// By byte when it cannot be read as characters, which loses nothing.
		{name: "cp936 reverses bytes", in: gbkSample, want: "\xd4\xca\xe2\xb2\xc4\xce\xd0\xd6"},
		{name: "a lone invalid byte", in: "a\xffb", want: "b\xffa"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := reverseText(test.in)

			// Then
			if got != test.want {
				t.Fatalf("reverseText(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}

// The property that matters more than any single answer: reversing twice returns the file. It
// held for UTF-8 before and not for anything else, which is how a GBK file was destroyed.
func TestReverseText_roundTripsAnything(t *testing.T) {
	for _, text := range []string{"abc", "中文测试", gbkSample, "a\xffb", "\x80\x81\x82", ""} {
		// When
		got := reverseText(reverseText(text))

		// Then
		if got != text {
			t.Fatalf("reverseText twice on %q gave %q", text, got)
		}
	}
}

func TestSubstringText(t *testing.T) {
	tests := []struct {
		name  string
		value string
		start int
		count int
		want  string
	}{
		{name: "ascii", value: "abcdef", start: 1, count: 3, want: "bcd"},
		{name: "utf-8 counts characters", value: "中文测试", start: 1, count: 2, want: "文测"},
		{name: "past the end is clamped", value: "中文", start: 1, count: 99, want: "文"},
		{name: "start past the end", value: "中文", start: 9, count: 1, want: ""},
		// Bytes, because there is no character boundary to be found. Two bytes of CP936
		// happen to be one character, and answering with them is at least reversible.
		{name: "cp936 counts bytes", value: gbkSample, start: 0, count: 2, want: "\xd6\xd0"},
		{name: "cp936 past the end", value: gbkSample, start: 99, count: 2, want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := substringText(test.value, test.start, test.count)

			// Then
			if got != test.want {
				t.Fatalf("substringText(%q, %d, %d) = %q, want %q",
					test.value, test.start, test.count, got, test.want)
			}
		})
	}
}

// U+FFFD must never appear in output that did not contain it, which is the shape the bug took:
// one byte in, three bytes of replacement character out.
func TestReverseText_neverInventsAReplacementCharacter(t *testing.T) {
	for _, text := range []string{gbkSample, "a\xffb", "\x80\x81"} {
		// When
		got := reverseText(text)

		// Then
		// strings.Contains on the encoded bytes, not ContainsRune: Go documents
		// IndexRune(RuneError) as matching *any* invalid byte sequence, so the
		// obvious spelling of this test reports every CP936 byte as a replacement
		// character and passes for the wrong reason.
		if strings.Contains(got, string([]byte{0xef, 0xbf, 0xbd})) {
			t.Fatalf("reverseText(%q) = %q, which invented a replacement character", text, got)
		}
	}
}
