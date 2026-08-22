package applets

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// nc, whois and ssl_client: the three that speak a socket directly.
//
// nc connects or listens and pipes bytes; whois is one query on port 43;
// ssl_client is a TLS pipe. All three are Go's net and crypto/tls, so none needs
// a helper binary -- which is why nemosh's own wget does not use ssl_client the
// way busybox's must.

const netcatDialTimeout = 30 * time.Second

func newNcApplet() Applet {
	return simpleApplet{name: "nc", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletOptions(args, "l", "pw")
		if err != nil {
			return err
		}
		if options.has('e') {
			return fmt.Errorf("-e is not implemented: running a program on a connection is a remote shell, and this build does not offer one")
		}
		timeout := netcatDialTimeout
		if options.has('w') {
			seconds, err := parseGrepNumber(options.value('w'))
			if err != nil {
				return err
			}
			timeout = time.Duration(seconds) * time.Second
		}
		if options.has('l') {
			return listenNetcat(ctx, options.value('p'), operands, stdin, stdout, stderr)
		}
		if len(operands) < 2 {
			return fmt.Errorf("a host and a port are required")
		}
		return dialNetcat(ctx, net.JoinHostPort(operands[0], operands[1]), timeout, stdin, stdout)
	}}
}

func dialNetcat(ctx context.Context, address string, timeout time.Duration, stdin io.Reader, stdout io.Writer) error {
	dialer := net.Dialer{Timeout: timeout}
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("cannot connect to %s: %v", address, err)
	}
	defer connection.Close()
	return pipeConnection(ctx, connection, stdin, stdout)
}

// listenNetcat accepts one connection, which is what busybox's -l does: it is a
// one-shot pipe rather than a server.
//
// Bound to localhost unless an address was given, which is the same default the
// server applet takes and for the same reason -- a listener reachable from the
// network is a decision, not a default.
func listenNetcat(ctx context.Context, port string, operands []string, stdin io.Reader, stdout, stderr io.Writer) error {
	address := "127.0.0.1:" + port
	switch {
	case port == "" && len(operands) >= 2:
		address = net.JoinHostPort(operands[0], operands[1])
	case port == "" && len(operands) == 1:
		address = "127.0.0.1:" + operands[0]
	case port == "":
		return fmt.Errorf("a port is required to listen")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %v", address, err)
	}
	defer listener.Close()
	fmt.Fprintf(stderr, "listening on %s\n", listener.Addr())
	// AfterFunc, not a goroutine waiting on ctx.Done(): an uncancellable context
	// has a nil Done channel and the receive would never return. See serveHTTP.
	defer context.AfterFunc(ctx, func() { listener.Close() })()
	connection, err := listener.Accept()
	if err != nil {
		return fmt.Errorf("cannot accept: %v", err)
	}
	defer connection.Close()
	return pipeConnection(ctx, connection, stdin, stdout)
}

// pipeConnection copies both ways. **The read side is what ends the session.**
//
// Two things make a request-and-response pipeline work -- a hand-written GET fed
// in on stdin, say -- and the first draft here had only one of them.
//
// The half-close: once stdin is exhausted the write side is shut down, so the peer
// sees the end of the request and answers rather than both sides waiting for each
// other.
//
// And then *waiting for that answer*. Returning on whichever direction finished
// first looked symmetric and was wrong: for a request and a response -- which is
// most of what nc is for -- the write side always finishes first, so nc exited
// before reading a byte and printed nothing at all. The bug survived a passing
// test because Go's select chooses uniformly among ready cases, so it was a coin
// flip per run; a manual `nc 127.0.0.1 8231` against this build's own httpd is
// what caught it.
func pipeConnection(ctx context.Context, connection net.Conn, stdin io.Reader, stdout io.Writer) error {
	sent := make(chan error, 1)
	go func() {
		_, err := io.Copy(connection, stdin)
		if closer, ok := connection.(interface{ CloseWrite() error }); ok {
			closer.CloseWrite()
		}
		sent <- err
	}()
	received := make(chan error, 1)
	go func() {
		_, err := io.Copy(stdout, connection)
		received <- err
	}()
	select {
	case err := <-received:
		if err != nil {
			return err
		}
		// A failure to send is worth reporting, but not at the cost of the reply,
		// so it is read here rather than raced against the response above.
		select {
		case err := <-sent:
			return err
		default:
			return nil
		}
	case <-ctx.Done():
		return ctx.Err()
	}
}

