package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/netio"
)

// httpd: serve a directory over HTTP.
//
// The only *listening* applet here, so its safety argument is stated rather than
// assumed:
//
//   - **It binds 127.0.0.1 unless -a says otherwise.** busybox binds every
//     interface by default. Reaching the network is a decision, and the common use
//     -- moving a file between two windows on one machine -- does not need it.
//   - **No CGI.** busybox runs a matching file as a program; that turns a file
//     server into an execution service and there is no way to offer it safely by
//     default.
//   - **Every request path goes through the same containment helper the archivers
//     use**, so a request cannot escape the served root, name a Windows device, or
//     reach a `..` after normalisation. A URL is untrusted input naming a
//     destination, exactly like an archive entry.
//   - No authentication, so nothing is served that the caller did not point it at.
//   - Its parser is the one here a network reaches, so its request line and header are
//     limited; see httpd_request.go, and httpd_serve.go for the server.

func newHttpdApplet() Applet {
	return simpleApplet{name: "httpd", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletOptions(ctx, args, "fv", "pha")
		if err != nil {
			return err
		}
		if len(operands) > 0 {
			return fmt.Errorf("extra operand '%s'", operands[0])
		}
		home := options.value('h')
		if home == "" {
			home = "."
		}
		root, err := resolveHostPath(ProcessViewFromContext(ctx), home)
		if err != nil {
			return operandFailure(home, err)
		}
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			return fmt.Errorf("%s is not a directory to serve", home)
		}
		address, err := httpdAddress(options)
		if err != nil {
			return err
		}
		return serveHTTP(ctx, root, address, options.has('v'), stdout, stderr)
	}}
}

// httpdAddress builds the bind address. The default host is loopback, and -a is
// how somebody asks for more on purpose.
func httpdAddress(options appletOptions) (string, error) {
	port := options.value('p')
	if port == "" {
		// Not 80: binding a privileged port is a second surprise on top of
		// listening at all, and 8080 is what every other quick server uses.
		port = "8080"
	}
	host := options.value('a')
	if host == "" {
		host = "127.0.0.1"
	}
	if strings.Contains(port, ":") {
		// busybox accepts `-p IP:PORT`, so that spelling still works.
		return port, nil
	}
	return netio.JoinHostPort(host, port), nil
}

func serveHTTP(ctx context.Context, root, address string, verbose bool, stdout, stderr io.Writer) error {
	host, port, err := netio.SplitHostPort(address)
	if err != nil {
		return err
	}
	listener, err := netio.Listen(host, port)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %v", address, err)
	}
	defer listener.Close()
	if _, err := fmt.Fprintf(stdout, "serving %s on http://%s/\n", root, listener.Addr()); err != nil {
		return err
	}
	return serveHTTPOn(ctx, listener, &httpdHandler{root: root, verbose: verbose, log: stderr})
}

type httpdHandler struct {
	root    string
	verbose bool
	log     io.Writer
}

// resolve turns a request path into a file inside the root, or refuses.
//
// A URL is untrusted input that names a destination, exactly like an archive
// entry, so it goes through the same helper. Two different things happen to a
// hostile path, and the distinction is worth knowing.
func (h *httpdHandler) resolve(requestPath string) (string, error) {
	// Cleaned against "/" first, which *absorbs* a forward-slash traversal rather
	// than refusing it: `/../../etc/passwd` becomes `etc/passwd` and lands inside
	// the root. That is what net/http does and it is safe -- the escape is gone,
	// not merely reported.
	//
	// What cleaning cannot absorb is everything Windows-shaped: a drive letter, a
	// reserved device name, a component Windows would strip a character from, or a
	// backslash, which path.Clean does not treat as a separator at all. Those go
	// to the shared check below.
	cleaned := strings.TrimPrefix(path.Clean("/"+requestPath), "/")
	if cleaned == "" || cleaned == "." {
		return h.root, nil
	}
	safe, err := safeArchivePath(cleaned)
	if err != nil {
		return "", err
	}
	return filepath.Join(h.root, filepath.FromSlash(safe)), nil
}

// writeListing is the directory index: an index.html if there is one, otherwise
// the names.
func (h *httpdHandler) writeListing(out io.Writer, urlPath, target string, head bool) {
	var body string
	if index := filepath.Join(target, "index.html"); fileExists(index) {
		data, err := os.ReadFile(index)
		if err != nil {
			writeHTTPError(out, 404, "not found", head)
			return
		}
		body = string(data)
	} else {
		entries, err := os.ReadDir(target)
		if err != nil {
			writeHTTPError(out, 404, "not found", head)
			return
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				name += "/"
			}
			names = append(names, name)
		}
		sort.Strings(names)
		var listing strings.Builder
		fmt.Fprintf(&listing, "<html><body><h1>%s</h1>\n", htmlEscape(urlPath))
		for _, name := range names {
			fmt.Fprintf(&listing, "<a href=\"%s\">%s</a><br>\n", htmlEscape(name), htmlEscape(name))
		}
		listing.WriteString("</body></html>\n")
		body = listing.String()
	}
	fields := []string{"Content-Type: text/html; charset=utf-8", "Content-Length: " + strconv.Itoa(len(body))}
	if writeHTTPHead(out, 200, fields) == nil && !head {
		_, _ = io.WriteString(out, body)
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// htmlEscape is enough for a file name in a link. A name is attacker-controlled
// only in the sense that somebody put it on this disk, but a directory holding a
// file called `<script>` should not run it in the browser of whoever listed it.
func htmlEscape(text string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return replacer.Replace(text)
}
