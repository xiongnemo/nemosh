package applets

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/capability"
)

// The network applets, tested against servers started in the test rather than
// against the internet: a test that needs a name resolved is a test that fails on
// a train.
//
// In package `applets` rather than `applets_test`, because the parsing pieces --
// the passive-mode reply, the WHOIS server table, the containment check on a
// request path -- are where the bugs live and they are unexported.

// A passive-mode reply carries the port as two bytes, high first. It is p1*256+p2
// and not a decimal spelled across two fields, which is the part everyone gets
// wrong.
func TestFtp_parsesThePassiveReply(t *testing.T) {
	for _, test := range []struct {
		reply string
		want  string
	}{
		{reply: "227 Entering Passive Mode (127,0,0,1,195,149)", want: "127.0.0.1:50069"},
		{reply: "227 Entering Passive Mode (10,0,0,7,0,21)", want: "10.0.0.7:21"},
		{reply: "227 PASV ok (192,168,1,5,255,255)", want: "192.168.1.5:65535"},
	} {
		got, err := parsePassiveReply(test.reply)
		if err != nil {
			t.Fatalf("%q: %v", test.reply, err)
		}
		if got != test.want {
			t.Fatalf("%q gave %q, want %q", test.reply, got, test.want)
		}
	}
	for _, bad := range []string{
		"227 no parentheses",
		"227 (1,2,3)",
		"227 (1,2,3,4,5,6,7)",
		"227 (1,2,3,4,5,999)",
		"227 (a,b,c,d,e,f)",
	} {
		if _, err := parsePassiveReply(bad); err == nil {
			t.Fatalf("%q was accepted", bad)
		}
	}
}

func TestWhois_choosesAServerFromTheName(t *testing.T) {
	for name, want := range map[string]string{
		"example.com":  "whois.verisign-grs.com",
		"example.ORG":  "whois.pir.org",
		"thing.io":     "whois.nic.io",
		"what.unknown": "whois.iana.org",
		"nodot":        "whois.iana.org",
	} {
		if got := whoisServerFor(name); got != want {
			t.Fatalf("whoisServerFor(%q) = %q, want %q", name, got, want)
		}
	}
}

// wget against a server started here. The interesting cases are the ones where
// the *server* decides something: the file name, and an error status.
func TestWget_savesAndRefuses(t *testing.T) {
	server := http.Server{}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/hello.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "payload\n")
	})
	mux.HandleFunc("/missing", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	})
	server.Handler = mux
	go server.Serve(listener)
	defer server.Close()
	base := "http://" + listener.Addr().String()

	dir := t.TempDir()
	ctx := WithProcessView(context.Background(), hostProcessView{cwd: dir})
	applet := newWgetApplet()

	// -O - writes the body to stdout.
	var stdout, stderr strings.Builder
	if err := applet.Run(ctx, []string{"-q", "-O", "-", base + "/hello.txt"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("wget -O -: %v (%s)", err, stderr.String())
	}
	if stdout.String() != "payload\n" {
		t.Fatalf("wget -O - = %q", stdout.String())
	}

	// With no -O the name comes from the URL.
	stdout.Reset()
	stderr.Reset()
	if err := applet.Run(ctx, []string{"-q", base + "/hello.txt"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("wget: %v (%s)", err, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, "hello.txt"))
	if err != nil {
		t.Fatalf("wget did not save hello.txt: %v", err)
	}
	if string(data) != "payload\n" {
		t.Fatalf("the saved file holds %q", data)
	}

	// A 404 is a failed download, not a file whose contents are the error page.
	stdout.Reset()
	stderr.Reset()
	if err := applet.Run(ctx, []string{"-q", base + "/missing"}, strings.NewReader(""), &stdout, &stderr); err == nil {
		t.Fatal("wget treated a 404 as a success")
	}
	if _, err := os.Stat(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("wget saved the body of an error response")
	}
}

// The name wget saves under comes from the URL, and the server chose that -- so it
// goes through the same containment check an archive entry does.
func TestWget_refusesAServerChosenNameThatEscapes(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := WithProcessView(context.Background(), hostProcessView{cwd: root})
	applet := newWgetApplet()
	var stdout, stderr strings.Builder
	// -O names the destination directly, which is the reachable way to ask for an
	// escape without a server that redirects.
	for _, name := range []string{"../escape.txt", "NUL", `C:\windows\evil`} {
		stdout.Reset()
		stderr.Reset()
		err := applet.Run(ctx, []string{"-q", "-O", name, "http://127.0.0.1:1/x"}, strings.NewReader(""), &stdout, &stderr)
		if err == nil {
			t.Fatalf("wget -O %q was accepted", name)
		}
	}
	entries, err := os.ReadDir(outer)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Join(outer, entry.Name()) != root {
			t.Fatalf("wget created %s outside the working directory", entry.Name())
		}
	}
}

