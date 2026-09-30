package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// What a read with -t took before the time ran out is assigned, as busybox and bash assign it:
// of a `te` whose `st` comes too late, reply holds te, status 142. Split as any line is, and a
// pending backslash dropped. It was left empty. busybox's ash_test read_t.
func TestRead_keepsWhatItReadBeforeATimeout(t *testing.T) {
	for _, test := range []struct{ input, script, want string }{
		{"te", "read -t 0.2 reply; echo \">$reply:$?<\"\n", ">te:142<\n"},
		{"a b", "read -t 0.2 x y; echo \">$x|$y:$?<\"\n", ">a|b:142<\n"},
		{"a\\", "read -t 0.2 z; echo \">$z:$?<\"\n", ">a:142<\n"},
		{"", "reply=old; read -t 0.2 reply; echo \">$reply:$?<\"\n", ">:142<\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{
				Stdin: &stallingReader{data: []byte(test.input)}, Stdout: &stdout, Stderr: &stderr,
			})
			rt.RunScript(context.Background(), test.script)
			if stdout.String() != test.want {
				t.Errorf("got %q (stderr %q), want %q, as busybox and bash answer", stdout.String(), stderr.String(), test.want)
			}
		})
	}
}

// stallingReader gives its data, a byte at a time as read asks for it, and then never returns:
// a writer that has sent part of a line and gone quiet.
type stallingReader struct{ data []byte }

func (reader *stallingReader) Read(buffer []byte) (int, error) {
	if len(reader.data) == 0 {
		select {}
	}
	count := copy(buffer, reader.data)
	reader.data = reader.data[count:]
	return count, nil
}
