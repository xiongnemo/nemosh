package netio

import (
	"context"
	"strings"
	"testing"
)

// A refused connection is worded as a POSIX system says it, `Connection refused`, and not as
// Go's net package frames a dial, `dial tcp ADDRESS:` and then Windows' sentence that the target
// machine actively refused it, which nc and wget both printed whole.
func TestDial_aRefusedConnectionIsWordedPlainly(t *testing.T) {
	listener, err := Listen("127.0.0.1", "0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := SplitHostPort(listener.Addr())
	listener.Close()
	_, err = Dial(context.Background(), "127.0.0.1", port, 0)
	if err == nil {
		t.Skip("something took the port between closing it and dialling it")
	}
	if text := err.Error(); text != "Connection refused" {
		t.Errorf("a refused dial says %q; want %q", text, "Connection refused")
	}
	if strings.Contains(err.Error(), "dial tcp") {
		t.Errorf("a refused dial still says dial tcp: %v", err)
	}
}
