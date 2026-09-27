package runtime

import "io"

// externalStreams is an external command's stdout and stderr: each descriptor's real handle
// where there is one, so the child writes to the console or the file itself, and otherwise
// the writer, which exec copies from a pipe of its own. See nativeWriter.
//
// **One writer for one open file.** Under `2>&1` the two descriptors are one description,
// and a child given two writers got two pipes, each copied by its own goroutine: what it
// wrote to stdout and to stderr in one order reached the file in another. `make > log 2>&1`
// put its errors wherever the copiers happened to, and an Oils case that traces a child
// passed on one machine and failed on the next. exec gives the child a single pipe when the
// two are the same writer, and its writes then arrive as it made them, as in both
// references.
func (r Runtime) externalStreams() (io.Writer, io.Writer) {
	stdout, stderr := r.streams.Stdout, r.streams.Stderr
	if native := nativeWriter(stdout); native != nil {
		stdout = native
	}
	if shared := streamDescription(r.streams.Stdout); shared != nil && shared == streamDescription(r.streams.Stderr) {
		return stdout, stdout
	}
	if native := nativeWriter(stderr); native != nil {
		stderr = native
	}
	return stdout, stderr
}

// streamDescription is the open file a stream writes to, when it is one of the descriptor
// table's, and nil otherwise.
func streamDescription(writer io.Writer) *openDescription {
	for range maxWriterUnwrapDepth {
		switch current := writer.(type) {
		case synchronizedWriter:
			writer = current.writer
		case descriptorWriter:
			resolved, err := current.table.writer(current.fd)
			if err != nil {
				return nil
			}
			writer = resolved
		case *openDescription:
			return current
		default:
			return nil
		}
	}
	return nil
}
