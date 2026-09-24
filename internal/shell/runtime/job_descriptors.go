package runtime

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// A job process's descriptors, section 4 of docs/design/background-processes.md. The
// launcher clones the shell's descriptor table, as the goroutine's worker does, so the job
// writes where the shell's descriptors pointed when it started: an `exec >file` in the
// shell afterwards does not take the job's output with it. Each descriptor reaches the
// child as a handle -- the file itself where there is one, and otherwise a pipe with a copy
// goroutine here, for what lives in memory: a buffer, a command substitution's capture, a
// heredoc.

// jobDescriptor is one entry of the job's descriptor table as the child receives it: the
// number the job knows it by, the handle it arrives under, what it may do. No handle is a
// descriptor the shell had closed.
type jobDescriptor struct {
	FD       int    `json:"fd"`
	Handle   string `json:"handle,omitempty"`
	Readable bool   `json:"readable,omitempty"`
	Writable bool   `json:"writable,omitempty"`
}

// jobHandoff is the parent's side of one job's descriptors: the cloned table, released when
// the job ends; the ends the child was given, closed here once it has them; and the copies
// of the child's output, which finish before the job counts as done.
type jobHandoff struct {
	command *exec.Cmd
	table   *fdTable
	given   []func() error
	copies  sync.WaitGroup
}

// handOffDescriptors wires the shell's descriptors into command, and answers with the ones
// the child binds itself. 0 is not among them: a job's stdin is /dev/null, as POSIX gives an
// asynchronous list without job control and as the goroutine's worker has it. 1 and 2 are
// the child's own standard handles.
func (r Runtime) handOffDescriptors(command *exec.Cmd) (*jobHandoff, []jobDescriptor, error) {
	table, err := r.fds.clone()
	if err != nil {
		return nil, nil, err
	}
	handoff := &jobHandoff{command: command, table: table}
	var descriptors []jobDescriptor
	for _, fd := range table.numbers() {
		if fd == 0 {
			continue
		}
		descriptor, err := handoff.pass(fd)
		if err != nil {
			return nil, nil, errors.Join(err, handoff.abandon())
		}
		if descriptor != nil {
			descriptors = append(descriptors, *descriptor)
		}
	}
	return handoff, descriptors, nil
}

func (h *jobHandoff) pass(fd int) (*jobDescriptor, error) {
	entry, err := h.table.lookup(fd)
	if err != nil {
		return &jobDescriptor{FD: fd}, nil
	}
	description := entry.description
	canRead := entry.capability&readable != 0 && description.reader != nil
	canWrite := entry.capability&writable != 0 && description.writer != nil
	if fd == 1 || fd == 2 {
		if !canWrite {
			return &jobDescriptor{FD: fd}, nil
		}
		// os/exec hands a file over as it is and copies anything else through a pipe.
		var output io.Writer = description
		if file := nativeWriter(description.writer); file != nil {
			output = file
		}
		if fd == 1 {
			h.command.Stdout = output
		} else {
			h.command.Stderr = output
		}
		return nil, nil
	}
	file, err := h.fileFor(description, canRead, canWrite)
	if err != nil {
		return nil, fmt.Errorf("descriptor %d: %w", fd, err)
	}
	handle, err := h.give(file)
	if err != nil {
		return nil, fmt.Errorf("descriptor %d: %w", fd, err)
	}
	return &jobDescriptor{FD: fd, Handle: handle, Readable: canRead, Writable: canWrite}, nil
}

// fileFor is the file the child is given for description: its own, or a pipe end.
func (h *jobHandoff) fileFor(description *openDescription, canRead, canWrite bool) (*os.File, error) {
	if canWrite {
		if file, ok := nativeWriter(description.writer).(*os.File); ok {
			return file, nil
		}
	} else if file, ok := nativeReader(description.reader).(*os.File); ok {
		return file, nil
	}
	source := description.reader
	if memory, ok := source.(memoryInput); ok && !canWrite {
		return memory.share()
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	if canWrite {
		h.given = append(h.given, writer.Close)
		h.copies.Go(func() {
			_, _ = io.Copy(description, reader)
			_ = reader.Close()
		})
		return writer, nil
	}
	// Not waited for: the source can be a device that never ends, and the copy stops once
	// the child has gone and the pipe refuses the next write. A stream that is not in
	// memory is shared with the shell, as a pipe two processes hold is: what the job reads
	// the shell does not.
	h.given = append(h.given, reader.Close)
	go func() {
		_, _ = io.Copy(writer, source)
		_ = writer.Close()
	}()
	return reader, nil
}

// give makes file inheritable by the child, and answers with the handle it will find it
// under. What give made is closed with the ends, once the child has started.
func (h *jobHandoff) give(file *os.File) (string, error) {
	handle, release, err := inheritFile(h.command, file)
	if err != nil {
		return "", err
	}
	h.given = append(h.given, release)
	return handle, nil
}

// started closes this process's copies of what the child now holds, so a pipe the child
// writes to ends when the child does.
func (h *jobHandoff) started() error {
	var err error
	for _, release := range h.given {
		err = errors.Join(err, release())
	}
	h.given = nil
	return err
}

// finish is the job's end: the output copied, the cloned table released.
func (h *jobHandoff) finish() error {
	h.copies.Wait()
	return h.table.closeAll()
}

// abandon is a launch that did not happen.
func (h *jobHandoff) abandon() error {
	return errors.Join(h.started(), h.finish())
}

// bindJobDescriptors is the child's side: each descriptor the job was given, bound under
// the number the job knows it by.
func (r Runtime) bindJobDescriptors(descriptors []jobDescriptor) error {
	for _, descriptor := range descriptors {
		if descriptor.Handle == "" {
			if err := r.fds.close(descriptor.FD); err != nil {
				return err
			}
			continue
		}
		file, err := inheritedFile(descriptor.Handle, fmt.Sprintf("fd%d", descriptor.FD))
		if err != nil {
			return err
		}
		var capability fdCapability
		if descriptor.Readable {
			capability |= readable
		}
		if descriptor.Writable {
			capability |= writable
		}
		if err := r.fds.bindOwned(descriptor.FD, file, capability); err != nil {
			return err
		}
	}
	return nil
}
