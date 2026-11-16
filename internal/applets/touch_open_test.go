package applets

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Each FILE is opened once, whatever becomes of it -- one that is there, one touch makes, one it
// makes with the time it is given, a directory, a path past MAX_PATH, one not there under -c --
// and it ends with the times asked for, a time -m leaves alone staying as it was.
func TestTouch_opensEachFileOnce(t *testing.T) {
	saved := touchOpen
	t.Cleanup(func() { touchOpen = saved })
	opens := 0
	touchOpen = func(name *uint16, access, share uint32, sa *windows.SecurityAttributes, disposition, flags uint32, template windows.Handle) (windows.Handle, error) {
		opens++
		return saved(name, access, share, sa, disposition, flags, template)
	}
	dir := t.TempDir()
	long := filepath.Join(dir, strings.Repeat("d", 120), strings.Repeat("e", 120))
	if err := os.MkdirAll(long, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, name := range []string{"there", "kept"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(filepath.Join(dir, name), old, old); err != nil {
			t.Fatal(err)
		}
	}
	given := time.Date(2021, 3, 4, 5, 6, 7, 0, time.Local)
	applet, _ := DefaultRegistry.Lookup("touch")
	before := time.Now().Add(-time.Minute)
	for _, test := range []struct {
		args       []string
		file       string
		modified   time.Time // zero: about now
		accessKept bool
		notThereOK bool
	}{
		{args: []string{"there"}, file: "there"},
		{args: []string{"made"}, file: "made"},
		{args: []string{"-d", "2021-03-04 05:06:07", "given"}, file: "given", modified: given},
		{args: []string{"-m", "-d", "2021-03-04 05:06:07", "kept"}, file: "kept", modified: given, accessKept: true},
		{args: []string{"."}, file: "."},
		{args: []string{filepath.Join(long, "deep")}, file: filepath.Join(long, "deep")},
		{args: []string{"-c", "absent"}, file: "absent", notThereOK: true},
	} {
		opens = 0
		var stderr bytes.Buffer
		ctx := WithProcessView(context.Background(), hostProcessView{cwd: dir, environ: os.Environ()})
		if err := applet.Run(ctx, test.args, strings.NewReader(""), &bytes.Buffer{}, &stderr); err != nil {
			t.Fatalf("touch %q: %v, %q", test.args, err, stderr.String())
		}
		if opens != 1 {
			t.Errorf("touch %q opened the file %d times", test.args, opens)
		}
		path := test.file
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		info, err := os.Stat(path)
		if test.notThereOK {
			if !os.IsNotExist(err) {
				t.Errorf("touch -c made %s: %v", test.file, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("touch %q: %v", test.args, err)
		}
		if want := test.modified; want.IsZero() && info.ModTime().Before(before) || !want.IsZero() && !info.ModTime().Equal(want) {
			t.Errorf("touch %q left %s at %v; want %v", test.args, test.file, info.ModTime(), test.modified)
		}
		if test.accessKept && !accessTime(path, info).Equal(old) {
			t.Errorf("touch %q changed %s's access time to %v", test.args, test.file, accessTime(path, info))
		}
	}
}
