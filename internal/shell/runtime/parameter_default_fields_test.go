package runtime_test

import "testing"

// The word of `${name:-word}` and `${name:+word}` is expanded as a word of its own: what it
// quotes stays whole, what it leaves unquoted is split, and "$@" in it is a field per
// parameter. It was expanded to one string, so `${u:-"a b"}` split into two words and
// `"${u:-$@}"` joined into one. busybox-w32 and bash agree on every answer here but two:
// `set --; "${@-x}"`, where busybox has the positional parameters always set and so does
// this, and the last, which is bash's.
func TestRuntime_defaultWordKeepsItsQuotingAndFields(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`set -- "1 2" 3; printf '[%s]' "${undef:-$@}"`, "[1 2][3]"},
		{`set -- "1 2" 3; printf '[%s]' ${undef:-"$@"}`, "[1 2][3]"},
		{`set -- "1 2" 3; x=1; printf '[%s]' "${x:+$@}"`, "[1 2][3]"},
		{`set -- 1 2; printf '[%s]' "${u:-"$@"}"`, "[1][2]"},
		{`set -- "1 2" 3; printf '[%s]' "${undef:-x$@y}"`, "[x1 2][3y]"},
		{`printf '[%s]' ${undef:-"a b"}`, "[a b]"},
		{`printf '[%s]' ${undef:-"a b" c}`, "[a b][c]"},
		{`printf '[%s]' ${u:-'a b'}`, "[a b]"},
		{`y='p q'; printf '[%s]' ${u:-"$y"}`, "[p q]"},
		{`printf '[%s]' ${u:-a\ b}`, "[a b]"},
		{`printf '[%s]' ${u:-${v:-"a b"}}`, "[a b]"},
		{`set -- a b; printf '[%s]' "${@:-x}"`, "[a][b]"},
		// Unchanged, and pinned beside the rest.
		{`printf '[%s]' "${undef:-a b}"`, "[a b]"},
		{`printf '[%s]' ${undef:-a b}`, "[a][b]"},
		{`set -- "1 2" 3; printf '[%s]' ${undef:-$@}`, "[1][2][3]"},
		{`set --; set -- "${undef:-$@}"; echo $#`, "1\n"},
		{`set --; set -- ${undef:-"$@"}; echo $#`, "0\n"},
		{`set -- a b; printf '[%s]' "${undef:-$*}"`, "[a b]"},
		{`IFS=:; printf '[%s]' ${undef:-a b} ${undef:-a:b}`, "[a b][a][b]"},
		{`printf '[%s]' "${u:-'a b'}"`, "['a b']"},
		{`printf '[%s]' "${u:-a\ b}"`, `[a\ b]`},
		{`printf '[%s]' ${u:-$(echo a b)}`, "[a][b]"},
		{`printf '[%s]' "${u:-}"; echo; set -- ${u:-}; echo $#`, "[]\n0\n"},
		{`x=; printf '[%s]' "${x:+y}"; echo; set -- ${x:+y}; echo $#`, "[]\n0\n"},
		{`set -- ""; echo "[${@-x}]" "[${@:-x}]"`, "[] [x]\n"},
		{`set -- "" ""; echo "[${@-x}]" "[${@:-x}]"`, "[ ] [ ]\n"},
		{`set --; echo "[${@-x}]" "[${@:-x}]"`, "[] [x]\n"},
		// Under an empty IFS a quoted `*` form of empty parameters joins to nothing, and
		// an unquoted one is two empty words, which is something.
		{`IFS=; set -- "" ""; echo "[${*:-x}]" "[${*:+y}]" "[${@:-x}]"`, "[x] [] [ ]\n"},
		{`IFS=; set -- "" ""; set -- ${*:-x}; echo $#`, "0\n"},
		// bash's answer: busybox gives no word here, while it takes the same parameters as
		// something for `${*:-x}` above.
		{`IFS=; set -- "" ""; set -- ${*:+y}; echo $# "$1"`, "1 y\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}

// The same for an array, whose answers are bash 5.3's; busybox-w32 has no arrays. The value,
// when it is the array's own, is a field per element, and the word may be a list too.
func TestRuntime_arrayDefaultIsAFieldPerElement(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`a=(1 "2 3"); printf '[%s]' "${a[@]:-x}"`, "[1][2 3]"},
		{`default=('1 2' 3); printf '[%s]' "${undef[@]:-${default[@]}}"`, "[1 2][3]"},
		{`a=(); echo "[${a[@]-x}]" "[${a[@]:-x}]"`, "[x] [x]\n"},
		{`a=(""); echo "[${a[@]-x}]" "[${a[@]:-x}]"`, "[] [x]\n"},
		{`a=("" ""); echo "[${a[@]-x}]" "[${a[@]:-x}]"`, "[ ] [ ]\n"},
		{`a=(1 "2 3"); printf '[%s]' ${a[@]:-x}`, "[1][2][3]"},
		{`a=(1 "2 3"); printf '[%s]' "${a[@]:+y}"`, "[y]"},
		{`a=(1 "2 3"); printf '[%s]' "${a[*]:-x}"`, "[1 2 3]"},
		{`IFS=; a=("" ""); echo "[${a[*]:-x}]" "[${a[*]:+y}]"; set -- ${a[*]:-x}; echo $#`, "[x] []\n0\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
