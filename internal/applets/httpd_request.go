package applets

import (
	"bufio"
	"errors"
	"strconv"
	"strings"
)

// An HTTP/1.x request as httpd reads one: the request line, the header fields it uses, and limits
// on both, since it is the one parser here a network reaches. A request line past 8 KiB is 414; a
// header past 16 KiB or 100 fields is 431; anything that is not a request is 400.

const (
	httpdRequestLineLimit = 8 << 10
	httpdHeaderLimit      = 16 << 10
	httpdHeaderFields     = 100
)

type httpRequest struct {
	method          string
	path            string // the target's path, decoded
	rangeHeader     string
	ifModifiedSince string
}

// httpError is a request refused before it is served, with the status that says why.
type httpError struct {
	status int
	reason string
}

func (e httpError) Error() string { return e.reason }

// readHTTPRequest reads one request's line and header from reader.
func readHTTPRequest(reader *bufio.Reader) (httpRequest, error) {
	line, err := readHTTPLine(reader, httpdRequestLineLimit, 414)
	if err != nil {
		return httpRequest{}, err
	}
	method, rest, ok1 := strings.Cut(line, " ")
	target, version, ok2 := strings.Cut(rest, " ")
	if !ok1 || !ok2 || method == "" || target == "" || !strings.HasPrefix(version, "HTTP/1.") {
		return httpRequest{}, httpError{400, "not a request line"}
	}
	path, err := requestPath(target)
	if err != nil {
		return httpRequest{}, err
	}
	request := httpRequest{method: method, path: path}
	total := 0
	for fields := 0; ; fields++ {
		field, err := readHTTPLine(reader, httpdHeaderLimit-total, 431)
		if err != nil {
			return httpRequest{}, err
		}
		total += len(field) + 2
		if field == "" {
			return request, nil
		}
		if fields >= httpdHeaderFields {
			return httpRequest{}, httpError{431, "too many header fields"}
		}
		name, value, found := strings.Cut(field, ":")
		if !found || name == "" || strings.ContainsAny(name, " \t") {
			return httpRequest{}, httpError{400, "not a header field"}
		}
		value = strings.TrimSpace(value)
		switch strings.ToLower(name) {
		case "range":
			request.rangeHeader = value
		case "if-modified-since":
			request.ifModifiedSince = value
		}
	}
}

// readHTTPLine is one line, its CRLF or LF taken off, refused with status past limit bytes.
func readHTTPLine(reader *bufio.Reader, limit, status int) (string, error) {
	var line []byte
	for {
		piece, err := reader.ReadSlice('\n')
		line = append(line, piece...)
		if len(line) > limit {
			return "", httpError{status, "too long"}
		}
		if err == nil {
			break
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return "", err
		}
	}
	text := strings.TrimSuffix(string(line), "\n")
	return strings.TrimSuffix(text, "\r"), nil
}

// requestPath is the path of a request's target, an origin-form /path or an absolute-form
// http://host/path, without its query, percent-decoded.
func requestPath(target string) (string, error) {
	if scheme, rest, found := strings.Cut(target, "://"); found && !strings.Contains(scheme, "/") {
		slash := strings.IndexByte(rest, '/')
		if slash < 0 {
			return "/", nil
		}
		target = rest[slash:]
	}
	if !strings.HasPrefix(target, "/") {
		return "", httpError{400, "not a path"}
	}
	if query := strings.IndexAny(target, "?#"); query >= 0 {
		target = target[:query]
	}
	return percentDecode(target)
}

// percentDecode is text with each %XX the byte it names.
func percentDecode(text string) (string, error) {
	if !strings.Contains(text, "%") {
		return text, nil
	}
	var out strings.Builder
	for index := 0; index < len(text); index++ {
		if text[index] != '%' {
			out.WriteByte(text[index])
			continue
		}
		if index+2 >= len(text) {
			return "", httpError{400, "a bad escape"}
		}
		value, err := strconv.ParseUint(text[index+1:index+3], 16, 8)
		if err != nil {
			return "", httpError{400, "a bad escape"}
		}
		out.WriteByte(byte(value))
		index += 2
	}
	return out.String(), nil
}

// byteRange is one range a Range header asks for, as first and last byte, both within size.
type byteRange struct{ first, last int64 }

// parseRange reads `bytes=A-B`, `bytes=A-` or `bytes=-N` against a file of size bytes: the range,
// whether one was asked for at all, and whether it can be satisfied. Several ranges, or a header
// that is not one, are not asked for, and the whole file is served.
func parseRange(header string, size int64) (byteRange, bool, bool) {
	spec, found := strings.CutPrefix(header, "bytes=")
	if !found || strings.Contains(spec, ",") {
		return byteRange{}, false, false
	}
	start, end, found := strings.Cut(strings.TrimSpace(spec), "-")
	if !found {
		return byteRange{}, false, false
	}
	if start == "" {
		suffix, err := strconv.ParseInt(end, 10, 64)
		if err != nil || suffix < 0 {
			return byteRange{}, false, false
		}
		if suffix == 0 || size == 0 {
			return byteRange{}, true, false
		}
		return byteRange{max(size-suffix, 0), size - 1}, true, true
	}
	first, err := strconv.ParseInt(start, 10, 64)
	if err != nil || first < 0 {
		return byteRange{}, false, false
	}
	last := size - 1
	if end != "" {
		if last, err = strconv.ParseInt(end, 10, 64); err != nil || last < first {
			return byteRange{}, false, false
		}
		last = min(last, size-1)
	}
	if first >= size {
		return byteRange{}, true, false
	}
	return byteRange{first, last}, true, true
}
