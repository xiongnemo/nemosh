package runtime_test

import "testing"

// An indexed array in bash is sparse. `a=(p); a[3]=z` has two elements, not four.
//
// This store kept a dense slice and grew it with empty strings, so a gap became a real
// element: `"${a[@]}"` produced `p`, “, “, `z` and a loop over it ran four times.
// The count and the subscript list were wrong the same way. All three are measured
// against bash below.
func TestSparseArrays_countOnlyWhatIsSet(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "the count", script: "a=(p)\na[3]=z\necho ${#a[@]}\n", want: "2\n"},
		{name: "the subscripts", script: "a=(p)\na[3]=z\necho ${!a[@]}\n", want: "0 3\n"},
		{name: "the values", script: "a=(p)\na[3]=z\necho \"[${a[@]}]\"\n", want: "[p z]\n"},
		{
			// The one that would break a script: a loop must run twice, not four times.
			name:   "a loop sees no phantom elements",
			script: "a=(p)\na[3]=z\nfor e in \"${a[@]}\"; do printf '<%s>' \"$e\"; done\necho\n",
			want:   "<p><z>\n",
		},
		{name: "a gap reads as empty", script: "a=(p)\na[3]=z\necho \"[${a[1]}]\"\n", want: "[]\n"},
		{name: "filling a gap", script: "a=(p)\na[3]=z\na[1]=q\necho \"${#a[@]} ${a[@]}\"\n", want: "3 p q z\n"},
		{name: "a far gap", script: "a[10]=x\necho \"${#a[@]} ${!a[@]}\"\n", want: "1 10\n"},
		// Dense arrays are unaffected, which is most of them.
		{name: "a dense array", script: "a=(p q r)\necho \"${#a[@]} ${!a[@]} ${a[@]}\"\n", want: "3 0 1 2 p q r\n"},
		{name: "appending stays dense", script: "a=(p)\na+=(q)\necho \"${#a[@]} ${a[@]}\"\n", want: "2 p q\n"},
		{name: "an empty array", script: "a=()\necho ${#a[@]}\n", want: "0\n"},
		{name: "one element", script: "a=(only)\necho \"${#a[@]} ${a[0]}\"\n", want: "1 only\n"},
		{name: "an element containing a blank", script: "a=(p \"q r\")\necho ${#a[@]}\n", want: "2\n"},
		{
			// A reassignment replaces the whole array, gaps included.
			name: "reassigning clears the gaps", script: "a=(p)\na[5]=x\na=(one two)\necho \"${#a[@]} ${!a[@]}\"\n",
			want: "2 0 1\n",
		},
		{
			name:   "a sparse array survives a subshell",
			script: "a=(p)\na[3]=z\n(echo \"${#a[@]} ${!a[@]}\")\n", want: "2 0 3\n",
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

// A negative subscript counts from the end, which is bash's rule and the usual way to
// reach the last element.
func TestSparseArrays_negativeSubscriptsCountFromTheEnd(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "the last element", script: "a=(p q r)\necho ${a[-1]}\n", want: "r\n"},
		{name: "the one before it", script: "a=(p q r)\necho ${a[-2]}\n", want: "q\n"},
		{name: "the first, counted back", script: "a=(p q r)\necho ${a[-3]}\n", want: "p\n"},
		{name: "assigning to the last", script: "a=(p q r)\na[-1]=Z\necho \"${a[@]}\"\n", want: "p q Z\n"},
		{name: "assigning to the first, counted back", script: "a=(p q r)\na[-3]=A\necho \"${a[@]}\"\n", want: "A q r\n"},
		{name: "past the start reads empty", script: "a=(p q r)\necho \"[${a[-9]}]\"\n", want: "[]\n"},
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

// Assigning past the start is refused rather than wrapped, because wrapping would write
// to the first element and look like it worked.
func TestSparseArrays_refusesAnAssignmentPastTheStart(t *testing.T) {
	// When
	_, stdout, stderr := runSetScript(t, "a=(p q r)\na[-9]=x\necho \"${a[@]}\"\n")

	// Then
	if want := "p q r\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q -- the array must be untouched", stdout, want)
	}
	if !contains(stderr, "bad array subscript") {
		t.Fatalf("stderr = %q, want it to say the subscript is bad", stderr)
	}
}
