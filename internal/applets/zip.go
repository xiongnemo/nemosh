package applets

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// zip, Info-ZIP's, as Debian has it; busybox-w32 has unzip and no zip. `zip -r a.zip dir` says
// `  adding: NAME (deflated N%)` of each entry, (stored 0%) of a directory and of a file deflating
// would not make smaller, as Info-ZIP decides after compressing; -r goes into directories, an
// entry for each but with -D; -j keeps the names alone; -0 to -9 is the compression, 6 by default;
// -q says nothing; -x PATTERN... leaves out what its patterns match, until the next option, `*`
// crossing `/` as Info-ZIP's wildcards do. An archive's name with no extension gets `.zip`. An
// archive that is there is updated: its other entries are copied as they are, a name in it again
// is `updating:`, and the new go at its end. A NAME that matches nothing is warned of, and nothing
// to do is `Nothing to do!`, status 12. The percent is this deflate's, not zlib's.

type zipRun struct {
	recurse, quiet, junk, noDirs bool
	level                        int
	exclude                      []string
	stdout, stderr               io.Writer
}

// zipEntry is a file or directory to add, and the name it has in the archive.
type zipEntry struct {
	name, host string
	info       fs.FileInfo
}

func newZipApplet() Applet {
	return simpleApplet{name: "zip", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		run := &zipRun{level: flate.DefaultCompression, stdout: stdout, stderr: stderr}
		archive, names, err := run.arguments(args)
		if err != nil {
			return err
		}
		if archive == "" {
			return ExitStatusMessage(16, errors.New("zip error: Invalid command arguments (nothing to select from)"))
		}
		if filepath.Ext(archive) == "" {
			archive += ".zip"
		}
		view := ProcessViewFromContext(ctx)
		var entries []zipEntry
		for _, name := range names {
			entries = append(entries, run.collect(view, name)...)
		}
		if len(entries) == 0 {
			return ExitStatusMessage(12, fmt.Errorf("zip error: Nothing to do! (%s)", archive))
		}
		host, err := resolveHostPath(view, archive)
		if err != nil {
			return err
		}
		return run.write(host, entries)
	}}
}

// arguments reads Info-ZIP's: options anywhere, -x's patterns running to the next option, the
// first word the archive and the rest the names.
func (r *zipRun) arguments(args []string) (string, []string, error) {
	var archive string
	var names []string
	excluding := false
	for _, arg := range args {
		if len(arg) > 1 && arg[0] == '-' {
			excluding = false
			for _, letter := range arg[1:] {
				switch {
				case letter == 'r':
					r.recurse = true
				case letter == 'q':
					r.quiet = true
				case letter == 'j':
					r.junk = true
				case letter == 'D':
					r.noDirs = true
				case letter == 'x':
					excluding = true
				case letter >= '0' && letter <= '9':
					r.level = int(letter - '0')
				default:
					return "", nil, invalidOption(byte(letter))
				}
			}
			continue
		}
		switch {
		case excluding:
			r.exclude = append(r.exclude, arg)
		case archive == "":
			archive = arg
		default:
			names = append(names, arg)
		}
	}
	return archive, names, nil
}

// collect is what one NAME adds: itself, and with -r what is under it.
func (r *zipRun) collect(view ProcessView, name string) []zipEntry {
	host, err := resolveHostPath(view, name)
	var info fs.FileInfo
	if err == nil {
		info, err = os.Stat(host)
	}
	if err != nil {
		fmt.Fprintf(r.stderr, "zip warning: name not matched: %s\n", name)
		return nil
	}
	if !info.IsDir() || !r.recurse {
		return r.keep(nil, zipEntry{name: zipName(name, r.junk), host: host, info: info})
	}
	var entries []zipEntry
	_ = filepath.WalkDir(host, func(walked string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		relative, _ := filepath.Rel(host, walked)
		entryInfo, err := entry.Info()
		if err != nil {
			return nil
		}
		entries = r.keep(entries, zipEntry{name: zipName(path.Join(filepath.ToSlash(name), filepath.ToSlash(relative)), r.junk), host: walked, info: entryInfo})
		return nil
	})
	return entries
}

