package applets

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// nc's listening half, its option handling, and whois' server table -- the paths
// the round-trip test does not reach because it only dials.

// nc -l accepts one connection and pipes it, which is what busybox's -l does: a
// one-shot pipe rather than a server. The port is chosen by the OS and read back
// from the "listening on" line, because a fixed port makes a test that fails when
// something else on the machine holds it.
func TestNc_listensAndAcceptsOneConnection(t *testing.T) {
	// Port 0, so the OS picks one and nothing collides.
	var stdout, stderr syncBuilder
	done := make(chan error, 1)
	go func() {
		done <- newNcApplet().Run(t.Context(), []string{"-l", "-p", "0"},
			strings.NewReader("from the listener\n"), &stdout, &stderr)
	}()

	address := waitForListeningLine(t, &stderr)
	connection, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		t.Fatalf("cannot reach the listener at %s: %v", address, err)
	}
	// The listener's stdin reaches the client, which is the half `nc -l < file`
	// depends on.
	reply, err := bufio.NewReader(connection).ReadString('\n')
	if err != nil {
		t.Fatalf("the listener sent nothing: %v", err)
	}
	if reply != "from the listener\n" {
		t.Fatalf("the client read %q", reply)
	}
	// And the client's bytes reach the listener's stdout.
	fmt.Fprintf(connection, "from the client\n")
	connection.Close()
	if err := <-done; err != nil {
		t.Fatalf("nc -l: %v (%s)", err, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "from the client") {
		t.Fatalf("the listener's stdout = %q", got)
	}
}

// The three spellings of an address to listen on, which the switch in listenNetcat
// distinguishes and which are easy to get the wrong way round.
func TestNc_listenAddressSpellings(t *testing.T) {
	for _, test := range []struct {
		name     string
		args     []string
		wantHost string
	}{
		// -p alone binds loopback, which is the default a listener reachable from
		// the network has to be asked for.
		{name: "-p only", args: []string{"-l", "-p", "0"}, wantHost: "127.0.0.1"},
		// One operand is a port, still on loopback.
		{name: "a port operand", args: []string{"-l", "0"}, wantHost: "127.0.0.1"},
		// Two operands are host and port, which is how somebody asks for more.
		{name: "host and port operands", args: []string{"-l", "127.0.0.1", "0"}, wantHost: "127.0.0.1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr syncBuilder
			ctx, cancel := contextWithCancel(t)
			go func() { newNcApplet().Run(ctx, test.args, strings.NewReader(""), &stdout, &stderr) }()
			address := waitForListeningLine(t, &stderr)
			cancel()
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				t.Fatal(err)
			}
			if host != test.wantHost {
				t.Fatalf("nc %v bound %s, want %s", test.args, host, test.wantHost)
			}
		})
	}
}

// Listening with no port at all is an error rather than a guess.
func TestNc_needsAPortToListen(t *testing.T) {
	var stdout, stderr strings.Builder
	err := newNcApplet().Run(t.Context(), []string{"-l"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("nc -l with no port was accepted")
	}
	if !strings.Contains(err.Error(), "port") {
		t.Fatalf("nc -l said %q, which does not mention a port", err)
	}
}

// Dialling needs both a host and a port: one operand is not half an address.
func TestNc_needsBothHostAndPort(t *testing.T) {
	for _, args := range [][]string{{}, {"127.0.0.1"}} {
		var stdout, stderr strings.Builder
		err := newNcApplet().Run(t.Context(), args, strings.NewReader(""), &stdout, &stderr)
		if err == nil {
			t.Errorf("nc %v was accepted", args)
		}
	}
}

// -w is a timeout in seconds, so a value that is not a number is refused rather
// than silently becoming zero -- which would be an immediate failure to connect.
func TestNc_refusesATimeoutThatIsNotANumber(t *testing.T) {
	for _, value := range []string{"abc", "1.5", "-3", ""} {
		var stdout, stderr strings.Builder
		err := newNcApplet().Run(t.Context(), []string{"-w", value, "127.0.0.1", "1"},
			strings.NewReader(""), &stdout, &stderr)
		if err == nil {
			t.Errorf("nc -w %q was accepted", value)
		}
	}
}

// A connection that cannot be made is an error naming the address, not a silent
// success. Port 1 on loopback is chosen because nothing listens there and the
// refusal is immediate rather than a timeout.
func TestNc_reportsAConnectionItCannotMake(t *testing.T) {
	var stdout, stderr strings.Builder
	err := newNcApplet().Run(t.Context(), []string{"-w", "2", "127.0.0.1", "1"},
		strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Skip("something is listening on 127.0.0.1:1 on this machine")
	}
	if !strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Fatalf("nc said %q, which does not name the address", err)
	}
}

// A listener on an address that cannot be bound reports it rather than hanging.
func TestNc_reportsAnAddressItCannotBind(t *testing.T) {
	var stdout, stderr strings.Builder
	// 240.0.0.1 is reserved and not assigned to any interface on any machine.
	err := newNcApplet().Run(t.Context(), []string{"-l", "240.0.0.1", "9"},
		strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("nc -l bound an address that is not on this machine")
	}
	if !strings.Contains(err.Error(), "240.0.0.1") {
		t.Fatalf("nc said %q, which does not name the address", err)
	}
}

