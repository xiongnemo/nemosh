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

// A comment in a backquoted command runs to the end of that command, as busybox and bash read
// it: "`echo Ok1 #comment`" is Ok1. Rewritten as $( ... ), the comment took the `)` too, and
// the substitution never closed. busybox's ash_test comment2.
func TestBackquote_aCommentEndsWithItsBackquotes(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"echo \"`echo Ok1 #comment is ignored`\"\n", "Ok1\n"},
		{"echo `echo Ok2 #comment is ignored`\n", "Ok2\n"},
		{"echo `echo a # c` after\n", "a after\n"},
		{"x=`echo \"#not\"`; echo $x\n", "#not\n"},
		{"echo end # no newline", "end\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}
