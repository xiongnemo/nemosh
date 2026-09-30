package runtime_test

import "testing"

// `[k]+=v` in a compound assignment appends to the element, as `+=` does to a variable: to
// what an indexed element holds by then, and to what an associative key held before a new
// assignment, which bash writes apart from the old array. An integer array adds. It was the
// text `[k]+=v`, an element of its own. A negative subscript counts back from the end of the
// array as it stands by then, where it was refused. The answers are bash 5.3's; busybox has no
// arrays.
func TestCompoundAssignment_appendsToAnElement(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"hello=100; d=([hello]=1 [hello]+=2); declare -p d; d+=([hello]+=:34 [hello]+=:56); declare -p d\n",
			"declare -a d=([100]=\"12\")\ndeclare -a d=([100]=\"12:34:56\")\n"},
		{"declare -A e; e=([hello]=1 [hello]+=2); declare -p e; e+=([hello]+=:34 [hello]+=:56); declare -p e\n",
			"declare -A e=([hello]=\"2\" )\ndeclare -A e=([hello]=\"2:34:56\" )\n"},
		{"declare -A f=([x]=old); f=([x]+=new [y]=1); declare -p f\n", "declare -A f=([y]=\"1\" [x]=\"oldnew\" )\n"},
		{"declare -ai g=(1 2); g=([0]+=5 [1]+=3); declare -p g; g+=([0]+=10); declare -p g\n",
			"declare -ai g=([0]=\"5\" [1]=\"3\")\ndeclare -ai g=([0]=\"15\" [1]=\"3\")\n"},
		{"declare -Ai h=([k]=1); h+=([k]+=4); declare -p h; h=([k]+=7); declare -p h\n",
			"declare -Ai h=([k]=\"5\" )\ndeclare -Ai h=([k]=\"12\" )\n"},
		{"i=(a b c); i=([1]+=x [5]+=y z); declare -p i\n", "declare -a i=([1]=\"x\" [5]=\"y\" [6]=\"z\")\n"},
		{"j=(a b c); j+=([1]+=x z); declare -p j\n", "declare -a j=([0]=\"a\" [1]=\"bx\" [2]=\"z\")\n"},
		{"n=(a b); n+=([-1]+=x); declare -p n\n", "declare -a n=([0]=\"a\" [1]=\"bx\")\n"},
		{"m=(a b c); m=([5]=y [-1]=x); declare -p m\n", "declare -a m=([5]=\"x\")\n"},
		{"m=(a b c); m+=([-1]=x); declare -p m\n", "declare -a m=([0]=\"a\" [1]=\"b\" [2]=\"x\")\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}

// An associative list whose first word does not open with `[` is keys and values in turn, the
// whole of it, as bash reads it: a word written `[k]=v` is that text. Each word is expanded
// whole, with a tilde at its start -- not split, globbed or brace-expanded, which made it as
// many words as it came to. An empty key is said and passed over.
func TestCompoundAssignment_readsKeysAndValuesInTurn(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"declare -A mix=(1 2 [j]=3); declare -p mix\n", "declare -A mix=([1]=\"2\" [\"[j]=3\"]=\"\" )\n"},
		{"declare -A k=([a]=1); k+=(x y); declare -p k\n", "declare -A k=([x]=\"y\" [a]=\"1\" )\n"},
		{"declare -A l=([a]=1); l=(x y [z]=w); declare -p l\n", "declare -A l=([\"[z]=w\"]=\"\" [x]=\"y\" )\n"},
		{"list=\"a b\"; declare -A m=($list x); declare -p m\n", "declare -A m=([\"a b\"]=\"x\" )\n"},
		{"declare -A g=(*.sh v); declare -p g\n", "declare -A g=([\"*.sh\"]=\"v\" )\n"},
		{"declare -A br=({a,b} c); declare -p br\n", "declare -A br=([\"{a,b}\"]=\"c\" )\n"},
		{"set -- p1 p2; declare -A at=(\"$@\"); declare -p at\n", "declare -A at=([\"p1 p2\"]=\"\" )\n"},
		{"HOME=/h; declare -A t=(~ v ~/x w a:~ y); declare -p t\n", "declare -A t=([\"a:~\"]=\"y\" [/h]=\"v\" [/h/x]=\"w\" )\n"},
		{"HOME=/h; declare -A t=([~]=v [a]=~ [b]=x:~); declare -p t\n", "declare -A t=([b]=\"x:/h\" [a]=\"/h\" [/h]=\"v\" )\n"},
		{"declare -A e=(\"\" v k w); echo \"st=$?\"; declare -p e\n", "st=0\ndeclare -A e=([k]=\"w\" )\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}

// An element a compound assignment cannot write abandons the command, status 1, from declare
// and local too, as bash has it: a subscript before the front of the array, a word without a
// key in a keyed associative list, an empty key. The elements before it stay, and the words
// after it are not expanded. Each went on to the next element, or took the word as a key.
func TestCompoundAssignment_refusedElementAbandonsTheCommand(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"declare -A a; a=([j]=1 2 3 4); echo \"status=$?\"\necho \"next $?\"; declare -p a\n", "next 1\ndeclare -A a=([j]=\"1\" )\n"},
		{"m=(a b c); m=([-1]=x); echo \"st=$?\"\necho \"next $?\"; declare -p m\n", "next 1\ndeclare -a m=()\n"},
		{"declare -a m=([-1]=x); echo \"st=$?\"\necho \"next $?\"\n", "next 1\n"},
		{"f() { local -a m=([-1]=x); echo in; }; f; echo after\necho next\n", "next\n"},
		{"declare -A e3; e3=([a]=1 2 [b]=${z:=set})\necho \"[$z]\"; declare -p e3\n", "[]\ndeclare -A e3=([a]=\"1\" )\n"},
		{"declare -A e4; e4=([\"\"]=1 [b]=${z:=set})\necho \"[$z]\"; declare -p e4\n", "[]\ndeclare -A e4=()\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
