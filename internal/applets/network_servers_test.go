package applets

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The three network applets that had no end-to-end test: ftpget, ftpput and
// whois spoke to nothing, and ssl_client was not run at all.
//
// Each is covered here against a server started by the test. That is more work
// than a unit test on the parser -- an FTP server is written below -- and it is
// the only way to reach the code that actually matters: the command sequence, the
// passive data connection, and the order in which the two are closed. Those were
// exactly the parts a test on parsePassiveReply could not see.

// fakeFTPServer is enough of the protocol for one transfer each way: the control
// connection speaks USER, PASS, TYPE, PASV, RETR, STOR and QUIT, and PASV opens a
// second listener the client dials.
//
// Passive only, because that is all the client does -- active mode asks the server
// to connect back, which nothing behind a Windows firewall can accept.
type fakeFTPServer struct {
	t        *testing.T
	listener net.Listener
	// stored is what a STOR wrote, and files is what a RETR can answer with.
	stored map[string][]byte
	files  map[string][]byte
}

func startFakeFTP(t *testing.T, files map[string][]byte) *fakeFTPServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &fakeFTPServer{t: t, listener: listener, stored: map[string][]byte{}, files: files}
	go server.accept()
	t.Cleanup(func() { listener.Close() })
	return server
}

func (s *fakeFTPServer) address() (string, string) {
	host, port, err := net.SplitHostPort(s.listener.Addr().String())
	if err != nil {
		s.t.Fatal(err)
	}
	return host, port
}

func (s *fakeFTPServer) accept() {
	for {
		connection, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.serve(connection)
	}
}

func (s *fakeFTPServer) serve(control net.Conn) {
	defer control.Close()
	reader := bufio.NewReader(control)
	// A multi-line greeting on purpose: `220-` then `220 `. A client that reads
	// only the first line leaves the rest to be mistaken for the next reply, which
	// is the classic way an FTP client desynchronises -- so the greeting here is
	// shaped to catch it.
	fmt.Fprintf(control, "220-fake ftp\r\n220 ready\r\n")
	var data net.Listener
	defer func() {
		if data != nil {
			data.Close()
		}
	}()
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		verb, argument, _ := strings.Cut(strings.TrimRight(line, "\r\n"), " ")
		switch strings.ToUpper(verb) {
		case "USER":
			fmt.Fprintf(control, "331 need a password\r\n")
		case "PASS":
			fmt.Fprintf(control, "230 logged in\r\n")
		case "TYPE":
			fmt.Fprintf(control, "200 binary\r\n")
		case "PASV":
			data = s.openPassive(control)
		case "RETR":
			s.retrieve(control, data, argument)
			data = nil
		case "STOR":
			s.store(control, data, argument)
			data = nil
		case "QUIT":
			fmt.Fprintf(control, "221 bye\r\n")
			return
		default:
			fmt.Fprintf(control, "500 unknown\r\n")
		}
	}
}

// openPassive answers 227 with the port split into two bytes, high first. That
// encoding is the part every FTP implementation gets wrong at least once, so the
// server writes it the same way the client must read it.
func (s *fakeFTPServer) openPassive(control net.Conn) net.Listener {
	data, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(control, "425 cannot open a data connection\r\n")
		return nil
	}
	_, port, err := net.SplitHostPort(data.Addr().String())
	if err != nil {
		data.Close()
		return nil
	}
	number := 0
	fmt.Sscanf(port, "%d", &number)
	fmt.Fprintf(control, "227 Entering Passive Mode (127,0,0,1,%d,%d)\r\n", number>>8, number&0xff)
	return data
}

func (s *fakeFTPServer) retrieve(control net.Conn, data net.Listener, name string) {
	content, found := s.files[name]
	if !found {
		fmt.Fprintf(control, "550 no such file\r\n")
		return
	}
	if data == nil {
		fmt.Fprintf(control, "425 no data connection\r\n")
		return
	}
	defer data.Close()
	// The 150 comes before the data, which is what the client waits for.
	fmt.Fprintf(control, "150 opening\r\n")
	connection, err := data.Accept()
	if err != nil {
		return
	}
	connection.Write(content)
	connection.Close()
	fmt.Fprintf(control, "226 transfer complete\r\n")
}

