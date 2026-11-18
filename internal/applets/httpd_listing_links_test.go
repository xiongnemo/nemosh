package applets

import (
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every link in a listing leads to what it names: a name with `#`, `%`, a space, `&`, `'` or
// Chinese in it is percent-encoded in the href and HTML-escaped around it. A `#` began a fragment
// and the file was not found; a `%` was a bad escape, and the answer 400.
func TestHttpd_aListingsLinksLeadToTheirFiles(t *testing.T) {
	root := t.TempDir()
	names := []string{"a#b.txt", "100%.txt", "中文.txt", "a b.txt", "x&y.txt", "it's.txt", "plain.txt"}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(root, name), []byte("content of "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "sub #1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub #1", "in%side.txt"), []byte("content of in%side.txt"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := startHttpd(t, root)
	base, _ := url.Parse(server.URL + "/")
	follow := func(from *url.URL) map[string]string {
		t.Helper()
		response, err := http.Get(from.String())
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		links := map[string]string{}
		for _, match := range regexp.MustCompile(`<a href="([^"]*)">([^<]*)</a>`).FindAllStringSubmatch(string(body), -1) {
			links[html.UnescapeString(match[2])] = html.UnescapeString(match[1])
		}
		return links
	}
	get := func(target *url.URL) (int, string) {
		t.Helper()
		response, err := http.Get(target.String())
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		return response.StatusCode, string(body)
	}
	links := follow(base)
	for _, name := range names {
		href, found := links[name]
		if !found {
			t.Errorf("the listing has no link shown as %q: %v", name, links)
			continue
		}
		reference, err := url.Parse(href)
		if err != nil {
			t.Errorf("the link for %q is %q, which is no URL: %v", name, href, err)
			continue
		}
		if status, body := get(base.ResolveReference(reference)); status != 200 || body != "content of "+name {
			t.Errorf("the link for %q, %q, answered %d %q", name, href, status, body)
		}
	}
	directory, found := links["sub #1/"]
	if !found {
		t.Fatalf("the listing has no link for the directory: %v", links)
	}
	reference, _ := url.Parse(directory)
	inner := base.ResolveReference(reference)
	if !strings.HasSuffix(inner.Path, "/") {
		t.Errorf("the directory's link %q does not end in /", directory)
	}
	innerLinks := follow(inner)
	nested, _ := url.Parse(innerLinks["in%side.txt"])
	if status, body := get(inner.ResolveReference(nested)); status != 200 || body != "content of in%side.txt" {
		t.Errorf("the link inside the directory, %q, answered %d %q", innerLinks["in%side.txt"], status, body)
	}
}
