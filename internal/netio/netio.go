// Package netio is the sockets the network applets speak through: nc, whois, ftpget and ftpput,
// ssl_client, httpd and wget. One package, so the applets' protocols are written once against
// it and what is beneath can be Windows's own -- Winsock and Schannel, as busybox-w32's are
// (win32/net.c, networking/tls.c) -- without any applet changing.
package netio

import (
	"errors"
	"io"
	"strings"
	"time"
)

// Conn is a connection: a stream both ways, either of which may be closed alone.
type Conn interface {
	io.ReadWriteCloser
	// CloseWrite ends what this side sends, so the peer reads the end of it, and leaves what it
	// sends back to be read. Over TLS it sends nothing, which has no way to say it.
	CloseWrite() error
	SetReadDeadline(time.Time) error
	SetWriteDeadline(time.Time) error
	LocalAddr() string
	RemoteAddr() string
}

// Listener waits for connections.
type Listener interface {
	Accept() (Conn, error)
	Close() error
	Addr() string
}

var (
	// ErrTimeout is a read, a write or a dial that ran past its deadline.
	ErrTimeout = errors.New("i/o timeout")
	// ErrClosed is a connection or a listener used after Close, or one Close ended under way.
	ErrClosed = errors.New("use of closed network connection")
)

// JoinHostPort is host:port, a host with a colon in it, an IPv6 address, in brackets.
func JoinHostPort(host, port string) string {
	if strings.IndexByte(host, ':') >= 0 {
		return "[" + host + "]:" + port
	}
	return host + ":" + port
}

// SplitHostPort is JoinHostPort undone.
func SplitHostPort(address string) (string, string, error) {
	if strings.HasPrefix(address, "[") {
		end := strings.IndexByte(address, ']')
		if end < 0 || end+1 >= len(address) || address[end+1] != ':' {
			return "", "", errors.New("missing port in address " + address)
		}
		return address[1:end], address[end+2:], nil
	}
	colon := strings.LastIndexByte(address, ':')
	if colon < 0 {
		return "", "", errors.New("missing port in address " + address)
	}
	if strings.IndexByte(address[:colon], ':') >= 0 {
		return "", "", errors.New("too many colons in address " + address)
	}
	return address[:colon], address[colon+1:], nil
}
