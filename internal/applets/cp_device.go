package applets

import (
	"fmt"
	"io"
	"io/fs"
	"os"
)

// cp and install take a device of the shell's as SOURCE or DEST, as busybox-w32's do: `cp
// /dev/null log` empties a log, `cp /dev/stdin f` keeps what was piped in, `install -m 644
// /dev/null f` makes an empty file with a mode, and `cp f /dev/stdout` shows f. What a device
// reads goes into a new file, as busybox copies a character device without -R, and what is
// copied to one is written to it, as a redirection writes it. A device has no host path, so each
// was "not a host path".

// copyOperand resolves an operand for cp and install: a host path, or a device of the shell's,
// which has none. /dev itself is a directory and no device, and resolveHostPath refuses it.
func copyOperand(view ProcessView, operand string) (pathOperand, error) {
	if resolved, err := ResolveProcessPath(view, operand); err == nil && resolved.Device && resolved.Canonical != "/dev" {
		return pathOperand{operand: operand, device: true}, nil
	}
	host, err := resolveHostPath(view, operand)
	if err != nil {
		return pathOperand{}, err
	}
	return pathOperand{host: host, operand: operand}, nil
}

// statSource is SOURCE as stat sees it, the shell's answer for a device of its own.
func (r *cpRun) statSource(source pathOperand, stat func(string) (fs.FileInfo, error)) (fs.FileInfo, error) {
	if source.device {
		return statProcessPath(r.view, source.operand, false)
	}
	return stat(source.host)
}

// openSource opens SOURCE for what it holds.
func (r *cpRun) openSource(source pathOperand) (io.ReadCloser, error) {
	if source.device {
		return openProcessInput(r.view, source.operand)
	}
	return os.Open(source.host)
}

// copyToDevice writes what source holds to a device, which is opened as a redirection opens it
// rather than removed and made afresh as a file is.
func (r *cpRun) copyToDevice(source, dest pathOperand, info fs.FileInfo) bool {
	if info.IsDir() {
		return r.fail(fmt.Errorf("cannot overwrite non-directory '%s' with directory '%s'", dest.operand, source.operand))
	}
	reader, err := r.openSource(source)
	if err != nil {
		return r.fail(cannotOpen(source.operand, err))
	}
	defer reader.Close()
	writer, err := openProcessOutput(r.view, dest.operand, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0)
	if err != nil {
		return r.fail(cannotCreate(dest.operand, err))
	}
	_, copyErr := io.Copy(writer, reader)
	if closeErr := writer.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return r.fail(fmt.Errorf("error writing to '%s': %s", dest.operand, causeText(copyErr)))
	}
	// -v names a copy of a file, as busybox's copy_file names it, and not one of a device.
	if info.Mode().IsRegular() {
		r.report(source, dest)
	}
	return true
}
