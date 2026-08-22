package applets

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wget's options and its refusals, against a server the test starts.

// -P saves into a directory, -O names the file, and -O - is stdout. Each decides
// where the body goes, and the name a redirect chose is the one that has to be
// checked -- so all three are worth pinning separately.
func TestWget_decidesWhereTheBodyGoes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprintf(writer, "body of %s", request.URL.Path)
	}))
	defer server.Close()

	t.Run("-P into a directory", func(t *testing.T) {
		root := t.TempDir()
		ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})
		var stdout, stderr strings.Builder
		if err := newWgetApplet().Run(ctx, []string{"-q", "-P", "sub", server.URL + "/a.txt"},
			strings.NewReader(""), &stdout, &stderr); err != nil {
			t.Fatalf("wget -P: %v (%s)", err, stderr.String())
		}
		// The directory is created, because -P names where to put things rather
		// than requiring it to exist.
		got, err := os.ReadFile(filepath.Join(root, "sub", "a.txt"))
		if err != nil {
			t.Fatalf("-P did not save into the directory: %v", err)
		}
		if string(got) != "body of /a.txt" {
			t.Fatalf("saved %q", got)
		}
	})

	t.Run("-O names the file", func(t *testing.T) {
		root := t.TempDir()
		ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})
		var stdout, stderr strings.Builder
		if err := newWgetApplet().Run(ctx, []string{"-q", "-O", "named.dat", server.URL + "/a.txt"},
			strings.NewReader(""), &stdout, &stderr); err != nil {
			t.Fatalf("wget -O: %v (%s)", err, stderr.String())
		}
		if _, err := os.Stat(filepath.Join(root, "named.dat")); err != nil {
			t.Fatalf("-O did not use the name given: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "a.txt")); err == nil {
			t.Fatal("-O saved under the URL's name as well")
		}
	})

	t.Run("a URL with no usable name", func(t *testing.T) {
		root := t.TempDir()
		ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})
		var stdout, stderr strings.Builder
		// A bare host: there is nothing in the path to name the file, which is what
		// an index page looks like.
		if err := newWgetApplet().Run(ctx, []string{"-q", server.URL + "/"},
			strings.NewReader(""), &stdout, &stderr); err != nil {
			t.Fatalf("wget: %v (%s)", err, stderr.String())
		}
		if _, err := os.Stat(filepath.Join(root, "index.html")); err != nil {
			t.Fatalf("a URL with no name did not become index.html: %v", err)
		}
	})
}

// Every status at or above 400 is a failed download rather than a file whose
// contents are the error page. Writing the body would leave something that looks
// like a success.
func TestWget_treatsAnErrorStatusAsAFailure(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 500, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(status)
				fmt.Fprint(writer, "<html>an error page</html>")
			}))
			defer server.Close()
			root := t.TempDir()
			ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})

			var stdout, stderr strings.Builder
			err := newWgetApplet().Run(ctx, []string{"-q", server.URL + "/a.txt"},
				strings.NewReader(""), &stdout, &stderr)
			if err == nil {
				t.Fatalf("a %d was reported as a success", status)
			}
			if !strings.Contains(err.Error(), fmt.Sprint(status)) {
				t.Fatalf("the error does not name the status: %v", err)
			}
			// And no file: the error page's bytes must not be on disk under the
			// name the caller expected content at.
			if _, err := os.Stat(filepath.Join(root, "a.txt")); err == nil {
				t.Fatal("the error page was saved as the download")
			}
		})
	}
	// A 3xx is followed rather than refused, which is what net/http does and what
	// makes the *redirected* name the one that needs checking.
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(writer, "after the redirect")
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL+"/final.txt", http.StatusFound)
	}))
	defer redirector.Close()
	root := t.TempDir()
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})
	var stdout, stderr strings.Builder
	if err := newWgetApplet().Run(ctx, []string{"-q", redirector.URL + "/start.txt"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("a redirect was not followed: %v (%s)", err, stderr.String())
	}
	// Saved under the name the *server* chose at the end of the chain, which is
	// exactly why that name goes through the containment check.
	if got, err := os.ReadFile(filepath.Join(root, "final.txt")); err != nil || string(got) != "after the redirect" {
		t.Fatalf("the redirected download landed as %q (%v)", got, err)
	}
}