// httpd's containment is the whole safety argument, so it is asserted directly
// against the resolver rather than only through a request.
func TestHttpd_refusesEveryEscapingRequestPath(t *testing.T) {
	root := filepath.Join("C:", "served")
	handler := &httpdHandler{root: root, log: io.Discard}

	// A forward-slash traversal is *absorbed* rather than refused: cleaning the
	// path against "/" first turns `/../../etc/passwd` into `etc/passwd`, so it
	// lands inside the root. That is what net/http does and it is safe -- but the
	// test has to say which of the two happened, or a version that refused
	// nothing and cleaned nothing would still pass.
	for _, request := range []string{"/../../etc/passwd", "/a/b/../../../escape"} {
		resolved, err := handler.resolve(request)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(resolved, root) {
			t.Fatalf("httpd resolved %q to %q, which is outside the root", request, resolved)
		}
	}
	// These cannot be absorbed and must be refused: a drive letter, a Windows
	// device name, a name Windows would strip a character from, and a backslash
	// traversal that path.Clean does not treat as one.
	for _, request := range []string{
		"/C:/Windows/System32",
		"/NUL",
		"/sub/CON",
		"/evil.",
		`/a\..\..\escape`,
	} {
		if _, err := handler.resolve(request); err == nil {
			t.Fatalf("httpd would have served %q", request)
		}
	}
	// An ordinary path resolves inside the root.
	got, err := handler.resolve("/sub/page.html")
	if err != nil {
		t.Fatalf("httpd refused an ordinary path: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(got), "served/sub/page.html") {
		t.Fatalf("httpd resolved to %q", got)
	}
	// The root itself is the directory.
	if got, err := handler.resolve("/"); err != nil || got != handler.root {
		t.Fatalf("httpd resolved / to %q (%v)", got, err)
	}
}

// The default bind address is loopback, and a test asserts it: busybox binds every
// interface, and reaching the network is a decision rather than a default.
func TestHttpd_bindsLoopbackByDefault(t *testing.T) {
	options, _, err := parseAppletOptions(nil, "fv", "pha")
	if err != nil {
		t.Fatal(err)
	}
	address, err := httpdAddress(options)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(address, "127.0.0.1:") {
		t.Fatalf("httpd would bind %q by default; it must be loopback", address)
	}
	// And 8080 rather than 80, because binding a privileged port is a second
	// surprise on top of listening at all.
	if !strings.HasSuffix(address, ":8080") {
		t.Fatalf("the default port is %q", address)
	}
	// -a is how somebody widens it on purpose.
	widened, _, err := parseAppletOptions([]string{"-a", "0.0.0.0"}, "fv", "pha")
	if err != nil {
		t.Fatal(err)
	}
	address, err = httpdAddress(widened)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(address, "0.0.0.0:") {
		t.Fatalf("httpd -a 0.0.0.0 gave %q", address)
	}
}

// httpd end to end: serve a directory, fetch a file, list an index, and be
// refused an escape.
func TestHttpd_servesAndContains(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "page.txt"), []byte("served\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: &httpdHandler{root: dir, log: io.Discard}}
	go server.Serve(listener)
	defer server.Close()
	base := "http://" + listener.Addr().String()
	client := http.Client{Timeout: 5 * time.Second}

	response, err := client.Get(base + "/page.txt")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if string(body) != "served\n" {
		t.Fatalf("httpd served %q", body)
	}

	// A directory lists its entries.
	response, err = client.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(response.Body)
	response.Body.Close()
	for _, want := range []string{"page.txt", "sub/"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("the listing is missing %q: %q", want, body)
		}
	}

	// A traversal is a 404 rather than a description of why it failed: telling a
	// caller *why* maps out the filesystem for them.
	response, err = client.Get(base + "/../../../../Windows/System32/drivers/etc/hosts")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("a traversal returned %s", response.Status)
	}

	// Only reads are served, because nothing here writes.
	request, _ := http.NewRequest(http.MethodPost, base+"/page.txt", strings.NewReader("x"))
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST returned %s, want 405", response.Status)
	}
}

