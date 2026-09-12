package applets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ed.
//
// Literal expectations rather than a differential sweep, because busybox-w32's ed cannot be
// asked most of these: it answers `unimplemented command` to `n`, does not wrap a forward
// search, and has no `g`. Where the two overlap they agree; the cases below are POSIX's, and
// the support matrix records that this one applet follows GNU rather than busybox.

// runEd drives ed over a file of three lines and answers what it printed.
func runEd(t *testing.T, script string, contents string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	out, stderr, status := runApplet(t, "ed", []string{"-s", path}, script)
	return out, stderr, status
}

const edThreeLines = "one\ntwo\nthree\n"

func TestEdPrintingAndAddresses(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "the whole buffer", script: "1,$p\nQ\n", want: "one\ntwo\nthree\n"},
		{name: "a comma is the whole buffer", script: ",p\nQ\n", want: "one\ntwo\nthree\n"},
		{name: "numbered", script: "1,$n\nQ\n", want: "1\tone\n2\ttwo\n3\tthree\n"},
		{name: "a range", script: "2,3p\nQ\n", want: "two\nthree\n"},
		{name: "the last line", script: "$p\nQ\n", want: "three\n"},
		{name: "an address alone prints it", script: "2\nQ\n", want: "two\n"},
		{name: "the line count", script: "$=\nQ\n", want: "3\n"},
		{name: "a forward search", script: "/three/=\nQ\n", want: "3\n"},
		{
			// The search wraps, so looking forward from the last line finds the first.
			// busybox does not wrap, which is one reason this follows GNU.
			name: "a forward search wraps", script: "$\n/one/=\nQ\n", want: "three\n1\n",
		},
		{name: "a backward search", script: "?two?=\nQ\n", want: "2\n"},
		{name: "an empty pattern repeats the last", script: "/two/=\n//=\nQ\n", want: "2\n2\n"},
		{name: "a range between searches", script: "/two/,/three/n\nQ\n", want: "2\ttwo\n3\tthree\n"},
		{name: "relative addressing", script: "2\n-1p\n+1p\nQ\n", want: "two\none\ntwo\n"},
		{name: "list shows the line end", script: "1l\nQ\n", want: "one$\n"},
		{name: "a mark", script: "2ka\n1\n'ap\nQ\n", want: "one\ntwo\n"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, stderr, status := runEd(t, testcase.script, edThreeLines)
			if got != testcase.want || stderr != "" || status != 0 {
				t.Fatalf("ed %q\n got %q stderr %q status %d\nwant %q",
					testcase.script, got, stderr, status, testcase.want)
			}
		})
	}
}

func TestEdEditing(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "delete", script: "2d\n1,$n\nQ\n", want: "1\tone\n2\tthree\n"},
		{name: "append", script: "$a\nfour\n.\n1,$n\nQ\n", want: "1\tone\n2\ttwo\n3\tthree\n4\tfour\n"},
		{name: "insert before", script: "1i\nzero\n.\n1,$n\nQ\n", want: "1\tzero\n2\tone\n3\ttwo\n4\tthree\n"},
		{name: "change", script: "2c\nTWO\n.\n1,$n\nQ\n", want: "1\tone\n2\tTWO\n3\tthree\n"},
		{name: "move", script: "1m$\n1,$n\nQ\n", want: "1\ttwo\n2\tthree\n3\tone\n"},
		{name: "copy", script: "1t$\n1,$n\nQ\n", want: "1\tone\n2\ttwo\n3\tthree\n4\tone\n"},
		{name: "join", script: "1,2j\n1,$n\nQ\n", want: "1\tonetwo\n2\tthree\n"},
		{name: "join with one address takes the next line", script: "1j\n1,$n\nQ\n", want: "1\tonetwo\n2\tthree\n"},
		{name: "substitute", script: "2s/two/TWO/\n1,$n\nQ\n", want: "1\tone\n2\tTWO\n3\tthree\n"},
		{name: "substitute globally", script: ",s/e/E/g\n1,$n\nQ\n", want: "1\tonE\n2\ttwo\n3\tthrEE\n"},
		// The second `e` of "three" is the last character, which `sed 's/e/E/2'` agrees on.
		{name: "substitute the second occurrence", script: "3s/e/E/2\n3p\nQ\n", want: "threE\n"},
		{name: "substitute with an ampersand", script: "1s/one/[&]/\n1p\nQ\n", want: "[one]\n"},
		{name: "substitute with a group", script: `1s/\(o\)ne/\1K/` + "\n1p\nQ\n", want: "oK\n"},
		{name: "substitute and print", script: "1s/one/1/p\nQ\n", want: "1\n"},
		{name: "global print", script: "g/o/p\nQ\n", want: "one\ntwo\n"},
		{name: "global inverted", script: "v/o/p\nQ\n", want: "three\n"},
		{name: "global delete", script: "g/o/d\n1,$n\nQ\n", want: "1\tthree\n"},
		{name: "global substitute", script: "g/e/s/e/E/\n1,$n\nQ\n", want: "1\tonE\n2\ttwo\n3\tthrEe\n"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, stderr, status := runEd(t, testcase.script, edThreeLines)
			if got != testcase.want || stderr != "" || status != 0 {
				t.Fatalf("ed %q\n got %q stderr %q status %d\nwant %q",
					testcase.script, got, stderr, status, testcase.want)
			}
		})
	}
}

