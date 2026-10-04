package applets

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// gzip, gunzip, zcat, bzip2, bunzip2 and bzcat.
//
// These are *stream filters*, which is why Windows shipping `tar.exe` does not
// cover them: `... | gzip > x.gz` and `zcat log.gz | grep` are pipelines, and
// bsdtar cannot stand in for either. Stock Windows has no gzip at all -- measured,
// `System32` holds `tar.exe` and `curl.exe` and neither `gzip` nor `unzip`.
//
// One implementation, six names, the mode chosen by the name -- the same shape
// the checksums and dos2unix use. bzip2 compresses through dsnet/compress, since the
// standard library only decompresses it; see compress_write.go. The name was left
// unregistered for want of a writer.

// compressMode is what a name does by default.
type compressMode struct {
	// codec is "gzip" or "bzip2", or "" for zcat's, which the data's first bytes choose.
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
	return newCompressApplet("zcat", compressMode{decompress: true, alwaysStdout: true, suffixes: []string{".gz", ".tgz", ".z"}})
}

func newBzip2Applet() Applet {
	return newCompressApplet("bzip2", compressMode{codec: "bzip2", suffixes: []string{".bz2", ".tbz2", ".tbz"}})
}

func newBunzip2Applet() Applet {
	return newCompressApplet("bunzip2", compressMode{codec: "bzip2", decompress: true, suffixes: []string{".bz2", ".tbz2", ".tbz"}})
}

func newBzcatApplet() Applet {
	return newCompressApplet("bzcat", compressMode{codec: "bzip2", decompress: true, alwaysStdout: true, suffixes: []string{".bz2", ".tbz2", ".tbz"}})
}

func newCompressApplet(name string, mode compressMode) Applet {
	return simpleApplet{name: name, runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		options, paths, err := parseAppletOptions(ctx, args, "cdfkt123456789", "")
		if err != nil {
			return err
		}
		request := compressRequest{
			applet:     name,
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
		return request.eachFile(ctx, paths, stdin, stdout, stderr)
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
	// applet is the name it was run as, which its messages start with: zcat's are zcat's.
	applet     string
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
	if r.test {
		stdout = io.Discard
	}
	_, err := r.copyThrough(stdin, stdout)
	return err
}

// eachFile handles the operand form, where the default is to *replace* the file
// on disk and remove the original -- which is the behaviour that surprises people
// and is what both references do. A FILE named - is standard input, to standard
// output, as busybox's bbunpack takes one; it was looked for as a file.
func (r compressRequest) eachFile(ctx context.Context, paths []string, stdin io.Reader, stdout, stderr io.Writer) error {
	view := ProcessViewFromContext(ctx)
	failed := false
	for _, path := range paths {
		var err error
		if path == "-" {
			err = r.filter(stdin, stdout)
		} else {
			err = r.oneFile(view, path, stdout)
		}
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", r.applet, err)
			failed = true
		}
	}
	if failed {
		return ExitStatus(1)
	}
	return nil
}

// oneFile takes FILE in bbunpack's order, so each failure is the one busybox reports: it
// is stat'ed, `FILE: No such file or directory`, then opened, `cannot open 'FILE'`, a
// directory among what cannot be, and only then named and written. The name was looked
// at first, so `gunzip nope` was an unknown suffix and a second `gzip f` said f.gz
// already existed, where there was no f to compress.
func (r compressRequest) oneFile(view ProcessView, path string, stdout io.Writer) error {
	native, err := resolveHostPath(view, path)
	if err != nil {
		return operandFailure(path, err)
	}
	info, err := os.Stat(native)
	if err != nil {
		return operandFailure(path, err)
	}
	source, err := OpenHostInput(native)
	if err != nil {
		return cannotOpen(path, err)
	}
	if !r.toStdout {
		return r.rewriteFile(view, native, path, info, source)
	}
	defer source.Close()
	return fileFault(path, r.filter(source, stdout))
}

// rewriteFile is the default: write the companion file, then remove the original
// unless -k said to keep it.
//
// The companion is made where nothing is, as busybox opens it O_EXCL: what is there is
// `cannot open 'FILE.gz': File exists`, and -f removes it first. It has the original's
// permissions less the umask's, as busybox gives it the original's mode, so a private
// file's archive is private too; it was made 0666 less the umask's. gunzip gives it the
// time the data holds, when it holds one.
//
// The original is removed only after the new file is complete *and both handles
// are closed*, so an interrupted run leaves the input intact rather than losing
// both -- and so Windows will actually let the remove happen. Holding the source open
// across the remove failed with "The process cannot access the file because it is
// being used by another process" -- measured, and invisible on Unix.
func (r compressRequest) rewriteFile(view ProcessView, native, path string, info os.FileInfo, source io.ReadCloser) error {
	target, err := r.targetName(native)
	if err != nil {
		source.Close()
		return operandFailure(path, err)
	}
	shown, _ := r.targetName(path)
	// unlink(2)'s, which leaves a directory where it is.
	if found, err := os.Lstat(target); err == nil && r.force && !found.IsDir() {
		os.Remove(target)
	}
	mode := maskedMode(info.Mode().Perm(), processFileModeMask(view))
	destination, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		source.Close()
		return cannotOpen(shown, err)
	}
	stamp, writeErr := r.copyThrough(source, destination)
	closeErr := destination.Close()
	if err := source.Close(); err != nil && closeErr == nil {
		closeErr = err
	}
	if writeErr != nil || closeErr != nil {
		// The half-written companion is removed, so a failure does not leave a
		// truncated archive looking like a real one.
		os.Remove(target)
		if writeErr != nil {
			return fileFault(path, writeErr)
		}
		return operandFailure(path, closeErr)
	}
	if !stamp.IsZero() {
		os.Chtimes(target, stamp, stamp)
	}
	if r.keep {
		return nil
	}
	return os.Remove(native)
}

// copyThrough writes source to destination through the codec. Decompressing gzip, it
// says when the data was modified, by the last member's MTIME, as busybox's gunzip
// sets the file it writes from it.
func (r compressRequest) copyThrough(source io.Reader, destination io.Writer) (time.Time, error) {
	if r.decompress {
		reader, err := decompressor(r.mode.codec, source)
		if err != nil {
			return time.Time{}, err
		}
		_, err = io.Copy(destination, reader)
		if gunzip, ok := reader.(*gunzipReader); ok {
			return gunzip.member.ModTime, err
		}
		return time.Time{}, err
	}
	writer, err := compressor(destination, r.mode.codec, r.level)
	if err != nil {
		return time.Time{}, err
	}
	if _, err := io.Copy(writer, source); err != nil {
		return time.Time{}, err
	}
	return time.Time{}, writer.Close()
}

// fileFault is err as it is said of FILE. A fault in the data stands alone, as busybox
// says one.
func fileFault(path string, err error) error {
	if _, ok := errors.AsType[compressFault](err); ok || err == nil {
		return err
	}
	return operandFailure(path, err)
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
	return "", errors.New("unknown suffix - ignored")
}

func filepathBase(path string) string {
	if index := strings.LastIndexAny(path, `/\`); index >= 0 {
		return path[index+1:]
	}
	return path
}
