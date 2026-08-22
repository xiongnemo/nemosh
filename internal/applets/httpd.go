package applets

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
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

func newHttpdApplet() Applet {
	return simpleApplet{name: "httpd", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletOptions(args, "fv", "pha")
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
	return net.JoinHostPort(host, port), nil
}

func serveHTTP(ctx context.Context, root, address string, verbose bool, stdout, stderr io.Writer) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %v", address, err)
	}
	if _, err := fmt.Fprintf(stdout, "serving %s on http://%s/\n", root, listener.Addr()); err != nil {
		return err
	}
	server := &http.Server{
		Handler:           &httpdHandler{root: root, verbose: verbose, log: stderr},
		ReadHeaderTimeout: 10 * time.Second,
	}
	// AfterFunc rather than a goroutine on ctx.Done(): a context that is never
	// cancelled has a *nil* Done channel, and receiving from one blocks forever,
	// so the goroutine shape leaked one per call. The capability drift test found
	// this by running `httpd -h DIR` with context.Background().
	defer context.AfterFunc(ctx, func() { server.Close() })()
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

type httpdHandler struct {
	root    string
	verbose bool
	log     io.Writer
}

func (h *httpdHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if h.verbose {
		fmt.Fprintf(h.log, "%s %s %s\n", request.RemoteAddr, request.Method, request.URL.Path)
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		// Nothing here writes, so anything but a read is refused rather than
		// silently treated as a GET.
		http.Error(writer, "only GET and HEAD are served", http.StatusMethodNotAllowed)
		return
	}
	target, err := h.resolve(request.URL.Path)
	if err != nil {
		// The reason is logged and not returned: telling a caller *why* their
		// traversal failed maps out the filesystem for them.
		fmt.Fprintf(h.log, "httpd: refusing %q: %v\n", request.URL.Path, err)
		http.Error(writer, "not found", http.StatusNotFound)
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		http.Error(writer, "not found", http.StatusNotFound)
		return
	}
	if info.IsDir() {
		h.writeListing(writer, request.URL.Path, target)
		return
	}
	file, err := os.Open(target)
	if err != nil {
		http.Error(writer, "not found", http.StatusNotFound)
		return
	}
	defer file.Close()
	http.ServeContent(writer, request, info.Name(), info.ModTime(), file)
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
func (h *httpdHandler) writeListing(writer http.ResponseWriter, urlPath, target string) {
	if index := filepath.Join(target, "index.html"); fileExists(index) {
		// Read and written directly rather than through http.ServeFile: that
		// needs the real *http.Request for range handling, and the first draft
		// here handed it a fabricated one and then wrote the body a second time.
		data, err := os.ReadFile(index)
		if err != nil {
			http.Error(writer, "not found", http.StatusNotFound)
			return
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Write(data)
		return
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		http.Error(writer, "not found", http.StatusNotFound)
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
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(writer, "<html><body><h1>%s</h1>\n", htmlEscape(urlPath))
	for _, name := range names {
		fmt.Fprintf(writer, "<a href=\"%s\">%s</a><br>\n", htmlEscape(name), htmlEscape(name))
	}
	fmt.Fprint(writer, "</body></html>\n")
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
