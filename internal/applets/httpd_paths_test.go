package applets

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// httpd's request paths, driven through a real ServeHTTP rather than through the
// resolver alone.
//
// The resolver is tested directly elsewhere; what this adds is that a *request*
// reaches it in the shape the resolver expects, and that the reply is a 404 rather
// than a reason. net/http does its own decoding before a handler sees the path, and
// that decoding is where the interesting inputs come from -- `%00`, `%2e%2e%2f`,
// and a path that is already absolute.

func serveTestDirectory(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("served\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "inner.txt"), []byte("inner\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := &httpdHandler{root: root, log: &strings.Builder{}}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server, root
}

// A NUL smuggled in as %00 is the reachable-from-the-network form of the bug the
// archive fuzzer found: `NUL%00.txt` decodes to a name whose reserved-device stem
// is `nul\x00`, which was not in the table.
//
// It is a 404 either way -- Go's syscall layer refuses a path with a NUL in it --
// but that is the layer below being careful rather than this code, so the check is
// asserted here at the level a request arrives on.
func TestHttpd_refusesANulSmuggledThroughTheURL(t *testing.T) {
	server, root := serveTestDirectory(t)
	for _, target := range []string{
		"/NUL%00.txt",
		"/nul%00",
		"/f.txt%00.png",
		"/sub%00/inner.txt",
		"/evil.%00",
	} {
		response, err := server.Client().Get(server.URL + target)
		if err != nil {
			// A NUL in a URL that net/http itself refuses to send is also a pass:
			// the request never reaches the handler, which is the same outcome.
			continue
		}
		body := readAndClose(t, response)
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d %q, want 404", target, response.StatusCode, body)
		}
	}
	// Nothing was created in the served directory by any of it.
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("the served directory holds %d entries, want the 2 it started with", len(entries))
	}
}

// An encoded traversal. net/http decodes `%2e%2e%2f` to `../` before the handler
// runs, so this is the same absorption the plain form gets: cleaned against "/"
// makes it a path inside the root rather than a refusal, and the escape is gone
// rather than merely reported.
func TestHttpd_absorbsAnEncodedTraversal(t *testing.T) {
	server, _ := serveTestDirectory(t)
	for _, target := range []string{
		"/%2e%2e%2f%2e%2e%2ff.txt",
		"/sub/%2e%2e/f.txt",
		"/../f.txt",
		"/../../../../f.txt",
	} {
		response, err := server.Client().Get(server.URL + target)
		if err != nil {
			t.Fatalf("GET %s: %v", target, err)
		}
		body := readAndClose(t, response)
		// Absorbed, so it finds f.txt inside the root. That is what net/http does
		// and it is safe: the escape is removed, not reported.
		if response.StatusCode != http.StatusOK || body != "served\n" {
			t.Errorf("GET %s = %d %q, want the absorbed path to reach f.txt", target, response.StatusCode, body)
		}
	}
}

// A backslash cannot be absorbed, because path.Clean does not treat it as a
// separator at all. It is the one traversal shape that has to be *refused*.
func TestHttpd_refusesWhatCleaningCannotAbsorb(t *testing.T) {
	server, _ := serveTestDirectory(t)
	for _, target := range []string{
		`/..%5c..%5cwindows%5cwin.ini`,
		`/a%5c..%5c..%5cevil`,
		"/C:/windows/win.ini",
		"/C:%5Cwindows",
		"/CON",
		"/sub/COM1",
		"/PRN.txt",
	} {
		response, err := server.Client().Get(server.URL + target)
		if err != nil {
			t.Fatalf("GET %s: %v", target, err)
		}
		body := readAndClose(t, response)
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d %q, want 404", target, response.StatusCode, body)
		}
		// The reason is never in the reply: telling a caller *why* their traversal
		// failed maps out the filesystem for them.
		if strings.Contains(body, "reserved") || strings.Contains(body, "drive") || strings.Contains(body, "escape") {
			t.Errorf("GET %s leaked the refusal reason: %q", target, body)
		}
	}
}

