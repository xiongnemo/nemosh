package applets

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// unzip: the archive format Windows actually uses, and one it has no command-line
// tool for -- `System32` holds `tar.exe` and `curl.exe` and no `unzip` (measured).
//
// zip is read through archive/zip, which needs a ReaderAt and a size, so unlike
// tar this cannot work on a pipe: the central directory lives at the *end* of the
// file. An operand is therefore required, and saying so is better than reading
// the whole of stdin into memory to pretend otherwise.

func newUnzipApplet() Applet {
	return simpleApplet{name: "unzip", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletOptions(args, "lnojptqK", "dx")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			// Not a pipe: the central directory is at the end of the file, so the
			// reader must be able to seek.
			return fmt.Errorf("an archive operand is required; zip cannot be read from a pipe")
		}
		view := ProcessViewFromContext(ctx)
		native, err := resolveHostPath(view, operands[0])
		if err != nil {
			return operandFailure(operands[0], err)
		}
		archive, err := zip.OpenReader(native)
		if err != nil {
			return operandFailure(operands[0], fmt.Errorf("cannot read as a zip archive"))
		}
		defer archive.Close()
		request := unzipRequest{
			list:      options.has('l'),
			test:      options.has('t'),
			toStdout:  options.has('p'),
			flatten:   options.has('j'),
			never:     options.has('n'),
			overwrite: options.has('o'),
			quiet:     options.has('q'),
			exclude:   options.value('x'),
			wanted:    operands[1:],
		}
		root, err := unzipRoot(ctx, options)
		if err != nil {
			return err
		}
		return request.run(&archive.Reader, root, stdout, stderr)
	}}
}

func unzipRoot(ctx context.Context, options appletOptions) (string, error) {
	target := "."
	if options.has('d') {
		target = options.value('d')
	}
	native, err := resolveHostPath(ProcessViewFromContext(ctx), target)
	if err != nil {
		return "", operandFailure(target, err)
	}
	return native, nil
}

type unzipRequest struct {
	list      bool
	test      bool
	toStdout  bool
	flatten   bool
	never     bool
	overwrite bool
	quiet     bool
	exclude   string
	wanted    []string
}

func (r unzipRequest) run(archive *zip.Reader, root string, stdout, stderr io.Writer) error {
	if r.list {
		return r.writeListing(archive, stdout)
	}
	collisions := newArchiveCollisions()
	for _, entry := range archive.File {
		if !r.selects(entry.Name) {
			continue
		}
		if err := r.oneEntry(entry, root, collisions, stdout, stderr); err != nil {
			return err
		}
	}
	return nil
}

// selects applies the operand list and -x. With no operands every entry is
// wanted, which is the common case.
func (r unzipRequest) selects(name string) bool {
	if r.exclude != "" {
		if matched, err := filepath.Match(r.exclude, name); err == nil && matched {
			return false
		}
	}
	if len(r.wanted) == 0 {
		return true
	}
	for _, pattern := range r.wanted {
		if matched, err := filepath.Match(pattern, name); err == nil && matched {
			return true
		}
		if pattern == name {
			return true
		}
	}
	return false
}

// writeListing prints the table busybox prints: length, date, time, name, with a
// header and a total.
//
// Names are listed as the archive holds them, unchecked, because listing is how
// somebody inspects an archive they do not trust -- refusing here would hide the
// very entry they are looking for.
func (r unzipRequest) writeListing(archive *zip.Reader, stdout io.Writer) error {
	if !r.quiet {
		if _, err := fmt.Fprintf(stdout, "%9s  %-19s %s\n", "Length", "Date", "Name"); err != nil {
			return err
		}
	}
	var total uint64
	count := 0
	for _, entry := range archive.File {
		if !r.selects(entry.Name) {
			continue
		}
		total += entry.UncompressedSize64
		count++
		when := entry.Modified.Format("2006-01-02 15:04")
		if _, err := fmt.Fprintf(stdout, "%9d  %-19s %s\n", entry.UncompressedSize64, when, entry.Name); err != nil {
			return err
		}
	}
	if r.quiet {
		return nil
	}
	_, err := fmt.Fprintf(stdout, "%9d  %-19s %d files\n", total, "", count)
	return err
}

func (r unzipRequest) oneEntry(entry *zip.File, root string, collisions *archiveCollisions, stdout, stderr io.Writer) error {
	safe, err := safeArchivePath(entry.Name)
	if err != nil {
		// Skipped with the reason rather than aborting: one hostile entry among
		// honest ones should not cost the honest ones.
		fmt.Fprintf(stderr, "unzip: skipping %v\n", err)
		return nil
	}
	if r.flatten {
		// -j discards the stored directories, so the name has to be re-checked:
		// flattening `../evil` to `evil` is safe, but flattening to a reserved
		// device name is not.
		safe = filepath.Base(safe)
		if _, err := safeArchivePath(safe); err != nil {
			fmt.Fprintf(stderr, "unzip: skipping %v\n", err)
			return nil
		}
	}
	if err := collisions.check(safe); err != nil {
		fmt.Fprintf(stderr, "unzip: skipping %v\n", err)
		return nil
	}
	if entry.FileInfo().IsDir() {
		if r.test || r.toStdout {
			return nil
		}
		return os.MkdirAll(filepath.Join(root, filepath.FromSlash(safe)), 0o755)
	}
	source, err := entry.Open()
	if err != nil {
		return fmt.Errorf("cannot read %s: %v", entry.Name, err)
	}
	defer source.Close()

	switch {
	case r.test:
		// Read and discarded, which is what makes -t detect a corrupt member
		// rather than merely reading the directory.
		if _, err := io.Copy(io.Discard, source); err != nil {
			return fmt.Errorf("%s is corrupt: %v", entry.Name, err)
		}
		return nil
	case r.toStdout:
		_, err := io.Copy(stdout, source)
		return err
	}
	destination := filepath.Join(root, filepath.FromSlash(safe))
	if _, err := os.Stat(destination); err == nil {
		if r.never {
			return nil
		}
		if !r.overwrite {
			// Neither -o nor -n given. busybox asks interactively; there is no
			// prompt here, so the safe half of that choice is taken and named.
			fmt.Fprintf(stderr, "unzip: %s exists; use -o to overwrite or -n to skip\n", safe)
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, source)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if !r.quiet {
		fmt.Fprintf(stderr, "  inflating: %s\n", safe)
	}
	return nil
}
