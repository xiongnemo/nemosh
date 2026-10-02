package applets

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A MODE is read as busybox's bb_parse_mode reads it: octal outright, or clauses of class
// letters and actions against the file's own mode, a clause with no class letters filtered
// through the umask. Only octal was read, so every one of the symbolic ones was refused.
func TestApplyChmodMode_readsAModeAsBusyboxDoes(t *testing.T) {
	for _, test := range []struct {
		spec          string
		current, mask uint32
		isDir         bool
		want          uint32
		ok            bool
	}{
		{"644", 0o7777, 0o022, false, 0o644, true},
		{"0", 0o644, 0o022, false, 0, true},
		{"000644", 0, 0o022, false, 0o644, true},
		{"7777", 0, 0o022, false, 0o7777, true},
		{"17777", 0, 0o022, false, 0, false},
		{"8", 0o644, 0o022, false, 0, false},
		{"08", 0o644, 0o022, false, 0, false},
		{"u", 0o644, 0o022, false, 0, false},
		{"ug", 0o644, 0o022, false, 0, false},
		{"u+q", 0o644, 0o022, false, 0, false},
		{"u=a", 0o644, 0o022, false, 0, false},
		{"+x", 0o644, 0o022, false, 0o755, true},
		{"+x", 0o644, 0o077, false, 0o744, true},
		{"a+x", 0o644, 0o077, false, 0o755, true},
		{"-w", 0o666, 0o022, false, 0o466, true},
		{"a-w", 0o666, 0o022, false, 0o444, true},
		{"u=rwx,g=rx,o=", 0, 0o022, false, 0o750, true},
		{"=r", 0o777, 0o022, false, 0o444, true},
		{"=u", 0o755, 0o022, false, 0, true},
		{"go=u", 0o640, 0o022, false, 0o666, true},
		{"a=,u=r,g=u", 0o777, 0o022, false, 0o440, true},
		{"u+rw,g=u,o-rwx", 0o444, 0o022, false, 0o660, true},
		{"u+s,g+s,+t", 0o444, 0o022, false, 0o7444, true},
		{"o+s", 0o644, 0o022, false, 0o644, true},
		{"u+t", 0o644, 0o022, false, 0o644, true},
		{"u-s", 0o4755, 0o022, false, 0o755, true},
		{"+X", 0o644, 0o022, false, 0o644, true},
		{"+X", 0o644, 0o022, true, 0o755, true},
		{"+X", 0o744, 0o022, false, 0o755, true},
		{"+r+w-x", 0o111, 0o022, false, 0o644, true},
		{"u+x+", 0o644, 0o022, false, 0o744, true},
		{"ug=rx,a+", 0o644, 0o022, false, 0o554, true},
		{"", 0o640, 0o022, false, 0o640, true},
		{",", 0o640, 0o022, false, 0o640, true},
		{"u+", 0o640, 0o022, false, 0o640, true},
		{"-", 0o640, 0o022, false, 0o640, true},
	} {
		got, ok := applyChmodMode(test.spec, test.current, test.mask, test.isDir)
		if ok != test.ok || ok && got != test.want {
			t.Errorf("%q on %04o under umask %03o: got %04o/%v, want %04o/%v, as busybox reads it",
				test.spec, test.current, test.mask, got, ok, test.want, test.ok)
		}
	}
}

func TestChmodModeLetters_marksTheSpecialBitsAsBusyboxDoes(t *testing.T) {
	for mode, want := range map[uint32]string{
		0o644: "rw-r--r--", 0o4755: "rwsr-xr-x", 0o7444: "r-Sr-Sr-T", 0o1777: "rwxrwxrwt", 0: "---------",
	} {
		if got := chmodModeLetters(mode); got != want {
			t.Errorf("%04o: got %q, want %q", mode, got, want)
		}
	}
}

type chmodTestView struct {
	cwd  string
	env  map[string]string
	mask uint16
}

func (v chmodTestView) WorkingDirectory() string { return v.cwd }
func (v chmodTestView) Environ() []string        { return nil }
func (v chmodTestView) LookupEnv(name string) (string, bool) {
	value, ok := v.env[name]
	return value, ok
}
func (v chmodTestView) ResolvePath(path string) string { return filepath.Join(v.cwd, path) }
func (v chmodTestView) FileModeMask() uint16           { return v.mask }

func runChmod(t *testing.T, view chmodTestView, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	err := newChmodApplet().Run(WithProcessView(context.Background(), view), args, strings.NewReader(""), &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

// writableFile makes a file whose mode is 644 on every platform: on Windows that is what a
// writable file reads as under umask 022, as busybox-w32's stat makes one up.
func writableFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
}

func isWritable(t *testing.T, path string) bool {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()&0o200 != 0
}

// chmod takes busybox's symbolic modes and options: a-w makes a file read-only and u+w
// writable again, -R goes down a tree, -v and -c report, a MODE may start with a dash, and
// options may follow the operands. Each was refused as an invalid mode.
func TestChmod_changesModesAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	view := chmodTestView{cwd: dir, mask: 0o022}
	file := filepath.Join(dir, "f")
	writableFile(t, file)

	if _, _, err := runChmod(t, view, "a-w", "f"); err != nil || isWritable(t, file) {
		t.Fatalf("chmod a-w: err %v, writable %v; want a read-only file", err, isWritable(t, file))
	}
	if _, _, err := runChmod(t, view, "u+w", "f"); err != nil || !isWritable(t, file) {
		t.Fatalf("chmod u+w: err %v, writable %v; want a writable file", err, isWritable(t, file))
	}
	if _, _, err := runChmod(t, view, "-w", "f"); err != nil || isWritable(t, file) {
		t.Fatalf("chmod -w: err %v; want the dash read as the MODE's own", err)
	}
	if _, _, err := runChmod(t, view, "644", "f"); err != nil || !isWritable(t, file) {
		t.Fatalf("chmod 644: err %v", err)
	}
	for _, test := range []struct {
		args       []string
		mask       uint16
		wantStdout string
	}{
		{[]string{"-v", "+x", "f"}, 0o022, "mode of 'f' changed to 0755 (rwxr-xr-x)\n"},
		{[]string{"644", "f", "-v"}, 0o022, "mode of 'f' changed to 0644 (rw-r--r--)\n"},
		{[]string{"-c", "644", "f"}, 0o022, ""},
		{[]string{"-v", "+x", "f"}, 0o077, "mode of 'f' changed to 0744 (rwxr--r--)\n"},
		{[]string{"-v", "--", "644", "f"}, 0o022, "mode of 'f' changed to 0644 (rw-r--r--)\n"},
	} {
		view.mask = test.mask
		stdout, stderr, err := runChmod(t, view, test.args...)
		if stdout != test.wantStdout || stderr != "" || err != nil {
			t.Errorf("chmod %q: got %q, %q, %v; want %q", test.args, stdout, stderr, err, test.wantStdout)
		}
	}
}

