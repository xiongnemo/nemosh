package runtime_test

import (
	"strings"
	"testing"
)

// `declare` was not a builtin at all, so `declare -A m` failed with
// `declare: not found` -- and with it went associative arrays, because there is no
// other way to say a name is one.
//
// Every expectation is measured against bash.
func TestDeclare_associativeArrays(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "set and read", script: "declare -A m\nm[k]=v\necho ${m[k]}\n", want: "v\n"},
		{
			name: "two keys", script: "declare -A m\nm[k]=v\nm[o]=2\necho \"${m[k]} ${m[o]}\"\n", want: "v 2\n",
		},
		{name: "count", script: "declare -A m\nm[a]=1\nm[b]=2\necho ${#m[@]}\n", want: "2\n"},
		{name: "keys", script: "declare -A m\nm[a]=1\nm[b]=2\necho ${!m[@]}\n", want: "a b\n"},
		{name: "values", script: "declare -A m\nm[a]=1\nm[b]=2\necho ${m[@]}\n", want: "1 2\n"},
		{name: "overwriting a key", script: "declare -A m\nm[k]=1\nm[k]=2\necho ${m[k]}\n", want: "2\n"},
		{
			// A key that was overwritten keeps its place, so the listing is stable.
			name:   "a rewritten key keeps its position",
			script: "declare -A m\nm[a]=1\nm[b]=2\nm[a]=3\necho ${!m[@]}\n", want: "a b\n",
		},
		{name: "a missing key is empty", script: "declare -A m\necho [${m[nope]}]\n", want: "[]\n"},
		{name: "an empty one", script: "declare -A e\necho ${#e[@]}\n", want: "0\n"},
		{name: "a key from a variable", script: "declare -A m\nk=abc\nm[$k]=found\necho ${m[$k]}\n", want: "found\n"},
		{
			// The word must not field split, or the key becomes two words and the
			// second is run as a command -- which is what happened before
			// isElementAssignmentWord.
			name:   "a key holding a blank",
			script: "declare -A m\nk=\"two words\"\nm[$k]=sp\necho [${m[$k]}]\n", want: "[sp]\n",
		},
		{
			// A key is a string, not arithmetic: `m[k]` is the key `k`, not the value
			// of a variable called k.
			name:   "a key is not evaluated as arithmetic",
			script: "declare -A m\nk=99\nm[k]=literal\necho ${m[k]}\n", want: "literal\n",
		},
		{
			name:   "walking the keys",
			script: "declare -A m\nm[a]=1\nm[b]=2\nfor k in \"${!m[@]}\"; do printf '%s=%s ' \"$k\" \"${m[$k]}\"; done\necho\n",
			want:   "a=1 b=2 \n",
		},
		{name: "typeset is the same builtin", script: "typeset -A t\nt[z]=9\necho ${t[z]}\n", want: "9\n"},
		{
			// Declaring twice must not empty it.
			name: "declaring twice keeps the contents", script: "declare -A m\nm[k]=v\ndeclare -A m\necho ${m[k]}\n",
			want: "v\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if strings.Contains(stderr, "not found") {
				t.Fatalf("something was run that should not have been: %q", stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}

func TestDeclare_otherForms(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "a plain scalar", script: "declare plain=5\necho $plain\n", want: "5\n"},
		{name: "a name with no value", script: "declare bare\necho [$bare]\n", want: "[]\n"},
		{
			// The lexer keeps the parenthesised list in one word, so it arrives whole
			// and has to be split into elements here.
			name: "an indexed array with a list", script: "declare -a idx=(x y)\necho \"${idx[1]} ${#idx[@]}\"\n",
			want: "y 2\n",
		},
		{name: "an empty indexed array", script: "declare -a e=()\necho ${#e[@]}\n", want: "0\n"},
		{name: "declaring indexed then assigning", script: "declare -a a\na[1]=q\necho ${a[1]}\n", want: "q\n"},
		{name: "-p prints an associative array", script: "declare -A m\nm[a]=1\ndeclare -p m\n", want: "declare -A m=([a]=\"1\")\n"},
		{name: "-p prints an indexed array", script: "declare -a a=(x)\ndeclare -p a\n", want: "declare -a a=([0]=\"x\")\n"},
		{name: "-p prints a scalar", script: "declare s=v\ndeclare -p s\n", want: "declare -- s=\"v\"\n"},
		{name: "-x exports", script: "declare -x E=1\nenv | grep '^E=1$'\n", want: "E=1\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}

// -r goes to the same set `readonly` writes, so a name made read-only either way is
// refused by the one check that matters.
func TestDeclare_readonlyIsEnforced(t *testing.T) {
	// When
	status, _, stderr := runSetScript(t, "declare -r ro=1\nro=2\n")

	// Then
	if status == 0 {
		t.Fatalf("status = 0, want a failure; stderr = %q", stderr)
	}
	if !strings.Contains(stderr, "readonly") {
		t.Fatalf("stderr = %q, want it to name the readonly variable", stderr)
	}
}

// An option this build cannot honour is refused by name. An ignored `-i` would leave a
// variable that is not an integer and a script that believes it is.
func TestDeclare_refusesWhatItCannotHonour(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		fragment string
	}{
		{name: "integer", script: "declare -i n=1\n", fragment: "not an option this build has"},
		{name: "nameref", script: "declare -n ref=x\n", fragment: "not an option this build has"},
		{name: "both kinds at once", script: "declare -Aa m\n", fragment: "cannot both be given"},
		{name: "not a name", script: "declare 9bad=1\n", fragment: "not a valid name"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, _, stderr := runSetScript(t, test.script)

			// Then
			if status == 0 {
				t.Fatalf("status = 0, want a failure")
			}
			if !strings.Contains(stderr, test.fragment) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, test.fragment)
			}
		})
	}
}

// Indexed arrays keep working, and the two kinds stay apart: an indexed subscript is
// arithmetic and an associative one is a string.
func TestDeclare_leavesIndexedArraysAlone(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "a literal", script: "a=(p q)\necho ${a[1]}\n", want: "q\n"},
		{name: "an arithmetic subscript", script: "a=(p q r)\necho ${a[1+1]}\n", want: "r\n"},
		{name: "subscripts are numbers", script: "a=(p q)\necho ${!a[@]}\n", want: "0 1\n"},
		{name: "appending", script: "a=(p)\na+=(q)\necho ${#a[@]}\n", want: "2\n"},
		{name: "a subshell still inherits", script: "a=(p q)\n(echo ${a[1]})\n", want: "q\n"},
		{
			// An associative array in a subshell: inherited, and a write inside does
			// not escape.
			name:   "an associative array in a subshell",
			script: "declare -A m\nm[k]=v\n(echo ${m[k]}; m[k]=changed)\necho ${m[k]}\n", want: "v\nv\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}
