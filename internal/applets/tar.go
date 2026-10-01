package applets

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// tar: create, list and extract.
//
// Windows does ship `tar.exe` (bsdtar), so this is not a capability gap the way
// gzip is -- but it reuses this build's own compressors, so `tar -czf` works in a
// pipeline without a second program, and every entry goes through the shared
// containment check in archive_path.go.
//
// Extraction is the dangerous direction: an archive names its own destinations,
// so it is untrusted input. Nothing is created until the name has been checked.

func newTarApplet() Applet {
	return simpleApplet{name: "tar", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		request, options, err := newTarRequest(ctx, args, stdin)
		if err != nil {
			return err
		}
		// Exactly one operation, counted rather than fallen through. A switch on the
		// three letters in order silently *chose* one when several were given, so
		// `tar -c -x -f a.tar src` created the archive and ignored the -x -- and
		// somebody who typed both meant one of them and got the other half the time.
		// busybox refuses the same invocation by printing its usage, and this
		// applet's own sibling cpio already requires exactly one of -t -i -o.
		operations := 0
		for _, letter := range "ctx" {
			if options.has(byte(letter)) {
				operations++
			}
		}
		if operations != 1 {
			return fmt.Errorf("exactly one of -c, -t or -x is required")
		}
		if options.has('c') {
			return request.create(ctx, stdout, stderr)
		}
		// Listing and extracting take the names given; see tarSelection.
		request.selection.accept = request.operands
		if options.has('t') {
			err = request.list(ctx, stdin, stdout)
		} else {
			err = request.extract(ctx, stdin, stdout, stderr)
		}
		if err == nil && request.selection.unmatched(stderr) {
			return ExitStatus(1)
		}
		return err
	}}
}

// tarOldStyle is busybox's reading of a first argument with no dash, the form every tar takes:
// `tar cf a.tar dir`, `tar xzf a.tgz`. Its letters are options, and f's value is the next
// argument even when letters follow it, so f moves to the end before the dash goes in: `tar fx
// a.tar` is -xf a.tar, as busybox moves it (archival/tar.c). It was an operand, and `tar cf`
// said that one of -c, -t or -x was required.
func tarOldStyle(args []string) []string {
	if len(args) == 0 || args[0] == "" || args[0][0] == '-' {
		return args
	}
	letters := args[0]
	if at := strings.IndexByte(letters, 'f'); at >= 0 {
		letters = letters[:at] + letters[at+1:] + "f"
	}
	return append([]string{"-" + letters}, args[1:]...)
}

type tarRequest struct {
	// verbose counts -t and -v, as busybox's verboseFlag does; see tar_listing.go.
	verbose    int
	toStdout   bool
	gzip       bool
	bzip2      bool
	autoDetect bool
	file       string
	directory  string
	operands   []string
	// keepOld is -k, keepTime all but -m, dereference -h, and the rest their long options;
	// selection is what is listed, extracted or left out of an archive (tar_select.go).
	keepOld, keepTime, dereference, noRecursion, overwrite bool
	selection                                              *tarSelection
	// view is the shell tar runs in, whose umask what it extracts is made through.
	view ProcessView
}

// openArchiveInput resolves -f, defaulting to stdin so `tar -tzf -` and a pipe
// both work.
func (r tarRequest) openArchiveInput(ctx context.Context, stdin io.Reader) (io.Reader, func(), error) {
	if r.file == "" || r.file == "-" {
		return stdin, func() {}, nil
	}
	// A device too, as busybox's xopen takes one: `tar tf /dev/stdin`.
	file, err := openProcessInput(ProcessViewFromContext(ctx), r.file)
	if err != nil {
		return nil, nil, cannotOpen(r.file, err)
	}
	return file, func() { file.Close() }, nil
}

// decompressed wraps the archive stream in whatever -z, -j or -a asked for.
func (r tarRequest) decompressed(input io.Reader) (io.Reader, error) {
	compressed := r.gzip
	bunzip := r.bzip2
	if r.autoDetect {
		// -a decides from the name, which is the only information available
		// before the first byte is read.
		lowered := strings.ToLower(r.file)
		compressed = compressed || strings.HasSuffix(lowered, ".gz") || strings.HasSuffix(lowered, ".tgz")
		bunzip = bunzip || strings.HasSuffix(lowered, ".bz2") || strings.HasSuffix(lowered, ".tbz2")
	}
	switch {
	case bunzip:
		return bzip2.NewReader(input), nil
	case compressed:
		reader, err := gzip.NewReader(input)
		if err != nil {
			return nil, fmt.Errorf("invalid compressed data")
		}
		return reader, nil
	}
	return input, nil
}

