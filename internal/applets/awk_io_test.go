package applets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Redirection, getline, close and system.
//
// These cases cannot be handed to the references as they stand, because a command here is
// an applet rather than an OS process -- `"sort" | getline` reaches nemosh's sort and not
// the one on PATH, which is the boundary awk_command.go explains. The file and getline
// rules *were* measured against both references first; what the comparison could not cover,
// the literal expectations do.
//
// Every file a case writes goes in t.TempDir(), because a redirect test that writes to a
// relative name drops the file into the package directory -- which this suite did once.

// awkTempPath makes a path inside the test's own directory, in the forward-slash form an
// awk program can hold in a string without escaping.
func awkTempPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.ToSlash(filepath.Join(t.TempDir(), name))
}

func TestAwkOutputRedirection(t *testing.T) {
	t.Parallel()
	t.Run("a file is truncated once and then appended to", func(t *testing.T) {
		t.Parallel()
		path := awkTempPath(t, "out.txt")
		program := `{ print $1 > "` + path + `" }`
		_, stderr, status := runAwk(t, program, "one\ntwo\nthree\n")
		if stderr != "" || status != 0 {
			t.Fatalf("stderr %q status %d", stderr, status)
		}
		// The point of the case: three records, three lines. Re-opening per record
		// would leave only the last.
		if got := readFileText(t, path); got != "one\ntwo\nthree\n" {
			t.Fatalf("the file holds %q", got)
		}
	})

	t.Run("append adds to what was there", func(t *testing.T) {
		t.Parallel()
		path := awkTempPath(t, "out.txt")
		if err := os.WriteFile(filepath.FromSlash(path), []byte("first\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, stderr, status := runAwk(t, `BEGIN { print "second" >> "`+path+`" }`, ""); stderr != "" || status != 0 {
			t.Fatalf("stderr %q status %d", stderr, status)
		}
		if got := readFileText(t, path); got != "first\nsecond\n" {
			t.Fatalf("the file holds %q", got)
		}
	})

	t.Run("truncation happens even with no records", func(t *testing.T) {
		t.Parallel()
		path := awkTempPath(t, "out.txt")
		if err := os.WriteFile(filepath.FromSlash(path), []byte("old\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, status := runAwk(t, `BEGIN { print "new" > "`+path+`" }`, ""); status != 0 {
			t.Fatalf("status %d", status)
		}
		if got := readFileText(t, path); got != "new\n" {
			t.Fatalf("the file holds %q", got)
		}
	})

	t.Run("printf redirects too", func(t *testing.T) {
		t.Parallel()
		path := awkTempPath(t, "out.txt")
		if _, _, status := runAwk(t, `BEGIN { printf "%d-%s", 7, "x" > "`+path+`" }`, ""); status != 0 {
			t.Fatalf("status %d", status)
		}
		if got := readFileText(t, path); got != "7-x" {
			t.Fatalf("the file holds %q", got)
		}
	})

	t.Run("the standard streams are reachable by name", func(t *testing.T) {
		t.Parallel()
		got, _, status := runAwk(t, `BEGIN { print "out" > "/dev/stdout" }`, "")
		if got != "out\n" || status != 0 {
			t.Fatalf("got %q status %d", got, status)
		}
	})

	t.Run("a pipe runs an applet over what was written", func(t *testing.T) {
		t.Parallel()
		// sort is an applet here, so the pipe is in-process. The output arrives when the
		// pipe closes, which is why `direct` comes first.
		got, stderr, status := runAwk(t, `BEGIN { print "c" | "sort"; print "a" | "sort"; print "direct" }`, "")
		if got != "direct\na\nc\n" || stderr != "" || status != 0 {
			t.Fatalf("got %q stderr %q status %d", got, stderr, status)
		}
	})

	t.Run("close runs a pipe early", func(t *testing.T) {
		t.Parallel()
		got, _, status := runAwk(t, `BEGIN { print "c" | "sort"; print "a" | "sort"; close("sort"); print "after" }`, "")
		if got != "a\nc\nafter\n" || status != 0 {
			t.Fatalf("got %q status %d", got, status)
		}
	})
}

func TestAwkGetline(t *testing.T) {
	t.Parallel()
	t.Run("a file is read a record at a time", func(t *testing.T) {
		t.Parallel()
		path := awkTempPath(t, "in.txt")
		writeFileText(t, path, "l1\nl2\nl3\n")
		program := `BEGIN { while ((getline line < "` + path + `") > 0) n++; print n }`
		got, _, _ := runAwk(t, program, "")
		if got != "3\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("a missing file is -1 and not a failure", func(t *testing.T) {
		t.Parallel()
		got, stderr, status := runAwk(t, `BEGIN { print (getline x < "/nonexistent/nope") }`, "")
		if got != "-1\n" || stderr != "" || status != 0 {
			t.Fatalf("got %q stderr %q status %d", got, stderr, status)
		}
	})

	t.Run("the end of a file is 0", func(t *testing.T) {
		t.Parallel()
		path := awkTempPath(t, "in.txt")
		writeFileText(t, path, "only\n")
		program := `BEGIN { getline x < "` + path + `"; print (getline x < "` + path + `") }`
		if got, _, _ := runAwk(t, program, ""); got != "0\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("a file read does not move NR", func(t *testing.T) {
		t.Parallel()
		path := awkTempPath(t, "in.txt")
		writeFileText(t, path, "from-file\n")
		program := `{ getline x < "` + path + `"; print NR, x, $0 }`
		if got, _, _ := runAwk(t, program, "record\n"); got != "1 from-file record\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("plain getline takes the next record", func(t *testing.T) {
		t.Parallel()
		program := `{ if ((getline nxt) > 0) print "pair:" $0 "," nxt; else print "odd:" $0 }`
		got, _, _ := runAwk(t, program, "l1\nl2\nl3\n")
		if got != "pair:l1,l2\nodd:l3\n" {
			t.Fatalf("got %q", got)
		}
		checkAgainstReferences(t, program, "l1\nl2\nl3\n", got)
	})

	t.Run("getline in BEGIN sets the record and NF", func(t *testing.T) {
		t.Parallel()
		program := `BEGIN { getline; print "after:" $0, NR, NF }`
		got, _, _ := runAwk(t, program, "a b\n")
		if got != "after:a b 1 2\n" {
			t.Fatalf("got %q", got)
		}
		checkAgainstReferences(t, program, "a b\n", got)
	})

	t.Run("the var form does not split", func(t *testing.T) {
		t.Parallel()
		// NF stays 0: nothing became the record, so nothing was split.
		program := `BEGIN { getline v; print "v=" v, NR, NF }`
		got, _, _ := runAwk(t, program, "a b\n")
		if got != "v=a b 1 0\n" {
			t.Fatalf("got %q", got)
		}
		checkAgainstReferences(t, program, "a b\n", got)
	})

	t.Run("a getline variable is a strnum", func(t *testing.T) {
		t.Parallel()
		path := awkTempPath(t, "in.txt")
		writeFileText(t, path, "10\n")
		program := `BEGIN { getline n < "` + path + `"; print (n > 9), (n == 10) }`
		if got, _, _ := runAwk(t, program, ""); got != "1 1\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("a command's output is read", func(t *testing.T) {
		t.Parallel()
		program := `BEGIN { "echo hi" | getline v; print "[" v "]" }`
		if got, _, _ := runAwk(t, program, ""); got != "[hi]\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("a command is read to its end", func(t *testing.T) {
		t.Parallel()
		program := `BEGIN { while (("seq 3" | getline v) > 0) n++; print n, NR }`
		if got, _, _ := runAwk(t, program, ""); got != "3 3\n" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestAwkCloseAndSystem(t *testing.T) {
	t.Parallel()
	t.Run("closing something never opened is -1", func(t *testing.T) {
		t.Parallel()
		if got, _, _ := runAwk(t, `BEGIN { print close("never-opened") }`, ""); got != "-1\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("closing an open file is 0", func(t *testing.T) {
		t.Parallel()
		path := awkTempPath(t, "out.txt")
		program := `BEGIN { print "x" > "` + path + `"; print close("` + path + `") }`
		if got, _, _ := runAwk(t, program, ""); got != "0\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("a closed file can be read back", func(t *testing.T) {
		t.Parallel()
		path := awkTempPath(t, "out.txt")
		program := `BEGIN { print "written" > "` + path + `"; close("` + path + `");` +
			` getline back < "` + path + `"; print "back:" back }`
		if got, _, _ := runAwk(t, program, ""); got != "back:written\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("system answers an applet's status", func(t *testing.T) {
		t.Parallel()
		if got, _, _ := runAwk(t, `BEGIN { print system("true"), system("false") }`, ""); got != "0 1\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("system output comes out in order", func(t *testing.T) {
		t.Parallel()
		program := `BEGIN { print "before"; system("echo middle"); print "after" }`
		if got, _, _ := runAwk(t, program, ""); got != "before\nmiddle\nafter\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("fflush reports an unknown name", func(t *testing.T) {
		t.Parallel()
		if got, _, _ := runAwk(t, `BEGIN { print fflush(), fflush("nope") }`, ""); got != "0 -1\n" {
			t.Fatalf("got %q", got)
		}
	})
}

// TestAwkCommandWords covers the splitting on its own, including what it refuses.
func TestAwkCommandWords(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name    string
		command string
		want    []string
		refuses string
	}{
		{name: "a bare name", command: "sort", want: []string{"sort"}},
		{name: "arguments", command: "sort -n -r", want: []string{"sort", "-n", "-r"}},
		{name: "runs of blanks", command: "  sort   -n  ", want: []string{"sort", "-n"}},
		{name: "single quotes", command: `grep 'a b'`, want: []string{"grep", "a b"}},
		{name: "double quotes", command: `grep "a b"`, want: []string{"grep", "a b"}},
		{name: "a quoted metacharacter is ordinary", command: `grep '$'`, want: []string{"grep", "$"}},
		{name: "quotes join to a word", command: `grep 'a'b`, want: []string{"grep", "ab"}},
		{name: "a semicolon needs a shell", command: "echo a; echo b", refuses: "needs a shell"},
		{name: "a pipe needs a shell", command: "ls | sort", refuses: "needs a shell"},
		{name: "a redirect needs a shell", command: "ls > f", refuses: "needs a shell"},
		{name: "an unbalanced quote", command: `grep 'a`, refuses: "unbalanced"},
		{name: "nothing at all", command: "   ", refuses: "empty"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, err := awkCommandWords(testcase.command)
			if testcase.refuses != "" {
				if err == nil || !strings.Contains(err.Error(), testcase.refuses) {
					t.Fatalf("%q gave %v, wanted a refusal containing %q", testcase.command, err, testcase.refuses)
				}
				return
			}
			if err != nil {
				t.Fatalf("%q: %v", testcase.command, err)
			}
			if strings.Join(got, "\x00") != strings.Join(testcase.want, "\x00") {
				t.Fatalf("%q split to %q, want %q", testcase.command, got, testcase.want)
			}
		})
	}
}

func writeFileText(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.FromSlash(path), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFileText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
