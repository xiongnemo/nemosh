package runtime

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

func (r Runtime) applyRedirectOperations(table *fdTable, operations []redirectOperation) error {
	for _, operation := range operations {
		var err error
		if operation.name != "" {
			if operation.target, err = r.namedTarget(table, operation); err != nil {
				return err
			}
		}
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
			err = table.bindOwnedReader(operation.target, newMemoryInput([]byte(operation.body)))
		case redirectDup:
			// A descriptor made a copy of itself is left as it is, open or not, as both
			// references leave it (busybox's `if (fd == newfd) continue`); `3>&3` with 3
			// closed was an error, which a special builtin's now ends the script with.
			// Moved onto itself, `3>&3-`, it stays too, as in bash.
			if operation.target == operation.source {
				break
			}
			if err = table.dup(operation.target, operation.source); err != nil {
				return dupFailure{source: operation.source, target: operation.target, err: err}
			}
			if operation.move {
				err = table.close(operation.source)
			}
		case redirectClose:
			err = table.close(operation.target)
		}
		if err != nil {
			if failure := (redirectFailure{}); errors.As(err, &failure) {
				return err
			}
			return fmt.Errorf("redirect descriptor %d: %w", operation.target, err)
		}
	}
	return nil
}

func (r Runtime) bindInputRedirect(table *fdTable, operation redirectOperation) error {
	resolved, err := r.ResolveNemoshPath(operation.path)
	if err != nil {
		return redirectFailure{path: operation.path, err: err}
	}
	if !resolved.Device {
		// A directory is refused as it is opened, as every reader here refuses one: Windows lets
		// Go open it, and the first read failed with `Incorrect function`.
		resource, openErr := applets.OpenHostInput(resolved.Native)
		if openErr != nil {
			return redirectFailure{path: operation.path, err: openErr}
		}
		return table.bindOwnedReader(operation.target, resource)
	}
	path := string(resolved.Canonical)
	if source, alias, err := deviceAlias(path); err != nil {
		return redirectFailure{path: operation.path, err: err}
	} else if alias {
		if err := table.alias(operation.target, source, readable); err != nil {
			return redirectFailure{path: operation.path, err: badDescriptor{err}}
		}
		return nil
	}
	resource, err := openInputDevice(path)
	if err != nil {
		return redirectFailure{path: operation.path, err: err}
	}
	return table.bindOwnedReader(operation.target, resource)
}

func (r Runtime) bindOutputRedirect(table *fdTable, operation redirectOperation) error {
	resolved, err := r.ResolveNemoshPath(operation.path)
	if err != nil {
		return redirectFailure{create: true, path: operation.path, err: err}
	}
	if !resolved.Device {
		if err := r.refuseClobber(resolved, operation); err != nil {
			return err
		}
		resource, openErr := openHostOutput(resolved, operation.kind, r.createMode(0o666))
		if openErr != nil {
			return redirectFailure{create: true, path: operation.path, err: openErr}
		}
		// `<>` is opened for both, and the descriptor reads as well as writes. It was bound
		// as a writer, so `exec 3<> f; read <&3` answered "file descriptor is not readable".
		if operation.kind == redirectReadWrite {
			return table.bindOwned(operation.target, resource, readWrite)
		}
		return table.bindOwnedWriter(operation.target, resource)
	}
	path := string(resolved.Canonical)
	if source, alias, err := deviceAlias(path); err != nil {
		return redirectFailure{create: true, path: operation.path, err: err}
	} else if alias {
		if err := table.alias(operation.target, source, writable); err != nil {
			return redirectFailure{create: true, path: operation.path, err: badDescriptor{err}}
		}
		return nil
	}
	resource, err := openOutputDevice(path, operation.kind == redirectAppend)
	if err != nil {
		return redirectFailure{create: true, path: operation.path, err: err}
	}
	return table.bindOwnedWriter(operation.target, resource)
}

func openHostOutput(resolved pathmodel.ResolvedPath, kind redirectKind, perm os.FileMode) (*os.File, error) {
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	switch kind {
	case redirectAppend:
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	case redirectReadWrite:
		// `<>` opens for both and truncates nothing, which is what makes it
		// usable on a file you mean to overwrite in place.
		flags = os.O_RDWR | os.O_CREATE
	}
	return os.OpenFile(resolved.Native, flags, perm)
}

// refuseClobber is `set -C`: a plain `>` must not truncate a file that is
// already there. `>|` says to do it anyway, which is the only reason that
// operator exists, and `>>` and `<>` never truncate so neither is affected.
//
// The check is a stat a moment before the open rather than part of it, which is
// a race Go's portable API leaves no way to close. What it buys is the ordinary
// case: a typo in a redirect target cannot destroy the file it names.
//
// The refusal is busybox's, `cannot create f: File exists`, where it was bash's `f: cannot
// overwrite existing file`.
func (r Runtime) refuseClobber(resolved pathmodel.ResolvedPath, operation redirectOperation) error {
	if !r.options.noClobber || operation.kind != redirectOutput {
		return nil
	}
	info, err := os.Lstat(resolved.Native)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	return redirectFailure{create: true, path: operation.path, err: fs.ErrExist}
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