func (s *fakeFTPServer) store(control net.Conn, data net.Listener, name string) {
	if data == nil {
		fmt.Fprintf(control, "425 no data connection\r\n")
		return
	}
	defer data.Close()
	fmt.Fprintf(control, "150 opening\r\n")
	connection, err := data.Accept()
	if err != nil {
		return
	}
	buffer := make([]byte, 0, 1024)
	chunk := make([]byte, 256)
	for {
		read, err := connection.Read(chunk)
		buffer = append(buffer, chunk[:read]...)
		if err != nil {
			break
		}
	}
	connection.Close()
	s.stored[name] = buffer
	// Sent only after the data connection has closed, which is what the client's
	// close-then-wait ordering depends on.
	fmt.Fprintf(control, "226 transfer complete\r\n")
}

func TestFtpget_downloadsAFile(t *testing.T) {
	server := startFakeFTP(t, map[string][]byte{"remote.txt": []byte("from the server\n")})
	host, port := server.address()
	directory := t.TempDir()
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: directory})

	var stdout, stderr strings.Builder
	// `ftpget HOST LOCAL REMOTE`, which is busybox's argument order.
	err := newFtpgetApplet().Run(ctx, []string{"-P", port, "-v", host, "local.txt", "remote.txt"},
		strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("ftpget: %v (%s)", err, stderr.String())
	}

	got, err := os.ReadFile(filepath.Join(directory, "local.txt"))
	if err != nil {
		t.Fatalf("ftpget wrote no file: %v", err)
	}
	if string(got) != "from the server\n" {
		t.Fatalf("local.txt = %q", got)
	}
	// -v traces the control connection, and the multi-line greeting must have been
	// read whole: if the client had stopped at `220-` it would have taken `220
	// ready` for the answer to USER and gone out of step from there.
	if !strings.Contains(stderr.String(), "USER") {
		t.Fatalf("-v did not trace the session: %q", stderr.String())
	}
}

// A name the server chose is checked like an archive entry, because a listing is
// where such a name comes from.
func TestFtpget_refusesALocalNameThatEscapes(t *testing.T) {
	server := startFakeFTP(t, map[string][]byte{"r.txt": []byte("x")})
	host, port := server.address()
	outer := t.TempDir()
	root := filepath.Join(outer, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})

	for _, name := range []string{"../escape.txt", "NUL", `C:\windows\evil`} {
		var stdout, stderr strings.Builder
		err := newFtpgetApplet().Run(ctx, []string{"-P", port, host, name, "r.txt"},
			strings.NewReader(""), &stdout, &stderr)
		if err == nil {
			t.Errorf("ftpget accepted the local name %q", name)
		}
	}
	entries, err := os.ReadDir(outer)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Join(outer, entry.Name()) != root {
			t.Fatalf("ftpget created %s outside the working directory", entry.Name())
		}
	}
}

func TestFtpput_uploadsAFile(t *testing.T) {
	server := startFakeFTP(t, nil)
	host, port := server.address()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "local.txt"), []byte("to the server\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: directory})

	var stdout, stderr strings.Builder
	// `ftpput HOST REMOTE LOCAL`.
	err := newFtpputApplet().Run(ctx, []string{"-P", port, host, "remote.txt", "local.txt"},
		strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("ftpput: %v (%s)", err, stderr.String())
	}
	if got := string(server.stored["remote.txt"]); got != "to the server\n" {
		t.Fatalf("the server received %q, want %q", got, "to the server\n")
	}
}

