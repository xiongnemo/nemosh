package applets_test

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// nc -z asks whether a port takes a connection and sends nothing, as netcat-openbsd's does,
// Debian's nc: `nc -zv host 22`. With -v it says so in openbsd's words on stderr. The status is 0
// when any port asked took one. It was `unknown option -- z`.
func TestNc_scansPorts(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			connection.Close()
		}
	}()
	open := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	closedListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := strconv.Itoa(closedListener.Addr().(*net.TCPAddr).Port)
	closedListener.Close()

	for _, test := range []struct {
		args   []string
		status int
		says   []string
	}{
		{args: []string{"-zv", "127.0.0.1", open}, says: []string{fmt.Sprintf("Connection to 127.0.0.1 %s port [tcp/*] succeeded!", open)}},
		{args: []string{"-z", "127.0.0.1", open}},
		{args: []string{"-zv", "-w", "2", "127.0.0.1", closed}, status: 1,
			says: []string{fmt.Sprintf("connect to 127.0.0.1 port %s (tcp) failed", closed)}},
		{args: []string{"-zv", "-w", "2", "127.0.0.1", closed, open}, says: []string{"failed", "succeeded!"}},
		{args: []string{"-zv", "127.0.0.1", open + "-" + open}, says: []string{"succeeded!"}},
	} {
		stdout, stderr, err := runAppletWithInput(t, "", "nc", test.args...)
		status := 0
		if err != nil {
			status, _ = applets.StatusCode(err)
			if status == 0 {
				status = -1
			}
		}
		if status != test.status || stdout != "" {
			t.Errorf("nc %q: status %d (%v), stdout %q; want %d", test.args, status, err, stdout, test.status)
		}
		for _, want := range test.says {
			if !strings.Contains(stderr, want) {
				t.Errorf("nc %q said %q, want %q", test.args, stderr, want)
			}
		}
	}
	if _, _, err := runAppletWithInput(t, "", "nc", "-z", "127.0.0.1", "9-x"); err == nil {
		t.Error("nc -z with a port range that is no range succeeded")
	}
}
