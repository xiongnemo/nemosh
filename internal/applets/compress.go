package applets

import (
	"compress/bzip2"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// gzip, gunzip, zcat, bunzip2 and bzcat.
//
// These are *stream filters*, which is why Windows shipping `tar.exe` does not
// cover them: `... | gzip > x.gz` and `zcat log.gz | grep` are pipelines, and
// bsdtar cannot stand in for either. Stock Windows has no gzip at all -- measured,
// `System32` holds `tar.exe` and `curl.exe` and neither `gzip` nor `unzip`.
//
// One implementation, five names, the mode chosen by the name -- the same shape
// the checksums and dos2unix use.
//
// `bzip2` compression is deliberately absent and unregistered: the standard
// library decompresses bzip2 but cannot compress it. Leaving the name
// unregistered rather than refusing it means PATH lookup still finds a real
// `bzip2.exe` if the machine has one, which is more useful than a refusal.

// compressMode is what a name does by default.
type compressMode struct {
	// codec is "gzip" or "bzip2".
	codec string
	// decompress is the default direction for this name.
	decompress bool
	// alwaysStdout is zcat and bzcat: they never touch the file on disk.
	alwaysStdout bool
	// suffixes are the extensions this codec's files carry, longest first, and
	// the first is what compression appends.
	suffixes []string
}

func newGzipApplet() Applet {
	return newCompressApplet("gzip", compressMode{codec: "gzip", suffixes: []string{".gz", ".tgz", ".z"}})
}

func newGunzipApplet() Applet {
	return newCompressApplet("gunzip", compressMode{codec: "gzip", decompress: true, suffixes: []string{".gz", ".tgz", ".z"}})
}

func newZcatApplet() Applet {
	return newCompressApplet("zcat", compressMode{codec: "gzip", decompress: true, alwaysStdout: true, suffixes: []string{".gz", ".tgz", ".z"}})
}

func newBunzip2Applet() Applet {
	return newCompressApplet("bunzip2", compressMode{codec: "bzip2", decompress: true, suffixes: []string{".bz2", ".tbz2", ".tbz"}})
}

func newBzcatApplet() Applet {
	return newCompressApplet("bzcat", compressMode{codec: "bzip2", decompress: true, alwaysStdout: true, suffixes: []string{".bz2", ".tbz2", ".tbz"}})
}

func newCompressApplet(name string, mode compressMode) Applet {
	return simpleApplet{name: name, runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		options, paths, err := parseAppletOptions(args, "cdfkt123456789", "")
		if err != nil {
			return err
		}
		request := compressRequest{
			mode:       mode,
			decompress: mode.decompress || options.has('d'),
			toStdout:   mode.alwaysStdout || options.has('c'),
			keep:       options.has('k'),
			force:      options.has('f'),
			test:       options.has('t'),
			level:      compressionLevel(options),
		}
		if request.test {
			// -t reads and discards, so it never writes and never removes.
			request.decompress, request.toStdout, request.keep = true, true, true
		}
		if len(paths) == 0 {
			return request.filter(stdin, stdout)
		}
		return request.eachFile(ctx, paths, stdout, stderr)
	}}
}

func compressionLevel(options appletOptions) int {
	for digit := '9'; digit >= '1'; digit-- {
		if options.has(byte(digit)) {
			return int(digit - '0')
		}
	}
	return gzip.DefaultCompression
}

type compressRequest struct {
	mode       compressMode
	decompress bool
	toStdout   bool
	keep       bool
	force      bool
	test       bool
	level      int
}

// filter is the no-operand form: stdin to stdout.
//
// busybox's own zcat cannot read a *pipe*: `cat x.gz | busybox zcat` answers
// `lseek(18446744073709551614, 1): Invalid seek`, while `busybox zcat < x.gz`
// works. Measured 2026-08-22. It seeks on its input, which a redirect allows and
// a pipe does not. This reads sequentially and handles both, which is a
// divergence where the reference is simply broken.
func (r compressRequest) filter(stdin io.Reader, stdout io.Writer) error {
	if r.decompress {
		reader, err := r.reader(stdin)
		if err != nil {
			return err
		}
		out := io.Writer(stdout)
		if r.test {
			out = io.Discard
		}
		if _, err := io.Copy(out, reader); err != nil {
			return err
		}
		return closeIfCloser(reader)
	}
	writer, err := gzip.NewWriterLevel(stdout, r.level)
	if err != nil {
		return err
	}
	if _, err := io.Copy(writer, stdin); err != nil {
		return err
	}
	return writer.Close()
}

func (r compressRequest) reader(input io.Reader) (io.Reader, error) {
	if r.mode.codec == "bzip2" {
		return bzip2.NewReader(input), nil
	}
	reader, err := gzip.NewReader(input)
	if err != nil {
		return nil, fmt.Errorf("invalid compressed data")
	}
	return reader, nil
}

