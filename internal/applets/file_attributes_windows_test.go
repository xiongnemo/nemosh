//go:build windows

package applets

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// attributeFixture is the tree busybox-w32's lsattr was measured on, as the working directory:
// a hidden B.txt, a plain f.txt, a dotted .hid, and sub holding a read-only s.txt. Every
// entry's attributes are set rather than left to what the volume gives a new file, and so are
// those of . and .., which -a lists.
func attributeFixture(t *testing.T) {
	t.Helper()
	parent := t.TempDir()
	work := filepath.Join(parent, "work")
	if err := os.MkdirAll(filepath.Join(work, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"B.txt": "z\n", "f.txt": "hi\n", ".hid": "y\n", "sub/s.txt": "x\n"} {
		if err := os.WriteFile(filepath.Join(work, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, attributes := range map[string]uint32{
		parent:      0,
		work:        0,
		"sub":       0,
		"B.txt":     windows.FILE_ATTRIBUTE_HIDDEN | windows.FILE_ATTRIBUTE_ARCHIVE,
		"f.txt":     windows.FILE_ATTRIBUTE_ARCHIVE,
		".hid":      windows.FILE_ATTRIBUTE_ARCHIVE,
		"sub/s.txt": windows.FILE_ATTRIBUTE_READONLY | windows.FILE_ATTRIBUTE_ARCHIVE,
	} {
		if !filepath.IsAbs(name) {
			name = filepath.Join(work, name)
		}
		setFileAttributes(t, name, attributes)
	}
	t.Chdir(work)
	// The read-only file would outlive the test: TempDir's cleanup cannot remove it.
	t.Cleanup(func() { setFileAttributes(t, filepath.Join(work, "sub", "s.txt"), 0) })
}

func setFileAttributes(t *testing.T, native string, attributes uint32) {
	t.Helper()
	name, err := windows.UTF16PtrFromString(native)
	if err == nil {
		err = windows.SetFileAttributes(name, attributes)
	}
	if err != nil {
		t.Fatalf("SetFileAttributes(%s): %v", native, err)
	}
}

// lsattr lists what busybox-w32's lists, line for line: the eleven columns, -a with . and ..,
// -l spelled out, -d a directory itself, -R each directory under a heading, and a FILE named
// as it was written.
func TestLsattr_listsAsBusyboxW32Does(t *testing.T) {
	attributeFixture(t)
	for _, test := range []struct {
		args []string
		want string
	}{
		{nil, "------h-a-- ./B.txt\n--------a-- ./f.txt\n----------- ./sub\n"},
		{[]string{"-a"}, "----------- ./.\n----------- ./..\n--------a-- ./.hid\n------h-a-- ./B.txt\n--------a-- ./f.txt\n----------- ./sub\n"},
		{[]string{"-l"}, "./B.txt                      Hidden, Archive\n./f.txt                      Archive\n./sub                        ---\n"},
		{[]string{"-d", ".", "sub"}, "----------- .\n----------- sub\n"},
		{[]string{"-R"}, "------h-a-- ./B.txt\n--------a-- ./f.txt\n----------- ./sub\n\n./sub:\n-----r--a-- ./sub/s.txt\n\n"},
		{[]string{"f.txt", "sub/"}, "--------a-- f.txt\n-----r--a-- sub/s.txt\n"},
		{[]string{"-dl", "sub"}, "sub                          ---\n"},
	} {
		stdout, stderr, status := runApplet(t, "lsattr", test.args, "")
		if stdout != test.want || stderr != "" || status != 0 {
			t.Errorf("lsattr %q: %q, stderr %q, status %d; want %q", test.args, stdout, stderr, status, test.want)
		}
	}
}

// A FILE that is not there is named, quoted as busybox quotes it, and the rest are still listed.
// The status is 1, where busybox-w32 answers 0 whatever happened.
func TestLsattr_namesAFileThatIsNotThere(t *testing.T) {
	attributeFixture(t)

	stdout, stderr, status := runApplet(t, "lsattr", []string{"nosuch", "f.txt"}, "")

	if stdout != "--------a-- f.txt\n" || stderr != "lsattr: cannot stat 'nosuch': No such file or directory\n" || status != 1 {
		t.Fatalf("stdout %q, stderr %q, status %d", stdout, stderr, status)
	}
}

// A junction is j, a link and not a directory, so lsattr lists it rather than what it leads to,
// as busybox-w32's lstat has it; Go calls a junction a directory.
func TestLsattr_listsAJunctionAsItself(t *testing.T) {
	attributeFixture(t)
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", "jn", "sub").CombinedOutput(); err != nil {
		t.Skipf("no junction can be made here: %v %s", err, out)
	}

	short, _, _ := runApplet(t, "lsattr", []string{"jn"}, "")
	long, _, _ := runApplet(t, "lsattr", []string{"-dl", "jn"}, "")

	if short != "j---------- jn\n" || long != "jn                           Junction\n" {
		t.Fatalf("lsattr jn = %q, lsattr -dl jn = %q", short, long)
	}
}

// chattr sets with + and clears with -, and -R goes down through a directory. An attribute
// Windows will not put on a directory is reported, with strerror's reason, and the files
// under it are changed all the same.
func TestChattr_setsAndClearsAsBusyboxW32Does(t *testing.T) {
	attributeFixture(t)
	for _, step := range []struct {
		args       []string
		stderr     string
		status     int
		list, want string
	}{
		{args: []string{"+h", "f.txt"}, list: "f.txt", want: "------h-a-- f.txt\n"},
		{args: []string{"-h", "+s", "f.txt"}, list: "f.txt", want: "-------sa-- f.txt\n"},
		{args: []string{"-sa", "f.txt"}, list: "f.txt", want: "----------- f.txt\n"},
		{args: []string{"-R", "+n", "sub"}, list: "-R", want: "------h-a-- ./B.txt\n----------- ./f.txt\n----------n ./sub\n\n./sub:\n-----r--a-n ./sub/s.txt\n\n"},
		{args: []string{"-Rn", "+t", "sub"}, stderr: "chattr: cannot set the attributes of sub: Invalid argument\n", status: 1, list: "sub", want: "-----r--at- sub/s.txt\n"},
	} {
		_, stderr, status := runApplet(t, "chattr", step.args, "")
		if stderr != step.stderr || status != step.status {
			t.Fatalf("chattr %q: stderr %q, status %d; want %q, %d", step.args, stderr, status, step.stderr, step.status)
		}
		if listed, _, _ := runApplet(t, "lsattr", []string{step.list}, ""); listed != step.want {
			t.Fatalf("after chattr %q, lsattr %s = %q, want %q", step.args, step.list, listed, step.want)
		}
	}
}

// What busybox's chattr refuses, this refuses, in its own words: no letter to change, one set
// and cleared at once, a letter it has not got, and no FILE.
func TestChattr_refusesWhatBusyboxRefuses(t *testing.T) {
	attributeFixture(t)
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"f.txt"}, "nothing to change: -LETTERS clears attributes and +LETTERS sets them"},
		{[]string{"-R", "f.txt"}, "nothing to change: -LETTERS clears attributes and +LETTERS sets them"},
		{[]string{"+r", "-r", "f.txt"}, "an attribute cannot be both set and cleared"},
		{[]string{"+x", "f.txt"}, "invalid option -- 'x'"},
		{[]string{"+R", "f.txt"}, "invalid option -- 'R'"},
		{[]string{"+r"}, "missing operand"},
	} {
		// The error comes back for the shell to print, as "chattr: " and these words.
		_, stderr, status := runApplet(t, "chattr", test.args, "")
		if stderr != test.want || status != 1 {
			t.Errorf("chattr %q: stderr %q, status %d; want %q and 1", test.args, stderr, status, test.want)
		}
	}
	if _, stderr, status := runApplet(t, "chattr", []string{"+", "f.txt"}, ""); stderr != "" || status != 0 {
		t.Errorf("chattr + f.txt: stderr %q, status %d; a bare + still says which way", stderr, status)
	}
}
