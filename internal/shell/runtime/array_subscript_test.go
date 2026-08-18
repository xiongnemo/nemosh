package runtime_test

import "testing"

// A subscript is an expression, not a literal. `${a[$i]}` gave the empty string
// where bash gives the element, because the text went to strconv.Atoi, failed, and a
// failed conversion was treated as out of range -- which is empty. Indexing an array
// by a loop variable is most of what an array is for, and it silently produced
// nothing.
//
// And arithmetic saw the text before expansion, so `${#a[@]}` in a condition
// reported `unexpected "$"`. Between them they ruled out the standard way to walk an
// array.
func TestArraySubscript_isAnExpression(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "a parameter with a dollar", script: "a=(x y z)\ni=1\necho ${a[$i]}\n", want: "y\n"},
		{name: "a bare name", script: "a=(x y z)\ni=1\necho ${a[i]}\n", want: "y\n"},
		{name: "arithmetic", script: "a=(x y z)\necho ${a[1+1]}\n", want: "z\n"},
		{name: "arithmetic on a name", script: "a=(x y z)\ni=1\necho ${a[i+1]}\n", want: "z\n"},
		{name: "a literal is unchanged", script: "a=(x y z)\necho ${a[2]}\n", want: "z\n"},
		{name: "out of range is still empty", script: "a=(x)\necho [${a[9]}]\n", want: "[]\n"},
		{name: "assignment with a parameter", script: "a=(x y z)\ni=1\na[$i]=C\necho ${a[@]}\n", want: "x C z\n"},
		{name: "assignment with arithmetic", script: "a=(x y z)\na[1+1]=Q\necho ${a[@]}\n", want: "x y Q\n"},
		{
			// The idiom this exists for.
			name:   "walking an array",
			script: "a=(p q r)\nfor ((i=0;i<${#a[@]};i++)); do printf %s \"${a[$i]}\"; done\necho\n",
			want:   "pqr\n",
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

// Arithmetic expands its text first, which the evaluator's own lexer cannot do: it
// reads a bare name, so the spelling without a dollar worked and the one with it did
// not. Both are common, which is how this went unnoticed.
func TestArithmetic_expandsParametersFirst(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "a dollar name", script: "i=3\necho $(( $i * 2 ))\n", want: "6\n"},
		{name: "a braced name", script: "i=3\necho $(( ${i} + 1 ))\n", want: "4\n"},
		{name: "an array count", script: "a=(x y z)\necho $(( ${#a[@]} + 1 ))\n", want: "4\n"},
		{name: "an array element", script: "a=(1 2 3)\necho $(( ${a[1]} + 10 ))\n", want: "12\n"},
		{name: "a positional parameter", script: "set -- 7\necho $(( $1 * 3 ))\n", want: "21\n"},
		{name: "a bare name still works", script: "i=4\necho $(( i + 1 ))\n", want: "5\n"},
		{name: "no parameters at all", script: "echo $(( 2 * 3 ))\n", want: "6\n"},
		{name: "an assignment inside", script: "echo $(( x = 5 ))\necho $x\n", want: "5\n5\n"},
		{name: "a string length", script: "s=abcd\necho $(( ${#s} * 2 ))\n", want: "8\n"},
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