// nc dials a listener started here and pipes both ways.
func TestNc_pipesBothWays(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		// Read to EOF *before* replying, then close, which is what ends the
		// client's read.
		//
		// Reading to EOF rather than once is what makes this deterministic, and it
		// is the whole regression guard. EOF here means the client shut its write
		// side, so the reply cannot arrive until after nc has finished sending --
		// and a build that returns as soon as stdin is exhausted then reads nothing
		// at all. With a single Read the reply raced nc's own select and the bug was
		// a coin flip per run, which is how it survived to a manual test.
		data, _ := io.ReadAll(connection)
		fmt.Fprintf(connection, "got:%s", data)
	}()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := newNcApplet().Run(ctx, []string{host, port}, strings.NewReader("ping\n"), &stdout, &stderr); err != nil {
		t.Fatalf("nc: %v (%s)", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "got:ping") {
		t.Fatalf("nc read %q, want the echo", stdout.String())
	}
}

// -e would run a program on a connection, which is a remote shell. Refused by
// name rather than quietly absent.
func TestNc_refusesToRunAProgram(t *testing.T) {
	var stdout, stderr strings.Builder
	err := newNcApplet().Run(context.Background(), []string{"-e", "cmd"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("nc -e was accepted")
	}
	if !strings.Contains(err.Error(), "-e") && !strings.Contains(err.Error(), "e") {
		t.Fatalf("nc -e said %q", err)
	}
}

// Every option httpd's capability row claims must be accepted, which is what the
// drift test in internal/capability does for every other applet.
//
// It cannot do it for this one: `httpd -h DIR` is a complete invocation, so the
// drift test bound port 8080 and served until the package timed out ten minutes
// later. The trick here is a *second* operand. httpd rejects an extra operand
// immediately after parsing its options and before it listens, so every case
// below fails for a reason that is not about options -- which is exactly the
// state the drift test reads.
//
// Cancelling the context instead does not work and is worth recording: the applet
// runner checks the context before the applet sees its arguments, so every case
// returned "context canceled" and the test passed without parsing anything.
func TestHttpd_acceptsItsDeclaredOptions(t *testing.T) {
	command, ok := capability.Lookup("httpd")
	if !ok {
		t.Fatal("httpd has no capability row")
	}
	for _, letter := range command.Short {
		t.Run("-"+string(letter), func(t *testing.T) {
			var stdout, stderr strings.Builder
			err := newHttpdApplet().Run(context.Background(),
				[]string{"-" + string(letter), t.TempDir(), "extra"},
				strings.NewReader(""), &stdout, &stderr)
			if err == nil {
				t.Fatalf("httpd -%c with two operands was accepted; it would have listened", letter)
			}
			if strings.Contains(err.Error(), "invalid option") {
				t.Fatalf("httpd claims -%c and refused it: %v", letter, err)
			}
		})
	}
}

// And the reverse: a letter no row claims is refused, so the row above is saying
// something. -Z is the drift test's choice for the same reason.
func TestHttpd_refusesAnUndeclaredOption(t *testing.T) {
	var stdout, stderr strings.Builder
	err := newHttpdApplet().Run(context.Background(), []string{"-Z", t.TempDir(), "extra"},
		strings.NewReader(""), &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "invalid option") {
		t.Fatalf("httpd -Z said %v, want an invalid option", err)
	}
}

// wget's long options, which parseAppletOptions cannot see: it reads letter by
// letter, so --header reaching it is the cluster -h -e -a -d -e -r.
func TestWget_readsItsLongOptions(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		rest    []string
		headers []string
		spider  bool
	}{
		{name: "detached", args: []string{"--header", "A: b", "URL"}, rest: []string{"URL"}, headers: []string{"A: b"}},
		{name: "attached", args: []string{"--header=A: b", "URL"}, rest: []string{"URL"}, headers: []string{"A: b"}},
		{name: "repeated", args: []string{"--header", "A: b", "--header", "C: d", "URL"},
			rest: []string{"URL"}, headers: []string{"A: b", "C: d"}},
		{name: "a flag", args: []string{"--spider", "URL"}, rest: []string{"URL"}, spider: true},
		{name: "mixed with letters", args: []string{"-q", "--spider", "URL"}, rest: []string{"-q", "URL"}, spider: true},
		// The argument of a valued letter is whatever the user wrote, including
		// something shaped like a long option.
		{name: "a value that looks like an option", args: []string{"-U", "--spider", "URL"},
			rest: []string{"-U", "--spider", "URL"}},
		{name: "after the operand", args: []string{"URL", "--spider"}, rest: []string{"URL", "--spider"}},
		{name: "after a double dash", args: []string{"--", "--spider"}, rest: []string{"--", "--spider"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rest, headers, spider, err := wgetLongOptions(test.args)
			if err != nil {
				t.Fatalf("wgetLongOptions(%q): %v", test.args, err)
			}
			if !slices.Equal(rest, test.rest) {
				t.Errorf("rest = %q, want %q", rest, test.rest)
			}
			if !slices.Equal(headers, test.headers) {
				t.Errorf("headers = %q, want %q", headers, test.headers)
			}
			if spider != test.spider {
				t.Errorf("spider = %v, want %v", spider, test.spider)
			}
		})
	}
}

