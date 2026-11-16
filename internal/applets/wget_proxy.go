package applets

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net/netip"
	"net/url"
	"strings"

	"github.com/xiongnemo/nemosh/internal/netio"
)

// wget's proxies, from the shell's own environment as busybox's come from its: http_proxy for an
// http URL and https_proxy for an https one, and none under `-Y off`. Each is read in either
// case, lower first, busybox's spelling, and no_proxy names hosts that go direct -- which
// busybox does not read, and net/http, which fetched for wget until it had a client of its own,
// did; so do loopback addresses, always. An https URL goes through a CONNECT tunnel, as
// net/http sent it, where busybox would send the request in the clear to the proxy. Only an
// http:// proxy is spoken to: an https:// or socks5:// one is refused by name.

// proxyFor answers the proxy a request for location goes through, or nil to go direct.
func (r wgetRequest) proxyFor(ctx context.Context, location *url.URL) (*url.URL, error) {
	if r.noProxy {
		return nil, nil
	}
	view := ProcessViewFromContext(ctx)
	value := lookupEitherCase(view, location.Scheme+"_proxy")
	if value == "" || bypassesProxy(lookupEitherCase(view, "no_proxy"), location) {
		return nil, nil
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	proxy, err := url.Parse(value)
	if err != nil || proxy.Hostname() == "" {
		return nil, fmt.Errorf("bad proxy %q in %s_proxy", value, location.Scheme)
	}
	if proxy.Scheme != "http" {
		return nil, fmt.Errorf("proxy %s: only an http:// proxy is supported", proxy.Redacted())
	}
	return proxy, nil
}

func lookupEitherCase(view ProcessView, name string) string {
	if value, ok := view.LookupEnv(name); ok {
		return value
	}
	value, _ := view.LookupEnv(strings.ToUpper(name))
	return value
}

// bypassesProxy is whether location goes direct: a loopback host does, and so does one no_proxy
// names. no_proxy is a comma-separated list; `*` is every host; an entry is a name, which is that
// host and those within it, `.name` only those within it, an address, or a CIDR prefix; and a
// name or address with a port is that port only.
func bypassesProxy(noProxy string, location *url.URL) bool {
	host := strings.ToLower(location.Hostname())
	port := portOf(location)
	address, err := netip.ParseAddr(host)
	isAddress := err == nil
	address = address.Unmap()
	if host == "localhost" || isAddress && address.IsLoopback() {
		return true
	}
	for entry := range strings.SplitSeq(noProxy, ",") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "*" {
			return true
		}
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			if isAddress && prefix.Contains(address) {
				return true
			}
			continue
		}
		name, entryPort, err := netio.SplitHostPort(entry)
		if err != nil {
			name, entryPort = entry, ""
		}
		name = strings.TrimPrefix(strings.Trim(name, "[]"), "*")
		if name == "" || entryPort != "" && entryPort != port {
			continue
		}
		if other, err := netip.ParseAddr(name); err == nil {
			if isAddress && other.Unmap() == address {
				return true
			}
			continue
		}
		if strings.HasPrefix(name, ".") && strings.HasSuffix(host, name) ||
			host == name || strings.HasSuffix(host, "."+name) {
			return true
		}
	}
	return false
}

// portOf is the port location names, or its scheme's.
func portOf(location *url.URL) string {
	if port := location.Port(); port != "" {
		return port
	}
	if location.Scheme == "https" {
		return "443"
	}
	return "80"
}

// proxyAuthorization is the field that sends a proxy's credentials, if its URL has them.
func proxyAuthorization(proxy *url.URL) []string {
	if proxy.User == nil {
		return nil
	}
	password, _ := proxy.User.Password()
	return []string{"Proxy-Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(proxy.User.Username()+":"+password))}
}

// absoluteForm is location as a request to a proxy names it: whole, but for its credentials and
// fragment, which are not the server's.
func absoluteForm(location *url.URL) string {
	target := *location
	target.User, target.Fragment, target.RawFragment = nil, "", ""
	if target.Path == "" && target.RawPath == "" && target.Opaque == "" {
		target.Path = "/"
	}
	return target.String()
}

// tunnel asks the proxy at the other end of conn for a connection to address, through which TLS
// is then spoken to the server.
func (r wgetRequest) tunnel(conn netio.Conn, address string, fields []string) error {
	var request strings.Builder
	fmt.Fprintf(&request, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n", address, address)
	for _, field := range fields {
		request.WriteString(field + "\r\n")
	}
	request.WriteString("\r\n")
	if _, err := conn.Write([]byte(request.String())); err != nil {
		return err
	}
	reader := bufio.NewReader(&timedReader{conn: conn, timeout: r.timeout})
	answer, err := readHTTPResponseHead(reader)
	if err != nil {
		return fmt.Errorf("proxy: %v", err)
	}
	if answer.status < 200 || answer.status > 299 {
		return fmt.Errorf("proxy answered CONNECT with %s", answer.line)
	}
	if reader.Buffered() > 0 {
		// The server speaks first in nothing TLS does, so bytes here are the proxy's, and
		// handing TLS a connection with them read off it would lose them.
		return fmt.Errorf("proxy sent more than its answer to CONNECT")
	}
	return nil
}
