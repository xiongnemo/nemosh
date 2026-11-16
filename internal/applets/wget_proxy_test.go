package applets

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

// runWgetWith is runWget with exactly environ, so nothing the machine has set -- an HTTP_PROXY
// of its own -- decides which way a request goes.
func runWgetWith(t *testing.T, environ []string, args ...string) (string, error) {
	t.Helper()
	var stdout, stderr strings.Builder
	ctx := WithProcessView(context.Background(), hostProcessView{cwd: t.TempDir(), environ: environ})
	err := newWgetApplet().Run(ctx, args, strings.NewReader(""), &stdout, &stderr)
	return stdout.String(), err
}

// An http URL through a proxy is asked for whole, with the proxy's credentials beside it.
func TestWget_asksTheProxyForTheWholeURL(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s %s host=%s %q", r.Method, r.RequestURI, r.Host, r.Header.Get("Proxy-Authorization"))
	}))
	defer proxy.Close()
	withUser := strings.Replace(proxy.URL, "://", "://who:secret@", 1)
	out, err := runWgetWith(t, []string{"http_proxy=" + withUser}, "-q", "-O", "-", "http://example.invalid/a?b=1#c")
	if want := `GET http://example.invalid/a?b=1 host=example.invalid "Basic d2hvOnNlY3JldA=="`; err != nil || out != want {
		t.Errorf("through the proxy: %q, %v; want %q", out, err, want)
	}
}

// An https URL goes through a CONNECT tunnel, and TLS is spoken through it; a proxy that refuses
// the tunnel is an error that says how.
func TestWget_tunnelsHttpsThroughTheProxy(t *testing.T) {
	seen := make(chan []string, 1)
	proxy := fakeProxy(t, "HTTP/1.1 200 Connection established\r\n\r\n", seen)
	_, err := runWgetWith(t, []string{"HTTPS_PROXY=who:secret@" + proxy}, "-q", "-O", "-", "https://example.invalid/")
	if err == nil {
		t.Fatal("a tunnel to nothing fetched something")
	}
	var lines []string
	select {
	case lines = <-seen:
	case <-time.After(10 * time.Second):
		t.Fatal("the proxy was sent nothing")
	}
	if len(lines) == 0 || lines[0] != "CONNECT example.invalid:443 HTTP/1.1" || !slices.Contains(lines, "Proxy-Authorization: Basic d2hvOnNlY3JldA==") ||
		lines[len(lines)-1] != "then 0x16" {
		t.Errorf("the proxy saw %q", lines)
	}
	refusing := fakeProxy(t, "HTTP/1.1 407 Proxy Authentication Required\r\n\r\n", make(chan []string, 1))
	if _, err := runWgetWith(t, []string{"https_proxy=http://" + refusing}, "-q", "-O", "-", "https://example.invalid/"); err == nil ||
		!strings.Contains(err.Error(), "proxy answered CONNECT with HTTP/1.1 407") {
		t.Errorf("a refused tunnel: %v", err)
	}
}

// -Y off goes direct whatever the environment says, and a proxy that is not http:// is refused.
func TestWget_proxyOffAndAProxyNotSpoken(t *testing.T) {
	environ := []string{"http_proxy=socks5://127.0.0.1:1"}
	if _, err := runWgetWith(t, environ, "-q", "-O", "-", "http://example.invalid/"); err == nil ||
		!strings.Contains(err.Error(), "only an http:// proxy is supported") {
		t.Errorf("a socks5 proxy: %v", err)
	}
	for _, off := range [][]string{{"-Y", "off"}, {"--proxy=off"}, {"--proxy", "off"}} {
		args := append(append([]string{"-q"}, off...), "-O", "-", "http://example.invalid/")
		if _, err := runWgetWith(t, environ, args...); err == nil || strings.Contains(err.Error(), "proxy") {
			t.Errorf("%q went to the proxy: %v", off, err)
		}
	}
}

// fakeProxy answers one connection with answer and reports what it was sent, and the first byte
// after the request -- 0x16 is TLS's handshake.
func fakeProxy(t *testing.T, answer string, seen chan<- []string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		var lines []string
		for {
			line, err := reader.ReadString('\n')
			line = strings.TrimRight(line, "\r\n")
			if err != nil || line == "" {
				break
			}
			lines = append(lines, line)
		}
		fmt.Fprint(conn, answer)
		if next, err := reader.ReadByte(); err == nil {
			lines = append(lines, fmt.Sprintf("then %#x", next))
		}
		seen <- lines
	}()
	return listener.Addr().String()
}