// Only GET and HEAD. Nothing here writes, so anything else is refused rather than
// silently treated as a read.
func TestHttpd_refusesEveryMethodThatIsNotARead(t *testing.T) {
	server, _ := serveTestDirectory(t)
	for _, method := range []string{"POST", "PUT", "DELETE", "PATCH", "OPTIONS", "TRACE"} {
		request, err := http.NewRequest(method, server.URL+"/f.txt", strings.NewReader(""))
		if err != nil {
			t.Fatal(err)
		}
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		readAndClose(t, response)
		if response.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s /f.txt = %d, want 405", method, response.StatusCode)
		}
	}
	// HEAD is a read, so it is served -- with no body, which is what makes it HEAD.
	request, err := http.NewRequest(http.MethodHead, server.URL+"/f.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if body := readAndClose(t, response); response.StatusCode != http.StatusOK || body != "" {
		t.Fatalf("HEAD /f.txt = %d %q, want 200 with no body", response.StatusCode, body)
	}
}

// The directory listing, including the escaping a file name needs. A directory
// holding a file called `<script>` must not run it in the browser of whoever
// listed the directory.
func TestHttpd_escapesNamesInItsListing(t *testing.T) {
	root := t.TempDir()
	// Characters a Windows file name may actually hold. `<` and `>` cannot be in
	// one, so the test uses what can: an ampersand and a single quote.
	for _, name := range []string{"a&b.txt", "it's.txt", "plain.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	handler := &httpdHandler{root: root, log: &strings.Builder{}}
	server := httptest.NewServer(handler)
	defer server.Close()

	response, err := server.Client().Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body := readAndClose(t, response)
	if !strings.Contains(body, "a&amp;b.txt") {
		t.Errorf("the ampersand was not escaped: %q", body)
	}
	if strings.Contains(body, "a&b.txt") {
		t.Errorf("the raw ampersand reached the listing: %q", body)
	}
	if !strings.Contains(body, "it&#39;s.txt") {
		t.Errorf("the quote was not escaped: %q", body)
	}
	// A directory is marked with a trailing slash, which is how a listing says
	// which entries can be entered.
	if !strings.Contains(body, "adir/") {
		t.Errorf("the directory was not marked: %q", body)
	}
	// Sorted, so two runs of the same directory answer the same bytes.
	if strings.Index(body, "a&amp;b.txt") > strings.Index(body, "plain.txt") {
		t.Errorf("the listing is not sorted: %q", body)
	}
}

// index.html replaces the listing, and is written exactly once. The first draft
// called http.ServeFile with a fabricated request *and* wrote the bytes, so the
// body arrived twice.
func TestHttpd_servesIndexHtmlExactlyOnce(t *testing.T) {
	root := t.TempDir()
	const page = "<html><body>index</body></html>\n"
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "other.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := &httpdHandler{root: root, log: &strings.Builder{}}
	server := httptest.NewServer(handler)
	defer server.Close()

	response, err := server.Client().Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body := readAndClose(t, response)
	if body != page {
		t.Fatalf("GET / = %q, want the index page exactly once", body)
	}
	// And the listing is gone: other.txt is not named, because index.html replaced
	// the listing rather than being added to it.
	if strings.Contains(body, "other.txt") {
		t.Fatalf("the listing was served alongside index.html: %q", body)
	}
	if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", got)
	}
}

// -v logs one line per request, to stderr rather than to the reply.
func TestHttpd_verboseLogsEachRequest(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	log := &strings.Builder{}
	handler := &httpdHandler{root: root, verbose: true, log: log}
	server := httptest.NewServer(handler)
	defer server.Close()

	for _, target := range []string{"/f.txt", "/missing"} {
		response, err := server.Client().Get(server.URL + target)
		if err != nil {
			t.Fatal(err)
		}
		readAndClose(t, response)
	}
	written := log.String()
	for _, want := range []string{"GET", "/f.txt", "/missing"} {
		if !strings.Contains(written, want) {
			t.Errorf("the log does not mention %q: %q", want, written)
		}
	}
}

// A refusal is logged with its reason even though the reply carries none, because
// the person running the server is the one who should be told.
func TestHttpd_logsTheReasonItRefused(t *testing.T) {
	root := t.TempDir()
	log := &strings.Builder{}
	handler := &httpdHandler{root: root, log: log}
	server := httptest.NewServer(handler)
	defer server.Close()

	response, err := server.Client().Get(server.URL + "/CON")
	if err != nil {
		t.Fatal(err)
	}
	body := readAndClose(t, response)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /CON = %d, want 404", response.StatusCode)
	}
	if strings.Contains(body, "reserved") {
		t.Fatalf("the reply carried the reason: %q", body)
	}
	if !strings.Contains(log.String(), "reserved") {
		t.Fatalf("the log does not carry the reason: %q", log.String())
	}
}

