package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xiongnemo/nemosh/internal/netio"
)

// httpd's server, its own over netio rather than net/http's: one request a connection, which is
// what a file server for a browser or a wget needs, at most 64 connections at once, ten seconds to
// send a request's header and a minute for each write of the answer. It answers GET and HEAD, a
// file with its Last-Modified, a single range as 206 and one past the end as 416, an
// If-Modified-Since not older than the file as 304, and anything else 405 with the methods it
// does take.

const (
	httpdConnections   = 64
	httpdHeaderTimeout = 10 * time.Second
	httpdWriteTimeout  = time.Minute
	httpTimeFormat     = "Mon, 02 Jan 2006 15:04:05 GMT"
)

// serveHTTPOn answers connections on listener until ctx is cancelled.
func serveHTTPOn(ctx context.Context, listener netio.Listener, handler *httpdHandler) error {
	defer context.AfterFunc(ctx, func() { listener.Close() })()
	slots := make(chan struct{}, httpdConnections)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, netio.ErrClosed) {
				return nil
			}
			return err
		}
		slots <- struct{}{}
		go func() {
			defer func() { <-slots }()
			handler.serveConn(conn)
		}()
	}
}

func (h *httpdHandler) serveConn(conn netio.Conn) {
	defer func() {
		// The rest of what the client sent is read and dropped before the socket goes, so the
		// answer is not cut off by a reset: a refused POST's body, say.
		_ = conn.CloseWrite()
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		_, _ = io.CopyN(io.Discard, conn, 256<<10)
		conn.Close()
	}()
	_ = conn.SetReadDeadline(time.Now().Add(httpdHeaderTimeout))
	request, err := readHTTPRequest(bufio.NewReaderSize(conn, 4096))
	_ = conn.SetReadDeadline(time.Time{})
	out := &deadlineWriter{conn: conn}
	var refused httpError
	switch {
	case errors.As(err, &refused):
		writeHTTPError(out, refused.status, refused.reason, false)
		return
	case err != nil:
		return
	}
	if h.verbose {
		fmt.Fprintf(h.log, "%s %s %s\n", conn.RemoteAddr(), request.method, request.path)
	}
	h.serve(out, request)
}

// deadlineWriter gives each write of an answer a minute.
type deadlineWriter struct{ conn netio.Conn }

func (w *deadlineWriter) Write(data []byte) (int, error) {
	_ = w.conn.SetWriteDeadline(time.Now().Add(httpdWriteTimeout))
	return w.conn.Write(data)
}

func (h *httpdHandler) serve(out io.Writer, request httpRequest) {
	head := request.method == "HEAD"
	if request.method != "GET" && !head {
		// Nothing here writes, so anything but a read is refused rather than silently treated
		// as a GET.
		writeHTTPError(out, 405, "only GET and HEAD are served", false)
		return
	}
	target, err := h.resolve(request.path)
	if err != nil {
		// The reason is logged and not returned: telling a caller *why* their traversal failed
		// maps out the filesystem for them.
		fmt.Fprintf(h.log, "httpd: refusing %q: %v\n", request.path, err)
		writeHTTPError(out, 404, "not found", head)
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		writeHTTPError(out, 404, "not found", head)
		return
	}
	if info.IsDir() {
		h.writeListing(out, request.path, target, head)
		return
	}
	h.serveFile(out, request, target, info, head)
}

func (h *httpdHandler) serveFile(out io.Writer, request httpRequest, target string, info os.FileInfo, head bool) {
	file, err := os.Open(target)
	if err != nil {
		writeHTTPError(out, 404, "not found", head)
		return
	}
	defer file.Close()
	modified := info.ModTime().UTC().Truncate(time.Second)
	if since, err := time.Parse(httpTimeFormat, request.ifModifiedSince); err == nil && !modified.After(since) {
		writeHTTPHead(out, 304, []string{"Last-Modified: " + modified.Format(httpTimeFormat)})
		return
	}
	size := info.Size()
	fields := []string{"Content-Type: " + contentType(target, file), "Last-Modified: " + modified.Format(httpTimeFormat), "Accept-Ranges: bytes"}
	status, first, length := 200, int64(0), size
	if span, asked, satisfiable := parseRange(request.rangeHeader, size); asked && !satisfiable {
		writeHTTPHead(out, 416, []string{"Content-Range: bytes */" + strconv.FormatInt(size, 10), "Content-Length: 0"})
		return
	} else if asked {
		status, first, length = 206, span.first, span.last-span.first+1
		fields = append(fields, fmt.Sprintf("Content-Range: bytes %d-%d/%d", span.first, span.last, size))
	}
	fields = append(fields, "Content-Length: "+strconv.FormatInt(length, 10))
	if err := writeHTTPHead(out, status, fields); err != nil || head {
		return
	}
	if _, err := file.Seek(first, io.SeekStart); err == nil {
		_, _ = io.CopyN(out, file, length)
	}
}

// contentType is the type the name's extension has, or what the first bytes look like: text if
// they are UTF-8 with no NUL, bytes otherwise.
func contentType(name string, file *os.File) string {
	if known := mime.TypeByExtension(filepath.Ext(name)); known != "" {
		return known
	}
	sample := make([]byte, 512)
	read, _ := file.Read(sample)
	_, _ = file.Seek(0, io.SeekStart)
	if utf8.Valid(sample[:read]) && !strings.ContainsRune(string(sample[:read]), 0) {
		return "text/plain; charset=utf-8"
	}
	return "application/octet-stream"
}

var httpStatusText = [...]struct {
	code int
	text string
}{{200, "OK"}, {206, "Partial Content"}, {304, "Not Modified"}, {400, "Bad Request"}, {404, "Not Found"},
	{405, "Method Not Allowed"}, {414, "URI Too Long"}, {416, "Requested Range Not Satisfiable"},
	{431, "Request Header Fields Too Large"}}

// writeHTTPHead writes the status line and fields of an answer, and the blank line after them.
func writeHTTPHead(out io.Writer, status int, fields []string) error {
	text := ""
	for _, known := range httpStatusText {
		if known.code == status {
			text = known.text
		}
	}
	var head strings.Builder
	fmt.Fprintf(&head, "HTTP/1.1 %d %s\r\nDate: %s\r\nConnection: close\r\n", status, text, time.Now().UTC().Format(httpTimeFormat))
	for _, field := range fields {
		head.WriteString(field + "\r\n")
	}
	head.WriteString("\r\n")
	_, err := io.WriteString(out, head.String())
	return err
}

// writeHTTPError is a refusal, its reason as plain text, as net/http's Error writes one; a 405
// says which methods are served.
func writeHTTPError(out io.Writer, status int, reason string, head bool) {
	body := reason + "\n"
	fields := []string{"Content-Type: text/plain; charset=utf-8", "X-Content-Type-Options: nosniff", "Content-Length: " + strconv.Itoa(len(body))}
	if status == 405 {
		fields = append(fields, "Allow: GET, HEAD")
	}
	if writeHTTPHead(out, status, fields) == nil && !head {
		_, _ = io.WriteString(out, body)
	}
}