// A FILE that is not there is named and the rest are changed, and a MODE that cannot be read
// ends chmod at the first FILE it is read against, as in busybox, where it is read against
// each file. Under POSIXLY_CORRECT an option after an operand is a FILE.
func TestChmod_goesOnPastAMissingFile(t *testing.T) {
	dir := t.TempDir()
	view := chmodTestView{cwd: dir, mask: 0o022}
	writableFile(t, filepath.Join(dir, "f"))
	_, stderr, err := runChmod(t, view, "a-w", "nope", "f")
	if want := "chmod: nope: No such file or directory\n"; stderr != want || err == nil || err.Error() != "exit status 1" {
		t.Errorf("chmod a-w nope f: got %q, %v; want %q and status 1", stderr, err, want)
	}
	if isWritable(t, filepath.Join(dir, "f")) {
		t.Error("chmod a-w nope f left f writable; the missing operand ended it")
	}
	for _, test := range []struct {
		args       []string
		env        map[string]string
		wantStderr string
		wantErr    string
	}{
		{[]string{"zz", "nope", "f", "g"}, nil, "chmod: nope: No such file or directory\n", "invalid mode 'zz'"},
		{[]string{"zz", "nope2"}, nil, "chmod: nope2: No such file or directory\n", "exit status 1"},
		{[]string{"644", "f", "-v"}, map[string]string{"POSIXLY_CORRECT": "1"}, "chmod: -v: No such file or directory\n", "exit status 1"},
		{[]string{"-vr", "f"}, nil, "", "unknown option -- r"},
		{[]string{"-R", "f"}, nil, "", "missing operand"},
	} {
		view.env = test.env
		_, stderr, err := runChmod(t, view, test.args...)
		if stderr != test.wantStderr || err == nil || err.Error() != test.wantErr {
			t.Errorf("chmod %q: got %q, %v; want %q, %q", test.args, stderr, err, test.wantStderr, test.wantErr)
		}
	}
}

// -R changes a directory and then what it holds, each reported under the name it was reached
// by. On Windows the directory itself stays writable, as busybox-w32 keeps it.
func TestChmod_recursesThroughADirectory(t *testing.T) {
	dir := t.TempDir()
	view := chmodTestView{cwd: dir, mask: 0o022}
	if err := os.MkdirAll(filepath.Join(dir, "d", "e"), 0o755); err != nil {
		t.Fatal(err)
	}
	writableFile(t, filepath.Join(dir, "d", "e", "y"))
	writableFile(t, filepath.Join(dir, "d", "x"))
	t.Cleanup(func() { _, _, _ = runChmod(t, view, "-R", "u+w", "d") })

	stdout, stderr, err := runChmod(t, view, "-Rv", "a-w", "d")
	want := "mode of 'd' changed to 0555 (r-xr-xr-x)\n" +
		"mode of 'd/e' changed to 0555 (r-xr-xr-x)\n" +
		"mode of 'd/e/y' changed to 0444 (r--r--r--)\n" +
		"mode of 'd/x' changed to 0444 (r--r--r--)\n"
	if stdout != want || stderr != "" || err != nil {
		t.Fatalf("chmod -Rv a-w d: got %q, %q, %v; want %q", stdout, stderr, err, want)
	}
	if isWritable(t, filepath.Join(dir, "d", "x")) || isWritable(t, filepath.Join(dir, "d", "e", "y")) {
		t.Error("chmod -R a-w d left a file under d writable")
	}
}

// mkdir -m reads chmod's MODE, symbolic ones too, against 777 as busybox's mkdir does.
func TestMkdir_readsAModeAsChmodDoes(t *testing.T) {
	dir := t.TempDir()
	view := chmodTestView{cwd: dir, mask: 0o022}
	var stdout, stderr bytes.Buffer
	ctx := WithProcessView(context.Background(), view)
	if err := newMkdirApplet().Run(ctx, []string{"-m", "u=rwx,go=rx", "d"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("mkdir -m u=rwx,go=rx d: %v, want the symbolic mode read", err)
	}
	if info, err := os.Stat(filepath.Join(dir, "d")); err != nil || !info.IsDir() {
		t.Fatalf("mkdir -m u=rwx,go=rx d made no directory: %v", err)
	}
	err := newMkdirApplet().Run(ctx, []string{"-m", "zz", "e"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil || err.Error() != "invalid mode 'zz'" {
		t.Fatalf("mkdir -m zz e: %v, want invalid mode 'zz'", err)
	}
}
