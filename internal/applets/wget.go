package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"
)

// wget: fetch a URL.
//
// Over its own client, wget_client.go, which does TLS through netio -- so unlike busybox
// this needs no `ssl_client` helper alongside it. busybox shells out to one because its own
// HTTP client cannot speak TLS; `ssl_client` exists here as a TLS pipe in its own right
// rather than as wget's plumbing.
//
// One coupling worth naming: `completions/wget.toml` describes the *external*
// wget and carries `meta.tool-version` precisely because "one name can be two
// programs" (internal/completionspec/spec.go:56). This makes it three.

// wgetDefaultTimeout is busybox's -T: how long a connection, and each read after it, may take.
// It was 30 seconds for the whole fetch, which a large download outlived.
const wgetDefaultTimeout = 900 * time.Second

// wgetValued is the letters whose argument may be the next word. Named because
// wgetLongOptions has to skip such an argument without reading it: a User-Agent
// or an output name is allowed to look exactly like a long option.
const wgetValued = "OoPUTY"

func newWgetApplet() Applet {
	return simpleApplet{name: "wget", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		rest, headers, spider, err := wgetLongOptions(args)
		if err != nil {
			return err
		}
		options, operands, err := parseAppletOptions(ctx, rest, "cqS", wgetValued)
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
			// busybox's -Y: off is no proxy, and anything else is the environment's.
			noProxy: options.value('Y') == "off",
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
		case "header", "proxy":
			if !attached {
				if index+1 >= len(args) {
					return nil, nil, false, missingOptionArgument(name)
				}
				index++
				value = args[index]
			}
			if name == "proxy" {
				// busybox's long spelling of -Y.
				kept = append(kept, "-Y", value)
				continue
			}
			// Repeatable, because one request usually needs more than one header.
			headers = append(headers, value)
		default:
			return nil, nil, false, unknownLongOption(arg)
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
	noProxy    bool
}

func (r wgetRequest) fetch(ctx context.Context, address string, stdout, stderr io.Writer) error {
	if !strings.Contains(address, "://") {
		// A bare host is assumed to be http, which is what wget does and what
		// anyone typing `wget example.com/x` means.
		address = "http://" + address
	}
	// -S is each response's status line and every header field, as busybox prints them.
	var heads io.Writer
	if r.showServer {
		heads = stderr
	}
	fetched, err := r.get(ctx, address, heads)
	if err != nil {
		return err
	}
	defer fetched.Close()
	status := strings.TrimPrefix(fetched.line[strings.IndexByte(fetched.line, ' ')+1:], " ")
	if fetched.status >= 400 {
		// A 404 is a failed download, not a file whose contents are the error
		// page -- writing the body would leave a file that looks like a success.
		return fmt.Errorf("server returned %s", status)
	}
	if r.spider {
		if !r.quiet {
			if _, err := fmt.Fprintf(stderr, "%s exists (%s)\n", address, status); err != nil {
				return err
			}
		}
		return nil
	}
	return r.save(ctx, fetched, stdout, stderr)
}

// save decides where the body goes, as busybox's wget does (networking/wget.c).
//
// `-O -` is stdout, and any other -O is opened as the user wrote it -- a path anywhere, a
// drive, /dev/null -- as busybox's xopen opens it. Without -O the name is the last path element
// of the URL that was asked for, which busybox takes before it sends a request, so a redirect
// to an opaque name -- GitHub's release assets -- does not rename the file. It is still checked
// as an archive entry's name is: a URL can end in `/NUL` or `..` as easily as a server's
// answer can. -P leads to that name only, never to -O's, and names a directory that is there:
// busybox makes none, and fails to open the file.
func (r wgetRequest) save(ctx context.Context, fetched *wgetFetched, stdout, stderr io.Writer) error {
	if r.output == "-" {
		_, err := io.Copy(stdout, fetched.body)
		return err
	}
	name := r.output
	if name == "" {
		base := path.Base(fetched.requested.Path)
		if base == "" || base == "/" || base == "." {
			// Nothing usable in the URL, which is what index pages look like.
			base = "index.html"
		}
		safe, err := safeArchivePath(base)
		if err != nil {
			return err
		}
		name = safe
		if r.directory != "" {
			name = path.Join(r.directory, name)
		}
	}
	file, remove, err := openDownload(ProcessViewFromContext(ctx), name)
	if err != nil {
		return cannotOpen(name, err)
	}
	written, copyErr := io.Copy(file, fetched.body)
	closeErr := file.Close()
	if copyErr != nil {
		// The partial file is removed: a truncated download that looks complete
		// is worse than no download.
		remove()
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if !r.quiet {
		if _, err := fmt.Fprintf(stderr, "'%s' saved [%d]\n", name, written); err != nil {
			return err
		}
	}
	return nil
}

// openDownload opens name to write a download into, as busybox's xopen does: a file wherever
// the name says, or through the view a device such as /dev/null. remove takes a partial file
// away again; a device has none to take.
func openDownload(view ProcessView, name string) (io.WriteCloser, func(), error) {
	file, err := openProcessOutput(view, name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, createMode(view, 0o666))
	if err != nil {
		return nil, nil, err
	}
	remove := func() {}
	if resolved, err := ResolveProcessPath(view, name); err == nil && !resolved.Device {
		remove = func() { _ = os.Remove(resolved.Native) }
	}
	return file, remove, nil
}
