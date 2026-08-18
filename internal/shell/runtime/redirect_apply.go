package runtime

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

func (r Runtime) applyRedirectOperations(table *fdTable, operations []redirectOperation) error {
	for _, operation := range operations {
		var err error
		switch operation.kind {
		case redirectInput:
			err = r.bindInputRedirect(table, operation)
		case redirectOutput, redirectClobber, redirectReadWrite, redirectAppend:
			err = r.bindOutputRedirect(table, operation)
			// `&>`: stderr goes wherever stdout just went. After the bind rather
			// than before, because it is descriptor 1 as it now is that 2 has to
			// follow -- the same reason `2>&1` has to come after `>file`.
			if err == nil && operation.bothStreams {
				err = table.dup(2, 1)
			}
		case redirectHeredoc, redirectHereString:
			err = table.bindOwnedReader(operation.target, io.NopCloser(bytes.NewReader([]byte(operation.body))))
		case redirectDup:
			err = table.dup(operation.target, operation.source)
		case redirectClose:
			err = table.close(operation.target)
		}
		if err != nil {
			return fmt.Errorf("redirect descriptor %d: %w", operation.target, err)
		}
	}
	return nil
}

func (r Runtime) bindInputRedirect(table *fdTable, operation redirectOperation) error {
	resolved, err := r.ResolveNemoshPath(operation.path)
	if err != nil {
		return err
	}
	if !resolved.Device {
		resource, openErr := os.Open(resolved.Native)
		if openErr != nil {
			return fmt.Errorf("open %s: %w", operation.path, openErr)
		}
		return table.bindOwnedReader(operation.target, resource)
	}
	path := string(resolved.Canonical)
	if source, alias, err := deviceAlias(path); err != nil {
		return err
	} else if alias {
		return table.alias(operation.target, source, readable)
	}
	resource, err := openInputDevice(path)
	if err != nil {
		return err
	}
	return table.bindOwnedReader(operation.target, resource)
}

func (r Runtime) bindOutputRedirect(table *fdTable, operation redirectOperation) error {
	resolved, err := r.ResolveNemoshPath(operation.path)
	if err != nil {
		return err
	}
	if !resolved.Device {
		if err := r.refuseClobber(resolved, operation); err != nil {
			return err
		}
		resource, openErr := openHostOutput(resolved, operation.kind)
		if openErr != nil {
			return fmt.Errorf("open %s: %w", operation.path, openErr)
		}
		return table.bindOwnedWriter(operation.target, resource)
	}
	path := string(resolved.Canonical)
	if source, alias, err := deviceAlias(path); err != nil {
		return err
	} else if alias {
		return table.alias(operation.target, source, writable)
	}
	resource, err := openOutputDevice(path, operation.kind == redirectAppend)
	if err != nil {
		return err
	}
	return table.bindOwnedWriter(operation.target, resource)
}

func openHostOutput(resolved pathmodel.ResolvedPath, kind redirectKind) (*os.File, error) {
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	switch kind {
	case redirectAppend:
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	case redirectReadWrite:
		// `<>` opens for both and truncates nothing, which is what makes it
		// usable on a file you mean to overwrite in place.
		flags = os.O_RDWR | os.O_CREATE
	}
	return os.OpenFile(resolved.Native, flags, 0o666)
}

// refuseClobber is `set -C`: a plain `>` must not truncate a file that is
// already there. `>|` says to do it anyway, which is the only reason that
// operator exists, and `>>` and `<>` never truncate so neither is affected.
//
// The check is a stat a moment before the open rather than part of it, which is
// a race Go's portable API leaves no way to close. What it buys is the ordinary
// case: a typo in a redirect target cannot destroy the file it names.
func (r Runtime) refuseClobber(resolved pathmodel.ResolvedPath, operation redirectOperation) error {
	if !r.options.noClobber || operation.kind != redirectOutput {
		return nil
	}
	info, err := os.Lstat(resolved.Native)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	return fmt.Errorf("%s: cannot overwrite existing file", operation.path)
}

func (t *fdTable) bindOwnedReader(fd int, resource io.ReadCloser) error {
	description := &openDescription{reader: resource, closer: resource, refs: 1}
	if err := validateDescriptor(fd); err != nil {
		return errors.Join(err, description.release())
	}
	return t.rebind(fd, &fdEntry{description: description, capability: readable})
}

func (t *fdTable) bindOwnedWriter(fd int, resource io.WriteCloser) error {
	description := &openDescription{writer: resource, closer: resource, refs: 1}
	if err := validateDescriptor(fd); err != nil {
		return errors.Join(err, description.release())
	}
	return t.rebind(fd, &fdEntry{description: description, capability: writable})
}

func (t *fdTable) bindBorrowedReader(fd int, reader io.Reader) error {
	if err := validateDescriptor(fd); err != nil {
		return err
	}
	return t.rebind(fd, &fdEntry{
		description: newBorrowedDescription(reader, nil),
		capability:  readable,
	})
}

func (t *fdTable) bindBorrowedWriter(fd int, writer io.Writer) error {
	if err := validateDescriptor(fd); err != nil {
		return err
	}
	return t.rebind(fd, &fdEntry{
		description: newBorrowedDescription(nil, writer),
		capability:  writable,
	})
}