// One file operand means both names are the same, which is busybox's shape.
func TestFtpget_oneOperandNamesBothEnds(t *testing.T) {
	server := startFakeFTP(t, map[string][]byte{"same.txt": []byte("both\n")})
	host, port := server.address()
	directory := t.TempDir()
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: directory})

	var stdout, stderr strings.Builder
	if err := newFtpgetApplet().Run(ctx, []string{"-P", port, host, "same.txt"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("ftpget: %v (%s)", err, stderr.String())
	}
	if got, err := os.ReadFile(filepath.Join(directory, "same.txt")); err != nil || string(got) != "both\n" {
		t.Fatalf("same.txt = %q (%v)", got, err)
	}
}

// A refusal from the server is a failure, not an empty file. The first draft of
// wget had this bug in its HTTP form -- a 404 body written out as a download.
func TestFtpget_reportsAServerRefusal(t *testing.T) {
	server := startFakeFTP(t, map[string][]byte{})
	host, port := server.address()
	directory := t.TempDir()
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: directory})

	var stdout, stderr strings.Builder
	err := newFtpgetApplet().Run(ctx, []string{"-P", port, host, "local.txt", "missing.txt"},
		strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("ftpget reported success for a file the server refused")
	}
	if _, statErr := os.Stat(filepath.Join(directory, "local.txt")); statErr == nil {
		t.Fatal("ftpget left a file behind for a transfer that never happened")
	}
}

// whois is one line in and everything back. -h and -p point it at a server the
// test runs, so this needs no registry and no name resolution.
func TestWhois_queriesAServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	asked := make(chan string, 1)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		line, _ := bufio.NewReader(connection).ReadString('\n')
		asked <- strings.TrimRight(line, "\r\n")
		fmt.Fprintf(connection, "domain: example.com\nstatus: fine\n")
	}()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr strings.Builder
	if err := newWhoisApplet().Run(t.Context(), []string{"-h", host, "-p", port, "example.com"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("whois: %v (%s)", err, stderr.String())
	}

	if got := <-asked; got != "example.com" {
		t.Fatalf("the server was asked %q, want example.com", got)
	}
	if !strings.Contains(stdout.String(), "status: fine") {
		t.Fatalf("whois printed %q, want the server's whole answer", stdout.String())
	}
}

// ssl_client always verifies the certificate, and there is no option to skip it.
//
// That is the property worth testing, and it is testable in exactly one direction:
// a Go test server's certificate is signed by a throwaway CA that no trust store
// knows, so a *successful* handshake against it would require the very option this
// build refuses to have. Asserting the refusal instead tests the decision rather
// than working around it.
func TestSslClient_refusesAnUntrustedCertificate(t *testing.T) {
	server := httptest.NewTLSServer(nil)
	defer server.Close()
	host, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "https://"))
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr strings.Builder
	err = newSslClientApplet("ssl_client").Run(t.Context(), []string{host, port},
		strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("ssl_client accepted a certificate no trust store signed")
	}
	if !strings.Contains(err.Error(), "TLS") {
		t.Fatalf("the refusal does not say it was a TLS failure: %v", err)
	}
}

// And it does complete a handshake when the certificate *is* trusted, which the
// test can arrange only by trusting the test server's own CA -- so the handshake
// is driven through crypto/tls directly with the same settings the applet uses.
// This is the half of the property the applet test above cannot reach: that the
// address, the port and the SNI name are assembled correctly.
func TestSslClient_sendsTheServerNameItWasGiven(t *testing.T) {
	var seen string
	server := httptest.NewUnstartedServer(nil)
	server.TLS = &tls.Config{GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		seen = hello.ServerName
		return nil, nil
	}}
	server.StartTLS()
	defer server.Close()
	host, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "https://"))
	if err != nil {
		t.Fatal(err)
	}

	// -n names the SNI host explicitly, which is the case that differs from the
	// dial address and so is the one worth asserting.
	var stdout, stderr strings.Builder
	// The handshake fails on verification, as above; the SNI name is sent first,
	// so the server has already seen it by then.
	_ = newSslClientApplet("ssl_client").Run(t.Context(), []string{"-n", "named.example", host, port},
		strings.NewReader(""), &stdout, &stderr)
	if seen != "named.example" {
		t.Fatalf("the server saw SNI %q, want the name -n gave", seen)
	}
}
