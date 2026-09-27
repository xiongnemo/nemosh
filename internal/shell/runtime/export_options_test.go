package runtime_test

import "testing"

// export's and readonly's options, busybox's where busybox has them. `export -n` was "bad
// variable name", a shell error that ended the script; `readonly` listed nothing.
func TestExportAndReadonlyOptions(t *testing.T) {
	tests := []struct {
		name, script, stdout string
		status               int
	}{
		{
			name:   "export -n takes a name out of the environment",
			script: "export A=1\nexport -n A\necho \"[$(printenv A)] [$A]\"\n",
			stdout: "[] [1]\n",
		},
		{
			// busybox assigns and leaves the export flag as it was; bash would unexport too.
			name:   "export -n with a value assigns it",
			script: "export A=1\nexport -n A=5\necho \"[$(printenv A)] [$A]\"\n",
			stdout: "[5] [5]\n",
		},
		{name: "export -f is busybox's illegal option", script: "f() { :; }\nexport -f f\necho after\n", stdout: "", status: 2},
		{
			name:   "readonly lists, busybox's form",
			script: "readonly R1=x R2\nreadonly | grep '^readonly R'\nreadonly -p | grep -c '^readonly R'\n",
			stdout: "readonly R1='x'\nreadonly R2\n2\n",
		},
		{
			name:   "readonly -a and -A, bash's",
			script: "readonly -a RA=(1 2)\nreadonly -A RM=([k]=v)\necho \"${RA[1]} ${RM[k]}\"\n(RA[0]=9) 2>/dev/null\necho \"st=$?\"\n",
			stdout: "2 v\nst=2\n",
		},
		{
			name:   "declare -p of a name that is not there is 1, as in bash",
			script: "declare -p NOSUCHVAR 2>/dev/null\necho \"st=$?\"\n",
			stdout: "st=1\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if stdout != test.stdout || status != test.status {
				t.Fatalf("got %q/%d, want %q/%d; stderr = %q", stdout, status, test.stdout, test.status, stderr)
			}
		})
	}
}
