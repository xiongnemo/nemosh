package applets

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// oneEntry extracts an entry as busybox's unzip does, and says so on standard output unless
// -q: `   creating: DIR/` for a directory that is not there yet, `  inflating: NAME` for a
// file. A directory went unsaid, and each file was named on standard error.
func (r unzipRequest) oneEntry(entry *zip.File, root string, collisions *archiveCollisions, stdout, stderr io.Writer) error {
	safe, err := safeArchivePath(entry.Name)
	if err != nil {
		// Skipped with the reason rather than aborting: one hostile entry among
		// honest ones should not cost the honest ones.
		fmt.Fprintf(stderr, "unzip: skipping %v\n", err)
		return nil
	}
	directory := entry.FileInfo().IsDir()
	if r.flatten {
		// -j keeps no directory, so one is passed over, as busybox's is; it was made.
		if directory {
			return nil
		}
		// -j discards the stored directories, so the name has to be re-checked:
		// flattening `../evil` to `evil` is safe, but flattening to a reserved
		// device name is not.
		safe = path.Base(safe)
		if _, err := safeArchivePath(safe); err != nil {
			fmt.Fprintf(stderr, "unzip: skipping %v\n", err)
			return nil
		}
	}
	if err := collisions.check(safe); err != nil {
		fmt.Fprintf(stderr, "unzip: skipping %v\n", err)
		return nil
	}
	if r.test || r.toStdout {
		if directory {
			return nil
		}
		return r.copyEntry(entry, stdout)
	}
	destination := filepath.Join(root, filepath.FromSlash(safe))
	if directory {
		return r.makeDirectory(destination, safe+"/", stdout)
	}
	return r.extractFile(entry, destination, safe, stdout, stderr)
}

// makeDirectory is a directory entry: made, and said, where nothing is; a directory there
// already is left as it is, and anything else there is busybox's failure.
func (r unzipRequest) makeDirectory(destination, name string, stdout io.Writer) error {
	info, err := os.Lstat(destination)
	switch {
	case err == nil && info.IsDir():
		return nil
	case err == nil:
		return fmt.Errorf("'%s' exists but is not a directory", name)
	case !errors.Is(err, fs.ErrNotExist):
		return cannotStat(name, err)
	}
	if r.quiet == 0 {
		fmt.Fprintf(stdout, "   creating: %s\n", name)
	}
	if err := os.MkdirAll(destination, createMode(r.view, 0o755)); err != nil {
		return cannotCreateDirectory(name, err)
	}
	return nil
}

// extractFile writes a file entry where nothing is, or over a file with -o; -n leaves one
// there as it is. What is there and is no file -- a directory -- stops the extraction, as
// busybox's does.
func (r unzipRequest) extractFile(entry *zip.File, destination, name string, stdout, stderr io.Writer) error {
	info, err := os.Lstat(destination)
	switch {
	case err == nil && r.overwrite == 'n':
		return nil
	case err == nil && !info.Mode().IsRegular():
		return fmt.Errorf("'%s' exists but is not a regular file", name)
	case err == nil && r.overwrite != 'o':
		// Neither -o nor -n given. busybox asks interactively; there is no
		// prompt here, so the safe half of that choice is taken and named.
		fmt.Fprintf(stderr, "unzip: %s exists; use -o to overwrite or -n to skip\n", name)
		return nil
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return cannotStat(name, err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), createMode(r.view, 0o755)); err != nil {
		return cannotCreateDirectory(path.Dir(name), err)
	}
	file, err := createFile(r.view, destination)
	if err != nil {
		return cannotOpen(name, err)
	}
	if r.quiet == 0 {
		fmt.Fprintf(stdout, "  inflating: %s\n", name)
	}
	copyErr := r.copyEntry(entry, file)
	if closeErr := file.Close(); copyErr == nil {
		copyErr = closeErr
	}
	return copyErr
}

// copyEntry writes an entry's data to out, or under -t reads it to nothing, which is what
// makes -t find a corrupt member rather than merely read the directory.
func (r unzipRequest) copyEntry(entry *zip.File, out io.Writer) error {
	source, err := entry.Open()
	if err != nil {
		return fmt.Errorf("cannot read %s: %v", entry.Name, err)
	}
	defer source.Close()
	if !r.test {
		_, err := io.Copy(out, source)
		return err
	}
	if _, err := io.Copy(io.Discard, source); err != nil {
		return fmt.Errorf("%s is corrupt: %v", entry.Name, err)
	}
	return nil
}
