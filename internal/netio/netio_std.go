package netio

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"os"
	"time"
)

// Go's net and crypto/tls beneath the interface, until Winsock and Schannel are.

type stdConn struct{ net.Conn }

func (c stdConn) LocalAddr() string  { return c.Conn.LocalAddr().String() }
func (c stdConn) RemoteAddr() string { return c.Conn.RemoteAddr().String() }

func (c stdConn) Read(buffer []byte) (int, error) {
	read, err := c.Conn.Read(buffer)
	return read, stdError(err)
}

func (c stdConn) Write(buffer []byte) (int, error) {
	written, err := c.Conn.Write(buffer)
	return written, stdError(err)
}

func (c stdConn) CloseWrite() error {
	if closer, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return stdError(closer.CloseWrite())
	}
	return nil
}

func stdError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, os.ErrDeadlineExceeded):
		return ErrTimeout
	case errors.Is(err, net.ErrClosed):
		return ErrClosed
	}
	return err
}

// Dial connects to host's port, giving up after timeout, or when ctx is cancelled.
func Dial(ctx context.Context, host, port string, timeout time.Duration) (Conn, error) {
	dialer := net.Dialer{Timeout: timeout}
	connection, err := dialer.DialContext(ctx, "tcp", JoinHostPort(host, port))
	if err != nil {
		return nil, stdError(err)
	}
	return stdConn{connection}, nil
}

type stdListener struct{ net.Listener }

func (l stdListener) Accept() (Conn, error) {
	connection, err := l.Listener.Accept()
	if err != nil {
		return nil, stdError(err)
	}
	return stdConn{connection}, nil
}

func (l stdListener) Addr() string { return l.Listener.Addr().String() }

// Listen waits for connections on host's port; port 0 is one the system picks.
func Listen(host, port string) (Listener, error) {
	listener, err := net.Listen("tcp", JoinHostPort(host, port))
	if err != nil {
		return nil, err
	}
	return stdListener{listener}, nil
}

// Client is TLS over conn to serverName, its certificate verified against the system's roots.
func Client(conn Conn, serverName string) (Conn, error) {
	inner, ok := conn.(stdConn)
	if !ok {
		return nil, errors.New("not a connection Client can secure")
	}
	secured := tls.Client(inner.Conn, &tls.Config{ServerName: serverName})
	if err := secured.Handshake(); err != nil {
		return nil, err
	}
	return stdConn{secured}, nil
}

// LookupAddr is the names an address has, by a reverse lookup.
func LookupAddr(ctx context.Context, address string) ([]string, error) {
	return net.DefaultResolver.LookupAddr(ctx, address)
}
