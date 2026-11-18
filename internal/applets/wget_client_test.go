package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A response is read as it says it is sent: chunked, by a length, or to the close; a HEAD, a 204
// and a 304 have no body; a 1xx before it is passed over; a body short of what it said is an error.
func TestWgetResponse_readsTheBodyAsItIsSent(t *testing.T) {
	for _, test := range []struct {
		name, method, input, body string
		status                    int
		err                       error
	}{
		{name: "a length", method: "GET", input: "HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nhelloEXTRA", status: 200, body: "hello"},
		{name: "chunked", method: "GET", input: "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n3\r\nhel\r\n2;x=y\r\nlo\r\n0\r\nTrailer: z\r\n\r\n", status: 200, body: "hello"},
		{name: "to the close", method: "GET", input: "HTTP/1.0 200 OK\r\n\r\nhello", status: 200, body: "hello"},
		{name: "a HEAD", method: "HEAD", input: "HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\n", status: 200},
		{name: "a 304", method: "GET", input: "HTTP/1.1 304 Not Modified\r\n\r\n", status: 304},
		{name: "a 100 first", method: "GET", input: "HTTP/1.1 100 Continue\r\n\r\nHTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok", status: 200, body: "ok"},
		{name: "a short length", method: "GET", input: "HTTP/1.1 200 OK\r\nContent-Length: 9\r\n\r\nhello", status: 200, body: "hello", err: errTruncatedBody},
		{name: "a short chunk", method: "GET", input: "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n9\r\nhello", status: 200, body: "hello", err: errTruncatedBody},
	} {
		response, err := readHTTPResponse(bufio.NewReader(strings.NewReader(test.input)), test.method)
		if err != nil || response.status != test.status {
			t.Errorf("%s: %d, %v", test.name, response.status, err)
			continue
		}
		body, err := io.ReadAll(response.body)
		if string(body) != test.body || !errors.Is(err, test.err) && (err != nil || test.err != nil) {
			t.Errorf("%s: body %q, %v; want %q, %v", test.name, body, err, test.body, test.err)
		}
	}
	for _, broken := range []string{"", "SMTP ready\r\n", "HTTP/1.1 2000 OK\r\n\r\n", "HTTP/1.1 200 OK\r\nno colon\r\n\r\n",
		"HTTP/1.1 200 OK\r\nX: " + strings.Repeat("a", 70<<10) + "\r\n\r\n"} {
		if _, err := readHTTPResponse(bufio.NewReader(strings.NewReader(broken)), "GET"); err == nil {
			t.Errorf("%.30q was read as a response", broken)
		}
	}
}

// Seeds only: fuzzing runs on CI, never on the machine that develops this.
func FuzzWgetResponse(f *testing.F) {
	f.Add("HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n3\r\nhel\r\n0\r\n\r\n")
	f.Add("HTTP/1.1 200 OK\r\nContent-Length: 3\r\n\r\nabc")
	f.Add("HTTP/1.0 301 Moved\r\nLocation: /x\r\n\r\n")
	f.Fuzz(func(t *testing.T, input string) {
		response, err := readHTTPResponse(bufio.NewReader(strings.NewReader(input)), "GET")
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.body, 1<<20))
		}
	})
}

func runWget(t *testing.T, dir string, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr strings.Builder
	ctx := WithProcessView(context.Background(), hostProcessView{cwd: dir, environ: os.Environ()})
	err := newWgetApplet().Run(ctx, args, strings.NewReader(""), &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

// Redirects are followed, sixteen at most, busybox's limit, and the file is named by the URL that
// was asked for, as busybox names it before it sends a request (networking/wget.c): a release
// asset GitHub redirects to an opaque name was saved as that name. Credentials in the URL go as
// Basic, and not to another host a redirect names.
func TestWget_followsRedirectsAndSendsCredentials(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "authorization %q", r.Header.Get("Authorization"))
	}))
	defer other.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/start":
			http.Redirect(w, r, "/middle", http.StatusMovedPermanently)
		case r.URL.Path == "/middle":
			http.Redirect(w, r, "/final.txt", http.StatusFound)
		case r.URL.Path == "/final.txt":
			fmt.Fprintf(w, "arrived with %q", r.Header.Get("Authorization"))
		case r.URL.Path == "/away":
			http.Redirect(w, r, other.URL+"/there", http.StatusFound)
		case strings.HasPrefix(r.URL.Path, "/loop"):
			http.Redirect(w, r, r.URL.Path+"x", http.StatusFound)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	if _, _, err := runWget(t, dir, "-q", server.URL+"/start"); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "start")); err != nil || string(data) != `arrived with ""` {
		t.Errorf("start holds %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "final.txt")); err == nil {
		t.Error("the file was named by where the redirects ended")
	}
	withUser := strings.Replace(server.URL, "://", "://who:secret@", 1)
	if out, _, err := runWget(t, dir, "-q", "-O", "-", withUser+"/start"); err != nil || out != `arrived with "Basic d2hvOnNlY3JldA=="` {
		t.Errorf("credentials: %q, %v", out, err)
	}
	if out, _, err := runWget(t, dir, "-q", "-O", "-", withUser+"/away"); err != nil || out != `authorization ""` {
		t.Errorf("credentials sent to another host: %q, %v", out, err)
	}
	if _, _, err := runWget(t, dir, "-q", "-O", "-", server.URL+"/loop"); err == nil || !strings.Contains(err.Error(), "too many redirections") {
		t.Errorf("a redirect loop: %v", err)
	}
}

// -S is every response's status line and fields, a redirect's too, in the order they came, as
// busybox prints them: it printed only the last, so a chain was invisible.
func TestWget_showsEveryResponseOfARedirectChain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()
	_, stderr, err := runWget(t, t.TempDir(), "-q", "-S", "-O", "-", server.URL+"/start")
	found, final := strings.Index(stderr, "  HTTP/1.1 302 Found\n"), strings.Index(stderr, "  HTTP/1.1 200 OK\n")
	if err != nil || found < 0 || final < found || !strings.Contains(stderr[found:final], "  Location: /final\n") {
		t.Errorf("-S over a redirect said %q, %v", stderr, err)
	}
}

// -S is the status line and each header field; -T ends a read that waits longer, where the
// whole fetch had thirty seconds.
func TestWget_showsTheServerAndTimesOutAWaitingRead(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stall" {
			w.Header().Set("Content-Length", "10")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			<-release
			return
		}
		w.Header().Set("X-Seen", "yes")
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()
	defer close(release)
	dir := t.TempDir()
	_, stderr, err := runWget(t, dir, "-q", "-S", "-O", "-", server.URL+"/")
	if err != nil || !strings.Contains(stderr, "  HTTP/1.1 200 OK\n") || !strings.Contains(stderr, "  X-Seen: yes\n") {
		t.Errorf("-S said %q, %v", stderr, err)
	}
	start := time.Now()
	if _, _, err := runWget(t, dir, "-q", "-T", "1", "-O", "-", server.URL+"/stall"); err == nil || time.Since(start) > 5*time.Second {
		t.Errorf("a stalled body: %v after %v", err, time.Since(start))
	}
}
