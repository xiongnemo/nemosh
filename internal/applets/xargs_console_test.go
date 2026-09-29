package applets

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

// -p asks at the console before each command line, as busybox's does, and runs it on a reply
// that begins with y. The console is its own here: TestDeclaredOptionsAreAccepted leaves
// `xargs -p` to this test, for the real one waits for someone to answer.
func TestXargs_asksAtTheConsole(t *testing.T) {
	replies := []string{"y\n", "no\n", "Yes\n"}
	saved := xargsConsole
	defer func() { xargsConsole = saved }()
	xargsConsole = func() (io.ReadCloser, error) {
		reply := replies[0]
		replies = replies[1:]
		return io.NopCloser(strings.NewReader(reply)), nil
	}
	var stdout, stderr bytes.Buffer
	err := xargsApplet{}.Run(context.Background(), []string{"-p", "-n", "1", "echo"}, strings.NewReader("a b c\n"), &stdout, &stderr)
	if err != nil || stdout.String() != "a\nc\n" || stderr.String() != "echo a ?...echo b ?...echo c ?..." {
		t.Fatalf("xargs -p: %q, %q, %v; want a and c run, each asked about", stdout.String(), stderr.String(), err)
	}
}