// -S prints the status line to stderr, and -q silences the saved-size line. Both
// write to stderr so that `-O -` stays clean for a pipe.
func TestWget_reportingGoesToStderrOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(writer, "payload")
	}))
	defer server.Close()
	root := t.TempDir()
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})

	var stdout, stderr strings.Builder
	if err := newWgetApplet().Run(ctx, []string{"-S", "-O", "-", server.URL + "/a.txt"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("wget -S -O -: %v (%s)", err, stderr.String())
	}
	// The body and nothing else on stdout, which is what makes -O - usable in a
	// pipeline.
	if stdout.String() != "payload" {
		t.Fatalf("stdout = %q, want only the body", stdout.String())
	}
	if !strings.Contains(stderr.String(), "200") {
		t.Fatalf("-S did not print the status: %q", stderr.String())
	}

	// Without -q the saved line names the file and its size; with -q it does not.
	stderr.Reset()
	if err := newWgetApplet().Run(ctx, []string{server.URL + "/loud.txt"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "loud.txt") || !strings.Contains(stderr.String(), "7") {
		t.Fatalf("the saved line does not name the file and its size: %q", stderr.String())
	}
	stderr.Reset()
	if err := newWgetApplet().Run(ctx, []string{"-q", server.URL + "/quiet.txt"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stderr.String(), "quiet.txt") {
		t.Fatalf("-q still reported the save: %q", stderr.String())
	}
}

// -U sets the User-Agent, and the default names this build rather than
// impersonating something else.
func TestWget_sendsAUserAgent(t *testing.T) {
	seen := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		seen <- request.Header.Get("User-Agent")
		fmt.Fprint(writer, "x")
	}))
	defer server.Close()
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: t.TempDir()})

	var stdout, stderr strings.Builder
	if err := newWgetApplet().Run(ctx, []string{"-q", "-O", "-", server.URL + "/a"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if got := <-seen; got != "nemosh-wget" {
		t.Fatalf("the default User-Agent is %q, want nemosh-wget", got)
	}
	if err := newWgetApplet().Run(ctx, []string{"-q", "-U", "custom/1.0", "-O", "-", server.URL + "/a"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if got := <-seen; got != "custom/1.0" {
		t.Fatalf("-U sent %q", got)
	}
	// --header wins over -U, because the more specific spelling is the later one
	// and dropping it silently would be worse than either.
	if err := newWgetApplet().Run(ctx,
		[]string{"-q", "-U", "from-dash-u", "--header", "User-Agent: from-header", "-O", "-", server.URL + "/a"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if got := <-seen; got != "from-header" {
		t.Fatalf("--header did not win over -U: %q", got)
	}
}

// The refusals: a URL that cannot be parsed, a header that is not NAME: VALUE, a
// timeout that is not a number, and no URL at all.
func TestWget_refusalsAndTheirReasons(t *testing.T) {
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: t.TempDir()})
	for _, test := range []struct {
		name    string
		args    []string
		because string
	}{
		{name: "no URL", args: []string{"-q"}, because: "URL"},
		{name: "a header with no colon", args: []string{"--header", "novalue", "http://127.0.0.1:1/"}, because: "NAME"},
		{name: "a timeout that is not a number", args: []string{"-T", "soon", "http://127.0.0.1:1/"}, because: "soon"},
		{name: "a long option it does not have", args: []string{"--recursive", "http://127.0.0.1:1/"}, because: "recursive"},
		{name: "--header with no argument", args: []string{"--header"}, because: "header"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			err := newWgetApplet().Run(ctx, test.args, strings.NewReader(""), &stdout, &stderr)
			if err == nil {
				t.Fatalf("wget %v was accepted", test.args)
			}
			if !strings.Contains(err.Error(), test.because) {
				t.Fatalf("wget %v said %q, which does not mention %q", test.args, err, test.because)
			}
		})
	}
}

// A host that cannot be reached is an error naming it. This uses a port nothing
// listens on rather than a name that must be resolved, so the test does not need
// the network.
func TestWget_reportsAHostItCannotReach(t *testing.T) {
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: t.TempDir()})
	var stdout, stderr strings.Builder
	err := newWgetApplet().Run(ctx, []string{"-q", "-T", "3", "http://127.0.0.1:1/a.txt"},
		strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Skip("something answered on 127.0.0.1:1")
	}
	if !strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Fatalf("wget said %q, which does not name the address", err)
	}
}

