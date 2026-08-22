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
		options, operands, err := parseAppletOptions(args, "ctxvzjaO", "fC")
		if err != nil {
			return err
		}
		request := tarRequest{
			verbose:    options.has('v'),
			toStdout:   options.has('O'),
			gzip:       options.has('z'),
			bzip2:      options.has('j'),
			file:       options.value('f'),
			directory:  options.value('C'),
			operands:   operands,
			autoDetect: options.has('a'),
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
		switch {
		case options.has('c'):
			return request.create(ctx, stdout, stderr)
		case options.has('t'):
			return request.list(ctx, stdin, stdout)
		}
		return request.extract(ctx, stdin, stdout, stderr)
	}}
}

type tarRequest struct {
	verbose    bool
	toStdout   bool
	gzip       bool
	bzip2      bool
	autoDetect bool
	file       string
	directory  string
	operands   []string
}

// openArchiveInput resolves -f, defaulting to stdin so `tar -tzf -` and a pipe
// both work.
func (r tarRequest) openArchiveInput(ctx context.Context, stdin io.Reader) (io.Reader, func(), error) {
	if r.file == "" || r.file == "-" {
		return stdin, func() {}, nil
	}
	native, err := resolveHostPath(ProcessViewFromContext(ctx), r.file)
	if err != nil {
		return nil, nil, operandFailure(r.file, err)
	}
	file, err := os.Open(native)
	if err != nil {
		return nil, nil, operandFailure(r.file, err)
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
		// The name is printed as the archive holds it, unchecked -- listing is
		// how somebody inspects a suspicious archive, so a refusal here would
		// hide exactly what they are looking for. Extraction is where the check
		// belongs.
		line := header.Name
		if r.verbose {
			line = fmt.Sprintf("%s %8d %s", header.FileInfo().Mode(), header.Size, header.Name)
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
		return "", fmt.Errorf("cannot change directory to '%s': %v", target, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("cannot change directory to '%s': not a directory", target)
	}
	return native, nil
}

// extractEntry writes one entry, having checked where it may land.
func (r tarRequest) extractEntry(reader *tar.Reader, header *tar.Header, root string,
	collisions *archiveCollisions, stdout, stderr io.Writer) error {
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
	if r.verbose {
		fmt.Fprintln(stderr, safe)
	}
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
		return os.MkdirAll(destination, 0o755)
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		file, err := os.Create(destination)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(file, reader)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	// A symlink, device, fifo or socket entry. Windows has no honest equivalent
	// for most of these and a symlink needs a privilege this may not have, so
	// they are skipped with a reason rather than approximated with a copy.
	fmt.Fprintf(stderr, "tar: skipping %s: unsupported entry type\n", safe)
	return nil
}