func TestWget_refusesALongOptionItDoesNotHave(t *testing.T) {
	for _, args := range [][]string{{"--mirror", "URL"}, {"--header"}} {
		if _, _, _, err := wgetLongOptions(args); err == nil {
			t.Errorf("wgetLongOptions(%q) was accepted", args)
		}
	}
}

// --header reaches the server, and --spider asks without downloading.
func TestWget_sendsHeadersAndSpiders(t *testing.T) {
	seen := make(chan *http.Request, 4)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		seen <- request
		fmt.Fprint(writer, "body")
	}))
	defer server.Close()

	// When: a header is sent
	var stdout, stderr strings.Builder
	if err := newWgetApplet().Run(t.Context(), []string{"--header", "X-Nemosh: yes", "-O", "-", server.URL},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("wget --header: %v (%s)", err, stderr.String())
	}

	// Then
	request := <-seen
	if got := request.Header.Get("X-Nemosh"); got != "yes" {
		t.Fatalf("X-Nemosh = %q, want yes", got)
	}
	if stdout.String() != "body" {
		t.Fatalf("wget wrote %q", stdout.String())
	}

	// When: --spider asks instead
	directory := t.TempDir()
	stdout.Reset()
	stderr.Reset()
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: directory})
	if err := newWgetApplet().Run(ctx, []string{"--spider", server.URL + "/f.txt"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("wget --spider: %v (%s)", err, stderr.String())
	}

	// Then: a HEAD, and nothing on disk
	if request := <-seen; request.Method != http.MethodHead {
		t.Fatalf("--spider sent %s, want HEAD", request.Method)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("--spider wrote %d files, want none", len(entries))
	}
}
