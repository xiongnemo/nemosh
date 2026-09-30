package applets_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A line sed writes ends as the command writing it says, as busybox's puts_maybe_newline has
// it: the pattern space printed at the end of the cycle, and by n, q and s///p, ends as its
// input line did; p, P, =, i, c and a's lines always end; G and x make the pattern space an
// ended line. The newline was held back until the next write, and the last one forgiven when
// the input's last line had none, so `sed -n p` lost the newline busybox ends the b with, and
// `s/b//;$a\X` wrote one too many. Each answer is busybox-w32's, measured, on `a\nb` with the b
// unterminated.
func TestSed_endsEachLineAsTheCommandWritingItSays(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"s/x/y/"}, "a\nb"},
		{[]string{"p"}, "a\na\nb\nb"},
		{[]string{"-n", "p"}, "a\nb\n"},
		{[]string{"-n", "$p"}, "b\n"},
		{[]string{"-n", "s/b/X/p"}, "X"},
		{[]string{"-n", "$="}, "2\n"},
		{[]string{"$a\\\nX"}, "a\nb\nX\n"},
		{[]string{"$i\\\nX"}, "a\nX\nb"},
		{[]string{"$c\\\nX"}, "a\nX\n"},
		{[]string{"-n", "n;p"}, "b\n"},
		{[]string{"-n", "P"}, "a\nb\n"},
		{[]string{"$!N;P;D"}, "a\nb\n"},
		{[]string{"G"}, "a\n\nb\n\n"},
		{[]string{"x"}, "\na\n"},
		{[]string{"1!G;h;$!d"}, "b\na\n"},
		{[]string{"-n", "g;p"}, "\n\n"},
		{[]string{"s/b//"}, "a\n"},
		{[]string{"s/b//p"}, "a\n"},
		{[]string{"-e", "s/b//", "-e", "$a\\\nX"}, "a\nX\n"},
		{[]string{"2q"}, "a\nb"},
		{[]string{"-n", "2{p;q}"}, "b\n"},
	} {
		stdout, _, err := runAppletWithInput(t, "a\nb", "sed", test.args...)
		if err != nil || stdout != test.want {
			t.Errorf("sed %q = %q (err %v), want %q", test.args, stdout, err, test.want)
		}
	}
}

// A line goes out with its newline, so whoever reads sed's output as it comes has whole lines:
// with the next line read and the input still open, the first is written, newline and all. It
// waited for the second to be written first.
func TestSed_writesALineAndItsNewlineTogether(t *testing.T) {
	sed, ok := applets.DefaultRegistry.Lookup("sed")
	if !ok {
		t.Fatal("expected sed to be registered")
	}
	input, feed := io.Pipe()
	out := &lockedBuffer{}
	done := make(chan error, 1)
	go func() { done <- sed.Run(context.Background(), []string{"s/a/b/"}, input, out, io.Discard) }()
	// sed reads a line ahead, to know the last, so the first is written once the second is in.
	if _, err := io.WriteString(feed, "abc\ndef\n"); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(5 * time.Second); out.String() == "" && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	got := out.String()
	feed.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got != "bbc\n" {
		t.Errorf("with the input open, sed had written %q, want %q", got, "bbc\n")
	}
}
