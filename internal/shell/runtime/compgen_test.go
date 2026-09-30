package runtime_test

import (
	"path/filepath"
	"testing"
)

// compgen is bash's, and every answer here was measured in bash 5.3; busybox has no
// programmable completion. It was not a command at all.
func TestCompgen_generatesAsBashDoes(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	setup := "cd '" + dir + "'\nmkdir -p sub\n: > one; : > two; : > three; : > QZ_FILE\n"
	tests := []struct {
		name, script, stdout string
		status               int
	}{
		{name: "files", script: "compgen -f t", stdout: "three\ntwo\n"},
		{name: "directories", script: "compgen -d; compgen -A directory s", stdout: "sub\nsub\n"},
		{name: "a missing directory", script: "compgen -f /nemosh-no-such-dir/", status: 1},
		{name: "variables, a local among them", script: "v1=0\nf() { local v2=0; compgen -v v; }\nf", stdout: "v1\nv2\n"},
		{name: "exported only", script: "export e_one=1 e_two=2; e_three=3; compgen -e e_", stdout: "e_one\ne_two\n"},
		{name: "bash's order of actions", script: "QZ_FUNC() { :; }\nQZ_VAR=1\ncompgen -A file -A variable -A function QZ", stdout: "QZ_FUNC\nQZ_VAR\nQZ_FILE\n"},
		{name: "aliases and set -o names", script: "alias v_alias=ls v_alias2=ls\ncompgen -A alias -A setopt v", stdout: "v_alias\nv_alias2\nverbose\nvi\n"},
		{name: "a prefix and a suffix", script: "compgen -A shopt -P [ -S ] nu", stdout: "[nullglob]\n"},
		{name: "commands", script: "our_func() { :; }\nalias our_alias=x\ncompgen -A command our_; compgen -c eva; compgen -c whil", stdout: "our_alias\nour_func\neval\nwhile\n"},
		{name: "keywords", script: "compgen -k do; compgen -k el", stdout: "do\ndone\nelse\nelif\n"},
		{name: "builtins", script: "compgen -b get", stdout: "getopts\n"},
		{name: "a word list, after the actions", script: "compgen -W 'one two three'; compgen -W 's1 s2 x' -A directory s", stdout: "one\ntwo\nthree\nsub\ns1\ns2\n"},
		{name: "IFS splits the list, a substitution's fields too", script: "IFS=':%'\ncompgen -W '$(echo \"spam:eggs%ham cheese\")'\ncompgen -W 'a:b\\:c d'", stdout: "spam\neggs\nham cheese\na\nb:c d\n"},
		{name: "an empty list is no error", script: "compgen -W '' -- foo", status: 1},
		{name: "a filter, and its negation", script: "compgen -X '@(two|bin)' -W 'one two three bin'\ncompgen -X '!t*' -W 'one two three'", stdout: "one\nthree\ntwo\nthree\n"},
		{name: "& is the word", script: "compgen -X '&c' -W 'ab bc' b", status: 1},
		{name: "a function", script: "fun() { echo \"[$*] $COMP_CWORD\"; COMPREPLY=(x y); }\ncompgen -F fun w 2>/dev/null\necho \"${COMPREPLY-unset} ${COMP_CWORD-unset}\"", stdout: "[compgen w ] -1\nx\ny\nunset unset\n"},
		{name: "a function's scalar reply", script: "g() { COMPREPLY=hello; }\ncompgen -F g 2>/dev/null", stdout: "hello\n"},
		{name: "a command's lines", script: "h() { echo foo; echo bar; }\ncompgen -C h b 2>/dev/null", stdout: "foo\nbar\n"},
		{name: "plusdirs", script: "compgen -o plusdirs -W 'a s1' s", stdout: "s1\nsub\n"},
		{name: "default is files when nothing else matched", script: "compgen -o default t", stdout: "three\ntwo\n"},
		{name: "a glob", script: "compgen -G 't*'", stdout: "three\ntwo\n"},
		{name: "signals", script: "compgen -A signal SIGH; compgen -A signal E", stdout: "SIGHUP\nEXIT\nERR\n"},
		{name: "into an array", script: "compgen -V found -W 'ab ac b' a\necho \"${#found[@]} ${found[1]}\"", stdout: "2 ac\n"},
		{name: "an unknown option", script: "compgen -Z 2>/dev/null", status: 2},
		{name: "an unknown action", script: "compgen -A nosuch 2>/dev/null", status: 2},
		// A shell error in the list fails compgen and discards the rest of its line, as bash's
		// does; the next line runs.
		{name: "an error in the list", script: "compgen -W 'a $(( 1 / 0 ))' 2>/dev/null; echo same\necho \"next $?\"", stdout: "next 1\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, setup+test.script+"\n")

			// Then
			if stdout != test.stdout || status != test.status {
				t.Fatalf("got %q/%d, want %q/%d; stderr = %q", stdout, status, test.stdout, test.status, stderr)
			}
		})
	}
}
