package netio

import (
	"crypto/tls"
	"net"
	"time"
)

// TLS is still crypto/tls's, over a Winsock connection through netAdapter, until Schannel is
// beneath it.

// Client is TLS over conn to serverName, its certificate verified against the system's roots.
func Client(conn Conn, serverName string) (Conn, error) {
	secured := tls.Client(netAdapter{conn}, &tls.Config{ServerName: serverName})
	if err := secured.Handshake(); err != nil {
		return nil, err
	}
	return tlsConn{secured, conn}, nil
}

type tlsConn struct {
	*tls.Conn
	plain Conn
}

func (c tlsConn) CloseWrite() error  { return c.Conn.CloseWrite() }
func (c tlsConn) LocalAddr() string  { return c.plain.LocalAddr() }
func (c tlsConn) RemoteAddr() string { return c.plain.RemoteAddr() }

// netAdapter is a Conn as crypto/tls wants one, a net.Conn.
type netAdapter struct{ Conn }

type textAddr string

func (a textAddr) Network() string { return "tcp" }
func (a textAddr) String() string  { return string(a) }

func (a netAdapter) LocalAddr() net.Addr  { return textAddr(a.Conn.LocalAddr()) }
func (a netAdapter) RemoteAddr() net.Addr { return textAddr(a.Conn.RemoteAddr()) }

func (a netAdapter) SetDeadline(deadline time.Time) error {
	_ = a.Conn.SetReadDeadline(deadline)
	return a.Conn.SetWriteDeadline(deadline)
}