// keep adds entry to entries unless it is a directory -D or -j leaves out, or -x excludes it.
func (r *zipRun) keep(entries []zipEntry, entry zipEntry) []zipEntry {
	if entry.info.IsDir() {
		if r.noDirs || r.junk {
			return entries
		}
		entry.name = strings.TrimSuffix(entry.name, "/") + "/"
	}
	for _, pattern := range r.exclude {
		if matcher, err := compileFindPathPattern(pattern); err == nil && matcher.MatchString(entry.name) {
			return entries
		}
	}
	return append(entries, entry)
}

// zipName is a path as an archive names it: slashes, no drive, no leading slash or ./, and with -j
// the last component alone.
func zipName(name string, junk bool) string {
	name = filepath.ToSlash(name)
	if len(name) >= 2 && name[1] == ':' {
		name = name[2:]
	}
	name = strings.TrimLeft(path.Clean(name), "/")
	if junk {
		name = path.Base(name)
	}
	return strings.TrimPrefix(name, "./")
}

// write writes the archive afresh beside it and puts it in its place: what was there copied as it
// was, but the names given again, which are written in their places, and the new after.
func (r *zipRun) write(archive string, entries []zipEntry) error {
	pending := make(map[string]zipEntry, len(entries))
	for _, entry := range entries {
		pending[entry.name] = entry
	}
	temporary, err := os.CreateTemp(filepath.Dir(archive), "zi*")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	writer := zip.NewWriter(temporary)
	if existing, err := zip.OpenReader(archive); err == nil {
		for _, file := range existing.File {
			entry, replaced := pending[file.Name]
			if !replaced {
				err = writer.Copy(file)
			} else {
				delete(pending, file.Name)
				err = r.add(writer, entry, "updating: ")
			}
			if err != nil {
				existing.Close()
				return err
			}
		}
		existing.Close()
	}
	for _, entry := range entries {
		if _, waiting := pending[entry.name]; waiting {
			if err := r.add(writer, entry, "  adding: "); err != nil {
				return err
			}
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), archive)
}

// add writes one entry, deflated unless -0 says not or deflating makes it no smaller, and says so.
func (r *zipRun) add(writer *zip.Writer, entry zipEntry, verb string) error {
	header, err := zip.FileInfoHeader(entry.info)
	if err != nil {
		return err
	}
	header.Name, header.Method = entry.name, zip.Store
	var data, packed []byte
	if !entry.info.IsDir() {
		if data, err = os.ReadFile(entry.host); err != nil {
			return err
		}
		if r.level != 0 && len(data) > 0 {
			var buffer bytes.Buffer
			compressor, _ := flate.NewWriter(&buffer, r.level)
			compressor.Write(data)
			compressor.Close()
			if buffer.Len() < len(data) {
				header.Method, packed = zip.Deflate, buffer.Bytes()
			}
		}
	}
	// Written as compressed already, rather than compressed a second time by the writer.
	stored := data
	if header.Method == zip.Deflate {
		stored = packed
	}
	header.CRC32, header.UncompressedSize64, header.CompressedSize64 = crc32.ChecksumIEEE(data), uint64(len(data)), uint64(len(stored))
	out, err := writer.CreateRaw(header)
	if err != nil {
		return err
	}
	if _, err := out.Write(stored); err != nil {
		return err
	}
	if !r.quiet {
		if header.Method == zip.Deflate {
			percent := 100 - (len(packed)*100+len(data)/2)/len(data)
			fmt.Fprintf(r.stdout, "%s%s (deflated %d%%)\n", verb, entry.name, percent)
		} else {
			fmt.Fprintf(r.stdout, "%s%s (stored 0%%)\n", verb, entry.name)
		}
	}
	return nil
}
