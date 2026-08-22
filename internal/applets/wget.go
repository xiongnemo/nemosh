package applets

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

// wget: fetch a URL.
//
// Over net/http, which does TLS itself -- so unlike busybox this needs no
// `ssl_client` helper alongside it. busybox shells out to one because its own
// HTTP client cannot speak TLS; Go's can, and `ssl_client` exists here as a
// TLS pipe in its own right rather than as wget's plumbing.
//
// One coupling worth naming: `completions/wget.toml` describes the *external*
// wget and carries `meta.tool-version` precisely because "one name can be two
// programs" (internal/completionspec/spec.go:56). This makes it three.

const wgetDefaultTimeout = 30 * time.Second

// wgetValued is the letters whose argument may be the next word. Named because
// wgetLongOptions has to skip such an argument without reading it: a User-Agent
// or an output name is allowed to look exactly like a long option.
const wgetValued = "OoPUT"

func newWgetApplet() Applet {
	return simpleApplet{name: "wget", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		rest, headers, spider, err := wgetLongOptions(args)
		if err != nil {
			return err
		}
		options, operands, err := parseAppletOptions(rest, "cqS", wgetValued)
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return fmt.Errorf("a URL is required")
		}
		request := wgetRequest{
			quiet:      options.has('q'),
			showServer: options.has('S'),
			output:     options.value('O'),
			directory:  options.value('P'),
			agent:      options.value('U'),
			headers:    headers,
			spider:     spider,
			timeout:    wgetDefaultTimeout,
		}
		if options.has('T') {
			seconds, err := parseGrepNumber(options.value('T'))
			if err != nil {
				return err
			}
			request.timeout = time.Duration(seconds) * time.Second
		}
		for _, address := range operands {
			if err := request.fetch(ctx, address, stdout, stderr); err != nil {
				return err
			}
		}
		return nil
	}}
}

// wgetLongOptions pulls the long options out before the letters are read.
//
// parseAppletOptions is letter-by-letter, so `--header` reaching it reads as the
// cluster `-`, `-h`, `-e`, ... and is refused as the bare `-` it began with --
// naming nothing the user typed. Every applet with long options does this
// pre-pass; grep_parse.go:40 is the same shape. Scanning stops where
// parseAppletOptions' own loop stops, at `--` or the first non-option word, so
// the two agree on where the options end.
func wgetLongOptions(args []string) ([]string, []string, bool, error) {
	kept := make([]string, 0, len(args))
	var headers []string
	spider := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" || len(arg) < 2 || arg[0] != '-' {
			return append(kept, args[index:]...), headers, spider, nil
		}
		if !strings.HasPrefix(arg, "--") {
			kept = append(kept, arg)
			if len(arg) == 2 && strings.ContainsAny(arg[1:], wgetValued) && index+1 < len(args) {
				// The next word is this option's argument, whatever it looks
				// like. `wget -U --spider URL` sends that User-Agent.
				index++
				kept = append(kept, args[index])
			}
			continue
		}
		name, value, attached := strings.Cut(arg[2:], "=")
		switch name {
		case "spider":
			spider = true
		case "header":
			if !attached {
				if index+1 >= len(args) {
					return nil, nil, false, fmt.Errorf("option requires an argument -- 'header'")
				}
				index++
				value = args[index]
			}
			// Repeatable, because one request usually needs more than one header.
			headers = append(headers, value)
		default:
			return nil, nil, false, fmt.Errorf("unrecognized option -- '%s'", arg)
		}
	}
	return kept, headers, spider, nil
}

type wgetRequest struct {
	quiet      bool
	showServer bool
	output     string
	directory  string
	agent      string
	headers    []string
	spider     bool
	timeout    time.Duration
}

func (r wgetRequest) fetch(ctx context.Context, address string, stdout, stderr io.Writer) error {
	if !strings.Contains(address, "://") {
		// A bare host is assumed to be http, which is what wget does and what
		// anyone typing `wget example.com/x` means.
		address = "http://" + address
	}
	fetchCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	// --spider asks whether the URL is there, not for its contents, so it is a
	// HEAD. Sending GET and discarding the body would download the thing the
	// caller said not to download.
	method := http.MethodGet
	if r.spider {
		method = http.MethodHead
	}
	request, err := http.NewRequestWithContext(fetchCtx, method, address, nil)
	if err != nil {
		return fmt.Errorf("bad URL %s: %v", address, err)
	}
	agent := r.agent
	if agent == "" {
		agent = "nemosh-wget"
	}
	request.Header.Set("User-Agent", agent)
	// After the User-Agent, so `--header 'User-Agent: x'` wins over -U rather
	// than being silently dropped: the more specific spelling is the later one.
	for _, header := range r.headers {
		name, value, found := strings.Cut(header, ":")
		if !found {
			return fmt.Errorf("bad header %q: a header is NAME: VALUE", header)
		}
		request.Header.Set(strings.TrimSpace(name), strings.TrimSpace(value))
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("cannot fetch %s: %v", address, err)
	}
	defer response.Body.Close()
	if r.showServer {
		if _, err := fmt.Fprintf(stderr, "  %s\n", response.Status); err != nil {
			return err
		}
	}
	if response.StatusCode >= 400 {
		// A 404 is a failed download, not a file whose contents are the error
		// page -- writing the body would leave a file that looks like a success.
		return fmt.Errorf("server returned %s", response.Status)
	}
	if r.spider {
		if !r.quiet {
			if _, err := fmt.Fprintf(stderr, "%s exists (%s)\n", address, response.Status); err != nil {
				return err
			}
		}
		return nil
	}
	return r.save(ctx, response, stdout, stderr)
}

// save decides where the body goes.
//
// `-O -` is stdout. Otherwise the name comes from the URL's last path element,
// and **that name came from the server**, so it goes through the same containment
// check an archive entry does: a redirect to `http://host/../../evil` must not
// write outside the working directory, and a URL ending in `/NUL` must not reach
// the device.
func (r wgetRequest) save(ctx context.Context, response *http.Response, stdout, stderr io.Writer) error {
	if r.output == "-" {
		_, err := io.Copy(stdout, response.Body)
		return err
	}
	name := r.output
	if name == "" {
		name = path.Base(response.Request.URL.Path)
		if name == "" || name == "/" || name == "." {
			// Nothing usable in the URL, which is what index pages look like.
			name = "index.html"
		}
	}
	if r.directory != "" {
		name = path.Join(r.directory, name)
	}
	safe, err := safeArchivePath(name)
	if err != nil {
		return err
	}
	native, err := resolveHostPath(ProcessViewFromContext(ctx), safe)
	if err != nil {
		return operandFailure(safe, err)
	}
	if err := os.MkdirAll(pathDirOf(native), 0o755); err != nil {
		return err
	}
	file, err := os.Create(native)
	if err != nil {
		return operandFailure(safe, err)
	}
	written, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		// The partial file is removed: a truncated download that looks complete
		// is worse than no download.
		os.Remove(native)
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if !r.quiet {
		if _, err := fmt.Fprintf(stderr, "'%s' saved [%d]\n", safe, written); err != nil {
			return err
		}
	}
	return nil
}

func pathDirOf(native string) string {
	if index := strings.LastIndexAny(native, `/\`); index > 0 {
		return native[:index]
	}
	return "."
}