// whois' server table, which is data and therefore worth walking rather than
// spot-checking. The default matters: IANA's answer names the registry to ask
// next, so an unrecognised suffix is still useful.
func TestWhoisServerFor_choosesFromTheLastLabel(t *testing.T) {
	for _, test := range []struct{ name, want string }{
		{name: "example.com", want: "whois.verisign-grs.com"},
		{name: "example.net", want: "whois.verisign-grs.com"},
		{name: "example.org", want: "whois.pir.org"},
		{name: "example.io", want: "whois.nic.io"},
		{name: "example.dev", want: "whois.nic.google"},
		{name: "example.uk", want: "whois.nic.uk"},
		{name: "example.cn", want: "whois.cnnic.cn"},
		{name: "example.jp", want: "whois.jprs.jp"},
		{name: "example.de", want: "whois.denic.de"},
		{name: "example.info", want: "whois.afilias.net"},
		// Case does not matter in a domain name.
		{name: "EXAMPLE.COM", want: "whois.verisign-grs.com"},
		{name: "Example.Org", want: "whois.pir.org"},
		// A subdomain is still chosen by its *last* label.
		{name: "a.b.c.example.com", want: "whois.verisign-grs.com"},
		// Anything unrecognised goes to IANA, whose reply carries the referral.
		{name: "example.zzz", want: "whois.iana.org"},
		{name: "example", want: "whois.iana.org"},
		{name: "", want: "whois.iana.org"},
		// An IP address has no suffix to key on, so IANA again -- and its answer
		// for an address names the regional registry.
		{name: "8.8.8.8", want: "whois.iana.org"},
		// A trailing dot leaves an empty last label, which must not match anything
		// by accident.
		{name: "example.com.", want: "whois.iana.org"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := whoisServerFor(test.name); got != test.want {
				t.Fatalf("whoisServerFor(%q) = %q, want %q", test.name, got, test.want)
			}
		})
	}
}

// whois with no name to look up is an error, and -h with an unreachable server is
// reported rather than hung on.
func TestWhois_reportsWhatItCannotDo(t *testing.T) {
	var stdout, stderr strings.Builder
	if err := newWhoisApplet().Run(t.Context(), []string{}, strings.NewReader(""), &stdout, &stderr); err == nil {
		t.Fatal("whois with no operand was accepted")
	}
	// A port nothing listens on, so the failure is a refused connection.
	err := newWhoisApplet().Run(t.Context(), []string{"-h", "127.0.0.1", "-p", "1", "example.com"},
		strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Skip("something is listening on 127.0.0.1:1")
	}
	if !strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("whois said %q, which does not name the server", err)
	}
}

// Several names are several queries, in order, all to the same server.
func TestWhois_queriesEveryNameGiven(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	asked := make(chan string, 4)
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			line, _ := bufio.NewReader(connection).ReadString('\n')
			asked <- strings.TrimRight(line, "\r\n")
			fmt.Fprintf(connection, "answer for %s\n", strings.TrimRight(line, "\r\n"))
			connection.Close()
		}
	}()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr strings.Builder
	if err := newWhoisApplet().Run(t.Context(), []string{"-h", host, "-p", port, "one.com", "two.org"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("whois: %v (%s)", err, stderr.String())
	}
	for _, want := range []string{"one.com", "two.org"} {
		got := <-asked
		if got != want {
			t.Fatalf("the server was asked %q, want %q", got, want)
		}
	}
	// Both answers reach stdout, concatenated, which is what a multi-name query is.
	for _, want := range []string{"answer for one.com", "answer for two.org"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout is missing %q: %q", want, stdout.String())
		}
	}
}

// ssl_client's operand and option assembly, which decides what is dialled. Every
// case here fails to connect -- port 1 -- and the failure names the address, which
// is how the assembly is observable without a server.
func TestSslClient_assemblesTheAddressItDials(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		// A bare host defaults to 443.
		{name: "host only", args: []string{"127.0.0.1"}, want: "127.0.0.1:443"},
		// A second operand is the port.
		{name: "host and port", args: []string{"127.0.0.1", "1"}, want: "127.0.0.1:1"},
		// -s names the host, which is busybox's spelling.
		{name: "-s names the host", args: []string{"-s", "127.0.0.1"}, want: "127.0.0.1:443"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			err := newSslClientApplet("ssl_client").Run(t.Context(), test.args,
				strings.NewReader(""), &stdout, &stderr)
			if err == nil {
				t.Skip("something answered TLS at " + test.want)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ssl_client %v said %q, which does not name %q", test.args, err, test.want)
			}
		})
	}
	// And no host at all is an error rather than a connection to nothing.
	var stdout, stderr strings.Builder
	if err := newSslClientApplet("ssl_client").Run(t.Context(), []string{},
		strings.NewReader(""), &stdout, &stderr); err == nil {
		t.Fatal("ssl_client with no host was accepted")
	}
}

// syncBuilder is a strings.Builder a second goroutine may write to.
//
// Necessary rather than tidy: nc runs in a goroutine here and writes its
// "listening on" line to stderr while the test polls for it, so an unguarded
// strings.Builder is a data race and `-race` says so. The applet is not at fault --
// a writer handed to an applet is the caller's to synchronise, which is what
// internal/shell/runtime's synchronizedWriter exists for in production.
type syncBuilder struct {
	mutex   sync.Mutex
	builder strings.Builder
}

func (s *syncBuilder) Write(data []byte) (int, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.builder.Write(data)
}

func (s *syncBuilder) String() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.builder.String()
}

// waitForListeningLine polls stderr for the address nc bound.
//
// Polling rather than sleeping, and reading the address rather than assuming a
// port: a fixed port makes a test that fails whenever something else on the
// machine holds it, and CI runners run many jobs at once.
func waitForListeningLine(t *testing.T, stderr *syncBuilder) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		written := stderr.String()
		if index := strings.Index(written, "listening on "); index >= 0 {
			rest := written[index+len("listening on "):]
			if line, _, found := strings.Cut(rest, "\n"); found {
				return strings.TrimSpace(line)
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("nc never reported an address it was listening on: %q", written)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func contextWithCancel(t *testing.T) (context.Context, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	return ctx, cancel
}