// A URL with no scheme is assumed to be http, which is what wget does and what
// anyone typing `wget example.com/x` means.
func TestWget_assumesHttpForABareHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(writer, "no scheme needed")
	}))
	defer server.Close()
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: t.TempDir()})

	var stdout, stderr strings.Builder
	// The server's URL without its http:// prefix.
	bare := strings.TrimPrefix(server.URL, "http://")
	if err := newWgetApplet().Run(ctx, []string{"-q", "-O", "-", bare + "/a.txt"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("a bare host was not assumed to be http: %v (%s)", err, stderr.String())
	}
	if stdout.String() != "no scheme needed" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

// Several URLs are several downloads, and the first failure stops the rest --
// because a script that asked for five files and got two should not exit 0.
func TestWget_fetchesEveryUrlAndStopsAtTheFirstFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.Path, "missing") {
			http.Error(writer, "gone", http.StatusNotFound)
			return
		}
		fmt.Fprintf(writer, "body of %s", request.URL.Path)
	}))
	defer server.Close()
	root := t.TempDir()
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})

	var stdout, stderr strings.Builder
	if err := newWgetApplet().Run(ctx, []string{"-q", server.URL + "/one.txt", server.URL + "/two.txt"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("two URLs: %v (%s)", err, stderr.String())
	}
	for _, name := range []string{"one.txt", "two.txt"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("%s was not saved: %v", name, err)
		}
	}

	err := newWgetApplet().Run(ctx, []string{"-q", server.URL + "/missing.txt", server.URL + "/after.txt"},
		strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("a failing URL among several was reported as a success")
	}
	if _, err := os.Stat(filepath.Join(root, "after.txt")); err == nil {
		t.Fatal("the URL after the failure was still fetched")
	}
}

// parsePassiveReply, whose two-byte port is the part every FTP client gets wrong.
// Already covered for the happy path; these are the malformed ones.
func TestParsePassiveReply_refusesWhatItCannotRead(t *testing.T) {
	for _, reply := range []string{
		"227 Entering Passive Mode",
		"227 Entering Passive Mode (1,2,3,4,5)",
		"227 Entering Passive Mode (1,2,3,4,5,6,7)",
		"227 Entering Passive Mode (1,2,3,4,5,256)",
		"227 Entering Passive Mode (1,2,3,4,5,-1)",
		"227 Entering Passive Mode (1,2,3,4,5,abc)",
		"227 (1,2,3,4,5,6",
		"227 1,2,3,4,5,6)",
		"",
	} {
		if _, err := parsePassiveReply(reply); err == nil {
			t.Errorf("parsePassiveReply(%q) was accepted", reply)
		}
	}
	// And the boundary values that are valid: 0 and 255 in every position.
	for _, test := range []struct{ reply, want string }{
		{reply: "227 (0,0,0,0,0,0)", want: "0.0.0.0:0"},
		{reply: "227 (255,255,255,255,255,255)", want: "255.255.255.255:65535"},
		// The port is p1*256+p2, not a decimal spelled across two fields.
		{reply: "227 (127,0,0,1,4,1)", want: "127.0.0.1:1025"},
		// Spaces around the numbers, which some servers write.
		{reply: "227 ( 127, 0, 0, 1, 4, 1 )", want: "127.0.0.1:1025"},
	} {
		got, err := parsePassiveReply(test.reply)
		if err != nil {
			t.Errorf("parsePassiveReply(%q): %v", test.reply, err)
			continue
		}
		if got != test.want {
			t.Errorf("parsePassiveReply(%q) = %q, want %q", test.reply, got, test.want)
		}
	}
}

// valueOrDefault, which supplies the anonymous credentials. Small, and the reason
// it is tested is that an empty password is a different thing from no password.
func TestValueOrDefault(t *testing.T) {
	if got := valueOrDefault("", "fallback"); got != "fallback" {
		t.Errorf("an empty value = %q, want the fallback", got)
	}
	if got := valueOrDefault("given", "fallback"); got != "given" {
		t.Errorf("a given value = %q", got)
	}
}
