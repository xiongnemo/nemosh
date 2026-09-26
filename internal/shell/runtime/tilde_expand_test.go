package runtime_test

import "testing"

// Tilde expansion as both references do it: in an assignment after the `=` and after every
// unquoted `:`, export, readonly and local included; in the word of `${u:-~}` and `${x:=~}`;
// with the prefix ending in the text it began in, so `~$y` is left alone; and with HOME
// taken as it stands, empty or with a trailing slash. busybox-w32 and bash 5.3 agree on every
// answer here except two, which are busybox's: bash also expands the second tilde of
// `echo ${u-~:~}`, where outside an assignment `:` does not end the prefix, and the one in
// the argument `a=~`, which is not an assignment.
func TestRuntime_tildeExpansion(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`HOME=/h; x=a:~:b; echo $x; y=q; x=$y:~; echo $x; x=~:~/z:"~"; echo $x`, "a:/h:b\nq:/h\n/h:/h/z:~\n"},
		{`HOME=/h; y=/a; x=~$y; echo $x`, "~/a\n"},
		{`HOME=/h; export x=foo:~; echo $x; readonly y=foo:~; echo $y; f() { local z=foo:~; echo $z; }; f`, "foo:/h\nfoo:/h\nfoo:/h\n"},
		{`HOME=/h; echo ${undef:-~}; echo ${undef:-~/z}; echo "${undef:-~}"; echo ${undef:-"~"}`, "/h\n/h/z\n~\n~\n"},
		{`HOME=/h; x=${undef-~:~}; echo $x; echo ${undef-~:~}`, "/h:/h\n~:~\n"},
		{`HOME=/h; : ${x:=~}; echo $x; : ${y:=a:~}; echo $y`, "/h\na:~\n"},
		{`HOME=/h; printf '[%s]' ${undef:-~/"a b"}`, "[/h/a b]"},
		{`HOME=''; x=~; echo "[$x]"; [[ ~ ]]; echo st=$?`, "[]\nst=1\n"},
		{`HOME=/h/; echo ~/x ~`, "/h//x /h/\n"},
		{`HOME='*'; echo ~; HOME='/a b'; set -- ~ ~/x; echo $#`, "*\n2\n"},
		// Unchanged, and pinned beside the rest.
		{`HOME=/h; echo ~"/x" ~/"x" ~\/x`, "~/x /h/x ~/x\n"},
		{`HOME=/h; echo a:~ a=~ x~`, "a:~ a=~ x~\n"},
		{`HOME=/h; for x in ~ ~/q; do echo $x; done; case /h in ~) echo match;; esac`, "/h\n/h/q\nmatch\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}

// In an array literal a plain element's tilde is a word's, at its start only, and a `[k]=v`
// value's is an assignment's, at its start and after each `:`. A tilde pass over the whole
// literal's text had half-expanded `[k]=~:~:~`. The answers are bash 5.3's; busybox-w32 has
// no arrays.
func TestRuntime_tildeExpansionInArrayLiterals(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`HOME=/h; a=([1]=a=~ [2]=~/x:~ [3]=x~); echo "${a[@]}"`, "a=~ /h/x:/h x~\n"},
		{`HOME=/h; declare -A m=([k]=a=~ [j]=~ [i]=~:~:~); echo "${m[k]} ${m[j]} ${m[i]}"`, "a=~ /h /h:/h:/h\n"},
		{`HOME=/h; b=(~ a=~ x:~ ~/y); echo "${b[@]}"`, "/h a=~ x:~ /h/y\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
