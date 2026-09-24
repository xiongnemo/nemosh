package runtime_test

import "testing"

// The quotes in an operator's word. Every row is busybox-w32's answer, measured, and bash
// gives the same; the quotes were left in the word, so a quoted default kept its quotes and
// a quoted pattern matched nothing.
func TestOperandQuoting_followsTheReferences(t *testing.T) {
	setup := "y=Y; s=abc; star='a*bc'; p='*'\n"
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "a quoted default", script: `echo [${x:-"a b"}] ["${x:-"a b"}"]`, want: "[a b] [a b]"},
		{name: "single quotes in a default", script: `echo [${x:-'l $y'}] ["${x:-'l $y'}"]`, want: "[l $y] ['l Y']"},
		{name: "expansions in a quoted default", script: `echo [${x:-"p $y"}] ["${x:-p "$y"}"]`, want: "[p Y] [p Y]"},
		{name: "backslashes in a default", script: `echo ["${x:-a\"b}"] [${x:-a\"b}] ["${x:-a\\b}"]`, want: `[a"b] [a"b] [a\b]`},
		{name: "a quoted prefix", script: `echo [${s#"a"}] ["${s#"a"}"] [${s#'a'}] ["${s#'a'}"]`, want: "[bc] [bc] [bc] [bc]"},
		{name: "quoting makes a star literal", script: `echo [${star#"a*"}] ["${star#"a*"}"] [${star#a*}] ["${star#a*}"]`, want: "[bc] [bc] [*bc] [*bc]"},
		{name: "a quoted expansion is literal, an unquoted one a pattern", script: `echo [${star#"$p"}] ["${star#$p}"] ["${star#"$p"}"]`, want: "[a*bc] [a*bc] [a*bc]"},
		{name: "a replacement", script: `echo [${s/"b"/"X Y"}] ["${s/b/'q'}"] [${s/b/'q'}]`, want: "[aX Yc] [aqc] [aqc]"},
		{name: "an alternative", script: `echo [${x:+"set"}] [${s:+"set $y"}] ["${s:+'k'}"]`, want: "[] [set Y] ['k']"},
		{name: "a quoted suffix", script: `echo [${s%"c"}] ["${s%%"bc"}"] [${s%\c}]`, want: "[ab] [a] [ab]"},
		{name: "the trim idiom", script: `str="  trim me  "; t="${str#"${str%%[![:space:]]*}"}"; t="${t%"${t##*[![:space:]]}"}"; echo "[$t]"`, want: "[trim me]"},
		{name: "a trailing slash", script: `f=/x/y/; echo "${f%"/"}"`, want: "/x/y"},
		{name: "over a list", script: `a=(pp qq); echo "${a[@]/p/"Z"}" "${a[@]#"p"}"`, want: "Zp qq p qq"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(setup + test.script + "\n"); stdout != test.want+"\n" {
				t.Fatalf("stdout = %q, want %q", stdout, test.want+"\n")
			}
		})
	}
}

// An assignment-only command with an array after a scalar runs them in order. The array
// pass stopped at the scalar, so the array was assigned as text. bash's answers, measured.
func TestMixedAssignments_runInOrder(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{script: "x=1 arr=(a b); echo \"${arr[1]} $x\"\n", want: "b 1\n"},
		{script: "IFS=, arr=($(echo \"p,q\")); echo \"${#arr[@]}\"\n", want: "2\n"},
		{script: "x=5 arr=($x $x); echo \"${arr[*]}\"\n", want: "5 5\n"},
		{script: "a=(3 1 2); IFS=$'\\n' sorted=($(sort <<<\"${a[*]}\")); unset IFS; echo \"${sorted[*]}\"\n", want: "1 2 3\n"},
	} {
		if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
			t.Errorf("%q = %q, want %q", test.script, stdout, test.want)
		}
	}
}

// `${!prefix@}` is a field per name and `${!prefix*}` one word joined as `$*` is, arrays
// included. Both were one word joined with a blank. bash's answers, measured.
func TestNamePrefix_listsTheNames(t *testing.T) {
	script := "ab1=1 ab2=2 abc=3; abz=(x)\nprintf '[%s]' \"${!ab@}\"; echo\nIFS=,; echo \"${!ab*}\"\nfor v in ${!ab@}; do echo \"v=$v\"; done\necho \"[${!zz@}]\"\n"
	want := "[ab1][ab2][abc][abz]\nab1,ab2,abc,abz\nv=ab1\nv=ab2\nv=abc\nv=abz\n[]\n"
	if stdout, _ := runScriptCapturing(script); stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}
