package netio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// trustOnly has Client verify by a chain engine that trusts der and nothing else, so a test server's
// certificate, which no trust store knows, is trusted as a public one is. It is the engine that
// changes and not the check, so the check the tests reach is the one every connection gets.
func trustOnly(t *testing.T, der []byte) {
	t.Helper()
	store, err := windows.CertOpenStore(windows.CERT_STORE_PROV_MEMORY, 0, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := windows.CertCreateCertificateContext(windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING, &der[0], uint32(len(der)))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CertFreeCertificateContext(certificate)
	if err := windows.CertAddCertificateContextToStore(store, certificate, windows.CERT_STORE_ADD_ALWAYS, nil); err != nil {
		t.Fatal(err)
	}
	config := struct { // CERT_CHAIN_ENGINE_CONFIG
		size                                             uint32
		restrictedRoot, restrictedTrust, restrictedOther windows.Handle
		additional                                       uint32
		additionalStores                                 *windows.Handle
		flags, timeout, cached, cycles                   uint32
		exclusiveRoot, exclusivePeople                   windows.Handle
		exclusiveFlags                                   uint32
	}{exclusiveRoot: store}
	config.size = uint32(unsafe.Sizeof(config))
	crypt32 := windows.NewLazySystemDLL("crypt32.dll")
	var engine windows.Handle
	if made, _, err := crypt32.NewProc("CertCreateCertificateChainEngine").Call(uintptr(unsafe.Pointer(&config)), uintptr(unsafe.Pointer(&engine))); made == 0 {
		t.Fatal(err)
	}
	before := chainEngine
	chainEngine = engine
	t.Cleanup(func() {
		chainEngine = before
		crypt32.NewProc("CertFreeCertificateChainEngine").Call(uintptr(engine))
		windows.CertCloseStore(store, 0)
	})
}

// tlsTo dials server and speaks TLS to it as name.
func tlsTo(t *testing.T, server *httptest.Server, name string) (Conn, error) {
	t.Helper()
	host, port, err := SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	plain, err := Dial(context.Background(), host, port, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { plain.Close() })
	return Client(plain, name)
}

// A request too large for one record goes out as several, a reply of many records comes back
// whole, the server is sent the name it was asked for, and its close_notify is the end.
func TestClient_speaksTLSToAServerItTrusts(t *testing.T) {
	large := strings.Repeat("0123456789abcdef", 20<<10)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fmt.Fprintf(w, "%s %s %d %s|", r.Method, r.Host, len(body), r.TLS.ServerName)
		io.WriteString(w, large)
	}))
	defer server.Close()
	trustOnly(t, server.Certificate().Raw)
	conn, err := tlsTo(t, server, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	sent := strings.Repeat("x", 80<<10)
	if n, err := io.WriteString(conn, "POST / HTTP/1.0\r\nHost: example.com\r\nContent-Length: 81920\r\n\r\n"+sent); err != nil || n < len(sent) {
		t.Fatalf("wrote %d, %v", n, err)
	}
	reply, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("read %d bytes, then %v", len(reply), err)
	}
	_, body, _ := strings.Cut(string(reply), "\r\n\r\n")
	if want := "POST example.com 81920 example.com|" + large; body != want {
		t.Errorf("the reply was %d bytes, %.60q; want %d", len(body), body, len(want))
	}
	if n, err := conn.Read(make([]byte, 1)); n != 0 || err != io.EOF {
		t.Errorf("a read after the end: %d, %v", n, err)
	}
}

// A certificate for another name, or one no trusted root signed, is refused, saying which.
func TestClient_refusesACertificateItShould(t *testing.T) {
	server := httptest.NewTLSServer(http.NotFoundHandler())
	defer server.Close()
	if _, err := tlsTo(t, server, "example.com"); err == nil || !strings.Contains(err.Error(), "signed by unknown authority") {
		t.Errorf("an untrusted root: %v", err)
	}
	trustOnly(t, server.Certificate().Raw)
	if _, err := tlsTo(t, server, "other.example"); err == nil || !strings.Contains(err.Error(), "not valid for other.example") {
		t.Errorf("another name: %v", err)
	}
}

// A read waiting on a quiet server ends at its deadline, and the connection is still good after;
// one ended by Close says it was closed.
func TestClient_deadlineAndCloseEndAWaitingRead(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		io.WriteString(w, "late")
	}))
	defer server.Close()
	defer close(release)
	trustOnly(t, server.Certificate().Raw)
	conn, err := tlsTo(t, server, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, "GET / HTTP/1.0\r\nHost: example.com\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if _, err := conn.Read(make([]byte, 64)); !errors.Is(err, ErrTimeout) {
		t.Fatalf("a read past its deadline: %v", err)
	}
	conn.SetReadDeadline(time.Time{})
	ended := make(chan error, 1)
	go func() {
		_, err := conn.Read(make([]byte, 64))
		ended <- err
	}()
	time.Sleep(50 * time.Millisecond)
	conn.Close()
	select {
	case err := <-ended:
		if !errors.Is(err, ErrClosed) {
			t.Errorf("a read Close ended: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not end a waiting read")
	}
}
