package runtime_test

import (
	"os"
	"path/filepath"
	"testing"
)

// An unquoted arithmetic expansion is split and globbed as a parameter's value is, in busybox
// and bash: with IFS=0, $((1001)) is three fields, and the - of $((-9)) in [0$((-9))] makes the
// range 0-9. It went in as quoted text, one field and no pattern. Quoted, it is still one field
// and its - a member. busybox's ash_test negative_arith.
func TestRuntime_anUnquotedArithmeticExpansionIsSplitAndGlobbed(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"IFS=0; set -- $((1001)); echo \"$# [$1][$2][$3]\"\n", "3 [1][][1]\n"},
		{"IFS=0; set -- \"$((1001))\"; echo \"$# [$1]\"\n", "1 [1001]\n"},
		{"IFS=-; set -- $((-5)); echo \"$# [$1][$2]\"\n", "2 [][5]\n"},
		{"IFS=0; x=$((1001)); echo \"$x\"\n", "1001\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
	dir := t.TempDir()
	for _, name := range []string{"tempfile0.tmp", "tempfile1.tmp", "tempfile9.tmp"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	script := "cd '" + filepath.ToSlash(dir) + "'\necho tempfile[0$((-9))].tmp\necho tempfile[0\"$((-9))\"].tmp\n"
	want := "tempfile0.tmp tempfile1.tmp tempfile9.tmp\ntempfile0.tmp tempfile9.tmp\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, want)
	}
}