// httpdAddress, which decides what the server binds. The default is loopback and
// that is the whole safety argument, so it is asserted rather than described.
func TestHttpdAddress_defaultsToLoopback(t *testing.T) {
	for _, test := range []struct {
		name  string
		short string
		value string
		want  string
	}{
		{name: "nothing given", want: "127.0.0.1:8080"},
		{name: "a port", short: "p", value: "9999", want: "127.0.0.1:9999"},
		{name: "an address", short: "a", value: "0.0.0.0", want: "0.0.0.0:8080"},
		// busybox accepts -p IP:PORT, so that spelling still works and wins.
		{name: "a port that carries its own host", short: "p", value: "10.0.0.1:8000", want: "10.0.0.1:8000"},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := appletOptions{given: map[byte]bool{}, values: map[byte]string{}}
			if test.short != "" {
				options.given[test.short[0]] = true
				options.values[test.short[0]] = test.value
			}
			got, err := httpdAddress(options)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("httpdAddress = %q, want %q", got, test.want)
			}
		})
	}
}

func readAndClose(t *testing.T, response *http.Response) string {
	t.Helper()
	defer response.Body.Close()
	body := make([]byte, 0, 512)
	chunk := make([]byte, 256)
	for {
		read, err := response.Body.Read(chunk)
		body = append(body, chunk[:read]...)
		if err != nil {
			break
		}
	}
	return string(body)
}

// The applet end to end: it binds, announces the address it bound, serves, and
// stops when its context is cancelled.
//
// This is the only test that runs serveHTTP, which is why it exists -- every other
// httpd test drives the handler directly. Port 0 so the OS chooses and nothing
// collides on a CI runner with many jobs; the announced line is how the test finds
// out which port it got, which is also the line a user relies on.
func TestHttpd_bindsServesAndStopsOnCancel(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("through the applet\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr syncBuilder
	ctx, cancel := contextWithCancel(t)
	finished := make(chan error, 1)
	go func() {
		finished <- newHttpdApplet().Run(ctx,
			[]string{"-p", "0", "-v", "-h", root}, strings.NewReader(""), &stdout, &stderr)
	}()

	// "serving ROOT on http://127.0.0.1:PORT/" is the announcement, and it is on
	// stdout because it is the answer to the command rather than a diagnostic.
	base := waitForServingLine(t, &stdout)
	response, err := http.Get(base + "f.txt")
	if err != nil {
		t.Fatalf("cannot reach the server at %s: %v", base, err)
	}
	if body := readAndClose(t, response); body != "through the applet\n" {
		t.Fatalf("GET f.txt = %q", body)
	}
	// -v logged the request, which is the option's whole effect.
	if !strings.Contains(stderr.String(), "GET") {
		t.Fatalf("-v logged nothing: %q", stderr.String())
	}

	// Cancelling stops it, and cleanly: ErrServerClosed is not an error to report.
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("httpd reported an error on shutdown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("httpd did not stop when its context was cancelled")
	}
}

// -h has to name a directory that exists. Serving something that is not there
// would bind a port and answer 404 to everything, which looks like a working
// server with an empty directory.
func TestHttpd_refusesARootThatIsNotADirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})
	for _, home := range []string{"nope", "f.txt"} {
		var stdout, stderr strings.Builder
		err := newHttpdApplet().Run(ctx, []string{"-p", "0", "-h", home},
			strings.NewReader(""), &stdout, &stderr)
		if err == nil {
			t.Fatalf("httpd -h %q was accepted", home)
		}
		if !strings.Contains(err.Error(), home) {
			t.Fatalf("the error does not name %q: %v", home, err)
		}
	}
}

// An address that cannot be bound is reported rather than hung on.
func TestHttpd_reportsAnAddressItCannotBind(t *testing.T) {
	var stdout, stderr strings.Builder
	err := newHttpdApplet().Run(t.Context(), []string{"-a", "240.0.0.1", "-p", "9"},
		strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("httpd bound an address that is not on this machine")
	}
	if !strings.Contains(err.Error(), "240.0.0.1") {
		t.Fatalf("the error does not name the address: %v", err)
	}
}

// waitForServingLine reads the base URL out of httpd's announcement.
func waitForServingLine(t *testing.T, stdout *syncBuilder) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		written := stdout.String()
		if index := strings.Index(written, "http://"); index >= 0 {
			rest := written[index:]
			if line, _, found := strings.Cut(rest, "\n"); found {
				return strings.TrimSpace(line)
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("httpd never announced an address: %q", written)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