func TestEdFiles(t *testing.T) {
	t.Parallel()
	t.Run("write saves the buffer", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "f.txt")
		if err := os.WriteFile(path, []byte(edThreeLines), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, stderr, status := runApplet(t, "ed", []string{"-s", path}, "2s/two/TWO/\nw\nq\n"); stderr != "" || status != 0 {
			t.Fatalf("stderr %q status %d", stderr, status)
		}
		if got := readFileText(t, path); got != "one\nTWO\nthree\n" {
			t.Fatalf("the file holds %q", got)
		}
	})

	t.Run("the byte count is printed without -s", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "f.txt")
		if err := os.WriteFile(path, []byte(edThreeLines), 0o644); err != nil {
			t.Fatal(err)
		}
		// 14 bytes: three lines and their newlines. Reported on loading and on writing,
		// which is what -s exists to suppress.
		got, _, _ := runApplet(t, "ed", []string{path}, "w\nq\n")
		if got != "14\n14\n" {
			t.Fatalf("got %q, want the size twice", got)
		}
	})

	t.Run("read inserts a file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "f.txt")
		other := filepath.Join(dir, "extra.txt")
		if err := os.WriteFile(path, []byte(edThreeLines), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(other, []byte("added\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		script := "1r " + filepath.ToSlash(other) + "\n1,$n\nQ\n"
		got, _, _ := runApplet(t, "ed", []string{"-s", path}, script)
		if got != "1\tone\n2\tadded\n3\ttwo\n4\tthree\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("a file that is not there yet is an empty buffer", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "new.txt")
		script := "a\nfresh\n.\nw\nq\n"
		if _, stderr, status := runApplet(t, "ed", []string{"-s", path}, script); stderr != "" || status != 0 {
			t.Fatalf("stderr %q status %d", stderr, status)
		}
		if got := readFileText(t, path); got != "fresh\n" {
			t.Fatalf("the file holds %q", got)
		}
	})
}

// TestEdRefuses covers ed's error convention, which is a bare `?` and an explanation on
// request -- unhelpful on purpose, because scripts read what ed prints.
func TestEdRefuses(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name   string
		script string
		says   string
	}{
		{name: "an address past the end", script: "99p\nh\nQ\n", says: "invalid address"},
		{name: "an unknown command", script: "Z\nh\nQ\n", says: "unknown command"},
		{name: "a substitution that matches nothing", script: "1s/zzz/x/\nh\nQ\n", says: "no match"},
		{name: "a search that finds nothing", script: "/zzz/\nh\nQ\n", says: "no match"},
		{name: "an unset mark", script: "'zp\nh\nQ\n", says: "invalid mark"},
		{name: "moving a range into itself", script: "1,2m1\nh\nQ\n", says: "invalid destination"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, _, _ := runEd(t, testcase.script, edThreeLines)
			if !strings.HasPrefix(got, "?\n") {
				t.Fatalf("ed %q printed %q, which does not begin with ?", testcase.script, got)
			}
			if !strings.Contains(got, testcase.says) {
				t.Fatalf("ed %q explained %q, which does not contain %q", testcase.script, got, testcase.says)
			}
		})
	}

	// `q` refuses once when there are unsaved changes and accepts the second time; `Q`
	// never refuses, which is the whole difference between them.
	got, _, _ := runEd(t, "1d\nq\nq\n", edThreeLines)
	if got != "?\n" {
		t.Fatalf("q on a modified buffer printed %q", got)
	}
	if quiet, _, _ := runEd(t, "1d\nQ\n", edThreeLines); quiet != "" {
		t.Fatalf("Q printed %q", quiet)
	}
	// H turns the explanations on as they happen rather than on request.
	if explained, _, _ := runEd(t, "H\n99p\nQ\n", edThreeLines); !strings.Contains(explained, "invalid address") {
		t.Fatalf("H gave %q", explained)
	}
}
