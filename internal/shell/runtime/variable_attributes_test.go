package runtime_test

import "testing"

// `declare -F` names functions: each one asked about that exists, status 1 if one does not,
// and every one as `declare -f name` when none is asked about. bash's transcript, measured.
func TestDeclareF_namesFunctions(t *testing.T) {
	script := "f() { :; }\ng() { :; }\ndeclare -F f\necho \"st=$?\"\ndeclare -F nope\necho \"st=$?\"\ndeclare -F\n"
	want := "f\nst=0\nst=1\ndeclare -f f\ndeclare -f g\n"
	if stdout, _ := runScriptCapturing(script); stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

// `declare -i -l -u` and `+=`. The attributes were refused; `+=` was run as a command called
// `x+=y` and left x as it was. Every answer here is bash's, measured -- busybox has neither.
func TestAttributesAndAppend_answerAsBashDoes(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "+= appends", script: "x=a\nx+=b\necho $x\n", want: "ab\n"},
		{name: "+= on a PATH", script: "p=/bin\np+=\":/opt\"\necho $p\n", want: "/bin:/opt\n"},
		{name: "+= in front of a command", script: "x=1\nx+=2 env | grep '^x='\n", want: "x=12\n"},
		{name: "+= on an exported name reaches the environment", script: "export e=1\ne+=2\nenv | grep '^e='\n", want: "e=12\n"},
		{name: "an element appends", script: "a=(p q)\na[1]+=z\necho ${a[1]}\n", want: "qz\n"},
		{name: "an array's bare name is element zero", script: "a=(p q)\na=z\necho ${a[@]}\n", want: "z q\n"},
		{name: "and appends there too", script: "a=(p)\na+=q\necho ${a[@]}\n", want: "pq\n"},
		{name: "-i evaluates", script: "declare -i n\nn=2+3\necho $n\n", want: "5\n"},
		{name: "-i reads a name", script: "abc=4\ndeclare -i n\nn=abc*2\necho $n\n", want: "8\n"},
		{name: "-i makes += add", script: "declare -i n=5\nn+=3\necho $n\n", want: "8\n"},
		{name: "-i through read and a loop", script: "declare -i n\nread n <<< '1+1'\necho $n\nfor n in 2*3; do echo $n; done\n", want: "2\n6\n"},
		{name: "-l and -u fold", script: "declare -l s=ABC\ns+=DEF\ndeclare -u t=abc\necho $s $t\n", want: "abcdef ABC\n"},
		{name: "-u replaces -l", script: "declare -l s\ndeclare -u s\ns=Mix\necho $s\n", want: "MIX\n"},
		{name: "-l over an array", script: "declare -l a=(A B)\necho \"${a[@]}\"\n", want: "a b\n"},
		{name: "-u through export", script: "declare -u x\nexport x=abc\nenv | grep '^x='\n", want: "x=ABC\n"},
		{name: "+i takes it away", script: "declare -i n=3\ndeclare +i n\nn=1+1\necho $n\n", want: "1+1\n"},
		{name: "declare -p shows them", script: "declare -i n=5\nexport n\nreadonly n\ndeclare -p n\ndeclare -xl q=Z\ndeclare -p q\n", want: "declare -irx n=\"5\"\ndeclare -xl q=\"z\"\n"},
		{name: "declare += appends", script: "declare x+=y\ndeclare x+=z\necho $x\n", want: "yz\n"},
		{name: "a negative subscript counts from the end", script: "a=(x y z)\na[-1]=Z\necho ${a[@]}\n", want: "x y Z\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}
