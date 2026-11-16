package netio

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// What every implementation of the package owes the applets, held against whichever is beneath.

func listenLocal(t *testing.T) Listener {
	t.Helper()
	listener, err := Listen("127.0.0.1", "0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	return listener
}

func dialListener(t *testing.T, listener Listener) Conn {
	t.Helper()
	host, port, err := SplitHostPort(listener.Addr())
	if err != nil {
		t.Fatal(err)
	}
	conn, err := Dial(context.Background(), host, port, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// A request written and its end sent, CloseWrite, reaches the peer whole, and the answer the peer
// writes after reading to the end comes back: nc's request and response.
func TestConn_halfCloseAndTheAnswerAfterIt(t *testing.T) {
	listener := listenLocal(t)
	served := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			served <- err
			return
		}
		defer conn.Close()
		request, err := io.ReadAll(conn)
		if err == nil {
			_, err = conn.Write([]byte("got " + string(request)))
		}
		served <- err
	}()
	conn := dialListener(t, listener)
	if !strings.HasPrefix(conn.RemoteAddr(), "127.0.0.1:") || !strings.HasPrefix(conn.LocalAddr(), "127.0.0.1:") {
		t.Errorf("addresses %q and %q", conn.LocalAddr(), conn.RemoteAddr())
	}
	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := conn.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	answer, err := io.ReadAll(conn)
	if err != nil || string(answer) != "got hello" {
		t.Fatalf("answer %q, %v", answer, err)
	}
	if err := <-served; err != nil {
		t.Fatal(err)
	}
}

// A read past its deadline is ErrTimeout, and one waiting when Close comes ends with ErrClosed.
func TestConn_deadlineAndCloseEndAWaitingRead(t *testing.T) {
	listener := listenLocal(t)
	go func() {
		if conn, err := listener.Accept(); err == nil {
			time.Sleep(3 * time.Second)
			conn.Close()
		}
	}()
	conn := dialListener(t, listener)
	if err := conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := conn.Read(make([]byte, 1)); !errors.Is(err, ErrTimeout) || time.Since(start) > time.Second {
		t.Fatalf("a read past its deadline: %v after %v", err, time.Since(start))
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	ended := make(chan error, 1)
	go func() {
		_, err := conn.Read(make([]byte, 1))
		ended <- err
	}()
	time.Sleep(50 * time.Millisecond)
	conn.Close()
	select {
	case err := <-ended:
		if !errors.Is(err, ErrClosed) {
			t.Fatalf("a read Close ended: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not end a waiting read")
	}
}

// A dial is ended by its context, and refused where nothing listens; Close ends a waiting Accept.
func TestDialAndListen_endEarly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Dial(ctx, "127.0.0.1", "9", 5*time.Second); err == nil {
		t.Error("a dial under a cancelled context connected")
	}
	listener := listenLocal(t)
	_, port, _ := SplitHostPort(listener.Addr())
	listener.Close()
	if _, err := Dial(context.Background(), "127.0.0.1", port, 5*time.Second); err == nil {
		t.Error("a dial where nothing listens connected")
	}
	waiting := listenLocal(t)
	accepted := make(chan error, 1)
	go func() {
		_, err := waiting.Accept()
		accepted <- err
	}()
	time.Sleep(50 * time.Millisecond)
	waiting.Close()
	select {
	case err := <-accepted:
		if !errors.Is(err, ErrClosed) {
			t.Fatalf("an Accept Close ended: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not end a waiting Accept")
	}
}

// JoinHostPort and SplitHostPort answer as net's do.
func TestHostPort_asNetHasThem(t *testing.T) {
	for _, pair := range [][2]string{{"example.com", "80"}, {"127.0.0.1", "0"}, {"::1", "443"}, {"fe80::1%eth0", "22"}, {"", "8080"}} {
		joined := JoinHostPort(pair[0], pair[1])
		if want := net.JoinHostPort(pair[0], pair[1]); joined != want {
			t.Errorf("JoinHostPort%q = %q, want %q", pair, joined, want)
		}
		host, port, err := SplitHostPort(joined)
		if err != nil || host != pair[0] || port != pair[1] {
			t.Errorf("SplitHostPort(%q) = %q, %q, %v", joined, host, port, err)
		}
	}
	for _, bad := range []string{"example.com", "a:b:c", "[::1]", "[::1"} {
		if _, _, err := SplitHostPort(bad); err == nil {
			t.Errorf("SplitHostPort(%q) took it", bad)
		}
		if _, _, err := net.SplitHostPort(bad); err == nil {
			t.Errorf("net.SplitHostPort(%q) took it, and this did not", bad)
		}
	}
}
