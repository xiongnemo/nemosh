package runtime

import (
	"strings"
	"testing"
	"time"
)

// facts answers a prompt's escapes from a table, so a test does not depend on who runs it.
func facts(table map[byte]string) func(byte) string {
	return func(escape byte) string { return table[escape] }
}

func TestDecodePrompt_escapes(t *testing.T) {
	now := time.Date(2026, time.September, 30, 21, 5, 9, 0, time.UTC)
	known := facts(map[byte]string{'u': "nemo", 'h': "workstation", 'H': "workstation.lan", 'w': "/c/work", 'W': "work", '$': "$", 's': "nemosh", 'v': "1.3", 'V': "1.3.5", '#': "7", '!': "12", 'j': "2", 'l': "tty"})
	tests := []struct{ name, text, want string }{
		{name: "the usual", text: `\u@\h:\w\n\$ \\`, want: "nemo@workstation:/c/work\n$ \\"},
		{name: "an unknown escape is left as written", text: `left\qright`, want: `left\qright`},
		{name: "a lone backslash at the end", text: `a\`, want: `a\`},
		{name: "the whole host and the directory's last part", text: `\H \W`, want: "workstation.lan work"},
		{name: "the shell and its version", text: `\s-\v \V`, want: "nemosh-1.3 1.3.5"},
		{name: "the numbers", text: `\# \! \j \l`, want: "7 12 2 tty"},
		{name: "times", text: `\d|\t|\T|\@|\A`, want: "Wed Sep 30|21:05:09|09:05:09|09:05 PM|21:05"},
		{name: "strftime", text: `\D{%H:%M} \D{} \D`, want: `21:05 21:05:09 \D`},
		{name: "strftime unclosed", text: `x\D{%M`, want: "x05"},
		{name: "octal is a byte", text: `\1004$ [\045] \555 [\0455]`, want: "@4$ [%] m [%5]"},
		{name: "octal zero is nothing", text: `a\000b`, want: "ab"},
		{name: "bell escape return", text: `\a\e\r`, want: "\a\x1b\r"},
		{name: "non-printing markers are dropped", text: `\[\033[0m\]x`, want: "\033[0mx"},
		{name: "no hex, as bash has none", text: `[\x55]`, want: `[\x55]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := decodePrompt(test.text, now, known, false); got != test.want {
				t.Fatalf("decodePrompt(%q) = %q, want %q", test.text, got, test.want)
			}
		})
	}
}

// What an escape stands for cannot drive the terminal: a control character or a byte that is
// not UTF-8 is written \xNN, while the prompt's own text keeps its escape sequences.
func TestDecodePrompt_sanitizesWhatItInterpolates(t *testing.T) {
	values := facts(map[byte]string{'u': "user\x00\x07\x1b", 'h': "host\r\n\u0085", 'w': "dir\t\b\x7f\xff"})

	got := decodePrompt(`\u|\h|\w`, time.Now(), values, false)

	if want := `user\x00\x07\x1b|host\x0d\x0a\x85|dir\x09\x08\x7f\xff`; got != want {
		t.Fatalf("decodePrompt() = %q, want %q", got, want)
	}
	for _, control := range []string{"\x00", "\x07", "\x08", "\x09", "\x0d", "\x1b", "\x7f", "\u0085"} {
		if strings.Contains(got, control) {
			t.Fatalf("decodePrompt() let %q through in %q", control, got)
		}
	}
	if got := decodePrompt("\x1b[32m\\u\x1b[0m", time.Now(), facts(map[byte]string{'u': "nemo\x1b[31m"}), false); got != "\x1b[32mnemo\\x1b[31m\x1b[0m" {
		t.Fatalf("the prompt's own colour or the value's escape was lost: %q", got)
	}
	if got := decodePrompt(`\u@\h:\w`, time.Now(), facts(map[byte]string{'u': "用户", 'h': "主机", 'w': "/工作/目录"}), false); got != "用户@主机:/工作/目录" {
		t.Fatalf("printable Unicode was escaped: %q", got)
	}
}

// ${var@P} decodes and then expands, so what an escape stands for is quoted for the expansion,
// as bash's sh_backslash_quote_for_double_quotes quotes it, and \$ is the expansion's own `\$`.
func TestDecodePrompt_quotesForTheExpansion(t *testing.T) {
	values := facts(map[byte]string{'w': "/tmp/$foo `x` \"q\" a\\b", '$': "$", 'u': "nemo"})

	if got, want := decodePrompt(`\w \$`, time.Now(), values, true), "/tmp/\\$foo \\`x\\` \\\"q\\\" a\\\\b \\$"; got != want {
		t.Fatalf("decodePrompt quoted = %q, want %q", got, want)
	}
	if got := decodePrompt(`\$`, time.Now(), facts(map[byte]string{'$': "#"}), true); got != "#" {
		t.Fatalf("root's symbol quoted = %q, want #", got)
	}
}
