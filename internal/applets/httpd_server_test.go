package applets

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/netio"
)

// testHTTPServer is httpd's own server answering on a port of this machine, for Go's http.Client
// to ask: the client is the independent half, written by someone else.
type testHTTPServer struct{ URL string }

func (testHTTPServer) Client() *http.Client { return &http.Client{Timeout: 5 * time.Second} }

func startHttpd(t *testing.T, root string) testHTTPServer {
	t.Helper()
	return startHttpdWith(t, &httpdHandler{root: root, log: io.Discard})
}

func startHttpdWith(t *testing.T, handler *httpdHandler) testHTTPServer {
	t.Helper()
	listener, err := netio.Listen("127.0.0.1", "0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveHTTPOn(ctx, listener, handler) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("the server ended with %v", err)
		}
	})
	return testHTTPServer{URL: "http://" + listener.Addr()}
}

// syncLog is a log the server writes from its goroutines and a test reads, which the race
// detector cannot see ordered through a socket.
type syncLog struct {
	mu   sync.Mutex
	text strings.Builder
}

func (l *syncLog) Write(data []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.text.Write(data)
}

func (l *syncLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.text.String()
}

// A request's line and header are read within limits, its target's path decoded, and each
// refusal is the status that says why.
func TestHttpdRequest_isReadWithinLimits(t *testing.T) {
	for _, test := range []struct {
		name, input, path string
		status            int
	}{
		{name: "a GET", input: "GET /a%20b/c.txt?x=1 HTTP/1.1\r\nHost: h\r\nRange: bytes=1-2\r\n\r\n", path: "/a b/c.txt"},
		{name: "absolute form", input: "GET http://h:8080/x HTTP/1.0\r\n\r\n", path: "/x"},
		{name: "a bare LF", input: "HEAD / HTTP/1.1\nHost: h\n\n", path: "/"},
		{name: "no version", input: "GET /\r\n\r\n", status: 400},
		{name: "not a path", input: "GET x HTTP/1.1\r\n\r\n", status: 400},
		{name: "a bad escape", input: "GET /%zz HTTP/1.1\r\n\r\n", status: 400},
		{name: "a field with no colon", input: "GET / HTTP/1.1\r\nbroken\r\n\r\n", status: 400},
		{name: "a long line", input: "GET /" + strings.Repeat("a", 9000) + " HTTP/1.1\r\n\r\n", status: 414},
		{name: "a long header", input: "GET / HTTP/1.1\r\nX: " + strings.Repeat("a", 17000) + "\r\n\r\n", status: 431},
		{name: "too many fields", input: "GET / HTTP/1.1\r\n" + strings.Repeat("X: y\r\n", 101) + "\r\n", status: 431},
	} {
		request, err := readHTTPRequest(bufio.NewReader(strings.NewReader(test.input)))
		refused, isRefusal := err.(httpError)
		switch {
		case test.status != 0 && (!isRefusal || refused.status != test.status):
			t.Errorf("%s: %v; want %d", test.name, err, test.status)
		case test.status == 0 && (err != nil || request.path != test.path):
			t.Errorf("%s: %+v, %v; want path %q", test.name, request, err, test.path)
		}
	}
}

// A range is a 206 of those bytes, one past the end a 416, several the whole file; an
// If-Modified-Since not older than the file is a 304; HEAD is the head alone; a method that is
// not a read is 405 with the two that are.
func TestHttpd_answersRangesDatesAndMethods(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := startHttpd(t, root)
	ask := func(method, rangeHeader, since string) (*http.Response, string) {
		request, _ := http.NewRequest(method, server.URL+"/f.txt", nil)
		if rangeHeader != "" {
			request.Header.Set("Range", rangeHeader)
		}
		if since != "" {
			request.Header.Set("If-Modified-Since", since)
		}
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		return response, string(body)
	}
	for _, test := range []struct {
		method, rangeHeader, since string
		status                     int
		body, contentRange         string
	}{
		{method: "GET", status: 200, body: "0123456789"},
		{method: "GET", rangeHeader: "bytes=2-4", status: 206, body: "234", contentRange: "bytes 2-4/10"},
		{method: "GET", rangeHeader: "bytes=7-", status: 206, body: "789", contentRange: "bytes 7-9/10"},
		{method: "GET", rangeHeader: "bytes=-3", status: 206, body: "789", contentRange: "bytes 7-9/10"},
		{method: "GET", rangeHeader: "bytes=20-", status: 416, contentRange: "bytes */10"},
		{method: "GET", rangeHeader: "bytes=0-1,4-5", status: 200, body: "0123456789"},
		{method: "GET", since: time.Now().Add(time.Hour).UTC().Format(http.TimeFormat), status: 304},
		{method: "GET", since: "Mon, 02 Jan 2006 15:04:05 GMT", status: 200, body: "0123456789"},
		{method: "HEAD", status: 200},
		{method: "DELETE", status: 405, body: "only GET and HEAD are served\n"},
	} {
		response, body := ask(test.method, test.rangeHeader, test.since)
		if response.StatusCode != test.status || body != test.body || response.Header.Get("Content-Range") != test.contentRange {
			t.Errorf("%s %q %q: %d %q %q; want %d %q %q", test.method, test.rangeHeader, test.since,
				response.StatusCode, body, response.Header.Get("Content-Range"), test.status, test.body, test.contentRange)
		}
		if test.status == 405 && response.Header.Get("Allow") != "GET, HEAD" {
			t.Errorf("405 said Allow %q", response.Header.Get("Allow"))
		}
		if test.method == "HEAD" && response.ContentLength != 10 {
			t.Errorf("HEAD said Content-Length %d", response.ContentLength)
		}
	}
	if response, _ := ask("GET", "", ""); response.Header.Get("Content-Type") != "text/plain; charset=utf-8" || response.Header.Get("Last-Modified") == "" {
		t.Errorf("headers %v", response.Header)
	}
}
