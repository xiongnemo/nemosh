package runtime_test

import "testing"

// awk, bc and dc find the files they are named from the shell's working directory, as every
// other applet does: awk's FILEs, -f progfile, getline < file and print > file, and bc's and
// dc's FILEs. Each opened the name as the process has it, from where the shell was started,
// since the shell never changes that: after `cd sub`, `awk '{print}' f` read the f the shell
// began beside, and without one could not open sub's. `print > "/dev/null"` is the shell's
// device, as a redirection's is. Each answer is busybox-w32's, measured.
func TestRuntime_awkBcAndDcFindFilesFromTheShellsDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	script := "echo decoy > f; echo 'BEGIN{print \"decoy\"}' > p.awk\n" +
		"mkdir sub; echo hello > sub/f; cd sub\n" +
		"awk '{print}' f\n" +
		"echo 'BEGIN{print \"p\"}' > p.awk; awk -f p.awk\n" +
		"awk 'BEGIN{while ((getline l < \"f\") > 0) print \"got\", l}'\n" +
		"awk 'BEGIN{print \"x\" > \"out\"}'; cat out\n" +
		"awk 'BEGIN{print \"y\" > \"/dev/null\"; print \"z\" > \"/dev/stderr\"}' 2>&1\n" +
		"echo '1 2 + p' > d.dc; dc d.dc\n" +
		"echo '3+4' > b.bc; bc b.bc </dev/null\n"
	want := "hello\np\ngot hello\nx\nz\n3\n7\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Errorf("got %q, status %d; want %q", stdout, status, want)
	}
}
