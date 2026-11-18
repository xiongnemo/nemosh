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
// A proxy, if the environment names one, is wget_proxy.go's.

const wgetRedirects = 16

// wgetFetched is a response wget is reading: the URL asked for, where it came from in the end,
// and its connection, which closing the body closes.
type wgetFetched struct {
	httpResponse
	requested, location *url.URL
	conn                netio.Conn
	// unwatch stops the context's closing the connection, which it does while the body is read.
	unwatch func() bool
}

func (f *wgetFetched) Close() error {
	f.unwatch()
	return f.conn.Close()
}

// get asks for address, following redirects, and answers the last response with its body unread.
// heads, when not nil, is given every response's status line and fields as each arrives, a
// redirect's too: -S, as busybox prints it.
func (r wgetRequest) get(ctx context.Context, address string, heads io.Writer) (*wgetFetched, error) {
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
	requested := location
	for redirects := 0; ; redirects++ {
		fetched, err := r.exchange(ctx, method, location, &authorization)
		if err != nil {
			return nil, err
		}
		if heads != nil {
			fmt.Fprintf(heads, "  %s\n", fetched.line)
			for _, field := range fetched.fields {
				fmt.Fprintf(heads, "  %s\n", field)
			}
		}
		target := fetched.headers["location"]
		switch fetched.status {
		case 301, 302, 303, 307, 308:
		default:
			fetched.requested = requested
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

// exchange sends one request for location, through a proxy if there is one, and reads the head of
// its answer.
func (r wgetRequest) exchange(ctx context.Context, method string, location *url.URL, authorization *string) (*wgetFetched, error) {
	proxy, err := r.proxyFor(ctx, location)
	if err != nil {
		return nil, err
	}
	host, port := location.Hostname(), portOf(location)
	dialHost, dialPort := host, port
	if proxy != nil {
		dialHost, dialPort = proxy.Hostname(), portOf(proxy)
	}
	plain, err := netio.Dial(ctx, dialHost, dialPort, r.timeout)
	if err != nil {
		// busybox's words, `can't connect to remote host (HOST)`, with the port, which busybox
		// leaves out, and the reason netio gives.
		return nil, fmt.Errorf("cannot connect to remote host (%s): %v", netio.JoinHostPort(dialHost, dialPort), err)
	}
	// From here the context closes the connection, so Ctrl-C ends a handshake or a CONNECT as
	// surely as it ends the body; closing the plain connection ends TLS over it too.
	unwatch := context.AfterFunc(ctx, func() { plain.Close() })
	fail := func(err error) (*wgetFetched, error) {
		unwatch()
		plain.Close()
		return nil, fmt.Errorf("cannot fetch %s: %v", location, err)
	}
	if r.timeout > 0 {
		// The handshake and the CONNECT are reads too, which -T bounds as it bounds the rest.
		_ = plain.SetReadDeadline(time.Now().Add(r.timeout))
	}
	conn, target, fields := plain, location.RequestURI(), []string(nil)
	if proxy != nil && location.Scheme == "https" {
		if err := r.tunnel(plain, netio.JoinHostPort(host, port), proxyAuthorization(proxy)); err != nil {
			return fail(err)
		}
	} else if proxy != nil {
		target, fields = absoluteForm(location), proxyAuthorization(proxy)
	}
	if location.Scheme == "https" {
		secured, err := netio.Client(plain, host)
		if err != nil {
			return fail(err)
		}
		conn = secured
	}
	if location.User != nil {
		password, _ := location.User.Password()
		*authorization = "Basic " + base64.StdEncoding.EncodeToString([]byte(location.User.Username()+":"+password))
	}
	if *authorization != "" {
		fields = append(fields, "Authorization: "+*authorization)
	}
	if err := r.writeRequest(conn, method, target, location.Host, fields); err != nil {
		return fail(err)
	}
	reader := bufio.NewReaderSize(&timedReader{conn: conn, timeout: r.timeout}, 32<<10)
	response, err := readHTTPResponse(reader, method)
	if err != nil {
		return fail(err)
	}
	return &wgetFetched{httpResponse: response, location: location, conn: conn, unwatch: unwatch}, nil
}

// writeRequest sends method for target to host, with fields after wget's own.
func (r wgetRequest) writeRequest(conn io.Writer, method, target, host string, extra []string) error {
	agent := r.agent
	if agent == "" {
		agent = "nemosh-wget"
	}
	fields := append([]string{"Host: " + host, "User-Agent: " + agent, "Accept: */*", "Connection: close"}, extra...)
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
	fmt.Fprintf(&request, "%s %s HTTP/1.1\r\n", method, target)
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
