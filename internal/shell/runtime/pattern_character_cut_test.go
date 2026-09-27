package runtime_test

import "testing"

// The trim and replacement operators find a match only between characters, as bash does in a
// UTF-8 locale, so `?` is a whole character there as it is to `case`, `${#v}` and `${v:1}`.
// They stepped a byte at a time, and a character cut in two read as U+FFFD, which `?` and
// `[!μ]` match: `${v#?}` over `μ-` was `\xbc-`. busybox-w32 counts bytes everywhere (its
// `${#v}` there is 3), and nemosh took bash's characters already. A value that is not UTF-8 is
// still cut at any byte, as bash goes back to bytes for one.
func TestRuntime_patternOperatorsCutBetweenCharacters(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{`x=μabcμ; echo "[${x#?abc?}] [${x##?abc?}] [${x%?abc?}] [${x%%?abc?}]"`, "[] [] [] []\n"},
		{`v='μ-'; echo "[${v#?}] [${v##?}]"; v='-μ'; echo "[${v%?}] [${v%%?}]"`, "[-] [-]\n[-] [-]\n"},
		{`x=μμ; echo "[${x#*?}] [${x%?*}] [${x##?*}] [${x%%*?}]"`, "[μ] [μ] [] []\n"},
		{`x=日本語; echo "[${x#?}] [${x%?}] [${x#??}] [${x%??}]"`, "[本語] [日本] [語] [日]\n"},
		{`x=μb; echo "[${x/[!μ]b/X}] [${x/%[!μ]b/X}] [${x//[!μ]/X}] [${x/#?/X}] [${x/%?/X}]"`, "[μb] [μb] [μX] [Xb] [μX]\n"},
		{`shopt -s nocasematch; x=ÄbÄ; echo "[${x/[!Ä]B/X}] [${x//?/X}]"`, "[ÄbÄ] [XXX]\n"},
		{`set -- μ- μ+; echo "${@#?}"`, "- +\n"},
		{`v=$'\xce\xbc\xce'; printf '%s|%s\n' "${v#?}" "${v%?}"`, "\xbc\xce|\xce\xbc\n"},
	} {
		stdout, status := runScriptCapturing(test.script + "\n")
		if stdout != test.want || status != 0 {
			t.Errorf("%d: %s: got %q/%d, want %q/0, as bash answers", index, test.script, stdout, status, test.want)
		}
	}
}
