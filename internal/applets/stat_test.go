package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// statIn runs stat in dir, and gives what it printed on each stream and its status.
func statIn(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()
	applet, _ := applets.DefaultRegistry.Lookup("stat")
	ctx := applets.WithProcessView(context.Background(), permuteTestView{cwd: dir})
	var stdout, stderr bytes.Buffer
	status := 0
	if err := applet.Run(ctx, args, strings.NewReader(""), &stdout, &stderr); err != nil {
		code, ok := applets.StatusCode(err)
		if !ok {
			t.Fatalf("stat %q: %v", args, err)
		}
		status = code
	}
	return stdout.String(), stderr.String(), status
}

// statDir is a directory holding five.txt, of five bytes, an empty file, and a directory with
// two directories in it.
func statDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]string{"five.txt": "hello", "empty": ""} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"sub/a", "sub/b"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// stat prints busybox's layout when -c gives none (coreutils/stat.c:627-632). It refused to,
// as mostly fields Windows has not got; busybox-w32 has them all, and so does this.
func TestStat_printsBusyboxLayout(t *testing.T) {
	dir := statDir(t)
	stdout, stderr, status := statIn(t, dir, "five.txt")
	when := `\d{4}-\d\d-\d\d \d\d:\d\d:\d\d\.\d{9} [-+]\d{4}`
	layout := regexp.MustCompile(`^  File: five\.txt\n  Size: 5         \tBlocks: [0-9]+ +IO Block: [0-9]+ +regular file\n` +
		`Device: [0-9a-f]+h/[0-9]+d\tInode: [0-9]+ +Links: 1\n` +
		`Access: \(0[0-7]{3}/-[-rwx]{9}\)  Uid: \( *[0-9]+/ *\S+\)   Gid: \( *[0-9]+/ *\S+\)\n` +
		`Access: ` + when + `\nModify: ` + when + `\nChange: ` + when + `\n$`)
	if status != 0 || stderr != "" || !layout.MatchString(stdout) {
		t.Fatalf("stat five.txt: %q, %q, status %d; want busybox's layout", stdout, stderr, status)
	}
	info, err := os.Stat(filepath.Join(dir, "five.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got, _, _ := statIn(t, dir, "-c", "%Y", "five.txt"); got != strconv.FormatInt(info.ModTime().Unix(), 10)+"\n" {
		t.Fatalf("stat -c %%Y: %q, want the modification time, %d", got, info.ModTime().Unix())
	}
}

// A -c FORMAT is busybox's print_it: flags, a width and a precision before each letter, as its
// printf takes them; a letter stat does not know printed as itself; %% whatever is between;
// and no newline after a % that ends it. It refused every letter but six.
func TestStat_formatsAsBusybox(t *testing.T) {
	dir := statDir(t)
	for _, test := range []struct{ format, operand, want string }{
		{"%n|%s|%F", "five.txt", "five.txt|5|regular file\n"},
		{"%F", "empty", "regular empty file\n"},
		{"%F|%h", "sub", "directory|4\n"},
		{"[%5q][%-10n][%012n][%.2n][%5%][%B]", "five.txt", "[    q][five.txt  ][0000five.txt][fi][%][512]\n"},
		{"%N", "five.txt", "five.txt\n"},
		{"x%", "five.txt", "x%"},
		{"y%-5", "five.txt", "y%"},
		{"", "five.txt", "\n"},
	} {
		if got, stderr, status := statIn(t, dir, "-c", test.format, test.operand); got != test.want || status != 0 {
			t.Errorf("stat -c %q %s: %q, %q, status %d; want %q", test.format, test.operand, got, stderr, status, test.want)
		}
	}
}

// -t is busybox's terse line of fifteen fields, and -f the filesystem's layout and line.
func TestStat_terseAndFilesystem(t *testing.T) {
	dir := statDir(t)
	fields := func(line string) []string { return strings.Fields(strings.TrimSuffix(line, "\n")) }
	if got, _, _ := statIn(t, dir, "-t", "five.txt"); len(fields(got)) != 15 || fields(got)[0] != "five.txt" || fields(got)[1] != "5" {
		t.Errorf("stat -t five.txt: %q, want fifteen fields, the name and the size first", got)
	}
	layout := regexp.MustCompile(`^  File: "five\.txt"\n    ID: [0-9a-f]+ +Namelen: [0-9]+ +Type: \S.*\nBlock size: [0-9]+ *\n` +
		`Blocks: Total: [0-9]+ +Free: [0-9]+ +Available: [0-9]+\nInodes: Total: [0-9]+ +Free: [0-9]+\n$`)
	if got, stderr, status := statIn(t, dir, "-f", "five.txt"); status != 0 || !layout.MatchString(got) {
		t.Errorf("stat -f five.txt: %q, %q, status %d; want busybox's layout", got, stderr, status)
	}
	if got, _, _ := statIn(t, dir, "-f", "-t", "five.txt"); len(fields(got)) != 10 || fields(got)[0] != "five.txt" {
		t.Errorf("stat -f -t five.txt: %q, want ten fields, the name first", got)
	}
}

// A FILE stat cannot stat is named as busybox names it, the rest are printed, and the status is 1.
func TestStat_goesOnPastWhatItCannotStat(t *testing.T) {
	dir := statDir(t)
	stdout, stderr, status := statIn(t, dir, "-c", "%n", "nosuch", "five.txt")
	if stdout != "five.txt\n" || stderr != "stat: cannot stat 'nosuch': No such file or directory\n" || status != 1 {
		t.Errorf("stat nosuch five.txt: %q, %q, status %d", stdout, stderr, status)
	}
	_, stderr, status = statIn(t, dir, "-f", "nosuch")
	if stderr != "stat: cannot read file system information for 'nosuch': No such file or directory\n" || status != 1 {
		t.Errorf("stat -f nosuch: %q, status %d", stderr, status)
	}
}

// A link is the link itself, and with -L what it names.
func TestStat_linkAndFollowed(t *testing.T) {
	dir := statDir(t)
	if err := os.Symlink("five.txt", filepath.Join(dir, "link")); err != nil {
		t.Skipf("no symbolic link here: %v", err)
	}
	// A link's own permissions are lrwxrwxrwx, as Linux and busybox-w32 show them. macOS gives a
	// link the umask's, and CI found lrwxr-xr-x there, so on macOS the want is read from the link.
	mode := "lrwxrwxrwx"
	if runtime.GOOS == "darwin" {
		info, err := os.Lstat(filepath.Join(dir, "link"))
		if err != nil {
			t.Fatal(err)
		}
		mode = "l" + info.Mode().String()[1:]
	}
	want := "'link' -> 'five.txt'|symbolic link|8|" + mode + "\n"
	if got, _, _ := statIn(t, dir, "-c", "%N|%F|%s|%A", "link"); got != want {
		t.Errorf("stat -c %%N|%%F|%%s|%%A link: %q, want %q", got, want)
	}
	if got, _, _ := statIn(t, dir, "-L", "-c", "%N|%F|%s", "link"); got != "link|regular file|5\n" {
		t.Errorf("stat -L -c %%N|%%F|%%s link: %q", got)
	}
}