func closeIfCloser(reader io.Reader) error {
	if closer, ok := reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// eachFile handles the operand form, where the default is to *replace* the file
// on disk and remove the original -- which is the behaviour that surprises people
// and is what both references do.
func (r compressRequest) eachFile(ctx context.Context, paths []string, stdout, stderr io.Writer) error {
	view := ProcessViewFromContext(ctx)
	failed := false
	for _, path := range paths {
		if err := r.oneFile(view, path, stdout); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", r.name(), err)
			failed = true
		}
	}
	if failed {
		return ExitStatus(1)
	}
	return nil
}

func (r compressRequest) name() string {
	if r.mode.codec == "bzip2" {
		return "bunzip2"
	}
	return "gzip"
}

func (r compressRequest) oneFile(view ProcessView, path string, stdout io.Writer) error {
	native, err := resolveHostPath(view, path)
	if err != nil {
		return operandFailure(path, err)
	}
	if !r.toStdout {
		// Not opened here: on Windows a file cannot be deleted while a handle to
		// it is open, so rewriteFile opens and closes the source itself before
		// removing it. Holding it open across the remove failed with "The process
		// cannot access the file because it is being used by another process" --
		// measured, and invisible on Unix where the unlink would have succeeded.
		return r.rewriteFile(native, path)
	}
	source, err := os.Open(native)
	if err != nil {
		return operandFailure(path, err)
	}
	defer source.Close()

	{
		if r.decompress {
			reader, err := r.reader(source)
			if err != nil {
				return operandFailure(path, err)
			}
			out := io.Writer(stdout)
			if r.test {
				out = io.Discard
			}
			_, err = io.Copy(out, reader)
			return err
		}
		writer, err := gzip.NewWriterLevel(stdout, r.level)
		if err != nil {
			return err
		}
		if _, err := io.Copy(writer, source); err != nil {
			return err
		}
		return writer.Close()
	}
}

// rewriteFile is the default: write the companion file, then remove the original
// unless -k said to keep it.
//
// The original is removed only after the new file is complete *and both handles
// are closed*, so an interrupted run leaves the input intact rather than losing
// both -- and so Windows will actually let the remove happen.
func (r compressRequest) rewriteFile(native, path string) error {
	target, err := r.targetName(native)
	if err != nil {
		return operandFailure(path, err)
	}
	if !r.force {
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("%s already exists", filepathBase(target))
		}
	}
	source, err := os.Open(native)
	if err != nil {
		return operandFailure(path, err)
	}
	destination, err := os.Create(target)
	if err != nil {
		source.Close()
		return operandFailure(path, err)
	}
	writeErr := r.copyThrough(source, destination)
	closeErr := destination.Close()
	if err := source.Close(); err != nil && closeErr == nil {
		closeErr = err
	}
	if writeErr != nil || closeErr != nil {
		// The half-written companion is removed, so a failure does not leave a
		// truncated archive looking like a real one.
		os.Remove(target)
		if writeErr != nil {
			return operandFailure(path, writeErr)
		}
		return operandFailure(path, closeErr)
	}
	if r.keep {
		return nil
	}
	return os.Remove(native)
}

func (r compressRequest) copyThrough(source io.Reader, destination io.Writer) error {
	if r.decompress {
		reader, err := r.reader(source)
		if err != nil {
			return err
		}
		if _, err := io.Copy(destination, reader); err != nil {
			return err
		}
		return closeIfCloser(reader)
	}
	writer, err := gzip.NewWriterLevel(destination, r.level)
	if err != nil {
		return err
	}
	if _, err := io.Copy(writer, source); err != nil {
		return err
	}
	return writer.Close()
}

// targetName is the companion file's name: the suffix appended when compressing,
// or stripped when decompressing.
//
// A name with no recognised suffix cannot be decompressed to anything, and
// guessing would overwrite the input -- so it is refused by name.
func (r compressRequest) targetName(native string) (string, error) {
	if !r.decompress {
		return native + r.mode.suffixes[0], nil
	}
	lowered := strings.ToLower(native)
	for _, suffix := range r.mode.suffixes {
		if strings.HasSuffix(lowered, suffix) {
			stripped := native[:len(native)-len(suffix)]
			// .tgz and .tbz stand for .tar.gz and .tar.bz2, so the restored name
			// gets its .tar back rather than losing the extension entirely.
			if suffix == ".tgz" || suffix == ".tbz" || suffix == ".tbz2" {
				return stripped + ".tar", nil
			}
			return stripped, nil
		}
	}
	return "", fmt.Errorf("unknown suffix")
}

func filepathBase(path string) string {
	if index := strings.LastIndexAny(path, `/\`); index >= 0 {
		return path[index+1:]
	}
	return path
}
