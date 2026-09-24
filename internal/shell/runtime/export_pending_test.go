package runtime_test

import "testing"

// `export X` with X unset marks it and exports nothing until X has a value, and `unset`
// takes a name's attributes with it. Each transcript is busybox-w32's, measured, or bash's
// where busybox has no declare.
func TestExport_waitsForAValue(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "nothing reaches a child until the name is assigned",
			script: "unset PX\nexport PX\nenv | grep '^PX=' || echo none\nPX=v\nenv | grep '^PX='\n",
			want:   "none\nPX=v\n",
		},
		{
			name:   "export -p lists a marked name without a value",
			script: "unset PX\nexport PX\nexport -p | grep PX\n",
			want:   "export PX\n",
		},
		{
			name:   "declare -x marks, and +x unmarks",
			script: "declare -x DX\ndeclare -p DX\nDX=1\ndeclare +x DX\nenv | grep '^DX=' || echo gone\n",
			want:   "declare -x DX\ngone\n",
		},
		{
			name:   "unset takes the attributes too",
			script: "declare -i n\nunset n\nn=2+3\necho $n\n",
			want:   "2+3\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}
