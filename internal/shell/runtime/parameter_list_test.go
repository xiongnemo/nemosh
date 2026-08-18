package runtime_test

import "testing"

// An operator applied to a *list* rather than to a string.
//
// `${@:2:2}` joined the positional parameters and took a substring of that, so
// `set -- a b c d` gave `b ` where bash gives `b c`. The array forms found no variable
// called `a[@]` and gave nothing at all. Slicing a list is how a script drops its first
// argument or takes a window, so both were answers a script would act on.
//
// Every case measured against bash; the whole file below was run through both shells
// and the output diffed byte for byte.
func TestParameterList_slicesAndMapsOverElements(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		// Positional parameters. The offset counts a list whose first entry is $0,
		// which is why ${@:1} is the first argument.
		{name: "all arguments", script: "set -- a b c d\necho \"[${@:1}]\"\n", want: "[a b c d]\n"},
		{name: "a window", script: "set -- a b c d\necho \"[${@:2:2}]\"\n", want: "[b c]\n"},
		{name: "from an offset", script: "set -- a b c d\necho \"[${@:2}]\"\n", want: "[b c d]\n"},
		{name: "the last two", script: "set -- a b c d\necho \"[${@: -2}]\"\n", want: "[c d]\n"},
		{
			// The star form joins into one word, which is what makes it the star form.
			name: "the star form joins", script: "set -- a b c d\necho \"[${*:2:2}]\"\n", want: "[b c]\n",
		},
		// Arrays, zero-based.
		{name: "an array window", script: "a=(p q r s)\necho \"[${a[@]:1:2}]\"\n", want: "[q r]\n"},
		{name: "an array from an offset", script: "a=(p q r s)\necho \"[${a[@]:1}]\"\n", want: "[q r s]\n"},
		{name: "an array's last two", script: "a=(p q r s)\necho \"[${a[@]: -2}]\"\n", want: "[r s]\n"},
		{name: "an array star window", script: "a=(p q r s)\necho \"[${a[*]:1:2}]\"\n", want: "[q r]\n"},
		// Per-element operators.
		{name: "replace in each", script: "b=(ax bx cx)\necho \"[${b[@]/x/y}]\"\n", want: "[ay by cy]\n"},
		{name: "replace all in each", script: "b=(axx bxx)\necho \"[${b[@]//x/y}]\"\n", want: "[ayy byy]\n"},
		{name: "trim a suffix from each", script: "b=(ax bx cx)\necho \"[${b[@]%x}]\"\n", want: "[a b c]\n"},
		{name: "trim a prefix from each", script: "b=(xa xb)\necho \"[${b[@]#x}]\"\n", want: "[a b]\n"},
		{name: "upper each", script: "c=(ab cd)\necho \"[${c[@]^^}]\"\n", want: "[AB CD]\n"},
		{name: "lower each", script: "c=(AB CD)\necho \"[${c[@],,}]\"\n", want: "[ab cd]\n"},
		{name: "on the positional parameters", script: "set -- ax bx\necho \"[${@/x/y}]\"\n", want: "[ay by]\n"},
		// The forms that already worked, so the new path cannot cost them.
		{name: "a whole array", script: "a=(p q r s)\necho \"[${a[@]}]\"\n", want: "[p q r s]\n"},
		{name: "a count", script: "a=(p q r s)\necho \"[${#a[@]}]\"\n", want: "[4]\n"},
		{name: "the subscripts", script: "a=(p q)\necho \"[${!a[@]}]\"\n", want: "[0 1]\n"},
		{name: "one element", script: "a=(p q)\necho \"[${a[1]}]\"\n", want: "[q]\n"},
		{name: "a string substring", script: "x=abcdef\necho \"[${x:1:3}]\"\n", want: "[bcd]\n"},
		{name: "a string replace", script: "x=axb\necho \"[${x/x/y}]\"\n", want: "[ayb]\n"},
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

// A slice keeps its elements separate, so an element containing a blank survives as one
// word. That is the property arrays exist for, and a slice that lost it would be worse
// than no slice.
func TestParameterList_keepsElementsSeparate(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "an array slice",
			script: "d=(one \"two words\" three)\nfor e in \"${d[@]:1:2}\"; do printf '<%s>' \"$e\"; done\necho\n",
			want:   "<two words><three>\n",
		},
		{
			name:   "a positional slice",
			script: "set -- one \"two words\" three\nfor e in \"${@:2:2}\"; do printf '<%s>' \"$e\"; done\necho\n",
			want:   "<two words><three>\n",
		},
		{
			name:   "a mapped array",
			script: "d=(\"a x\" \"b x\")\nfor e in \"${d[@]/x/y}\"; do printf '<%s>' \"$e\"; done\necho\n",
			want:   "<a y><b y>\n",
		},
		{
			// The star form is one word, so the loop runs once.
			name:   "a star slice is one word",
			script: "d=(one two three)\nfor e in \"${d[*]:0:2}\"; do printf '<%s>' \"$e\"; done\necho\n",
			want:   "<one two>\n",
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

// A slice past the end is nothing rather than an error, which is what makes
// `"${@:5}"` safe to write when there may be fewer than five arguments.
func TestParameterList_sliceOutOfRangeIsEmpty(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "set -- a b\nfor e in \"${@:9}\"; do echo never; done\necho done\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if stdout != "done\n" {
		t.Fatalf("stdout = %q, want the loop not to run", stdout)
	}
}