func (r tarRequest) list(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	input, release, err := r.openArchiveInput(ctx, stdin)
	if err != nil {
		return err
	}
	defer release()
	stream, err := r.decompressed(input)
	if err != nil {
		return err
	}
	reader := tar.NewReader(stream)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("invalid tar archive: %v", err)
		}
		if !r.selection.takes(header.Name) {
			continue
		}
		// The name is printed as the archive holds it, unchecked -- listing is
		// how somebody inspects a suspicious archive, so a refusal here would
		// hide exactly what they are looking for. Extraction is where the check
		// belongs.
		line := header.Name
		if r.verbose > 1 {
			line = tarListing(header)
		}
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return err
		}
	}
}

func (r tarRequest) extract(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	input, release, err := r.openArchiveInput(ctx, stdin)
	if err != nil {
		return err
	}
	defer release()
	stream, err := r.decompressed(input)
	if err != nil {
		return err
	}
	root, err := r.extractionRoot(ctx)
	if err != nil {
		return err
	}
	r.view = ProcessViewFromContext(ctx)
	collisions := newArchiveCollisions()
	reader := tar.NewReader(stream)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("invalid tar archive: %v", err)
		}
		if !r.selection.takes(header.Name) {
			continue
		}
		if err := r.extractEntry(reader, header, root, collisions, stdout, stderr); err != nil {
			return err
		}
	}
}

func (r tarRequest) extractionRoot(ctx context.Context) (string, error) {
	target := r.directory
	if target == "" {
		return resolveHostPath(ProcessViewFromContext(ctx), ".")
	}
	native, err := resolveHostPath(ProcessViewFromContext(ctx), target)
	if err != nil {
		return "", operandFailure(target, err)
	}
	// -C has to *exist*. Without this check the directory was created as a side
	// effect of writing the first entry into it, so `tar -xf a.tar -C /tpm` made a
	// new directory instead of reporting the misspelling -- and the option is
	// spelled "change to this directory", which is an error for a directory that is
	// not there. busybox says "can't change directory to 'nope'" and refuses; GNU
	// does the same.
	info, err := os.Stat(native)
	if err != nil {
		return "", fmt.Errorf("cannot change directory to '%s': %s", target, causeText(err))
	}
	if !info.IsDir() {
		return "", fmt.Errorf("cannot change directory to '%s': Not a directory", target)
	}
	return native, nil
}

// extractEntry writes one entry, having checked where it may land. -v says the name the archive
// holds, even of an entry --strip-components leaves nothing of, as busybox lists it.
func (r tarRequest) extractEntry(reader *tar.Reader, header *tar.Header, root string,
	collisions *archiveCollisions, stdout, stderr io.Writer) error {
	listed := header.Name
	if r.verbose > 1 {
		listed = tarListing(header)
	}
	if !r.selection.strippedEntry(header) {
		r.sayExtracted(stdout, stderr, listed)
		return nil
	}
	safe, err := safeArchivePath(header.Name)
	if err != nil {
		// Refused and skipped rather than aborting: a hostile entry among honest
		// ones should not cost the honest ones, and the reason is reported so the
		// skip is visible.
		fmt.Fprintf(stderr, "tar: skipping %v\n", err)
		return nil
	}
	if header.Linkname != "" {
		if err := safeLinkTarget(safe, header.Linkname); err != nil {
			fmt.Fprintf(stderr, "tar: skipping %v\n", err)
			return nil
		}
	}
	if err := collisions.check(safe); err != nil {
		fmt.Fprintf(stderr, "tar: skipping %v\n", err)
		return nil
	}
	r.sayExtracted(stdout, stderr, listed)
	if r.toStdout {
		if header.Typeflag != tar.TypeReg {
			return nil
		}
		_, err := io.Copy(stdout, reader)
		return err
	}
	destination := filepath.Join(root, filepath.FromSlash(safe))
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(destination, createMode(r.view, 0o755))
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(destination), createMode(r.view, 0o755)); err != nil {
			return err
		}
		return r.extractFile(reader, header, safe, destination)
	}
	// A symlink, device, fifo or socket entry. Windows has no honest equivalent
	// for most of these and a symlink needs a privilege this may not have, so
	// they are skipped with a reason rather than approximated with a copy.
	fmt.Fprintf(stderr, "tar: skipping %s: unsupported entry type\n", safe)
	return nil
}
