package runtime

import (
	"strings"
	"testing"
)

// `$-` spells the letters that are on from the end of busybox's option table, how the shell
// was started among them, as busybox-w32 does: `set -eu` under -c is `uce`, `-ex` `xce`,
// `-fe` `cfe`, `-E` `Ec`, `-I` `cI`, and a script read from standard input with -e is `se`.
func TestShellOptions_lettersAreInBusyboxsOrder(t *testing.T) {
	for _, test := range []struct{ set, invocation, want string }{
		{set: "eu", invocation: "c", want: "uce"},
		{set: "ex", invocation: "c", want: "xce"},
		{set: "fe", invocation: "c", want: "cfe"},
		{set: "E", invocation: "c", want: "Ec"},
		{set: "I", invocation: "c", want: "cI"},
		{set: "aC", invocation: "c", want: "aCc"},
		{set: "e", invocation: "s", want: "se"},
		{set: "", invocation: "is", want: "si"},
	} {
		options := newShellOptions()
		options.invocation = test.invocation
		for _, letter := range []byte(test.set) {
			spec, _ := shellOptionSpecByLetter(letter)
			*spec.field(options) = true
		}
		if got := options.letters(); got != test.want {
			t.Errorf("set -%s under %s: $- = %q, want %q", test.set, test.invocation, got, test.want)
		}
	}
}

// Every letter `$-` can say is in the table it is spelled from, so none is left out.
func TestShellOptions_everyLetterHasItsPlace(t *testing.T) {
	for _, spec := range shellOptionSpecs {
		if spec.letter != 0 && !spec.bash && strings.IndexByte(busyboxLetters, spec.letter) < 0 {
			t.Errorf("-%c is not in busyboxLetters", spec.letter)
		}
	}
	for _, letter := range "cis" {
		if !strings.ContainsRune(busyboxLetters, letter) {
			t.Errorf("%c is not in busyboxLetters", letter)
		}
	}
}
