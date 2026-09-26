package runtime_test

import "testing"

// Under `set -u` an unset parameter is an error whatever operator is applied to it -- `#`,
// `%`, `/`, `:`, the length -- as it is bare. Only the default forms, `-` `=` `+` `?` and their
// colon kin, may test an unset name, which is what they are for. The others expanded to the
// empty string and the script went on. busybox-w32 and bash both stop it; case and
// transformation operators are bash's.
func TestRuntime_nounsetReachesEveryOperator(t *testing.T) {
	for _, expansion := range []string{"${v#x}", "${v%x}", "${v/x/y}", "${v:0}", "${#v}", "${v^^}", "${v@Q}", "${a[5]#x}"} {
		script := "set -u; a=(1); echo \"[" + expansion + "]\"; echo reached"
		t.Run(expansion, func(t *testing.T) {
			if stdout, status := runScriptCapturing(script); stdout != "" || status == 0 {
				t.Errorf("got %q/%d, want the script stopped with a failing status", stdout, status)
			}
		})
	}
	// A name that is set, empty or not, is not the error; and the default forms still test.
	script := `set -u; v=; echo "[${v#x}][${#v}][${u-d}][${u:+p}]"`
	if stdout, status := runScriptCapturing(script); stdout != "[][0][d][]\n" || status != 0 {
		t.Errorf("got %q/%d, want %q/0", stdout, status, "[][0][d][]\n")
	}
}
