package applets

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A file the user names is opened as named, as busybox's xopen opens it: wget's -O and the file -P
// leads to, ftpget's LOCAL_FILE and uudecode's -o, absolute or on another drive. Only a name that
// came from the other end -- the URL's last element, a uuencoded file's begin line -- is held to
// what an archive entry may name. All three were refused as "absolute entry name" or
// "drive-qualified entry name" from 2026-08-23; busybox-w32 writes each.
func TestWget_writesWhereTheUserSaid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "body of %s", r.URL.Path)
	}))
	defer server.Close()
	dir, elsewhere := t.TempDir(), t.TempDir()
	for _, test := range []struct {
		args []string
		file string
	}{
		{args: []string{"-O", filepath.Join(elsewhere, "native.dat")}, file: filepath.Join(elsewhere, "native.dat")},
		{args: []string{"-O", filepath.ToSlash(filepath.Join(elsewhere, "slashed.dat"))}, file: filepath.Join(elsewhere, "slashed.dat")},
		{args: []string{"-P", elsewhere}, file: filepath.Join(elsewhere, "a.txt")},
		{args: []string{"-P", filepath.ToSlash(elsewhere)}, file: filepath.Join(elsewhere, "a.txt")},
	} {
		args := append(append([]string{"-q"}, test.args...), server.URL+"/a.txt")
		if _, stderr, err := runWget(t, dir, args...); err != nil {
			t.Errorf("wget %q: %v, %q", test.args, err, stderr)
			continue
		}
		if data, err := os.ReadFile(test.file); err != nil || string(data) != "body of /a.txt" {
			t.Errorf("wget %q wrote %q, %v", test.args, data, err)
		}
		os.Remove(test.file)
	}
}

// -P names a directory that is there, as busybox's concat_path_file leads to one: it makes none, and
// the open fails as busybox's does, naming the file.
func TestWget_makesNoDirectoryForP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "x") }))
	defer server.Close()
	dir := t.TempDir()
	_, _, err := runWget(t, dir, "-q", "-P", "missing", server.URL+"/a.txt")
	if err == nil || !strings.Contains(err.Error(), "missing/a.txt") {
		t.Errorf("wget -P into a missing directory: %v; want a failure naming missing/a.txt", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "missing")); statErr == nil {
		t.Error("wget -P made the directory")
	}
}

// The name a URL ends in is still checked: a server cannot have wget write a device or above the
// directory it was started in.
func TestWget_stillChecksTheNameAURLGives(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "x") }))
	defer server.Close()
	dir := t.TempDir()
	for _, tail := range []string{"/dir/NUL", "/dir/..", "/dir/CON.txt"} {
		if _, _, err := runWget(t, dir, "-q", server.URL+tail); err == nil {
			t.Errorf("wget of %s saved under the name it ends in", tail)
		}
	}
}

func TestFtpget_writesTheLocalNameAsGiven(t *testing.T) {
	server := startFakeFTP(t, map[string][]byte{"r.txt": []byte("from the server\n")})
	host, port := server.address()
	outer := t.TempDir()
	root := filepath.Join(outer, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	elsewhere := t.TempDir()
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})
	for name, file := range map[string]string{
		"../above.txt":                                            filepath.Join(outer, "above.txt"),
		filepath.Join(elsewhere, "native.txt"):                    filepath.Join(elsewhere, "native.txt"),
		filepath.ToSlash(filepath.Join(elsewhere, "slashed.txt")): filepath.Join(elsewhere, "slashed.txt"),
	} {
		var stdout, stderr strings.Builder
		if err := newFtpgetApplet().Run(ctx, []string{"-P", port, host, name, "r.txt"}, strings.NewReader(""), &stdout, &stderr); err != nil {
			t.Errorf("ftpget to %q: %v (%s)", name, err, stderr.String())
			continue
		}
		if data, err := os.ReadFile(file); err != nil || string(data) != "from the server\n" {
			t.Errorf("ftpget to %q wrote %q, %v", name, data, err)
		}
	}
}

// uudecode's -o is the user's, and opened as given; the begin line's name is the sender's, and
// held to what an archive entry may name.
func TestUudecode_writesMinusOAsGiven(t *testing.T) {
	dir, elsewhere := t.TempDir(), t.TempDir()
	encoded := "begin 644 sent\n#:&D*\n`\nend\n"
	target := filepath.Join(elsewhere, "decoded")
	if stderr, err := runUudecode(t, dir, encoded, "-o", target); err != nil {
		t.Fatalf("uudecode -o %s: %v, %q", target, err, stderr)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "hi\n" {
		t.Errorf("uudecode -o wrote %q, %v", data, err)
	}
	for _, sent := range []string{filepath.Join(elsewhere, "sent"), "../sent", "NUL"} {
		if _, err := runUudecode(t, dir, "begin 644 "+sent+"\n#:&D*\n`\nend\n"); err == nil {
			t.Errorf("uudecode wrote the begin line's %q", sent)
		}
	}
}

func runUudecode(t *testing.T, dir, stdin string, args ...string) (string, error) {
	t.Helper()
	applet, _ := DefaultRegistry.Lookup("uudecode")
	var stdout, stderr strings.Builder
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: dir})
	err := applet.Run(ctx, args, strings.NewReader(stdin), &stdout, &stderr)
	return stderr.String(), err
}
