package netio

import (
	"net/netip"
	"os"
	"strconv"
	"testing"
)

// Sockets is every TCP and UDP socket with its owner, read at the rows' layout: a listener this
// test opens is there, LISTEN, at its own port and address, owned by this process.
func TestSockets_findsAListenerAndItsOwner(t *testing.T) {
	listener, err := Listen("127.0.0.1", "0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	_, portText, err := SplitHostPort(listener.Addr())
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portText)
	sockets, err := Sockets()
	if err != nil {
		t.Fatal(err)
	}
	want := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), uint16(port))
	for _, socket := range sockets {
		if socket.Protocol == "tcp" && socket.Local == want {
			if socket.State != TCPListen || socket.PID != os.Getpid() {
				t.Errorf("the listener at %v is state %d, pid %d; want LISTEN and %d", want, socket.State, socket.PID, os.Getpid())
			}
			return
		}
	}
	t.Fatalf("the listener at %v is not among %d sockets", want, len(sockets))
}
