package applets

import (
	"context"
	"net/url"
	"testing"
)

// Which proxy a URL goes through, as the environment, no_proxy and -Y say.
func TestWgetProxy_asTheEnvironmentSays(t *testing.T) {
	for _, test := range []struct {
		environ   []string
		url, want string
		off       bool
	}{
		{environ: []string{"http_proxy=proxy:3128"}, url: "http://example.com/", want: "http://proxy:3128"},
		{environ: []string{"http_proxy=proxy:3128"}, url: "https://example.com/"},
		{environ: []string{"HTTPS_PROXY=http://u:p@proxy"}, url: "https://example.com/", want: "http://u:p@proxy"},
		{environ: []string{"http_proxy="}, url: "http://example.com/"},
		{environ: []string{"http_proxy=proxy"}, url: "http://example.com/", off: true},
		{environ: []string{"http_proxy=proxy"}, url: "http://127.0.0.1:8080/"},
		{environ: []string{"http_proxy=proxy"}, url: "http://127.9.9.9/"},
		{environ: []string{"http_proxy=proxy"}, url: "http://LocalHost/"},
		{environ: []string{"http_proxy=proxy"}, url: "http://[::1]/"},
		{environ: []string{"http_proxy=proxy", "no_proxy=example.com"}, url: "http://example.com/"},
		{environ: []string{"http_proxy=proxy", "no_proxy=example.com"}, url: "http://a.Example.com/"},
		{environ: []string{"http_proxy=proxy", "no_proxy=example.com"}, url: "http://notexample.com/", want: "http://proxy"},
		{environ: []string{"http_proxy=proxy", "no_proxy=.example.com"}, url: "http://example.com/", want: "http://proxy"},
		{environ: []string{"http_proxy=proxy", "no_proxy=*.example.com"}, url: "http://a.example.com/"},
		{environ: []string{"http_proxy=proxy", "NO_PROXY=10.0.0.0/8"}, url: "http://10.1.2.3/"},
		{environ: []string{"http_proxy=proxy", "NO_PROXY=10.0.0.0/8"}, url: "http://11.1.2.3/", want: "http://proxy"},
		{environ: []string{"http_proxy=proxy", "no_proxy=example.com:8080"}, url: "http://example.com:8080/"},
		{environ: []string{"http_proxy=proxy", "no_proxy=example.com:8080"}, url: "http://example.com/", want: "http://proxy"},
		{environ: []string{"http_proxy=proxy", "no_proxy=[2001:db8::1]:80"}, url: "http://[2001:db8::1]/"},
		{environ: []string{"http_proxy=proxy", "no_proxy=192.0.2.1"}, url: "http://192.0.2.1/"},
		{environ: []string{"http_proxy=proxy", "no_proxy= other , *"}, url: "http://example.com/"},
	} {
		location, err := url.Parse(test.url)
		if err != nil {
			t.Fatal(err)
		}
		ctx := WithProcessView(context.Background(), hostProcessView{environ: test.environ})
		proxy, err := wgetRequest{noProxy: test.off}.proxyFor(ctx, location)
		got := ""
		if proxy != nil {
			got = proxy.String()
		}
		if err != nil || got != test.want {
			t.Errorf("%q for %s: %q, %v; want %q", test.environ, test.url, got, err, test.want)
		}
	}
}
