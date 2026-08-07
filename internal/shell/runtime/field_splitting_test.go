package runtime_test

import "testing"

func TestRuntime_splitsUnquotedExpansions(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "a variable holding two words becomes two fields",
			script: "x='a b'\nset -- $x\necho [$#]\n",
			want:   "[2]\n",
		},
		{
			name:   "quoting keeps it one field",
			script: "x='a b'\nset -- \"$x\"\necho [$#]\n",
			want:   "[1]\n",
		},
		{
			name:   "a command substitution splits on newlines too",
			script: "set -- $(echo a; echo b)\necho [$#] [$1] [$2]\n",
			want:   "[2] [a] [b]\n",
		},
		{
			name:   "a quoted command substitution stays whole",
			script: "set -- \"$(echo a; echo b)\"\necho [$#]\n",
			want:   "[1]\n",
		},
		{
			name:   "runs of whitespace are one delimiter",
			script: "x='a   b'\nset -- $x\necho [$#]\n",
			want:   "[2]\n",
		},
		{
			name:   "leading and trailing whitespace make no empty fields",
			script: "x='  a b  '\nset -- $x\necho [$#]\n",
			want:   "[2]\n",
		},
		{
			name:   "an empty expansion contributes no field",
			script: "x=\nset -- $x\necho [$#]\n",
			want:   "[0]\n",
		},
		{
			name:   "adjacent literals join the first and last fields",
			script: "x='a b'\nset -- [$x]\necho [$1] [$2]\n",
			want:   "[[a] [b]]\n",
		},
		{
			name:   "a loop iterates once per field",
			script: "list='one two three'\nfor item in $list; do echo $item; done\n",
			want:   "one\ntwo\nthree\n",
		},
		{
			name:   "a custom IFS delimits on its own characters",
			script: "IFS=:\nx=a:b:c\nset -- $x\necho [$#] [$2]\n",
			want:   "[3] [b]\n",
		},
		{
			name:   "a non-whitespace separator keeps an empty field between two of them",
			script: "IFS=:\nx=a::b\nset -- $x\necho [$#] [$2]\n",
			want:   "[3] []\n",
		},
		{
			name:   "an empty IFS turns splitting off",
			script: "IFS=\nx='a b'\nset -- $x\necho [$#]\n",
			want:   "[1]\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}
