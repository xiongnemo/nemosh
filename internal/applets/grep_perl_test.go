package applets_test

import (
	"bufio"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// grep -P is Perl's syntax as RE2 reads it, which is most of what people write with -P: \d, \w,
// \s, \b, lazy quantifiers, (?i) and named groups. It was `unknown option -- P`.
func TestGrep_perlSyntax(t *testing.T) {
	input := "a123:x\nb12\nfoo bar\nfoobar\naXb then b\nABC\n"
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"-P", `\d{3}`}, want: "a123:x\n"},
		{args: []string{"-oP", `\d+`}, want: "123\n12\n"},
		{args: []string{"-P", `\bfoo\b`}, want: "foo bar\n"},
		{args: []string{"-oP", `a.*?b`}, want: "aXb\n"},
		{args: []string{"-P", `(?i)abc`}, want: "ABC\n"},
		{args: []string{"-cP", `^\w+$`}, want: "3\n"},
		{args: []string{"-oP", `(?<word>fo+)b`}, want: "foob\n"},
		{args: []string{"-xP", `b\d\d`}, want: "b12\n"},
	} {
		stdout, stderr, err := runAppletWithInput(t, input, "grep", test.args...)
		if err != nil || stdout != test.want {
			t.Errorf("grep %q = %q, %v (%s); want %q", test.args, stdout, err, stderr, test.want)
		}
	}
}

// What RE2 has not got is refused by name, status 2, rather than matched some other way:
// lookaround and backreferences are the Perl a -P pattern most often leans on.
func TestGrep_perlRefusesWhatRE2HasNot(t *testing.T) {
	for _, test := range []struct {
		args []string
		says string
	}{
		{args: []string{"-P", `\w+(?=:)`}, says: "lookaround"},
		{args: []string{"-P", `(?<!x)y`}, says: "lookaround"},
		{args: []string{"-P", `(a)\1`}, says: "backreference"},
		{args: []string{"-FP", "x"}, says: "conflicting matchers"},
	} {
		_, stderr, err := runAppletWithInput(t, "a:b\n", "grep", test.args...)
		status, _ := applets.StatusCode(err)
		if reported := stderr + errorText(err); status != 2 || !strings.Contains(reported, test.says) {
			t.Errorf("grep %q: status %d, %q; want 2 and %q", test.args, status, reported, test.says)
		}
	}
}

// grep writes each line as it finds it, so `tail -f log | grep --line-buffered x | ...` shows a
// line while the log is still open. --line-buffered asks for what it does anyway, and is taken.
func TestGrep_writesEachLineAsItFindsIt(t *testing.T) {
	for _, args := range [][]string{{"x"}, {"--line-buffered", "x"}} {
		inputReader, inputWriter := io.Pipe()
		outputReader, outputWriter := io.Pipe()
		applet, _ := applets.DefaultRegistry.Lookup("grep")
		done := make(chan error, 1)
		go func() {
			done <- applet.Run(context.Background(), args, inputReader, outputWriter, io.Discard)
			outputWriter.Close()
		}()
		go func() { _, _ = io.WriteString(inputWriter, "one x\ntwo\n") }()
		line := make(chan string, 1)
		go func() {
			text, _ := bufio.NewReader(outputReader).ReadString('\n')
			line <- text
		}()
		select {
		case got := <-line:
			if got != "one x\n" {
				t.Errorf("grep %q wrote %q first", args, got)
			}
		case <-time.After(5 * time.Second):
			t.Errorf("grep %q wrote nothing while its input was still open", args)
		}
		inputWriter.Close()
		if err := <-done; err != nil {
			t.Errorf("grep %q: %v", args, err)
		}
	}
}
