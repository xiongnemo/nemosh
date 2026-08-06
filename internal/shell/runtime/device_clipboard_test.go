package runtime

import "testing"

// /dev/clipboard is text-only and UTF-8 at the shell/applet boundary
// (docs/design/windows-path-model.md:241). Windows programs put CRLF on the
// clipboard and expect CRLF back, so the two directions translate line breaks.
// A lone CR stays data either way, matching the rule the rest of the shell
// follows (docs/design/windows-execution-model.md, "Line Endings").
func TestClipboardTextForShell_translatesOnlyCarriageReturnLineFeedPairs(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{name: "empty", text: "", want: ""},
		{name: "no line break", text: "hello", want: "hello"},
		{name: "one pair", text: "a\r\nb", want: "a\nb"},
		{name: "trailing pair", text: "a\r\n", want: "a\n"},
		{name: "already line feed", text: "a\nb", want: "a\nb"},
		{name: "lone carriage return is data", text: "a\rb", want: "a\rb"},
		{name: "mixed", text: "a\r\nb\nc", want: "a\nb\nc"},
		{name: "non-ascii survives", text: "你好\r\n世界", want: "你好\n世界"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// When
			got := clipboardTextForShell(testCase.text)

			// Then
			if got != testCase.want {
				t.Fatalf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestClipboardTextForWindows_endsEveryLineWithCarriageReturnLineFeed(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{name: "empty", text: "", want: ""},
		{name: "no line break", text: "hello", want: "hello"},
		{name: "one line feed", text: "a\nb", want: "a\r\nb"},
		{name: "trailing line feed", text: "a\n", want: "a\r\n"},
		{name: "existing pair is not doubled", text: "a\r\nb", want: "a\r\nb"},
		{name: "lone carriage return is data", text: "a\rb", want: "a\rb"},
		{name: "mixed", text: "a\r\nb\nc", want: "a\r\nb\r\nc"},
		{name: "non-ascii survives", text: "你好\n世界", want: "你好\r\n世界"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// When
			got := clipboardTextForWindows(testCase.text)

			// Then
			if got != testCase.want {
				t.Fatalf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}

// Whatever a script writes must come back to it unchanged, or a
// `cmd > /dev/clipboard` followed by a `cat /dev/clipboard` would not agree.
func TestClipboardText_roundTripsThroughTheWindowsForm(t *testing.T) {
	cases := []string{"", "hello", "a\nb", "a\n", "a\nb\nc\n", "你好\n世界"}
	for _, text := range cases {
		t.Run(text, func(t *testing.T) {
			// When
			got := clipboardTextForShell(clipboardTextForWindows(text))

			// Then
			if got != text {
				t.Fatalf("expected round trip to yield %q, got %q", text, got)
			}
		})
	}
}
