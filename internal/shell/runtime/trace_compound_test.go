package runtime_test

import "testing"

// set -x traces what is not a command name and its words: `(( ))` and an arithmetic for's
// parts as bash does, busybox having neither; `[[ ]]` as busybox does, the command it runs
// it as; and array assignments as bash does. They ran untraced, and `(( ))` was traced as the
// `let` it runs as. Each line was measured in bash 5.3 or busybox-w32.
func TestTrace_compoundCommands(t *testing.T) {
	for _, test := range []struct{ name, script, want string }{
		{name: "(( ))", script: "x=5\n(( $x + 1 ))\n((b=x*2))", want: "+ x=5\n+ ((  5 + 1  ))\n+ (( b=x*2 ))\n"},
		{
			name:   "an arithmetic for's parts, each time",
			script: "for ((  i = 0 ;  i < 1 ; i++  )); do :; done",
			want:   "+ (( i = 0  ))\n+ (( i < 1  ))\n+ :\n+ (( i++   ))\n+ (( i < 1  ))\n",
		},
		{name: "empty parts are 1", script: "for (( ; ; )); do break; done", want: "+ (( 1 ))\n+ (( 1 ))\n+ break\n"},
		{name: "[[ ]], busybox's", script: "x=5\n[[ $x == 5 && -n \"a b\" ]]", want: "+ x=5\n+ '[[' 5 == 5 '&&' -n 'a b' ]]\n"},
		{name: "a list as written", script: "x=5\na=(1 \"2 3\" $x)\na+=( \"d e\" )", want: "+ x=5\n+ a=(1 \"2 3\" $x)\n+ a+=(\"d e\")\n"},
		{name: "an element with its value expanded", script: "a[1]=\"x y\"\na[2]+=z\na[3]=", want: "+ a[1]='x y'\n+ a[2]+=z\n+ a[3]=\n"},
		{name: "a list beside a scalar, a line each", script: "a=(1) b=2", want: "+ a=(1)\n+ b=2\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, stderr := runSetScript(t, "set -x\n"+test.script+"\nset +x\n")
			if want := test.want + "+ set +x\n"; stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
		})
	}
}
