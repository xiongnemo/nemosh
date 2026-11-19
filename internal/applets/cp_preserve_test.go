package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cp takes GNU's --preserve[=LIST] and --no-preserve=LIST, which busybox's has not: mode,
// ownership, timestamps and links each, or all, read in order with -p and -a, so `cp -a
// --no-preserve=timestamps` keeps everything but the times. links keeps a file with two names
// one file with two names in the copy, as -a does too, as GNU's -a is --preserve=all. It was
// `unknown option -- preserve=all`.
func TestCp_preservesWhatItIsAsked(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"src": "s\n", "tree/a": "linked\n", "tree/c": "c\n"})
	old := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(dir, "src"), old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(dir, "tree", "a"), filepath.Join(dir, "tree", "b")); err != nil {
		t.Skipf("no hard links here: %v", err)
	}
	keptTimes := func(name string) bool {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return info.ModTime().Equal(old)
	}
	for _, test := range []struct {
		args  []string
		times bool
	}{
		{[]string{"--preserve=timestamps", "src", "t1"}, true},
		{[]string{"--preserve", "src", "t2"}, true},
		{[]string{"--preserve=mode,timestamps", "src", "t3"}, true},
		{[]string{"--preserve=all", "src", "t4"}, true},
		{[]string{"--preserve=mode", "src", "t5"}, false},
		{[]string{"-a", "--no-preserve=timestamps", "src", "t6"}, false},
		{[]string{"-p", "--no-preserve=all", "src", "t7"}, false},
		{[]string{"--no-preserve=timestamps", "-p", "src", "t8"}, true},
		{[]string{"--no-preserve", "context,xattr", "-p", "src", "t9"}, true},
		{[]string{"src", "t10"}, false},
	} {
		if _, stderr, err := runSmall(t, dir, "", "cp", test.args...); err != nil {
			t.Errorf("cp %q: %v (%s)", test.args, err, stderr)
			continue
		}
		if got := keptTimes(test.args[len(test.args)-1]); got != test.times {
			t.Errorf("cp %q kept the time: %v, want %v", test.args, got, test.times)
		}
	}

	sameFile := func(out string) bool {
		a, errA := os.Stat(filepath.Join(dir, out, "a"))
		b, errB := os.Stat(filepath.Join(dir, out, "b"))
		if errA != nil || errB != nil {
			t.Fatalf("%s: %v %v", out, errA, errB)
		}
		return os.SameFile(a, b)
	}
	for _, test := range []struct {
		args   []string
		linked bool
	}{
		{[]string{"-R", "--preserve=links", "tree", "l1"}, true},
		{[]string{"-a", "tree", "l2"}, true},
		{[]string{"-R", "tree", "l3"}, false},
		{[]string{"-a", "--no-preserve=links", "tree", "l4"}, false},
	} {
		if _, stderr, err := runSmall(t, dir, "", "cp", test.args...); err != nil {
			t.Errorf("cp %q: %v (%s)", test.args, err, stderr)
			continue
		}
		if got := sameFile(test.args[len(test.args)-1]); got != test.linked {
			t.Errorf("cp %q kept a and b one file: %v, want %v", test.args, got, test.linked)
		}
	}

	for _, test := range []struct {
		args []string
		says string
	}{
		{[]string{"--preserve=context", "src", "x1"}, "cannot preserve security context without an SELinux-enabled kernel"},
		{[]string{"--preserve=xattr", "src", "x2"}, "cannot preserve extended attributes"},
		{[]string{"--preserve=colour", "src", "x3"}, "invalid argument 'colour' for '--preserve'"},
		{[]string{"--no-preserve", "src", "x4"}, "invalid argument 'src' for '--no-preserve'"},
	} {
		_, _, err := runSmall(t, dir, "", "cp", test.args...)
		if err == nil || !strings.Contains(err.Error(), test.says) {
			t.Errorf("cp %q = %v, want %q", test.args, err, test.says)
		}
		if _, statErr := os.Stat(filepath.Join(dir, test.args[len(test.args)-1])); statErr == nil {
			t.Errorf("cp %q copied, refused as it was", test.args)
		}
	}
}
