package applets

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// ar's two halves that touch the filesystem, split from ar.go for the line
// ceiling on the same seam cpio uses.

// extract writes one member into the working directory.
//
// ar has no directories at all -- every member is a plain file with a flat name --
// which removes most of the ways an archive can escape but not all of them. A
// member called `..\evil` or `NUL` is still a name from an untrusted document, so
// it goes through the same check tar, unzip, cpio and httpd use.
func (r arRequest) extract(reader io.Reader, member arMember, name string,
	collisions *archiveCollisions, stderr io.Writer) error {
	safe, err := safeArchivePath(name)
	if err != nil {
		fmt.Fprintf(stderr, "ar: skipping %v\n", err)
		return skipArBody(reader, member)
	}
	if err := collisions.check(safe); err != nil {
		fmt.Fprintf(stderr, "ar: skipping %v\n", err)
		return skipArBody(reader, member)
	}
	native, err := resolveHostPath(r.view, safe)
	if err != nil {
		return operandFailure(safe, err)
	}
	file, err := os.Create(native)
	if err != nil {
		return operandFailure(safe, err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(reader, member.size))
	closeErr := file.Close()
	if copyErr != nil {
		os.Remove(native)
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written != member.size {
		os.Remove(native)
		return fmt.Errorf("%s is truncated: %d of %d bytes", safe, written, member.size)
	}
	if r.keepTime && member.mtime != 0 {
		when := time.Unix(member.mtime, 0)
		if err := os.Chtimes(native, when, when); err != nil {
			return err
		}
	}
	if r.verbose {
		fmt.Fprintf(stderr, "x - %s\n", safe)
	}
	return skipArPadding(reader, member.size)
}

// create writes a new archive from the named files.
//
// `r` means replace, and in both references it adds to an existing archive. This
// one *creates*, and refuses to touch an archive that already exists rather than
// pretending: appending correctly means reading every existing header, deciding
// which member each operand replaces, and rewriting the long-name table -- and a
// half-done version of that silently corrupts the archive it was given.
func (r arRequest) create(native string, stderr io.Writer) error {
	if len(r.members) == 0 {
		return fmt.Errorf("at least one file to archive is required")
	}
	if _, err := os.Stat(native); err == nil {
		return fmt.Errorf("%s exists; this build creates an archive and does not add to one", r.archive)
	}
	file, err := os.Create(native)
	if err != nil {
		return operandFailure(r.archive, err)
	}
	writer := bufio.NewWriter(file)
	writeErr := r.writeMembers(writer, stderr)
	if writeErr == nil {
		writeErr = writer.Flush()
	}
	closeErr := file.Close()
	if writeErr != nil {
		// A half-written archive that looks like a real one is worse than none.
		os.Remove(native)
		return writeErr
	}
	return closeErr
}

func (r arRequest) writeMembers(writer io.Writer, stderr io.Writer) error {
	if _, err := io.WriteString(writer, arMagic); err != nil {
		return err
	}
	for _, name := range r.members {
		if err := r.addMember(writer, name, stderr); err != nil {
			return err
		}
	}
	return nil
}

func (r arRequest) addMember(writer io.Writer, name string, stderr io.Writer) error {
	native, err := resolveHostPath(r.view, name)
	if err != nil {
		return operandFailure(name, err)
	}
	info, err := os.Stat(native)
	if err != nil {
		return operandFailure(name, err)
	}
	if info.IsDir() {
		// Not a silent skip: ar has no way to represent a directory, so a caller
		// who named one meant something the format cannot do.
		return fmt.Errorf("%s is a directory, and an ar archive holds only files", name)
	}
	source, err := os.Open(native)
	if err != nil {
		return operandFailure(name, err)
	}
	defer source.Close()
	member := arMember{
		// The stored name is the base name, which is what both references do: ar
		// has no directories, so `ar r lib.a src/a.txt` stores `a.txt`.
		name:  filepath.Base(name),
		mtime: info.ModTime().Unix(),
		mode:  arModeOf(info),
		size:  info.Size(),
	}
	if err := writeArHeader(writer, member); err != nil {
		return err
	}
	written, err := io.Copy(writer, source)
	if err != nil {
		return err
	}
	if written != member.size {
		return fmt.Errorf("%s changed size while being archived: %d bytes, header says %d",
			name, written, member.size)
	}
	if r.verbose {
		fmt.Fprintf(stderr, "a - %s\n", member.name)
	}
	return writeArPadding(writer, member.size)
}

// arModeOf synthesises the Unix mode, the same way cpio does and for the same
// reason: Windows has no execute bit and no group or other permissions, so writing
// Go's own os.FileMode bits into a field that means something else would be a
// misreport rather than a translation.
func arModeOf(info os.FileInfo) int64 {
	if info.Mode().Perm()&0o200 == 0 {
		return 0o444
	}
	return 0o644
}
