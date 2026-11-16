package applets

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// An HTTP/1.x response as wget reads one: the status line and header, 64 KiB at most, and then
// the body as the response says it is sent -- chunked, a Content-Length, or until the connection
// closes. A HEAD, a 1xx, a 204 and a 304 have none. A body that ends before it says it will is an
// error, so a truncated download is not taken for a whole one.

const wgetHeaderLimit = 64 << 10

type httpResponse struct {
	status  int
	line    string   // the status line, `HTTP/1.1 200 OK`
	fields  []string // each header field as sent, for -S
	headers map[string]string
	body    io.Reader
}

var errTruncatedBody = errors.New("the connection closed before the body ended")

// readHTTPResponse reads a response to method from reader, passing over any 1xx before it.
func readHTTPResponse(reader *bufio.Reader, method string) (httpResponse, error) {
	for {
		response, err := readHTTPResponseHead(reader)
		if err != nil {
			return response, err
		}
		if response.status >= 100 && response.status < 200 {
			continue
		}
		switch {
		case method == "HEAD" || response.status == 204 || response.status == 304:
			response.body = strings.NewReader("")
		case strings.EqualFold(response.headers["transfer-encoding"], "chunked"):
			response.body = &chunkedReader{reader: reader}
		case response.headers["content-length"] != "":
			length, err := strconv.ParseInt(response.headers["content-length"], 10, 64)
			if err != nil || length < 0 {
				return response, fmt.Errorf("bad Content-Length %q", response.headers["content-length"])
			}
			response.body = &lengthReader{reader: reader, left: length}
		default:
			response.body = reader
		}
		return response, nil
	}
}

func readHTTPResponseHead(reader *bufio.Reader) (httpResponse, error) {
	line, err := readHTTPLine(reader, wgetHeaderLimit, 0)
	if err != nil {
		return httpResponse{}, fmt.Errorf("no response: %w", err)
	}
	version, rest, _ := strings.Cut(line, " ")
	code, _, _ := strings.Cut(rest, " ")
	status, err := strconv.Atoi(code)
	if !strings.HasPrefix(version, "HTTP/1.") || err != nil || len(code) != 3 {
		return httpResponse{}, fmt.Errorf("not an HTTP response: %q", line)
	}
	response := httpResponse{status: status, line: line, headers: map[string]string{}}
	total := len(line)
	for {
		field, err := readHTTPLine(reader, wgetHeaderLimit-total, 0)
		if err != nil {
			return response, fmt.Errorf("a broken header: %w", err)
		}
		total += len(field) + 2
		if field == "" {
			return response, nil
		}
		name, value, found := strings.Cut(field, ":")
		if !found {
			return response, fmt.Errorf("a broken header field: %q", field)
		}
		response.fields = append(response.fields, field)
		response.headers[strings.ToLower(strings.TrimSpace(name))] = strings.TrimSpace(value)
	}
}

// lengthReader is a body of a known length, short of which an EOF is an error.
type lengthReader struct {
	reader *bufio.Reader
	left   int64
}

func (r *lengthReader) Read(buffer []byte) (int, error) {
	if r.left == 0 {
		return 0, io.EOF
	}
	if int64(len(buffer)) > r.left {
		buffer = buffer[:r.left]
	}
	read, err := r.reader.Read(buffer)
	r.left -= int64(read)
	if errors.Is(err, io.EOF) && r.left > 0 {
		return read, errTruncatedBody
	}
	if r.left == 0 && err == nil {
		err = io.EOF
	}
	return read, err
}

// chunkedReader is a chunked body: each chunk's size in hex, its bytes and a CRLF, until a chunk
// of none, and any trailer fields after it.
type chunkedReader struct {
	reader *bufio.Reader
	left   int64
	done   bool
}

func (r *chunkedReader) Read(buffer []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	if r.left == 0 {
		line, err := readHTTPLine(r.reader, 4096, 0)
		if err != nil {
			return 0, errTruncatedBody
		}
		size, _, _ := strings.Cut(line, ";")
		r.left, err = strconv.ParseInt(strings.TrimSpace(size), 16, 64)
		if err != nil || r.left < 0 {
			return 0, fmt.Errorf("a bad chunk size %q", line)
		}
		if r.left == 0 {
			for {
				trailer, err := readHTTPLine(r.reader, 4096, 0)
				if err != nil {
					return 0, errTruncatedBody
				}
				if trailer == "" {
					r.done = true
					return 0, io.EOF
				}
			}
		}
	}
	if int64(len(buffer)) > r.left {
		buffer = buffer[:r.left]
	}
	read, err := r.reader.Read(buffer)
	r.left -= int64(read)
	if errors.Is(err, io.EOF) {
		return read, errTruncatedBody
	}
	if r.left == 0 {
		if end, err := readHTTPLine(r.reader, 2, 0); err != nil || end != "" {
			return read, fmt.Errorf("a chunk without its CRLF")
		}
	}
	return read, err
}
