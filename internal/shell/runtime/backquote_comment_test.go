package runtime_test

import "testing"

// A comment is text to the end of its line, and a quote or backquote in it is no quoting. An
// apostrophe in one opened a quote to the backquote pass, which hid every backquote after it,
// so a backquoted `echo c` ran a command called c and a backquote. A backquote in one began a
// substitution that never closed, and the script was refused. Both references print each line,
// measured.
func TestBackquote_aCommentIsNoQuoting(t *testing.T) {
	script := "# don't\nx=`echo c`\necho \"[$x]\"\n# it's\n" +
		"# like the APL `iota' function\necho a # a `quoted' word\n" +
		"f() {\n\t# it's `here'\n\techo b\n}\nf\n" +
		"y=`echo d` # and `e'\necho \"$y\"\n" +
		"echo \"# `echo g`\" '#' `echo h`#i\n"
	want := "[c]\na\nb\nd\n# g # h#i\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as both references answer", stdout, status, want)
	}
}