func newWhoisApplet() Applet {
	return simpleApplet{name: "whois", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(args, "i", "hp")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return fmt.Errorf("a name to look up is required")
		}
		server, port := options.value('h'), options.value('p')
		if port == "" {
			port = "43"
		}
		for _, name := range operands {
			if err := queryWhois(ctx, name, server, port, stdout); err != nil {
				return err
			}
		}
		return nil
	}}
}

// queryWhois asks one server. The protocol is one line in, everything back, close.
//
// The server defaults by top-level domain rather than to one registry, because
// there is no single server that answers for every name and IANA's own referral
// is the only general starting point.
func queryWhois(ctx context.Context, name, server, port string, stdout io.Writer) error {
	if server == "" {
		server = whoisServerFor(name)
	}
	dialer := net.Dialer{Timeout: netcatDialTimeout}
	connection, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(server, port))
	if err != nil {
		return fmt.Errorf("cannot reach %s: %v", server, err)
	}
	defer connection.Close()
	if _, err := fmt.Fprintf(connection, "%s\r\n", name); err != nil {
		return err
	}
	_, err = io.Copy(stdout, connection)
	return err
}

// whoisServerFor picks a server from the name's last label. IANA answers for
// anything unrecognised, and its answer includes the referral for that registry.
func whoisServerFor(name string) string {
	known := map[string]string{
		"com": "whois.verisign-grs.com", "net": "whois.verisign-grs.com",
		"org": "whois.pir.org", "io": "whois.nic.io", "dev": "whois.nic.google",
		"uk": "whois.nic.uk", "cn": "whois.cnnic.cn", "jp": "whois.jprs.jp",
		"de": "whois.denic.de", "info": "whois.afilias.net",
	}
	if index := strings.LastIndex(name, "."); index >= 0 {
		if server, found := known[strings.ToLower(name[index+1:])]; found {
			return server
		}
	}
	return "whois.iana.org"
}

// newSslClientApplet is a TLS pipe: connect, negotiate, then copy both ways.
//
// busybox needs this because its own wget cannot speak TLS and shells out to it.
// Go's net/http can, so nemosh's wget does not use this at all -- it is here for
// what the name actually means, which is `openssl s_client` without openssl.
// The name is a parameter rather than a literal here, which is what
// newTestApplet("[") does and for the same reason: internal/appletmanifest reads
// the registry as *source* (manifest.go:11) and lowercases the constructor's
// identifier when no string is given. `newSslClientApplet()` reads as `sslclient`,
// and the underscore an identifier cannot hold is the whole difference.
func newSslClientApplet(name string) Applet {
	return simpleApplet{name: name, runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(args, "e", "nsh")
		if err != nil {
			return err
		}
		host := options.value('s')
		if host == "" && len(operands) > 0 {
			host = operands[0]
		}
		if host == "" {
			return fmt.Errorf("a host is required")
		}
		port := "443"
		if len(operands) > 1 {
			port = operands[1]
		}
		serverName := options.value('n')
		if serverName == "" {
			serverName = host
		}
		dialer := &net.Dialer{Timeout: netcatDialTimeout}
		connection, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(host, port),
			// The certificate is verified. -e in busybox means "exit on error";
			// there is no option here to skip verification, because a TLS pipe
			// that does not check the certificate is a plaintext pipe wearing a
			// costume.
			&tls.Config{ServerName: serverName})
		if err != nil {
			return fmt.Errorf("cannot establish TLS to %s: %v", host, err)
		}
		defer connection.Close()
		return pipeConnection(ctx, connection, stdin, stdout)
	}}
}
