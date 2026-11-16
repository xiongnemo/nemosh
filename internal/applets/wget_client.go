package applets

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/xiongnemo/nemosh/internal/netio"
)

// wget's client, its own over netio as busybox's is its own: HTTP/1.1, one request a connection
// with Connection: close, no Accept-Encoding so nothing comes back compressed, TLS through netio
// for https. A redirect is followed sixteen times at most, busybox's limit, a 303 as a GET, and
// an Authorization dropped when the host changes; credentials in the URL are sent as Basic, as
// busybox sends them. -T is busybox's: how long a connection, and each read after it, may take.

const wgetRedirects = 16

// wgetFetched is a response wget is reading: where it came from in the end, and its connection,
// which closing the body closes.
type wgetFetched struct {
	httpResponse
	location *url.URL
	conn     netio.Conn
	// unwatch stops the context's closing the connection, which it does while the body is read.
	unwatch func() bool
}

func (f *wgetFetched) Close() error {
	f.unwatch()
	return f.conn.Close()
}

// get asks for address, following redirects, and answers the last response with its body unread.
func (r wgetRequest) get(ctx context.Context, address string) (*wgetFetched, error) {
	for _, header := range r.headers {
		if !strings.Contains(header, ":") {
			return nil, fmt.Errorf("bad header %q: a header is NAME: VALUE", header)
		}
	}
	location, err := url.Parse(address)
	if err != nil || location.Host == "" || location.Scheme != "http" && location.Scheme != "https" {
		return nil, fmt.Errorf("bad URL %s", address)
	}
	method := "GET"
	if r.spider {
		// --spider asks whether the URL is there, not for its contents, so it is a HEAD.
		method = "HEAD"
	}
	authorization := ""
	for redirects := 0; ; redirects++ {
		fetched, err := r.exchange(ctx, method, location, &authorization)
		if err != nil {
			return nil, err
		}
		target := fetched.headers["location"]
		switch fetched.status {
		case 301, 302, 303, 307, 308:
		default:
			return fetched, nil
		}
		fetched.Close()
		if target == "" {
			return nil, fmt.Errorf("server returned %s and no Location", fetched.line)
		}
		if redirects+1 >= wgetRedirects {
			return nil, fmt.Errorf("too many redirections")
		}
		next, err := location.Parse(target)
		if err != nil {
			return nil, fmt.Errorf("bad Location %q", target)
		}
		if next.Host != location.Host {
			authorization = ""
		}
		if fetched.status == 303 {
			method = "GET"
		}
		location = next
	}
}

// exchange sends one request for location and reads the head of its answer.
func (r wgetRequest) exchange(ctx context.Context, method string, location *url.URL, authorization *string) (*wgetFetched, error) {
	host, port := location.Hostname(), location.Port()
	if port == "" && location.Scheme == "https" {
		port = "443"
	} else if port == "" {
		port = "80"
	}
	conn, err := netio.Dial(ctx, host, port, r.timeout)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch %s: %v", location, err)
	}
	if location.Scheme == "https" {
		secured, err := netio.Client(conn, host)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("cannot fetch %s: %v", location, err)
		}
		conn = secured
	}
	unwatch := context.AfterFunc(ctx, func() { conn.Close() })
	if location.User != nil {
		password, _ := location.User.Password()
		*authorization = "Basic " + base64.StdEncoding.EncodeToString([]byte(location.User.Username()+":"+password))
	}
	if err := r.writeRequest(conn, method, location, *authorization); err != nil {
		unwatch()
		conn.Close()
		return nil, fmt.Errorf("cannot fetch %s: %v", location, err)
	}
	reader := bufio.NewReaderSize(&timedReader{conn: conn, timeout: r.timeout}, 32<<10)
	response, err := readHTTPResponse(reader, method)
	if err != nil {
		unwatch()
		conn.Close()
		return nil, fmt.Errorf("cannot fetch %s: %v", location, err)
	}
	return &wgetFetched{httpResponse: response, location: location, conn: conn, unwatch: unwatch}, nil
}

func (r wgetRequest) writeRequest(conn io.Writer, method string, location *url.URL, authorization string) error {
	agent := r.agent
	if agent == "" {
		agent = "nemosh-wget"
	}
	fields := []string{"Host: " + location.Host, "User-Agent: " + agent, "Accept: */*", "Connection: close"}
	if authorization != "" {
		fields = append(fields, "Authorization: "+authorization)
	}
	// After the User-Agent, so `--header 'User-Agent: x'` wins over -U rather than being
	// silently dropped: the more specific spelling is the later one.
	for _, header := range r.headers {
		name, value, found := strings.Cut(header, ":")
		if !found {
			return fmt.Errorf("bad header %q: a header is NAME: VALUE", header)
		}
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		for index := 0; index < len(fields); index++ {
			if field, _, _ := strings.Cut(fields[index], ":"); strings.EqualFold(field, name) {
				fields = append(fields[:index], fields[index+1:]...)
				index--
			}
		}
		fields = append(fields, name+": "+value)
	}
	var request strings.Builder
	fmt.Fprintf(&request, "%s %s HTTP/1.1\r\n", method, location.RequestURI())
	for _, field := range fields {
		request.WriteString(field + "\r\n")
	}
	request.WriteString("\r\n")
	_, err := io.WriteString(conn, request.String())
	return err
}

// timedReader gives each read of a response r.timeout, as busybox's -T does.
type timedReader struct {
	conn    netio.Conn
	timeout time.Duration
}

func (r *timedReader) Read(buffer []byte) (int, error) {
	if r.timeout > 0 {
		_ = r.conn.SetReadDeadline(time.Now().Add(r.timeout))
	}
	return r.conn.Read(buffer)
}
