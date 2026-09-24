package runtime_test

import "testing"

// A name a function makes its own -- by `local`, or by `declare` or `typeset` inside it --
// is all the call's: value, array, attributes, read-only, export. Each transcript is bash
// 5.3's, measured, where busybox has no declare or local options; busybox's where it does,
// and those are marked.
func TestLocal_makesTheWholeNameTheCallsOwn(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "local -a", script: "f() { local -a arr=(1 2 3); echo \"${#arr[@]} ${arr[1]}\"; }\nf\necho \"[${arr[*]}]\"\n", want: "3 2\n[]\n"},
		{name: "local -A", script: "f() { local -A m=([k]=v [j]=w); echo \"${m[k]}${m[j]}\"; }\nf\necho \"[${m[k]}]\"\n", want: "vw\n[]\n"},
		{name: "local -i", script: "f() { local -i n=2+3; n+=1; echo $n; }\nf\n", want: "6\n"},
		{name: "local -l", script: "f() { local -l s=ABC; echo $s; }\nf\n", want: "abc\n"},
		{name: "local -r ends with the call", script: "f() { local -r c=1; }\nf\nc=2\necho \"c=$c\"\n", want: "c=2\n"},
		{name: "local -x reaches a child and goes", script: "f() { local -x e=1; env | grep '^e='; }\nf\nenv | grep '^e=' || echo none\n", want: "e=1\nnone\n"},
		{name: "a local array shadows the caller's", script: "a=(1 2)\nf() { local a; a[0]=x; echo \"in [${a[*]}]\"; }\nf\necho \"out [${a[*]}]\"\n", want: "in [x]\nout [1 2]\n"},
		{name: "declare in a function is local", script: "f() { declare d=1; typeset t=2; }\nf\necho \"[$d$t]\"\n", want: "[]\n"},
		{name: "declare -g is the caller's", script: "f() { declare -g g=1; }\nf\necho \"[$g]\"\n", want: "[1]\n"},
		{name: "a second local keeps the value", script: "f() { local x=1; local x; echo \"[$x]\"; }\nf\n", want: "[1]\n"},
		{name: "declare on a local changes it in place", script: "f() { local x=1; declare -i x; x=2+3; echo \"[$x]\"; }\nf\necho \"out [$x]\"\n", want: "[5]\nout []\n"},
		// busybox and bash both.
		{name: "a local over an exported name is exported", script: "export X=1\nf() { local X=2; env | grep '^X='; }\nf\nenv | grep '^X='\n", want: "X=2\nX=1\n"},
		// busybox's answer; bash leaves the caller's value in the environment until then.
		{name: "an unset local exports nothing until assigned", script: "export X=1\nf() { local X; env | grep '^X=' || echo none; X=3; env | grep '^X='; }\nf\nenv | grep '^X='\n", want: "none\nX=3\nX=1\n"},
		{name: "a caller sees its own value again", script: "g() { echo \"g [$v]\"; }\nf() { local v=inner; g; }\nv=outer\nf\necho \"$v\"\n", want: "g [inner]\nouter\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// The operands of export, readonly, local, declare and typeset that are assignments are
// expanded as assignments: not split, not globbed. They were split, so a value with a
// space -- a Windows PATH -- was cut there and the rest taken as another name.
func TestDeclarationUtilities_doNotSplitTheirAssignments(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "each of the five", script: "y='a b'\nf() { local x=$y; declare z=$y; typeset t=$y; echo \"[$x][$z][$t]\"; }\nf\nexport e=$y\nreadonly r=$y\necho \"[$e][$r]\"\n", want: "[a b][a b][a b]\n[a b][a b]\n"},
		{name: "a PATH with a space", script: "P='/a/Program Files/x:/b'\nexport P=$P:/c\nenv | grep '^P='\n", want: "P=/a/Program Files/x:/b:/c\n"},
		{name: "not globbed", script: "y='*'\nexport g=$y\necho \"[$g]\"\n", want: "[*]\n"},
		{name: "a compound value keeps its elements", script: "f() { local -a x=(\"a b\" c); echo ${#x[@]} \"[${x[0]}]\"; }\nf\n", want: "2 [a b]\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// An operand that is not a name ends the script, busybox's answer; it returned 0 and was
// exported or made read-only as written. `readonly X` leaves an unset X unset.
func TestDeclarationUtilities_refuseWhatIsNotAName(t *testing.T) {
	for _, script := range []string{"export 'a/b'\necho after\n", "export 'c/d=1'\necho after\n", "readonly 'e/f'\necho after\n", "f() { local 'a b'; }\nf\necho after\n"} {
		if stdout, status := runScriptCapturing(script); stdout != "" || status != 2 {
			t.Errorf("%q = %d %q, want 2 and nothing after", script, status, stdout)
		}
	}
	if stdout, _ := runScriptCapturing("readonly X\necho \"${X-unset}\"\n"); stdout != "unset\n" {
		t.Errorf("readonly X = %q, want X still unset", stdout)
	}
}

// `${!a[*]}` is every index in one field, as `${a[*]}` is every element. It gave the first.
func TestArrayKeys_starJoinsThemAll(t *testing.T) {
	script := "a=(x y z)\ndeclare -A m=([k]=1)\nm[j]=2\nIFS=,\necho \"${!a[*]}\"\necho \"${#m[@]}\" ${!a[*]}\n"
	if stdout, _ := runScriptCapturing(script); stdout != "0,1,2\n2 0 1 2\n" {
		t.Fatalf("stdout = %q", stdout)
	}
}
