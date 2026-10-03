package runtime_test

import "testing"

// complete keeps a specification per command and prints it back as bash does, so a script's
// `complete -p` output reads in again; compopt changes one's -o options. They were "not found",
// so a tool's completion script stopped at its first line. Each line is bash 5.3's, but for the
// order of `complete -p`, which is the order of bash's hash table there and sorted here.
func TestComplete_keepsPrintsAndRemovesSpecifications(t *testing.T) {
	script := `complete -o default -F __start_gh gh
complete -W 'a b "c d"' -P pre -S suf -X '!*a*' -A function -abcv x y
complete -o nospace -o filenames -G '*.txt' -C 'my cmd' 'odd name'
complete -D -F _default
complete -E -W e
complete -p
complete -p gh nosuch; echo "st=$?"
compopt gh
compopt -o nospace +o default gh
complete -p gh
complete -r x nosuch; echo "st=$?"
complete -r
complete; echo "st=$?"
`
	want := `complete -F _default -D
complete -W 'e' -E
complete -o default -F __start_gh gh
complete -o filenames -o nospace -G '*.txt' -C 'my cmd' 'odd name'
complete -a -b -c -v -A function -W 'a b "c d"' -P 'pre' -S 'suf' -X '!*a*' x
complete -a -b -c -v -A function -W 'a b "c d"' -P 'pre' -S 'suf' -X '!*a*' y
complete -o default -F __start_gh gh
st=1
compopt +o bashdefault -o default +o dirnames +o filenames +o fullquote +o noquote +o nosort +o nospace +o plusdirs gh
complete -o nospace -F __start_gh gh
st=1
st=0
`
	wantErr := "nemosh: line 7: complete: nosuch: no completion specification\n" +
		"nemosh: line 11: complete: nosuch: no completion specification\n"
	if status, stdout, stderr := runSetScript(t, script); status != 0 || stdout != want || stderr != wantErr {
		t.Errorf("got %d\n%s\nstderr %q\nwant\n%s\nstderr %q", status, stdout, stderr, want, wantErr)
	}
}

// What complete and compopt refuse, in bash's words and with its statuses: compopt with no name
// outside a completion, a definition with no name, a function name that is no name, an option
// name neither knows.
func TestComplete_refusesWhatBashRefuses(t *testing.T) {
	for _, test := range []struct{ script, stdout, stderr string }{
		{"compopt -o nosort; echo st=$?", "st=1\n", "nemosh: line 1: compopt: not currently executing completion function\n"},
		{"complete -o default; echo st=$?", "st=2\n", "complete: usage: complete [-abcdefgjksuv] [-pr] [-DEI] [-o option] [-A action] [-G globpat] [-W wordlist] [-F function] [-C command] [-X filterpat] [-P prefix] [-S suffix] [name ...]\n"},
		{"complete -F 'a b' z; echo st=$?", "st=2\n", "nemosh: line 1: complete: `a b': not a valid identifier\n"},
		{"complete -o nope z; echo st=$?", "st=2\n", "nemosh: line 1: complete: nope: invalid option name\n"},
		{"compopt -o nope z; echo st=$?", "st=2\n", "nemosh: line 1: compopt: nope: invalid option name\n"},
	} {
		if status, stdout, stderr := runSetScript(t, test.script+"\n"); status != 0 || stdout != test.stdout || stderr != test.stderr {
			t.Errorf("%q: got %d/%q/%q, want 0/%q/%q", test.script, status, stdout, stderr, test.stdout, test.stderr)
		}
	}
}
