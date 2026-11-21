package runtime

import "testing"

// A key sequence is written as readline writes it, in a `bind` argument and in ~/.inputrc:
// \C-x a control key, \M-x the same key after Escape, \e Escape, the C escapes, \nnn in octal
// and \xHH in hex; and printed back as bash's `bind -p` prints it, "\C-a", "\ey", "\C-?".
func TestKeyseq_readsAndPrintsReadlinesNotation(t *testing.T) {
	for _, test := range []struct {
		text    string
		want    string
		printed string
	}{
		{text: `\C-t`, want: "\x14", printed: `\C-t`},
		{text: `\C-T`, want: "\x14", printed: `\C-t`},
		{text: `\C-x\C-r`, want: "\x18\x12", printed: `\C-x\C-r`},
		{text: `\M-y`, want: "\x1by", printed: `\ey`},
		{text: `\ec`, want: "\x1bc", printed: `\ec`},
		{text: `\C-?`, want: "\x7f", printed: `\C-?`},
		{text: `\C-\e(`, want: "\x1b(", printed: `\e(`},
		{text: `\e[A`, want: "\x1b[A", printed: `\e[A`},
		{text: `\t\n\r\a`, want: "\t\n\r\a", printed: `\C-i\C-j\C-m\C-g`},
		{text: `\101\x42`, want: "AB", printed: "AB"},
		{text: `a\"b\\c`, want: `a"b\c`, printed: `a\"b\\c`},
		{text: `\C-x\C-_A1\a`, want: "\x18\x1fA1\a", printed: `\C-x\C-_A1\C-g`},
	} {
		got, err := parseKeyseq(test.text)
		if err != nil || string(got) != test.want {
			t.Errorf("parseKeyseq(%q) = %q, %v; want %q", test.text, got, err, test.want)
			continue
		}
		if printed := printKeyseq(got); printed != test.printed {
			t.Errorf("printKeyseq(%q) = %q, want %q", got, printed, test.printed)
		}
	}
	for _, text := range []string{"", `\C-`} {
		if got, err := parseKeyseq(text); err == nil {
			t.Errorf("parseKeyseq(%q) = %q, want an error", text, got)
		}
	}
}

// The other spelling, an inputrc's `Control-u: ...`, names a key: Control- or C-, Meta- or M-,
// and the names readline gives the keys that have no character.
func TestKeyseq_readsAKeyName(t *testing.T) {
	for name, want := range map[string]string{
		"Control-u": "\x15", "C-u": "\x15", "Meta-x": "\x1bx", "M-x": "\x1bx",
		"TAB": "\t", "RET": "\r", "Return": "\r", "ESC": "\x1b", "SPC": " ", "Space": " ",
		"Rubout": "\x7f", "DEL": "\x7f", "LFD": "\n", "Newline": "\n", "x": "x", "M-C-x": "\x1b\x18",
	} {
		if got, ok := keyNamed(name); !ok || string(got) != want {
			t.Errorf("keyNamed(%q) = %q, %v; want %q", name, got, ok, want)
		}
	}
}
