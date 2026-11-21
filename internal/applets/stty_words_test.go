package applets

import "testing"

// stty's options are -a and -g; a dash begins a setting as often as an option, and -echo is a
// setting, not the options e, c, h and o. getopt read it as those and refused the e, so `stty
// -echo` -- the reason a script calls stty at all, before a password -- failed on a real
// console with "unknown option -- e", where no test had a console to find it.
func TestSttyWords_aDashedSettingIsNoOption(t *testing.T) {
	for _, words := range [][]string{{"-echo"}, {"-a"}, {"-g"}, {"echo", "-echo", "sane"}, {"1f7:7"}} {
		if err := checkSttyWords(words); err != nil {
			t.Errorf("stty %q: %v", words, err)
		}
	}
	for _, words := range [][]string{{"-Z"}, {"-icanon"}, {"-echo", "-x"}} {
		if err := checkSttyWords(words); err == nil {
			t.Errorf("stty %q was taken", words)
		}
	}
}
