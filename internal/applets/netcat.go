package applets

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/xiongnemo/nemosh/internal/netio"
)

// nc, whois and ssl_client: the three that speak a socket directly.
//
// nc connects or listens and pipes bytes; whois is one query on port 43;
// ssl_client is a TLS pipe. All three speak through internal/netio, so none needs
// a helper binary -- which is why nemosh's own wget does not use ssl_client the
// way busybox's must.

const netcatDialTimeout = 30 * time.Second

func newNcApplet() Applet {
	return simpleApplet{name: "nc", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletOptions(ctx, args, "lzv", "pw")
		if err != nil {
			return err
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
		if options.has('z') {
			return scanNetcat(ctx, operands[0], operands[1:], timeout, options.has('v'), stderr)
		}
		return dialNetcat(ctx, operands[0], operands[1], timeout, options.has('v'), stdin, stdout, stderr)
	}}
}

// dialNetcat connects and pipes, and with -v says it connected, as netcat-openbsd says it.
func dialNetcat(ctx context.Context, host, port string, timeout time.Duration, verbose bool, stdin io.Reader, stdout, stderr io.Writer) error {
	connection, err := netio.Dial(ctx, host, port, timeout)
	if err != nil {
		return fmt.Errorf("cannot connect to %s: %v", netio.JoinHostPort(host, port), err)
	}
	defer connection.Close()
	if verbose {
		fmt.Fprintf(stderr, "Connection to %s %s port [tcp/*] succeeded!\n", host, port)
	}
	return pipeConnection(ctx, connection, stdin, stdout)
}

// listenNetcat accepts one connection, which is what busybox's -l does: it is a
// one-shot pipe rather than a server.
//
// Bound to localhost unless an address was given, which is the same default the
// server applet takes and for the same reason -- a listener reachable from the
// network is a decision, not a default.
func listenNetcat(ctx context.Context, port string, operands []string, stdin io.Reader, stdout, stderr io.Writer) error {
	host := "127.0.0.1"
	switch {
	case port == "" && len(operands) >= 2:
		host, port = operands[0], operands[1]
	case port == "" && len(operands) == 1:
		port = operands[0]
	case port == "":
		return fmt.Errorf("a port is required to listen")
	}
	listener, err := netio.Listen(host, port)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %v", netio.JoinHostPort(host, port), err)
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
func pipeConnection(ctx context.Context, connection netio.Conn, stdin io.Reader, stdout io.Writer) error {
	sent := make(chan error, 1)
	go func() {
		_, err := io.Copy(connection, stdin)
		connection.CloseWrite()
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
		options, operands, err := parseAppletOptions(ctx, args, "i", "hp")
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
	connection, err := netio.Dial(ctx, server, port, netcatDialTimeout)
	if err != nil {
		return fmt.Errorf("cannot reach %s: %v", netio.JoinHostPort(server, port), err)
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
// nemosh's wget speaks TLS through netio, Schannel, so it does not use this at all --
// it is here for what the name actually means, which is `openssl s_client` without
// openssl. When stdin ends nothing is sent, as busybox's sends nothing; see
// schannelConn.CloseWrite.
// The name is a parameter rather than a literal here, which is what
// newTestApplet("[") does and for the same reason: internal/appletmanifest reads
// the registry as *source* (manifest.go:11) and lowercases the constructor's
// identifier when no string is given. `newSslClientApplet()` reads as `sslclient`,
// and the underscore an identifier cannot hold is the whole difference.
func newSslClientApplet(name string) Applet {
	return simpleApplet{name: name, runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(ctx, args, "e", "nsh")
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
		plain, err := netio.Dial(ctx, host, port, netcatDialTimeout)
		if err != nil {
			return fmt.Errorf("cannot connect to %s: %v", netio.JoinHostPort(host, port), err)
		}
		// The certificate is verified. -e in busybox means "exit on error"; there is no option
		// here to skip verification, because a TLS pipe that does not check the certificate is a
		// plaintext pipe wearing a costume.
		connection, err := netio.Client(plain, serverName)
		if err != nil {
			plain.Close()
			return fmt.Errorf("cannot establish TLS to %s: %v", host, err)
		}
		defer connection.Close()
		return pipeConnection(ctx, connection, stdin, stdout)
	}}
}
